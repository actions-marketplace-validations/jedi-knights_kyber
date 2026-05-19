package metrics

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Effort implements the Halstead Effort measure: an estimate of the total
// mental effort to write or comprehend a program. Formula:
//
//	E = D * V
//
// where D is Halstead Difficulty and V is Halstead Volume. Higher is worse.
//
// Reference: Halstead, M. H. (1977). Elements of Software Science.
type Effort struct{}

// NewEffort constructs the metric.
func NewEffort() *Effort { return &Effort{} }

// ID returns the metric's stable identifier.
func (Effort) ID() string { return "effort" }

// Name returns the metric's human-readable name.
func (Effort) Name() string { return "Halstead Effort" }

// Description returns a one-line description of what the metric measures.
func (Effort) Description() string {
	return "Halstead Effort — E = D * V (Difficulty times Volume)."
}

// DefaultThreshold of 10000 is a commonly cited per-function yellow flag.
func (Effort) DefaultThreshold() float64 { return 10000 }

// HigherIsWorse reports that larger effort values indicate worse code.
func (Effort) HigherIsWorse() bool { return true }

// Analyze computes Halstead Effort for fn. Emits a Warning when the value
// exceeds opts.Threshold; severity escalates to Error at ≥ 2× threshold.
func (m Effort) Analyze(ctx context.Context, fn *domain.Function, opts domain.MetricOptions) (domain.Score, error) {
	if err := ctx.Err(); err != nil {
		return domain.Score{}, err
	}
	src := []byte(strings.Join(fn.SourceLines, "\n"))
	counts := tallyHalsteadTokens(src)
	d := halsteadDifficulty(counts)
	v := halsteadVolume(counts)
	e := d * v
	score := domain.Score{MetricID: m.ID(), Function: fn, Value: e}

	threshold := opts.Threshold
	if threshold == 0 {
		threshold = m.DefaultThreshold()
	}
	if e > threshold {
		sev := domain.SeverityWarning
		if e >= 2*threshold {
			sev = domain.SeverityError
		}
		score.Findings = []domain.Finding{{
			Severity: sev,
			Line:     fn.Position().Line,
			Message:  fmt.Sprintf("halstead effort %.0f exceeds threshold %g", e, threshold),
		}}
	}
	return score, nil
}

// halsteadVolume computes V = N * log2(n) from token counts; returns 0
// when no tokens were scanned.
func halsteadVolume(c halsteadCounts) float64 {
	n := c.uniqueOps + c.uniqueOperands
	N := c.totalOps + c.totalOperands
	if n == 0 || N == 0 {
		return 0
	}
	return float64(N) * math.Log2(float64(n))
}
