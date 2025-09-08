package sync

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeClock allows deterministic control of time passage for testing
type fakeClock struct {
	now   time.Time
	slept time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(0, 0)}
}

func (fc *fakeClock) Now() time.Time {
	return fc.now
}

func (fc *fakeClock) Sleep(d time.Duration) {
	fc.now = fc.now.Add(d)
	fc.slept += d
}

// mockBucket wraps tbucket to allow testing with fake clock
type mockBucket struct {
	*tbucket
	clock *fakeClock
}

func newMockBucket(rps, burst float64, clock *fakeClock) *mockBucket {
	b := newBucket(rps, burst)
	b.last = clock.Now()
	return &mockBucket{tbucket: b, clock: clock}
}

func (mb *mockBucket) wait(ctx context.Context) error {
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		mb.mu.Lock()
		now := mb.clock.Now()
		mb.refillLocked(now)
		if mb.tokens >= 1 {
			mb.tokens -= 1
			mb.mu.Unlock()
			return nil
		}
		need := 1 - mb.tokens
		rps := mb.rps
		mb.mu.Unlock()
		if rps <= 0 {
			rps = 1
		}
		wait := time.Duration((need / rps) * float64(time.Second))
		if wait < 5*time.Millisecond {
			wait = 5 * time.Millisecond
		}
		mb.clock.Sleep(wait)
	}
}

func TestNewSpaceLimiter(t *testing.T) {
	tests := []struct {
		name      string
		readRPS   float64
		writeRPS  float64
		burst     int
		wantRead  float64
		wantWrite float64
		wantBurst float64
	}{
		{
			name:      "default values",
			readRPS:   0,
			writeRPS:  0,
			burst:     0,
			wantRead:  7,
			wantWrite: 7,
			wantBurst: 7,
		},
		{
			name:      "custom values",
			readRPS:   10,
			writeRPS:  5,
			burst:     15,
			wantRead:  10,
			wantWrite: 5,
			wantBurst: 15,
		},
		{
			name:      "negative values use defaults",
			readRPS:   -1,
			writeRPS:  -5,
			burst:     -10,
			wantRead:  7,
			wantWrite: 7,
			wantBurst: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := NewSpaceLimiter(tt.readRPS, tt.writeRPS, tt.burst)
			if sl.defReadRPS != tt.wantRead {
				t.Errorf("defReadRPS = %v, want %v", sl.defReadRPS, tt.wantRead)
			}
			if sl.defWriteRPS != tt.wantWrite {
				t.Errorf("defWriteRPS = %v, want %v", sl.defWriteRPS, tt.wantWrite)
			}
			if sl.burst != tt.wantBurst {
				t.Errorf("burst = %v, want %v", sl.burst, tt.wantBurst)
			}
		})
	}
}

func TestSpaceLimiter_Get(t *testing.T) {
	sl := NewSpaceLimiter(5, 3, 10)

	// Test getting same space multiple times returns same buckets
	buckets1 := sl.get(123)
	buckets2 := sl.get(123)
	if buckets1 != buckets2 {
		t.Error("Expected same buckets instance for same space ID")
	}

	// Test different spaces get different buckets
	buckets3 := sl.get(456)
	if buckets1 == buckets3 {
		t.Error("Expected different buckets for different space IDs")
	}

	// Test buckets have correct initial values
	if buckets1.read.rps != 5 {
		t.Errorf("read RPS = %v, want 5", buckets1.read.rps)
	}
	if buckets1.write.rps != 3 {
		t.Errorf("write RPS = %v, want 3", buckets1.write.rps)
	}
	if buckets1.read.burst != 10 {
		t.Errorf("read burst = %v, want 10", buckets1.read.burst)
	}
	if buckets1.write.burst != 10 {
		t.Errorf("write burst = %v, want 10", buckets1.write.burst)
	}
}

