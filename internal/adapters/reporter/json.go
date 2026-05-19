package reporter

import (
	"encoding/json"
	"io"

	"github.com/jedi-knights/kyber/internal/domain"
)

// JSON renders a report as a structured JSON document. The shape is stable
// and intended for downstream tooling (CI dashboards, code-review bots).
type JSON struct{}

// NewJSON constructs a JSON reporter.
func NewJSON() *JSON { return &JSON{} }

type jsonReport struct {
	Scores       []jsonScore   `json:"scores"`
	Aggregates   jsonAggregate `json:"aggregates"`
	StartTime    string        `json:"start_time"`
	EndTime      string        `json:"end_time"`
	FilesScanned int           `json:"files_scanned"`
	Errors       []string      `json:"errors,omitempty"`
}

type jsonAggregate struct {
	Packages []jsonPackageStats `json:"packages"`
	Overall  jsonOverallStats   `json:"overall"`
}

type jsonPackageStats struct {
	ImportPath    string            `json:"import_path"`
	Name          string            `json:"name"`
	FunctionCount int               `json:"function_count"`
	Metrics       []jsonMetricStats `json:"metrics"`
}

type jsonOverallStats struct {
	FunctionCount int               `json:"function_count"`
	Metrics       []jsonMetricStats `json:"metrics"`
}

type jsonMetricStats struct {
	MetricID string  `json:"metric_id"`
	Count    int     `json:"count"`
	Mean     float64 `json:"mean"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
}

type jsonScore struct {
	MetricID string        `json:"metric_id"`
	Function jsonFunction  `json:"function"`
	Value    float64       `json:"value"`
	Findings []jsonFinding `json:"findings,omitempty"`
}

type jsonFunction struct {
	Name     string `json:"name"`
	Receiver string `json:"receiver,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type jsonFinding struct {
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
}

// Render writes the report to w as indented JSON.
func (JSON) Render(w io.Writer, r *domain.Report) error {
	out := jsonReport{
		StartTime:    r.StartTime.UTC().Format("2006-01-02T15:04:05Z"),
		EndTime:      r.EndTime.UTC().Format("2006-01-02T15:04:05Z"),
		FilesScanned: r.FilesScanned,
	}
	for _, s := range r.Scores {
		js := jsonScore{
			MetricID: s.MetricID,
			Function: jsonFunction{
				Name:     s.Function.Name,
				Receiver: s.Function.Receiver,
				File:     s.Function.File,
				Line:     s.Function.Position().Line,
			},
			Value: s.Value,
		}
		for _, f := range s.Findings {
			js.Findings = append(js.Findings, jsonFinding{
				Severity: f.Severity.String(),
				Line:     f.Line,
				Column:   f.Column,
				Message:  f.Message,
			})
		}
		out.Scores = append(out.Scores, js)
	}
	for _, e := range r.Errors {
		out.Errors = append(out.Errors, e.Error())
	}
	out.Aggregates = buildJSONAggregate(r)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func buildJSONAggregate(r *domain.Report) jsonAggregate {
	pkgStats := r.PackageStats()
	packages := make([]jsonPackageStats, 0, len(pkgStats))
	for _, ps := range pkgStats {
		packages = append(packages, jsonPackageStats{
			ImportPath:    ps.Package.ImportPath,
			Name:          ps.Package.Name,
			FunctionCount: ps.FunctionCount,
			Metrics:       convertMetricStats(ps.Metrics),
		})
	}
	overallCount, overallMetrics := r.OverallStats()
	return jsonAggregate{
		Packages: packages,
		Overall: jsonOverallStats{
			FunctionCount: overallCount,
			Metrics:       convertMetricStats(overallMetrics),
		},
	}
}

func convertMetricStats(in []domain.MetricStats) []jsonMetricStats {
	out := make([]jsonMetricStats, 0, len(in))
	for _, s := range in {
		out = append(out, jsonMetricStats{
			MetricID: s.MetricID,
			Count:    s.Count,
			Mean:     s.Mean,
			Min:      s.Min,
			Max:      s.Max,
		})
	}
	return out
}
