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

// maxLineWidth is the hard upper bound on any output line in the text format.
const maxLineWidth = 80

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

// fullName maps metric IDs to their human-readable names for the legend.
var fullName = map[string]string{
	"maintainability": "Maintainability Index",
	"cognitive":       "Cognitive Complexity",
	"cyclomatic":      "Cyclomatic Complexity",
	"difficulty":      "Halstead Difficulty",
	"effort":          "Halstead Effort",
	"funclen":         "Function Length",
	"halstead":        "Halstead Volume",
	"nesting":         "Maximum Nesting Depth",
	"npath":           "NPath Complexity",
	"readability":     "Readability Score",
	"returns":         "Return Count",
	"testability":     "Testability Score",
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

	renderLegend(w, metricIDs)

	totalFindings := 0
	for _, file := range files {
		scoresInFile := byFile[file]
		byFn := groupByFunctionName(scoresInFile)
		fnNames := sortedFunctionNames(byFn, scoresInFile)
		nameWidth := cappedNameWidth(fnNames)
		widths := columnWidths(metricIDs, byFn)

		panels := splitIntoPanels(nameWidth, metricIDs, widths)
		for pi, panel := range panels {
			if len(panels) > 1 {
				suffix := fmt.Sprintf("  [%d/%d]", pi+1, len(panels))
				fmt.Fprintf(w, "%s%s\n", truncatePath(file, maxLineWidth-len(suffix)), suffix)
			} else {
				fmt.Fprintln(w, truncatePath(file, maxLineWidth))
			}
			renderHeader(w, nameWidth, panel, widths)
			for _, name := range fnNames {
				scores := byFn[name]
				renderFunctionRow(w, name, nameWidth, scores, panel, widths)
				if pi == 0 {
					for _, s := range scores {
						totalFindings += len(s.Findings)
					}
				}
			}
			fmt.Fprintln(w)
		}
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
	display := name
	if len(display) > nameWidth {
		display = display[:nameWidth-3] + "..."
	}
	fmt.Fprintf(w, "  %-*s", nameWidth, display)
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

// renderLegend prints a two-column key mapping each short label to its full
// metric name, followed by a note explaining the ! threshold marker. Only
// metrics present in the report are shown. Nothing is printed for an empty
// metric list.
func renderLegend(w io.Writer, metricIDs []string) {
	if len(metricIDs) == 0 {
		return
	}
	entries := buildLegendEntries(metricIDs)
	maxWidth := maxLegendEntry(entries)
	twoPerLine := 2+maxWidth+2+maxWidth <= maxLineWidth
	fmt.Fprintln(w, "[LEGEND]")
	for i := 0; i < len(entries); {
		if twoPerLine && i+1 < len(entries) {
			fmt.Fprintf(w, "  %-*s  %s\n", maxWidth, entries[i], entries[i+1])
			i += 2
		} else {
			fmt.Fprintf(w, "  %s\n", entries[i])
			i++
		}
	}
	fmt.Fprintln(w, "  ! value crossed its configured threshold")
	fmt.Fprintln(w)
}

func buildLegendEntries(metricIDs []string) []string {
	entries := make([]string, 0, len(metricIDs))
	for _, id := range metricIDs {
		name := fullName[id]
		if name == "" {
			name = id
		}
		entries = append(entries, fmt.Sprintf("%s=%s", labelFor(id), name))
	}
	return entries
}

func maxLegendEntry(entries []string) int {
	n := 0
	for _, e := range entries {
		if len(e) > n {
			n = len(e)
		}
	}
	return n
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
		single := fmt.Sprintf("  %-*s   %s   (%d fns)",
			nameWidth, ps.Package.ImportPath, strings.Join(cells, " "), ps.FunctionCount)
		if len(single) <= maxLineWidth {
			fmt.Fprintln(w, single)
		} else {
			suffix := fmt.Sprintf("  (%d fns)", ps.FunctionCount)
			path := truncatePath(ps.Package.ImportPath, maxLineWidth-2-len(suffix))
			fmt.Fprintf(w, "  %s%s\n", path, suffix)
			for _, l := range wrapStatCells(cells, "    ") {
				fmt.Fprintln(w, l)
			}
		}
	}
	fmt.Fprintln(w)

	overallCount, overallMetrics := r.OverallStats()
	cells := renderStatsCells(overallMetrics, metricIDs)
	single := fmt.Sprintf("  %s   (%d fns)", strings.Join(cells, " "), overallCount)
	if len(single) <= maxLineWidth {
		fmt.Fprintln(w, "[OVERALL]")
		fmt.Fprintln(w, single)
	} else {
		fmt.Fprintf(w, "[OVERALL]  (%d fns)\n", overallCount)
		for _, l := range wrapStatCells(cells, "  ") {
			fmt.Fprintln(w, l)
		}
	}
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

// cappedNameWidth returns the column width for function names, capped so at
// least one metric column can still fit in maxLineWidth.
func cappedNameWidth(names []string) int {
	n := longestName(names)
	const cap = maxLineWidth - 2 - 3 // 2-char indent + narrowest possible metric col
	if n > cap {
		return cap
	}
	return n
}

// truncatePath trims path to maxLen characters, appending "..." when truncated.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	if maxLen <= 3 {
		return path[:maxLen]
	}
	return path[:maxLen-3] + "..."
}

// splitIntoPanels partitions metricIDs into groups such that each panel's
// rendered row (2-char indent + nameWidth + metric columns) fits within
// maxLineWidth. At least one metric is always placed in each panel.
func splitIntoPanels(nameWidth int, metricIDs []string, widths map[string]int) [][]string {
	base := 2 + nameWidth
	var panels [][]string
	var panel []string
	used := base
	for _, id := range metricIDs {
		colWidth := 1 + widths[id]
		if len(panel) > 0 && used+colWidth > maxLineWidth {
			panels = append(panels, panel)
			panel = nil
			used = base
		}
		panel = append(panel, id)
		used += colWidth
	}
	if len(panel) > 0 {
		panels = append(panels, panel)
	}
	return panels
}

// wrapStatCells packs key=value cells onto lines prefixed by indent, breaking
// before a cell would push the line past maxLineWidth. Each cell is placed on
// its own line if it alone exceeds the limit.
func wrapStatCells(cells []string, indent string) []string {
	var lines []string
	line := indent
	for _, c := range cells {
		candidate := c
		if line != indent {
			candidate = " " + c
		}
		if len(line)+len(candidate) > maxLineWidth && line != indent {
			lines = append(lines, line)
			line = indent + c
		} else {
			line += candidate
		}
	}
	if line != indent {
		lines = append(lines, line)
	}
	return lines
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
