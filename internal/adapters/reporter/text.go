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

// Render writes the report to w. Lines are grouped by source file and
// sorted by function start position. Per-package and overall aggregates
// are printed after the per-function detail.
func (Text) Render(w io.Writer, r *domain.Report) error {
	byFile := groupScoresByFile(r.Scores)
	files := sortedKeys(byFile)

	totalFindings := 0
	for _, file := range files {
		scoresInFile := byFile[file]
		fmt.Fprintln(w, file)
		byFn := groupByFunctionName(scoresInFile)
		fnNames := sortedFunctionNames(byFn, scoresInFile)
		nameWidth := longestName(fnNames)
		for _, name := range fnNames {
			scores := byFn[name]
			cells := renderScoreCells(scores)
			fmt.Fprintf(w, "  %-*s   %s\n", nameWidth, name, strings.Join(cells, "   "))
			for _, s := range scores {
				totalFindings += len(s.Findings)
			}
		}
		fmt.Fprintln(w)
	}

	renderAggregates(w, r)

	duration := r.EndTime.Sub(r.StartTime).Round(1e6)
	fmt.Fprintf(w, "Functions: %d   Findings: %d   Files: %d   Time: %s\n",
		uniqueFunctionCount(r.Scores), totalFindings, r.FilesScanned, duration)
	return nil
}

func renderAggregates(w io.Writer, r *domain.Report) {
	pkgStats := r.PackageStats()
	if len(pkgStats) == 0 {
		return
	}
	fmt.Fprintln(w, "[PACKAGE MEANS]")
	nameWidth := longestPackageName(pkgStats)
	for _, ps := range pkgStats {
		cells := renderStatsCells(ps.Metrics)
		fmt.Fprintf(w, "  %-*s   %s   (%d fns)\n",
			nameWidth, ps.Package.ImportPath, strings.Join(cells, "   "), ps.FunctionCount)
	}
	fmt.Fprintln(w)

	overallCount, overallMetrics := r.OverallStats()
	fmt.Fprintln(w, "[OVERALL]")
	fmt.Fprintf(w, "  %s   (%d fns)\n",
		strings.Join(renderStatsCells(overallMetrics), "   "), overallCount)
	fmt.Fprintln(w)
}

func renderStatsCells(stats []domain.MetricStats) []string {
	out := make([]string, 0, len(stats))
	for _, s := range stats {
		out = append(out, fmt.Sprintf("%s=%s", s.MetricID, formatValue(s.Mean)))
	}
	return out
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

func renderScoreCells(scores []domain.Score) []string {
	// Sort by metric ID so column order is stable across functions.
	sort.Slice(scores, func(i, j int) bool { return scores[i].MetricID < scores[j].MetricID })
	out := make([]string, 0, len(scores))
	for _, s := range scores {
		marker := ""
		if len(s.Findings) > 0 {
			marker = " !"
		}
		out = append(out, fmt.Sprintf("%s=%s%s", s.MetricID, formatValue(s.Value), marker))
	}
	return out
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
