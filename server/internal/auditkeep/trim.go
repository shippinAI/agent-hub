package auditkeep

import (
	"context"
	"fmt"
	"time"
)

// count is the exact row count of a category (prune only calls it when max_rows is set).
func (s *Service) count(ctx context.Context, cat string) int64 {
	var n int64
	_ = s.St.DB.QueryRow(ctx, `SELECT count(*) FROM action_events WHERE category=$1`, cat).Scan(&n)
	return n
}

// bytes is what a category costs: closed partitions by on-disk size, the open (today and later) ones by the
// size of their live rows, so a trimmed open partition stops counting the space VACUUM made reusable.
func (s *Service) bytes(ctx context.Context, cat string) int64 {
	pts, _ := s.Partitions(ctx, cat)
	today := s.now().Truncate(24 * time.Hour)
	var total int64
	for _, p := range pts {
		if p.Day.Before(today) {
			total += p.Bytes
			continue
		}
		var live int64
		_ = s.St.DB.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(sum(pg_column_size(t.*)),0) FROM %q t`, p.Name)).Scan(&live)
		total += live
	}
	return total
}

// trim deletes the oldest n rows of a category that are safe to lose, then VACUUMs so the space is reused.
func (s *Service) trim(ctx context.Context, cat string, n, safe int64, reason string, rep *Report) (int64, error) {
	if n <= 0 {
		return 0, nil
	}
	tag, err := s.St.DB.Exec(ctx, `DELETE FROM action_events WHERE category=$1 AND id IN
		(SELECT id FROM action_events WHERE category=$1 AND id <= $3 ORDER BY id LIMIT $2)`, cat, n, safe)
	if err != nil {
		return 0, err
	}
	if d := tag.RowsAffected(); d > 0 {
		rep.Deleted += d
		_, _ = s.St.DB.Exec(ctx, fmt.Sprintf(`VACUUM %q`, listTable(cat)))
		s.record(ctx, "audit.trimmed", map[string]any{"category": cat, "rows": d, "reason": reason, "archived_up_to": safe})
	}
	if tag.RowsAffected() < n {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %d rows over %s are not archived yet and were kept", cat, n-tag.RowsAffected(), reason))
	}
	return tag.RowsAffected(), nil
}

// globalCap drops the oldest closed partition of any category while the total is over the cap; when only open
// partitions are left it trims the largest category proportionally. capBytes <= 0 disables the cap.
func (s *Service) globalCap(ctx context.Context, capBytes int, safe map[string]int64, rep *Report) error {
	if capBytes <= 0 {
		return nil
	}
	limit := int64(capBytes)
	cats, err := s.Categories(ctx)
	if err != nil {
		return err
	}
	total := func() (int64, string, int64) {
		var sum, big int64
		var bigCat string
		for _, c := range cats {
			b := s.bytes(ctx, c.Category)
			sum += b
			if b > big {
				big, bigCat = b, c.Category
			}
		}
		return sum, bigCat, big
	}
	today := s.now().Truncate(24 * time.Hour)
	for guard := 0; guard < 1000; guard++ { // re-measure after each step: row sizes vary
		sum, bigCat, big := total()
		if sum <= limit {
			return nil
		}
		pts, err := s.Partitions(ctx, "")
		if err != nil {
			return err
		}
		dropped := false
		for _, p := range pts {
			if p.Day.Before(today) && s.dropIfSafe(ctx, p, safe[p.Category], "audit_total_max_bytes", rep) {
				dropped = true
				break
			}
		}
		if dropped {
			continue
		}
		n := s.count(ctx, bigCat)
		share := (sum - limit) * n / big
		if d, err := s.trim(ctx, bigCat, min(n, share+1), safe[bigCat], "audit_total_max_bytes", rep); err != nil || d == 0 {
			return err // nothing safe left to trim: the warning says why
		}
	}
	return nil
}
