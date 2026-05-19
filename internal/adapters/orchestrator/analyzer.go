// Package orchestrator wires the walker, parser, and metric registry into a
// single pipeline that turns a set of input paths into a Report.
package orchestrator

import (
	"context"
	"time"

	"github.com/jedi-knights/kyber/internal/domain"
	"github.com/jedi-knights/kyber/internal/ports"
)

// Analyzer runs every enabled metric against every function discovered by
// the walker and parsed by the parser. It is the orchestrator the CLI
// commands call into; it has no knowledge of how files are walked, how
// source is parsed, or how reports are rendered.
type Analyzer struct {
	walker   ports.FileWalker
	parser   ports.SourceParser
	registry *domain.Registry
}

// New constructs an Analyzer from its three collaborators.
func New(walker ports.FileWalker, parser ports.SourceParser, registry *domain.Registry) *Analyzer {
	return &Analyzer{walker: walker, parser: parser, registry: registry}
}

// Options configures one run.
type Options struct {
	Roots          []string
	WalkOpts       ports.WalkOptions
	EnabledMetrics []string                        // empty = all registered
	MetricOpts     map[string]domain.MetricOptions // keyed by metric ID
}

// Analyze walks the roots, parses every discovered file, and applies every
// enabled metric to every function. Errors parsing individual files become
// non-fatal entries on Report.Errors so a single broken file doesn't abort
// the whole run.
func (a *Analyzer) Analyze(ctx context.Context, opts Options) (*domain.Report, error) {
	start := time.Now()
	report := &domain.Report{StartTime: start}

	files, err := a.walker.Walk(ctx, opts.Roots, opts.WalkOpts)
	if err != nil {
		return nil, err
	}
	report.FilesScanned = len(files)

	funcs, err := a.parser.ParseFiles(ctx, files)
	if err != nil {
		report.Errors = append(report.Errors, err)
		report.EndTime = time.Now()
		return report, nil
	}

	metrics := a.registry.Enabled(opts.EnabledMetrics)
	for _, fn := range funcs {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return report, ctxErr
		}
		for _, m := range metrics {
			mopts := opts.MetricOpts[m.ID()]
			score, err := m.Analyze(ctx, fn, mopts)
			if err != nil {
				report.Errors = append(report.Errors, err)
				continue
			}
			report.Scores = append(report.Scores, score)
		}
	}
	report.EndTime = time.Now()
	return report, nil
}
