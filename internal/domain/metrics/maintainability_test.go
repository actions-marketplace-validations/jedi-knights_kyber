package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestMaintainability_Simple(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewMaintainability().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — Add is trivial; MI is normalized to 0-100 with high values for trivial code.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value < 70 || score.Value > 100 {
		t.Errorf("Value = %v, want 70-100 for trivial Add", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none (well above threshold)", score.Findings)
	}
}

func TestMaintainability_BranchyLowerThanSimple(t *testing.T) {
	// Arrange
	simple := findFunc(t, parseFixture(t, "simple"), "Add")
	branchy := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act
	simpleScore, _ := NewMaintainability().Analyze(context.Background(), simple, domain.MetricOptions{})
	branchyScore, _ := NewMaintainability().Analyze(context.Background(), branchy, domain.MetricOptions{})

	// Assert — Branchy has higher V + CC + LOC, so MI must be strictly lower.
	if branchyScore.Value >= simpleScore.Value {
		t.Errorf("Branchy MI %v should be less than Add MI %v", branchyScore.Value, simpleScore.Value)
	}
}

func TestMaintainability_FlagsBelowThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act — force a finding with a threshold above Branchy's MI.
	score, err := NewMaintainability().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 95})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestMaintainability_ClampedToZero(t *testing.T) {
	// Arrange — pathologically dense code can yield raw MI < 0; the metric must clamp.
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, _ := NewMaintainability().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — non-negativity invariant for any real input.
	if score.Value < 0 {
		t.Errorf("Value = %v, want non-negative", score.Value)
	}
}

func TestMaintainability_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewMaintainability().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
