package runner

// capped is a writer that keeps at most max bytes and flags truncation.
type capped struct {
	buf       []byte
	max       int
	truncated bool
}

func (c *capped) Write(p []byte) (int, error) {
	if len(c.buf) < c.max {
		room := c.max - len(c.buf)
		if room >= len(p) {
			c.buf = append(c.buf, p...)
		} else {
			c.buf = append(c.buf, p[:room]...)
			c.truncated = true
		}
	} else {
		c.truncated = true
	}
	return len(p), nil
}

func (c *capped) String() string { return string(c.buf) }
