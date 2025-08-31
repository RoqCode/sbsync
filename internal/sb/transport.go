package sb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Clock abstracts time for deterministic tests.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// Limit defines a simple rate limit: RPS with a burst capacity.
type Limit struct {
	RPS   float64
	Burst int
}

// TransportOptions configures the retrying, rate-limited transport.
type TransportOptions struct {
	RetryMax    int
	BackoffBase time.Duration
	BackoffCap  time.Duration
	JitterFn    func(base time.Duration, attempt int) time.Duration
	Clock       Clock
	Metrics     *Metrics

	// Host-specific limits (by req.URL.Host). If missing, defaults apply.
	HostLimits map[string]Limit
}

// DefaultTransportOptionsFromEnv returns defaults suitable for Storyblok MA/CDA.
func DefaultTransportOptionsFromEnv() TransportOptions {
    // Defaults: host-level safety should not bottleneck per-space read/write buckets.
    // Set combined host ceiling high enough to accommodate ~7 read + ~7 write.
    maLimit := Limit{RPS: 14, Burst: 14}
    cdaLimit := Limit{RPS: 20, Burst: 20}

	// Environment overrides for MA
	if v := strings.TrimSpace(os.Getenv("SB_MA_RPS")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			maLimit.RPS = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("SB_MA_BURST")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maLimit.Burst = n
		}
	}

	// Retry/backoff tuning via env
	retryMax := 4
	if v := strings.TrimSpace(os.Getenv("SB_MA_RETRY_MAX")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			retryMax = n
		}
	}
	backoffBase := 250 * time.Millisecond
	if v := strings.TrimSpace(os.Getenv("SB_MA_RETRY_BASE_MS")); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			backoffBase = time.Duration(ms) * time.Millisecond
		}
	}
	backoffCap := 5 * time.Second
	if v := strings.TrimSpace(os.Getenv("SB_MA_RETRY_CAP_MS")); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			backoffCap = time.Duration(ms) * time.Millisecond
		}
	}

	return TransportOptions{
		RetryMax:    retryMax,
		BackoffBase: backoffBase,
		BackoffCap:  backoffCap,
		Clock:       realClock{},
		JitterFn: func(base time.Duration, attempt int) time.Duration {
			// Full jitter on top of base backoff
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			if base <= 0 {
				return 0
			}
			return time.Duration(r.Int63n(base.Nanoseconds()))
		},
		Metrics: NewMetrics(),
		HostLimits: map[string]Limit{
			"mapi.storyblok.com": maLimit,
			"api.storyblok.com":  cdaLimit,
		},
	}
}

// tokenBucket is a simple per-host rate limiter with fractional tokens.
type tokenBucket struct {
	mu     sync.Mutex
	rps    float64
	burst  float64
	tokens float64
	last   time.Time
	clock  Clock
}

func newTokenBucket(lim Limit, clock Clock) *tokenBucket {
	return &tokenBucket{
		rps:    lim.RPS,
		burst:  float64(max(1, lim.Burst)),
		tokens: float64(max(1, lim.Burst)),
		last:   clock.Now(),
		clock:  clock,
	}
}

func (tb *tokenBucket) refillLocked(now time.Time) {
	delta := now.Sub(tb.last).Seconds() * tb.rps
	if delta > 0 {
		tb.tokens = math.Min(tb.burst, tb.tokens+delta)
		tb.last = now
	}
}

func (tb *tokenBucket) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tb.mu.Lock()
		now := tb.clock.Now()
		tb.refillLocked(now)
		if tb.tokens >= 1 {
			tb.tokens -= 1
			tb.mu.Unlock()
			return nil
		}
		// compute time to next token
		need := 1 - tb.tokens
		wait := time.Duration((need / tb.rps) * float64(time.Second))
		tb.mu.Unlock()
		// poll sleep in small intervals to observe context
		// cap minimal sleep to 5ms to avoid busy wait
		if wait <= 0 {
			wait = 5 * time.Millisecond
		}
		deadline := tb.clock.Now().Add(wait)
		for tb.clock.Now().Before(deadline) {
			if err := ctx.Err(); err != nil {
				return err
			}
			next := time.Until(tb.clock.Now().Add(5 * time.Millisecond))
			_ = next // not used by realClock; fake clocks can advance immediately
			tb.clock.Sleep(5 * time.Millisecond)
		}
	}
}

// adjustRPS nudges the limiter RPS within [min,max].
func (tb *tokenBucket) adjustRPS(delta, min, max float64) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	newRPS := tb.rps + delta
	if newRPS < min {
		newRPS = min
	}
	if newRPS > max {
		newRPS = max
	}
	tb.rps = newRPS
}

// RetryingLimiterTransport wraps a base RoundTripper with host-based rate limiting and retries.
type RetryingLimiterTransport struct {
	Base     http.RoundTripper
	Opts     TransportOptions
	limMu    sync.Mutex
	limiters map[string]*tokenBucket
}

func NewRetryingLimiterTransport(opts TransportOptions) *RetryingLimiterTransport {
	return &RetryingLimiterTransport{Opts: opts, limiters: make(map[string]*tokenBucket)}
}

func (t *RetryingLimiterTransport) getLimiter(host string) *tokenBucket {
	if host == "" {
		host = "_default_"
	}
	t.limMu.Lock()
	defer t.limMu.Unlock()
	if tb, ok := t.limiters[host]; ok {
		return tb
	}
	lim := Limit{RPS: 10, Burst: 10}
	if t.Opts.HostLimits != nil {
		if v, ok := t.Opts.HostLimits[host]; ok {
			lim = v
		}
	}
	tb := newTokenBucket(lim, t.clock())
	t.limiters[host] = tb
	return tb
}

