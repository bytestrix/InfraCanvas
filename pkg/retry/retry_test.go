package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		return nil
	}

	err := Do(op, DefaultConfig())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	config := &Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := Do(op, config)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_MaxRetriesExceeded(t *testing.T) {
	attempts := 0
	op := func() error {
		attempts++
		return errors.New("persistent error")
	}

	config := &Config{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := Do(op, config)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts != 3 { // MaxRetries + 1
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithContext_Success(t *testing.T) {
	ctx := context.Background()
	attempts := 0
	op := func() error {
		attempts++
		return nil
	}

	err := DoWithContext(ctx, op, DefaultConfig())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	op := func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return errors.New("error")
	}

	config := &Config{
		MaxRetries:     5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		Multiplier:     2.0,
	}

	err := DoWithContext(ctx, op, config)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	attempts := 0
	op := func() (string, error) {
		attempts++
		return "success", nil
	}

	result, err := DoWithResult(op, DefaultConfig())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("expected 'success', got %s", result)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoWithResult_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	op := func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, errors.New("temporary error")
		}
		return 42, nil
	}

	config := &Config{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
	}

	result, err := DoWithResult(op, config)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := &Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		Multiplier:     2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // Capped at MaxBackoff
		{5, 1 * time.Second}, // Capped at MaxBackoff
	}

	for _, tt := range tests {
		backoff := calculateBackoff(tt.attempt, config)
		if backoff != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, backoff)
		}
	}
}

func TestNetworkConfig(t *testing.T) {
	config := NetworkConfig()
	
	if config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", config.MaxRetries)
	}
	
	if config.InitialBackoff != 500*time.Millisecond {
		t.Errorf("expected InitialBackoff=500ms, got %v", config.InitialBackoff)
	}
	
	if config.MaxBackoff != 5*time.Second {
		t.Errorf("expected MaxBackoff=5s, got %v", config.MaxBackoff)
	}
	
	if config.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", config.Multiplier)
	}
}
