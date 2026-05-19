package domain

// Severity classifies how serious a finding is. Reporters map this to their
// own severity vocabulary (e.g. SARIF's "note"/"warning"/"error").
type Severity int

// Severity levels in ascending order of seriousness.
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

// String returns the lowercase name of the severity. Stable; safe to use in
// machine-readable output.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Score is a single metric's result for a single function. Value is the raw
// numeric score (interpretation is metric-specific); Findings are localized
// issues that reporters render at specific source positions.
type Score struct {
	MetricID string
	Function *Function
	Value    float64
	Findings []Finding
}

// Finding is one localized issue inside a function. Line and Column are
// 1-based; Column may be 0 when not known.
type Finding struct {
	Severity Severity
	Line     int
	Column   int
	Message  string
}

// ExceedsThreshold reports whether the score's value crosses the supplied
// threshold in the unfavorable direction. Set higherIsWorse=true for metrics
// like cyclomatic complexity (more is bad); set false for normalized 0–1
// scores like readability and testability where lower is bad.
//
// Equal-to-threshold values are NOT considered breaches.
func (s Score) ExceedsThreshold(threshold float64, higherIsWorse bool) bool {
	if higherIsWorse {
		return s.Value > threshold
	}
	return s.Value < threshold
}
