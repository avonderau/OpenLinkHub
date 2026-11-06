package devices

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"OpenLinkHub/src/common"
	"OpenLinkHub/src/devices/deck"
	"github.com/sstallion/go-hid"
)

func deckTestInfo(vid, pid uint16, path, serial string) *hid.DeviceInfo {
	return &hid.DeviceInfo{
		VendorID: vid, ProductID: pid, Path: path, SerialNbr: serial,
		UsagePage: 0xffa0, Usage: 1,
	}
}

type fakeDeckDiscovery struct {
	devices []*hid.DeviceInfo
	registered map[string]*common.Device
	enabled map[string]bool
	initializeCalls int
	registerCalls int
	failInitialization bool
}

func newFakeDeckDiscovery() *fakeDeckDiscovery {
	return &fakeDeckDiscovery{
		registered: make(map[string]*common.Device),
		enabled: make(map[string]bool),
	}
}

func (h *fakeDeckDiscovery) dependencies() deckScanDependencies {
	return deckScanDependencies{
		enumerate: func(vendor uint16, visit hid.EnumFunc) error {
			for _, device := range h.devices {
				if device.VendorID == vendor {
					if err := visit(device); err != nil { return err }
				}
			}
			return nil
		},
		initialize: func(info *hid.DeviceInfo) *common.Device {
			h.initializeCalls++
			if h.failInitialization { return nil }
			model, _ := deck.LookupModel(info.VendorID, info.ProductID)
			return &common.Device{
				Serial: deck.CandidateDeviceID(info, model), Product: model.Name, Instance: struct{}{},
			}
		},
		enabled: func(key string) bool { return h.enabled[key] },
		present: func(serial string) bool { return h.registered[serial] != nil },
		register: func(device *common.Device) {
			h.registerCalls++
			if h.registered[device.Serial] != nil {
				panic("duplicate Deck registration")
			}
			h.registered[device.Serial] = device
		},
	}
}

func TestDeckHotplugFindsDevicesAddedAfterStartup(t *testing.T) {
	h := newFakeDeckDiscovery()
	h.enabled[deck.DeviceKey(0x5548,0x6670)] = true
	deps := h.dependencies()
	if got := scanDeckDevicesWith(deps); got != 0 { t.Fatalf("startup discovered %d phantom devices",got) }
	h.devices = []*hid.DeviceInfo{deckTestInfo(0x5548,0x6670,"/dev/hidraw1","")}
	if got := scanDeckDevicesWith(deps); got != 1 { t.Fatalf("hotplug discovered %d, want one",got) }
	if got := scanDeckDevicesWith(deps); got != 0 { t.Fatalf("repeat scan reinitialized existing device %d times",got) }
	if h.initializeCalls != 1 || h.registerCalls != 1 {
		t.Fatalf("hotplug must register exactly once: init=%d register=%d",h.initializeCalls,h.registerCalls)
	}
	// A disconnected device stays registered and runs its own HID reconnect
	// loop. The scanner must not race it by opening a second handle.
	h.devices = nil
	scanDeckDevicesWith(deps)
	h.devices = []*hid.DeviceInfo{deckTestInfo(0x5548,0x6670,"/dev/hidraw8","")}
	if got := scanDeckDevicesWith(deps); got != 0 { t.Fatalf("reconnect opened duplicate device: %d",got) }
	if h.initializeCalls != 1 { t.Fatalf("already-registered Deck reopened %d times",h.initializeCalls) }
}

func TestDeckHotplugFiltersWrongHIDInterfacesAndDisabledModels(t *testing.T) {
	h := newFakeDeckDiscovery()
	valid := deckTestInfo(0x6603,0x1014,"/dev/hidraw9","deck-one")
	wrongUsage := *valid
	wrongUsage.Path = "/dev/hidraw10"
	wrongUsage.UsagePage = 2
	wrongVendor := *valid
	wrongVendor.VendorID = 0xabcd
	wrongVendor.Path = "/dev/hidraw11"
	noPath := *valid
	noPath.Path = ""
	unknownPID := *valid
	unknownPID.ProductID = 0xffff
	unknownPID.Path = "/dev/hidraw12"
	h.devices = []*hid.DeviceInfo{&wrongUsage,&wrongVendor,&noPath,&unknownPID,valid,valid}
	deps := h.dependencies()
	if got := scanDeckDevicesWith(deps); got != 0 { t.Fatalf("disabled Deck registered: %d",got) }
	h.enabled[deck.DeviceKey(valid.VendorID,valid.ProductID)] = true
	if got := scanDeckDevicesWith(deps); got != 1 { t.Fatalf("valid Deck registered %d times",got) }
	if h.initializeCalls != 1 || h.registerCalls != 1 { t.Fatalf("wrong/duplicate HID interfaces were claimed: %+v",h) }
}

