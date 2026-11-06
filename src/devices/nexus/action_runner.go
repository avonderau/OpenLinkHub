package nexus

import (
	"time"
)

// runNexusMacroSteps executes a validated macro sequentially. The cancellation
// channel is the Nexus LCD shutdown channel, so a pending delay cannot keep
// an action running after device shutdown. No global goroutines are created.
func runNexusMacroSteps(steps []MacroStep, done <-chan struct{}, execute func(string, string)) {
	for _, step := range steps {
		select {
		case <-done:
			return
		default:
		}
		if step.Type == "delay" {
			if step.DelayMs <= 0 {
				continue
			}
			timer := time.NewTimer(time.Duration(step.DelayMs) * time.Millisecond)
			select {
			case <-done:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
			}
			continue
		}
		execute(step.Type, step.Value)
	}
}
