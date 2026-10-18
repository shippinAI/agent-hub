package auditkeep

import (
	"context"
	"fmt"
	"time"

	"agenthub/internal/store"
)

// Report is what one prune run did.
type Report struct {
	At       time.Time `json:"at"`
	Archived []Segment `json:"archived"`
	Dropped  []string  `json:"dropped"`
	Deleted  int64     `json:"deleted_rows"`
	Bytes    int64     `json:"bytes_after"`
	Warnings []string  `json:"warnings"`
}

// settle keeps the newest rows out of an archive so slow transactions with lower ids commit first.
const settle = 5 * time.Second

// Prune applies every category's limits, then the global byte cap. Rows are archived before anything is
// dropped unless the archive is off; with audit_on_archive_failure=drop, storage wins over completeness.
func (s *Service) Prune(ctx context.Context) (Report, error) {
	rep := Report{At: s.now(), Archived: []Segment{}, Dropped: []string{}, Warnings: []string{}}
	if err := s.EnsureAll(ctx); err != nil {
		return rep, err
	}
	cats, err := s.Categories(ctx)
	if err != nil {
		return rep, err
	}
	set, _ := s.St.LoadSettings(ctx)
	keepOnFail := set.String("audit_on_archive_failure", "drop") == "keep"
	safe := map[string]int64{} // category -> highest id that may be dropped
	for _, c := range cats {
		safe[c.Category] = s.archiveFor(ctx, c, keepOnFail, &rep)
	}
	for _, c := range cats {
		if err := s.applyLimits(ctx, c, safe[c.Category], &rep); err != nil {
			return rep, err
		}
	}
	if err := s.globalCap(ctx, set.Int("audit_total_max_bytes", 1<<30), safe, &rep); err != nil {
		return rep, err
	}
	pts, _ := s.Partitions(ctx, "")
	for _, p := range pts {
		rep.Bytes += p.Bytes
	}
	return rep, nil
}

// archiveFor archives a category and returns the id below which dropping loses nothing (or everything,
// when archiving is off or failed under the drop policy).
func (s *Service) archiveFor(ctx context.Context, c Category, keepOnFail bool, rep *Report) int64 {
	const all = int64(1<<62 - 1)
	if !c.Archive {
		return all
	}
	segs, err := s.Archive(ctx, c.Category, s.now().Add(-settle))
	rep.Archived = append(rep.Archived, segs...)
	if err == nil {
		to, _ := s.archivedUpTo(ctx, c.Category)
		return to
	}
	rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: archive failed: %v", c.Category, err))
	if keepOnFail {
		to, _ := s.archivedUpTo(ctx, c.Category)
		return to
	}
	return all
}

func (s *Service) applyLimits(ctx context.Context, c Category, safe int64, rep *Report) error {
	pts, err := s.Partitions(ctx, c.Category)
	if err != nil {
		return err
	}
	today := s.now().Truncate(24 * time.Hour)
	var old []Partition // closed partitions, oldest first
	for _, p := range pts {
		if p.Day.Before(today) {
			old = append(old, p)
		}
	}
	for len(old) > 0 && c.RetainDays > 0 && !old[0].Day.AddDate(0, 0, 1+c.RetainDays).After(s.now()) {
		if !s.dropIfSafe(ctx, old[0], safe, "retain_days", rep) {
			break
		}
		old = old[1:]
	}
	over := func(limit int64, size func() int64) bool { return limit > 0 && size() > limit }
	rows := func() int64 { return s.count(ctx, c.Category) }
	size := func() int64 { return s.bytes(ctx, c.Category) }
	for len(old) > 0 && (over(c.MaxRows, rows) || over(c.MaxBytes, size)) {
		reason := map[bool]string{true: "max_rows", false: "max_bytes"}[over(c.MaxRows, rows)]
		if !s.dropIfSafe(ctx, old[0], safe, reason, rep) {
			break
		}
		old = old[1:]
	}
	if n := rows(); c.MaxRows > 0 && n > c.MaxRows {
		if _, err := s.trim(ctx, c.Category, n-c.MaxRows, safe, "max_rows", rep); err != nil {
			return err
		}
	}
	for i := 0; i < 100 && c.MaxBytes > 0; i++ { // proportional share of live rows, re-measured until under
		b := size()
		if b <= c.MaxBytes {
			break
		}
		n := rows()
		if d, err := s.trim(ctx, c.Category, n-n*c.MaxBytes/b+1, safe, "max_bytes", rep); err != nil || d == 0 {
			return err
		}
	}
	return nil
}

func (s *Service) dropIfSafe(ctx context.Context, p Partition, safe int64, reason string, rep *Report) bool {
	var maxID int64
	_ = s.St.DB.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(max(id),0) FROM %q`, p.Name)).Scan(&maxID)
	if maxID > safe {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: kept %s (%s) because it is not archived yet", p.Category, p.Name, reason))
		return false
	}
	if err := s.drop(ctx, p); err != nil {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("drop %s: %v", p.Name, err))
		return false
	}
	rep.Dropped = append(rep.Dropped, p.Name)
	s.record(ctx, "audit.pruned", map[string]any{"category": p.Category, "partition": p.Name, "reason": reason,
		"bytes": p.Bytes, "est_rows": p.EstRows, "archived_up_to": safe})
	return true
}

func (s *Service) record(ctx context.Context, action string, data any) {
	_, _ = s.St.Write(ctx, store.System, "", "audit_prune", nil, func(w *store.W) (any, error) { return nil, w.Event(action, nil, nil, data) })
}
