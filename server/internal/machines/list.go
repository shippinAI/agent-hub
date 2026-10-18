package machines

import (
	"context"
	"encoding/json"
	"time"

	"agenthub/internal/store"
)

// Machine is a machine row with derived presence. Online is only ever derived from a completed call.
type Machine struct {
	ID              string          `json:"id"`
	PrincipalID     string          `json:"principal_id"`
	Label           string          `json:"label"`
	OS              string          `json:"os"`
	Kind            string          `json:"kind"`
	HostMachineID   *string         `json:"host_machine_id"`
	Arch            string          `json:"arch"`
	BridgeVersion   string          `json:"bridge_version"`
	SandboxKind     string          `json:"sandbox_kind"`
	DisplayServer   *string         `json:"display_server"`
	GUIReady        json.RawMessage `json:"gui_ready"`
	ShellProfileIDs []string        `json:"shell_profile_ids"`
	EnrolledBy      string          `json:"enrolled_by"`
	EnrolledAt      *time.Time      `json:"enrolled_at"`
	LastExecOKAt    *time.Time      `json:"last_exec_ok_at"`
	LastExecErrAt   *time.Time      `json:"last_exec_err_at"`
	LastError       *string         `json:"last_error"`
	LastPollAt      *time.Time      `json:"last_poll_at"`
	Connected       bool            `json:"connected"`
	Online          bool            `json:"online"`
	Disabled        bool            `json:"disabled"`
	GUIPaused       bool            `json:"gui_paused"`
	LiveGrants      int             `json:"live_grants"`
	HighRiskGrants  int             `json:"high_risk_grants"`
	Version         int64           `json:"version"`
}

const cols = `m.id,m.principal_id,m.label,m.os,m.kind,m.host_machine_id,m.arch,m.bridge_version,m.sandbox_kind,m.display_server,m.gui_ready,
	m.shell_profile_ids,m.enrolled_by,m.enrolled_at,m.last_exec_ok_at,m.last_exec_err_at,m.last_error,m.last_poll_at,
	(m.connected OR coalesce(m.last_poll_at > now() - make_interval(secs => 2*$1), false)),
	coalesce(m.last_exec_ok_at > now() - make_interval(secs => $2) AND (m.last_exec_err_at IS NULL OR m.last_exec_err_at < m.last_exec_ok_at), false),
	m.disabled, m.gui_paused,
	(SELECT count(*) FROM grants g WHERE g.machine_id=m.id AND g.ended_at IS NULL),
	(SELECT count(*) FROM grants g WHERE g.machine_id=m.id AND g.ended_at IS NULL AND (g.sandbox_level='unsandboxed' OR g.full_access OR g.capability LIKE 'gui.%')),
	m.version`

func scan(r interface{ Scan(...any) error }) (Machine, error) {
	var m Machine
	err := r.Scan(&m.ID, &m.PrincipalID, &m.Label, &m.OS, &m.Kind, &m.HostMachineID, &m.Arch, &m.BridgeVersion, &m.SandboxKind, &m.DisplayServer,
		&m.GUIReady, &m.ShellProfileIDs, &m.EnrolledBy, &m.EnrolledAt, &m.LastExecOKAt, &m.LastExecErrAt, &m.LastError, &m.LastPollAt,
		&m.Connected, &m.Online, &m.Disabled, &m.GUIPaused, &m.LiveGrants, &m.HighRiskGrants, &m.Version)
	return m, err
}

func windows(ctx context.Context, st *store.Store) (int, int) {
	s, _ := st.LoadSettings(ctx)
	return s.Int("bridge_poll_seconds", 25), s.Int("online_window_seconds", 120)
}

// List returns every machine with derived presence.
func (s *Service) List(ctx context.Context) ([]Machine, error) {
	poll, online := windows(ctx, s.St)
	rows, err := s.St.DB.Query(ctx, `SELECT `+cols+` FROM machines m ORDER BY m.disabled, m.label`, poll, online)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Machine{}
	for rows.Next() {
		m, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Get returns one machine.
func (s *Service) Get(ctx context.Context, id string) (Machine, error) {
	poll, online := windows(ctx, s.St)
	m, err := scan(s.St.DB.QueryRow(ctx, `SELECT `+cols+` FROM machines m WHERE m.id=$3`, poll, online, id))
	if err != nil {
		return m, store.ErrNotFound("machine " + id)
	}
	return m, nil
}

// ByPrincipal maps a bridge's token principal to its machine.
func (s *Service) ByPrincipal(ctx context.Context, principalID string) (Machine, error) {
	poll, online := windows(ctx, s.St)
	m, err := scan(s.St.DB.QueryRow(ctx, `SELECT `+cols+` FROM machines m WHERE m.principal_id=$3`, poll, online, principalID))
	if err != nil {
		return m, store.ErrNotFound("machine for principal")
	}
	return m, nil
}

// Seen records that the bridge is connected (socket or poll). It never sets online (01 invariant 5).
func (s *Service) Seen(ctx context.Context, id string, connected bool) {
	_, _ = s.St.DB.Exec(ctx, `UPDATE machines SET last_poll_at=now(), connected=$2 WHERE id=$1`, id, connected)
}

// CallDone records the outcome of an executed call; only a completed call moves last_exec_ok_at.
func (s *Service) CallDone(ctx context.Context, id string, completed bool, errText string) {
	if completed {
		_, _ = s.St.DB.Exec(ctx, `UPDATE machines SET last_exec_ok_at=now() WHERE id=$1`, id)
		return
	}
	if len(errText) > 300 {
		errText = errText[:300]
	}
	_, _ = s.St.DB.Exec(ctx, `UPDATE machines SET last_exec_err_at=now(), last_error=$2 WHERE id=$1`, id, errText)
}
