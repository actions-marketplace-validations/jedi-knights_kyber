package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestNPath_Simple(t *testing.T) {
	// Arrange — straight-line function has exactly one path.
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewNPath().Analyze(context.Background(), fn, domain.MetricOptions{})

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

func TestNPath_Branchy(t *testing.T) {
	// Arrange — see testdata/npath_branchy package comment for hand-computed value.
	fn := findFunc(t, parseFixture(t, "npath_branchy"), "Triple")

	// Act
	score, err := NewNPath().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — three sequential if-else blocks: 2 × 2 × 2 = 8.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value != 8 {
		t.Errorf("Value = %v, want 8", score.Value)
	}
}

func TestNPath_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "npath_branchy"), "Triple")

	// Act — Branchy scores 8, threshold of 6 forces a Warning (8 > 6 but < 2×6).
	score, err := NewNPath().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 6})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
	if score.Findings[0].Severity != domain.SeverityWarning {
		t.Errorf("Severity = %v, want Warning", score.Findings[0].Severity)
	}
}

func TestNPath_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewNPath().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
