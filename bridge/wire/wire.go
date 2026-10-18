// Package wire is the hub <-> bridge protocol (docs/devagotchi/03): frames and call signing. The hub
// imports it too, so both sides share one definition.
package wire

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Version is the frame version.
const Version = 1

// Frame is every message on the wire. Type is call, ack, chunk, result, heartbeat, cancel, revoke,
// gui_stop, ping, pong.
type Frame struct {
	V            int             `json:"v"`
	Type         string          `json:"type"`
	CallID       string          `json:"call_id,omitempty"`
	GrantID      string          `json:"grant_id,omitempty"`
	GrantVersion int64           `json:"grant_version,omitempty"`
	Fence        int64           `json:"fence,omitempty"`
	Op           string          `json:"op,omitempty"`
	Args         json.RawMessage `json:"args,omitempty"`
	Grant        *Grant          `json:"grant,omitempty"`
	Deadline     time.Time       `json:"deadline,omitempty"`
	Sig          string          `json:"sig,omitempty"`
	Stream       string          `json:"stream,omitempty"`
	Seq          int64           `json:"seq,omitempty"`
	Data         string          `json:"data,omitempty"`
	Result       *Result         `json:"result,omitempty"`
}

// Grant is the scope the bridge builds the sandbox from (from the grant, never from call args).
type Grant struct {
	Capability string         `json:"capability"`
	Level      string         `json:"sandbox_level"`
	Selector   map[string]any `json:"selector"`
	Principal  string         `json:"principal"`
}

// Result ends a call.
type Result struct {
	OK         bool            `json:"ok"`
	ExitCode   *int            `json:"exit_code,omitempty"`
	Error      string          `json:"error,omitempty"`
	Message    string          `json:"message,omitempty"`
	DurationMS int64           `json:"duration_ms"`
	Stdout     string          `json:"stdout,omitempty"`
	Stderr     string          `json:"stderr,omitempty"`
	OutSHA256  string          `json:"out_sha256,omitempty"`
	Truncated  bool            `json:"truncated,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// SignKey derives the per-machine HMAC key from the machine token.
func SignKey(token string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("devagotchi call signing v1"))
	return m.Sum(nil)
}

func material(f Frame) []byte {
	h := sha256.Sum256(f.Args)
	g, _ := json.Marshal(f.Grant)
	gh := sha256.Sum256(g)
	return []byte(strings.Join([]string{f.CallID, f.GrantID, strconv.FormatInt(f.GrantVersion, 10), strconv.FormatInt(f.Fence, 10), f.Op,
		hex.EncodeToString(h[:]), hex.EncodeToString(gh[:]), f.Deadline.UTC().Format(time.RFC3339Nano)}, "\n"))
}

// Sign sets f.Sig over call_id, grant_id, grant_version, fence, op, args_sha256, grant scope and deadline.
func Sign(f *Frame, key []byte) {
	m := hmac.New(sha256.New, key)
	m.Write(material(*f))
	f.Sig = hex.EncodeToString(m.Sum(nil))
}

// Verify checks the signature and that the deadline has not passed.
func Verify(f Frame, key []byte, now time.Time) error {
	want, err := hex.DecodeString(f.Sig)
	if err != nil {
		return errors.New("bad signature encoding")
	}
	m := hmac.New(sha256.New, key)
	m.Write(material(f))
	if !hmac.Equal(want, m.Sum(nil)) {
		return errors.New("signature does not verify")
	}
	if now.After(f.Deadline) {
		return errors.New("call deadline passed")
	}
	return nil
}
