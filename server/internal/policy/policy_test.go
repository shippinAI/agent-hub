package policy

import "testing"

func TestDecide(t *testing.T) {
	rules := []Rule{
		{Name: "agents claim", Subject: "agent", Action: "task.claim:*", Effect: "allow", Priority: 10, Enabled: true},
		{Name: "human-only", Subject: "agent", Action: "task.claim:*", Effect: "deny", Priority: 100, Enabled: true,
			Selector: Selector{Labels: []string{"human-only"}}},
		{Name: "tie deny", Subject: "role:x", Action: "a", Effect: "deny", Priority: 5, Enabled: true},
		{Name: "tie allow", Subject: "role:x", Action: "a", Effect: "allow", Priority: 5, Enabled: true},
		{Name: "disabled", Subject: "*", Action: "b", Effect: "allow", Enabled: false},
	}
	agent := Subject{ID: "codex", Roles: []string{"x"}}
	cases := []struct {
		act  string
		res  Resource
		want bool
	}{
		{"task.claim:ready", Resource{}, true},
		{"task.claim:ready", Resource{Labels: []string{"human-only"}}, false},
		{"a", Resource{}, false},
		{"b", Resource{}, false},
		{"unknown", Resource{}, false},
	}
	for _, c := range cases {
		if got := Decide(rules, agent, c.act, c.res).Allowed; got != c.want {
			t.Errorf("%s %v: got %v", c.act, c.res, got)
		}
	}
	if !Glob("task.transition:*->done", "task.transition:review->done") || Glob("admin.*", "task.x") {
		t.Error("glob")
	}
}
