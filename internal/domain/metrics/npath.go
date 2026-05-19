package metrics

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/jedi-knights/kyber/internal/domain"
)

// NPath implements Nejmeh's NPath complexity (Nejmeh, 1988). Where cyclomatic
// counts decision points additively, NPath counts the number of acyclic
// execution paths through a function — which is multiplicative across
// sequential statements. A function with seven sequential if-else blocks
// has cyclomatic 8 but NPath 128, exposing the combinatorial explosion that
// cyclomatic misses.
//
// Reference: Nejmeh, B. A. (1988). NPATH: A measure of execution path
// complexity and its applications. Communications of the ACM, 31(2).
type NPath struct{}

// NewNPath constructs the metric.
func NewNPath() *NPath { return &NPath{} }

// ID returns the metric's stable identifier.
func (NPath) ID() string { return "npath" }

// Name returns the metric's human-readable name.
func (NPath) Name() string { return "NPath Complexity" }

// Description returns a one-line description of what the metric measures.
func (NPath) Description() string {
	return "Nejmeh NPath — acyclic execution paths through a function (multiplicative)."
}

// DefaultThreshold of 200 is the commonly cited upper bound for individual
// functions; values above multiply rapidly with each added branch.
func (NPath) DefaultThreshold() float64 { return 200 }

// HigherIsWorse reports that larger NPath values indicate worse code.
func (NPath) HigherIsWorse() bool { return true }

// Analyze walks fn.FuncDecl.Body applying Nejmeh's rules and emits a
// Warning finding when the result exceeds opts.Threshold; severity
// escalates to Error at ≥ 2× threshold.
func (m NPath) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	v := computeNPath(fn.FuncDecl)
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
			Message:  fmt.Sprintf("npath complexity %d exceeds threshold %g", v, threshold),
		}}
	}
	return score, nil
}

func computeNPath(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 1
	}
	return npathBlock(fn.Body)
}

func npathBlock(b *ast.BlockStmt) int {
	if b == nil || len(b.List) == 0 {
		return 1
	}
	product := 1
	for _, stmt := range b.List {
		product *= npathStmt(stmt)
	}
	return product
}

func npathStmt(stmt ast.Stmt) int {
	if v, ok := npathControl(stmt); ok {
		return v
	}
	if v, ok := npathContainer(stmt); ok {
		return v
	}
	return 1 + countLogicalOps(stmt)
}

func npathControl(stmt ast.Stmt) (int, bool) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		return npathIf(s), true
	case *ast.ForStmt:
		return npathBlock(s.Body) + npathExpr(s.Cond) + 1, true
	case *ast.RangeStmt:
		return npathBlock(s.Body) + 1, true
	case *ast.SwitchStmt:
		return npathSwitch(s.Body) + npathExpr(s.Tag), true
	case *ast.TypeSwitchStmt:
		return npathSwitch(s.Body), true
	case *ast.SelectStmt:
		return npathSelect(s.Body), true
	}
	return 0, false
}

func npathContainer(stmt ast.Stmt) (int, bool) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		return npathBlock(s), true
	case *ast.LabeledStmt:
		return npathStmt(s.Stmt), true
	}
	return 0, false
}

func npathIf(s *ast.IfStmt) int {
	then := npathBlock(s.Body)
	elsePart := 1
	if s.Else != nil {
		switch e := s.Else.(type) {
		case *ast.IfStmt:
			elsePart = npathIf(e)
		case *ast.BlockStmt:
			elsePart = npathBlock(e)
		}
	}
	return then + elsePart + npathExpr(s.Cond)
}

func npathSwitch(body *ast.BlockStmt) int {
	if body == nil {
		return 1
	}
	sum, hasDefault := 0, false
	for _, clause := range body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(cc.List) == 0 {
			hasDefault = true
		}
		sum += npathStmts(cc.Body)
	}
	if !hasDefault {
		sum++
	}
	if sum == 0 {
		return 1
	}
	return sum
}

func npathSelect(body *ast.BlockStmt) int {
	if body == nil {
		return 1
	}
	sum, hasDefault := 0, false
	for _, clause := range body.List {
		cc, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}
		if cc.Comm == nil {
			hasDefault = true
		}
		sum += npathStmts(cc.Body)
	}
	if !hasDefault {
		sum++
	}
	if sum == 0 {
		return 1
	}
	return sum
}

func npathStmts(stmts []ast.Stmt) int {
	product := 1
	for _, s := range stmts {
		product *= npathStmt(s)
	}
	return product
}

// npathExpr counts logical operators (&& and ||) anywhere in e. Each adds
// one path to the parent control structure's NPath value.
func npathExpr(e ast.Expr) int {
	if e == nil {
		return 0
	}
	count := 0
	ast.Inspect(e, func(n ast.Node) bool {
		if be, ok := n.(*ast.BinaryExpr); ok && isLogicalOp(be.Op) {
			count++
		}
		return true
	})
	return count
}

// countLogicalOps counts logical operators in a non-control statement,
// stopping at any nested control structure or function literal (those are
// counted by their own npathStmt invocation).
func countLogicalOps(stmt ast.Stmt) int {
	count := 0
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt,
			*ast.FuncLit:
			return false
		}
		if be, ok := n.(*ast.BinaryExpr); ok {
			if be.Op == token.LAND || be.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}
