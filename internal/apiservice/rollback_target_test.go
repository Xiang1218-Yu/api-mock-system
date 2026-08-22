package apiservice

import (
	"testing"

	"api-mock-system/internal/models"
)

func TestRollbackRestoresNamedSchemaMaps(t *testing.T) {
	api := &models.API{}
	snapshot := models.JSONMap{
		"RequestSchema":  models.JSONMap{"customer": models.JSONMap{"code": "string"}},
		"ResponseSchema": models.JSONMap{"result": models.JSONMap{"ok": "boolean"}},
	}
	restoreAPIFromSnapshot(api, snapshot)
	if api.RequestSchema == nil || api.ResponseSchema == nil {
		t.Fatalf("rollback dropped named schema maps: request=%v response=%v", api.RequestSchema, api.ResponseSchema)
	}
	if _, ok := api.RequestSchema["customer"].(models.JSONMap); !ok {
		t.Fatalf("rollback did not preserve nested request schema type: %T", api.RequestSchema["customer"])
	}
}
