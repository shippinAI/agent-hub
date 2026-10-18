package auditkeep

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sink stores archive objects. FileSink ships; an S3-compatible sink is planned (40, G-30) behind this interface.
type Sink interface {
	Put(ctx context.Context, name string, data []byte) (location string, err error)
}

// FileSink writes objects under Dir with 0600 files; names may contain "/" (category folders).
type FileSink struct{ Dir string }

// Put writes atomically (temp file + rename) and refuses to overwrite an existing segment.
func (f FileSink) Put(_ context.Context, name string, data []byte) (string, error) {
	if f.Dir == "" {
		return "", fmt.Errorf("archive dir not set")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("bad object name %q", name)
	}
	p := filepath.Join(f.Dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err == nil {
		return "", fmt.Errorf("archive object %s already exists", p)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", err
	}
	return "file://" + p, nil
}

// Signer signs manifest hashes. Ed25519Signer is the built-in; a KMS or Vaultix signer can implement it.
type Signer interface {
	KeyID() string
	Sign(msg []byte) []byte
}

// Ed25519Signer signs with a private key; KeyID is the first 16 hex chars of sha256(public key).
type Ed25519Signer struct{ Key ed25519.PrivateKey }

// KeyID identifies the public key that verifies this signer's signatures.
func (e Ed25519Signer) KeyID() string {
	h := sha256.Sum256(e.Key.Public().(ed25519.PublicKey))
	return hex.EncodeToString(h[:8])
}

// Sign signs msg.
func (e Ed25519Signer) Sign(msg []byte) []byte { return ed25519.Sign(e.Key, msg) }

// SignerFromFile loads a 32-byte seed (raw or hex) from path; "" = unsigned archives.
func SignerFromFile(path string) (Signer, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed := []byte(strings.TrimSpace(string(b)))
	if len(seed) == 64 {
		if seed, err = hex.DecodeString(string(seed)); err != nil {
			return nil, err
		}
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("signing key must be a 32-byte ed25519 seed (raw or hex)")
	}
	return Ed25519Signer{Key: ed25519.NewKeyFromSeed(seed)}, nil
}
