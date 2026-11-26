package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"time"

	"agenthub/internal/policy"
)

var providerID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// UsageInput is one model call or turn, as the client reported it. Tokens are by kind: cache_read is part of
// input and reasoning is part of output, as the providers report them. Cost is either reported (charged by the
// provider, or the client's own estimate) or computed here from price_catalog; never both.
type UsageInput struct {
	TaskID           *int64     `json:"task_id,omitempty"`
	Fence            *int64     `json:"fence,omitempty"`
	OccurredAt       *time.Time `json:"occurred_at,omitempty"`
	ProviderID       string     `json:"provider_id"`
	Model            string     `json:"model,omitempty"`
	BillingKind      string     `json:"billing_kind"`
	InputTokens      int64      `json:"input_tokens,omitempty"`
	CacheReadTokens  int64      `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64      `json:"cache_write_tokens,omitempty"`
	OutputTokens     int64      `json:"output_tokens,omitempty"`
	ReasoningTokens  int64      `json:"reasoning_tokens,omitempty"`
	Requests         *int       `json:"requests,omitempty"`
	CostMicros       *int64     `json:"cost_micros,omitempty"`
	CostBasis        string     `json:"cost_basis,omitempty"`
	Source           string     `json:"source"`
	ExternalRef      string     `json:"external_ref,omitempty"`
	DedupeKey        string     `json:"dedupe_key,omitempty"`
}

// UsageEvent is a ledger row.
type UsageEvent struct {
	ID               int64     `json:"id"`
	OccurredAt       time.Time `json:"occurred_at"`
	PrincipalID      string    `json:"principal_id"`
	TaskID           *int64    `json:"task_id"`
	Fence            *int64    `json:"fence"`
	Project          *string   `json:"project"`
	ProviderID       string    `json:"provider_id"`
	Model            string    `json:"model"`
	BillingKind      string    `json:"billing_kind"`
	InputTokens      int64     `json:"input_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	ReasoningTokens  int64     `json:"reasoning_tokens"`
	Requests         int       `json:"requests"`
	CostMicros       *int64    `json:"cost_micros"`
	CostBasis        *string   `json:"cost_basis"`
	PriceID          *string   `json:"price_id"`
	Source           string    `json:"source"`
	ExternalRef      string    `json:"external_ref"`
	Duplicate        bool      `json:"duplicate,omitempty"`
}

const usageCols = `id,occurred_at,principal_id,task_id,fence,project,provider_id,model,billing_kind,input_tokens,cache_read_tokens,
	cache_write_tokens,output_tokens,reasoning_tokens,requests,cost_micros,cost_basis,price_id,source,external_ref`

func scanUsage(r interface{ Scan(...any) error }) (UsageEvent, error) {
	var u UsageEvent
	err := r.Scan(&u.ID, &u.OccurredAt, &u.PrincipalID, &u.TaskID, &u.Fence, &u.Project, &u.ProviderID, &u.Model, &u.BillingKind,
		&u.InputTokens, &u.CacheReadTokens, &u.CacheWriteTokens, &u.OutputTokens, &u.ReasoningTokens, &u.Requests,
		&u.CostMicros, &u.CostBasis, &u.PriceID, &u.Source, &u.ExternalRef)
	return u, err
}

var (
	billingKinds = []string{"api_key", "subscription", "router"}
	usageSources = []string{"stream", "otel", "app_server", "api_response", "import", "manual"}
)

func (in *UsageInput) validate() error {
	switch {
	case !providerID.MatchString(in.ProviderID):
		return errInvalid("provider_id must match %s", providerID)
	case len(in.Model) > 200:
		return errInvalid("model is limited to 200 chars")
	case !slices.Contains(billingKinds, in.BillingKind):
		return errInvalid("billing_kind must be one of %v", billingKinds)
	case !slices.Contains(usageSources, in.Source):
		return errInvalid("source must be one of %v", usageSources)
	case in.InputTokens < 0 || in.CacheReadTokens < 0 || in.CacheWriteTokens < 0 || in.OutputTokens < 0 || in.ReasoningTokens < 0:
		return errInvalid("token counts cannot be negative")
	case in.CacheReadTokens > in.InputTokens:
		return errInvalid("cache_read_tokens is part of input_tokens, so it cannot exceed it")
	case in.ReasoningTokens > in.OutputTokens:
		return errInvalid("reasoning_tokens is part of output_tokens, so it cannot exceed it")
	case in.Requests != nil && *in.Requests < 0:
		return errInvalid("requests cannot be negative")
	case in.CostMicros != nil && *in.CostMicros < 0:
		return errInvalid("cost_micros cannot be negative")
	case in.CostMicros != nil && in.CostBasis != "provider_charged" && in.CostBasis != "client_estimate":
		return errInvalid("a reported cost needs cost_basis provider_charged or client_estimate")
	case in.CostMicros == nil && in.CostBasis != "":
		return errInvalid("cost_basis without cost_micros")
	case len(in.ExternalRef) > 300 || len(in.DedupeKey) > 300:
		return errInvalid("external_ref and dedupe_key are limited to 300 chars")
	}
	return nil
}

