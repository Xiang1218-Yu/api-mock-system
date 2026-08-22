package aggregateservice

import (
	"api-mock-system/internal/aggregator"
	"api-mock-system/internal/models"
)

// buildDownstreams extracts the configured downstream list from the stored JSON
// and resolves each to an internal URL. The downstream_apis field is expected
// to be a {"downstreams": [ {api_id, name, method, ...} ]} structure.
func buildDownstreams(a *models.Aggregate, baseURL string) []aggregator.Downstream {
	raw, ok := a.DownstreamAPIs["downstreams"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]aggregator.Downstream, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ds := aggregator.Downstream{
			APIID:     str(m, "api_id"),
			Name:      str(m, "name"),
			Method:    str(m, "method"),
			URL:       resolveURL(m, baseURL),
			Headers:   toStringMap(m["headers"]),
			Condition: str(m, "condition"),
		}
		if b, ok := m["body"].(map[string]any); ok {
			ds.Body = b
		}
		out = append(out, ds)
	}
	return out
}

// buildMappings extracts field mappings into the aggregator's slice form.
func buildMappings(a *models.Aggregate) []aggregator.FieldMapping {
	raw, ok := a.FieldMappings["mappings"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]aggregator.FieldMapping, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, aggregator.FieldMapping{From: str(m, "from"), To: str(m, "to")})
	}
	return out
}

// resolveURL builds the downstream target URL. If the downstream declares an
// explicit "url", use it; otherwise build from the api_id against baseURL.
func resolveURL(m map[string]any, baseURL string) string {
	if u := str(m, "url"); u != "" {
		return u
	}
	if apiID := str(m, "api_id"); apiID != "" {
		return baseURL + "/internal/api/" + apiID
	}
	return ""
}
