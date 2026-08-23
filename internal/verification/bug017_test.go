package verification_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"api-mock-system/internal/aggregator"

	"go.uber.org/zap"
)

type contextTimeoutTransport struct{}

func (contextTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// source marker: aggregator.call -> aggregateservice.Execute -> aggregatehandler.Serve
func TestTimeoutRemainsVisibleInAggregateResults(t *testing.T) {
	client := &http.Client{Transport: contextTimeoutTransport{}}
	executor := aggregator.New(client, zap.NewNop())
	merged, results := executor.Run(
		context.Background(),
		"serial",
		[]aggregator.Downstream{{Name: "slow", URL: "http://downstream.invalid"}},
		nil,
		5*time.Millisecond,
		nil,
	)
	if len(results) != 1 || !results[0].Timeout {
		t.Fatalf("timeout outcome was not recorded: %#v", results)
	}
	if results[0].Error == "" {
		t.Fatal("timeout result lost its error status")
	}
	if len(merged.Errors) == 0 {
		t.Fatal("aggregate result lost the downstream timeout")
	}
}
