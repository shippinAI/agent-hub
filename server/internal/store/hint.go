package store

// beatEvery is the heartbeat interval told to agents: the configured hint, but never more than TTL/3.
func beatEvery(st Settings, ttl int) int {
	h := st.Int("heartbeat_hint_seconds", 300)
	if ttl > 0 && h > ttl/3 {
		h = ttl / 3
	}
	if h < 1 {
		h = 1
	}
	return h
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
