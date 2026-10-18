// Package secretbox encrypts write-only secrets (decision D5, personal edition): a 32-byte key in a 0600 file,
// AES-256-GCM in the hub process. The key never reaches Postgres, logs or any API; callers only ever see Fp.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Box seals and opens secrets.
type Box struct{ aead cipher.AEAD }

// Open loads the key file, creating it (0600, parent 0700) when it does not exist yet.
func Open(path string) (*Box, error) {
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("secretbox: create key file: %w", err)
		}
		defer f.Close()
		if _, err := f.Write(key); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, fmt.Errorf("secretbox: read key file: %w", err)
	}
	if st, err := os.Stat(path); err == nil && st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secretbox: key file %s must not be readable by group/others", path)
	}
	return FromKey(key)
}

// FromKey builds a box from raw key bytes (32 bytes).
func FromKey(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("secretbox: key must be 32 bytes")
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	return &Box{aead: g}, nil
}

// Seal encrypts plaintext; aad binds the ciphertext to its row (e.g. the secret id).
func (b *Box) Seal(plain, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plain, aad), nil
}

// OpenBytes decrypts a sealed value.
func (b *Box) OpenBytes(ct, aad []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(ct) < n {
		return nil, errors.New("secretbox: ciphertext too short")
	}
	return b.aead.Open(nil, ct[:n], ct[n:], aad)
}

// Fp is the only thing ever shown about a secret: the first 8 hex chars of its SHA-256.
func Fp(plain []byte) string {
	s := sha256.Sum256(plain)
	return hex.EncodeToString(s[:4])
}
