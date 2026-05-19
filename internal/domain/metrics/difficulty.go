package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Difficulty implements the Halstead Difficulty measure: an estimate of how
// hard a program is to write, given its vocabulary. Formula:
//
//	D = (n1 / 2) * (N2 / n2)
//
// where n1, n2 are unique operator and operand counts and N2 is total
// operands. Higher is worse.
//
// Reference: Halstead, M. H. (1977). Elements of Software Science.
type Difficulty struct{}

// NewDifficulty constructs the metric.
func NewDifficulty() *Difficulty { return &Difficulty{} }

// ID returns the metric's stable identifier.
func (Difficulty) ID() string { return "difficulty" }

// Name returns the metric's human-readable name.
func (Difficulty) Name() string { return "Halstead Difficulty" }

// Description returns a one-line description of what the metric measures.
func (Difficulty) Description() string {
	return "Halstead Difficulty — D = (n1/2) * (N2/n2)."
}

// DefaultThreshold of 15 matches the commonly cited per-function yellow flag.
func (Difficulty) DefaultThreshold() float64 { return 15 }

// HigherIsWorse reports that larger difficulty values indicate worse code.
func (Difficulty) HigherIsWorse() bool { return true }

// Analyze computes Halstead Difficulty for fn. Emits a Warning when the
// value exceeds opts.Threshold; severity escalates to Error at ≥ 2× threshold.
func (m Difficulty) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	src := []byte(strings.Join(fn.SourceLines, "\n"))
	d := halsteadDifficulty(tallyHalsteadTokens(src))
	score := domain.Score{MetricID: m.ID(), Function: fn, Value: d}

	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if d > threshold {
		sev := domain.SeverityWarning
		if d >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("halstead difficulty %.1f exceeds threshold %g", d, threshold),
		}}
	}
	return score, nil
}

// halsteadDifficulty computes D = (n1/2) * (N2/n2) from token counts.
// Returns 0 when n2 is zero (no operands) to avoid division by zero.
func halsteadDifficulty(c halsteadCounts) float64 {
	if c.uniqueOperands == 0 {
		return 0
	}
	return (float64(c.uniqueOps) / 2) * (float64(c.totalOperands) / float64(c.uniqueOperands))
}
