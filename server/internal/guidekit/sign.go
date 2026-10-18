package guidekit

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Bundle is a signed guide version as shipped by Shippin (12-guides.md#updates).
type Bundle struct {
	GuideID     string          `json:"guide_id"`
	Semver      string          `json:"semver"`
	Title       string          `json:"title"`
	Goal        string          `json:"goal"`
	Tags        []string        `json:"tags"`
	AppliesTo   json.RawMessage `json:"applies_to"`
	ChangeNotes string          `json:"change_notes"`
	RiskIDs     []string        `json:"risk_ids"`
	Steps       json.RawMessage `json:"steps"`
	KeyID       string          `json:"key_id"`
	Signature   string          `json:"signature,omitempty"`
}

// Canonical is the signed form: the bundle without its signature, with steps re-encoded compactly
// through a generic decode so key order is sorted.
func Canonical(b Bundle) ([]byte, error) {
	b.Signature = ""
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// ContentSHA is the hex sha-256 of the canonical bundle.
func ContentSHA(b Bundle) string {
	c, _ := Canonical(b)
	h := sha256.Sum256(c)
	return hex.EncodeToString(h[:])
}

// Sign signs the canonical bundle.
func Sign(b Bundle, priv ed25519.PrivateKey) (Bundle, error) {
	c, err := Canonical(b)
	if err != nil {
		return b, err
	}
	b.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, c))
	return b, nil
}

// TrustedKey is one entry of the guide_signing_keys setting.
type TrustedKey struct {
	ID     string `json:"id"`
	PubB64 string `json:"public_key"`
}

// Verify checks the bundle against the trusted keys; an unknown key id or a bad signature is refused.
func Verify(b Bundle, keys []TrustedKey) error {
	for _, k := range keys {
		if k.ID != b.KeyID {
			continue
		}
		pub, err := base64.StdEncoding.DecodeString(k.PubB64)
		sig, err2 := base64.StdEncoding.DecodeString(b.Signature)
		if err != nil || err2 != nil || len(pub) != ed25519.PublicKeySize {
			return errors.New("malformed key or signature")
		}
		c, err := Canonical(b)
		if err != nil {
			return err
		}
		if !ed25519.Verify(pub, c, sig) {
			return errors.New("signature does not verify")
		}
		return nil
	}
	return fmt.Errorf("key %q is not trusted", b.KeyID)
}

// Semver parses "x.y.z".
func Semver(s string) ([3]int, error) {
	var v [3]int
	p := strings.Split(s, ".")
	if len(p) != 3 {
		return v, fmt.Errorf("bad semver %q", s)
	}
	for i := range p {
		n, err := strconv.Atoi(p[i])
		if err != nil || n < 0 {
			return v, fmt.Errorf("bad semver %q", s)
		}
		v[i] = n
	}
	return v, nil
}

// Bump classifies a version change as major, minor, patch or "" (not newer).
func Bump(from, to string) string {
	a, e1 := Semver(from)
	b, e2 := Semver(to)
	if e1 != nil || e2 != nil {
		return ""
	}
	for i, n := range []string{"major", "minor", "patch"} {
		if b[i] != a[i] {
			if b[i] > a[i] {
				return n
			}
			return ""
		}
	}
	return ""
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
