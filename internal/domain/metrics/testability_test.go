package metrics

import (
	"context"
	"testing"

	"github.com/jedi-knights/kyber/internal/domain"
)

func TestTestability_SimpleScoresHigh(t *testing.T) {
	fn := findFunc(t, parseFixture(t, "simple"), "Add")
	score, err := NewTestability().Analyze(context.Background(), fn, domain.MetricOptions{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	// 2 params (max 5), no side effects, 0/2 interfaces (signal 0), 3 lines.
	// score = (1 - 2/5 + 1 + 0 + 1 - 3/40) / 4 ≈ (0.6 + 1 + 0 + 0.925) / 4 ≈ 0.63
	if score.Value < 0.5 {
		t.Errorf("Value = %v, want ≥ 0.5", score.Value)
	}
}

func TestTestability_UntestableScoresLow(t *testing.T) {
	fn := findFunc(t, parseFixture(t, "untestable"), "Dispatch")
	score, err := NewTestability().Analyze(context.Background(), fn, domain.MetricOptions{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value >= 0.6 {
		t.Errorf("Value = %v, want < 0.6 for untestable fixture", score.Value)
	}
	if len(score.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(score.Findings))
	}
}

func TestTestability_InterfaceParamScoresHigher(t *testing.T) {
	funcs := parseFixture(t, "package_context")
	viaInterface := findFunc(t, funcs, "SendViaInterface")
	viaConcrete := findFunc(t, funcs, "SendViaConcrete")

	scoreIface, err := NewTestability().Analyze(context.Background(), viaInterface, domain.MetricOptions{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	scoreConcrete, err := NewTestability().Analyze(context.Background(), viaConcrete, domain.MetricOptions{})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if scoreIface.Value <= scoreConcrete.Value {
		t.Errorf("interface-typed parameters should score ≥ concrete: iface=%v concrete=%v",
			scoreIface.Value, scoreConcrete.Value)
	}
}

func TestTestability_PureFmtIsNotSideEffect(t *testing.T) {
	// Arrange — PureFmt calls only fmt.Sprintf/Errorf; IOFmt calls Println/Printf.
	// Same param count, same interface fraction, comparable length — the only
	// difference is the side-effect class of the fmt calls used.
	funcs := parseFixture(t, "fmtcalls")
	pure := findFunc(t, funcs, "PureFmt")
	io := findFunc(t, funcs, "IOFmt")
	opts := domain.MetricOptions{
		Params: map[string]any{
			"weight_params":       0.0,
			"weight_side_effects": 1.0,
			"weight_interfaces":   0.0,
			"weight_length":       0.0,
		},
	}

	// Act
	pureScore, err := NewTestability().Analyze(context.Background(), pure, opts)
	if err != nil {
		t.Fatalf("Analyze pure: %v", err)
	}
	ioScore, err := NewTestability().Analyze(context.Background(), io, opts)
	if err != nil {
		t.Fatalf("Analyze io: %v", err)
	}

	// Assert — with only the side-effect signal weighted, PureFmt should score
	// at the ceiling (no side effects detected) and IOFmt should score strictly
	// lower (Println + Printf are real I/O).
	if pureScore.Value != 1 {
		t.Errorf("PureFmt side-effect signal = %v, want 1.0 (Sprintf/Errorf are pure)", pureScore.Value)
	}
	if ioScore.Value >= pureScore.Value {
		t.Errorf("IOFmt %v should score below PureFmt %v", ioScore.Value, pureScore.Value)
	}
}

func TestTestability_GlobalAccessIsSideEffect(t *testing.T) {
	// BumpCounter reads and writes a package-level global PackageCounter,
	// which should be detected via Package.Globals and contribute to the
	// side-effect signal — driving the score lower than a clean function.
	bump := findFunc(t, parseFixture(t, "package_context"), "BumpCounter")
	// Weight only the side-effect signal.
	opts := domain.MetricOptions{
		Threshold: 0.6,
		Params: map[string]any{
			"weight_params":       0.0,
			"weight_side_effects": 1.0,
			"weight_interfaces":   0.0,
			"weight_length":       0.0,
		},
	}
	score, err := NewTestability().Analyze(context.Background(), bump, opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if score.Value >= 1 {
		t.Errorf("Value = %v, want < 1 (global accesses should be penalized)", score.Value)
	}
}
