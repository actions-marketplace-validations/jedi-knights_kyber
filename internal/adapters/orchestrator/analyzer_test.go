package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jedi-knights/kyber/internal/adapters/parser"
	"github.com/jedi-knights/kyber/internal/adapters/walker"
	"github.com/jedi-knights/kyber/internal/domain"
	"github.com/jedi-knights/kyber/internal/domain/metrics"
	"github.com/jedi-knights/kyber/internal/ports"
)

func testdataRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return root
}

func TestAnalyzer_EndToEnd(t *testing.T) {
	a := New(walker.New(), parser.New(), metrics.DefaultRegistry())
	report, err := a.Analyze(context.Background(), Options{
		Roots:    []string{testdataRoot(t) + "/..."},
		WalkOpts: ports.WalkOptions{},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(report.Scores) == 0 {
		t.Fatal("expected at least one score")
	}
	// Every function should receive one score per registered metric.
	want := len(metrics.DefaultRegistry().All())
	byFn := report.ByFunction()
	for fn, scores := range byFn {
		if len(scores) != want {
			t.Errorf("function %s has %d scores, want %d (one per registered metric)",
				fn.Name, len(scores), want)
		}
	}
}

func TestAnalyzer_RespectsEnabledFilter(t *testing.T) {
	a := New(walker.New(), parser.New(), metrics.DefaultRegistry())
	report, err := a.Analyze(context.Background(), Options{
		Roots:          []string{testdataRoot(t) + "/..."},
		EnabledMetrics: []string{"cyclomatic"},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for _, s := range report.Scores {
		if s.MetricID != "cyclomatic" {
			t.Errorf("got score for %q, want only cyclomatic when filter is set", s.MetricID)
		}
	}
}

func TestAnalyzer_ByMetric(t *testing.T) {
	a := New(walker.New(), parser.New(), metrics.DefaultRegistry())
	report, err := a.Analyze(context.Background(), Options{
		Roots: []string{testdataRoot(t) + "/..."},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	cyclo := report.ByMetric("cyclomatic")
	if len(cyclo) == 0 {
		t.Errorf("ByMetric(cyclomatic) returned no scores")
	}
	// Branchy from complex/ should appear with value 12 and a finding.
	var foundBranchy bool
	for _, s := range cyclo {
		if s.Function.Name == "Branchy" {
			foundBranchy = true
			if s.Value != 12 {
				t.Errorf("Branchy cyclomatic = %v, want 12", s.Value)
			}
			if len(s.Findings) == 0 {
				t.Errorf("Branchy should have findings (12 > 7)")
			}
		}
	}
	if !foundBranchy {
		t.Errorf("did not find Branchy in cyclomatic scores")
	}
}

func TestAnalyzer_CancelledContext(t *testing.T) {
	a := New(walker.New(), parser.New(), metrics.DefaultRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Analyze(ctx, Options{Roots: []string{testdataRoot(t) + "/..."}})
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

// Ensure that the registry helper from the metrics package is accessible to
// downstream callers without circular imports.
var _ = domain.NewRegistry
