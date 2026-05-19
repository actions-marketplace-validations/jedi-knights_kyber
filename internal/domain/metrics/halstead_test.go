package metrics

import (
	"context"
	"math"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestHalstead_Simple(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	fn := findFunc(t, funcs, "Add")

	// Act
	score, err := NewHalstead().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// Add has very few unique operators/operands; Volume should be small.
	// Sanity bounds rather than exact value since tokenization choices vary.
	if score.Value <= 0 {
		t.Errorf("Value = %v, want > 0", score.Value)
	}
	if score.Value > 100 {
		t.Errorf("Value = %v, want < 100 for a trivial function", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none (below default threshold)", score.Findings)
	}
}

func TestHalstead_BranchyHigherThanSimple(t *testing.T) {
	// Arrange
	simpleFn := findFunc(t, parseFixture(t, "simple"), "Add")
	branchyFn := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act
	simpleScore, _ := NewHalstead().Analyze(context.Background(), simpleFn, domain.MetricOptions{})
	branchyScore, _ := NewHalstead().Analyze(context.Background(), branchyFn, domain.MetricOptions{})

	// Assert — Branchy has many more tokens and unique identifiers; volume must be higher.
	if branchyScore.Value <= simpleScore.Value {
		t.Errorf("Branchy volume %v should exceed Add volume %v", branchyScore.Value, simpleScore.Value)
	}
}

func TestHalstead_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act — force a finding with a tiny threshold.
	score, err := NewHalstead().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 10})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if math.IsNaN(score.Value) || math.IsInf(score.Value, 0) {
		t.Fatalf("Value = %v, want finite", score.Value)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestHalstead_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewHalstead().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
