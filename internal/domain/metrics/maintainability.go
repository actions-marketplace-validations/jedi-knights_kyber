package metrics

import (
	"context"
	"fmt"
	"math"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Maintainability implements the Microsoft Maintainability Index — a single
// 0–100 composite of Halstead Volume, cyclomatic complexity, and lines of
// code. Higher is better; below 65 is the commonly cited yellow-flag line.
//
// Formula (Coleman/Oman 1994 variant used by Visual Studio):
//
//	raw = 171 - 5.2*ln(V) - 0.23*CC - 16.2*ln(LOC)
//	MI  = max(0, min(100, raw * 100 / 171))
//
// The 100/171 normalization plus the floor at 0 produces a stable 0–100
// scale; the original 1994 paper used the raw value which could be negative
// or exceed 171.
type Maintainability struct{}

// NewMaintainability constructs the metric.
func NewMaintainability() *Maintainability { return &Maintainability{} }

// ID returns the metric's stable identifier.
func (Maintainability) ID() string { return "maintainability" }

// Name returns the metric's human-readable name.
func (Maintainability) Name() string { return "Maintainability Index" }

// Description returns a one-line description of what the metric measures.
func (Maintainability) Description() string {
	return "Microsoft Maintainability Index — composite of Halstead Volume, cyclomatic, and LOC."
}

// DefaultThreshold of 65 matches Visual Studio's traffic-light boundary
// between yellow (moderate) and green (high) maintainability.
func (Maintainability) DefaultThreshold() float64 { return 65 }

// HigherIsWorse reports that smaller maintainability scores indicate worse code.
func (Maintainability) HigherIsWorse() bool { return false }

// Analyze computes MI by combining Halstead Volume, cyclomatic complexity,
// and effective line count. Emits a Warning when the score falls below
// opts.Threshold.
func (m Maintainability) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	v := computeHalsteadVolume(fn.SourceLines)
	cc := float64(computeComplexity(fn.FuncDecl))
	loc := math.Max(1, float64(fn.LineCount()))

	mi := normalizeMI(v, cc, loc)
	score := domain.Score{MetricID: m.ID(), Function: fn, Value: mi}

	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if mi < threshold {
		score.Findings = []domain.Finding{{
			Severity: domain.SeverityWarning,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("maintainability index %.0f below threshold %g", mi, threshold),
		}}
	}
	return score, nil
}

func normalizeMI(v, cc, loc float64) float64 {
	// ln(0) is undefined; guard with a small floor so trivial functions
	// don't NaN. Volume of 0 means no tokens — unreachable in practice for
	// a real function but possible for a body-less declaration.
	safeV := math.Max(v, 1)
	raw := 171 - 5.2*math.Log(safeV) - 0.23*cc - 16.2*math.Log(loc)
	mi := raw * 100 / 171
	if mi < 0 {
		return 0
	}
	if mi > 100 {
		return 100
	}
	return mi
}
