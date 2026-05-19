package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestReturns_Simple(t *testing.T) {
	// Arrange — Add has one return.
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewReturns().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 1 {
		t.Errorf("Value = %v, want 1", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none", score.Findings)
	}
}

func TestReturns_MultiReturn(t *testing.T) {
	// Arrange — see testdata/multi_return: five returns.
	fn := findFunc(t, parseFixture(t, "multi_return"), "Classify")

	// Act
	score, err := NewReturns().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 5 {
		t.Errorf("Value = %v, want 5", score.Value)
	}
}

func TestReturns_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "multi_return"), "Classify")

	// Act — 5 returns, threshold 3 → Warning.
	score, err := NewReturns().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 3})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestReturns_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewReturns().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
