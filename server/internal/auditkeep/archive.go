package auditkeep

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"agenthub/internal/store"
)

// Segment is one archive object plus its manifest (audit_archives row).
type Segment struct {
	Category     string    `json:"category"`
	FromID       int64     `json:"from_id"`
	ToID         int64     `json:"to_id"`
	Rows         int64     `json:"rows"`
	FromAt       time.Time `json:"from_at"`
	ToAt         time.Time `json:"to_at"`
	Location     string    `json:"location"`
	Bytes        int64     `json:"bytes"`
	SHA256       string    `json:"sha256"`
	RowsHash     string    `json:"rows_hash"`
	PrevHash     string    `json:"prev_hash"`
	ManifestHash string    `json:"manifest_hash"`
	Signature    string    `json:"signature,omitempty"`
	KeyID        string    `json:"key_id,omitempty"`
}

const genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// batchRows caps one segment so archiving never needs unbounded memory.
const batchRows = 50000

// sink returns the configured sink: an injected one, else a FileSink on audit_archive_dir, else nil.
func (s *Service) sink(ctx context.Context) Sink {
	if s.Sink != nil {
		return s.Sink
	}
	set, _ := s.St.LoadSettings(ctx)
	if d := set.String("audit_archive_dir", ""); d != "" {
		return FileSink{Dir: d}
	}
	return nil
}

// archivedUpTo returns the last archived id and manifest hash of a category.
func (s *Service) archivedUpTo(ctx context.Context, cat string) (int64, string) {
	var to int64
	prev := genesis
	_ = s.St.DB.QueryRow(ctx, `SELECT to_id, manifest_hash FROM audit_archives WHERE category=$1 ORDER BY to_id DESC LIMIT 1`, cat).Scan(&to, &prev)
	return to, prev
}

// Archive writes every not-yet-archived row of cat older than before into chained segments.
func (s *Service) Archive(ctx context.Context, cat string, before time.Time) ([]Segment, error) {
	sk := s.sink(ctx)
	if sk == nil {
		return nil, fmt.Errorf("no archive sink (set audit_archive_dir)")
	}
	var out []Segment
	for {
		seg, n, err := s.archiveBatch(ctx, sk, cat, before)
		if err != nil || n == 0 {
			return out, err
		}
		out = append(out, seg)
		if n < batchRows {
			return out, nil
		}
	}
}

func (s *Service) archiveBatch(ctx context.Context, sk Sink, cat string, before time.Time) (Segment, int64, error) {
	after, prev := s.archivedUpTo(ctx, cat)
	rows, err := s.St.DB.Query(ctx, `SELECT id, at, category, kind, call_id, actor, machine, status, data FROM action_events
		WHERE category=$1 AND id > $2 AND at < $3 ORDER BY id LIMIT $4`, cat, after, before, batchRows)
	if err != nil {
		return Segment{}, 0, err
	}
	defer rows.Close()
	seg := Segment{Category: cat, PrevHash: prev}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	chain := prev
	for rows.Next() {
		var r struct {
			ID       int64           `json:"id"`
			At       time.Time       `json:"at"`
			Category string          `json:"category"`
			Kind     string          `json:"kind"`
			CallID   string          `json:"call_id"`
			Actor    string          `json:"actor"`
			Machine  string          `json:"machine"`
			Status   string          `json:"status"`
			Data     json.RawMessage `json:"data"`
		}
		if err := rows.Scan(&r.ID, &r.At, &r.Category, &r.Kind, &r.CallID, &r.Actor, &r.Machine, &r.Status, &r.Data); err != nil {
			return seg, 0, err
		}
		line, _ := json.Marshal(r)
		chain = chainHash(chain, line)
		_, _ = zw.Write(append(line, '\n'))
		if seg.Rows == 0 {
			seg.FromID, seg.FromAt = r.ID, r.At
		}
		seg.ToID, seg.ToAt = r.ID, r.At
		seg.Rows++
	}
	if err := rows.Err(); err != nil || seg.Rows == 0 {
		return seg, 0, err
	}
	if err := zw.Close(); err != nil {
		return seg, 0, err
	}
	sum := sha256.Sum256(buf.Bytes())
	seg.SHA256, seg.RowsHash, seg.Bytes = hex.EncodeToString(sum[:]), chain, int64(buf.Len())
	seg.ManifestHash = ManifestHash(seg)
	if s.Signer != nil {
		seg.KeyID, seg.Signature = s.Signer.KeyID(), hex.EncodeToString(s.Signer.Sign([]byte(seg.ManifestHash)))
	}
	base := fmt.Sprintf("%s/%020d-%020d", cat, seg.FromID, seg.ToID)
	if seg.Location, err = sk.Put(ctx, base+".jsonl.gz", buf.Bytes()); err != nil {
		return seg, 0, err
	}
	man, _ := json.MarshalIndent(seg, "", "  ")
	if _, err := sk.Put(ctx, base+".manifest.json", man); err != nil {
		return seg, 0, err
	}
	_, err = s.St.Write(ctx, store.System, "", "audit_archive", nil, func(w *store.W) (any, error) {
		_, err := w.Exec(`INSERT INTO audit_archives(category, from_id, to_id, rows, from_at, to_at, location, bytes, sha256, rows_hash,
			prev_hash, manifest_hash, signature, key_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			cat, seg.FromID, seg.ToID, seg.Rows, seg.FromAt, seg.ToAt, seg.Location, seg.Bytes, seg.SHA256, seg.RowsHash,
			seg.PrevHash, seg.ManifestHash, seg.Signature, seg.KeyID)
		if err != nil {
			return nil, err
		}
		return nil, w.Event("audit.archived", nil, nil, map[string]any{"category": cat, "from_id": seg.FromID, "to_id": seg.ToID,
			"rows": seg.Rows, "manifest_hash": seg.ManifestHash, "location": seg.Location})
	})
	return seg, seg.Rows, err
}

// chainHash links one archived line to the previous: sha256(prev + "\n" + line).
func chainHash(prev string, line []byte) string {
	h := sha256.Sum256(append([]byte(prev+"\n"), line...))
	return hex.EncodeToString(h[:])
}

// ManifestHash hashes the manifest fields that the signature covers (everything except location and signature).
func ManifestHash(g Segment) string {
	b, _ := json.Marshal([]any{g.Category, g.FromID, g.ToID, g.Rows, g.FromAt.UTC().Format(time.RFC3339Nano),
		g.ToAt.UTC().Format(time.RFC3339Nano), g.Bytes, g.SHA256, g.RowsHash, g.PrevHash})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
