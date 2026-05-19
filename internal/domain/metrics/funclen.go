package metrics

import (
	"context"
	"fmt"

	"github.com/jedi-knights/kyber/internal/domain"
)

// FuncLen reports the effective line count of a function — total raw
// SourceLines minus blank lines and lines whose first non-whitespace token
// is a `//` or `/*` comment. The result is closer to "what a reviewer
// actually reads" than the raw line span.
//
// Higher is worse.
type FuncLen struct{}

// NewFuncLen constructs the metric.
func NewFuncLen() *FuncLen { return &FuncLen{} }

// ID returns the metric's stable identifier.
func (FuncLen) ID() string { return "funclen" }

// Name returns the metric's human-readable name.
func (FuncLen) Name() string { return "Function Length" }

// Description returns a one-line description of what the metric measures.
func (FuncLen) Description() string {
	return "Non-blank, non-comment line count of the function body."
}

// DefaultThreshold of 40 matches Readability's length sub-signal default
// and the rules/go-conventions.md guidance of functions ≤ 40 lines.
func (FuncLen) DefaultThreshold() float64 { return 40 }

// HigherIsWorse reports that longer functions indicate worse code.
func (FuncLen) HigherIsWorse() bool { return true }

// Analyze counts effective source lines in fn.SourceLines. Emits a Warning
// when the count exceeds opts.Threshold; severity escalates to Error at
// ≥ 2× threshold.
func (m FuncLen) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	n := effectiveLineCount(fn.SourceLines)
	score := domain.Score{
		MetricID: m.ID(),
		Function: fn,
		Value:    float64(n),
	}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if float64(n) > threshold {
		sev := domain.SeverityWarning
		if float64(n) >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("function length %d exceeds threshold %g", n, threshold),
		}}
	}
	return score, nil
}

// effectiveLineCount returns the number of lines that are neither blank nor
// pure comment. Uses trimLeftSpace and startsWithComment from readability.go.
func effectiveLineCount(lines []string) int {
	count := 0
	for _, raw := range lines {
		trimmed := trimLeftSpace(raw)
		if trimmed == "" || startsWithComment(trimmed) {
			continue
		}
		count++
	}
	return count
}
