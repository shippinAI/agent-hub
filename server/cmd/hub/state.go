package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// claimState remembers fence/version per held task so agents don't copy them by hand.
type claimState map[string]struct {
	Fence   int64 `json:"fence"`
	Version int64 `json:"version"`
}

func statePath() string {
	if p := os.Getenv("HUB_STATE"); p != "" {
		return expand(p)
	}
	d, _ := os.UserCacheDir()
	return filepath.Join(d, "hub", "claims.json")
}

func loadState() claimState {
	s := claimState{}
	if b, err := os.ReadFile(statePath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func (s claimState) save() {
	p := statePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	b, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(p, b, 0o600)
}

func (s claimState) put(id, fence, version int64) {
	e := s[strconv.FormatInt(id, 10)]
	e.Fence, e.Version = fence, version
	s[strconv.FormatInt(id, 10)] = e
	s.save()
}

func (s claimState) drop(id string) { delete(s, id); s.save() }

func (s claimState) fence(id string) int64 { return s[id].Fence }