func TestTokenBucket_RefillLocked(t *testing.T) {
	fc := newFakeClock()
	b := newMockBucket(2, 5, fc) // 2 RPS, burst 5

	// Initial state: full burst
	if b.tokens != 5 {
		t.Errorf("initial tokens = %v, want 5", b.tokens)
	}

	// Advance time by 1 second, should add 2 tokens (but capped at burst)
	fc.now = fc.now.Add(time.Second)
	b.refillLocked(fc.now)
	if b.tokens != 5 {
		t.Errorf("tokens after 1s = %v, want 5 (capped)", b.tokens)
	}

	// Consume 3 tokens
	b.tokens = 2

	// Advance time by 1 second, should add 2 tokens
	fc.now = fc.now.Add(time.Second)
	b.refillLocked(fc.now)
	if b.tokens != 4 {
		t.Errorf("tokens after refill = %v, want 4", b.tokens)
	}

	// Advance time by 0.5 seconds, should add 1 token
	fc.now = fc.now.Add(500 * time.Millisecond)
	b.refillLocked(fc.now)
	if b.tokens != 5 {
		t.Errorf("tokens after 0.5s = %v, want 5", b.tokens)
	}
}

func TestTokenBucket_Wait(t *testing.T) {
	fc := newFakeClock()
	b := newMockBucket(2, 3, fc) // 2 RPS, burst 3

	// First request should succeed immediately (uses burst)
	ctx := context.Background()
	err := b.wait(ctx)
	if err != nil {
		t.Errorf("first wait failed: %v", err)
	}
	if b.tokens != 2 {
		t.Errorf("tokens after first wait = %v, want 2", b.tokens)
	}

	// Second request should succeed immediately (uses burst)
	err = b.wait(ctx)
	if err != nil {
		t.Errorf("second wait failed: %v", err)
	}
	if b.tokens != 1 {
		t.Errorf("tokens after second wait = %v, want 1", b.tokens)
	}

	// Third request should succeed immediately (uses last burst token)
	err = b.wait(ctx)
	if err != nil {
		t.Errorf("third wait failed: %v", err)
	}
	if b.tokens != 0 {
		t.Errorf("tokens after third wait = %v, want 0", b.tokens)
	}

	// Fourth request should wait ~0.5 seconds (1 token at 2 RPS)
	startSleep := fc.slept
	err = b.wait(ctx)
	if err != nil {
		t.Errorf("fourth wait failed: %v", err)
	}
	expectedSleep := 500 * time.Millisecond
	if fc.slept-startSleep < expectedSleep-50*time.Millisecond || fc.slept-startSleep > expectedSleep+50*time.Millisecond {
		t.Errorf("fourth wait slept %v, want ~%v", fc.slept-startSleep, expectedSleep)
	}
}

func TestTokenBucket_Wait_ContextCancellation(t *testing.T) {
	fc := newFakeClock()
	b := newMockBucket(0.1, 1, fc) // Very slow rate to force waiting

	// Consume the burst token
	b.tokens = 0

	ctx, cancel := context.WithCancel(context.Background())

	// Start waiting in goroutine
	var wg sync.WaitGroup
	var waitErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		waitErr = b.wait(ctx)
	}()

	// Cancel context immediately (fake clock doesn't actually sleep)
	cancel()

	wg.Wait()

	if waitErr != context.Canceled {
		t.Errorf("wait error = %v, want context.Canceled", waitErr)
	}
}

func TestSpaceLimiter_Nudge(t *testing.T) {
	sl := NewSpaceLimiter(5, 3, 10)
	spaceID := 123

	// Test nudge read
	sl.NudgeRead(spaceID, 2, 1, 10)
	buckets := sl.get(spaceID)
	if buckets.read.rps != 7 {
		t.Errorf("read RPS after nudge +2 = %v, want 7", buckets.read.rps)
	}

	// Test nudge write
	sl.NudgeWrite(spaceID, -1, 1, 10)
	if buckets.write.rps != 2 {
		t.Errorf("write RPS after nudge -1 = %v, want 2", buckets.write.rps)
	}

	// Test nudge respects min bound
	sl.NudgeRead(spaceID, -10, 1, 10)
	if buckets.read.rps != 1 {
		t.Errorf("read RPS after nudge below min = %v, want 1", buckets.read.rps)
	}

	// Test nudge respects max bound
	sl.NudgeWrite(spaceID, 20, 1, 10)
	if buckets.write.rps != 10 {
		t.Errorf("write RPS after nudge above max = %v, want 10", buckets.write.rps)
	}
}

