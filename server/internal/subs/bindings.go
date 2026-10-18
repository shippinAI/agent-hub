package subs

import (
	"context"

	"agenthub/internal/store"
)

// Binding maps (role, chat app) to a subscription with priority and an optional fallback.
type Binding struct {
	ID       string  `json:"id"`
	RoleID   string  `json:"role_id"`
	ChatApp  string  `json:"chat_app"`
	SubID    string  `json:"subscription_id"`
	Priority int     `json:"priority"`
	Fallback *string `json:"fallback_subscription_id"`
	Enabled  bool    `json:"enabled"`
	Version  int64   `json:"version"`
}

// Bindings lists role bindings in routing order.
func (s *Service) Bindings(ctx context.Context) ([]Binding, error) {
	rows, err := s.St.DB.Query(ctx, `SELECT id::text,role_id,chat_app,subscription_id::text,priority,fallback_subscription_id::text,enabled,version
		FROM role_bindings ORDER BY role_id, priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Binding{}
	for rows.Next() {
		var b Binding
		if err := rows.Scan(&b.ID, &b.RoleID, &b.ChatApp, &b.SubID, &b.Priority, &b.Fallback, &b.Enabled, &b.Version); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// PutBinding creates (Version 0) or updates a binding.
func (s *Service) PutBinding(ctx context.Context, p *store.Principal, reqID string, b Binding) (any, error) {
	return s.St.Write(ctx, p, reqID, "role_binding_put", b, func(w *store.W) (any, error) {
		if err := w.Human("role_binding.put"); err != nil {
			return nil, err
		}
		var id string
		var err error
		if b.Version == 0 {
			err = w.Row(`INSERT INTO role_bindings(owner,role_id,chat_app,subscription_id,priority,fallback_subscription_id,enabled)
				VALUES($1,$2,$3,$4::uuid,$5,$6::uuid,true) RETURNING id::text`, w.P.ID, b.RoleID, b.ChatApp, b.SubID, b.Priority, b.Fallback).Scan(&id)
			if err != nil {
				return nil, store.ErrInvalid("%v", err)
			}
		} else if err = w.Row(`UPDATE role_bindings SET role_id=$2,chat_app=$3,subscription_id=$4::uuid,priority=$5,fallback_subscription_id=$6::uuid,enabled=$7,
			version=version+1 WHERE id::text=$1 AND version=$8 RETURNING id::text`, b.ID, b.RoleID, b.ChatApp, b.SubID, b.Priority, b.Fallback, b.Enabled, b.Version).Scan(&id); err != nil {
			return nil, store.ErrVersion(nil)
		}
		return map[string]any{"id": id}, w.Event("role_binding.saved", nil, nil, map[string]any{"binding": id, "role": b.RoleID, "chat_app": b.ChatApp})
	})
}

// Delete removes a subscription or role binding (kind "subscriptions" | "role_bindings").
func (s *Service) Delete(ctx context.Context, p *store.Principal, reqID, kind, id string) (any, error) {
	table := map[string]string{"subscriptions": "subscriptions", "role-bindings": "role_bindings"}[kind]
	return s.St.Write(ctx, p, reqID, "delete_"+kind, id, func(w *store.W) (any, error) {
		if table == "" {
			return nil, store.ErrNotFound(kind)
		}
		if err := w.Human(map[string]string{"subscriptions": "subscription.put", "role_bindings": "role_binding.put"}[table]); err != nil {
			return nil, err
		}
		if table == "subscriptions" {
			if _, err := w.Exec(`UPDATE secrets SET revoked_at=now() WHERE id=(SELECT secret_id FROM subscriptions WHERE id::text=$1)`, id); err != nil {
				return nil, err
			}
		}
		tag, err := w.Exec(`DELETE FROM `+table+` WHERE id::text=$1`, id)
		if err != nil {
			return nil, store.Fail(409, "in_use", "%v", err)
		}
		if tag.RowsAffected() == 0 {
			return nil, store.ErrNotFound(id)
		}
		return map[string]any{"deleted": id}, w.Event(table+".deleted", nil, nil, map[string]any{"id": id})
	})
}
