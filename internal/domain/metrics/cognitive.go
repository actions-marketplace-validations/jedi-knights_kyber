package metrics

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Cognitive implements SonarSource Cognitive Complexity (Campbell, 2018).
// Where cyclomatic complexity counts independent paths, cognitive complexity
// estimates how hard the code is for a human to follow: every control
// structure adds 1, plus an extra point for every level of nesting it sits
// inside; sequences of like boolean operators count once per operator change.
//
// Differences from McCabe cyclomatic:
//   - `else` and `else if` add to the score; in McCabe they don't (they don't
//     introduce a new independent path).
//   - Deeply nested control flow is penalized; shallow control flow is not.
//   - `switch` adds 1 total, not 1 per case clause.
//
// Reference: https://www.sonarsource.com/docs/CognitiveComplexity.pdf
type Cognitive struct{}

// NewCognitive constructs the metric.
func NewCognitive() *Cognitive { return &Cognitive{} }

// ID returns the metric's stable identifier.
func (Cognitive) ID() string { return "cognitive" }

// Name returns the metric's human-readable name.
func (Cognitive) Name() string { return "Cognitive Complexity" }

// Description returns a one-line description of what the metric measures.
func (Cognitive) Description() string {
	return "SonarSource Cognitive Complexity — control flow + nesting penalty."
}

// DefaultThreshold of 15 matches SonarQube's default per-function warning.
func (Cognitive) DefaultThreshold() float64 { return 15 }

// HigherIsWorse reports that larger cognitive scores indicate worse code.
func (Cognitive) HigherIsWorse() bool { return true }

// Analyze walks fn.FuncDecl.Body, applying SonarSource increment rules, and
// emits a Warning finding when the result exceeds opts.Threshold; severity
// escalates to Error at ≥ 2× threshold.
func (m Cognitive) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	v := computeCognitive(fn.FuncDecl)
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
			Message:  fmt.Sprintf("cognitive complexity %d exceeds threshold %g", v, threshold),
		}}
	}
	return score, nil
}

func computeCognitive(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 0
	}
	c := &cognitiveWalker{}
	c.walkBlock(fn.Body, 0)
	return c.score
}

type cognitiveWalker struct{ score int }

func (c *cognitiveWalker) walkBlock(b *ast.BlockStmt, nesting int) {
	for _, stmt := range b.List {
		c.walkStmt(stmt, nesting)
	}
}

func (c *cognitiveWalker) walkStmt(stmt ast.Stmt, nesting int) {
	switch {
	case c.walkLoopStmt(stmt, nesting):
	case c.walkSwitchFamily(stmt, nesting):
	case c.walkIfOrBranch(stmt, nesting):
	case c.walkExprContainer(stmt, nesting):
	default:
		c.walkBlockOrLabeled(stmt, nesting)
	}
}

func (c *cognitiveWalker) walkLoopStmt(stmt ast.Stmt, nesting int) bool {
	switch s := stmt.(type) {
	case *ast.ForStmt:
		c.score += 1 + nesting
		c.walkExpr(s.Cond, nesting)
		c.walkBlock(s.Body, nesting+1)
	case *ast.RangeStmt:
		c.score += 1 + nesting
		c.walkBlock(s.Body, nesting+1)
	default:
		return false
	}
	return true
}

func (c *cognitiveWalker) walkSwitchFamily(stmt ast.Stmt, nesting int) bool {
	switch s := stmt.(type) {
	case *ast.SwitchStmt:
		c.score += 1 + nesting
		c.walkExpr(s.Tag, nesting)
		c.walkCases(s.Body, nesting+1)
	case *ast.TypeSwitchStmt:
		c.score += 1 + nesting
		c.walkCases(s.Body, nesting+1)
	case *ast.SelectStmt:
		c.score += 1 + nesting
		c.walkCommClauses(s.Body, nesting+1)
	default:
		return false
	}
	return true
}

func (c *cognitiveWalker) walkIfOrBranch(stmt ast.Stmt, nesting int) bool {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		c.walkIf(s, nesting)
	case *ast.BranchStmt:
		if s.Label != nil || s.Tok == token.GOTO {
			c.score++
		}
	default:
		return false
	}
	return true
}

// walkExprContainer handles statements that hold expressions worth descending
// into for nested FuncLits or boolean-operator sequences.
func (c *cognitiveWalker) walkExprContainer(stmt ast.Stmt, nesting int) bool {
	switch s := stmt.(type) {
	case *ast.DeferStmt:
		c.walkExpr(s.Call, nesting)
	case *ast.GoStmt:
		c.walkExpr(s.Call, nesting)
	case *ast.ExprStmt:
		c.walkExpr(s.X, nesting)
	case *ast.AssignStmt:
		c.walkExprs(s.Rhs, nesting)
	case *ast.ReturnStmt:
		c.walkExprs(s.Results, nesting)
	default:
		return false
	}
	return true
}

func (c *cognitiveWalker) walkBlockOrLabeled(stmt ast.Stmt, nesting int) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		c.walkBlock(s, nesting)
	case *ast.LabeledStmt:
		c.walkStmt(s.Stmt, nesting)
	}
}

