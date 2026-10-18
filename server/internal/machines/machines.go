// Package machines holds enrolled machines (bridge principals), enrollment codes and honest presence.
package machines

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"regexp"
	"strings"

	"agenthub/internal/secretbox"
	"agenthub/internal/store"
)

// Service is the machine store.
type Service struct {
	St  *store.Store
	Box *secretbox.Box
}

// CreateIn is what Add machine collects.
type CreateIn struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	OS              string   `json:"os"`
	Kind            string   `json:"kind"`
	HostMachineID   *string  `json:"host_machine_id"`
	ShellProfileIDs []string `json:"shell_profile_ids"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// newCode returns "<pub>-<secret>" and the secret's hash; only the hash is stored.
func newCode() (pub, full string, hash []byte) {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	pub, secret := enc[:6], enc[6:]
	h := sha256.Sum256([]byte(secret))
	return pub, pub + "-" + secret, h[:]
}

// Create adds a machine row, its machine principal and a one-time enrollment code (returned once).
func (s *Service) Create(ctx context.Context, p *store.Principal, reqID string, in CreateIn) (any, error) {
	return s.St.Write(ctx, p, reqID, "machine_create", in, func(w *store.W) (any, error) {
		if err := w.Human("machine.enroll"); err != nil {
			return nil, err
		}
		if in.ID == "" {
			in.ID = slug(in.Label)
		}
		if in.Label == "" || in.ID == "" || in.OS == "" {
			return nil, store.ErrInvalid("label and os are required")
		}
		st, _ := w.Settings()
		if in.Kind == "" {
			in.Kind = "native"
		}
		if kinds := st.Strings("machine_kinds"); len(kinds) > 0 && !contains(kinds, in.Kind) {
			return nil, store.ErrInvalid("kind must be one of %v", kinds)
		}
		if in.Kind == "wsl" && (in.HostMachineID == nil || in.OS != "linux") {
			return nil, store.ErrInvalid("a WSL machine is os linux with host_machine_id set to the Windows machine (D2)")
		}
		if in.ShellProfileIDs == nil {
			in.ShellProfileIDs = []string{"direct"}
		}
		pid := "m-" + in.ID
		if _, err := w.Exec(`INSERT INTO principals(id,display_name,kind,is_human,roles,enabled) VALUES($1,$2,'machine',false,'{}',true)`,
			pid, "Machine: "+in.Label); err != nil {
			return nil, store.Fail(409, "exists", "machine %q already exists", in.ID)
		}
		if _, err := w.Exec(`INSERT INTO machines(id,principal_id,label,os,kind,host_machine_id,shell_profile_ids,enrolled_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
			in.ID, pid, in.Label, in.OS, in.Kind, in.HostMachineID, in.ShellProfileIDs, w.P.ID); err != nil {
			return nil, store.ErrInvalid("machine: %v", err)
		}
		code, err := issueCode(w, in.ID, st)
		if err != nil {
			return nil, err
		}
		if err := w.Event("machine.created", nil, nil, map[string]any{"machine": in.ID, "os": in.OS, "kind": in.Kind}); err != nil {
			return nil, err
		}
		return map[string]any{"machine": in, "enrollment": code}, nil
	})
}

func issueCode(w *store.W, machineID string, st store.Settings) (map[string]any, error) {
	pub, full, hash := newCode()
	mins := st.Int("enroll_code_minutes", 10)
	var exp any
	if err := w.Row(`INSERT INTO machine_enroll_codes(pub,machine_id,hash,expires_at,created_by) VALUES($1,$2,$3,now()+make_interval(mins=>$4),$5)
		RETURNING expires_at`, pub, machineID, hash, mins, w.P.ID).Scan(&exp); err != nil {
		return nil, err
	}
	if err := w.Event("machine.enroll_code", nil, nil, map[string]any{"machine": machineID, "code_prefix": pub}); err != nil {
		return nil, err
	}
	return map[string]any{"code": full, "code_prefix": pub, "expires_at": exp, "shown_once": true}, nil
}

// NewCode replaces any unused code of a machine with a fresh one.
func (s *Service) NewCode(ctx context.Context, p *store.Principal, reqID, machineID string) (any, error) {
	return s.St.Write(ctx, p, reqID, "machine_code", machineID, func(w *store.W) (any, error) {
		if err := w.Human("machine.enroll"); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`DELETE FROM machine_enroll_codes WHERE machine_id=$1 AND used_at IS NULL`, machineID); err != nil {
			return nil, err
		}
		var ok bool
		if w.Row(`SELECT true FROM machines WHERE id=$1 AND NOT disabled`, machineID).Scan(&ok); !ok {
			return nil, store.ErrNotFound("machine " + machineID)
		}
		st, _ := w.Settings()
		return issueCode(w, machineID, st)
	})
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// GUIReady stores the per-permission state the GUI helper reports (shown on the machine page).
func (s *Service) GUIReady(ctx context.Context, machineID string, ready map[string]string, display string) error {
	b, _ := json.Marshal(ready)
	_, err := s.St.DB.Exec(ctx, `UPDATE machines SET gui_ready=$2, display_server=nullif($3,'') WHERE id=$1`, machineID, b, display)
	return err
}
