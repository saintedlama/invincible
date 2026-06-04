package supervisor

import (
	"fmt"
	"testing"
)

func TestRingBuffer_Empty(t *testing.T) {
	var r ringBuffer
	if got := r.entries(5); len(got) != 0 {
		t.Errorf("empty buffer: got %d entries, want 0", len(got))
	}
}

func TestRingBuffer_BasicReadWrite(t *testing.T) {
	var r ringBuffer
	r.write("a", "stdout")
	r.write("b", "stderr")
	r.write("c", "stdout")

	got := r.entries(3)
	cases := []struct {
		line   string
		source string
	}{
		{"a", "stdout"},
		{"b", "stderr"},
		{"c", "stdout"},
	}
	for i, c := range cases {
		if got[i].Line != c.line || got[i].Source != c.source {
			t.Errorf("[%d]: got {%q %v}, want {%q %v}", i, got[i].Line, got[i].Source, c.line, c.source)
		}
	}
}

func TestRingBuffer_RequestMoreThanSize(t *testing.T) {
	var r ringBuffer
	r.write("only", "stdout")
	got := r.entries(100)
	if len(got) != 1 || got[0].Line != "only" {
		t.Errorf("got %v, want [{\"only\" \"stdout\"}]", got)
	}
}

func TestRingBuffer_LastN(t *testing.T) {
	var r ringBuffer
	for i := 0; i < 5; i++ {
		r.write(fmt.Sprintf("line%d", i), "stdout")
	}
	got := r.entries(3)
	want := []string{"line2", "line3", "line4"}
	for i, w := range want {
		if got[i].Line != w {
			t.Errorf("[%d]: got %q, want %q", i, got[i].Line, w)
		}
	}
}

func TestRingBuffer_Wraparound(t *testing.T) {
	var r ringBuffer
	total := ringSize + 50
	for i := 0; i < total; i++ {
		r.write(fmt.Sprintf("line%d", i), "stdout")
	}
	got := r.entries(ringSize)
	if len(got) != ringSize {
		t.Fatalf("got %d entries, want %d", len(got), ringSize)
	}
	wantFirst := fmt.Sprintf("line%d", total-ringSize)
	if got[0].Line != wantFirst {
		t.Errorf("first line: got %q, want %q", got[0].Line, wantFirst)
	}
	wantLast := fmt.Sprintf("line%d", total-1)
	if got[ringSize-1].Line != wantLast {
		t.Errorf("last line: got %q, want %q", got[ringSize-1].Line, wantLast)
	}
}
