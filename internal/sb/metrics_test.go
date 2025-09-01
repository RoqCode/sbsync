package sb

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.hostCounts == nil {
		t.Error("hostCounts map not initialized")
	}
	if m.TotalRequests.Load() != 0 {
		t.Errorf("TotalRequests = %d, want 0", m.TotalRequests.Load())
	}
	if m.TotalRetries.Load() != 0 {
		t.Errorf("TotalRetries = %d, want 0", m.TotalRetries.Load())
	}
}

func TestMetrics_IncRequest(t *testing.T) {
	m := NewMetrics()

	// Test GET request
	m.IncRequest("api.storyblok.com", http.MethodGet)
	if m.TotalRequests.Load() != 1 {
		t.Errorf("TotalRequests = %d, want 1", m.TotalRequests.Load())
	}
	if m.ReadRequests.Load() != 1 {
		t.Errorf("ReadRequests = %d, want 1", m.ReadRequests.Load())
	}
	if m.WriteRequests.Load() != 0 {
		t.Errorf("WriteRequests = %d, want 0", m.WriteRequests.Load())
	}

	// Test POST request
	m.IncRequest("mapi.storyblok.com", http.MethodPost)
	if m.TotalRequests.Load() != 2 {
		t.Errorf("TotalRequests = %d, want 2", m.TotalRequests.Load())
	}
	if m.ReadRequests.Load() != 1 {
		t.Errorf("ReadRequests = %d, want 1", m.ReadRequests.Load())
	}
	if m.WriteRequests.Load() != 1 {
		t.Errorf("WriteRequests = %d, want 1", m.WriteRequests.Load())
	}

	// Test PUT request
	m.IncRequest("mapi.storyblok.com", http.MethodPut)
	if m.WriteRequests.Load() != 2 {
		t.Errorf("WriteRequests = %d, want 2", m.WriteRequests.Load())
	}

	// Test PATCH request
	m.IncRequest("mapi.storyblok.com", http.MethodPatch)
	if m.WriteRequests.Load() != 3 {
		t.Errorf("WriteRequests = %d, want 3", m.WriteRequests.Load())
	}

	// Test DELETE request
	m.IncRequest("mapi.storyblok.com", http.MethodDelete)
	if m.WriteRequests.Load() != 4 {
		t.Errorf("WriteRequests = %d, want 4", m.WriteRequests.Load())
	}

	// Test unknown method
	m.IncRequest("api.storyblok.com", "UNKNOWN")
	if m.TotalRequests.Load() != 6 {
		t.Errorf("TotalRequests = %d, want 6", m.TotalRequests.Load())
	}
	// Unknown methods shouldn't increment read/write counters
	if m.ReadRequests.Load() != 1 {
		t.Errorf("ReadRequests = %d, want 1", m.ReadRequests.Load())
	}
	if m.WriteRequests.Load() != 4 {
		t.Errorf("WriteRequests = %d, want 4", m.WriteRequests.Load())
	}
}

func TestMetrics_IncRequest_HostCounts(t *testing.T) {
	m := NewMetrics()

	// Test host counting
	m.IncRequest("api.storyblok.com", http.MethodGet)
	m.IncRequest("api.storyblok.com", http.MethodGet)
	m.IncRequest("mapi.storyblok.com", http.MethodPost)

	m.mu.Lock()
	apiCount := m.hostCounts["api.storyblok.com"]
	mapiCount := m.hostCounts["mapi.storyblok.com"]
	m.mu.Unlock()

	if apiCount != 2 {
		t.Errorf("api.storyblok.com count = %d, want 2", apiCount)
	}
	if mapiCount != 1 {
		t.Errorf("mapi.storyblok.com count = %d, want 1", mapiCount)
	}
}

func TestMetrics_IncRetry(t *testing.T) {
	m := NewMetrics()

	m.IncRetry()
	if m.TotalRetries.Load() != 1 {
		t.Errorf("TotalRetries = %d, want 1", m.TotalRetries.Load())
	}

	m.IncRetry()
	m.IncRetry()
	if m.TotalRetries.Load() != 3 {
		t.Errorf("TotalRetries = %d, want 3", m.TotalRetries.Load())
	}
}

func TestMetrics_AddBackoff(t *testing.T) {
	m := NewMetrics()

	duration1 := 100 * time.Millisecond
	duration2 := 250 * time.Millisecond

	m.AddBackoff(duration1)
	if m.TotalBackoffNanos.Load() != duration1.Nanoseconds() {
		t.Errorf("TotalBackoffNanos = %d, want %d", m.TotalBackoffNanos.Load(), duration1.Nanoseconds())
	}

	m.AddBackoff(duration2)
	expected := duration1.Nanoseconds() + duration2.Nanoseconds()
	if m.TotalBackoffNanos.Load() != expected {
		t.Errorf("TotalBackoffNanos = %d, want %d", m.TotalBackoffNanos.Load(), expected)
	}
}

