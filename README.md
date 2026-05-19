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
$ kyber analyze ./internal/adapters/parser/...

internal/adapters/parser/goast.go
  New                cognitive=0  cyclomatic=1  difficulty=6.00     effort=259      funclen=1   halstead=43    maintainability=88   nesting=1  npath=1   readability=0.68    returns=1  testability=0.99
  GoAST.ParseFiles   cognitive=5  cyclomatic=5  difficulty=20.14 !  effort=14074 !  funclen=18  halstead=699  maintainability=51 !  nesting=3  npath=12  readability=0.31 !  returns=4  testability=0.53 !
  parseFiles         cognitive=7  cyclomatic=5  difficulty=20.57 !  effort=22624 !  funclen=20  halstead=1100 ! maintainability=50 ! nesting=3  npath=9   readability=0.38 !  returns=4  testability=0.39 !

[PACKAGE MEANS]
  internal/adapters/parser   cognitive=3.47   cyclomatic=3.53   maintainability=59.84   npath=5.40   readability=0.46   testability=0.58   ...   (15 fns)

[OVERALL]
  cognitive=2.17   cyclomatic=2.84   maintainability=67.13   npath=4.95   readability=0.52   testability=0.73   ...   (241 fns)

Functions: 241   Findings: 521   Files: 33   Time: 27ms
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
ID               NAME                    DEFAULT  DIRECTION        DESCRIPTION
cognitive        Cognitive Complexity    15       higher is worse  SonarSource Cognitive Complexity — control flow + nesting penalty.
cyclomatic       Cyclomatic Complexity   7        higher is worse  McCabe decision-point count.
difficulty       Halstead Difficulty     15       higher is worse  Halstead Difficulty — D = (n1/2) * (N2/n2).
effort           Halstead Effort         10000    higher is worse  Halstead Effort — E = D * V (Difficulty times Volume).
funclen          Function Length         40       higher is worse  Non-blank, non-comment line count of the function body.
halstead         Halstead Volume         1000     higher is worse  Halstead Volume — token counts weighted by vocabulary size.
maintainability  Maintainability Index   65       lower is worse   Microsoft Maintainability Index — composite of Volume, cyclomatic, and LOC.
nesting          Maximum Nesting Depth   4        higher is worse  Deepest block nesting level inside the function body.
npath            NPath Complexity        200      higher is worse  Nejmeh NPath — acyclic execution paths (multiplicative).
readability      Readability Score       0.6      lower is worse   Weighted 0–1 score from length, nesting, identifier length, comments.
returns          Return Statement Count  4        higher is worse  Number of return statements anywhere in the function body.
testability      Testability Score       0.6      lower is worse   Weighted 0–1 score from parameter count, side effects, interface params, length.
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
| `npath` | NPath complexity — number of acyclic execution paths (multiplicative) | `> 200` | higher is worse |
| `halstead` | Halstead Volume — token count weighted by vocabulary size | `> 1000` | higher is worse |
| `difficulty` | Halstead Difficulty — `(n1/2) · (N2/n2)` | `> 15` | higher is worse |
| `effort` | Halstead Effort — Difficulty × Volume | `> 10000` | higher is worse |
| `maintainability` | Microsoft Maintainability Index — composite of Volume, cyclomatic, and LOC | `< 65` | lower is worse |
| `nesting` | Maximum block nesting depth in the function body | `> 4` | higher is worse |
| `funclen` | Non-blank, non-comment line count of the function body | `> 40` | higher is worse |
| `returns` | Number of `return` statements anywhere in the function body | `> 4` | higher is worse |
| `readability` | Weighted 0–1 score from length, nesting, identifier length, and comment density | `< 0.6` | lower is worse |
| `testability` | Weighted 0–1 score from parameter count, side effects, interface parameters, and length | `< 0.6` | lower is worse |

