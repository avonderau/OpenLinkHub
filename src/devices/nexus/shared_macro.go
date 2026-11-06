package nexus

import (
	"OpenLinkHub/src/inputmanager"
	"OpenLinkHub/src/macro"
	"context"
	"strconv"
	"time"
)

// validSharedMacroID accepts only bounded positive numeric IDs from the
// existing OpenLinkHub macro library. No freeform commands are dispatched.
func validSharedMacroID(value string) bool {
	if value == "" || len(value) > 9 || value[0] == '0' {
		return false
	}
	id, err := strconv.Atoi(value)
	return err == nil && id > 0 && id <= 999999999 && strconv.Itoa(id) == value
}

const nexusSharedMacroMaxDuration = 15 * time.Second
const nexusSharedMacroMaxActions = 256

// validateSharedMacro rejects non-terminating or excessive profiles, rather
// than silently running an indefinite loop from a touch-screen button.
func validateSharedMacro(m macro.Macro) bool {
	if m.Repeat < 0 || m.Repeat > 10 || m.RepeatDelay < 0 || m.RepeatDelay > 5000 {
		return false
	}
	if len(m.Actions) == 0 || len(m.Actions) > 64 {
		return false
	}
	count := len(m.Actions)
	if m.Repeat > 1 {
		count *= m.Repeat
	}
	if count > nexusSharedMacroMaxActions {
		return false
	}
	for i := 0; i < len(m.Actions); i++ {
		a, ok := m.Actions[i]
		if !ok || a.ActionRepeat > 20 || a.ActionRepeatDelay > 5000 || a.ActionDelay > 5000 {
			return false
		}
		switch a.ActionType {
		case 1, 3, 9:
		case 5:
		case 6:
			if len(a.ActionText) > 80 {
				return false
			}
		case 20:
		default:
			return false
		}
	}
	return true
}

// runSavedMacro executes an immutable snapshot. No arbitrary shell commands or
// input from outside the already-configured shared macro library are accepted.
func runSavedMacro(ctx context.Context, m macro.Macro) bool {
	if !validateSharedMacro(m) {
		return false
	}
	cycles := m.Repeat
	if cycles == 0 {
		cycles = 1
	}
	for cycle := 0; cycle < cycles; cycle++ {
		for i := 0; i < len(m.Actions); i++ {
			if ctx.Err() != nil {
				return false
			}
			action := m.Actions[i]
			if action.ActionType == 5 {
				if !waitNexusMacro(ctx, time.Duration(action.ActionDelay)*time.Millisecond) {
					return false
				}
				continue
			}
			repeats := int(action.ActionRepeat)
			if repeats == 0 {
				repeats = 1
			}
			for j := 0; j < repeats; j++ {
				if ctx.Err() != nil {
					return false
				}
				switch action.ActionType {
				case 1, 3:
					if action.ActionHold {
						inputmanager.InputControlKeyboardHold(action.ActionCommand, true)
						inputmanager.InputControlKeyboardHold(action.ActionCommand, false)
					} else {
						inputmanager.InputControlKeyboard(action.ActionCommand, false)
					}
				case 9:
					if action.ActionHold {
						inputmanager.InputControlMouseHold(action.ActionCommand, true)
						inputmanager.InputControlMouseHold(action.ActionCommand, false)
					} else {
						inputmanager.InputControlMouse(action.ActionCommand)
					}
				case 6:
					inputmanager.InputControlKeyboardText(action.ActionText)
				case 20:
					if action.MousePositionAbsolute {
						inputmanager.InputControlMoveAbsolute(int32(action.MousePositionX), int32(action.MousePositionY))
					} else {
						inputmanager.InputControlMove(int32(action.MousePositionX), int32(action.MousePositionY))
					}
				}
				if j+1 < repeats && !waitNexusMacro(ctx, time.Duration(action.ActionRepeatDelay)*time.Millisecond) {
					return false
				}
			}
		}
		if cycle+1 < cycles && !waitNexusMacro(ctx, time.Duration(m.RepeatDelay)*time.Millisecond) {
			return false
		}
	}
	return true
}

func waitNexusMacro(ctx context.Context, delay time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (d *Device) runSharedMacro(target string) {
	if !validSharedMacroID(target) {
		return
	}
	id, _ := strconv.Atoi(target)
	profile, ok := macro.SnapshotProfile(id)
	if !ok || !validateSharedMacro(profile) {
		return
	}
	d.mutex.Lock()
	if d.macroRunning {
		d.mutex.Unlock()
		return
	}
	d.macroRunning = true
	d.mutex.Unlock()
	go func() {
		defer func() { d.mutex.Lock(); d.macroRunning = false; d.mutex.Unlock() }()
		ctx, cancel := context.WithTimeout(context.Background(), nexusSharedMacroMaxDuration)
		defer cancel()
		runSavedMacro(ctx, profile)
	}()
}
