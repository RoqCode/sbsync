package sb

import (
	"context"
	"testing"
)

func TestRetryCounters_WithRetryCounters(t *testing.T) {
	rc := &RetryCounters{}
	ctx := context.Background()

	// Test attaching retry counters to context
	ctxWithCounters := WithRetryCounters(ctx, rc)
	if ctxWithCounters == nil {
		t.Fatal("WithRetryCounters returned nil context")
	}

	// Test retrieving retry counters
	retrieved := getRetryCounters(ctxWithCounters)
	if retrieved != rc {
		t.Error("Retrieved retry counters don't match original")
	}

	// Test non-nil context handling (avoid passing nil; use TODO)
	todoCtx := WithRetryCounters(context.TODO(), rc)
	if todoCtx == nil {
		t.Error("WithRetryCounters with TODO context should return non-nil context")
	}
	retrieved = getRetryCounters(todoCtx)
	if retrieved != rc {
		t.Error("Retrieved retry counters from nil context don't match original")
	}
}

func TestRetryCounters_GetRetryCounters(t *testing.T) {
	// Test context without retry counters
	ctx := context.Background()
	retrieved := getRetryCounters(ctx)
	if retrieved != nil {
		t.Error("Expected nil retry counters for context without them")
	}

	// Test empty context (avoid nil; use TODO)
	retrieved = getRetryCounters(context.TODO())
	if retrieved != nil {
		t.Error("Expected nil retry counters for nil context")
	}

	// Test context with wrong value type
	wrongCtx := context.WithValue(ctx, retryCtxKey{}, "not a retry counter")
	retrieved = getRetryCounters(wrongCtx)
	if retrieved != nil {
		t.Error("Expected nil retry counters for context with wrong value type")
	}
}

func TestRetryCounters_ContextIsolation(t *testing.T) {
	rc1 := &RetryCounters{Total: 1}
	rc2 := &RetryCounters{Total: 2}

	ctx := context.Background()
	ctx1 := WithRetryCounters(ctx, rc1)
	ctx2 := WithRetryCounters(ctx, rc2)

	// Test that contexts are independent
	retrieved1 := getRetryCounters(ctx1)
	retrieved2 := getRetryCounters(ctx2)

	if retrieved1 != rc1 {
		t.Error("Context 1 should have rc1")
	}
	if retrieved2 != rc2 {
		t.Error("Context 2 should have rc2")
	}
	if retrieved1 == retrieved2 {
		t.Error("Contexts should have different retry counters")
	}
}

func TestRetryCounters_NestedContext(t *testing.T) {
	rc1 := &RetryCounters{Total: 1}
	rc2 := &RetryCounters{Total: 2}

	ctx := context.Background()
	ctx1 := WithRetryCounters(ctx, rc1)
	ctx2 := WithRetryCounters(ctx1, rc2) // Nested context

	// Nested context should override parent
	retrieved := getRetryCounters(ctx2)
	if retrieved != rc2 {
		t.Error("Nested context should have rc2, not rc1")
	}

	// Parent context should still have original
	retrieved = getRetryCounters(ctx1)
	if retrieved != rc1 {
		t.Error("Parent context should still have rc1")
	}
}

func TestRetryCounters_StructFields(t *testing.T) {
	rc := &RetryCounters{
		Total:     10,
		Status429: 3,
		Status5xx: 2,
		Net:       5,
	}

	ctx := WithRetryCounters(context.Background(), rc)
	retrieved := getRetryCounters(ctx)

	if retrieved.Total != 10 {
		t.Errorf("Total = %d, want 10", retrieved.Total)
	}
	if retrieved.Status429 != 3 {
		t.Errorf("Status429 = %d, want 3", retrieved.Status429)
	}
	if retrieved.Status5xx != 2 {
		t.Errorf("Status5xx = %d, want 2", retrieved.Status5xx)
	}
	if retrieved.Net != 5 {
		t.Errorf("Net = %d, want 5", retrieved.Net)
	}
}