All twelve are configurable per-project via `kyber.toml`. See [Configuration](#configuration).

To run only specific metrics, pass `--metric=<id>` (repeatable). To exclude metrics, pass `--disable=<id>` (also repeatable). The examples in each subsection below use `--metric` to keep the output focused on one column at a time.

### Cyclomatic Complexity

**Counts**: independent paths through a function — adds one for every `if`, `for`, `range`, non-default `case`, non-default `select` clause, and short-circuit boolean operator (`&&`, `||`).

**Formula**: `1 + decision_points`.

**How to read it**: 1 is a straight-line function; 5–7 is typical for branching logic; above 7 is a candidate for extraction. Cyclomatic alone misses *nesting* — a function with seven sequential `if` blocks scores the same as one with seven-deep nesting.

**Example**:

```bash
$ kyber analyze --metric=cyclomatic ./testdata/complex/

testdata/complex/complex.go
  Branchy   cyclomatic=12 !
```

**Reference**: McCabe, T. J. (1976). *A Complexity Measure*. IEEE Transactions on Software Engineering.

### Cognitive Complexity

**Counts**: control structures weighted by nesting depth, plus boolean-operator transitions. Each `if`/`for`/`switch`/`select` adds `1 + nesting_level`; `else` and `else if` each add 1 without the nesting penalty; sequences of like boolean operators (`&&` or `||`) add 1 per sequence with one more per operator change inside; labeled `break`/`continue`/`goto` each add 1; nested function literals increase the nesting level for their bodies.

**Formula**: walks the AST applying the rules above; no single closed-form expression.

**How to read it**: SonarQube's default per-function warning is 15. A function scoring close to its cyclomatic value has flat branching; a function scoring much higher than its cyclomatic (e.g. cyclomatic 4, cognitive 10) has deeply nested branching and is the primary type cognitive complexity is designed to catch.

**Example** — the `nested` fixture has four-deep nesting; cognitive is 10 (below the threshold but well above the same function's cyclomatic of 4):

```bash
$ kyber analyze --metric=cognitive ./testdata/nested/

testdata/nested/nested.go
  Nested   cognitive=10
```

**Reference**: Campbell, G. A. (2018). *Cognitive Complexity — A new way of measuring understandability*. [SonarSource white paper (PDF)](https://www.sonarsource.com/docs/CognitiveComplexity.pdf).

### NPath Complexity

**Counts**: number of acyclic execution paths through the function. Where cyclomatic adds decision points, NPath multiplies them — `if-else` contributes `paths(then) + paths(else)`, sequential statements multiply, loops contribute `paths(body) + 1`, switch contributes the sum of its cases.

**Formula**: recursive walk of the AST applying the rules above; no single closed-form expression. Logical operators (`&&`, `||`) inside conditions each add 1 path.

**How to read it**: 1 is a straight-line function; 8 is three sequential if-else blocks; values explode quickly past 200 (the standard yellow flag) because the count is multiplicative. A function with cyclomatic 8 but NPath 256 has the same number of decision points as a flat structure but stacks them — that's the case NPath is designed to catch.

**Example** — three sequential if-else blocks yield 2 × 2 × 2 = 8 paths:

```bash
$ kyber analyze --metric=npath ./testdata/npath_branchy/

testdata/npath_branchy/main.go
  Triple   npath=8
```

**Reference**: Nejmeh, B. A. (1988). *NPATH: A measure of execution path complexity and its applications*. Communications of the ACM, 31(2).

### Halstead Volume

**Counts**: every token in the function body, classified as an operator (keywords, punctuation, operators) or operand (identifiers, literals). Tracks `n1`, `n2` (unique operators/operands) and `N1`, `N2` (total counts).

**Formula**: `V = N × log₂(n)` where `N = N1 + N2` and `n = n1 + n2`.

**How to read it**: roughly proportional to source-code information content. Below ~200 is trivial; 200–1000 is typical for working code; above 1000 indicates either a function doing too much or a function with many distinct operators/identifiers (e.g. a long cobra command builder, which is unavoidable). Halstead Volume catches density that cyclomatic and cognitive both miss: a long straight-line function with no branches scores 1 on cyclomatic but can have very high Volume.

**Example**:

```bash
$ kyber analyze --metric=halstead ./testdata/complex/

testdata/complex/complex.go
  Branchy   halstead=670.20
```

**Reference**: Halstead, M. H. (1977). *Elements of Software Science*. Elsevier.

### Halstead Difficulty

**Counts**: same token classification as Volume.

**Formula**: `D = (n1 / 2) × (N2 / n2)`. High when there are many distinct operators against few distinct operands — the program manipulates a small data vocabulary in many ways.

**How to read it**: 15 is a reasonable yellow flag for a single function. Useful as a complement to Volume: two functions can have similar Volume but very different Difficulty depending on whether complexity comes from token sprawl (high V, lower D) or operator density (lower V, higher D).

**Example**:

```bash
$ kyber analyze --metric=difficulty ./testdata/complex/

testdata/complex/complex.go
  Branchy   difficulty=25 !
```

**Reference**: Halstead (1977), same source as Volume.

### Halstead Effort

**Counts**: same token classification as Volume.

**Formula**: `E = D × V`. Roughly proportional to total mental effort to write or understand the function.

**How to read it**: scales as the product of Difficulty and Volume, so values can be large (10⁴–10⁵ for working code). The standard yellow flag is 10000. Effort is the single most actionable Halstead measure when triaging — it captures both density (D) and total content (V) in one number.

**Example**:

```bash
$ kyber analyze --metric=effort ./testdata/complex/

testdata/complex/complex.go
  Branchy   effort=16754.89 !
```

**Reference**: Halstead (1977), same source as Volume.

### Maintainability Index

**Counts**: composite of Halstead Volume, cyclomatic complexity, and effective line count.

**Formula**: `MI = max(0, min(100, (171 − 5.2·ln(V) − 0.23·CC − 16.2·ln(LOC)) × 100/171))`. Normalized to a 0–100 scale; clamped at 0 and 100.

**How to read it**: this is the only metric where **higher is better**. Visual Studio's traffic-light convention: green ≥ 65, yellow 50–64, red < 50. A single function dropping below 65 isn't a crisis; a *package mean* below 65 is. MI is the most useful single-number summary because it composes three orthogonal signals.

**Example**:

```bash
$ kyber analyze --metric=maintainability ./testdata/complex/

testdata/complex/complex.go
  Branchy   maintainability=46.06 !
```

**Reference**: Coleman, D., Ash, D., Lowther, B., & Oman, P. (1994). *Using metrics to evaluate software system maintainability*. IEEE Computer, 27(8). Visual Studio's normalization variant is documented in the Visual Studio Code Metrics PowerTool.

### Maximum Nesting Depth

**Counts**: deepest level of nested `*ast.BlockStmt` inside the function body. Each `if`, `for`, `switch case`, or anonymous block adds one level.

**Formula**: walk all `BlockStmt` nodes; return the maximum nesting depth observed.

**How to read it**: depth 1 is the function body itself; 4 is the standard upper bound; depth 5+ is a strong refactoring signal. Often a leading indicator that cognitive complexity is also high.

**Example**:

```bash
$ kyber analyze --metric=nesting ./testdata/nested/

testdata/nested/nested.go
  Nested   nesting=5 !
```

### Function Length

**Counts**: source lines of the function body, excluding blank lines and lines whose first non-whitespace token is a `//` or `/*` comment.

**Formula**: count lines where the trimmed content is non-empty and does not start with a comment marker.

**How to read it**: 40 is the standard yellow flag and matches `rules/go-conventions.md`. Useful as a standalone gate because Readability bakes length into a composite — promoting it lets a project enforce length without dragging in identifier-length and comment-density signals.

**Example**:

```bash
$ kyber analyze --metric=funclen ./testdata/complex/

testdata/complex/complex.go
  Branchy   funclen=29
```

### Return Statement Count

**Counts**: number of `*ast.ReturnStmt` nodes anywhere in the function body. Returns inside nested function literals are not counted (they belong to the inner function).

**Formula**: AST walk with `ast.Inspect`, stopping at `*ast.FuncLit` boundaries.

**How to read it**: 4 is a reasonable yellow flag. Many early returns can be intentional (guard clauses, error handling) or accidental (a function trying to do too much) — the metric leaves the judgment to the threshold rather than baking a heuristic in.

**Example**:

```bash
$ kyber analyze --metric=returns ./testdata/multi_return/

testdata/multi_return/main.go
  Classify   returns=5 !
```

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

**Example**:

```bash
$ kyber analyze --metric=readability ./testdata/unreadable/

testdata/unreadable/deep.go
  Tangled   readability=0 !
```

### Testability Score

**Counts**: four 0–1 sub-signals, combined as a weighted average (all weights default to 1):

| Sub-signal | What it measures | Worst at |
|---|---|---|
| Parameters | parameter count vs. 5 | high parameter count |
| Side effects | observably-impure calls into `os`/`log`/`http`/`net`/`time`/`fmt` (pure `fmt.Sprintf`/`Errorf`/`Sprint`/`Append*`/`Sscan*` excluded) + reads of package globals, vs. 3 | many I/O calls or global reads |
| Interface params | fraction of parameters whose declared type is an interface | concrete dependencies dominate |
| Length | function lines vs. 40 | longer functions |

**Formula**: `(w_p·params + w_se·sideEffects + w_iface·interfaces + w_len·length) / sum(weights)`.

**How to read it**: 1.0 is ideal; below 0.6 flags. This metric is heuristic, not from published literature — closest published kin is Bruntink & van Deursen (2006), but that operates at class level and doesn't translate to Go functions. Pure `fmt` calls (`Sprintf`, `Errorf`, `Sprint`, `Sprintln`, `Append*`, `Sscan*`) are excluded from the side-effect count; only `Println`/`Printf`/`Fprint*` and other observably-impure functions are counted. Use this as a "watch the trend" signal rather than a strict gate.

**Example**:

```bash
$ kyber analyze --metric=testability ./testdata/untestable/

testdata/untestable/globals.go
  Dispatch   testability=0.20 !
```

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
