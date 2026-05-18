package agent

import (
	"sync/atomic"
)

// IterationBudget provides atomic, thread-safe iteration counting.
// Inspired by Hermes Agent's IterationBudget pattern.
// Supports consume (decrement), refund (increment), and remaining queries.
type IterationBudget struct {
	remaining atomic.Int64
	max       int64
}

// NewIterationBudget creates a budget with the given maximum iterations.
// Default 90 matches Hermes default.
func NewIterationBudget(max int) *IterationBudget {
	if max <= 0 {
		max = 90
	}
	b := &IterationBudget{max: int64(max)}
	b.remaining.Store(int64(max))
	return b
}

// Consume atomically decrements the budget by 1.
// Returns true if budget was consumed (remaining > 0).
// Returns false if budget exhausted.
func (b *IterationBudget) Consume() bool {
	for {
		current := b.remaining.Load()
		if current <= 0 {
			return false
		}
		if b.remaining.CompareAndSwap(current, current-1) {
			return true
		}
	}
}

// Refund atomically increments the budget by 1.
// Used when a tool call fails and the iteration should not count.
func (b *IterationBudget) Refund() {
	for {
		current := b.remaining.Load()
		if current >= b.max {
			return // can't exceed max
		}
		if b.remaining.CompareAndSwap(current, current+1) {
			return
		}
	}
}

// Remaining returns the current remaining budget.
func (b *IterationBudget) Remaining() int {
	return int(b.remaining.Load())
}

// Used returns the number of iterations consumed.
func (b *IterationBudget) Used() int {
	return int(b.max - b.remaining.Load())
}

// Exhausted returns true if no budget remains.
func (b *IterationBudget) Exhausted() bool {
	return b.remaining.Load() <= 0
}

// Reset restores the budget to its maximum.
func (b *IterationBudget) Reset() {
	b.remaining.Store(b.max)
}
