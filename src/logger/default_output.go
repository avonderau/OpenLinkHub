package logger

import (
	"log"
	"os"
)

// Fork-owned: give the package a stderr logger before Init runs. Without it,
// any package that logs before controller.Start calls logger.Init (including
// unit tests) dereferences a nil *log.Logger and panics. Init still replaces
// this with the configured log file.
func init() {
	if logger == nil {
		logger = log.New(os.Stderr, "", 0)
	}
}
