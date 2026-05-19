package metrics

import (
	"context"
	"fmt"
	"go/ast"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Readability is a weighted 0–1 score combining four sub-signals: function
// length, maximum nesting depth, median identifier length, and comment
// density. Higher is better; below threshold flags as unfavorable.
type Readability struct{}

// NewReadability constructs the metric.
func NewReadability() *Readability { return &Readability{} }

// ID returns the metric's stable identifier.
func (Readability) ID() string { return "readability" }

// Name returns the metric's human-readable name.
func (Readability) Name() string { return "Readability Score" }

// Description returns a one-line description of what the metric measures.
func (Readability) Description() string {
	return "Weighted 0–1 score from length, nesting depth, identifier length, and comment density."
}

// DefaultThreshold reports the score below which a function is flagged.
func (Readability) DefaultThreshold() float64 { return 0.6 }

// HigherIsWorse reports that smaller readability scores indicate worse code.
func (Readability) HigherIsWorse() bool { return false }

const (
	readDefaultMaxLines   = 40
	readDefaultMaxNesting = 4
	readDefaultMedianGood = 5
)

// triviaIdents are short names whose use is idiomatic Go (loop indices,
// errors, the blank identifier) and that should not penalize identifier
// length. They are filtered out of the median calculation.
var triviaIdents = map[string]bool{
	"i": true, "j": true, "k": true,
	"_": true, "ok": true, "err": true,
}

// Analyze computes the weighted readability score for fn and emits a Warning
// finding when the score falls below the configured threshold.
func (m Readability) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	maxLines := opts.IntParam("max_lines", readDefaultMaxLines)
	maxNesting := opts.IntParam("max_nesting", readDefaultMaxNesting)
	wLen := opts.FloatParam("weight_length", 1)
	wNest := opts.FloatParam("weight_nesting", 1)
	wIdent := opts.FloatParam("weight_idents", 1)
	wComment := opts.FloatParam("weight_comments", 1)

	lengthSignal := signalRatio(fn.LineCount(), maxLines)
	nestSignal := signalRatio(computeNestingDepth(fn.FuncDecl), maxNesting)
	identSignal := computeIdentSignal(fn.FuncDecl, fn.Receiver, readDefaultMedianGood)
	commentSignal := computeCommentSignal(fn)

	totalWeight := wLen + wNest + wIdent + wComment
	if totalWeight == 0 {
		totalWeight = 1
	}
	score := (wLen*lengthSignal + wNest*nestSignal + wIdent*identSignal + wComment*commentSignal) / totalWeight

	out := domain.Score{MetricID: m.ID(), Function: fn, Value: score}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if score < threshold {
		out.Findings = []domain.Finding{{
			Severity: domain.SeverityWarning,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("readability %.2f below threshold %.2f", score, threshold),
		}}
	}
	return out, nil
}

// signalRatio returns 1 when value ≤ 0, decreasing linearly to 0 as value
// reaches max. Used to convert "smaller is better" counts into 0–1 scores.
func signalRatio(value, max int) float64 {
	if max <= 0 {
		return 1
	}
	r := float64(value) / float64(max)
	if r > 1 {
		r = 1
	}
	return 1 - r
}

// computeNestingDepth returns the deepest block nesting inside fn.Body.
// Each *ast.BlockStmt inside the function body adds one level.
func computeNestingDepth(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 0
	}
	maxDepth := 0
	var walk func(n ast.Node, depth int)
	walk = func(n ast.Node, depth int) {
		if n == nil {
			return
		}
		if _, ok := n.(*ast.BlockStmt); ok {
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		}
		ast.Inspect(n, func(child ast.Node) bool {
			if child == n {
				return true
			}
			if _, ok := child.(*ast.BlockStmt); ok {
				walk(child, depth)
				return false
			}
			return true
		})
	}
	walk(fn.Body, 0)
	return maxDepth
}

// computeIdentSignal returns 1 when the median length of non-trivia identifiers
// is ≥ goodMedian, decreasing linearly to 0 at a median of 1.
func computeIdentSignal(fn *ast.FuncDecl, receiver string, goodMedian int) float64 {
	if fn == nil || fn.Body == nil {
		return 1
	}
	lengths := collectIdentLengths(fn.Body, receiver)
	if len(lengths) == 0 {
		return 1
	}
	med := median(lengths)
	if med >= goodMedian {
		return 1
	}
	if med <= 1 {
		return 0
	}
	return float64(med-1) / float64(goodMedian-1)
}

func collectIdentLengths(body *ast.BlockStmt, receiver string) []int {
	var lengths []int
	ast.Inspect(body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		name := id.Name
		if triviaIdents[name] || name == receiver {
			return true
		}
		lengths = append(lengths, len(name))
		return true
	})
	return lengths
}

// computeCommentSignal returns comment-line density / 0.2, capped at 1. A
// function with 20% comment lines or more scores 1.0; with no comments, 0.
func computeCommentSignal(fn *domain.Function) float64 {
	if len(fn.SourceLines) == 0 {
		return 0
	}
	const goodDensity = 0.2
	commentLines := 0
	for _, raw := range fn.SourceLines {
		trimmed := trimLeftSpace(raw)
		if startsWithComment(trimmed) {
			commentLines++
		}
	}
	density := float64(commentLines) / float64(len(fn.SourceLines))
	if density >= goodDensity {
		return 1
	}
	return density / goodDensity
}

func trimLeftSpace(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != ' ' && c != '\t' {
			return s[i:]
		}
	}
	return ""
}

func startsWithComment(s string) bool {
	return len(s) >= 2 && s[0] == '/' && (s[1] == '/' || s[1] == '*')
}

func median(xs []int) int {
	cp := make([]int, len(xs))
	copy(cp, xs)
	// Simple insertion sort — these slices are short.
	for i := 1; i < len(cp); i++ {
		for j := i; j > 0 && cp[j-1] > cp[j]; j-- {
			cp[j-1], cp[j] = cp[j], cp[j-1]
		}
	}
	return cp[len(cp)/2]
}