func TestRetryCounters_Modification(t *testing.T) {
	rc := &RetryCounters{}
	ctx := WithRetryCounters(context.Background(), rc)

	// Modify through original reference
	rc.Total = 5
	rc.Status429 = 2

	// Retrieve and verify changes are visible
	retrieved := getRetryCounters(ctx)
	if retrieved.Total != 5 {
		t.Errorf("Total = %d, want 5", retrieved.Total)
	}
	if retrieved.Status429 != 2 {
		t.Errorf("Status429 = %d, want 2", retrieved.Status429)
	}

	// Modify through retrieved reference
	retrieved.Status5xx = 3
	retrieved.Net = 1

	// Verify changes are visible through original reference
	if rc.Status5xx != 3 {
		t.Errorf("Status5xx = %d, want 3", rc.Status5xx)
	}
	if rc.Net != 1 {
		t.Errorf("Net = %d, want 1", rc.Net)
	}
}

func TestRetryCounters_ConcurrentAccess(t *testing.T) {
	rc := &RetryCounters{}
	ctx := WithRetryCounters(context.Background(), rc)

	// Test concurrent access to same retry counters
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			retrieved := getRetryCounters(ctx)
			if retrieved == nil {
				t.Error("Retrieved retry counters should not be nil")
				return
			}

			// Modify counters
			retrieved.Total++
			retrieved.Status429++
			retrieved.Status5xx++
			retrieved.Net++
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final values
	if rc.Total != 10 {
		t.Errorf("Total = %d, want 10", rc.Total)
	}
	if rc.Status429 != 10 {
		t.Errorf("Status429 = %d, want 10", rc.Status429)
	}
	if rc.Status5xx != 10 {
		t.Errorf("Status5xx = %d, want 10", rc.Status5xx)
	}
	if rc.Net != 10 {
		t.Errorf("Net = %d, want 10", rc.Net)
	}
}

func TestRetryCounters_IntegrationWithTransport(t *testing.T) {
	// This test simulates how the transport would use retry counters
	rc := &RetryCounters{}
	ctx := WithRetryCounters(context.Background(), rc)

	// Simulate transport retry logic
	simulateRetry := func(reason string) {
		retrieved := getRetryCounters(ctx)
		if retrieved != nil {
			retrieved.Total++
			switch reason {
			case "429":
				retrieved.Status429++
			case "5xx":
				retrieved.Status5xx++
			case "net":
				retrieved.Net++
			}
		}
	}

	// Simulate various retry scenarios
	simulateRetry("429")
	simulateRetry("5xx")
	simulateRetry("net")
	simulateRetry("429")
	simulateRetry("net")

	// Verify counters
	if rc.Total != 5 {
		t.Errorf("Total = %d, want 5", rc.Total)
	}
	if rc.Status429 != 2 {
		t.Errorf("Status429 = %d, want 2", rc.Status429)
	}
	if rc.Status5xx != 1 {
		t.Errorf("Status5xx = %d, want 1", rc.Status5xx)
	}
	if rc.Net != 2 {
		t.Errorf("Net = %d, want 2", rc.Net)
	}
}

func TestRetryCounters_ZeroValues(t *testing.T) {
	rc := &RetryCounters{} // Zero values
	ctx := WithRetryCounters(context.Background(), rc)

	retrieved := getRetryCounters(ctx)
	if retrieved.Total != 0 {
		t.Errorf("Total = %d, want 0", retrieved.Total)
	}
	if retrieved.Status429 != 0 {
		t.Errorf("Status429 = %d, want 0", retrieved.Status429)
	}
	if retrieved.Status5xx != 0 {
		t.Errorf("Status5xx = %d, want 0", retrieved.Status5xx)
	}
	if retrieved.Net != 0 {
		t.Errorf("Net = %d, want 0", retrieved.Net)
	}
}
