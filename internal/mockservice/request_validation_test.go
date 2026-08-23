package mockservice

import (
	"strings"
	"testing"

	"api-mock-system/internal/models"
)

// TestValidateRequestSurfacesAllViolations confirms the public mock entry point
// to request validation no longer keeps only the first violation. A batch with
// multiple errors must report every one in the error string.
func TestValidateRequestSurfacesAllViolations(t *testing.T) {
	body := `[{"name":123},{"age":"x"}]`
	schemaMap := models.JSONMap{
		"body": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []string{"name"},
			},
		},
	}
	err := validateRequest(schemaMap, body)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"$[0].name", "$[1].age", "$[1].name"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q: %s", want, msg)
		}
	}
}
