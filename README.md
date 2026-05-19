<div align="center">

# kyber

**A function-level Go code-quality analyzer — cyclomatic complexity, testability, readability, and more.**

[![CI](https://github.com/jedi-knights/kyber/actions/workflows/ci.yml/badge.svg)](https://github.com/jedi-knights/kyber/actions/workflows/ci.yml)
[![Release](https://github.com/jedi-knights/kyber/actions/workflows/release.yml/badge.svg)](https://github.com/jedi-knights/kyber/actions/workflows/release.yml)
[![GoReleaser](https://github.com/jedi-knights/kyber/actions/workflows/goreleaser.yml/badge.svg)](https://github.com/jedi-knights/kyber/actions/workflows/goreleaser.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedi-knights/kyber)](https://goreportcard.com/report/github.com/jedi-knights/kyber)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[Installation](#installation) · [Quickstart](#quickstart) · [Metrics](#metrics) · [Configuration](#configuration) · [GitHub Action](#github-action) · [Development](#development) · [Contributing](#contributing)

</div>

---

kyber walks a Go codebase, parses every function, and scores each one against a registered set of code-quality metrics. Output is human-readable text, machine-readable JSON, or SARIF v2.1.0 for GitHub code scanning.

```
$ kyber analyze ./...

internal/adapters/parser/goast.go
  parseDir          cognitive=3   cyclomatic=3   halstead=383.51   readability=0.47 !   testability=0.45 !
  extractInterfaces cognitive=3   cyclomatic=4   halstead=287.69   readability=0.36 !   testability=0.62

internal/domain/metrics/cyclomatic.go
  Analyze           cognitive=5   cyclomatic=5   halstead=540.41   readability=0.39 !   testability=0.35 !

[PACKAGE MEANS]
  internal/adapters/parser    cognitive=3.47   cyclomatic=3.53   halstead=500.74   readability=0.46   testability=0.56   (15 fns)
  internal/domain/metrics     cognitive=1.88   cyclomatic=2.81   halstead=306.28   readability=0.54   testability=0.73   (78 fns)

[OVERALL]
  cognitive=2.48   cyclomatic=3.10   halstead=394.77   readability=0.50   testability=0.66   (176 fns)

Functions: 176   Findings: 218   Files: 26   Time: 16ms
```

The architecture is built around a single `Metric` interface. Adding a new metric (nesting depth, parameter count, fan-out, etc.) is one new file in `internal/domain/metrics/` plus one registration line — nothing else needs to change.

## Why kyber?

Existing Go linters (`golangci-lint`, `gocyclo`, `revive`) enforce per-rule thresholds across an entire codebase. They tell you which lines violate a rule; they do not tell you which functions are healthy, mediocre, or risky overall. kyber works at the **function level**, producing per-function scores so you can:

- See at a glance which functions in a package carry the most quality risk
- Track readability and testability over time, not just complexity
- Plug a new metric into the same scoring pipeline without writing another linter

## Installation

### Go install (recommended for Go developers)

Requires Go 1.23 or later.

```bash
# Latest release
go install github.com/jedi-knights/kyber/cmd/kyber@latest

# Pinned to a specific version
go install github.com/jedi-knights/kyber/cmd/kyber@v0.1.0

# HEAD of main (unstable; pre-release work)
go install github.com/jedi-knights/kyber/cmd/kyber@main
```

`go install` places the binary in `$GOBIN` if set, otherwise in `$GOPATH/bin` (default: `$HOME/go/bin`). Make sure that directory is on your `PATH`:

```bash
# bash / zsh — append to ~/.bashrc or ~/.zshrc
export PATH="$(go env GOPATH)/bin:$PATH"
```

Verify the install:

```bash
$ kyber version
v0.1.0

$ kyber list-metrics
ID           NAME                   DEFAULT  DIRECTION        DESCRIPTION
cognitive    Cognitive Complexity   15       higher is worse  SonarSource Cognitive Complexity — control flow + nesting penalty.
cyclomatic   Cyclomatic Complexity  7        higher is worse  McCabe decision-point count.
halstead     Halstead Volume        1000     higher is worse  Halstead Volume — token counts weighted by vocabulary size.
readability  Readability Score      0.6      lower is worse   Weighted 0–1 score from length, nesting, identifier length, comments.
testability  Testability Score      0.6      lower is worse   Weighted 0–1 score from parameter count, side effects, interface params, length.
```

When you install a tagged release this way, `kyber version` reports the tag automatically — kyber reads its module version from the embedded build info.

### Homebrew

```bash
brew install jedi-knights/tap/kyber
```

### Pre-built binaries

Download from the [releases page](https://github.com/jedi-knights/kyber/releases) — Linux, macOS, and Windows binaries for amd64 and arm64 (Windows arm64 is not built).

### Docker

```bash
docker run --rm -v "$PWD:/src" -w /src ghcr.io/jedi-knights/kyber:latest analyze ./...
```

## Quickstart

```bash
# Analyze the current module recursively
kyber analyze ./...

# Show every registered metric and its default threshold
kyber list-metrics

# Fail the build if any function exceeds its threshold
kyber analyze --fail-on-threshold ./...

# Emit SARIF for GitHub code scanning
kyber analyze --format=sarif -o kyber.sarif ./...

# Limit to specific metrics
kyber analyze --metric=cyclomatic --metric=readability ./...
```

## Metrics

| ID | What it measures | Default threshold | Direction |
|---|---|---|---|
| `cyclomatic` | McCabe decision-point count — independent paths through a function | `> 7` | higher is worse |
| `cognitive` | SonarSource Cognitive Complexity — control flow weighted by nesting | `> 15` | higher is worse |
| `halstead` | Halstead Volume — token count weighted by vocabulary size | `> 1000` | higher is worse |
| `readability` | Weighted 0–1 score from length, nesting, identifier length, and comment density | `< 0.6` | lower is worse |
| `testability` | Weighted 0–1 score from parameter count, side effects, interface parameters, and length | `< 0.6` | lower is worse |

All five are configurable per-project via `kyber.toml`. See [Configuration](#configuration).

### Cyclomatic Complexity

**Counts**: independent paths through a function — adds one for every `if`, `for`, `range`, non-default `case`, non-default `select` clause, and short-circuit boolean operator (`&&`, `||`).

**Formula**: `1 + decision_points`.

**How to read it**: 1 is a straight-line function; 5–7 is typical for branching logic; above 7 is a candidate for extraction. Cyclomatic alone misses *nesting* — a function with seven sequential `if` blocks scores the same as one with seven-deep nesting.

**Reference**: McCabe, T. J. (1976). *A Complexity Measure*. IEEE Transactions on Software Engineering.

### Cognitive Complexity

**Counts**: control structures weighted by nesting depth, plus boolean-operator transitions. Each `if`/`for`/`switch`/`select` adds `1 + nesting_level`; `else` and `else if` each add 1 without the nesting penalty; sequences of like boolean operators (`&&` or `||`) add 1 per sequence with one more per operator change inside; labeled `break`/`continue`/`goto` each add 1; nested function literals increase the nesting level for their bodies.

**Formula**: walks the AST applying the rules above; no single closed-form expression.

**How to read it**: SonarQube's default per-function warning is 15. A function scoring close to its cyclomatic value has flat branching; a function scoring much higher than its cyclomatic (e.g. cyclomatic 4, cognitive 10) has deeply nested branching and is the primary type cognitive complexity is designed to catch.

**Reference**: Campbell, G. A. (2018). *Cognitive Complexity — A new way of measuring understandability*. [SonarSource white paper (PDF)](https://www.sonarsource.com/docs/CognitiveComplexity.pdf).

### Halstead Volume

**Counts**: every token in the function body, classified as an operator (keywords, punctuation, operators) or operand (identifiers, literals). Tracks `n1`, `n2` (unique operators/operands) and `N1`, `N2` (total counts).

**Formula**: `V = N × log₂(n)` where `N = N1 + N2` and `n = n1 + n2`.

**How to read it**: roughly proportional to source-code information content. Below ~200 is trivial; 200–1000 is typical for working code; above 1000 indicates either a function doing too much or a function with many distinct operators/identifiers (e.g. a long cobra command builder, which is unavoidable). Halstead Volume catches density that cyclomatic and cognitive both miss: a long straight-line function with no branches scores 1 on cyclomatic but can have very high Volume.

**Reference**: Halstead, M. H. (1977). *Elements of Software Science*. Elsevier.

### Readability Score

**Counts**: four 0–1 sub-signals, combined as a weighted average (all weights default to 1):

| Sub-signal | What it measures | Worst at |
|---|---|---|
| Length | function lines vs. 40 | longer functions |
| Nesting | max block depth vs. 4 | deeper nesting |
| Identifier length | median identifier length, with `i`/`j`/`k`/`_`/`ok`/`err` excluded | median below 5 chars |
| Comment density | comment-line ratio, capped at 20% | no comments |

**Formula**: `(w_len·length + w_nest·nesting + w_ident·idents + w_comment·comments) / sum(weights)`.

**How to read it**: 1.0 is ideal; below 0.6 flags. The score is a proxy, not a ground truth — short utility functions can score low simply because they lack comments and use single-letter loop variables. Trust the trend across a package more than any one function's value. (Real academic readability metrics like [Buse-Weimer (2010)](https://web.eecs.umich.edu/~weimerw/p/weimer-tse2010-readability-preprint.pdf) train weights against human ratings; kyber's weights are hand-picked.)

### Testability Score

**Counts**: four 0–1 sub-signals, combined as a weighted average (all weights default to 1):

| Sub-signal | What it measures | Worst at |
|---|---|---|
| Parameters | parameter count vs. 5 | high parameter count |
| Side effects | calls into `os`/`log`/`http`/`net`/`time`/`fmt` + reads of package globals, vs. 3 | many I/O calls or global reads |
| Interface params | fraction of parameters whose declared type is an interface | concrete dependencies dominate |
| Length | function lines vs. 40 | longer functions |

**Formula**: `(w_p·params + w_se·sideEffects + w_iface·interfaces + w_len·length) / sum(weights)`.

**How to read it**: 1.0 is ideal; below 0.6 flags. This metric is heuristic, not from published literature — closest published kin is Bruntink & van Deursen (2006), but that operates at class level and doesn't translate to Go functions. The "side effect" detection treats every `fmt` call as observable I/O, including pure `fmt.Sprintf`/`fmt.Errorf` — a known false positive. Use this as a "watch the trend" signal rather than a strict gate.

## Reading the report

### Per-function rows

Each row shows the function name and one `metric=value` cell per registered metric. A `!` marker after a value means the score crossed the metric's threshold:

```
parseDir          cognitive=3   cyclomatic=3   halstead=383.51   readability=0.47 !   testability=0.45 !
```

Findings escalate to **Error** severity at ≥ 2× threshold; in text output the marker is the same `!`, but JSON and SARIF distinguish `warning` vs `error`.

### `[PACKAGE MEANS]` and `[OVERALL]`

After the per-function detail, kyber prints the mean value of each metric per package and across the whole report. These aggregates always print — they exist to make module-level patterns visible without piping through `jq`.

```
[PACKAGE MEANS]
  internal/adapters/parser    cognitive=3.47   cyclomatic=3.53   halstead=500.74   readability=0.46   testability=0.56   (15 fns)
```

The trailing `(N fns)` is the number of unique functions in that package. The JSON output exposes the same aggregates plus `min`/`max`/`count` in an `aggregates` block.

### Adding a new metric

1. Implement `domain.Metric` in `internal/domain/metrics/<name>.go`
2. Register it in `internal/domain/metrics/all.go`
3. Add a test in `<name>_test.go` (and a `testdata/<fixture>/` package if needed)

That's it.

## Configuration

kyber reads `kyber.toml` from the current directory by default. Precedence (highest first):

1. CLI flags (e.g. `--threshold-cyclomatic=10`)
2. Environment variables (`KYBER_FORMAT`, `KYBER_FAIL_ON_THRESHOLD`, `KYBER_VERBOSE`, `KYBER_PATHS`)
3. `kyber.toml`
4. Built-in defaults

See `kyber.toml.example` in the repo for an annotated reference.

## GitHub Action

```yaml
- uses: jedi-knights/kyber@v1
  with:
    paths: "./..."
    format: sarif
    fail-on-threshold: "true"
```

When `format: sarif`, the action uploads the report to the GitHub code-scanning view automatically.

## Development

```bash
git clone https://github.com/jedi-knights/kyber.git
cd kyber
go mod download
go test -race -count=1 ./...
git config core.hooksPath .githooks    # enable pre-push linting
```

Useful Makefile targets: `make build`, `make test`, `make lint`, `make install`, `make run`, `make clean`.

## Contributing

Bug reports and PRs are welcome. Run `make test` and `make lint` locally before pushing. Conventional Commits (`feat`, `fix`, `refactor`, ...) drive the release pipeline — non-conformant commits are rejected by CI.

## License

[MIT](LICENSE) © Jedi Knights
