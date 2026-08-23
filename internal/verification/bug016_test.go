package verification_test

import (
	"math/rand"
	"testing"

	"api-mock-system/internal/mockengine"
)

// source marker: mockengine.generateArray -> mockservice.Resolve -> mockhandler.Serve
func TestArrayResponseSchemaWithoutItemsDoesNotPanic(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("array response generation panicked: %v", recovered)
		}
	}()

	value := mockengine.New(rand.NewSource(1)).Generate(map[string]any{
		"type":     "array",
		"minItems": 1,
		"maxItems": 1,
	})
	items, ok := value.([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("array response = %#v, want one generated item", value)
	}
}
