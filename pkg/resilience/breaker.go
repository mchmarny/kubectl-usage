// Package resilience provides fault tolerance and resource management for large-scale operations
package resilience

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the current state of a circuit breaker
type CircuitBreakerState int32

const (
	StateClosed CircuitBreakerState = iota
	StateHalfOpen
	StateOpen
)

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
// Reference: https://microservices.io/patterns/reliability/circuit-breaker.html
type CircuitBreaker struct {
	name         string
	maxFailures  int32
	timeout      time.Duration
	currentState int32 // Use atomic operations for thread safety
	failureCount int32
	lastFailure  int64 // Unix timestamp
	successCount int32
}

// NewCircuitBreaker creates a new circuit breaker with specified parameters
func NewCircuitBreaker(name string, maxFailures int32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		timeout:     timeout,
	}
}

// Execute runs the provided function with circuit breaker protection
func (cb *CircuitBreaker) Execute(_ context.Context, fn func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}

	// Execute the function
	err := fn()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// canExecute determines if a request can be executed based on circuit breaker state
func (cb *CircuitBreaker) canExecute() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.currentState))

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		lastFailure := atomic.LoadInt64(&cb.lastFailure)
		if time.Now().Unix()-lastFailure >= int64(cb.timeout.Seconds()) {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&cb.currentState, int32(StateOpen), int32(StateHalfOpen)) {
				return true
			}
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordFailure increments failure count and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure() {
	failures := atomic.AddInt32(&cb.failureCount, 1)
	atomic.StoreInt64(&cb.lastFailure, time.Now().Unix())

	if failures >= cb.maxFailures {
		atomic.StoreInt32(&cb.currentState, int32(StateOpen))
		atomic.StoreInt32(&cb.successCount, 0)
	}
}

// recordSuccess increments success count and potentially closes the circuit
func (cb *CircuitBreaker) recordSuccess() {
	atomic.StoreInt32(&cb.failureCount, 0)

	state := CircuitBreakerState(atomic.LoadInt32(&cb.currentState))
	if state == StateHalfOpen {
		successCount := atomic.AddInt32(&cb.successCount, 1)
		// Require multiple successes before closing circuit
		if successCount >= 3 {
			atomic.StoreInt32(&cb.currentState, int32(StateClosed))
			atomic.StoreInt32(&cb.successCount, 0)
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.currentState))
}

// ResourcePool manages limited resources with backpressure
type ResourcePool struct {
	name         string
	maxMemoryMB  int64
	currentMemMB int64
	semaphore    chan struct{}
	mutex        sync.RWMutex
}

// NewResourcePool creates a resource pool with memory and concurrency limits
func NewResourcePool(name string, maxConcurrency int, maxMemoryMB int64) *ResourcePool {
	return &ResourcePool{
		name:        name,
		maxMemoryMB: maxMemoryMB,
		semaphore:   make(chan struct{}, maxConcurrency),
	}
}

// Acquire attempts to acquire resources from the pool
func (rp *ResourcePool) Acquire(ctx context.Context, estimatedMemMB int64) error {
	// Check memory limits
	if !rp.checkMemoryLimit(estimatedMemMB) {
		return fmt.Errorf("resource pool %s: insufficient memory (need %dMB, limit %dMB)",
			rp.name, estimatedMemMB, rp.maxMemoryMB)
	}

	// Acquire concurrency slot
	select {
	case rp.semaphore <- struct{}{}:
		// Successfully acquired
		rp.addMemoryUsage(estimatedMemMB)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns resources to the pool
func (rp *ResourcePool) Release(memoryMB int64) {
	rp.removeMemoryUsage(memoryMB)
	<-rp.semaphore
}

// checkMemoryLimit verifies if requested memory is available
func (rp *ResourcePool) checkMemoryLimit(requestedMB int64) bool {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()

	// Also check system memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Safe conversion to avoid integer overflow
	// Convert to MB first, then to int64 to avoid overflow
	allocMB := m.Alloc / 1024 / 1024
	var usedSystemMB int64
	if allocMB > math.MaxInt64 {
		usedSystemMB = math.MaxInt64
	} else {
		usedSystemMB = int64(allocMB) // #nosec G115 - safe after bounds check
	}

	return (rp.currentMemMB+requestedMB) <= rp.maxMemoryMB &&
		usedSystemMB < (rp.maxMemoryMB*80/100) // Keep 20% buffer
}

// addMemoryUsage adds to current memory usage
func (rp *ResourcePool) addMemoryUsage(memoryMB int64) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()
	rp.currentMemMB += memoryMB
}

// removeMemoryUsage removes from current memory usage
func (rp *ResourcePool) removeMemoryUsage(memoryMB int64) {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()
	rp.currentMemMB -= memoryMB
	if rp.currentMemMB < 0 {
		rp.currentMemMB = 0
	}
}

// GetStats returns current resource usage statistics
func (rp *ResourcePool) GetStats() (currentMemMB int64, concurrency int, maxConcurrency int) {
	rp.mutex.RLock()
	defer rp.mutex.RUnlock()

	return rp.currentMemMB, len(rp.semaphore), cap(rp.semaphore)
}

// RetryConfig defines retry behavior for resilient operations
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig provides sensible defaults for Kubernetes API operations
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// ExecuteWithRetry executes a function with exponential backoff retry
func ExecuteWithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Exponential backoff with jitter
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Increase delay for next attempt
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
