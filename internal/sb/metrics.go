package sb

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds lightweight counters for HTTP activity.
type Metrics struct {
	// totals
	TotalRequests     atomic.Int64
	TotalRetries      atomic.Int64
	TotalBackoffNanos atomic.Int64

	// by operation type
	ReadRequests  atomic.Int64 // GET
	WriteRequests atomic.Int64 // POST/PUT/PATCH/DELETE

	mu         sync.Mutex
	hostCounts map[string]int64
	status2xx  int64
	status3xx  int64
	status4xx  int64
	status429  int64
	status5xx  int64
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics { return &Metrics{hostCounts: make(map[string]int64)} }

// IncRequest increments per-host and total request counters.
func (m *Metrics) IncRequest(host, method string) {
	m.TotalRequests.Add(1)
	switch strings.ToUpper(method) {
	case http.MethodGet:
		m.ReadRequests.Add(1)
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		m.WriteRequests.Add(1)
	}
	m.mu.Lock()
	m.hostCounts[host]++
	m.mu.Unlock()
}

// IncRetry increments retry counter.
func (m *Metrics) IncRetry() { m.TotalRetries.Add(1) }

// AddBackoff accumulates backoff sleep time.
func (m *Metrics) AddBackoff(d time.Duration) { m.TotalBackoffNanos.Add(d.Nanoseconds()) }

// IncStatus tracks status buckets.
func (m *Metrics) IncStatus(code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if code == 429 {
		m.status429++
		return
	}
	if code >= 200 && code < 300 {
		m.status2xx++
		return
	}
	if code >= 300 && code < 400 {
		m.status3xx++
		return
	}
	if code >= 400 && code < 500 {
		m.status4xx++
		return
	}
	if code >= 500 {
		m.status5xx++
	}
}

// MetricsSnapshot is a read-only copy of metrics state.
type MetricsSnapshot struct {
	TotalRequests     int64
	TotalRetries      int64
	TotalBackoffNanos int64
	HostCounts        map[string]int64
	ReadRequests      int64
	WriteRequests     int64
	Status2xx         int64
	Status3xx         int64
	Status4xx         int64
	Status429         int64
	Status5xx         int64
}

// Snapshot returns a copy of the metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyHosts := make(map[string]int64, len(m.hostCounts))
	for k, v := range m.hostCounts {
		copyHosts[k] = v
	}
	return MetricsSnapshot{
		TotalRequests:     m.TotalRequests.Load(),
		TotalRetries:      m.TotalRetries.Load(),
		TotalBackoffNanos: m.TotalBackoffNanos.Load(),
		HostCounts:        copyHosts,
		ReadRequests:      m.ReadRequests.Load(),
		WriteRequests:     m.WriteRequests.Load(),
		Status2xx:         m.status2xx,
		Status3xx:         m.status3xx,
		Status4xx:         m.status4xx,
		Status429:         m.status429,
		Status5xx:         m.status5xx,
	}
}
