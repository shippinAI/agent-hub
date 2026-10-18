package auditkeep

import (
	"context"
	"log"
	"time"
)

// Flush archives every not-yet-archived row now (no settle window), for one category or all ("").
// Nothing is deleted; use it before exports, upgrades or moving the database.
func (s *Service) Flush(ctx context.Context, cat string) ([]Segment, error) {
	cats, err := s.Categories(ctx)
	if err != nil {
		return nil, err
	}
	out := []Segment{}
	for _, c := range cats {
		if cat != "" && c.Category != cat {
			continue
		}
		segs, err := s.Archive(ctx, c.Category, s.now().Add(time.Second))
		out = append(out, segs...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// Run keeps partitions ahead of the clock and prunes every audit_prune_interval_seconds (re-read each tick,
// so the admin UI change applies without a restart). 0 disables the timer; Prune can still run on demand.
func (s *Service) Run(ctx context.Context) {
	if err := s.EnsureAll(ctx); err != nil {
		log.Printf("auditkeep: ensure partitions: %v", err)
	}
	for {
		set, _ := s.St.LoadSettings(ctx)
		every := time.Duration(set.Int("audit_prune_interval_seconds", 300)) * time.Second
		wait := every
		if every <= 0 {
			wait = time.Minute // poll for the setting to come back on
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if every <= 0 {
			continue
		}
		if rep, err := s.Prune(ctx); err != nil {
			log.Printf("auditkeep: prune: %v", err)
		} else if len(rep.Warnings) > 0 {
			log.Printf("auditkeep: prune warnings: %v", rep.Warnings)
		}
	}
}
