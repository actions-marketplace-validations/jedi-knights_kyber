package metrics

import (
	"context"
	"fmt"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Nesting reports the deepest block nesting level inside a function body —
// each `*ast.BlockStmt` adds one level. Already used internally as a
// sub-signal of Readability; promoted here so it can be gated independently.
//
// Higher is worse; functions exceeding the threshold are candidates for
// extraction.
type Nesting struct{}

// NewNesting constructs the metric.
func NewNesting() *Nesting { return &Nesting{} }

// ID returns the metric's stable identifier.
func (Nesting) ID() string { return "nesting" }

// Name returns the metric's human-readable name.
func (Nesting) Name() string { return "Maximum Nesting Depth" }

// Description returns a one-line description of what the metric measures.
func (Nesting) Description() string {
	return "Deepest block nesting level inside the function body."
}

// DefaultThreshold of 4 matches Readability's nesting sub-signal default.
func (Nesting) DefaultThreshold() float64 { return 4 }

// HigherIsWorse reports that deeper nesting indicates worse code.
func (Nesting) HigherIsWorse() bool { return true }

// Analyze returns the maximum nesting depth of fn.FuncDecl. Emits a
// Warning when the value exceeds opts.Threshold; severity escalates to
// Error at ≥ 2× threshold.
func (m Nesting) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	depth := computeNestingDepth(fn.FuncDecl)
	score := domain.Score{
		MetricID: m.ID(),
		Function: fn,
		Value:    float64(depth),
	}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if float64(depth) > threshold {
		sev := domain.SeverityWarning
		if float64(depth) >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("nesting depth %d exceeds threshold %g", depth, threshold),
		}}
	}
	return score, nil
}
