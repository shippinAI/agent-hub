package httpapi

import (
	"net/http"
	"strings"

	"agenthub/internal/policy"
	"agenthub/internal/store"
)

func (s *Server) me(w http.ResponseWriter, r *http.Request, p *store.Principal) (any, error) {
	_, via, _ := s.Auth.Identify(r)
	can := map[string]bool{}
	for _, a := range []string{"admin.principals.update", "admin.policies.put", "task.force_release", "task.create", "task.edit_held"} {
		d, err := s.St.Can(r.Context(), p, a, policy.Resource{})
		if err != nil {
			return nil, err
		}
		// code rules: admin.* and force-release also need a human principal
		can[a] = d.Allowed && (p.IsHuman || !(strings.HasPrefix(a, "admin.") || a == "task.force_release"))
	}
	return map[string]any{"principal": p, "auth": via, "can": can, "public_url": s.Cfg.PublicURL}, nil
}
