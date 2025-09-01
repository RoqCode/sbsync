package sync

import (
	"context"
	"sync"
	"time"
)

// SpaceLimiter provides per-space token buckets for reads and writes.
// It is intentionally simple: fixed RPS ceilings with gentle nudging via Nudge*.
type SpaceLimiter struct {
	mu     sync.Mutex
	spaces map[int]*spaceBuckets
	// defaults
	defReadRPS  float64
	defWriteRPS float64
	burst       float64
}

type spaceBuckets struct {
	read  *tbucket
	write *tbucket
}

type tbucket struct {
	mu     sync.Mutex
	rps    float64
	burst  float64
	tokens float64
	last   time.Time
}

func newBucket(rps, burst float64) *tbucket {
	now := time.Now()
	return &tbucket{rps: rps, burst: burst, tokens: burst, last: now}
}

func (b *tbucket) refillLocked(now time.Time) {
	delta := now.Sub(b.last).Seconds() * b.rps
	if delta > 0 {
		b.tokens += delta
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = now
	}
}

func (b *tbucket) wait(ctx context.Context) error {
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		b.mu.Lock()
		now := time.Now()
		b.refillLocked(now)
		if b.tokens >= 1 {
			b.tokens -= 1
			b.mu.Unlock()
			return nil
		}
		need := 1 - b.tokens
		rps := b.rps
		b.mu.Unlock()
		if rps <= 0 {
			rps = 1
		}
		wait := time.Duration((need / rps) * float64(time.Second))
		if wait < 5*time.Millisecond {
			wait = 5 * time.Millisecond
		}
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
}

func NewSpaceLimiter(readRPS, writeRPS float64, burst int) *SpaceLimiter {
	if readRPS <= 0 {
		readRPS = 7
	}
	if writeRPS <= 0 {
		writeRPS = 7
	}
	if burst <= 0 {
		burst = 7
	}
	return &SpaceLimiter{spaces: make(map[int]*spaceBuckets), defReadRPS: readRPS, defWriteRPS: writeRPS, burst: float64(burst)}
}

func (sl *SpaceLimiter) get(spaceID int) *spaceBuckets {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sb, ok := sl.spaces[spaceID]
	if ok {
		return sb
	}
	sb = &spaceBuckets{
		read:  newBucket(sl.defReadRPS, sl.burst),
		write: newBucket(sl.defWriteRPS, sl.burst),
	}
	sl.spaces[spaceID] = sb
	return sb
}

func (sl *SpaceLimiter) WaitRead(ctx context.Context, spaceID int) error {
	return sl.get(spaceID).read.wait(ctx)
}
func (sl *SpaceLimiter) WaitWrite(ctx context.Context, spaceID int) error {
	return sl.get(spaceID).write.wait(ctx)
}

// Nudge increases/decreases the effective RPS within [min,max].
func (sl *SpaceLimiter) NudgeRead(spaceID int, delta, min, max float64) {
	sl.nudge(sl.get(spaceID).read, delta, min, max)
}
func (sl *SpaceLimiter) NudgeWrite(spaceID int, delta, min, max float64) {
	sl.nudge(sl.get(spaceID).write, delta, min, max)
}

func (sl *SpaceLimiter) nudge(b *tbucket, delta, min, max float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.rps + delta
	if r < min {
		r = min
	}
	if r > max {
		r = max
	}
	b.rps = r
}
