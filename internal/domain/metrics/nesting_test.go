package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestNesting_Simple(t *testing.T) {
	// Arrange — Add has a single block (the function body).
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewNesting().Analyze(context.Background(), fn, domain.MetricOptions{})

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

func TestNesting_Nested(t *testing.T) {
	// Arrange — see testdata/nested package comment: four-deep nesting.
	fn := findFunc(t, parseFixture(t, "nested"), "Nested")

	// Act
	score, err := NewNesting().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — function body + outer for + if + inner for + inner if = depth 5.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 5 {
		t.Errorf("Value = %v, want 5", score.Value)
	}
}

func TestNesting_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "nested"), "Nested")

	// Act — depth 5, threshold 4 → Warning.
	score, err := NewNesting().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 4})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestNesting_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewNesting().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
