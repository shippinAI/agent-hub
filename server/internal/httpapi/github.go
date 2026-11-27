package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

// githubWebhook takes GitHub deliveries (push, pull_request, check_run). It carries no bearer token: the
// X-Hub-Signature-256 HMAC over the raw body, keyed by HUB_GITHUB_WEBHOOK_SECRET_FILE, is the authentication.
// With no secret configured the endpoint does not exist.
func (s *Server) githubWebhook(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.GitHubSecret == "" {
		js(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "GitHub webhook is not configured"}})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		js(w, 413, map[string]any{"error": map[string]string{"code": "too_large", "message": "body over 5 MiB"}})
		return
	}
	if !validSignature(s.Cfg.GitHubSecret, r.Header.Get("X-Hub-Signature-256"), body) {
		js(w, 401, map[string]any{"error": map[string]string{"code": "bad_signature", "message": "X-Hub-Signature-256 does not match"}})
		return
	}
	delivery, event := r.Header.Get("X-GitHub-Delivery"), r.Header.Get("X-GitHub-Event")
	if delivery == "" || event == "" || len(delivery) > 200 {
		js(w, 400, map[string]any{"error": map[string]string{"code": "invalid", "message": "missing X-GitHub-Delivery or X-GitHub-Event"}})
		return
	}
	out, err := s.St.IngestGitHub(r.Context(), delivery, event, body)
	if err != nil {
		writeErr(w, err)
		return
	}
	js(w, 200, out)
}

func validSignature(secret, header string, body []byte) bool {
	got, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	want, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(m.Sum(nil), want)
}
