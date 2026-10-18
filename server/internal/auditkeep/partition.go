package auditkeep

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Partition is one daily leaf partition of a category.
type Partition struct {
	Name     string    `json:"name"`
	Category string    `json:"category"`
	Day      time.Time `json:"day"`
	Bytes    int64     `json:"bytes"`
	EstRows  int64     `json:"est_rows"`
}

const lockPartitions = 7475 // serialises CREATE/DROP of partitions across hub processes

func listTable(cat string) string { return "action_events_" + cat }

func dayTable(cat string, day time.Time) string {
	return listTable(cat) + "_p" + day.Format("20060102")
}

// EnsureCategory creates the category's LIST partition and its daily partitions from today to the look-ahead.
func (s *Service) EnsureCategory(ctx context.Context, cat string) error {
	if !catRe.MatchString(cat) {
		return fmt.Errorf("bad category %q", cat)
	}
	set, _ := s.St.LoadSettings(ctx)
	ahead := set.Int("audit_partition_ahead_days", 2)
	today := s.now().Truncate(24 * time.Hour)
	return s.locked(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF action_events FOR VALUES IN ('%s')
			PARTITION BY RANGE (at)`, listTable(cat), cat)); err != nil {
			return err
		}
		for d := 0; d <= ahead; d++ {
			if err := createDay(ctx, tx, cat, today.AddDate(0, 0, d)); err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureDay creates the partition that holds t (writers call it when an insert finds no partition).
func (s *Service) EnsureDay(ctx context.Context, cat string, t time.Time) error {
	if err := s.EnsureCategory(ctx, cat); err != nil {
		return err
	}
	return s.locked(ctx, func(tx pgx.Tx) error { return createDay(ctx, tx, cat, t.UTC().Truncate(24*time.Hour)) })
}

// EnsureAll runs EnsureCategory for every category row (startup and every prune tick).
func (s *Service) EnsureAll(ctx context.Context) error {
	cats, err := s.Categories(ctx)
	if err != nil {
		return err
	}
	for _, c := range cats {
		if err := s.EnsureCategory(ctx, c.Category); err != nil {
			return err
		}
	}
	return nil
}

func createDay(ctx context.Context, tx pgx.Tx, cat string, day time.Time) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		dayTable(cat, day), listTable(cat), day.Format(time.RFC3339), day.AddDate(0, 0, 1).Format(time.RFC3339)))
	return err
}

func (s *Service) locked(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.St.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockPartitions); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Partitions lists the daily partitions of a category ("" = all), oldest first, with on-disk size.
func (s *Service) Partitions(ctx context.Context, cat string) ([]Partition, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT c.relname, p.relname, pg_total_relation_size(c.oid), greatest(c.reltuples, 0)::bigint
		FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid JOIN pg_class p ON p.oid = i.inhparent
		WHERE p.relname LIKE 'action\_events\_%' AND p.relispartition AND ($1 = '' OR p.relname = 'action_events_' || $1)`, cat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Partition{}
	for rows.Next() {
		var pt Partition
		var parent string
		if err := rows.Scan(&pt.Name, &parent, &pt.Bytes, &pt.EstRows); err != nil {
			return nil, err
		}
		pt.Category = strings.TrimPrefix(parent, "action_events_")
		i := strings.LastIndex(pt.Name, "_p")
		if i < 0 {
			continue
		}
		if pt.Day, err = time.Parse("20060102", pt.Name[i+2:]); err != nil {
			continue
		}
		out = append(out, pt)
	}
	sort.Slice(out, func(a, b int) bool {
		if !out[a].Day.Equal(out[b].Day) {
			return out[a].Day.Before(out[b].Day)
		}
		return out[a].Name < out[b].Name
	})
	return out, rows.Err()
}

// drop detaches and drops one partition (DROP of a leaf is a metadata change; space returns at once).
func (s *Service) drop(ctx context.Context, pt Partition) error {
	return s.locked(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, pgx.Identifier{pt.Name}.Sanitize()))
		return err
	})
}
