package ports

import (
	"net"
	"testing"
)

func TestIsFree_OccupiedPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if IsFree(port) {
		t.Errorf("port %d is occupied but IsFree returned true", port)
	}
}

func TestIsFree_FreePort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	if !IsFree(port) {
		t.Errorf("port %d should be free after closing, but IsFree returned false", port)
	}
}

func TestFindFree_NoHint(t *testing.T) {
	port, err := FindFree(0)
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("FindFree(0) returned invalid port %d", port)
	}
}

func TestFindFree_WithHint_BusyPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	busyPort := l.Addr().(*net.TCPAddr).Port

	port, err := FindFree(busyPort)
	if err != nil {
		t.Fatal(err)
	}
	if port == busyPort {
		t.Errorf("FindFree returned the busy port %d", busyPort)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("FindFree returned invalid port %d", port)
	}
}

func TestProbePort_NoListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	if ProbePort(port) {
		t.Errorf("ProbePort(%d) = true, want false (port is free)", port)
	}
}

func TestProbePort_HasListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if !ProbePort(port) {
		t.Errorf("ProbePort(%d) = false, want true (port is occupied)", port)
	}
}

func TestFindFree_WithHint_FreePort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	port, err := FindFree(freePort)
	if err != nil {
		t.Fatal(err)
	}
	if port != freePort {
		t.Errorf("FindFree(%d) = %d, want %d (port was free)", freePort, port, freePort)
	}
}
