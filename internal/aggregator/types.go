package aggregator

// Downstream is one configured target of an aggregate.
type Downstream struct {
	APIID     string            `json:"api_id"`
	Name      string            `json:"name"` // result key in merged output
	URL       string            `json:"url"`  // fully-resolved target URL
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      map[string]any    `json:"body,omitempty"`      // template/override body
	Condition string            `json:"condition,omitempty"` // for conditional mode: "key=value"
}

// Result is the outcome of a single downstream call.
type Result struct {
	Name       string `json:"name"`
	StatusCode int    `json:"status_code"`
	Body       any    `json:"body"`
	Duration   int    `json:"duration_ms"` // milliseconds
	Error      string `json:"error,omitempty"`
	Timeout    bool   `json:"timeout,omitempty"`
}

// Merged is the aggregate's overall response payload.
type Merged struct {
	Data   map[string]any `json:"data"`
	Errors []string       `json:"errors,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// FieldMapping renames a nested field from src to dst in the merged result.
type FieldMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}
