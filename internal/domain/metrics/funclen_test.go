package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestFuncLen_Simple(t *testing.T) {
	// Arrange — Add has one real line of code (`return a + b`).
	// Function signature and closing brace are counted too — see the metric
	// docstring for what's stripped.
	fn := findFunc(t, parseFixture(t, "simple"), "Add")

	// Act
	score, err := NewFuncLen().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — at least one line of body, well under threshold.
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value < 1 || score.Value > 10 {
		t.Errorf("Value = %v, want 1-10 for trivial Add", score.Value)
	}
	if len(score.Findings) != 0 {
		t.Errorf("Findings = %v, want none (below threshold)", score.Findings)
	}
}

func TestFuncLen_BranchyLongerThanSimple(t *testing.T) {
	// Arrange
	simple := findFunc(t, parseFixture(t, "simple"), "Add")
	branchy := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act
	simpleScore, _ := NewFuncLen().Analyze(context.Background(), simple, domain.MetricOptions{})
	branchyScore, _ := NewFuncLen().Analyze(context.Background(), branchy, domain.MetricOptions{})

	// Assert
	if branchyScore.Value <= simpleScore.Value {
		t.Errorf("Branchy len %v should exceed Add len %v", branchyScore.Value, simpleScore.Value)
	}
}

func TestFuncLen_ExceedsThreshold(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "complex"), "Branchy")

	// Act — force a finding with a tiny threshold.
	score, err := NewFuncLen().Analyze(context.Background(), fn, domain.MetricOptions{Threshold: 3})

	// Assert
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(score.Findings) != 1 {
		t.Fatalf("Findings = %v, want 1", score.Findings)
	}
}

func TestFuncLen_StripsBlankAndCommentLines(t *testing.T) {
	// Arrange
	fn := findFunc(t, parseFixture(t, "simple"), "Add")
	raw := fn.LineCount()

	// Act
	score, _ := NewFuncLen().Analyze(context.Background(), fn, domain.MetricOptions{})

	// Assert — the metric value must not exceed the raw line span.
	if int(score.Value) > raw {
		t.Errorf("Value = %v, raw line span = %d; metric must not exceed raw", score.Value, raw)
	}
}

func TestFuncLen_ContextCancellation(t *testing.T) {
	// Arrange
	funcs := parseFixture(t, "simple")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	_, err := NewFuncLen().Analyze(ctx, funcs[0], domain.MetricOptions{})

	// Assert
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}
