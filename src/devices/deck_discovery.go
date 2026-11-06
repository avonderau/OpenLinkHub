package devices

import (
	"context"
	"strconv"
	"sync"
	"time"

	"OpenLinkHub/src/common"
	"OpenLinkHub/src/config"
	"OpenLinkHub/src/devices/deck"
	"OpenLinkHub/src/logger"
	"github.com/sstallion/go-hid"
)

// A small periodic scan is sufficient for hot-plug discovery without adding a
// udev dependency or changing the upstream-owned Corsair enumeration path.
// Registered Mirabox devices already own their reconnect loops; discovery
// must never open a second HID handle for a registered device.
const deckDiscoveryInterval = 3 * time.Second

type deckScanDependencies struct {
	enumerate func(uint16, hid.EnumFunc) error
	initialize func(*hid.DeviceInfo) *common.Device
	enabled func(string) bool
	present func(string) bool
	register func(*common.Device)
}

func liveDeckScanDependencies() deckScanDependencies {
	return deckScanDependencies{
		enumerate: func(vendor uint16, visit hid.EnumFunc) error {
			return hid.Enumerate(vendor, hid.ProductIDAny, visit)
		},
		initialize: deck.Init,
		enabled: config.DeckEnabled,
		present: func(serial string) bool { return GetDevice(serial) != nil },
		register: addDevice,
	}
}

// scanDeckDevicesWith is injectable so that retry, duplication and
// multi-interface behavior can be tested without actual USB hardware.
func scanDeckDevicesWith(deps deckScanDependencies) int {
	seen := make(map[string]bool)
	added := 0
	for _, vendor := range deck.SupportedVendors() {
		err := deps.enumerate(vendor, hid.EnumFunc(func(info *hid.DeviceInfo) error {
			if info == nil || info.Path == "" { return nil }
			model, supported := deck.LookupModel(info.VendorID, info.ProductID)
			if !supported || info.VendorID != vendor { return nil }
			if !deps.enabled(deck.DeviceKey(info.VendorID, info.ProductID)) { return nil }
			// A Mirabox exposes multiple HID interfaces. Only the control
			// interface with the documented usage page/usage can be claimed.
			if model.Family == "mirabox" && (info.UsagePage != 0xffa0 || info.Usage != 1) {
				return nil
			}
			serial := deck.CandidateDeviceID(info, model)
			if serial == "" || seen[serial] || deps.present(serial) { return nil }

			device := deps.initialize(info)
			if device == nil { return nil } // Retry on the next scan.
			// The ID must match the driver ID so duplicate HID interfaces
			// and reconnect candidates can be recognized before opening.
			if device.Serial != serial { return nil }
			deps.register(device)
			seen[serial] = true
			added++
			return nil
		}))
		if err != nil {
			logger.Log(logger.Fields{"vendorId": vendor, "error": err}).
				Warn("Unable to enumerate Deck devices")
		}
	}
	return added
}

func initializeDeckDevices() {
	scanDeckDevicesWith(liveDeckScanDependencies())
}

var deckDiscoveryState struct {
	sync.Mutex
	cancel context.CancelFunc
	done chan struct{}
}

// runDeckDiscoveryPoller never enumerates after cancellation. The caller
// waits for it before exiting the HID library, including an in-flight scan.
func runDeckDiscoveryPoller(ctx context.Context, interval time.Duration, scan func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil { return }
			scan()
		}
	}
}

func startDeckDiscoveryLoop() {
	deckDiscoveryState.Lock()
	defer deckDiscoveryState.Unlock()
	if deckDiscoveryState.cancel != nil { return }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	deckDiscoveryState.cancel = cancel
	deckDiscoveryState.done = done
	go func() {
		defer close(done)
		runDeckDiscoveryPoller(ctx, deckDiscoveryInterval, func() {
			added := scanDeckDevicesWith(liveDeckScanDependencies())
			if added > 0 {
				logger.Log(logger.Fields{"newDevices": strconv.Itoa(added)}).
					Info("New Deck device discovered without restarting OpenLinkHub")
			}
		})
	}()
}

func stopDeckDiscoveryLoop() {
	deckDiscoveryState.Lock()
	cancel, done := deckDiscoveryState.cancel, deckDiscoveryState.done
	deckDiscoveryState.cancel, deckDiscoveryState.done = nil, nil
	deckDiscoveryState.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
