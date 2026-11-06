package deck

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sstallion/go-hid"
)

// interruptingSession returns the error hidapi produces when a signal
// interrupts poll(), then reports idle timeouts.
type interruptingSession struct {
	fakeDeckSession
	mu    sync.Mutex
	reads int
	err   error
}

func (s *interruptingSession) ReadWithTimeout(_ []byte, _ time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if s.reads <= 3 {
		return -1, s.err
	}
	time.Sleep(time.Millisecond)
	return 0, hid.ErrTimeout
}

func (s *interruptingSession) readCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

func runListenUntil(t *testing.T, d *Device, h *interruptingSession, minReads int) {
	t.Helper()
	d.readerDone = make(chan struct{})
	go d.listen()
	deadline := time.Now().Add(2 * time.Second)
	for h.readCount() < minReads && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(d.done)
	<-d.readerDone
}

func TestListenKeepsSessionOnInterruptedRead(t *testing.T) {
	d := testDevice(t)
	h := &interruptingSession{err: errors.New("hid: Interrupted system call")}
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	runListenUntil(t, d, h, 6)
	if !d.GetStatus().Connected || h.closed {
		t.Fatalf("interrupted read dropped the session: connected=%v closed=%v", d.GetStatus().Connected, h.closed)
	}
}

func TestListenDropsSessionOnDisconnectError(t *testing.T) {
	d := testDevice(t)
	h := &interruptingSession{err: errors.New("hid: hid_read_timeout: unexpected poll error (device disconnected)")}
	if err := d.attachSession(h); err != nil {
		t.Fatal(err)
	}
	runListenUntil(t, d, h, 1)
	if d.GetStatus().Connected || !h.closed {
		t.Fatal("real disconnect did not drop the session")
	}
}

func TestIsInterruptedRead(t *testing.T) {
	if !isInterruptedRead(errors.New("Interrupted system call")) {
		t.Fatal("strerror(EINTR) not recognized")
	}
	if isInterruptedRead(nil) || isInterruptedRead(hid.ErrTimeout) || isInterruptedRead(errors.New("No such device")) {
		t.Fatal("non-EINTR error treated as interrupted")
	}
}