func (t *RetryingLimiterTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *RetryingLimiterTransport) clock() Clock {
	if t.Opts.Clock != nil {
		return t.Opts.Clock
	}
	return realClock{}
}

func (t *RetryingLimiterTransport) jitter(base time.Duration, attempt int) time.Duration {
	if t.Opts.JitterFn != nil {
		return t.Opts.JitterFn(base, attempt)
	}
	return 0
}

// ensureGetBody guarantees the request body is replayable across retries.
func ensureGetBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || (req.Method != http.MethodPost && req.Method != http.MethodPut && req.Method != http.MethodPatch) {
		return nil, nil
	}
	if req.GetBody != nil {
		return nil, nil
	}
	// buffer body
	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	req.Body = io.NopCloser(bytes.NewReader(buf))
	return buf, nil
}

func (t *RetryingLimiterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Make body replayable if needed
	_, _ = ensureGetBody(req)

	lim := t.getLimiter(req.URL.Host)
	if t.Opts.Metrics != nil {
		t.Opts.Metrics.IncRequest(req.URL.Host, req.Method)
	}

	attempts := max(1, t.Opts.RetryMax+1) // e.g., RetryMax=4 => up to 5 tries total
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		// rate limit per attempt
		if err := lim.Wait(req.Context()); err != nil {
			return nil, err
		}

		// restore body for retries
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		resp, err := t.base().RoundTrip(req)
		if err != nil {
			// Retry on transient network errors
			if isTransientNetErr(err) && attempt < attempts-1 {
				lastErr = err
				if rc := getRetryCounters(req.Context()); rc != nil {
					rc.Total++
					rc.Net++
				}
				t.sleepBackoff(attempt)
				// adaptive: back off slightly on network errors
				lim.adjustRPS(-0.1, 1, t.maxRPSForHost(req.URL.Host))
				continue
			}
			return nil, err
		}

		// Track status
		if t.Opts.Metrics != nil {
			t.Opts.Metrics.IncStatus(resp.StatusCode)
		}

		// adaptive: on clean 2xx, gently nudge RPS up to the configured ceiling
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			lim.adjustRPS(+0.02, 1, t.maxRPSForHost(req.URL.Host))
		}

		// Retry on HTTP 429/5xx (subset)
		if shouldRetryStatus(resp.StatusCode) && attempt < attempts-1 {
			// Respect Retry-After when present
			if ra := parseRetryAfter(resp.Header.Get("Retry-After"), t.clock().Now()); ra > 0 {
				if t.Opts.Metrics != nil {
					t.Opts.Metrics.IncRetry()
					t.Opts.Metrics.AddBackoff(ra)
				}
				if rc := getRetryCounters(req.Context()); rc != nil {
					rc.Total++
					if resp.StatusCode == 429 {
						rc.Status429++
					} else if resp.StatusCode >= 500 {
						rc.Status5xx++
					}
				}
				// adaptive: back off RPS slightly on 429/5xx
				lim.adjustRPS(-0.3, 1, t.maxRPSForHost(req.URL.Host))
				resp.Body.Close()
				t.clock().Sleep(minDur(ra, t.Opts.BackoffCap))
				continue
			}
			// Otherwise exponential backoff with jitter
			resp.Body.Close()
			t.sleepBackoff(attempt)
			if t.Opts.Metrics != nil {
				t.Opts.Metrics.IncRetry()
			}
			if rc := getRetryCounters(req.Context()); rc != nil {
				rc.Total++
				if resp.StatusCode == 429 {
					rc.Status429++
				} else if resp.StatusCode >= 500 {
					rc.Status5xx++
				}
			}
			// adaptive: back off RPS slightly on 429/5xx
			lim.adjustRPS(-0.2, 1, t.maxRPSForHost(req.URL.Host))
			continue
		}

		return resp, nil
	}
	if lastErr == nil {
		lastErr = errors.New("max retries exceeded")
	}
	return nil, lastErr
}

func (t *RetryingLimiterTransport) sleepBackoff(attempt int) {
	base := t.Opts.BackoffBase
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	cap := t.Opts.BackoffCap
	if cap <= 0 {
		cap = 5 * time.Second
	}
	// exponential backoff: base * 2^attempt
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	delay = minDur(delay, cap)
	jit := t.jitter(delay, attempt)
	t.clock().Sleep(minDur(delay+jit, cap))
	if t.Opts.Metrics != nil {
		t.Opts.Metrics.AddBackoff(minDur(delay+jit, cap))
	}
}

func (t *RetryingLimiterTransport) maxRPSForHost(host string) float64 {
	if host == "" {
		host = "_default_"
	}
	if t.Opts.HostLimits != nil {
		if lim, ok := t.Opts.HostLimits[host]; ok {
			if lim.RPS > 0 {
				return lim.RPS
			}
		}
	}
	return 10
}

func isTransientNetErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout() || ne.Temporary()
	}
	// Sometimes wrapped or stringly-typed
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "temporary") || strings.Contains(msg, "transient")
}

func shouldRetryStatus(code int) bool {
	return code == 429 || code == 502 || code == 503 || code == 504
}

func parseRetryAfter(h string, now time.Time) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	// Integer seconds
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	// HTTP-date
	if when, err := http.ParseTime(h); err == nil {
		d := when.Sub(now)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
