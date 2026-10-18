package providers

import (
	"encoding/json"
	"fmt"
)

// upgrades[n] turns a version-n row into version n+1. Schema v1 is the first shape, so the table is empty;
// a v2 adds its v1 -> v2 step here and older rows keep loading (doc 34 rule 5).
var upgrades = map[int]func(map[string]any) map[string]any{}

// Convert brings a row up to SchemaVersion. A row newer than this hub knows is refused, never guessed at.
func Convert(raw []byte) ([]byte, error) { return convertTo(raw, SchemaVersion) }

func convertTo(raw []byte, target int) ([]byte, error) {
	var probe struct {
		SchemaVersion *json.Number `json:"schema_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("row is not a JSON object: %v", jsonMsg(err))
	}
	if probe.SchemaVersion == nil {
		return nil, fmt.Errorf("is required (this hub writes %d)", SchemaVersion)
	}
	v64, err := probe.SchemaVersion.Int64()
	if err != nil || v64 < 1 {
		return nil, fmt.Errorf("must be a positive integer")
	}
	v := int(v64)
	if v > target {
		return nil, fmt.Errorf("%d is newer than this hub knows (%d); update the hub before loading this row", v, target)
	}
	if v == target {
		return raw, nil
	}
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, err
	}
	for ; v < target; v++ {
		up, ok := upgrades[v]
		if !ok {
			return nil, fmt.Errorf("no converter from version %d", v)
		}
		row = up(row)
	}
	row["schema_version"] = target
	return json.Marshal(row)
}
