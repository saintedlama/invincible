package supervisor

import (
	"sync"
	"time"
)

const ringSize = 500

// LogEntry holds a single log line, its source, and when it was written.
// Source is "stdout", "stderr", or "invincible".
type LogEntry struct {
	Time   time.Time `json:"time"`
	Line   string    `json:"line"`
	Source string    `json:"source"`
}

type ringBuffer struct {
	mu   sync.Mutex
	buf  [ringSize]LogEntry
	head int
	size int
}

func (r *ringBuffer) write(line string, source string) {
	r.mu.Lock()
	r.buf[r.head] = LogEntry{Time: time.Now(), Line: line, Source: source}
	r.head = (r.head + 1) % ringSize
	if r.size < ringSize {
		r.size++
	}
	r.mu.Unlock()
}

func (r *ringBuffer) entries(n int) []LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n > r.size {
		n = r.size
	}
	out := make([]LogEntry, n)
	start := (r.head - n + ringSize) % ringSize
	for i := 0; i < n; i++ {
		out[i] = r.buf[(start+i)%ringSize]
	}
	return out
}
