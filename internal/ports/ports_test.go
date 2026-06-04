package ports

import (
	"net"
	"testing"
)

func TestEnvPair(t *testing.T) {
	tests := []struct {
		key, want string
		port      int
	}{
		{"PORT", "PORT=8080", 8080},
		{"API_PORT", "API_PORT=3000", 3000},
		{"X", "X=0", 0},
	}
	for _, tt := range tests {
		if got := EnvPair(tt.key, tt.port); got != tt.want {
			t.Errorf("EnvPair(%q, %d) = %q, want %q", tt.key, tt.port, got, tt.want)
		}
	}
}

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
