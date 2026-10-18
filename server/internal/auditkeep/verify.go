package auditkeep

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// VerifySegment checks one archive object against its manifest: file hash, per-line hash chain, manifest hash
// and (when pub is given and the manifest is signed) the ed25519 signature.
func VerifySegment(gz []byte, m Segment, pub ed25519.PublicKey) error {
	sum := sha256.Sum256(gz)
	if hex.EncodeToString(sum[:]) != m.SHA256 {
		return fmt.Errorf("sha256 mismatch")
	}
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	chain, n := m.PrevHash, int64(0)
	for sc.Scan() {
		chain = chainHash(chain, sc.Bytes())
		n++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if n != m.Rows || chain != m.RowsHash {
		return fmt.Errorf("row chain mismatch (rows %d/%d)", n, m.Rows)
	}
	if ManifestHash(m) != m.ManifestHash {
		return fmt.Errorf("manifest hash mismatch")
	}
	if pub != nil && m.Signature != "" {
		sig, err := hex.DecodeString(m.Signature)
		if err != nil || !ed25519.Verify(pub, []byte(m.ManifestHash), sig) {
			return fmt.Errorf("bad signature")
		}
	}
	return nil
}

// VerifyChainOfSegments checks that every category's segments link prev_hash -> manifest_hash without gaps.
func (s *Service) VerifyChainOfSegments(ctx context.Context) error {
	rows, err := s.St.DB.Query(ctx, `SELECT category, from_id, to_id, prev_hash, manifest_hash FROM audit_archives ORDER BY category, to_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	last := map[string][2]string{} // category -> {manifest_hash, to_id}
	for rows.Next() {
		var cat, prev, mh string
		var from, to int64
		if err := rows.Scan(&cat, &from, &to, &prev, &mh); err != nil {
			return err
		}
		want := genesis
		if l, ok := last[cat]; ok {
			want = l[0]
		}
		if prev != want {
			return fmt.Errorf("%s segment %d-%d: prev_hash does not match the previous segment", cat, from, to)
		}
		last[cat] = [2]string{mh, fmt.Sprint(to)}
	}
	return rows.Err()
}
