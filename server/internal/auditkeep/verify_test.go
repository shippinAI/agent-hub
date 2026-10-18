package auditkeep

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func segmentFor(t *testing.T, lines []string, sg Signer) ([]byte, Segment) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	chain := genesis
	for _, l := range lines {
		chain = chainHash(chain, []byte(l))
		zw.Write([]byte(l + "\n"))
	}
	zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	m := Segment{Category: "proc", FromID: 1, ToID: int64(len(lines)), Rows: int64(len(lines)), FromAt: time.Unix(0, 0), ToAt: time.Unix(1, 0),
		Bytes: int64(buf.Len()), SHA256: hex.EncodeToString(sum[:]), RowsHash: chain, PrevHash: genesis}
	m.ManifestHash = ManifestHash(m)
	if sg != nil {
		m.KeyID, m.Signature = sg.KeyID(), hex.EncodeToString(sg.Sign([]byte(m.ManifestHash)))
	}
	return buf.Bytes(), m
}

func TestVerifySegmentDetectsTampering(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	sg := Ed25519Signer{Key: priv}
	gz, m := segmentFor(t, []string{`{"id":1}`, `{"id":2}`}, sg)
	if err := VerifySegment(gz, m, pub); err != nil {
		t.Fatalf("good segment: %v", err)
	}
	bad := m
	bad.Rows = 3
	if VerifySegment(gz, bad, pub) == nil {
		t.Error("row count change not detected")
	}
	gz2, m2 := segmentFor(t, []string{`{"id":1}`, `{"id":9}`}, nil)
	m2.SHA256, m2.Signature = m.SHA256, m.Signature
	if VerifySegment(gz2, m2, pub) == nil {
		t.Error("content swap not detected")
	}
	forged := m
	forged.RowsHash = genesis
	forged.ManifestHash = ManifestHash(forged)
	if VerifySegment(gz, forged, pub) == nil {
		t.Error("forged manifest not detected")
	}
	other, _, _ := ed25519.GenerateKey(nil)
	if VerifySegment(gz, m, other) == nil {
		t.Error("wrong key accepted")
	}
}

func TestFileSinkRefusesOverwriteAndTraversal(t *testing.T) {
	f := FileSink{Dir: t.TempDir()}
	if _, err := f.Put(context.Background(), "proc/a.gz", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Put(context.Background(), "proc/a.gz", []byte("y")); err == nil {
		t.Error("overwrote a segment")
	}
	if _, err := f.Put(context.Background(), "../escape", []byte("y")); err == nil {
		t.Error("path traversal accepted")
	}
	if _, err := (FileSink{}).Put(context.Background(), "a", nil); err == nil {
		t.Error("empty dir accepted")
	}
}
