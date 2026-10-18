// Package schemacheck validates JSON documents against JSON Schemas that are stored as data
// (capability selector schemas, the guide step schema).
package schemacheck

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// Compile parses and resolves a schema.
func Compile(schema []byte) (*jsonschema.Resolved, error) {
	if len(bytes.TrimSpace(schema)) == 0 || string(bytes.TrimSpace(schema)) == "{}" {
		schema = []byte(`{"type":"object"}`)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil, fmt.Errorf("schema is not valid JSON Schema: %w", err)
	}
	r, err := s.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("schema does not resolve: %w", err)
	}
	return r, nil
}

// Validate checks one instance against a schema.
func Validate(schema, instance []byte) error {
	r, err := Compile(schema)
	if err != nil {
		return err
	}
	var v any
	if len(bytes.TrimSpace(instance)) == 0 {
		instance = []byte("{}")
	}
	if err := json.Unmarshal(instance, &v); err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	return r.Validate(v)
}