func TestDeckHotplugRetriesFailedOpenOnLaterScan(t *testing.T) {
	h := newFakeDeckDiscovery()
	h.enabled[deck.DeviceKey(0x5548,0x6670)] = true
	h.devices = []*hid.DeviceInfo{deckTestInfo(0x5548,0x6670,"/dev/hidraw9","")}
	deps := h.dependencies()
	h.failInitialization = true
	if got := scanDeckDevicesWith(deps); got != 0 { t.Fatalf("failed HID open registered a Deck: %d",got) }
	h.failInitialization = false
	if got := scanDeckDevicesWith(deps); got != 1 { t.Fatalf("retry did not register Deck: %d",got) }
	if h.initializeCalls != 2 || h.registerCalls != 1 {
		t.Fatalf("retry counts: init=%d register=%d",h.initializeCalls,h.registerCalls)
	}
}

func TestDeckHotplugUsesVendorQualifiedIdentityAndKeepsElgatoDiscoveryOnly(t *testing.T) {
	h := newFakeDeckDiscovery()
	h.enabled[deck.DeviceKey(0x5548,0x6670)] = true
	h.enabled[deck.DeviceKey(0x0fd9,0x0080)] = true
	h.devices = []*hid.DeviceInfo{
		deckTestInfo(0x5548,0x6670,"/dev/hidraw1","serial-a"),
		deckTestInfo(0x0fd9,0x0080,"/dev/hidraw2","serial-a"),
	}
	deps := h.dependencies()
	if got := scanDeckDevicesWith(deps); got != 2 { t.Fatalf("different vendors collided: %d devices",got) }
	if len(h.registered)!=2 {t.Fatalf("expected two distinct device identities: %d",len(h.registered))}
}

func TestDeckHotplugEnumerationErrorDoesNotRegisterOrStopFutureScans(t *testing.T) {
	h := newFakeDeckDiscovery()
	h.enabled[deck.DeviceKey(0x5548,0x6670)] = true
	h.devices = []*hid.DeviceInfo{deckTestInfo(0x5548,0x6670,"/dev/hidraw5","")}
	deps := h.dependencies()
	deps.enumerate = func(uint16,hid.EnumFunc)error{return errors.New("transient USB enumeration failure")}
	if got:=scanDeckDevicesWith(deps);got!=0{t.Fatalf("registered on failed scan: %d",got)}
	if got:=scanDeckDevicesWith(h.dependencies());got!=1{t.Fatalf("recovered scan missed device: %d",got)}
}

func TestDeckHotplugPollerStopsPromptlyAndNoLateScans(t *testing.T) {
	ctx,cancel:=context.WithCancel(context.Background())
	defer cancel()
	var scans atomic.Int32
	started:=make(chan struct{})
	var once sync.Once
	done:=make(chan struct{})
	go func(){
		defer close(done)
		runDeckDiscoveryPoller(ctx,5*time.Millisecond,func(){
			scans.Add(1)
			once.Do(func(){close(started)})
		})
	}()
	select{case <-started:case <-time.After(time.Second):t.Fatal("poller never scanned")}
	cancel()
	select{case <-done:case <-time.After(time.Second):t.Fatal("poller ignored shutdown")}
	before:=scans.Load()
	time.Sleep(20*time.Millisecond)
	if after:=scans.Load();after!=before{t.Fatalf("scanner ran after cancellation: %d to %d",before,after)}
}

func TestDeckRegistrySnapshotsAreIndependentDuringRegistration(t *testing.T) {
	serial:=fmt.Sprintf("deck-registry-test-%d",time.Now().UnixNano())
	defer deleteDevice(serial)
	const readers=5
	var wg sync.WaitGroup
	for i:=0;i<readers;i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()
			for j:=0;j<100;j++{
				_ = GetDevices()
				_ = GetDevice(serial)
			}
		}()
	}
	addDevice(&common.Device{Serial:serial,Product:"Test Deck",Instance:struct{}{}})
	snapshot:=GetDevices()
	if snapshot[serial]==nil{t.Fatal("new Deck not returned by live registry")}
	delete(snapshot,serial)
	if GetDevice(serial)==nil{t.Fatal("caller mutated live registry through snapshot")}
	wg.Wait()
}

func TestDeckHotplugShutdownWaitsForInflightEnumeration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runDeckDiscoveryPoller(ctx, time.Millisecond, func() {
			close(entered)
			<-release
		})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("scanner never entered enumeration")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("poller exited before its in-flight HID scan finished")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not wait for in-flight scan")
	}
}