// RecordUsage appends one usage row for the caller (a principal only ever records its own usage), attributed
// to a task when given. The dedupe key (or provider + external_ref) makes re-ingest idempotent: a repeat
// returns the stored row with duplicate=true. Budgets in scope are evaluated and reported back.
func (s *Store) RecordUsage(ctx context.Context, p *Principal, reqID string, in UsageInput) (any, error) {
	return s.write(ctx, p, reqID, "record_usage", in, func(w *W) (any, error) {
		if err := in.validate(); err != nil {
			return nil, err
		}
		var task *Task
		var fence *int64
		var project *string
		if in.TaskID != nil {
			t, err := getTask(ctx, w.tx, *in.TaskID, false)
			if err != nil {
				return nil, err
			}
			if in.Fence != nil && *in.Fence != t.Fence {
				e := fail(409, "stale_fence", "fence %d is not current", *in.Fence)
				e.Current = map[string]any{"fence": t.Fence, "holder": t.Holder}
				return nil, e
			}
			if t.Holder != nil && *t.Holder == p.ID {
				fence = &t.Fence
			}
			task, project = &t, &t.Project
		}
		if err := w.can("usage.record", policyRes(task)); err != nil {
			return nil, err
		}
		key := in.DedupeKey
		if key == "" && in.ExternalRef != "" {
			key = in.ProviderID + ":" + in.ExternalRef
		}
		if key == "" {
			b := make([]byte, 12)
			_, _ = rand.Read(b)
			key = "auto:" + hex.EncodeToString(b)
		}
		key = p.ID + "/" + key
		at := time.Now().UTC()
		if in.OccurredAt != nil {
			at = in.OccurredAt.UTC()
		}
		if at.After(time.Now().Add(5 * time.Minute)) {
			return nil, errInvalid("occurred_at is in the future")
		}
		reqs := 1
		if in.Requests != nil {
			reqs = *in.Requests
		}
		cost, basis := in.CostMicros, nilIfEmpty(in.CostBasis)
		var priceID *string
		if cost == nil {
			if pr, ok, err := w.priceAt(in.ProviderID, in.Model, at); err != nil {
				return nil, err
			} else if ok {
				c := pr.cost(in)
				cost, priceID = &c, &pr.ID
				computed := "computed"
				basis = &computed
			}
		}
		u, err := scanUsage(w.row(`INSERT INTO usage_events(occurred_at,principal_id,task_id,fence,project,provider_id,model,billing_kind,
			input_tokens,cache_read_tokens,cache_write_tokens,output_tokens,reasoning_tokens,requests,cost_micros,cost_basis,price_id,
			source,external_ref,dedupe_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
			ON CONFLICT (dedupe_key) DO NOTHING RETURNING `+usageCols,
			at, p.ID, in.TaskID, fence, project, in.ProviderID, in.Model, in.BillingKind, in.InputTokens, in.CacheReadTokens,
			in.CacheWriteTokens, in.OutputTokens, in.ReasoningTokens, reqs, cost, basis, priceID, in.Source, in.ExternalRef, key))
		if err != nil {
			if !isNoRows(err) {
				return nil, err
			}
			u, err = scanUsage(w.row(`SELECT `+usageCols+` FROM usage_events WHERE dedupe_key=$1`, key))
			u.Duplicate = true
			return map[string]any{"usage": u}, err
		}
		if err := w.Event("usage.recorded", in.TaskID, fence, map[string]any{"usage_id": u.ID, "provider_id": u.ProviderID,
			"model": u.Model, "tokens": u.InputTokens + u.CacheWriteTokens + u.OutputTokens, "cost_micros": u.CostMicros,
			"cost_basis": u.CostBasis}); err != nil {
			return nil, err
		}
		st, err := w.budgetStatus(p.ID, task)
		if err != nil {
			return nil, err
		}
		if err := w.fireAlerts(st); err != nil {
			return nil, err
		}
		return map[string]any{"usage": u, "budgets": st}, nil
	})
}

