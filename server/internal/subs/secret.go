package subs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"agenthub/internal/providers"
	"agenthub/internal/secretbox"
	"agenthub/internal/store"
)

// Doer sends HTTP requests (swapped in tests for a fake router).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// PutSecret stores an API key write-only (AES-GCM in the hub, D5) and returns only its fingerprint.
func (s *Service) PutSecret(ctx context.Context, p *store.Principal, reqID, subID, value string) (any, error) {
	if len(strings.TrimSpace(value)) < 8 {
		return nil, store.ErrInvalid("secret looks empty")
	}
	fp := secretbox.Fp([]byte(value))
	return s.St.Write(ctx, p, reqID, "secret_put", map[string]string{"sub": subID, "fp": fp}, func(w *store.W) (any, error) {
		if err := w.Human("secret.put"); err != nil {
			return nil, err
		}
		var kind, label string
		var old *string
		if err := w.Row(`SELECT kind, label, secret_id::text FROM subscriptions WHERE id::text=$1 FOR UPDATE`, subID).Scan(&kind, &label, &old); err != nil {
			return nil, store.ErrNotFound("subscription " + subID)
		}
		if kind != "api_key" {
			return nil, store.Fail(422, "cli_login_rule", "cli_login subscriptions never hold a credential")
		}
		var id string
		if err := w.Row(`SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
			return nil, err
		}
		ct, err := s.Box.Seal([]byte(value), []byte("secret:"+id))
		if err != nil {
			return nil, err
		}
		if _, err := w.Exec(`INSERT INTO secrets(id,owner,label,ciphertext,fp) VALUES($1::uuid,$2,$3,$4,$5)`, id, w.P.ID, label, ct, fp); err != nil {
			return nil, err
		}
		if old != nil {
			if _, err := w.Exec(`UPDATE secrets SET revoked_at=now(), rotated_at=now() WHERE id::text=$1`, *old); err != nil {
				return nil, err
			}
		}
		if _, err := w.Exec(`UPDATE subscriptions SET secret_id=$2::uuid, version=version+1 WHERE id::text=$1`, subID, id); err != nil {
			return nil, err
		}
		return map[string]string{"fp": fp}, w.Event("subscription.secret_set", nil, nil, map[string]any{"subscription": subID, "fp": fp, "rotated": old != nil})
	})
}

// Test makes a one-token call to the provider (direct or through its router) with the stored key; ok or a
// redacted error. The target comes from the provider row's adapter (providers.Resolve).
func (s *Service) Test(ctx context.Context, p *store.Principal, reqID, subID string) (any, error) {
	if !p.IsHuman {
		return nil, store.Fail(403, "humans_only", "only humans test subscriptions")
	}
	var kind string
	var adapter []byte
	var model *string
	var ct []byte
	var secretID *string
	err := s.St.DB.QueryRow(ctx, `SELECT s.kind, pr.adapter, (SELECT model FROM provider_models m WHERE m.provider_id=pr.id ORDER BY position, model LIMIT 1),
		x.ciphertext, x.id::text FROM subscriptions s JOIN providers pr ON pr.id=s.provider_id
		LEFT JOIN secrets x ON x.id=s.secret_id AND x.revoked_at IS NULL WHERE s.id::text=$1`, subID).Scan(&kind, &adapter, &model, &ct, &secretID)
	if err != nil {
		return nil, store.ErrNotFound("subscription " + subID)
	}
	var a *providers.Adapter
	_ = json.Unmarshal(adapter, &a)
	target, ok := providers.Resolve(a, func(id string) string {
		var u string
		_ = s.St.DB.QueryRow(ctx, `SELECT url FROM router_endpoints WHERE id=$1`, id).Scan(&u)
		return u
	})
	if kind != "api_key" || secretID == nil || !ok || model == nil {
		return nil, store.Fail(422, "not_testable", "only an api_key subscription with a key, an API adapter and a cached model can be tested here")
	}
	key, err := s.Box.OpenBytes(ct, []byte("secret:"+*secretID))
	if err != nil {
		return nil, store.Fail(500, "secret_unreadable", "the stored key could not be opened (key file changed?)")
	}
	errText := s.call(ctx, target, *model, string(key))
	if errText != "" {
		errText = strings.ReplaceAll(errText, string(key), "[redacted]")
	}
	_, err = s.St.Write(ctx, p, reqID, "subscription_test", subID, func(w *store.W) (any, error) {
		if _, err := w.Exec(`UPDATE subscriptions SET last_test_ok_at=CASE WHEN $2='' THEN now() ELSE last_test_ok_at END, last_test_error=nullif($2,'') WHERE id::text=$1`, subID, errText); err != nil {
			return nil, err
		}
		return nil, w.Event("subscription.tested", nil, nil, map[string]any{"subscription": subID, "ok": errText == "", "router": target.Router})
	})
	return map[string]any{"ok": errText == "", "error": errText}, err
}
