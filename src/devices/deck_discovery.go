package devices

import (
	"context"
	"sync"
	"time"

	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices/deck"
	"OpenLinkHub/src/logger"
	"github.com/sstallion/go-hid"
)

// Deck discovery uses the existing Linux USB Netlink monitor. A worker
// enumerates a specific VID/PID only after a matching USB-add event.
// Registered Decks continue to use their own HID reconnect loop.
const (
	deckEventQueueSize = 32
	deckEventRetryDelay = 450 * time.Millisecond
	deckEventRetryCount = 4
)

type deckScanDependencies struct {
	enumerate   func(uint16, uint16, hid.EnumFunc) error
	initialize  func(*hid.DeviceInfo) *common.Device
	enabled     func(string) bool
	present     func(string) bool
	register    func(*common.Device)
	reportError func(uint16, error)
}

func liveDeckScanDependencies() deckScanDependencies {
	return deckScanDependencies{
		enumerate: func(vendor, product uint16, visit hid.EnumFunc) error {
			return hid.Enumerate(vendor, product, visit)
		},
		initialize: deck.Init,
		enabled: config.DeckEnabled,
		present: func(serial string) bool { return GetDevice(serial) != nil },
		register: addDevice,
		reportError: func(vendor uint16, err error) {
			logger.Log(logger.Fields{"vendorId": vendor, "error": err}).
				Warn("Unable to enumerate Deck devices")
		},
	}
}

// scanDeckVendorWith performs a filtered HID enumeration. The boolean is
// true when the target is already registered or all matching interfaces
// were initialized; false asks the event worker to retry briefly while
// udev creates the HID command interface.
func scanDeckVendorWith(deps deckScanDependencies, vendor, product uint16) (int, bool) {
	seen := make(map[string]bool)
	added := 0
	matched := false
	needsRetry := false

	err := deps.enumerate(vendor, product, hid.EnumFunc(func(info *hid.DeviceInfo) error {
		if info == nil || info.Path == "" { return nil }
		if info.VendorID != vendor || (product != hid.ProductIDAny && info.ProductID != product) { return nil }
		model, supported := deck.LookupModel(info.VendorID, info.ProductID)
		if !supported { return nil }
		if !deps.enabled(deck.DeviceKey(info.VendorID, info.ProductID)) { return nil }
		// Mirabox has multiple interfaces; never claim anything other
		// than the command interface used by the existing driver.
		if model.Family == "mirabox" && (info.UsagePage != 0xffa0 || info.Usage != 1) {
			return nil
		}
		serial := deck.CandidateDeviceID(info, model)
		if serial == "" { return nil }
		matched = true
		if seen[serial] || deps.present(serial) {
			seen[serial] = true
			return nil
		}
		device := deps.initialize(info)
		if device == nil {
			needsRetry = true
			return nil
		}
		if device.Serial != serial {
			// Do not register an incorrectly identified handle.
			needsRetry = true
			return nil
		}
		deps.register(device)
		seen[serial] = true
		added++
		return nil
	}))
	if err != nil {
		if deps.reportError != nil { deps.reportError(vendor, err) }
		return added, false
	}
	return added, matched && !needsRetry
}

// Startup and the one-time monitor-armed reconciliation enumerate only
// supported vendors. Both use exactly the same registry and HID rules.
func scanDeckDevicesWith(deps deckScanDependencies) int {
	added := 0
	for _, vendor := range deck.SupportedVendors() {
		count, _ := scanDeckVendorWith(deps, vendor, hid.ProductIDAny)
		added += count
	}
	return added
}

func initializeDeckDevices() {
	scanDeckDevicesWith(liveDeckScanDependencies())
}

type deckUSBEvent struct {
	vendor uint16
	product uint16
	reconcile bool
}

