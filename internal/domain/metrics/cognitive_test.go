package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestCognitive_Simple(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	fn := findFunc(t, funcs, "Add")

	// Act
	score, err := NewCognitive().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 0 {
		t.Errorf("Value = %v, want 0 (straight-line function)", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none", score.Findings)
	}
}

func TestCognitive_Branchy(t *testing.T) {
	// Arrange — see testdata/complex package comment for hand-computed value.
	funcs := parseFixture(t, "complex")
	fn := findFunc(t, funcs, "Branchy")

	// Act
	score, err := NewCognitive().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	// 6 control structures (no nesting) + 2 boolean-operator sequences = 8.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 8 {
		t.Errorf("Value = %v, want 8", score.Value)
	}
}

func TestCognitive_Nested(t *testing.T) {
	// Arrange — see testdata/nested package comment for hand-computed value.
	funcs := parseFixture(t, "nested")
	fn := findFunc(t, funcs, "Nested")

	// Act
	score, err := NewCognitive().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	// Nesting penalty: 1 + 2 + 3 + 4 = 10.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 10 {
		t.Errorf("Value = %v, want 10", score.Value)
	}
}

func TestCognitive_ExceedsThreshold(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "nested")
	fn := findFunc(t, funcs, "Nested")

	// Act — Nested scores 10, threshold of 5 forces a finding.
	score, err := NewCognitive().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 5})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
	if score.Findings[0].Severity != domain.SeverityError {
		t.Errorf("Severity = %v, want Error (10 ≥ 2×5)", score.Findings[0].Severity)
	}
}

func TestCognitive_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewCognitive().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
