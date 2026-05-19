// Package config loads kyber settings from TOML, environment variables, and
// caller-provided overrides. Precedence: CLI flags > env > TOML > defaults.
// CLI flag application is the caller's responsibility (cmd/kyber/commands);
// this package owns the TOML+env+defaults layer.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the resolved configuration for one kyber run.
type Config struct {
	Paths           []string                `toml:"paths"`
	Exclude         []string                `toml:"exclude"`
	Format          string                  `toml:"format"`
	FailOnThreshold bool                    `toml:"fail_on_threshold"`
	Verbose         bool                    `toml:"verbose"`
	Metrics         map[string]MetricConfig `toml:"metrics"`
}

// MetricConfig is the TOML representation of one metric's settings. Enabled
// defaults to true (a metric appearing in [metrics.<id>] without an explicit
// enabled key is considered on); Threshold of 0 means "use the metric's
// DefaultThreshold". Params holds extra knobs (max_params, weight_*, ...).
type MetricConfig struct {
	Enabled   *bool   `toml:"enabled"`
	Threshold float64 `toml:"threshold"`

	// All other keys are captured into Params via the catch-all toml unmarshal
	// step (see Load). Not a struct field on its own.
	Params map[string]any `toml:"-"`
}

// Default returns a Config with all built-in defaults populated.
func Default() Config {
	return Config{
		Paths:           []string{"./..."},
		Exclude:         []string{"vendor/**", "testdata/**"},
		Format:          "text",
		FailOnThreshold: false,
		Verbose:         false,
		Metrics:         make(map[string]MetricConfig),
	}
}

// Load reads path (when non-empty) as TOML, overlays environment variables,
// and returns the resolved Config. Missing files are NOT errors when path is
// the empty string; missing files at an explicit path return an error.
func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		_, err := os.Stat(path)
		if err == nil {
			if err := loadTOML(path, &cfg); err != nil {
				return cfg, err
			}
		} else if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("stat config %s: %w", path, err)
		}
	}

	applyEnv(&cfg)
	return cfg, nil
}

func loadTOML(path string, cfg *Config) error {
	// First decode into the structured shape.
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	// Then decode into a generic map to capture the per-metric param keys
	// (TOML's struct unmarshaler drops keys not declared on the target).
	var raw map[string]any
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return fmt.Errorf("parsing %s (raw): %w", path, err)
	}
	rawMetrics, ok := raw["metrics"].(map[string]any)
	if !ok {
		return nil
	}
	if cfg.Metrics == nil {
		cfg.Metrics = make(map[string]MetricConfig)
	}
	for id, sub := range rawMetrics {
		if subMap, ok := sub.(map[string]any); ok {
			cfg.Metrics[id] = mergeMetricParams(cfg.Metrics[id], subMap)
		}
	}
	return nil
}

func mergeMetricParams(mc MetricConfig, subMap map[string]any) MetricConfig {
	mc.Params = make(map[string]any)
	for k, v := range subMap {
		if k == "enabled" || k == "threshold" {
			continue
		}
		mc.Params[k] = v
	}
	return mc
}

// applyEnv overlays KYBER_* environment variables onto cfg. Missing or
// unparseable values leave cfg untouched.
func applyEnv(cfg *Config) {
	if v := os.Getenv("KYBER_FORMAT"); v != "" {
		cfg.Format = v
	}
	if v := os.Getenv("KYBER_VERBOSE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Verbose = b
		}
	}
	if v := os.Getenv("KYBER_FAIL_ON_THRESHOLD"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.FailOnThreshold = b
		}
	}
	if v := os.Getenv("KYBER_PATHS"); v != "" {
		cfg.Paths = splitAndTrim(v)
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// IsEnabled reports whether the metric with the given ID is enabled by the
// resolved config. Metrics with no explicit [metrics.<id>] table default to
// on (the spirit of "no config means everything on"). An explicit
// `enabled = false` disables.
func (c Config) IsEnabled(id string) bool {
	mc, ok := c.Metrics[id]
	if !ok {
		return true
	}
	if mc.Enabled == nil {
		return true
	}
	return *mc.Enabled
}

// EnabledIDs returns the IDs from c.Metrics whose Enabled is not explicitly
// false, sorted by key. When the user has not configured metrics at all the
// returned slice is nil, which Registry.Enabled treats as "all".
func (c Config) EnabledIDs() []string {
	if len(c.Metrics) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Metrics))
	for id := range c.Metrics {
		if c.IsEnabled(id) {
			out = append(out, id)
		}
	}
	return out
}
