package domain

import (
	"math"
	"testing"
)

func newTestReport() *Report {
	walker := &Package{ImportPath: "/repo/internal/walker", Name: "walker"}
	parser := &Package{ImportPath: "/repo/internal/parser", Name: "parser"}

	walk := &Function{Name: "Walk", Package: walker}
	helper := &Function{Name: "helper", Package: walker}
	parse := &Function{Name: "Parse", Package: parser}

	return &Report{
		Scores: []Score{
			{MetricID: "cyclomatic", Function: walk, Value: 6},
			{MetricID: "cyclomatic", Function: helper, Value: 2},
			{MetricID: "cyclomatic", Function: parse, Value: 4},
			{MetricID: "halstead", Function: walk, Value: 500},
			{MetricID: "halstead", Function: helper, Value: 100},
			{MetricID: "halstead", Function: parse, Value: 300},
		},
	}
}

func TestReport_PackageStats_OneEntryPerPackage(t *testing.T) {
	// Arrange
	r := newTestReport()

	// Act
	stats := r.PackageStats()

	// Assert
	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2", len(stats))
	}
	if stats[0].Package.ImportPath != "/repo/internal/parser" {
		t.Errorf("stats[0] = %q, want parser first (sorted)", stats[0].Package.ImportPath)
	}
}

func TestReport_PackageStats_ComputesMeanMinMax(t *testing.T) {
	// Arrange
	r := newTestReport()

	// Act
	stats := r.PackageStats()

	// Assert — walker has Walk(6) and helper(2): mean=4, min=2, max=6.
	var walker *PackageStats
	for i, s := range stats {
		if s.Package.Name == "walker" {
			walker = &stats[i]
		}
	}
	if walker == nil {
		t.Fatal("walker package not found in stats")
	}
	if walker.FunctionCount != 2 {
		t.Errorf("FunctionCount = %d, want 2", walker.FunctionCount)
	}
	var cyc *MetricStats
	for i, m := range walker.Metrics {
		if m.MetricID == "cyclomatic" {
			cyc = &walker.Metrics[i]
		}
	}
	if cyc == nil {
		t.Fatal("cyclomatic stats not found")
	}
	if cyc.Mean != 4 || cyc.Min != 2 || cyc.Max != 6 || cyc.Count != 2 {
		t.Errorf("cyc stats = %+v, want mean=4 min=2 max=6 count=2", *cyc)
	}
}

func TestReport_OverallStats_AggregatesAcrossPackages(t *testing.T) {
	// Arrange
	r := newTestReport()

	// Act
	functionCount, metrics := r.OverallStats()

	// Assert
	if functionCount != 3 {
		t.Errorf("functionCount = %d, want 3", functionCount)
	}
	if len(metrics) != 2 {
		t.Fatalf("len(metrics) = %d, want 2 (cyclomatic, halstead)", len(metrics))
	}
	// Cyclomatic: 6,2,4 → mean 4, min 2, max 6.
	var cyc *MetricStats
	for i, m := range metrics {
		if m.MetricID == "cyclomatic" {
			cyc = &metrics[i]
		}
	}
	if cyc == nil {
		t.Fatal("cyclomatic overall stats not found")
	}
	if cyc.Count != 3 {
		t.Errorf("Count = %d, want 3", cyc.Count)
	}
	if math.Abs(cyc.Mean-4) > 1e-9 {
		t.Errorf("Mean = %v, want 4", cyc.Mean)
	}
	if cyc.Min != 2 || cyc.Max != 6 {
		t.Errorf("Min/Max = %v/%v, want 2/6", cyc.Min, cyc.Max)
	}
}

func TestReport_PackageStats_EmptyReport(t *testing.T) {
	// Arrange
	r := &Report{}

	// Act
	stats := r.PackageStats()

	// Assert
	if len(stats) != 0 {
		t.Errorf("len(stats) = %d, want 0 for empty report", len(stats))
	}
}
