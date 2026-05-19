package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/jedi-knights/kyber/internal/adapters/orchestrator"
	"github.com/jedi-knights/kyber/internal/adapters/parser"
	"github.com/jedi-knights/kyber/internal/adapters/reporter"
	"github.com/jedi-knights/kyber/internal/adapters/walker"
	"github.com/jedi-knights/kyber/internal/config"
	"github.com/jedi-knights/kyber/internal/domain"
	"github.com/jedi-knights/kyber/internal/domain/metrics"
	"github.com/jedi-knights/kyber/internal/ports"
)

type analyzeFlags struct {
	configPath           string
	metrics              []string
	disable              []string
	thresholdCyclomatic  float64
	thresholdReadability float64
	thresholdTestability float64
	format               string
	output               string
	failOnThreshold      bool
	exclude              []string
	includeTests         bool
	verbose              bool
}

func newAnalyzeCmd() *cobra.Command {
	f := &analyzeFlags{}
	cmd := &cobra.Command{
		Use:   "analyze [paths...]",
		Short: "Analyze Go source for function-level metrics.",
		Long:  "Walk the given paths, parse every .go file (excluding vendor/, testdata/, and *_test.go by default), and emit a per-function score for every enabled metric.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd.Context(), cmd.OutOrStdout(), args, f)
		},
	}

	cmd.Flags().StringVarP(&f.configPath, "config", "c", "kyber.toml", "path to config file")
	cmd.Flags().StringArrayVar(&f.metrics, "metric", nil, "enable only these metrics (repeatable)")
	cmd.Flags().StringArrayVar(&f.disable, "disable", nil, "explicitly disable these metrics (repeatable)")
	cmd.Flags().Float64Var(&f.thresholdCyclomatic, "threshold-cyclomatic", 0, "override cyclomatic threshold")
	cmd.Flags().Float64Var(&f.thresholdReadability, "threshold-readability", 0, "override readability threshold")
	cmd.Flags().Float64Var(&f.thresholdTestability, "threshold-testability", 0, "override testability threshold")
	cmd.Flags().StringVar(&f.format, "format", "", "output format: text | json | sarif")
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "write report to file instead of stdout")
	cmd.Flags().BoolVar(&f.failOnThreshold, "fail-on-threshold", false, "exit 1 if any function exceeds a metric threshold")
	cmd.Flags().StringArrayVar(&f.exclude, "exclude", nil, "glob patterns to exclude (repeatable)")
	cmd.Flags().BoolVar(&f.includeTests, "include-tests", false, "include *_test.go files in analysis")
	cmd.Flags().BoolVarP(&f.verbose, "verbose", "v", false, "verbose progress output")

	return cmd
}

func runAnalyze(ctx context.Context, stdout io.Writer, args []string, f *analyzeFlags) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return err
	}
	applyFlags(&cfg, args, f)

	registry := metrics.DefaultRegistry()
	report, err := analyzeOnce(ctx, registry, cfg, f)
	if err != nil {
		return err
	}
	if err := writeReport(stdout, cfg.Format, f.output, report); err != nil {
		return err
	}
	if cfg.FailOnThreshold && len(report.ExceedingThresholds()) > 0 {
		return fmt.Errorf("%d function(s) exceeded threshold", len(report.ExceedingThresholds()))
	}
	return nil
}

func analyzeOnce(ctx context.Context, registry *domain.Registry, cfg config.Config, f *analyzeFlags) (*domain.Report, error) {
	enabled := resolveEnabledMetrics(&cfg, registry, f)
	mopts := buildMetricOptions(cfg, registry, f)
	a := orchestrator.New(walker.New(), parser.New(), registry)
	report, err := a.Analyze(ctx, orchestrator.Options{
		Roots:          cfg.Paths,
		WalkOpts:       ports.WalkOptions{ExcludeGlobs: cfg.Exclude, IncludeTests: f.includeTests},
		EnabledMetrics: enabled,
		MetricOpts:     mopts,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	return report, nil
}

func writeReport(stdout io.Writer, format, outPath string, report *domain.Report) error {
	w, closer, err := openOutput(stdout, outPath)
	if err != nil {
		return err
	}
	defer closer()
	rep, err := reporterFor(format)
	if err != nil {
		return err
	}
	if err := rep.Render(w, report); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// applyFlags overlays positional args and CLI flags onto cfg.
func applyFlags(cfg *config.Config, args []string, f *analyzeFlags) {
	if len(args) > 0 {
		cfg.Paths = args
	}
	if f.format != "" {
		cfg.Format = f.format
	}
	if f.failOnThreshold {
		cfg.FailOnThreshold = true
	}
	if f.verbose {
		cfg.Verbose = true
	}
	if len(f.exclude) > 0 {
		cfg.Exclude = f.exclude
	}
}

// resolveEnabledMetrics combines TOML and flag inputs. --metric is an
// allowlist; --disable subtracts from whatever set is left.
func resolveEnabledMetrics(cfg *config.Config, registry *domain.Registry, f *analyzeFlags) []string {
	enabled := initialEnabledMetrics(cfg, registry, f)
	return subtractDisabled(enabled, f.disable)
}

func initialEnabledMetrics(cfg *config.Config, registry *domain.Registry, f *analyzeFlags) []string {
	switch {
	case len(f.metrics) > 0:
		return f.metrics
	case len(cfg.Metrics) > 0:
		return cfg.EnabledIDs()
	}
	ids := make([]string, 0, len(registry.All()))
	for _, m := range registry.All() {
		ids = append(ids, m.ID())
	}
	return ids
}

func subtractDisabled(enabled, disable []string) []string {
	if len(disable) == 0 {
		return enabled
	}
	disabled := make(map[string]bool, len(disable))
	for _, id := range disable {
		disabled[id] = true
	}
	out := enabled[:0]
	for _, id := range enabled {
		if !disabled[id] {
			out = append(out, id)
		}
	}
	return out
}

// buildMetricOptions assembles per-metric options from defaults, TOML, and
// CLI threshold overrides.
func buildMetricOptions(cfg config.Config, registry *domain.Registry, f *analyzeFlags) map[string]domain.MetricOptions {
	out := make(map[string]domain.MetricOptions)
	for _, m := range registry.All() {
		opts := domain.MetricOptions{
			Threshold: m.DefaultThreshold(),
			Params:    map[string]any{},
		}
		if mc, ok := cfg.Metrics[m.ID()]; ok {
			if mc.Threshold != 0 {
				opts.Threshold = mc.Threshold
			}
			for k, v := range mc.Params {
				opts.Params[k] = v
			}
		}
		applyThresholdFlag(m.ID(), &opts, f)
		out[m.ID()] = opts
	}
	return out
}

func applyThresholdFlag(id string, opts *domain.MetricOptions, f *analyzeFlags) {
	switch id {
	case "cyclomatic":
		if f.thresholdCyclomatic > 0 {
			opts.Threshold = f.thresholdCyclomatic
		}
	case "readability":
		if f.thresholdReadability > 0 {
			opts.Threshold = f.thresholdReadability
		}
	case "testability":
		if f.thresholdTestability > 0 {
			opts.Threshold = f.thresholdTestability
		}
	}
}

func openOutput(stdout io.Writer, path string) (io.Writer, func(), error) {
	if path == "" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open output %s: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

func reporterFor(format string) (ports.Reporter, error) {
	switch format {
	case "", "text":
		return reporter.NewText(), nil
	case "json":
		return reporter.NewJSON(), nil
	case "sarif":
		return reporter.NewSARIF(""), nil
	default:
		return nil, fmt.Errorf("unknown format %q (want text, json, or sarif)", format)
	}
}
