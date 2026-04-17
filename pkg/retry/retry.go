package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Multiplier     float64       // Backoff multiplier for exponential backoff
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
	}
}

// NetworkConfig returns a retry configuration optimized for network operations
func NetworkConfig() *Config {
	return &Config{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
	}
}

// Operation is a function that can be retried
type Operation func() error

// OperationWithResult is a function that returns a result and can be retried
type OperationWithResult[T any] func() (T, error)

// Do executes an operation with retry logic and exponential backoff
func Do(op Operation, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff duration with exponential backoff
		backoff := calculateBackoff(attempt, config)
		time.Sleep(backoff)
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// DoWithContext executes an operation with retry logic, exponential backoff, and context support
func DoWithContext(ctx context.Context, op Operation, config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// Execute the operation
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff duration with exponential backoff
		backoff := calculateBackoff(attempt, config)

		// Sleep with context awareness
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// DoWithResult executes an operation that returns a result with retry logic
func DoWithResult[T any](op OperationWithResult[T], config *Config) (T, error) {
	if config == nil {
		config = DefaultConfig()
	}

	var result T
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		res, err := op()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff duration with exponential backoff
		backoff := calculateBackoff(attempt, config)
		time.Sleep(backoff)
	}

	return result, fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// DoWithResultAndContext executes an operation that returns a result with retry logic and context support
func DoWithResultAndContext[T any](ctx context.Context, op OperationWithResult[T], config *Config) (T, error) {
	if config == nil {
		config = DefaultConfig()
	}

	var result T
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}

		// Execute the operation
		res, err := op()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff duration with exponential backoff
		backoff := calculateBackoff(attempt, config)

		// Sleep with context awareness
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("operation cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}

	return result, fmt.Errorf("operation failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// calculateBackoff calculates the backoff duration for a given attempt
func calculateBackoff(attempt int, config *Config) time.Duration {
	// Calculate exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))
	
	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	return time.Duration(backoff)
}
