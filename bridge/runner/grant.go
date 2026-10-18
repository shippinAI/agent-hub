package runner

import (
	"context"
	"devagotchi/bridge/wire"
	"strings"
	"time"
)

// grantLive checks the grant cache, refetching from the hub when older than grant_cache_seconds.
func (r *Runner) grantLive(ctx context.Context, f wire.Frame) bool {
	if f.GrantID == "" || f.GrantID == "probe" || f.GrantID == "operator" {
		return true
	}
	r.mu.Lock()
	e, ok := r.cache[f.GrantID]
	r.mu.Unlock()
	if ok && time.Since(e.at) < time.Duration(r.Cfg.CacheSeconds)*time.Second {
		return e.live && e.version >= f.GrantVersion
	}
	live, ver, err := r.Client.GrantLive(ctx, f.GrantID)
	if err != nil {
		return ok && e.live
	}
	r.mu.Lock()
	r.cache[f.GrantID] = cacheEntry{live: live, version: ver, at: time.Now()}
	r.mu.Unlock()
	return live && ver >= f.GrantVersion
}

func (r *Runner) cancelGrant(id string) {
	r.mu.Lock()
	r.cache[id] = cacheEntry{live: false, at: time.Now()}
	for cid, cancel := range r.running {
		if strings.HasPrefix(cid, id+"|") {
			cancel()
		}
	}
	r.mu.Unlock()
}

func (r *Runner) cancelCall(id string) {
	r.mu.Lock()
	if cancel := r.running[id]; cancel != nil {
		cancel()
	}
	r.mu.Unlock()
}
