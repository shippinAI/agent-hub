package hubtest

import "testing"

func TestAuthTokensAndCSRF(t *testing.T) {
	e := setup(t)
	if r := e.call("", "GET", "/me", nil); r.Status != 401 {
		t.Errorf("no token: %d", r.Status)
	}
	e.tokens["bogus"] = "ahk_not-a-real-token"
	if r := e.call("bogus", "GET", "/me", nil); r.Status != 401 {
		t.Errorf("bad token: %d", r.Status)
	}
	tr := e.call("jesse", "POST", "/principals/cursor/tokens", map[string]any{"name": "temp"})
	e.tokens["cursor2"] = tr.Body["token"].(string)
	if r := e.call("cursor2", "GET", "/me", nil); r.Body["principal"].(map[string]any)["id"] != "cursor" {
		t.Fatalf("new token: %v", r.Body)
	}
	tid := tr.Body["info"].(map[string]any)["id"].(string)
	if r := e.call("jesse", "DELETE", "/tokens/"+tid, nil); r.Status != 200 {
		t.Fatalf("revoke: %v", r.Body)
	}
	if r := e.call("cursor2", "GET", "/me", nil); r.Status != 401 {
		t.Errorf("revoked token still works: %d", r.Status)
	}
	list := e.call("jesse", "GET", "/principals/cursor/tokens", nil).Body["list"].([]any)
	for _, x := range list {
		if _, leaked := x.(map[string]any)["token"]; leaked {
			t.Error("token list must not contain secrets")
		}
	}
	if r := e.call("codex", "GET", "/principals/cursor/tokens", nil); r.Status != 403 {
		t.Errorf("agent lists another agent's tokens: %d", r.Status)
	}
}
