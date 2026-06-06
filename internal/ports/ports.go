package ports

import (
	"fmt"
	"net"
	"time"
)

// IsFree returns true if nothing is accepting connections on the given port.
// Uses dial rather than bind so it works correctly on Windows, where binding
// 127.0.0.1:X can succeed even when something is already listening on 0.0.0.0:X.
func IsFree(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 50*time.Millisecond)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

// FindFree returns a free TCP port. If hint is 0, the OS assigns an arbitrary
// free port. If hint is > 0, it scans upward from hint until a free port is found.
func FindFree(hint int) (int, error) {
	if hint == 0 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, fmt.Errorf("finding free port: %w", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return port, nil
	}
	for p := hint; p < 65536; p++ {
		if IsFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port found starting from %d", hint)
}

// ProbePort reports whether a TCP port on localhost is accepting connections.
func ProbePort(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
