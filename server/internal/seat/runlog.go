package seat

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// uploader streams a run's transcript to the hub in ordered chunks from a goroutine, so a slow hub never
// stalls the agent's stdout. A nil uploader (the run could not be opened) drops everything.
type uploader struct {
	c         *Config
	runID     int64
	lines     chan []byte
	done      chan struct{}
	once      sync.Once
	truncated bool
}

const chunkSize = 128 << 10

func (c *Config) startRun(ctx context.Context, taskID, fence int64, branch string) *uploader {
	var r struct {
		ID int64 `json:"id"`
	}
	err := c.Hub.do(ctx, "POST", fmt.Sprintf("/tasks/%d/runs", taskID),
		map[string]any{"fence": fence, "adapter": c.Adapter, "seat": c.Name, "model": c.Model, "branch": branch}, &r)
	if err != nil {
		c.logf("seat: could not open a run for #%d: %v", taskID, err)
		return nil
	}
	u := &uploader{c: c, runID: r.ID, lines: make(chan []byte, 1024), done: make(chan struct{})}
	go u.loop(ctx)
	return u
}

func (u *uploader) line(b []byte) {
	if u == nil {
		return
	}
	l := make([]byte, len(b)+1)
	copy(l, b)
	l[len(b)] = '\n'
	u.lines <- l
}

func (u *uploader) loop(ctx context.Context) {
	defer close(u.done)
	var buf []byte
	seq := 0
	flush := func() {
		for len(buf) > 0 && !u.truncated {
			n := min(len(buf), chunkSize)
			err := u.c.Hub.do(ctx, "POST", fmt.Sprintf("/runs/%d/transcript", u.runID), map[string]any{"seq": seq, "body": string(buf[:n])}, nil)
			if err != nil {
				u.c.logf("seat: transcript of run %d stops at chunk %d: %v", u.runID, seq, err)
				u.truncated = true
				break
			}
			seq++
			buf = buf[n:]
		}
		buf = buf[:0]
	}
	tk := time.NewTicker(2 * time.Second)
	defer tk.Stop()
	for {
		select {
		case l, ok := <-u.lines:
			if !ok {
				flush()
				return
			}
			buf = append(buf, l...)
			if len(buf) >= chunkSize {
				flush()
			}
		case <-tk.C:
			flush()
		}
	}
}

// finish uploads what is left and closes the run with the outcome.
func (u *uploader) finish(ctx context.Context, o *Outcome, st *Stream) {
	if u == nil {
		return
	}
	u.once.Do(func() { close(u.lines) })
	<-u.done
	status := map[string]string{"completed": "completed", "handed_off": "handed_off", "lost": "lost"}[o.State]
	if status == "" {
		status = "failed"
	}
	in := map[string]any{"status": status, "reason": firstLine(o.Reason, 4000), "turns": st.Turns, "model": st.Model,
		"session_id": st.SessionID, "branch": o.Branch, "head_sha": o.Head, "truncated": u.truncated}
	if err := u.c.Hub.do(ctx, "POST", fmt.Sprintf("/runs/%d/finish", u.runID), in, nil); err != nil {
		u.c.logf("seat: could not finish run %d: %v", u.runID, err)
	}
}
