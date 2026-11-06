package common

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// ErrCommandOutputTooLarge reports a command whose stdout exceeded the
// configured maximum. Never retain excess output in the service process.
var ErrCommandOutputTooLarge = errors.New("command output exceeds limit")

type boundedCommandOutput struct {
	buffer bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (out *boundedCommandOutput) Write(data []byte) (int, error) {
	if len(data) > out.limit-out.buffer.Len() {
		out.cancel()
		return 0, ErrCommandOutputTooLarge
	}
	return out.buffer.Write(data)
}

// BoundedCommandOutput runs a trusted, fixed executable with explicit args
// (never through a shell). Both execution time and retained stdout are
// bounded. WaitDelay also protects against subprocesses holding pipes open.
func BoundedCommandOutput(timeout time.Duration, maxBytes int, executable string, args ...string) ([]byte, error) {
	if timeout <= 0 || maxBytes <= 0 || executable == "" {
		return nil, errors.New("invalid command execution limits")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	output := &boundedCommandOutput{limit: maxBytes, cancel: cancel}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Stdout = output
	cmd.WaitDelay = 250 * time.Millisecond
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return append([]byte(nil), output.buffer.Bytes()...), nil
}
