// Package subs is subscriptions, write-only secrets, role bindings and role routing (05). The hub never
// stores, proxies or replays a consumer subscription credential; cli_login rows carry no secret at all.
package subs

import (
	"context"
	"encoding/json"
	"slices"

	"agenthub/internal/secretbox"
	"agenthub/internal/store"
)

// Service is the subscriptions domain.
type Service struct {
	St   *store.Store
	Box  *secretbox.Box
	HTTP Doer
}

// Sub is a subscription as every API returns it: never the secret, only its fingerprint.
type Sub struct {
	ID         string          `json:"id"`
	Owner      string          `json:"owner"`
	ProviderID string          `json:"provider_id"`
	Label      string          `json:"label"`
	Kind       string          `json:"kind"`
	Worker     *string         `json:"worker_principal_id"`
	MachineID  *string         `json:"machine_id"`
	Budget     json.RawMessage `json:"budget"`
	Spend      json.RawMessage `json:"spend"`
	Enabled    bool            `json:"enabled"`
	Version    int64           `json:"version"`
	SecretFp   *string         `json:"secret_fp"`
	LastOK     *string         `json:"last_test_ok_at"`
	LastError  *string         `json:"last_test_error"`
}

const subCols = `s.id::text,s.owner,s.provider_id,s.label,s.kind,s.worker_principal_id,s.machine_id,s.budget,s.spend,s.enabled,s.version,
	(SELECT fp FROM secrets x WHERE x.id=s.secret_id AND x.revoked_at IS NULL),s.last_test_ok_at::text,s.last_test_error`

func scanSub(r interface{ Scan(...any) error }) (Sub, error) {
	var s Sub
	err := r.Scan(&s.ID, &s.Owner, &s.ProviderID, &s.Label, &s.Kind, &s.Worker, &s.MachineID, &s.Budget, &s.Spend, &s.Enabled, &s.Version, &s.SecretFp, &s.LastOK, &s.LastError)
	return s, err
}

// List returns subscriptions.
func (s *Service) List(ctx context.Context) ([]Sub, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT `+subCols+` FROM subscriptions s ORDER BY s.label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sub{}
	for rows.Next() {
		x, err := scanSub(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Input creates or edits a subscription (Version 0 creates).
type Input struct {
	ID         string          `json:"id"`
	ProviderID string          `json:"provider_id"`
	Label      string          `json:"label"`
	Kind       string          `json:"kind"`
	Worker     string          `json:"worker_principal_id"`
	MachineID  string          `json:"machine_id"`
	Budget     json.RawMessage `json:"budget"`
	Enabled    *bool           `json:"enabled"`
	Version    int64           `json:"version"`
}

// Put creates or updates a subscription with the code rule: cli_login needs a machine and an agent worker
// principal (it runs on that machine) and a provider whose auth_kinds offer a client sign-in; api_key needs a
// provider that takes an API or plan key. Disabled providers cannot be picked.
func (s *Service) Put(ctx context.Context, p *store.Principal, reqID string, in Input) (any, error) {
	return s.St.Write(ctx, p, reqID, "subscription_put", in, func(w *store.W) (any, error) {
		if err := w.Human("subscription.put"); err != nil {
			return nil, err
		}
		var auth []string
		var disabled bool
		if err := w.Row(`SELECT auth_kinds, disabled_at IS NOT NULL FROM providers WHERE id=$1`, in.ProviderID).Scan(&auth, &disabled); err != nil {
			return nil, store.ErrNotFound("provider " + in.ProviderID)
		}
		if disabled {
			return nil, store.Fail(422, "provider_disabled", "provider %s is disabled; enable it under Catalog > Providers first", in.ProviderID)
		}
		if err := rule(w, in, auth); err != nil {
			return nil, err
		}
		en := in.Enabled == nil || *in.Enabled
		var id string
		var err error
		if in.Version == 0 {
			err = w.Row(`INSERT INTO subscriptions(owner,provider_id,label,kind,worker_principal_id,machine_id,budget,enabled)
				VALUES($1,$2,$3,$4,nullif($5,''),nullif($6,''),$7,$8) RETURNING id::text`, w.P.ID, in.ProviderID, in.Label, in.Kind, in.Worker, in.MachineID, nullJSON(in.Budget), en).Scan(&id)
		} else {
			err = w.Row(`UPDATE subscriptions SET provider_id=$2,label=$3,kind=$4,worker_principal_id=nullif($5,''),machine_id=nullif($6,''),budget=$7,enabled=$8,
				version=version+1 WHERE id::text=$1 AND version=$9 RETURNING id::text`, in.ID, in.ProviderID, in.Label, in.Kind, in.Worker, in.MachineID, nullJSON(in.Budget), en, in.Version).Scan(&id)
			if err != nil {
				return nil, store.ErrVersion(nil)
			}
		}
		if err != nil {
			return nil, store.ErrInvalid("%v", err)
		}
		return map[string]any{"id": id}, w.Event("subscription.saved", nil, nil, map[string]any{"subscription": id, "kind": in.Kind, "provider": in.ProviderID})
	})
}

func rule(w *store.W, in Input, auth []string) error {
	switch in.Kind {
	case "cli_login":
		if !slices.Contains(auth, "client_login") && !slices.Contains(auth, "signin_oauth") {
			return store.Fail(422, "provider_kind", "cli_login needs a provider with client_login or signin_oauth in auth_kinds")
		}
		if in.MachineID == "" || in.Worker == "" {
			return store.Fail(422, "cli_login_rule", "a cli_login subscription names the machine and the worker principal that runs the official CLI there")
		}
		var human bool
		var kind string
		if err := w.Row(`SELECT is_human, kind FROM principals WHERE id=$1`, in.Worker).Scan(&human, &kind); err != nil || human || kind == "machine" {
			return store.Fail(422, "cli_login_rule", "the worker must be an agent principal")
		}
	case "api_key":
		if !slices.Contains(auth, "api_key") && !slices.Contains(auth, "plan_key") {
			return store.Fail(422, "provider_kind", "api_key needs a provider with api_key or plan_key in auth_kinds")
		}
	default:
		return store.ErrInvalid("kind must be api_key or cli_login")
	}
	return nil
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return []byte(b)
}
