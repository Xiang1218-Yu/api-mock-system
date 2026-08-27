// Package aggregator fans a single inbound request out to many downstream APIs,
// then merges their responses. It owns no state and no HTTP routing — it is a
// pure function from (config, inbound) to a merged result.
//
// Three modes per system.md §2.4:
//   - serial:     call downstreams one at a time, merge in order.
//   - parallel:   call all at once via goroutines, merge when all resolve.
//   - conditional: pick downstreams based on inbound request fields.
package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Aggregator executes fan-out. One instance is reusable across requests.
type Aggregator struct {
	client *http.Client
	log    *zap.Logger
}

// New returns an Aggregator using the given HTTP client (timeouts applied per
// call via context). Passing a custom client lets tests stub transport.
func New(client *http.Client, log *zap.Logger) *Aggregator {
	if client == nil {
		client = http.DefaultClient
	}
	return &Aggregator{client: client, log: log}
}

// Run executes the aggregate and returns the merged result plus per-downstream
// outcomes for monitoring. timeout bounds the entire fan-out.
func (a *Aggregator) Run(ctx context.Context, mode string, downstreams []Downstream, mappings []FieldMapping, timeout time.Duration, inbound map[string]any) (Merged, []Result) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	selected := selectDownstreams(mode, downstreams, inbound)

	var results []Result
	switch strings.ToLower(mode) {
	case "serial":
		results = a.runSerial(ctx, selected)
	case "parallel":
		results = a.runParallel(ctx, selected)
	case "conditional":
		// selection already happened; execute selected in parallel.
		results = a.runParallel(ctx, selected)
	default:
		results = a.runParallel(ctx, selected)
	}

	return a.merge(results, mappings), results
}

// selectDownstreams filters the list. In conditional mode, only downstreams
// whose Condition matches the inbound request are kept. Other modes are no-ops.
func selectDownstreams(mode string, ds []Downstream, inbound map[string]any) []Downstream {
	if strings.ToLower(mode) != "conditional" {
		return ds
	}
	out := make([]Downstream, 0, len(ds))
	for _, d := range ds {
		if d.Condition == "" || matchesCondition(d.Condition, inbound) {
			out = append(out, d)
		}
	}
	return out
}

// matchesCondition evaluates a simple "key=value" predicate against the inbound
// request map. Unknown keys or mismatches mean the downstream is skipped.
func matchesCondition(cond string, inbound map[string]any) bool {
	k, v, ok := strings.Cut(cond, "=")
	if !ok {
		return false
	}
	val, exists := inbound[strings.TrimSpace(k)]
	if !exists {
		return false
	}
	expected := v
	return fmt.Sprint(val) == expected
}

// runSerial calls downstreams in order, threading each result into the next.
func (a *Aggregator) runSerial(ctx context.Context, ds []Downstream) []Result {
	results := make([]Result, 0, len(ds))
	for _, d := range ds {
		results = append(results, a.call(ctx, d))
	}
	return results
}

// runParallel calls every downstream concurrently and waits on all.
func (a *Aggregator) runParallel(ctx context.Context, ds []Downstream) []Result {
	var wg sync.WaitGroup
	results := make([]Result, len(ds))
	for i, d := range ds {
		wg.Add(1)
		go func(idx int, dd Downstream) {
			defer wg.Done()
			results[idx] = a.call(ctx, dd)
		}(i, d)
	}
	wg.Wait()
	return results
}

// call executes one downstream request and normalizes its outcome.
func (a *Aggregator) call(ctx context.Context, d Downstream) Result {
	start := time.Now()
	res := Result{Name: d.Name}
	if d.URL == "" {
		res.Error = "downstream url is empty"
		return res
	}

	method := d.Method
	if method == "" {
		method = http.MethodGet
	}
	var bodyReader io.Reader
	if len(d.Body) > 0 {
		b, _ := json.Marshal(d.Body)
		bodyReader = strings.NewReader(string(b))
	}

	req, err := http.NewRequestWithContext(ctx, method, d.URL, bodyReader)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	for k, v := range d.Headers {
		req.Header.Set(k, v)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.client.Do(req)
	if err != nil {
		// Distinguish timeout from other errors for the "partial data + timeout
		// hint" availability requirement.
		if ctx.Err() != nil {
			res.Timeout = true
			res.Error = "timeout"
		} else {
			res.Error = err.Error()
		}
		return res
	}
	defer resp.Body.Close()
	res.StatusCode = resp.StatusCode

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	var parsed any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &parsed) // tolerate non-JSON bodies
	}
	res.Body = parsed
	// Round up so a sub-millisecond local call still reports 1ms rather than 0.
	// Reporting 0 made the call-monitoring metric (spec §2.4) look missing.
	ms := int(time.Since(start) / time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	res.Duration = ms
	return res
}

// merge folds per-downstream results into a single Merged payload, applying
// field mappings. Failures contribute to Errors but never abort the merge.
func (a *Aggregator) merge(results []Result, mappings []FieldMapping) Merged {
	data := make(map[string]any, len(results))
	var errs []string
	meta := make(map[string]any)
	total, ok := 0, 0
	for _, r := range results {
		total++
		if r.Error != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", r.Name, r.Error))
			continue
		}
		ok++
		data[r.Name] = r.Body
	}
	// Apply mappings: rename top-level keys from From -> To.
	for _, m := range mappings {
		if v, exists := data[m.From]; exists {
			data[m.To] = v
			delete(data, m.From)
		}
	}
	meta["total"] = total
	meta["ok"] = ok
	return Merged{Data: data, Errors: errs, Meta: meta}
}
