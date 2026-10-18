package mcpserve

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TokenFile is the bearer token for HTTP mode, created 0600 in the config dir. It is never printed.
const TokenFile = "http-token"

// Token loads or creates the HTTP bearer token.
func Token(dir string) (string, error) {
	p := filepath.Join(dir, TokenFile)
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return tok, os.WriteFile(p, []byte(tok+"\n"), 0o600)
}

// Handler is the streamable HTTP endpoint (mounted at /mcp) behind bearer auth.
// The SDK adds DNS-rebinding protection for loopback addresses.
func (s *Server) Handler(token string) http.Handler {
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.MCP }, &mcp.StreamableHTTPOptions{SessionTimeout: 2 * time.Hour})
	mux := http.NewServeMux()
	mux.Handle("/mcp", auth(token, h))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	return mux
}

func auth(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="devagotchi"`)
			http.Error(w, "unauthorized: send Authorization: Bearer <contents of the http-token file>", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RunHTTP listens on addr until ctx ends. ready (optional) receives the bound address.
func (s *Server) RunHTTP(ctx context.Context, addr, token string, ready func(string)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if ready != nil {
		ready(ln.Addr().String())
	}
	srv := &http.Server{Handler: s.Handler(token), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sc, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		srv.Shutdown(sc)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
