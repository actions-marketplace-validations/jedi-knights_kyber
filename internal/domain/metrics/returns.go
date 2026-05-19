package metrics

import (
	"context"
	"fmt"
	"go/ast"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Returns counts return statements in a function body. Many early returns
// can be intentional (guard clauses) or accidental (a function trying to do
// too much) — the metric leaves the judgment to the threshold.
//
// Higher is worse.
type Returns struct{}

// NewReturns constructs the metric.
func NewReturns() *Returns { return &Returns{} }

// ID returns the metric's stable identifier.
func (Returns) ID() string { return "returns" }

// Name returns the metric's human-readable name.
func (Returns) Name() string { return "Return Statement Count" }

// Description returns a one-line description of what the metric measures.
func (Returns) Description() string {
	return "Number of return statements anywhere in the function body."
}

// DefaultThreshold of 4 is a commonly cited per-function guard against
// excessive early-return chains.
func (Returns) DefaultThreshold() float64 { return 4 }

// HigherIsWorse reports that more returns indicate worse code.
func (Returns) HigherIsWorse() bool { return true }

// Analyze counts *ast.ReturnStmt nodes in fn.FuncDecl.Body. Emits a
// Warning when the count exceeds opts.Threshold; severity escalates to
// Error at ≥ 2× threshold.
func (m Returns) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	n := countReturns(fn.FuncDecl)
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
			Message:  fmt.Sprintf("return statement count %d exceeds threshold %g", n, threshold),
		}}
	}
	return score, nil
}

func countReturns(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 0
	}
	count := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Don't descend into nested function literals — their returns are
		// theirs, not the enclosing function's.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if _, ok := n.(*ast.ReturnStmt); ok {
			count++
		}
		return true
	})
	return count
}
