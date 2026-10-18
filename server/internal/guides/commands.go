package guides

import (
	"context"
	"encoding/json"
)

// Commands resolves command_ref steps from docker_runtime_presets.operator_commands ({key: command}); each
// runtime preset is already for one OS, so os only matters through which runtime the step names.
func (s *Service) Commands(ctx context.Context) func(runtime, key, os string) string {
	cache := map[string]map[string]string{}
	return func(runtime, key, _ string) string {
		m, ok := cache[runtime]
		if !ok {
			var raw []byte
			_ = s.St.DB.QueryRow(ctx, `SELECT operator_commands FROM docker_runtime_presets WHERE id=$1`, runtime).Scan(&raw)
			_ = json.Unmarshal(raw, &m)
			cache[runtime] = m
		}
		return m[key]
	}
}
