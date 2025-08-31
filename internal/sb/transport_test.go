package sb

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock allows deterministic control of time passage.
type fakeClock struct {
	now   time.Time
	slept time.Duration
}

func newFakeClock() *fakeClock              { return &fakeClock{now: time.Unix(0, 0)} }
func (fc *fakeClock) Now() time.Time        { return fc.now }
func (fc *fakeClock) Sleep(d time.Duration) { fc.now = fc.now.Add(d); fc.slept += d }

// fakeRT returns a queued series of responses or errors.
type fakeRT struct {
	calls atomic.Int64
	queue []any // *http.Response or error
}

func (frt *fakeRT) RoundTrip(_ *http.Request) (*http.Response, error) {
	idx := frt.calls.Add(1) - 1
	if int(idx) >= len(frt.queue) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	}
	item := frt.queue[idx]
	if resp, ok := item.(*http.Response); ok {
		if resp.Body == nil {
			resp.Body = http.NoBody
		}
		return resp, nil
	}
	if err, ok := item.(error); ok {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func newReq(host string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+host+"/x", nil)
	return req
}

func TestRetryAfterSeconds(t *testing.T) {
	fc := newFakeClock()
	metrics := NewMetrics()
	opt := TransportOptions{
		RetryMax:    2,
		BackoffBase: 250 * time.Millisecond,
		BackoffCap:  5 * time.Second,
		Clock:       fc,
		JitterFn:    func(base time.Duration, _ int) time.Duration { return 0 },
		Metrics:     metrics,
		HostLimits:  map[string]Limit{"mapi.storyblok.com": {RPS: 1000, Burst: 1000}},
	}
	frt := &fakeRT{queue: []any{
		&http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"2"}}, Body: http.NoBody},
		&http.Response{StatusCode: 200, Body: http.NoBody},
	}}
	tr := NewRetryingLimiterTransport(opt)
	tr.Base = frt

	resp, err := tr.RoundTrip(newReq("mapi.storyblok.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := fc.slept; got < 2*time.Second || got > 2100*time.Millisecond {
		t.Fatalf("expected ~2s sleep, got %v", got)
	}
	if metrics.TotalRetries.Load() != 1 {
		t.Fatalf("expected 1 retry, got %d", metrics.TotalRetries.Load())
	}
}

func TestBackoffOn503(t *testing.T) {
	fc := newFakeClock()
	opt := TransportOptions{
		RetryMax:    2,
		BackoffBase: 250 * time.Millisecond,
		BackoffCap:  5 * time.Second,
		Clock:       fc,
		JitterFn:    func(base time.Duration, _ int) time.Duration { return 0 },
		HostLimits:  map[string]Limit{"mapi.storyblok.com": {RPS: 1000, Burst: 1000}},
		Metrics:     NewMetrics(),
	}
	frt := &fakeRT{queue: []any{
		&http.Response{StatusCode: 503, Body: http.NoBody},
		&http.Response{StatusCode: 200, Body: http.NoBody},
	}}
	tr := NewRetryingLimiterTransport(opt)
	tr.Base = frt

	resp, err := tr.RoundTrip(newReq("mapi.storyblok.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	// first backoff should be base (250ms)
	if fc.slept != 250*time.Millisecond {
		t.Fatalf("expected 250ms sleep, got %v", fc.slept)
	}
}

func TestLimiterPacing(t *testing.T) {
	fc := newFakeClock()
	opt := TransportOptions{
		RetryMax:    0,
		BackoffBase: 250 * time.Millisecond,
		BackoffCap:  5 * time.Second,
		Clock:       fc,
		HostLimits:  map[string]Limit{"mapi.storyblok.com": {RPS: 2, Burst: 1}},
		Metrics:     NewMetrics(),
	}
	frt := &fakeRT{queue: []any{
		&http.Response{StatusCode: 200, Body: http.NoBody},
		&http.Response{StatusCode: 200, Body: http.NoBody},
		&http.Response{StatusCode: 200, Body: http.NoBody},
	}}
	tr := NewRetryingLimiterTransport(opt)
	tr.Base = frt

	// Three sequential requests at 2 rps, burst 1 => expect at least ~1s total sleep for the latter 2 tokens
	_, _ = tr.RoundTrip(newReq("mapi.storyblok.com")) // consumes burst token
	_, _ = tr.RoundTrip(newReq("mapi.storyblok.com")) // should sleep ~0.5s to acquire next token
	_, _ = tr.RoundTrip(newReq("mapi.storyblok.com")) // another ~0.5s

	if fc.slept < 900*time.Millisecond { // allow some slack
		t.Fatalf("expected ~1s sleep due to limiter, got %v", fc.slept)
	}
}

type transientErr struct{}

func (transientErr) Error() string   { return "temporary network error" }
func (transientErr) Timeout() bool   { return false }
func (transientErr) Temporary() bool { return true }

func TestCancelDuringBackoff(t *testing.T) {
	fc := newFakeClock()
	opt := TransportOptions{
		RetryMax:    1,
		BackoffBase: 2 * time.Second,
		BackoffCap:  5 * time.Second,
		Clock:       fc,
		JitterFn:    func(base time.Duration, _ int) time.Duration { return 0 },
		HostLimits:  map[string]Limit{"mapi.storyblok.com": {RPS: 1000, Burst: 1000}},
		Metrics:     NewMetrics(),
	}
	frt := &fakeRT{queue: []any{transientErr{}, &http.Response{StatusCode: 200, Body: http.NoBody}}}
	tr := NewRetryingLimiterTransport(opt)
	tr.Base = frt

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://mapi.storyblok.com/x", nil)

	// Cancel context before RoundTrip backoff completes. Our fake clock advances immediately on Sleep,
	// so cancel right away and ensure RoundTrip respects ctx on limiter/waits.
	cancel()
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatalf("expected error due to cancellation")
	}
}
