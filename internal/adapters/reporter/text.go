// Package reporter renders kyber Reports in text, JSON, or SARIF formats.
// All three implementations of ports.Reporter live here side-by-side so
// adding a fourth format is one new file plus a switch entry in the CLI.
package reporter

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jedi-knights/kyber/internal/domain"
)

// Text renders a human-readable per-file table.
type Text struct{}

// NewText constructs a Text reporter.
func NewText() *Text { return &Text{} }

// shortLabel maps full metric IDs to their compact column labels used in
// the per-function table and the aggregate lines. The text reporter owns
// this mapping so the domain Metric interface stays free of presentation
// concerns. Unknown metric IDs fall back to the first 4 lowercase chars.
var shortLabel = map[string]string{
	"maintainability": "mi",
	"cognitive":       "cog",
	"cyclomatic":      "cyc",
	"difficulty":      "diff",
	"effort":          "effrt",
	"funclen":         "fln",
	"halstead":        "hal",
	"nesting":         "nst",
	"npath":           "npth",
	"readability":     "read",
	"returns":         "ret",
	"testability":     "tst",
}

// labelFor returns the short column label for a metric ID, falling back
// to the first 4 lowercase chars when the ID is unknown.
func labelFor(id string) string {
	if s, ok := shortLabel[id]; ok {
		return s
	}
	if len(id) > 4 {
		return strings.ToLower(id[:4])
	}
	return strings.ToLower(id)
}

// Render writes the report to w. Lines are grouped by source file and
// sorted by function start position. Per-package and overall aggregates
// are printed after the per-function detail.
func (Text) Render(w io.Writer, r *domain.Report) error {
	byFile := groupScoresByFile(r.Scores)
	files := sortedKeys(byFile)

	metricIDs := metricOrder(metricIDsInReport(r.Scores))

	totalFindings := 0
	for _, file := range files {
		scoresInFile := byFile[file]
		byFn := groupByFunctionName(scoresInFile)
		fnNames := sortedFunctionNames(byFn, scoresInFile)
		nameWidth := longestName(fnNames)
		widths := columnWidths(metricIDs, byFn)

		fmt.Fprintln(w, file)
		renderHeader(w, nameWidth, metricIDs, widths)
		for _, name := range fnNames {
			scores := byFn[name]
			renderFunctionRow(w, name, nameWidth, scores, metricIDs, widths)
			for _, s := range scores {
				totalFindings += len(s.Findings)
			}
		}
		fmt.Fprintln(w)
	}

	renderAggregates(w, r, metricIDs)

	duration := r.EndTime.Sub(r.StartTime).Round(1e6)
	fmt.Fprintf(w, "Functions: %d   Findings: %d   Files: %d   Time: %s\n",
		uniqueFunctionCount(r.Scores), totalFindings, r.FilesScanned, duration)
	return nil
}

// renderHeader writes the column header — blank space under the function
// name column, then each metric's short label right-aligned to its width.
func renderHeader(w io.Writer, nameWidth int, metricIDs []string, widths map[string]int) {
	fmt.Fprintf(w, "  %-*s", nameWidth, "")
	for _, id := range metricIDs {
		fmt.Fprintf(w, " %*s", widths[id], labelFor(id))
	}
	fmt.Fprintln(w)
}

// renderFunctionRow writes one function's row — name then each metric's
// value right-aligned, with `!` appended in-column when the score is flagged.
func renderFunctionRow(w io.Writer, name string, nameWidth int, scores []domain.Score, metricIDs []string, widths map[string]int) {
	byID := make(map[string]domain.Score, len(scores))
	for _, s := range scores {
		byID[s.MetricID] = s
	}
	fmt.Fprintf(w, "  %-*s", nameWidth, name)
	for _, id := range metricIDs {
		s, ok := byID[id]
		cell := ""
		if ok {
			cell = formatValue(s.Value)
			if len(s.Findings) > 0 {
				cell += "!"
			}
		}
		fmt.Fprintf(w, " %*s", widths[id], cell)
	}
	fmt.Fprintln(w)
}

