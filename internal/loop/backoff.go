// Package loop implements the Claude CLI execution loop.
package loop

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

const (
	DefaultInitialBackoff = 30 * time.Second
	DefaultMaxBackoff     = 10 * time.Minute
	DefaultMaxRetries     = 8
	DefaultJitterFraction = 0.2 // ±20% jitter
)

// Backoff tracks exponential backoff state for API 529 (overloaded) errors.
// It is safe for concurrent use.
type Backoff struct {
	mu             sync.Mutex
	initialBackoff time.Duration
	maxBackoff     time.Duration
	maxRetries     int
	jitterFraction float64
	consecutiveHits int
}

// NewBackoff creates a Backoff with default parameters:
// initial=30s, max=10min, maxRetries=8, jitter=±20%.
func NewBackoff() *Backoff {
	return &Backoff{
		initialBackoff: DefaultInitialBackoff,
		maxBackoff:     DefaultMaxBackoff,
		maxRetries:     DefaultMaxRetries,
		jitterFraction: DefaultJitterFraction,
	}
}

// BackoffOption configures a Backoff.
type BackoffOption func(*Backoff)

// WithInitialBackoff sets the initial backoff duration.
func WithInitialBackoff(d time.Duration) BackoffOption {
	return func(b *Backoff) { b.initialBackoff = d }
}

// WithMaxBackoff sets the maximum backoff duration.
func WithMaxBackoff(d time.Duration) BackoffOption {
	return func(b *Backoff) { b.maxBackoff = d }
}

// WithMaxRetries sets the maximum number of consecutive retries.
func WithMaxRetries(n int) BackoffOption {
	return func(b *Backoff) { b.maxRetries = n }
}

// WithJitterFraction sets the jitter fraction (e.g., 0.2 for ±20%).
func WithJitterFraction(f float64) BackoffOption {
	return func(b *Backoff) { b.jitterFraction = f }
}

// NewBackoffWithOptions creates a Backoff with custom parameters.
func NewBackoffWithOptions(opts ...BackoffOption) *Backoff {
	b := NewBackoff()
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Next records a 529 hit and returns the backoff duration and retry number.
// Returns (duration, retryNumber, exceeded) where exceeded is true if
// max retries have been reached.
func (b *Backoff) Next() (time.Duration, int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.consecutiveHits++
	retryNum := b.consecutiveHits

	if retryNum > b.maxRetries {
		return 0, retryNum, true
	}

	// Exponential: initial * 2^(hits-1), capped at max
	exp := math.Pow(2, float64(retryNum-1))
	backoff := time.Duration(float64(b.initialBackoff) * exp)
	if backoff > b.maxBackoff {
		backoff = b.maxBackoff
	}

	// Apply jitter: ±jitterFraction
	jitter := (rand.Float64()*2 - 1) * b.jitterFraction * float64(backoff)
	backoff = time.Duration(float64(backoff) + jitter)

	return backoff, retryNum, false
}

// Reset clears the consecutive hit counter (call on successful iteration).
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveHits = 0
}

// ConsecutiveHits returns the current consecutive 529 count.
func (b *Backoff) ConsecutiveHits() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.consecutiveHits
}

// MaxRetries returns the configured max retry count.
func (b *Backoff) MaxRetries() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxRetries
}
