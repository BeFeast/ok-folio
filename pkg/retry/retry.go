package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// Do executes the given function with retry logic
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Calculate backoff delay
		delay := calculateBackoff(attempt, cfg)

		// Sleep with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// DoWithValue executes a function that returns a value with retry logic
func DoWithValue[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var (
		result  T
		lastErr error
	)

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		val, err := fn()
		if err == nil {
			return val, nil
		}

		lastErr = err

		if attempt == cfg.MaxAttempts {
			break
		}

		delay := calculateBackoff(attempt, cfg)

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	return result, fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

func calculateBackoff(attempt int, cfg Config) time.Duration {
	// Exponential backoff: initialDelay * (multiplier ^ (attempt - 1))
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}
