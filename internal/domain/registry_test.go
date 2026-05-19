package domain

import (
	"context"
	"strings"
	"testing"
)

// fakeMetric is a minimal Metric used only for registry tests.
type fakeMetric struct{ id string }

func (f fakeMetric) ID() string                { return f.id }
func (f fakeMetric) Name() string              { return f.id }
func (f fakeMetric) Description() string       { return "fake " + f.id }
func (f fakeMetric) DefaultThreshold() float64 { return 0 }
func (f fakeMetric) HigherIsWorse() bool       { return true }
func (f fakeMetric) Analyze(context.Context, *Function, MetricOptions) (Score, error) {
	return Score{MetricID: f.id}, nil
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeMetric{id: "a"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register(fakeMetric{id: "a"})
	if err == nil {
		t.Fatalf("expected error on duplicate registration, got nil")
	}
	if !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("error %q should mention the duplicate id", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(fakeMetric{id: "alpha"})
	if got := r.Get("alpha"); got == nil || got.ID() != "alpha" {
		t.Errorf("Get(alpha) = %v, want metric with id alpha", got)
	}
	if got := r.Get("missing"); got != nil {
		t.Errorf("Get(missing) = %v, want nil", got)
	}
}

func TestRegistry_All_Sorted(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(fakeMetric{id: "charlie"})
	r.MustRegister(fakeMetric{id: "alpha"})
	r.MustRegister(fakeMetric{id: "bravo"})
	got := r.All()
	want := []string{"alpha", "bravo", "charlie"}
	if len(got) != len(want) {
		t.Fatalf("All() returned %d items, want %d", len(got), len(want))
	}
	for i, m := range got {
		if m.ID() != want[i] {
			t.Errorf("All()[%d].ID() = %q, want %q", i, m.ID(), want[i])
		}
	}
}

func TestRegistry_Enabled(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(fakeMetric{id: "alpha"})
	r.MustRegister(fakeMetric{id: "bravo"})
	r.MustRegister(fakeMetric{id: "charlie"})

	t.Run("empty returns all", func(t *testing.T) {
		got := r.Enabled(nil)
		if len(got) != 3 {
			t.Errorf("Enabled(nil) returned %d items, want 3 (all)", len(got))
		}
	})

	t.Run("filters and sorts", func(t *testing.T) {
		got := r.Enabled([]string{"charlie", "alpha"})
		if len(got) != 2 {
			t.Fatalf("got %d items, want 2", len(got))
		}
		if got[0].ID() != "alpha" || got[1].ID() != "charlie" {
			t.Errorf("Enabled returned %s,%s; want alpha,charlie",
				got[0].ID(), got[1].ID())
		}
	})

	t.Run("skips unknown ids", func(t *testing.T) {
		got := r.Enabled([]string{"alpha", "unknown", "bravo"})
		if len(got) != 2 {
			t.Errorf("got %d items, want 2 (unknown skipped)", len(got))
		}
	})
}

func TestMetricOptions_FloatParam(t *testing.T) {
	opts := MetricOptions{Params: map[string]any{
		"f": 3.5,
		"i": 7,
	}}
	if got := opts.FloatParam("f", 0); got != 3.5 {
		t.Errorf("FloatParam(f) = %v, want 3.5", got)
	}
	if got := opts.FloatParam("i", 0); got != 7 {
		t.Errorf("FloatParam(i) = %v, want 7 (int → float)", got)
	}
	if got := opts.FloatParam("missing", 9.9); got != 9.9 {
		t.Errorf("FloatParam(missing) = %v, want default 9.9", got)
	}
}

func TestMetricOptions_IntParam(t *testing.T) {
	opts := MetricOptions{Params: map[string]any{
		"f": 3.5,
		"i": 7,
	}}
	if got := opts.IntParam("i", 0); got != 7 {
		t.Errorf("IntParam(i) = %v, want 7", got)
	}
	if got := opts.IntParam("f", 0); got != 3 {
		t.Errorf("IntParam(f) = %v, want 3 (float truncated)", got)
	}
	if got := opts.IntParam("missing", 42); got != 42 {
		t.Errorf("IntParam(missing) = %v, want default 42", got)
	}
}