func TestSpaceLimiter_WaitRead_Write(t *testing.T) {
	sl := NewSpaceLimiter(2, 1, 3)
	spaceID := 123
	ctx := context.Background()

	// Test WaitRead
	err := sl.WaitRead(ctx, spaceID)
	if err != nil {
		t.Errorf("WaitRead failed: %v", err)
	}

	// Test WaitWrite
	err = sl.WaitWrite(ctx, spaceID)
	if err != nil {
		t.Errorf("WaitWrite failed: %v", err)
	}

	// Verify they use different buckets
	buckets := sl.get(spaceID)
	if buckets.read.rps != 2 {
		t.Errorf("read bucket RPS = %v, want 2", buckets.read.rps)
	}
	if buckets.write.rps != 1 {
		t.Errorf("write bucket RPS = %v, want 1", buckets.write.rps)
	}
}

func TestSpaceLimiter_ConcurrentAccess(t *testing.T) {
	sl := NewSpaceLimiter(10, 10, 20)
	spaceID := 123
	ctx := context.Background()

	// Test concurrent access to same space
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sl.WaitRead(ctx, spaceID)
			if err != nil {
				t.Errorf("concurrent WaitRead failed: %v", err)
			}
		}()
	}

	wg.Wait()

	// All requests should have succeeded
	buckets := sl.get(spaceID)
	// After 10 requests with burst 20, should still have tokens available
	if buckets.read.tokens < 10 {
		t.Errorf("tokens after concurrent access = %v, want >= 10", buckets.read.tokens)
	}
}

func TestSpaceLimiter_DifferentSpaces(t *testing.T) {
	sl := NewSpaceLimiter(5, 3, 10)
	ctx := context.Background()

	// Test that different spaces have independent rate limits
	err1 := sl.WaitRead(ctx, 1)
	err2 := sl.WaitRead(ctx, 2)

	if err1 != nil {
		t.Errorf("WaitRead for space 1 failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("WaitRead for space 2 failed: %v", err2)
	}

	// Verify they have separate buckets
	buckets1 := sl.get(1)
	buckets2 := sl.get(2)
	if buckets1 == buckets2 {
		t.Error("Expected different buckets for different spaces")
	}
}

func TestTokenBucket_EdgeCases(t *testing.T) {
	fc := newFakeClock()

	// Test with zero RPS (should default to 1)
	b := newMockBucket(0, 5, fc)
	b.rps = 0
	ctx := context.Background()

	// Consume burst tokens
	b.tokens = 0

	// Should still work with minimum RPS (but will take a long time)
	// Use a timeout to avoid hanging
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err := b.wait(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("wait with zero RPS should timeout, got: %v", err)
	}
}

func TestSpaceLimiter_Integration(t *testing.T) {
	// Test realistic usage pattern
	sl := NewSpaceLimiter(7, 7, 7)
	spaceID := 123
	ctx := context.Background()

	// Simulate burst of requests
	for i := 0; i < 7; i++ {
		err := sl.WaitRead(ctx, spaceID)
		if err != nil {
			t.Errorf("burst request %d failed: %v", i, err)
		}
	}

	// Next request should wait
	start := time.Now()
	err := sl.WaitRead(ctx, spaceID)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("post-burst request failed: %v", err)
	}

	// Should have waited approximately 1/7 second
	expectedWait := time.Second / 7
	if duration < expectedWait-50*time.Millisecond || duration > expectedWait+100*time.Millisecond {
		t.Errorf("wait duration = %v, want ~%v", duration, expectedWait)
	}
}

func TestDefaultLimitsForPlan(t *testing.T) {
	tests := []struct {
		plan  int
		wantR float64
		wantW float64
		wantB int
	}{
		{0, 4, 3, 3},  // dev
		{-1, 4, 3, 3}, // dev (defensive)
		{1, 7, 7, 7},  // paid
		{999, 7, 7, 7},
	}
	for _, tt := range tests {
		r, w, b := DefaultLimitsForPlan(tt.plan)
		if r != tt.wantR || w != tt.wantW || b != tt.wantB {
			t.Fatalf("plan=%d got r=%.0f w=%.0f b=%d; want r=%.0f w=%.0f b=%d", tt.plan, r, w, b, tt.wantR, tt.wantW, tt.wantB)
		}
	}
}
