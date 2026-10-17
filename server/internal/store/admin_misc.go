package store

import (
	"context"
	"encoding/json"
)

// SettingRow is one editable behaviour knob.
type SettingRow struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	Version     int64           `json:"version"`
}

// PutSetting updates a setting value (optimistic on version; 0 creates).
func (s *Store) PutSetting(ctx context.Context, p *Principal, reqID string, in SettingRow) (any, error) {
	return s.write(ctx, p, reqID, "put_setting", in, func(w *W) (any, error) {
		if err := w.admin("settings.put"); err != nil {
			return nil, err
		}
		var v any
		if json.Unmarshal(in.Value, &v) != nil {
			return nil, errInvalid("value must be JSON")
		}
		var cur int64
		err := w.row(`SELECT version FROM settings WHERE key=$1 FOR UPDATE`, in.Key).Scan(&cur)
		switch {
		case err != nil && in.Version == 0:
			_, err = w.exec(`INSERT INTO settings(key,value,description) VALUES($1,$2,$3)`, in.Key, in.Value, in.Description)
		case err != nil:
			return nil, errNotFound("setting " + in.Key)
		case cur != in.Version:
			return nil, errVersion(map[string]any{"version": cur})
		default:
			_, err = w.exec(`UPDATE settings SET value=$2, description=coalesce(nullif($3,''),description), version=version+1 WHERE key=$1`,
				in.Key, in.Value, in.Description)
		}
		if err != nil {
			return nil, err
		}
		return in, w.Event("admin.setting.put", nil, nil, map[string]any{"key": in.Key, "value": in.Value})
	})
}

// DeleteConfig removes a role, policy, workflow or project when nothing still refers to it.
func (s *Store) DeleteConfig(ctx context.Context, p *Principal, reqID, kind, id string) (any, error) {
	table := map[string]string{"roles": "roles", "policies": "policies", "workflows": "workflows", "projects": "projects"}[kind]
	return s.write(ctx, p, reqID, "delete_"+kind, id, func(w *W) (any, error) {
		if table == "" {
			return nil, errNotFound(kind)
		}
		if err := w.admin(kind + ".delete"); err != nil {
			return nil, err
		}
		inUse := map[string]string{
			"roles":     `SELECT EXISTS(SELECT 1 FROM principals WHERE $1=ANY(roles))`,
			"workflows": `SELECT EXISTS(SELECT 1 FROM projects WHERE workflow_id=$1)`,
			"projects":  `SELECT EXISTS(SELECT 1 FROM tasks WHERE project=$1)`,
		}[kind]
		if inUse != "" {
			var used bool
			if err := w.row(inUse, id).Scan(&used); err != nil || used {
				return nil, fail(409, "in_use", "%s %q is still referenced", kind, id)
			}
		}
		col := "id"
		if kind == "policies" {
			col = "id::text"
		}
		tag, err := w.exec(`DELETE FROM `+table+` WHERE `+col+`=$1`, id)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, errNotFound(kind + " " + id)
		}
		return map[string]any{"deleted": id}, w.Event("admin."+kind+".delete", nil, nil, map[string]any{"id": id})
	})
}
