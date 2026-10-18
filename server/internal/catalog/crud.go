package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"agenthub/internal/store"
)

// Service is the catalog API over the hub store.
type Service struct{ St *store.Store }

// List returns every row of a kind as JSON objects, ordered by id.
func (s *Service) List(ctx context.Context, name string) ([]json.RawMessage, error) {
	k, ok := kinds[name]
	if !ok {
		return nil, store.ErrNotFound("catalog " + name)
	}
	rows, err := s.St.DB.Query(ctx, `SELECT to_jsonb(t) FROM `+k.Table+` t ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Put creates (version 0) or updates (matching version) one row. Only the columns present in the body change.
func (s *Service) Put(ctx context.Context, p *store.Principal, reqID, name, id string, body map[string]json.RawMessage) (any, error) {
	k, ok := kinds[name]
	if !ok {
		return nil, store.ErrNotFound("catalog " + name)
	}
	var version int64
	_ = json.Unmarshal(body["version"], &version)
	return s.St.Write(ctx, p, reqID, "catalog_put_"+name, map[string]any{"id": id, "body": body}, func(w *store.W) (any, error) {
		if err := w.Human(k.Action + ".put"); err != nil {
			return nil, err
		}
		if id == "" || len(id) > 64 {
			return nil, store.ErrInvalid("id required (max 64 chars)")
		}
		if err := validate(w, name, body); err != nil {
			return nil, err
		}
		names, args, err := values(k, body)
		if err != nil {
			return nil, err
		}
		var cur int64
		err = w.Row(`SELECT version FROM `+k.Table+` WHERE id=$1 FOR UPDATE`, id).Scan(&cur)
		switch {
		case err != nil && version == 0:
			ph := make([]string, len(names)+1)
			for i := range ph {
				ph[i] = fmt.Sprintf("$%d", i+1)
			}
			_, err = w.Exec(`INSERT INTO `+k.Table+`(id,`+strings.Join(names, ",")+`) VALUES(`+strings.Join(ph, ",")+`)`,
				append([]any{id}, args...)...)
		case err != nil:
			return nil, store.ErrNotFound(name + " " + id)
		case cur != version:
			return nil, store.ErrVersion(map[string]any{"version": cur})
		default:
			sets := []string{"version=version+1"}
			for i, n := range names {
				sets = append(sets, fmt.Sprintf("%s=$%d", n, i+2))
			}
			_, err = w.Exec(`UPDATE `+k.Table+` SET `+strings.Join(sets, ",")+` WHERE id=$1`, append([]any{id}, args...)...)
		}
		if err != nil {
			return nil, dbErr(err)
		}
		var row json.RawMessage
		if err := w.Row(`SELECT to_jsonb(t) FROM `+k.Table+` t WHERE id=$1`, id).Scan(&row); err != nil {
			return nil, err
		}
		return row, w.Event("catalog."+name+".put", nil, nil, map[string]any{"id": id, "fields": names})
	})
}

// Delete removes a row when nothing still refers to it.
func (s *Service) Delete(ctx context.Context, p *store.Principal, reqID, name, id string) (any, error) {
	k, ok := kinds[name]
	if !ok {
		return nil, store.ErrNotFound("catalog " + name)
	}
	return s.St.Write(ctx, p, reqID, "catalog_delete_"+name, id, func(w *store.W) (any, error) {
		if err := w.Human(k.Action + ".delete"); err != nil {
			return nil, err
		}
		if name == "grant-presets" {
			var def bool
			if w.Row(`SELECT is_default FROM grant_presets WHERE id=$1`, id).Scan(&def); def {
				return nil, store.Fail(409, "in_use", "the default preset cannot be deleted; pick another default first")
			}
		}
		if name == "router-endpoints" {
			var users int
			_ = w.Row(`SELECT count(*) FROM providers p, jsonb_array_elements(coalesce(jsonb_build_array(p.adapter), '[]') || p.fallback_adapters) a
				WHERE a->'config'->'router'->>'endpoint_id' = $1`, id).Scan(&users)
			if users > 0 {
				return nil, store.Fail(409, "in_use", "%d provider adapter(s) route through %s; switch them to another router first", users, id)
			}
		}
		if name == "tenants" && id == "local" {
			return nil, store.Fail(409, "in_use", "the local tenant is required")
		}
		tag, err := w.Exec(`DELETE FROM `+k.Table+` WHERE id=$1`, id)
		if err != nil {
			return nil, dbErr(err)
		}
		if tag.RowsAffected() == 0 {
			return nil, store.ErrNotFound(name + " " + id)
		}
		return map[string]any{"deleted": id}, w.Event("catalog."+name+".delete", nil, nil, map[string]any{"id": id})
	})
}

func dbErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23503":
			return store.Fail(409, "in_use", "still referenced: %s", pg.ConstraintName)
		case "23505", "23514", "22P02", "23502":
			return store.ErrInvalid("%s", pg.Message)
		}
	}
	return err
}
