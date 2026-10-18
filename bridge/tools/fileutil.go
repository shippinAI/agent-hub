package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var imageMimes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp"}

func ext(p string) string { return strings.ToLower(filepath.Ext(p)) }

// fileSHA hashes a file up to limit bytes ("" when bigger: hashing a huge file is not free).
func fileSHA(p string, limit int64) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || st.Size() > limit {
		return ""
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func shaOf(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// readCapped reads a whole file, refusing more than limit bytes.
func readCapped(p string, limit int64) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		return nil, errTooBig(limit)
	}
	return b, err
}

type errTooBig int64

func (e errTooBig) Error() string {
	return "file is larger than this policy's max_read_bytes (" + strconv.FormatInt(int64(e), 10) + ")"
}
