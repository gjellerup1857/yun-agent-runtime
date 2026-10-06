package tools

import (
	"encoding/json"
	"fmt"
)

type SchemaValidator interface {
	Validate(schema json.RawMessage, value json.RawMessage) error
}

type BasicSchemaValidator struct{}

func NewBasicSchemaValidator() *BasicSchemaValidator { return &BasicSchemaValidator{} }

type simpleSchema struct {
	Type string `json:"type"`
	Required []string `json:"required"`
}

func (v *BasicSchemaValidator) Validate(schema json.RawMessage, value json.RawMessage) error {
	if len(schema) == 0 { return nil }
	var definition simpleSchema
	if err := json.Unmarshal(schema, &definition); err != nil {
		return fmt.Errorf("invalid tool schema: %w", err)
	}
	if definition.Type != "object" { return nil }
	var object map[string]json.RawMessage
	if err := json.Unmarshal(value, &object); err != nil {
		return fmt.Errorf("%w: expected JSON object", ErrInvalidArguments)
	}
	for _, required := range definition.Required {
		if _, ok := object[required]; !ok {
			return fmt.Errorf("%w: missing field %q", ErrInvalidArguments, required)
		}
	}
	return nil
}