func (c *cognitiveWalker) walkExprs(exprs []ast.Expr, nesting int) {
	for _, e := range exprs {
		c.walkExpr(e, nesting)
	}
}

// walkIf handles `if / else if / else` chains. Per SonarSource, every branch
// adds 1, but only the first `if` carries the nesting penalty — `else if` and
// `else` continue the chain, so they don't multiply the cost.
func (c *cognitiveWalker) walkIf(s *ast.IfStmt, nesting int) {
	c.score += 1 + nesting
	c.walkExpr(s.Cond, nesting)
	c.walkBlock(s.Body, nesting+1)
	switch e := s.Else.(type) {
	case *ast.IfStmt:
		c.score++
		c.walkExpr(e.Cond, nesting)
		c.walkBlock(e.Body, nesting+1)
		// Recurse into the chain's tail.
		if e.Else != nil {
			c.walkElseTail(e.Else, nesting)
		}
	case *ast.BlockStmt:
		c.score++
		c.walkBlock(e, nesting+1)
	}
}

func (c *cognitiveWalker) walkElseTail(s ast.Stmt, nesting int) {
	switch e := s.(type) {
	case *ast.IfStmt:
		c.score++
		c.walkExpr(e.Cond, nesting)
		c.walkBlock(e.Body, nesting+1)
		if e.Else != nil {
			c.walkElseTail(e.Else, nesting)
		}
	case *ast.BlockStmt:
		c.score++
		c.walkBlock(e, nesting+1)
	}
}

func (c *cognitiveWalker) walkCases(body *ast.BlockStmt, nesting int) {
	if body == nil {
		return
	}
	for _, clause := range body.List {
		cc, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, st := range cc.Body {
			c.walkStmt(st, nesting)
		}
	}
}

func (c *cognitiveWalker) walkCommClauses(body *ast.BlockStmt, nesting int) {
	if body == nil {
		return
	}
	for _, clause := range body.List {
		cc, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}
		for _, st := range cc.Body {
			c.walkStmt(st, nesting)
		}
	}
}

// walkExpr looks inside an expression for two things: nested function
// literals (which increase nesting for their bodies), and sequences of like
// boolean operators (each sequence adds 1, with one more per operator change
// within the sequence).
func (c *cognitiveWalker) walkExpr(e ast.Expr, nesting int) {
	if e == nil {
		return
	}
	c.walkFuncLits(e, nesting)
	c.countLogicalSequences(e, false)
}

func (c *cognitiveWalker) walkFuncLits(e ast.Expr, nesting int) {
	ast.Inspect(e, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			c.walkBlock(fl.Body, nesting+1)
			return false
		}
		return true
	})
}

// countLogicalSequences walks the expression tree. When it meets a
// logical-op BinaryExpr that is not itself a child of another logical-op
// BinaryExpr (i.e., the root of a sequence), it adds 1 plus the number of
// operator transitions within the sequence. Descent into nested operand
// expressions (function calls, parens, etc.) continues at the outer level so
// independent sequences inside the same condition are also counted.
func (c *cognitiveWalker) countLogicalSequences(e ast.Expr, inSequence bool) {
	if be, ok := e.(*ast.BinaryExpr); ok && isLogicalOp(be.Op) {
		if !inSequence {
			c.score += 1 + countOperatorChanges(be)
		}
		c.countLogicalSequences(be.X, true)
		c.countLogicalSequences(be.Y, true)
		return
	}
	c.descendLogicalSequences(e)
}

func (c *cognitiveWalker) descendLogicalSequences(e ast.Expr) {
	switch x := e.(type) {
	case *ast.BinaryExpr:
		c.countLogicalSequences(x.X, false)
		c.countLogicalSequences(x.Y, false)
	case *ast.UnaryExpr:
		c.countLogicalSequences(x.X, false)
	case *ast.ParenExpr:
		c.countLogicalSequences(x.X, false)
	case *ast.CallExpr:
		c.countLogicalSequences(x.Fun, false)
		for _, arg := range x.Args {
			c.countLogicalSequences(arg, false)
		}
	}
}

func isLogicalOp(op token.Token) bool {
	return op == token.LAND || op == token.LOR
}

// countOperatorChanges walks a tree of LAND/LOR operators and returns the
// number of transitions between the two. `a && b && c` yields 0; `a && b ||
// c` yields 1; `a && b || c && d` yields 2.
func countOperatorChanges(root *ast.BinaryExpr) int {
	ops := flattenLogicalOps(root)
	changes := 0
	for i := 1; i < len(ops); i++ {
		if ops[i] != ops[i-1] {
			changes++
		}
	}
	return changes
}

func flattenLogicalOps(e ast.Expr) []token.Token {
	be, ok := e.(*ast.BinaryExpr)
	if !ok || !isLogicalOp(be.Op) {
		return nil
	}
	out := flattenLogicalOps(be.X)
	out = append(out, be.Op)
	out = append(out, flattenLogicalOps(be.Y)...)
	return out
}
