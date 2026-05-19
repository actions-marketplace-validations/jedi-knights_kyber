package domain

import (
	"sort"
	"time"
)

// Report is the result of an analysis run — every Score from every metric
// against every function, plus run-time bookkeeping. Reporters read this
// structure to produce text, JSON, or SARIF output.
type Report struct {
	Scores       []Score
	StartTime    time.Time
	EndTime      time.Time
	FilesScanned int
	Errors       []error
}

// ByFunction groups scores by their function for per-function summaries.
// The iteration order of the result is not specified; callers that need a
// stable order should sort by Function.Position().
func (r *Report) ByFunction() map[*Function][]Score {
	out := make(map[*Function][]Score)
	for _, s := range r.Scores {
		out[s.Function] = append(out[s.Function], s)
	}
	return out
}

// ByMetric returns scores for one metric ID, in original order.
func (r *Report) ByMetric(id string) []Score {
	out := make([]Score, 0)
	for _, s := range r.Scores {
		if s.MetricID == id {
			out = append(out, s)
		}
	}
	return out
}

// ExceedingThresholds returns the subset of scores whose Findings list is
// non-empty (the metric already evaluated the breach).
func (r *Report) ExceedingThresholds() []Score {
	out := make([]Score, 0)
	for _, s := range r.Scores {
		if len(s.Findings) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// MetricStats is a per-metric aggregate over some set of functions. Count is
// the number of functions contributing; Mean/Min/Max are computed over their
// Score values.
type MetricStats struct {
	MetricID string
	Count    int
	Mean     float64
	Min      float64
	Max      float64
}

// PackageStats is the aggregate view of one package — function count plus
// one MetricStats per metric ID, sorted by ID.
type PackageStats struct {
	Package       *Package
	FunctionCount int
	Metrics       []MetricStats
}

// PackageStats returns one PackageStats per Package present in the report,
// sorted by Package.ImportPath. Scores with a nil Function or nil Package
// are skipped.
func (r *Report) PackageStats() []PackageStats {
	byPkg := map[*Package][]Score{}
	for _, s := range r.Scores {
		if s.Function == nil || s.Function.Package == nil {
			continue
		}
		byPkg[s.Function.Package] = append(byPkg[s.Function.Package], s)
	}
	out := make([]PackageStats, 0, len(byPkg))
	for pkg, scores := range byPkg {
		out = append(out, PackageStats{
			Package:       pkg,
			FunctionCount: countUniqueFunctions(scores),
			Metrics:       aggregateByMetric(scores),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Package.ImportPath < out[j].Package.ImportPath
	})
	return out
}

// OverallStats returns the unique function count across the whole report and
// one MetricStats per metric ID, sorted by ID.
func (r *Report) OverallStats() (int, []MetricStats) {
	return countUniqueFunctions(r.Scores), aggregateByMetric(r.Scores)
}

func countUniqueFunctions(scores []Score) int {
	seen := map[*Function]struct{}{}
	for _, s := range scores {
		if s.Function != nil {
			seen[s.Function] = struct{}{}
		}
	}
	return len(seen)
}

func aggregateByMetric(scores []Score) []MetricStats {
	type acc struct {
		count    int
		sum      float64
		min, max float64
	}
	byID := map[string]*acc{}
	for _, s := range scores {
		a, ok := byID[s.MetricID]
		if !ok {
			a = &acc{min: s.Value, max: s.Value}
			byID[s.MetricID] = a
		}
		a.count++
		a.sum += s.Value
		if s.Value < a.min {
			a.min = s.Value
		}
		if s.Value > a.max {
			a.max = s.Value
		}
	}
	out := make([]MetricStats, 0, len(byID))
	for id, a := range byID {
		out = append(out, MetricStats{
			MetricID: id,
			Count:    a.count,
			Mean:     a.sum / float64(a.count),
			Min:      a.min,
			Max:      a.max,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MetricID < out[j].MetricID })
	return out
}
