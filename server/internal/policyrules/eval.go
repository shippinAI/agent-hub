package policyrules

import (
	"errors"
	"slices"
	"sort"
	"strings"
)

// Engine evaluates one Doc in one Env against one set of Facts.
type Engine struct {
	Doc   *Doc
	Env   Env
	Facts Facts
}

var runnerRe = re(`^\s*(?:sudo\s+(?:-n\s+)?)?(?:\S*/)?gold-policy\s+run\s+([A-Za-z0-9][A-Za-z0-9._-]{2,63})\s*$`)
var errNoFacts = errors.New("ticket store unavailable (failing closed)")
var ticketEnvRe = re(`(?:^|\s)GOLD_TICKET=([A-Za-z0-9][A-Za-z0-9._-]{2,63})(?:\s|$)`)

// Normalize fills TicketID / ViaRunner from the command text when the caller didn't.
// "gold-policy run <id>" is the runner; "GOLD_TICKET=<id> <cmd>" names a ticket for a raw command.
func Normalize(r Request) Request {
	if r.Event == "" {
		r.Event = "pre"
	}
	if m := runnerRe.FindStringSubmatch(r.Command); m != nil {
		r.ViaRunner, r.TicketID = true, m[1]
	} else if r.TicketID == "" {
		if m := ticketEnvRe.FindStringSubmatch(r.Command); m != nil {
			r.TicketID = m[1]
		}
	}
	return r
}

// Evaluate decides one request. Strictest wins: any failing rule denies, whatever else allowed.
func (e *Engine) Evaluate(r Request) Decision {
	r = Normalize(r)
	d := Decision{Allow: true, Version: e.Doc.Version, Params: map[string]int{}, Ticket: r.TicketID}
	var t *Ticket
	var terr error
	if r.TicketID != "" {
		if e.Facts == nil {
			terr = errNoFacts
		} else {
			t, terr = e.Facts.Ticket(r.TicketID)
		}
	}
	text := r.Command
	if r.ViaRunner && t != nil {
		text = strings.Join(t.Lists["commands"], "\n")
	}
	d.Classes = e.Doc.Classify(text)
	hosts := append(slices.Clone(r.Hosts), Hosts(text)...)
	var sites []string
	if t != nil {
		hosts = append(hosts, t.Lists["targets"]...)
		sites = append(sites, t.Fields["site"])
	}
	d.Regions = e.Doc.RegionsFor(hosts, sites)
	rules := slices.Clone(e.Doc.Rules)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority > rules[j].Priority })
	for _, rule := range rules {
		if !rule.Enabled || !matches(rule.When, r, text, d) {
			continue
		}
		d.Matched = append(d.Matched, rule.ID)
		for k, v := range rule.Params {
			if old, ok := d.Params[k]; !ok || v < old {
				d.Params[k] = v // strictest (smallest) window wins
			}
		}
		var why []string
		switch rule.Effect {
		case "deny":
			why = append(why, or(rule.Message, "denied by rule "+rule.ID))
		case "require":
			if terr != nil && r.TicketID != "" {
				why = append(why, terr.Error())
				break
			}
			for _, c := range rule.Require {
				if w := e.run(c, r, t); w != "" && !slices.Contains(why, w) {
					why = append(why, w)
				}
			}
		case "collect":
			d.Collect = append(d.Collect, rule.Collect...)
		case "allow", "audit":
		default:
			why = append(why, "unknown effect "+rule.Effect+" (failing closed)")
		}
		if len(why) > 0 {
			d.Allow = false
			d.Failures = append(d.Failures, Failure{Rule: rule.ID, Hook: rule.Hook, Message: rule.Message, Why: why})
		}
	}
	return d
}

func matches(w When, r Request, text string, d Decision) bool {
	ev := w.Events
	if len(ev) == 0 {
		ev = []string{"pre"}
	}
	if !slices.Contains(ev, r.Event) && !slices.Contains(ev, "*") {
		return false
	}
	if len(w.Tools) > 0 && !slices.Contains(w.Tools, "*") && !slices.ContainsFunc(w.Tools, func(p string) bool { return re("^(?:" + p + ")$").MatchString(r.Tool) }) {
		return false
	}
	if len(w.Classes) > 0 && !slices.ContainsFunc(w.Classes, func(c string) bool { return slices.Contains(d.Classes, c) }) {
		return false
	}
	if len(w.Regions) > 0 && !slices.ContainsFunc(w.Regions, func(c string) bool { return slices.Contains(d.Regions, c) }) {
		return false
	}
	if w.CommandRe != "" && !re(w.CommandRe).MatchString(text) {
		return false
	}
	return w.ViaRunner == nil || *w.ViaRunner == r.ViaRunner
}

// Reason renders a decision for a human or an agent (one line per failure).
func (d Decision) Reason() string {
	if d.Allow {
		return "allowed by policy v" + itoa(d.Version)
	}
	var b strings.Builder
	b.WriteString("BLOCKED by policy v" + itoa(d.Version) + ":")
	for _, f := range d.Failures {
		b.WriteString("\n- [" + f.Rule + "] " + f.Message)
		for _, w := range f.Why {
			b.WriteString("\n    * " + w)
		}
	}
	return b.String()
}
