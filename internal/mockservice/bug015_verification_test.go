package mockservice

import (
	"strings"
	"testing"

	"api-mock-system/internal/models"
)

// source marker: schema.DecodeJSONBody -> schema.Validate -> validateRequest
func TestBatchRequestValidationChecksEveryArrayElement(t *testing.T) {
	requestSchema := models.JSONMap{
		"body": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":      "string",
				"minLength": 3,
			},
		},
	}
	err := validateRequest(requestSchema, `["x","y"]`)
	if err == nil {
		t.Fatal("expected the invalid batch request to be rejected")
	}
	message := err.Error()
	if !strings.Contains(message, "$[0]") || !strings.Contains(message, "$[1]") {
		t.Fatalf("validation error omitted an invalid array element: %s", message)
	}
}