func policyRes(t *Task) policy.Resource {
	if t == nil {
		return policy.Resource{}
	}
	return resOf(*t)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Price is a price_catalog row; prices are micro-USD per million tokens.
type Price struct {
	ID            string    `json:"id"`
	ProviderID    string    `json:"provider_id"`
	Model         string    `json:"model"`
	Input         int64     `json:"input_micros_per_mtok"`
	CacheRead     int64     `json:"cache_read_micros_per_mtok"`
	CacheWrite    int64     `json:"cache_write_micros_per_mtok"`
	Output        int64     `json:"output_micros_per_mtok"`
	EffectiveFrom time.Time `json:"effective_from"`
	Source        string    `json:"source"`
	SourceURL     string    `json:"source_url"`
	Version       int64     `json:"version"`
}

const priceCols = `id,provider_id,model,input_micros_per_mtok,cache_read_micros_per_mtok,cache_write_micros_per_mtok,output_micros_per_mtok,
	effective_from,source,source_url,version`

func scanPrice(r interface{ Scan(...any) error }) (Price, error) {
	var p Price
	err := r.Scan(&p.ID, &p.ProviderID, &p.Model, &p.Input, &p.CacheRead, &p.CacheWrite, &p.Output, &p.EffectiveFrom,
		&p.Source, &p.SourceURL, &p.Version)
	return p, err
}

// cost is tokens x price, rounded to the nearest micro-USD. Uncached input = input - cache read.
func (p Price) cost(in UsageInput) int64 {
	sum := (in.InputTokens-in.CacheReadTokens)*p.Input + in.CacheReadTokens*p.CacheRead +
		in.CacheWriteTokens*p.CacheWrite + in.OutputTokens*p.Output
	return (sum + 500_000) / 1_000_000
}

// priceAt is the row in force for provider/model at t (a past event is priced at that day's rate).
func (w *W) priceAt(provider, model string, at time.Time) (Price, bool, error) {
	if model == "" {
		return Price{}, false, nil
	}
	p, err := scanPrice(w.row(`SELECT `+priceCols+` FROM price_catalog WHERE provider_id=$1 AND model=$2 AND effective_from <= $3
		ORDER BY effective_from DESC LIMIT 1`, provider, model, at))
	if isNoRows(err) {
		return Price{}, false, nil
	}
	return p, err == nil, err
}

// PutPrice creates (version 0) or updates a price row. Humans only; prices are data with a source.
func (s *Store) PutPrice(ctx context.Context, p *Principal, reqID string, pr Price) (any, error) {
	return s.write(ctx, p, reqID, "put_price", pr, func(w *W) (any, error) {
		if err := w.admin("prices.put"); err != nil {
			return nil, err
		}
		switch {
		case !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,80}$`).MatchString(pr.ID):
			return nil, errInvalid("price id must be lowercase letters, digits, . _ -")
		case !providerID.MatchString(pr.ProviderID):
			return nil, errInvalid("provider_id must match %s", providerID)
		case pr.Model == "" || len(pr.Model) > 200:
			return nil, errInvalid("model must be 1-200 chars")
		case pr.Input < 0 || pr.CacheRead < 0 || pr.CacheWrite < 0 || pr.Output < 0:
			return nil, errInvalid("prices cannot be negative")
		case pr.EffectiveFrom.IsZero():
			return nil, errInvalid("effective_from is required")
		case !slices.Contains([]string{"user", "vendor_page", "litellm", "openrouter"}, pr.Source):
			return nil, errInvalid("source must be user, vendor_page, litellm or openrouter")
		case pr.Source != "user" && pr.SourceURL == "":
			return nil, errInvalid("a price taken from a source needs its source_url")
		}
		err := w.versioned("price_catalog", pr.ID, pr.Version, func() error {
			_, e := w.exec(`INSERT INTO price_catalog(id,provider_id,model,input_micros_per_mtok,cache_read_micros_per_mtok,
				cache_write_micros_per_mtok,output_micros_per_mtok,effective_from,source,source_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				pr.ID, pr.ProviderID, pr.Model, pr.Input, pr.CacheRead, pr.CacheWrite, pr.Output, pr.EffectiveFrom, pr.Source, pr.SourceURL)
			return e
		}, func() error {
			_, e := w.exec(`UPDATE price_catalog SET provider_id=$2,model=$3,input_micros_per_mtok=$4,cache_read_micros_per_mtok=$5,
				cache_write_micros_per_mtok=$6,output_micros_per_mtok=$7,effective_from=$8,source=$9,source_url=$10,version=version+1
				WHERE id=$1`, pr.ID, pr.ProviderID, pr.Model, pr.Input, pr.CacheRead, pr.CacheWrite, pr.Output, pr.EffectiveFrom, pr.Source, pr.SourceURL)
			return e
		})
		if err != nil {
			return nil, err
		}
		out, err := scanPrice(w.row(`SELECT `+priceCols+` FROM price_catalog WHERE id=$1`, pr.ID))
		if err != nil {
			return nil, err
		}
		return out, w.Event("admin.price.put", nil, nil, out)
	})
}