func renderAggregates(w io.Writer, r *domain.Report, metricIDs []string) {
	pkgStats := r.PackageStats()
	if len(pkgStats) == 0 {
		return
	}
	fmt.Fprintln(w, "[PACKAGE MEANS]")
	nameWidth := longestPackageName(pkgStats)
	for _, ps := range pkgStats {
		cells := renderStatsCells(ps.Metrics, metricIDs)
		fmt.Fprintf(w, "  %-*s   %s   (%d fns)\n",
			nameWidth, ps.Package.ImportPath, strings.Join(cells, " "), ps.FunctionCount)
	}
	fmt.Fprintln(w)

	overallCount, overallMetrics := r.OverallStats()
	fmt.Fprintln(w, "[OVERALL]")
	fmt.Fprintf(w, "  %s   (%d fns)\n",
		strings.Join(renderStatsCells(overallMetrics, metricIDs), " "), overallCount)
	fmt.Fprintln(w)
}

// renderStatsCells emits one `label=value` cell per metric, ordered to
// match the per-function table (MI first, then alphabetical).
func renderStatsCells(stats []domain.MetricStats, metricIDs []string) []string {
	byID := make(map[string]domain.MetricStats, len(stats))
	for _, s := range stats {
		byID[s.MetricID] = s
	}
	out := make([]string, 0, len(metricIDs))
	for _, id := range metricIDs {
		s, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%s=%s", labelFor(id), formatValue(s.Mean)))
	}
	return out
}

// metricIDsInReport returns the unique set of metric IDs that appear in
// the report's scores. Used to derive table columns directly from data,
// so `--metric` / `--disable` filtering needs no extra wiring.
func metricIDsInReport(scores []domain.Score) []string {
	seen := make(map[string]struct{})
	for _, s := range scores {
		seen[s.MetricID] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// metricOrder sorts metric IDs so `maintainability` is first when present
// and the rest follow alphabetically. Stable across runs for snapshot tests.
func metricOrder(ids []string) []string {
	out := make([]string, len(ids))
	copy(out, ids)
	sort.Slice(out, func(i, j int) bool {
		if out[i] == "maintainability" {
			return true
		}
		if out[j] == "maintainability" {
			return false
		}
		return out[i] < out[j]
	})
	return out
}

// columnWidths returns, per metric ID, the column width needed to fit the
// short label and every value (including a trailing `!` for flagged scores)
// across all functions in a file.
func columnWidths(metricIDs []string, byFn map[string][]domain.Score) map[string]int {
	widths := make(map[string]int, len(metricIDs))
	for _, id := range metricIDs {
		widths[id] = len(labelFor(id))
	}
	for _, scores := range byFn {
		for _, s := range scores {
			cell := formatValue(s.Value)
			if len(s.Findings) > 0 {
				cell += "!"
			}
			if w := len(cell); w > widths[s.MetricID] {
				widths[s.MetricID] = w
			}
		}
	}
	return widths
}

func longestPackageName(ps []domain.PackageStats) int {
	n := 0
	for _, p := range ps {
		if len(p.Package.ImportPath) > n {
			n = len(p.Package.ImportPath)
		}
	}
	return n
}

func formatValue(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.2f", v)
}

func groupScoresByFile(scores []domain.Score) map[string][]domain.Score {
	out := make(map[string][]domain.Score)
	for _, s := range scores {
		out[s.Function.File] = append(out[s.Function.File], s)
	}
	return out
}

func groupByFunctionName(scores []domain.Score) map[string][]domain.Score {
	out := make(map[string][]domain.Score)
	for _, s := range scores {
		out[s.Function.Name] = append(out[s.Function.Name], s)
	}
	return out
}

func sortedKeys(m map[string][]domain.Score) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedFunctionNames orders names by their function's start line within the
// file so reports follow source order rather than alphabetical order.
func sortedFunctionNames(byFn map[string][]domain.Score, fileScores []domain.Score) []string {
	startLines := make(map[string]int, len(byFn))
	for _, s := range fileScores {
		line := s.Function.Position().Line
		if existing, ok := startLines[s.Function.Name]; !ok || line < existing {
			startLines[s.Function.Name] = line
		}
	}
	out := make([]string, 0, len(byFn))
	for name := range byFn {
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool { return startLines[out[i]] < startLines[out[j]] })
	return out
}

func longestName(names []string) int {
	n := 0
	for _, s := range names {
		if len(s) > n {
			n = len(s)
		}
	}
	return n
}

func uniqueFunctionCount(scores []domain.Score) int {
	seen := make(map[*domain.Function]struct{})
	for _, s := range scores {
		seen[s.Function] = struct{}{}
	}
	return len(seen)
}
