package policyrules

import (
	"fmt"
	"slices"
)

// review needs at least Min PASS verdicts (default 1) on the current ticket hash from reviewers who are not the
// author and, when Role is set, hold that role. Any BLOCK on the current hash denies (forbid overrides permit).
func (e *Engine) review(c Check, t *Ticket) string {
	need := 1
	if c.Min != nil && *c.Min > 0 {
		need = *c.Min
	}
	pass, block, findings := tally(e.reviews(t), t.Author, func(r Review) bool { return c.Role == "" || slices.Contains(r.Roles, c.Role) })
	switch {
	case block > 0:
		return or(c.Msg, "review required") + fmt.Sprintf(": ticket %s has %d BLOCK verdict(s) on its current content", t.ID, block)
	case pass < need:
		who := "a reviewer"
		if c.Role != "" {
			who = "a '" + c.Role + "' reviewer"
		}
		extra := ""
		if findings > 0 {
			extra = fmt.Sprintf(" (%d FINDINGS verdict(s) to address)", findings)
		}
		return or(c.Msg, "review required") + fmt.Sprintf(": needs %d PASS from %s other than the author, has %d%s", need, who, pass, extra)
	}
	return ""
}

// approval needs a PASS from a human reviewer (not the author) on the current ticket hash, and no BLOCK.
func (e *Engine) approval(c Check, t *Ticket) string {
	pass, block, _ := tally(e.reviews(t), t.Author, func(r Review) bool { return r.Human })
	if block > 0 || pass == 0 {
		return or(c.Msg, "approval required") + ": a human approver must PASS ticket " + t.ID + " (its current content)"
	}
	return ""
}

func (e *Engine) reviews(t *Ticket) []Review {
	if e.Facts == nil {
		return nil
	}
	return e.Facts.Reviews(t)
}

// tally counts distinct reviewers' latest verdicts. Self-reviews never count (the store also refuses them).
func tally(rs []Review, author string, eligible func(Review) bool) (pass, block, findings int) {
	latest := map[string]string{}
	var order []string
	for _, r := range rs {
		if r.Reviewer == author || !eligible(r) {
			continue
		}
		if _, ok := latest[r.Reviewer]; !ok {
			order = append(order, r.Reviewer)
		}
		latest[r.Reviewer] = r.Verdict
	}
	for _, who := range order {
		switch latest[who] {
		case "PASS":
			pass++
		case "BLOCK":
			block++
		case "FINDINGS":
			findings++
		}
	}
	return
}
