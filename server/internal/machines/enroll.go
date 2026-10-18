package machines

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"agenthub/internal/store"
)

// EnrollIn is what the bridge sends with the code (entered at a prompt, never argv).
type EnrollIn struct {
	Code          string `json:"code"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	BridgeVersion string `json:"bridge_version"`
	SandboxKind   string `json:"sandbox_kind"` // e.g. bubblewrap, seatbelt, windows-user, none (self-test failed)
	DisplayServer string `json:"display_server"`
}

// Enrolled is returned once to the bridge. The token is written to a 0600 file and never echoed.
type Enrolled struct {
	MachineID   string `json:"machine_id"`
	Token       string `json:"token"`
	PollSeconds int    `json:"bridge_poll_seconds"`
	CacheSecs   int    `json:"grant_cache_seconds"`
}

// SignKey derives the per-machine HMAC key for signed calls from the machine token (03 "Per-call checks").
// The hub computes it once at enrollment and keeps it sealed; the bridge derives it from its token file.
func SignKey(token string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("devagotchi call signing v1"))
	return m.Sum(nil)
}

// Enroll trades a valid code for a machine token. Wrong secrets count against the code (enroll_code_tries).
func (s *Service) Enroll(ctx context.Context, in EnrollIn) (Enrolled, error) {
	pub, secret, _ := strings.Cut(strings.ToUpper(strings.TrimSpace(in.Code)), "-")
	var fail error
	out, err := s.St.Write(ctx, store.System, "", "machine_enroll", nil, func(w *store.W) (any, error) {
		st, _ := w.Settings()
		var machineID string
		var hash []byte
		var tries int
		var live bool
		err := w.Row(`SELECT machine_id, hash, tries, (used_at IS NULL AND expires_at > now()) FROM machine_enroll_codes WHERE pub=$1 FOR UPDATE`, pub).
			Scan(&machineID, &hash, &tries, &live)
		if err != nil {
			fail = store.Fail(403, "bad_code", "enrollment code is not valid")
			return nil, nil
		}
		sum := sha256.Sum256([]byte(secret))
		maxTries := st.Int("enroll_code_tries", 5)
		if !live || tries >= maxTries || subtle.ConstantTimeCompare(sum[:], hash) != 1 {
			_, _ = w.Exec(`UPDATE machine_enroll_codes SET tries=tries+1 WHERE pub=$1`, pub)
			fail = store.Fail(403, "bad_code", "enrollment code is not valid (expired, used, or wrong)")
			return nil, w.Event("machine.enroll_failed", nil, nil, map[string]any{"machine": machineID, "code_prefix": pub, "tries": tries + 1})
		}
		var pid string
		if err := w.Row(`SELECT principal_id FROM machines WHERE id=$1 AND NOT disabled`, machineID).Scan(&pid); err != nil {
			fail = store.Fail(403, "bad_code", "machine is removed or paused")
			return nil, nil
		}
		tok := store.NewToken()
		if _, err := w.Exec(`UPDATE tokens SET revoked_at=now() WHERE principal_id=$1 AND revoked_at IS NULL`, pid); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`INSERT INTO tokens(principal_id,name,hash,prefix) VALUES($1,'bridge',$2,$3)`, pid, store.HashToken(tok), tok[:10]); err != nil {
			return nil, err
		}
		ct, err := s.Box.Seal(SignKey(tok), []byte("machine:"+machineID))
		if err != nil {
			return nil, err
		}
		sk := in.SandboxKind
		if sk == "" {
			sk = "none"
		}
		if _, err := w.Exec(`UPDATE machine_enroll_codes SET used_at=now() WHERE pub=$1`, pub); err != nil {
			return nil, err
		}
		if _, err := w.Exec(`UPDATE machines SET os=coalesce(nullif($2,''),os), arch=$3, bridge_version=$4, sandbox_kind=$5, display_server=nullif($6,''),
			enrolled_at=now(), sign_key_ct=$7, version=version+1 WHERE id=$1`, machineID, in.OS, in.Arch, in.BridgeVersion, sk, in.DisplayServer, ct); err != nil {
			return nil, err
		}
		if err := w.Event("machine.enrolled", nil, nil, map[string]any{"machine": machineID, "code_prefix": pub, "sandbox_kind": sk,
			"os": in.OS, "arch": in.Arch, "bridge_version": in.BridgeVersion, "token_prefix": tok[:10]}); err != nil {
			return nil, err
		}
		return Enrolled{MachineID: machineID, Token: tok, PollSeconds: st.Int("bridge_poll_seconds", 25), CacheSecs: CacheSeconds(st)}, nil
	})
	if err != nil {
		return Enrolled{}, err
	}
	if fail != nil {
		return Enrolled{}, fail
	}
	return out.(Enrolled), nil
}

// CacheSeconds is grant_cache_seconds with the code clamp (01 invariant 3: at most 30).
func CacheSeconds(st store.Settings) int {
	n := st.Int("grant_cache_seconds", 10)
	if n > 30 {
		n = 30
	}
	if n < 0 {
		n = 0
	}
	return n
}

// SignKeyFor opens the sealed key of a machine.
func (s *Service) SignKeyFor(ctx context.Context, machineID string) ([]byte, error) {
	var ct []byte
	if err := s.St.DB.QueryRow(ctx, `SELECT sign_key_ct FROM machines WHERE id=$1 AND sign_key_ct IS NOT NULL`, machineID).Scan(&ct); err != nil {
		return nil, store.Fail(409, "not_enrolled", "machine %s is not enrolled", machineID)
	}
	return s.Box.OpenBytes(ct, []byte("machine:"+machineID))
}
