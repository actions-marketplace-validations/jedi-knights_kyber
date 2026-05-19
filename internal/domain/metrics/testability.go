package metrics

import (
	"context"
	"fmt"
	"go/ast"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Testability is a weighted 0–1 score combining four sub-signals: parameter
// count, observed side effects (calls into stdlib I/O packages and reads of
// package-level globals), the fraction of parameters whose types are
// interfaces, and function length. Higher is better; below threshold flags.
type Testability struct{}

// NewTestability constructs the metric.
func NewTestability() *Testability { return &Testability{} }

// ID returns the metric's stable identifier.
func (Testability) ID() string { return "testability" }

// Name returns the metric's human-readable name.
func (Testability) Name() string { return "Testability Score" }

// Description returns a one-line description of what the metric measures.
func (Testability) Description() string {
	return "Weighted 0–1 score from parameter count, side effects, interface params, and length."
}

// DefaultThreshold reports the score below which a function is flagged.
func (Testability) DefaultThreshold() float64 { return 0.6 }

// HigherIsWorse reports that smaller testability scores indicate worse code.
func (Testability) HigherIsWorse() bool { return false }

const (
	testDefaultMaxParams = 5
	testDefaultMaxLines  = 40
	testSideEffectMax    = 3
)

// sideEffectPackages are stdlib package prefixes whose calls indicate
// observable side-effects: I/O, time-of-day, process control, networking,
// logging. A function that calls any of these is harder to test in isolation.
var sideEffectPackages = map[string]bool{
	"os":   true,
	"log":  true,
	"http": true,
	"net":  true,
	"time": true,
	"fmt":  true,
}

// Analyze computes the weighted testability score for fn and emits a Warning
// finding when the score falls below the configured threshold.
func (m Testability) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	maxParams := opts.IntParam("max_params", testDefaultMaxParams)
	maxLines := opts.IntParam("max_lines", testDefaultMaxLines)
	wParams := opts.FloatParam("weight_params", 1)
	wSE := opts.FloatParam("weight_side_effects", 1)
	wIface := opts.FloatParam("weight_interfaces", 1)
	wLen := opts.FloatParam("weight_length", 1)

	paramSignal := signalRatio(fn.ParamCount(), maxParams)
	seSignal := computeSideEffectSignal(fn)
	ifaceSignal := computeInterfaceSignal(fn)
	lenSignal := signalRatio(fn.LineCount(), maxLines)

	total := wParams + wSE + wIface + wLen
	if total == 0 {
		total = 1
	}
	score := (wParams*paramSignal + wSE*seSignal + wIface*ifaceSignal + wLen*lenSignal) / total

	out := domain.Score{MetricID: m.ID(), Function: fn, Value: score}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if score < threshold {
		out.Findings = []domain.Finding{{
			Severity: domain.SeverityWarning,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("testability %.2f below threshold %.2f", score, threshold),
		}}
	}
	return out, nil
}

// computeSideEffectSignal scans the function body for calls into known
// side-effect packages and reads/writes of package-level globals. Score
// approaches 0 as the count rises toward testSideEffectMax.
func computeSideEffectSignal(fn *domain.Function) float64 {
	if fn.FuncDecl == nil || fn.FuncDecl.Body == nil {
		return 1
	}
	globals := collectGlobalNames(fn.Package)
	count := 0
	ast.Inspect(fn.FuncDecl.Body, func(n ast.Node) bool {
		if nodeIsSideEffect(n, globals) {
			count++
		}
		return true
	})
	return signalRatio(count, testSideEffectMax)
}

func collectGlobalNames(pkg *domain.Package) map[string]bool {
	globals := map[string]bool{}
	if pkg == nil {
		return globals
	}
	for name := range pkg.Globals {
		globals[name] = true
	}
	return globals
}

func nodeIsSideEffect(n ast.Node, globals map[string]bool) bool {
	switch x := n.(type) {
	case *ast.CallExpr:
		return isSideEffectCall(x)
	case *ast.Ident:
		return globals[x.Name]
	}
	return false
}

func isSideEffectCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return sideEffectPackages[pkg.Name]
}

// computeInterfaceSignal returns the fraction of parameters whose declared
// type is an interface — either an inline `interface{...}` or a named type
// that resolves to an interface declared in the same package.
func computeInterfaceSignal(fn *domain.Function) float64 {
	if fn.FuncDecl == nil || fn.FuncDecl.Type.Params == nil {
		return 1
	}
	total := fn.ParamCount()
	if total == 0 {
		return 1
	}
	ifaceParams := 0
	for _, field := range fn.FuncDecl.Type.Params.List {
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		if paramIsInterface(field.Type, fn.Package) {
			ifaceParams += n
		}
	}
	return float64(ifaceParams) / float64(total)
}

func paramIsInterface(expr ast.Expr, pkg *domain.Package) bool {
	switch t := expr.(type) {
	case *ast.InterfaceType:
		return true
	case *ast.Ident:
		if pkg == nil {
			return false
		}
		_, ok := pkg.Interfaces[t.Name]
		return ok
	case *ast.StarExpr:
		// A pointer to a named interface is rare but possible; check the
		// base name.
		if id, ok := t.X.(*ast.Ident); ok && pkg != nil {
			_, ok := pkg.Interfaces[id.Name]
			return ok
		}
	}
	return false
}