func TestMetrics_IncStatus(t *testing.T) {
	m := NewMetrics()

	// Test 2xx status
	m.IncStatus(200)
	m.IncStatus(201)
	m.IncStatus(299)

	m.mu.Lock()
	status2xx := m.status2xx
	m.mu.Unlock()

	if status2xx != 3 {
		t.Errorf("status2xx = %d, want 3", status2xx)
	}

	// Test 3xx status
	m.IncStatus(301)
	m.IncStatus(302)

	m.mu.Lock()
	status3xx := m.status3xx
	m.mu.Unlock()

	if status3xx != 2 {
		t.Errorf("status3xx = %d, want 2", status3xx)
	}

	// Test 4xx status
	m.IncStatus(400)
	m.IncStatus(404)
	m.IncStatus(422)

	m.mu.Lock()
	status4xx := m.status4xx
	m.mu.Unlock()

	if status4xx != 3 {
		t.Errorf("status4xx = %d, want 3", status4xx)
	}

	// Test 429 status (special case)
	m.IncStatus(429)

	m.mu.Lock()
	status429 := m.status429
	m.mu.Unlock()

	if status429 != 1 {
		t.Errorf("status429 = %d, want 1", status429)
	}
	// 429 should not increment 4xx counter
	if status4xx != 3 {
		t.Errorf("status4xx after 429 = %d, want 3", status4xx)
	}

	// Test 5xx status
	m.IncStatus(500)
	m.IncStatus(502)
	m.IncStatus(503)
	m.IncStatus(504)

	m.mu.Lock()
	status5xx := m.status5xx
	m.mu.Unlock()

	if status5xx != 4 {
		t.Errorf("status5xx = %d, want 4", status5xx)
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics()

	// Set up some test data
	m.IncRequest("api.storyblok.com", http.MethodGet)
	m.IncRequest("mapi.storyblok.com", http.MethodPost)
	m.IncRetry()
	m.AddBackoff(100 * time.Millisecond)
	m.IncStatus(200)
	m.IncStatus(429)
	m.IncStatus(500)

	snapshot := m.Snapshot()

	// Test atomic counters
	if snapshot.TotalRequests != 2 {
		t.Errorf("snapshot.TotalRequests = %d, want 2", snapshot.TotalRequests)
	}
	if snapshot.TotalRetries != 1 {
		t.Errorf("snapshot.TotalRetries = %d, want 1", snapshot.TotalRetries)
	}
	if snapshot.TotalBackoffNanos != 100*time.Millisecond.Nanoseconds() {
		t.Errorf("snapshot.TotalBackoffNanos = %d, want %d", snapshot.TotalBackoffNanos, 100*time.Millisecond.Nanoseconds())
	}
	if snapshot.ReadRequests != 1 {
		t.Errorf("snapshot.ReadRequests = %d, want 1", snapshot.ReadRequests)
	}
	if snapshot.WriteRequests != 1 {
		t.Errorf("snapshot.WriteRequests = %d, want 1", snapshot.WriteRequests)
	}

	// Test status counters
	if snapshot.Status2xx != 1 {
		t.Errorf("snapshot.Status2xx = %d, want 1", snapshot.Status2xx)
	}
	if snapshot.Status429 != 1 {
		t.Errorf("snapshot.Status429 = %d, want 1", snapshot.Status429)
	}
	if snapshot.Status5xx != 1 {
		t.Errorf("snapshot.Status5xx = %d, want 1", snapshot.Status5xx)
	}

	// Test host counts
	if len(snapshot.HostCounts) != 2 {
		t.Errorf("snapshot.HostCounts length = %d, want 2", len(snapshot.HostCounts))
	}
	if snapshot.HostCounts["api.storyblok.com"] != 1 {
		t.Errorf("snapshot.HostCounts[api.storyblok.com] = %d, want 1", snapshot.HostCounts["api.storyblok.com"])
	}
	if snapshot.HostCounts["mapi.storyblok.com"] != 1 {
		t.Errorf("snapshot.HostCounts[mapi.storyblok.com] = %d, want 1", snapshot.HostCounts["mapi.storyblok.com"])
	}
}

func TestMetrics_Snapshot_Isolation(t *testing.T) {
	m := NewMetrics()

	// Set up initial data
	m.IncRequest("api.storyblok.com", http.MethodGet)
	m.IncStatus(200)

	snapshot1 := m.Snapshot()

	// Modify original metrics
	m.IncRequest("mapi.storyblok.com", http.MethodPost)
	m.IncStatus(429)

	snapshot2 := m.Snapshot()

	// First snapshot should be unchanged
	if snapshot1.TotalRequests != 1 {
		t.Errorf("snapshot1.TotalRequests = %d, want 1", snapshot1.TotalRequests)
	}
	if len(snapshot1.HostCounts) != 1 {
		t.Errorf("snapshot1.HostCounts length = %d, want 1", len(snapshot1.HostCounts))
	}

	// Second snapshot should reflect changes
	if snapshot2.TotalRequests != 2 {
		t.Errorf("snapshot2.TotalRequests = %d, want 2", snapshot2.TotalRequests)
	}
	if len(snapshot2.HostCounts) != 2 {
		t.Errorf("snapshot2.HostCounts length = %d, want 2", len(snapshot2.HostCounts))
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()

	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent IncRequest calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			host := "api.storyblok.com"
			if i%2 == 0 {
				host = "mapi.storyblok.com"
			}
			method := http.MethodGet
			if i%3 == 0 {
				method = http.MethodPost
			}
			m.IncRequest(host, method)
		}(i)
	}

	// Concurrent IncRetry calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.IncRetry()
		}()
	}

	// Concurrent AddBackoff calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.AddBackoff(10 * time.Millisecond)
		}()
	}

	// Concurrent IncStatus calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status := 200 + (i%5)*100
			m.IncStatus(status)
		}(i)
	}

	wg.Wait()

	// Verify all operations completed
	if m.TotalRequests.Load() != int64(numGoroutines) {
		t.Errorf("TotalRequests = %d, want %d", m.TotalRequests.Load(), numGoroutines)
	}
	if m.TotalRetries.Load() != int64(numGoroutines) {
		t.Errorf("TotalRetries = %d, want %d", m.TotalRetries.Load(), numGoroutines)
	}
	if m.TotalBackoffNanos.Load() != int64(numGoroutines)*10*time.Millisecond.Nanoseconds() {
		t.Errorf("TotalBackoffNanos = %d, want %d", m.TotalBackoffNanos.Load(), int64(numGoroutines)*10*time.Millisecond.Nanoseconds())
	}
}

