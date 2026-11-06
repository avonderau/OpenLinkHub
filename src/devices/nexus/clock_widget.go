package nexus

import (
	"fmt"
	"time"
)

// Nexus clock widgets share the host's local timezone and update on the
// normal display refresh cycle. Display formats are explicitly allowlisted.
func validNexusClockFormat(value string) bool {
	switch value {
	case "", "24h", "12h", "date", "datetime":
		return true
	default:
		return false
	}
}

func formatNexusClock(settings map[string]string, now time.Time) string {
	switch settings["clockFormat"] {
	case "12h":
		return now.Format("3:04 PM")
	case "date":
		return now.Format("Mon Jan 2")
	case "datetime":
		return now.Format("Jan 2 15:04")
	case "", "24h":
		return now.Format("15:04")
	default:
		return fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	}
}
