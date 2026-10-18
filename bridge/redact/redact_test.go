package redact

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	r := New([]string{"tok_"})
	out := r.String("key sk-abcdefghijklmnop and tok_1234567890 and ahk_zzzzzzzzzz ok")
	for _, bad := range []string{"abcdefghijklmnop", "1234567890", "zzzzzzzzzz"} {
		if strings.Contains(out, bad) {
			t.Fatalf("leaked %s: %s", bad, out)
		}
	}
	if !strings.Contains(out, "sk-[redacted]") || !strings.HasSuffix(out, " ok") {
		t.Fatalf("redaction shape: %s", out)
	}
}
