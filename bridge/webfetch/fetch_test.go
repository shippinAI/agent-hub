package webfetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redir" {
			http.Redirect(w, r, "http://evil.example/", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("a", 100)))
	}))
	defer srv.Close()
	allow := func(h string) error {
		if h == "127.0.0.1" {
			return nil
		}
		return errors.New("grant_required: " + h)
	}
	r := Rules{HostOK: allow, Named: func(string) bool { return false }, Timeout: 5 * time.Second, MaxBytes: 10}
	if _, err := Fetch(context.Background(), srv.URL, r); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatal("private address must need a named host", err)
	}
	r.Named = func(h string) bool { return h == "127.0.0.1" }
	p, err := Fetch(context.Background(), srv.URL, r)
	if err != nil || !p.Truncated || len(p.Body) != 10 {
		t.Fatal(err, p)
	}
	if _, err := Fetch(context.Background(), srv.URL+"/redir", r); err == nil {
		t.Fatal("redirect to an unlisted host allowed")
	}
	if _, err := Fetch(context.Background(), "file:///etc/passwd", r); err == nil {
		t.Fatal("file scheme allowed")
	}
}
