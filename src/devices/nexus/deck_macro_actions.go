package nexus

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"OpenLinkHub/src/macro"
)

// ValidateDeckSavedMacroID accepts only existing numeric macro IDs.
func ValidateDeckSavedMacroID(id string) bool {
	return validSharedMacroID(id)
}

// RunDeckSavedMacro executes a bounded snapshot of a shared OpenLinkHub
// macro. The caller's context cancels execution when the device stops.
func RunDeckSavedMacro(ctx context.Context, id string) error {
	if !validSharedMacroID(id) {
		return errors.New("invalid saved macro ID")
	}
	if ctx == nil {
		return errors.New("missing macro cancellation context")
	}
	number, _ := strconv.Atoi(id)
	profile, exists := macro.SnapshotProfile(number)
	if !exists {
		return fmt.Errorf("saved macro %s not found", id)
	}
	if !validateSharedMacro(profile) {
		return fmt.Errorf("saved macro %s exceeds Nexus safety limits", id)
	}
	bound, cancel := context.WithTimeout(ctx, nexusSharedMacroMaxDuration)
	defer cancel()
	if !runSavedMacro(bound, profile) {
		if err := bound.Err(); err != nil {
			return fmt.Errorf("saved macro interrupted: %w", err)
		}
		return errors.New("saved macro action failed")
	}
	return nil
}
