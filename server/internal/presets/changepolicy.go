package presets

import (
	_ "embed"
	"encoding/json"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

//go:embed changepolicy/default.json
var changePolicyDoc []byte

//go:embed changepolicy/review.json
var reviewJSON []byte

// Review is the reviewer-role bundle: the role, reviewer principals and the data policies for tickets and reviews.
type Review struct {
	Roles      []store.Role      `json:"roles"`
	Principals []store.Principal `json:"principals"`
	Policies   []policy.Rule     `json:"policies"`
	Doc        json.RawMessage   `json:"-"`
}

// LoadReview parses the embedded change-policy presets (seeded once, then edited in the UI).
func LoadReview() (Review, error) {
	var r Review
	err := json.Unmarshal(reviewJSON, &r)
	for i := range r.Policies {
		r.Policies[i].Enabled = true
	}
	r.Doc = changePolicyDoc
	return r, err
}
