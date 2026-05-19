package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestDifficulty_Simple(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewDifficulty().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value <= 0 {
		t.Errorf("Value = %v, want > 0", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none (well below threshold)", score.Findings)
	}
}

func TestDifficulty_BranchyHigherThanSimple(t *testing.T) {
	// Arrange
	simple := findFunc(t, parseFixture(t, "simple"), "Add")
	branchy := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act
	simpleScore, _ := NewDifficulty().Analyze(context.Background(), simple, domain.MetricOptions{})
	branchyScore, _ := NewDifficulty().Analyze(context.Background(), branchy, domain.MetricOptions{})

	// Assert
	if branchyScore.Value <= simpleScore.Value {
		t.Errorf("Branchy difficulty %v should exceed Add %v", branchyScore.Value, simpleScore.Value)
	}
}

func TestDifficulty_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act
	score, err := NewDifficulty().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 1})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestDifficulty_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewDifficulty().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
