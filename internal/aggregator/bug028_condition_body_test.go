package aggregator

import "testing"

func TestBug028ConditionBodyPreservesCondition(t *testing.T) {
	inbound := map[string]any{"body": "pro"}
	if !matchesCondition("body = pro", inbound) {
		t.Fatal("body condition with surrounding spaces was not matched")
	}
}
