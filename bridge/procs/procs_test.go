package procs

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var prompts = NewPrompts([]string{`>>> $`, `\$ $`})

func TestPipeSessionAndReads(t *testing.T) {
	m := NewManager(1<<20, time.Minute, prompts)
	s, err := m.Start(Spec{Cmd: exec.Command("sh", "-c", "echo one; echo two; read x; echo got $x"), Command: "t"})
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Wait(context.Background(), 0, 2*time.Second, prompts, false, true)
	if !strings.Contains(snap.Output, "two") || snap.State != StateRunning {
		t.Fatalf("%+v %q", snap, snap.Output)
	}
	r := s.ReadLines(0, 10)
	if r.Returned != 2 || r.Text != "one\ntwo\n" {
		t.Fatalf("%+v", r)
	}
	seq := s.Output().Seq()
	_ = s.Write("hi", false)
	snap = s.Wait(context.Background(), seq, 2*time.Second, prompts, true, false)
	if snap.Reason != "exit" || !strings.Contains(snap.Output, "got hi") || *snap.ExitCode != 0 {
		t.Fatalf("%+v %q", snap, snap.Output)
	}
	if r := s.ReadLines(-1, 10); r.Text != "got hi\n" {
		t.Fatalf("tail %+v", r)
	}
	if l := m.List(); len(l) != 1 || l[0].State != StateFinished {
		t.Fatalf("%+v", l)
	}
}

func TestPTYPromptAndKill(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 missing")
	}
	m := NewManager(1<<20, time.Minute, prompts)
	s, err := m.Start(Spec{Cmd: exec.Command("python3", "-i", "-q"), PTY: true})
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Wait(context.Background(), 0, 5*time.Second, prompts, true, false)
	if snap.Reason != "prompt" || snap.State != StateWaiting {
		t.Fatalf("%+v %q", snap, snap.Output)
	}
	seq := s.Output().Seq()
	_ = s.Write("print(6*7)", false)
	snap = s.Wait(context.Background(), seq, 5*time.Second, prompts, true, false)
	if !strings.Contains(snap.Output, "42") {
		t.Fatalf("%q", snap.Output)
	}
	if err := m.Terminate(s.PID); err != nil || s.ExitCode() == nil {
		t.Fatal("terminate", err)
	}
}

func TestReapLeaseAndBuffer(t *testing.T) {
	m := NewManager(64, time.Hour, prompts)
	s, _ := m.Start(Spec{Cmd: exec.Command("sleep", "30"), TTL: time.Millisecond})
	time.Sleep(5 * time.Millisecond)
	if k := m.Reap(time.Now()); len(k) != 1 || s.ExitCode() == nil {
		t.Fatal("lease not reaped", k)
	}
	b := NewBuffer(16)
	_, _ = b.Write([]byte("aaaa\nbbbb\ncccc\ndddd\n"))
	l, first := b.Lines()
	if first == 0 || len(l) == 0 || l[len(l)-1] != "dddd\n" {
		t.Fatal(l, first)
	}
	if _, _, lost := b.Since(0); !lost {
		t.Fatal("lost not flagged")
	}
	if ps, err := ListAll(); err != nil || len(ps) == 0 {
		t.Fatal(err)
	}
	if KillPID(1, true) == nil {
		t.Fatal("pid 1 kill allowed")
	}
}
