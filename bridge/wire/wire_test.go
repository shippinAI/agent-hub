package wire

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	k := SignKey("tok")
	f := Frame{V: 1, Type: "call", CallID: "c", GrantID: "g", GrantVersion: 2, Op: "exec", Args: json.RawMessage(`{"argv":["ls"]}`),
		Grant: &Grant{Capability: "exec", Level: "read-only"}, Deadline: time.Now().Add(time.Minute)}
	Sign(&f, k)
	if err := Verify(f, k, time.Now()); err != nil {
		t.Fatal(err)
	}
	g := f
	g.Args = json.RawMessage(`{"argv":["rm"]}`)
	if Verify(g, k, time.Now()) == nil {
		t.Fatal("tampered args verified")
	}
	g = f
	g.Grant = &Grant{Capability: "exec", Level: "unsandboxed"}
	if Verify(g, k, time.Now()) == nil {
		t.Fatal("widened grant verified")
	}
	if Verify(f, SignKey("other"), time.Now()) == nil || Verify(f, k, time.Now().Add(2*time.Minute)) == nil {
		t.Fatal("wrong key or late call verified")
	}
}
