package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	attempts := 0
	fn := func() error {
		attempts++
		return nil
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got: %d", attempts)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	err := Do(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
}

func TestDo_FailureAfterMaxAttempts(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	expectedErr := errors.New("persistent error")
	attempts := 0
	fn := func() error {
		attempts++
		return expectedErr
	}

	err := Do(context.Background(), cfg, fn)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap %v, got: %v", expectedErr, err)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	fn := func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("error")
	}

	err := Do(ctx, cfg, fn)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts, got: %d", attempts)
	}
}

func TestDo_ContextTimeout(t *testing.T) {
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("error")
	}

	err := Do(ctx, cfg, fn)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestDoWithValue_Success(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	expectedValue := "success"
	attempts := 0
	fn := func() (string, error) {
		attempts++
		return expectedValue, nil
	}

	result, err := DoWithValue(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result != expectedValue {
		t.Errorf("Expected %s, got: %s", expectedValue, result)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got: %d", attempts)
	}
}

func TestDoWithValue_SuccessAfterRetries(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	expectedValue := 42
	attempts := 0
	fn := func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("temporary error")
		}
		return expectedValue, nil
	}

	result, err := DoWithValue(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result != expectedValue {
		t.Errorf("Expected %d, got: %d", expectedValue, result)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
}

func TestDoWithValue_FailureAfterMaxAttempts(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	expectedErr := errors.New("persistent error")
	attempts := 0
	fn := func() (int, error) {
		attempts++
		return 0, expectedErr
	}

	result, err := DoWithValue[int](context.Background(), cfg, fn)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if result != 0 {
		t.Errorf("Expected zero value, got: %d", result)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts)
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to wrap %v, got: %v", expectedErr, err)
	}
}

func TestDoWithValue_ContextCancellation(t *testing.T) {
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	fn := func() (string, error) {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return "", errors.New("error")
	}

	result, err := DoWithValue(ctx, cfg, fn)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string, got: %s", result)
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		cfg      Config
		expected time.Duration
	}{
		{
			name:    "first attempt",
			attempt: 1,
			cfg: Config{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   2.0,
			},
			expected: 100 * time.Millisecond,
		},
		{
			name:    "second attempt",
			attempt: 2,
			cfg: Config{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   2.0,
			},
			expected: 200 * time.Millisecond,
		},
		{
			name:    "third attempt",
			attempt: 3,
			cfg: Config{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   2.0,
			},
			expected: 400 * time.Millisecond,
		},
		{
			name:    "capped at max delay",
			attempt: 5,
			cfg: Config{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     500 * time.Millisecond,
				Multiplier:   2.0,
			},
			expected: 500 * time.Millisecond,
		},
		{
			name:    "different multiplier",
			attempt: 2,
			cfg: Config{
				InitialDelay: 100 * time.Millisecond,
				MaxDelay:     1 * time.Second,
				Multiplier:   3.0,
			},
			expected: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBackoff(tt.attempt, tt.cfg)
			if result != tt.expected {
				t.Errorf("Expected %v, got: %v", tt.expected, result)
			}
		})
	}
}

func TestDo_BackoffTiming(t *testing.T) {
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
	}

	attempts := 0
	start := time.Now()
	fn := func() error {
		attempts++
		return errors.New("error")
	}

	_ = Do(context.Background(), cfg, fn)
	elapsed := time.Since(start)

	// Should take at least: 50ms (1st backoff) + 100ms (2nd backoff) = 150ms
	expectedMin := 150 * time.Millisecond
	// Allow some tolerance for execution time
	expectedMax := 300 * time.Millisecond

	if elapsed < expectedMin {
		t.Errorf("Expected at least %v, got: %v", expectedMin, elapsed)
	}
	if elapsed > expectedMax {
		t.Errorf("Expected at most %v, got: %v", expectedMax, elapsed)
	}
}