// Prices lists the catalog.
func (s *Store) Prices(ctx context.Context) ([]Price, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+priceCols+` FROM price_catalog ORDER BY provider_id, model, effective_from`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Price{}
	for rows.Next() {
		p, err := scanPrice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// UsageTotals sums the ledger for one grouping.
type UsageTotals struct {
	Key              string `json:"key"`
	Events           int64  `json:"events"`
	InputTokens      int64  `json:"input_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CostMicros       int64  `json:"cost_micros"`
	// UnpricedEvents had no reported cost and no catalog price: their tokens count, their cost is unknown.
	UnpricedEvents int64 `json:"unpriced_events"`
	EstimateMicros int64 `json:"estimate_micros"`
}

// UsageFilter narrows Usage.
type UsageFilter struct {
	GroupBy   string // task | principal | project | model | provider | day
	TaskID    int64
	Principal string
	From, To  *time.Time
}

var usageGroups = map[string]string{"task": "coalesce(task_id::text,'')", "principal": "principal_id",
	"project": "coalesce(project,'')", "model": "provider_id||'/'||model", "provider": "provider_id",
	"day": "to_char(occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD')", "": "''"}

// Usage returns totals grouped as asked, largest cost first.
func (s *Store) Usage(ctx context.Context, f UsageFilter) ([]UsageTotals, error) {
	g, ok := usageGroups[f.GroupBy]
	if !ok {
		return nil, errInvalid("group_by must be task, principal, project, model, provider or day")
	}
	rows, err := s.DB.Query(ctx, `SELECT `+g+` k, count(*), sum(input_tokens), sum(cache_read_tokens), sum(cache_write_tokens),
		sum(output_tokens), coalesce(sum(cost_micros),0), count(*) FILTER (WHERE cost_micros IS NULL),
		coalesce(sum(cost_micros) FILTER (WHERE cost_basis='client_estimate'),0)
		FROM usage_events WHERE ($1=0 OR task_id=$1) AND ($2='' OR principal_id=$2)
		AND ($3::timestamptz IS NULL OR occurred_at >= $3) AND ($4::timestamptz IS NULL OR occurred_at < $4)
		GROUP BY 1 ORDER BY 7 DESC, 1 LIMIT 500`, f.TaskID, f.Principal, f.From, f.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageTotals{}
	for rows.Next() {
		var u UsageTotals
		if err := rows.Scan(&u.Key, &u.Events, &u.InputTokens, &u.CacheReadTokens, &u.CacheWriteTokens, &u.OutputTokens,
			&u.CostMicros, &u.UnpricedEvents, &u.EstimateMicros); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// TaskUsage is one task's totals (zero when it has none).
func (s *Store) TaskUsage(ctx context.Context, taskID int64) (UsageTotals, error) {
	l, err := s.Usage(ctx, UsageFilter{TaskID: taskID})
	if err != nil || len(l) == 0 {
		return UsageTotals{Key: strconv.FormatInt(taskID, 10)}, err
	}
	l[0].Key = strconv.FormatInt(taskID, 10)
	return l[0], nil
}
