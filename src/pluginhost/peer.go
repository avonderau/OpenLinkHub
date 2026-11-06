package pluginhost

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// The plugin WebSocket listens on 127.0.0.1, which every local user can
// reach, and the registration token travels on the plugin's command line
// (the Stream Deck SDK requires it), where other users can read it. So a
// connection is accepted only from a socket owned by the agent's own user,
// looked up in the kernel's TCP table.

// procNetTCP is replaced in tests.
var procNetTCP = []string{"/proc/net/tcp", "/proc/net/tcp6"}

// loopbackPeerUID returns the owner of the local socket at remote that is
// connected to local.
func loopbackPeerUID(remote, local string) (int, error) {
	rport, err := portOf(remote)
	if err != nil {
		return -1, err
	}
	lport, err := portOf(local)
	if err != nil {
		return -1, err
	}
	want := fmt.Sprintf("%04X", rport)
	peer := fmt.Sprintf("%04X", lport)
	for _, path := range procNetTCP {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			// sl local_address rem_address st ... uid (index 7)
			if len(fields) < 8 {
				continue
			}
			_, lp, ok1 := strings.Cut(fields[1], ":")
			_, rp, ok2 := strings.Cut(fields[2], ":")
			if !ok1 || !ok2 || lp != want || rp != peer {
				continue
			}
			uid, err := strconv.Atoi(fields[7])
			f.Close()
			if err != nil {
				return -1, err
			}
			return uid, nil
		}
		f.Close()
	}
	return -1, fmt.Errorf("no socket found for %s", remote)
}

func portOf(addr string) (int, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(port)
}
