// Package procs runs long-lived process sessions (DC start_process / interact_with_process / read_process_output,
// and the hub's proc leases): merged output in a bounded buffer, prompt detection, PTY on Unix and ConPTY on
// Windows, honest timeouts, and an idle/lease reaper that kills the whole process tree.
package procs

import (
	"bytes"
	"sync"
)

// Buffer keeps the newest max bytes of output, addressable by byte sequence and by absolute line number.
type Buffer struct {
	mu        sync.Mutex
	data      []byte
	base      int64 // byte seq of data[0]
	dropLines int   // whole lines dropped from the front
	max       int
	notify    chan struct{}
}

// NewBuffer makes a buffer that retains at most max bytes.
func NewBuffer(max int) *Buffer { return &Buffer{max: max, notify: make(chan struct{})} }

// Write appends output and wakes waiters. It trims the front at a line boundary when over max.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	b.data = append(b.data, p...)
	if over := len(b.data) - b.max; over > 0 {
		cut := over
		if i := bytes.IndexByte(b.data[over:], '\n'); i >= 0 && i < 4096 {
			cut = over + i + 1
		}
		b.dropLines += bytes.Count(b.data[:cut], []byte("\n"))
		b.base += int64(cut)
		b.data = append([]byte(nil), b.data[cut:]...)
	}
	close(b.notify)
	b.notify = make(chan struct{})
	b.mu.Unlock()
	return len(p), nil
}

// Changed returns a channel closed at the next write.
func (b *Buffer) Changed() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.notify
}

// Seq is the byte sequence just past the newest output.
func (b *Buffer) Seq() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.base + int64(len(b.data))
}

// Since returns output after seq, the new seq, and whether older output was already dropped.
func (b *Buffer) Since(seq int64) (string, int64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	lost := seq < b.base
	if lost {
		seq = b.base
	}
	end := b.base + int64(len(b.data))
	if seq > end {
		seq = end
	}
	return string(b.data[seq-b.base:]), end, lost
}

// Lines returns all retained lines (the last may be partial) and the absolute number of the first.
func (b *Buffer) Lines() ([]string, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == 0 {
		return nil, b.dropLines
	}
	l := bytes.SplitAfter(b.data, []byte("\n"))
	if len(l[len(l)-1]) == 0 {
		l = l[:len(l)-1]
	}
	out := make([]string, len(l))
	for i, s := range l {
		out[i] = string(s)
	}
	return out, b.dropLines
}

// TailLine is the text after the last newline (where a prompt sits).
func (b *Buffer) TailLine() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := bytes.LastIndexByte(b.data, '\n')
	return string(b.data[i+1:])
}
