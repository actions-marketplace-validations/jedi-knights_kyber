package reporter

import (
	"encoding/json"
	"io"

	"github.com/jedi-knights/kyber/internal/domain"
)

// SARIF renders a report in SARIF v2.1.0 format for upload to GitHub code
// scanning and similar systems. Only the subset of SARIF used in practice
// for static analysis is emitted.
type SARIF struct {
	// ToolVersion is the kyber version string included in the SARIF tool
	// metadata; defaults to "dev" when empty.
	ToolVersion string
}

// NewSARIF constructs a SARIF reporter.
func NewSARIF(version string) *SARIF { return &SARIF{ToolVersion: version} }

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	ShortDescription sarifMultiformat  `json:"shortDescription"`
	FullDescription  sarifMultiformat  `json:"fullDescription"`
	DefaultConfig    sarifDefaultLevel `json:"defaultConfiguration"`
}

type sarifMultiformat struct {
	Text string `json:"text"`
}

type sarifDefaultLevel struct {
	Level string `json:"level"`
}

type sarifResult struct {
	RuleID    string           `json:"ruleId"`
	Level     string           `json:"level"`
	Message   sarifMultiformat `json:"message"`
	Locations []sarifLocation  `json:"locations"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn,omitempty"`
}

// Render writes the report to w as SARIF v2.1.0.
func (s SARIF) Render(w io.Writer, r *domain.Report) error {
	version := s.ToolVersion
	if version == "" {
		version = "dev"
	}
	rules := buildRules(r)
	results := buildResults(r)

	doc := sarifDocument{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "kyber",
				Version:        version,
				InformationURI: "https://github.com/jedi-knights/kyber",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func buildRules(r *domain.Report) []sarifRule {
	seen := make(map[string]sarifRule)
	for _, s := range r.Scores {
		if _, ok := seen[s.MetricID]; ok {
			continue
		}
		seen[s.MetricID] = sarifRule{
			ID:               s.MetricID,
			Name:             s.MetricID,
			ShortDescription: sarifMultiformat{Text: s.MetricID},
			FullDescription:  sarifMultiformat{Text: s.MetricID + " score per function"},
			DefaultConfig:    sarifDefaultLevel{Level: "warning"},
		}
	}
	out := make([]sarifRule, 0, len(seen))
	for _, rule := range seen {
		out = append(out, rule)
	}
	return out
}

func buildResults(r *domain.Report) []sarifResult {
	var out []sarifResult
	for _, s := range r.Scores {
		for _, f := range s.Findings {
			out = append(out, sarifResult{
				RuleID:  s.MetricID,
				Level:   sarifLevel(f.Severity),
				Message: sarifMultiformat{Text: f.Message},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{URI: s.Function.File},
						Region:           sarifRegion{StartLine: f.Line, StartColumn: f.Column},
					},
				}},
			})
		}
	}
	return out
}

func sarifLevel(sev domain.Severity) string {
	switch sev {
	case domain.SeverityInfo:
		return "note"
	case domain.SeverityError:
		return "error"
	default:
		return "warning"
	}
}
