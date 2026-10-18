package presets

import (
	"context"

	"agenthub/internal/catalog"
	"agenthub/internal/guides"
	"agenthub/internal/store"
)

// SeedAll seeds the hub preset, then the Devagotchi settings, roles, catalog, policies and Shippin guides.
// Every part is insert-if-missing (and catalog files and guides are marked), so it is safe to re-run.
func SeedAll(ctx context.Context, st *store.Store, adminLogin string) (map[string]int, error) {
	p, err := Default()
	if err != nil {
		return nil, err
	}
	d, err := LoadDevagotchi()
	if err != nil {
		return nil, err
	}
	n, err := st.Seed(ctx, store.SeedData{Settings: append(p.Settings, d.Settings...), Roles: append(p.Roles, d.Roles...),
		Principals: p.Principals, Workflows: p.Workflows, Projects: p.Projects, Policies: p.Policies}, adminLogin)
	if err != nil {
		return nil, err
	}
	_, err = st.Write(ctx, store.System, "", "seed_devagotchi", nil, func(w *store.W) (any, error) {
		for _, f := range d.Catalog {
			c, err := catalog.SeedRows(w, f.Name, f.Kind, f.Rows)
			if err != nil {
				return nil, err
			}
			n["catalog"] += c
		}
		c, err := catalog.SeedPolicies(w, "devagotchi/policies.v1", d.Policies)
		if err != nil {
			return nil, err
		}
		n["devagotchi_policies"] = c
		for _, g := range d.Guides {
			c, err := guides.Seed(w, g)
			if err != nil {
				return nil, err
			}
			n["guides"] += c
		}
		return nil, nil
	})
	return n, err
}
