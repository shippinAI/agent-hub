package store

import (
	"encoding/json"
	"time"
)

// Principal is an agent or a human. Roles and limits are data.
type Principal struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Kind        string     `json:"kind"`
	IsHuman     bool       `json:"is_human"`
	Login       *string    `json:"login"`
	Roles       []string   `json:"roles"`
	Enabled     bool       `json:"enabled"`
	WIPLimit    *int       `json:"wip_limit"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
	Version     int64      `json:"version"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Task is a unit of work; holder/lease/fence describe the current claim.
type Task struct {
	ID             int64           `json:"id"`
	Project        string          `json:"project"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	State          string          `json:"state"`
	Priority       int             `json:"priority"`
	Labels         []string        `json:"labels"`
	Assignee       *string         `json:"assignee"`
	Holder         *string         `json:"holder"`
	LeaseID        *string         `json:"lease_id"`
	LeaseTTL       *int            `json:"lease_ttl_seconds"`
	LeaseExpiresAt *time.Time      `json:"lease_expires_at"`
	HeartbeatAt    *time.Time      `json:"heartbeat_at"`
	Fence          int64           `json:"fence"`
	Version        int64           `json:"version"`
	ExpiryCount    int             `json:"expiry_count"`
	LastWorker     *string         `json:"last_worker"`
	Progress       json.RawMessage `json:"progress"`
	CreatedBy      string          `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Ready          bool            `json:"ready"`
	LeaseExpired   bool            `json:"lease_expired"`
	BlockedBy      []int64         `json:"blocked_by"`
}

// Note is an append-only comment, handoff, blocker or decision on a task.
type Note struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"task_id"`
	Author      string    `json:"author"`
	Kind        string    `json:"kind"`
	Body        string    `json:"body"`
	ToPrincipal *string   `json:"to_principal"`
	Fence       *int64    `json:"fence"`
	CreatedAt   time.Time `json:"created_at"`
}

// Event is one row of the hash-chained audit log.
type Event struct {
	Seq       int64           `json:"seq"`
	At        time.Time       `json:"at"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	TaskID    *int64          `json:"task_id"`
	Data      json.RawMessage `json:"data"`
	Fence     *int64          `json:"fence"`
	RequestID *string         `json:"request_id"`
	PrevHash  string          `json:"prev_hash"`
	Hash      string          `json:"hash"`
}

// Lease is what a claim hands back to the agent.
type Lease struct {
	Task           Task      `json:"task"`
	Fence          int64     `json:"fence"`
	LeaseID        string    `json:"lease_id"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
	HeartbeatEvery int       `json:"heartbeat_every_seconds"`
	AlreadyHeld    bool      `json:"already_held"`
	Claimed        bool      `json:"claimed"`
}