func TestMetrics_EdgeCases(t *testing.T) {
	m := NewMetrics()

	// Test empty host
	m.IncRequest("", http.MethodGet)
	if m.TotalRequests.Load() != 1 {
		t.Errorf("TotalRequests with empty host = %d, want 1", m.TotalRequests.Load())
	}

	// Test zero duration backoff
	m.AddBackoff(0)
	if m.TotalBackoffNanos.Load() != 0 {
		t.Errorf("TotalBackoffNanos with zero duration = %d, want 0", m.TotalBackoffNanos.Load())
	}

	// Test negative status codes
	m.IncStatus(-1)
	m.IncStatus(0)
	m.IncStatus(1)

	// These should not increment any status counters
	m.mu.Lock()
	totalStatus := m.status2xx + m.status3xx + m.status4xx + m.status429 + m.status5xx
	m.mu.Unlock()

	if totalStatus != 0 {
		t.Errorf("total status counts = %d, want 0", totalStatus)
	}
}

func TestMetrics_RealisticUsage(t *testing.T) {
	m := NewMetrics()

	// Simulate realistic API usage pattern
	hosts := []string{"api.storyblok.com", "mapi.storyblok.com"}
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch}
	statuses := []int{200, 201, 400, 404, 422, 429, 500, 502, 503}

	// Simulate 100 requests
	for i := 0; i < 100; i++ {
		host := hosts[i%len(hosts)]
		method := methods[i%len(methods)]
		status := statuses[i%len(statuses)]

		m.IncRequest(host, method)
		m.IncStatus(status)

		// Some requests result in retries
		if i%10 == 0 {
			m.IncRetry()
			m.AddBackoff(time.Duration(i%1000) * time.Millisecond)
		}
	}

	snapshot := m.Snapshot()

	// Verify totals
	if snapshot.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d, want 100", snapshot.TotalRequests)
	}
	if snapshot.TotalRetries != 10 {
		t.Errorf("TotalRetries = %d, want 10", snapshot.TotalRetries)
	}

	// Verify read/write split (roughly 25% GET, 75% write operations)
	expectedReads := int64(25) // 100 * 25%
	if snapshot.ReadRequests < expectedReads-5 || snapshot.ReadRequests > expectedReads+5 {
		t.Errorf("ReadRequests = %d, want ~%d", snapshot.ReadRequests, expectedReads)
	}

	// Verify host distribution
	if len(snapshot.HostCounts) != 2 {
		t.Errorf("HostCounts length = %d, want 2", len(snapshot.HostCounts))
	}

	// Each host should have roughly 50 requests
	for host, count := range snapshot.HostCounts {
		if count < 45 || count > 55 {
			t.Errorf("HostCounts[%s] = %d, want ~50", host, count)
		}
	}
}
