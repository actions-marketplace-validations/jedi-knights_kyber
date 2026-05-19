package metrics

import (
	"context"
	"fmt"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Halstead implements Halstead's software science metrics (Halstead, 1977).
// Tokens are classified as operators (keywords, punctuation, binary/unary
// operators) or operands (identifiers, literals). The reported Value is
// Volume:
//
//	V = N * log2(n)
//
// where N = N1 + N2 (total operators and operands) and n = n1 + n2 (unique
// counts). Volume is the most actionable single-number Halstead measure for
// a function — Difficulty and Effort are derivable from the same counts but
// not currently surfaced.
//
// Reference: Halstead, M. H. (1977). Elements of Software Science.
type Halstead struct{}

// NewHalstead constructs the metric.
func NewHalstead() *Halstead { return &Halstead{} }

// ID returns the metric's stable identifier.
func (Halstead) ID() string { return "halstead" }

// Name returns the metric's human-readable name.
func (Halstead) Name() string { return "Halstead Volume" }

// Description returns a one-line description of what the metric measures.
func (Halstead) Description() string {
	return "Halstead Volume — token counts weighted by vocabulary size."
}

// DefaultThreshold of 1000 is a commonly cited yellow-flag boundary for
// individual functions. Library-wide Volume sums to far higher values;
// per-function the figure is more actionable.
func (Halstead) DefaultThreshold() float64 { return 1000 }

// HigherIsWorse reports that larger volumes indicate denser, harder-to-read code.
func (Halstead) HigherIsWorse() bool { return true }

// Analyze tokenizes fn.SourceLines, classifies each token as an operator or
// operand, and computes Volume. A finding is emitted when Volume exceeds
// opts.Threshold; severity escalates to Error at ≥ 2× threshold.
func (m Halstead) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	volume := computeHalsteadVolume(fn.SourceLines)
	score := domain.Score{
		MetricID: m.ID(),
		Function: fn,
		Value:    volume,
	}
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if volume > threshold {
		sev := domain.SeverityWarning
		if volume >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("halstead volume %.0f exceeds threshold %g", volume, threshold),
		}}
	}
	return score, nil
}

func computeHalsteadVolume(sourceLines []string) float64 {
	src := []byte(strings.Join(sourceLines, "\n"))
	if len(src) == 0 {
		return 0
	}
	return halsteadVolume(tallyHalsteadTokens(src))
}

type halsteadCounts struct {
	uniqueOps, uniqueOperands int
	totalOps, totalOperands   int
}

func tallyHalsteadTokens(src []byte) halsteadCounts {
	uniqueOps := map[string]struct{}{}
	uniqueOperands := map[string]struct{}{}
	var totalOps, totalOperands int

	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, src, nil, 0)

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if skipToken(tok) {
			continue
		}
		if isOperand(tok) {
			key := operandKey(tok, lit)
			uniqueOperands[key] = struct{}{}
			totalOperands++
			continue
		}
		uniqueOps[tok.String()] = struct{}{}
		totalOps++
	}
	return halsteadCounts{
		uniqueOps:      len(uniqueOps),
		uniqueOperands: len(uniqueOperands),
		totalOps:       totalOps,
		totalOperands:  totalOperands,
	}
}

func skipToken(tok token.Token) bool {
	// Scanner emits ILLEGAL on parse trouble, COMMENT for // and /* */, and
	// auto-inserts SEMICOLON at line breaks; none reflect real source operators.
	return tok == token.ILLEGAL || tok == token.COMMENT || tok == token.SEMICOLON
}

func operandKey(tok token.Token, lit string) string {
	if lit != "" {
		return lit
	}
	return tok.String()
}

func isOperand(tok token.Token) bool {
	switch tok {
	case token.IDENT, token.INT, token.FLOAT, token.IMAG, token.CHAR, token.STRING:
		return true
	}
	return false
}