// runDeckUSBEvents has no ticker and never wakes to scan idle hardware.
// On an add event, HID can lag behind the USB notification, so retry a
// bounded number of times. Context cancellation interrupts all delays;
// a currently running HID scan is allowed to finish before shutdown.
func runDeckUSBEvents(
	ctx context.Context, events <-chan deckUSBEvent, retryDelay time.Duration,
	attempts int, scan func(uint16, uint16) (int, bool), reconcile func(),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if ctx.Err() != nil { return }
			if event.reconcile {
				reconcile()
				continue
			}
			for attempt := 0; attempt < attempts; attempt++ {
				timer := time.NewTimer(retryDelay)
				select {
				case <-ctx.Done():
					if !timer.Stop() { <-timer.C }
					return
				case <-timer.C:
				}
				if ctx.Err() != nil { return }
				_, settled := scan(event.vendor, event.product)
				if settled { break }
			}
		}
	}
}

var deckDiscoveryState struct {
	sync.Mutex
	cancel context.CancelFunc
	done chan struct{}
	events chan deckUSBEvent
}

func startDeckDiscoveryEvents() {
	deckDiscoveryState.Lock()
	defer deckDiscoveryState.Unlock()
	if deckDiscoveryState.cancel != nil { return }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	events := make(chan deckUSBEvent, deckEventQueueSize)
	deckDiscoveryState.cancel = cancel
	deckDiscoveryState.done = done
	deckDiscoveryState.events = events

	go func() {
		defer close(done)
		deps := liveDeckScanDependencies()
		runDeckUSBEvents(ctx, events, deckEventRetryDelay, deckEventRetryCount,
			func(vendor, product uint16) (int, bool) {
				if !config.DeckEnabled(deck.DeviceKey(vendor, product)) { return 0, true }
				added, settled := scanDeckVendorWith(deps, vendor, product)
				if added > 0 {
					logger.Log(logger.Fields{"vendorId": vendor, "productId": product, "newDevices": added}).
						Info("USB event: Deck discovered without restarting OpenLinkHub")
				}
				return added, settled
			},
			func() {
				if added := scanDeckDevicesWith(deps); added > 0 {
					logger.Log(logger.Fields{"newDevices": added}).
						Info("USB monitor ready: reconciled newly attached Decks")
				}
			},
		)
	}()
}

// NotifyDeckUSBAdded is intentionally fast and nonblocking for the existing
// kernel Netlink USB monitor, including when a HID scan is in progress.
func NotifyDeckUSBAdded(vendor, product uint16) {
	if _, ok := deck.LookupModel(vendor, product); !ok { return }
	deckDiscoveryState.Lock()
	defer deckDiscoveryState.Unlock()
	if deckDiscoveryState.cancel == nil { return }
	event := deckUSBEvent{vendor: vendor, product: product}
	select {
	case deckDiscoveryState.events <- event:
	default:
		// Bounded queue: do not stall the upstream USB event monitor.
		logger.Log(logger.Fields{"vendorId": vendor, "productId": product}).
			Warn("Deck USB event queue full; event dropped")
	}
}

// NotifyDeckMonitorReady schedules a single reconciliation AFTER Netlink's
// socket has bound. This closes the startup gap between initial device
// detection and the monitor being able to receive subsequent events.
func NotifyDeckMonitorReady() {
	deckDiscoveryState.Lock()
	defer deckDiscoveryState.Unlock()
	if deckDiscoveryState.cancel == nil { return }
	select {
	case deckDiscoveryState.events <- deckUSBEvent{reconcile: true}:
	default:
		logger.Log(logger.Fields{}).Warn("Deck USB event queue full; startup reconciliation deferred")
	}
}

func stopDeckDiscoveryEvents() {
	deckDiscoveryState.Lock()
	cancel, done := deckDiscoveryState.cancel, deckDiscoveryState.done
	deckDiscoveryState.cancel, deckDiscoveryState.done, deckDiscoveryState.events = nil, nil, nil
	deckDiscoveryState.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
