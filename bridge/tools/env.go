package tools

import (
	"os"
	"time"

	"devagotchi/bridge/history"
	"devagotchi/bridge/policy"
	"devagotchi/bridge/procs"
	"devagotchi/bridge/redact"
	"devagotchi/bridge/search"
)

// NewEnv wires managers from the gate's policy limits. hist may be nil (history off).
// Call Close when done; the process reaper runs until then.
func NewEnv(g *policy.Gate, mode, self string, hist *history.Log, log func(string)) *Env {
	l := g.Limits()
	if log == nil {
		log = func(string) {}
	}
	e := &Env{Gate: g, Mode: mode, Self: self, History: hist, Log: log, Started: time.Now(),
		Redact:   redact.New(g.P.SecretPatterns),
		Procs:    procs.NewManager(l.OutputBufferBytes, time.Duration(l.SessionIdleSeconds)*time.Second, procs.NewPrompts(g.P.PromptPatterns)),
		Searches: search.NewManager(), stop: make(chan struct{})}
	go e.Procs.Run(10*time.Second, e.stop)
	return e
}

// Close kills every session this env started and stops the reaper.
func (e *Env) Close() {
	select {
	case <-e.stop:
	default:
		close(e.stop)
	}
	e.Procs.KillAll()
}

func lookupEnv(k string) (string, bool) { return os.LookupEnv(k) }
