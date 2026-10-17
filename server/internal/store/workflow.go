package store

import (
	"encoding/json"
	"fmt"
	"slices"
)

// State is one workflow column. Kind is what the safety rules reason about; Key/Label are data.
type State struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// Transition is a manual move allowed by the workflow (policy decides who may make it).
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Lane maps a claimable queue state to its active state and where complete/release go.
type Lane struct {
	Queue                string `json:"queue"`
	Active               string `json:"active"`
	CompleteTo           string `json:"complete_to"`
	ReleaseTo            string `json:"release_to"`
	DistinctFromPrevious bool   `json:"distinct_from_previous"`
}

// Workflow is a row of the workflows table.
type Workflow struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	States         []State      `json:"states"`
	Transitions    []Transition `json:"transitions"`
	Lanes          []Lane       `json:"lanes"`
	InitialState   string       `json:"initial_state"`
	AttentionState string       `json:"attention_state"`
	Version        int64        `json:"version"`
}

// StateKinds are the only fixed vocabulary; they are categories, not states.
var StateKinds = []string{"queue", "active", "review", "blocked", "done", "attention"}

func (wf *Workflow) state(key string) (State, bool) {
	for _, s := range wf.States {
		if s.Key == key {
			return s, true
		}
	}
	return State{}, false
}

func (wf *Workflow) laneByQueue(q string) (Lane, bool) {
	for _, l := range wf.Lanes {
		if l.Queue == q {
			return l, true
		}
	}
	return Lane{}, false
}

func (wf *Workflow) laneByActive(a string) (Lane, bool) {
	for _, l := range wf.Lanes {
		if l.Active == a {
			return l, true
		}
	}
	return Lane{}, false
}

func (wf *Workflow) allows(from, to string) bool {
	return slices.ContainsFunc(wf.Transitions, func(t Transition) bool {
		return (t.From == from || t.From == "*") && t.To == to
	})
}

// Validate checks internal consistency so a bad edit in the UI can't wedge the board.
func (wf *Workflow) Validate() error {
	if wf.ID == "" || wf.Name == "" || len(wf.States) == 0 {
		return errInvalid("workflow needs id, name and states")
	}
	seen := map[string]bool{}
	for _, s := range wf.States {
		if s.Key == "" || seen[s.Key] || !slices.Contains(StateKinds, s.Kind) {
			return errInvalid("state %q: keys must be unique and kind one of %v", s.Key, StateKinds)
		}
		seen[s.Key] = true
	}
	need := func(k, what string) error {
		if !seen[k] {
			return errInvalid("%s %q is not a state", what, k)
		}
		return nil
	}
	if err := need(wf.InitialState, "initial_state"); err != nil {
		return err
	}
	if err := need(wf.AttentionState, "attention_state"); err != nil {
		return err
	}
	for _, t := range wf.Transitions {
		if (t.From != "*" && !seen[t.From]) || !seen[t.To] {
			return errInvalid("transition %s->%s uses an unknown state", t.From, t.To)
		}
	}
	for i, l := range wf.Lanes {
		for _, k := range []string{l.Queue, l.Active, l.CompleteTo, l.ReleaseTo} {
			if err := need(k, fmt.Sprintf("lane %d", i)); err != nil {
				return err
			}
		}
		if s, _ := wf.state(l.Active); s.Kind != "active" {
			return errInvalid("lane %d: active state %q must have kind active", i, l.Active)
		}
	}
	return nil
}

func decodeWorkflow(id, name string, states, trans, lanes []byte, initial, attn string, ver int64) (Workflow, error) {
	wf := Workflow{ID: id, Name: name, InitialState: initial, AttentionState: attn, Version: ver}
	if err := json.Unmarshal(states, &wf.States); err != nil {
		return wf, err
	}
	if err := json.Unmarshal(trans, &wf.Transitions); err != nil {
		return wf, err
	}
	return wf, json.Unmarshal(lanes, &wf.Lanes)
}
