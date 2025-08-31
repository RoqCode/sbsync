package sb

import "context"

// retryCtxKey is an unexported key type for storing retry counters in context.
type retryCtxKey struct{}

// RetryCounters holds per-request retry attribution that the transport updates.
type RetryCounters struct {
    Total     int64
    Status429 int64
    Status5xx int64
    Net       int64
}

// WithRetryCounters attaches a RetryCounters struct to the context so that the
// transport can attribute retries to this context specifically.
func WithRetryCounters(ctx context.Context, rc *RetryCounters) context.Context {
    if ctx == nil {
        ctx = context.Background()
    }
    return context.WithValue(ctx, retryCtxKey{}, rc)
}

// getRetryCounters fetches the counters from context if present.
func getRetryCounters(ctx context.Context) *RetryCounters {
    if ctx == nil {
        return nil
    }
    if v := ctx.Value(retryCtxKey{}); v != nil {
        if rc, ok := v.(*RetryCounters); ok {
            return rc
        }
    }
    return nil
}

