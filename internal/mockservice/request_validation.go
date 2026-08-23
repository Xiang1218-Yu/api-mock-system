package mockservice

import (
	"errors"
	"fmt"

	"api-mock-system/internal/models"
	"api-mock-system/internal/schema"
)

// ErrInvalidRequest marks a request rejected by the API's request_schema.
var ErrInvalidRequest = errors.New("mock request does not match request schema")

func validateRequest(schemaMap models.JSONMap, body string) error {
	if len(schemaMap) == 0 {
		return nil
	}
	bodySchema, hasBodySchema := schemaMap["body"].(map[string]any)
	if !hasBodySchema {
		return nil
	}
	value, present, err := schema.DecodeJSONBody(body)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if !present {
		if required, ok := bodySchema["required"].(bool); ok && required {
			return fmt.Errorf("%w: body is required", ErrInvalidRequest)
		}
		return nil
	}
	if err := schema.Validate(value, bodySchema); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return nil
}
