// Package metrics holds the concrete Metric implementations shipped with
// kyber. Each metric is one file; adding a new metric is a single new file
// plus a registration line in all.go.
package metrics

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Cyclomatic implements McCabe's cyclomatic complexity: the count of linearly
// independent paths through a function, computed as 1 + (number of decision
// points). Decision points are if/for/range/case (non-default) clauses,
// select cases, and short-circuit boolean operators (&&, ||).
type Cyclomatic struct{}

// NewCyclomatic constructs the metric.
func NewCyclomatic() *Cyclomatic { return &Cyclomatic{} }

// ID returns the metric's stable identifier.
func (Cyclomatic) ID() string { return "cyclomatic" }

// Name returns the metric's human-readable name.
func (Cyclomatic) Name() string { return "Cyclomatic Complexity" }

// Description returns a one-line description of what the metric measures.
func (Cyclomatic) Description() string { return "McCabe decision-point count." }

// DefaultThreshold of 7 matches the project-wide gocyclo configuration in
// rules/go-conventions.md (functions must be ≤ 7).
func (Cyclomatic) DefaultThreshold() float64 { return 7 }

// HigherIsWorse reports that larger complexity values indicate worse code.
func (Cyclomatic) HigherIsWorse() bool { return true }

// Analyze counts decision points in fn.FuncDecl.Body and returns the score.
// When the result exceeds opts.Threshold, a single Finding is emitted at the
// function's start line; severity escalates to Error at ≥ 2× threshold.
func (m Cyclomatic) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	v := computeComplexity(fn.FuncDecl)
	score := domain.Score{
		MetricID: m.ID(),
		Function: fn,
		Value:    float64(v),
	}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if float64(v) > threshold {
		sev := domain.SeverityWarning
		if float64(v) >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("cyclomatic complexity %d exceeds threshold %g", v, threshold),
		}}
	}
	return score, nil
}

func computeComplexity(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 1
	}
	complexity := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if nodeAddsPath(n) {
			complexity++
		}
		return true
	})
	return complexity
}

func nodeAddsPath(n ast.Node) bool {
	switch x := n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
		return true
	case *ast.CaseClause:
		// CaseClause is used for both switch and type switch; the default
		// clause has nil List and does not add a path.
		return len(x.List) > 0
	case *ast.CommClause:
		// Comm clause is a case in a select statement; default has nil Comm.
		return x.Comm != nil
	case *ast.BinaryExpr:
		return x.Op == token.LAND || x.Op == token.LOR
	}
	return false
}
