# metrics

Concrete implementations of the `domain.Metric` interface. Each metric is one
file; adding a new one is a single new file plus a registration line in
`all.go` — nothing else needs to change.

## Adding a metric

1. Create `<id>.go` — implement `domain.Metric` (ID, Name, Description,
   DefaultThreshold, HigherIsWorse, Analyze).
2. Add `New<Name>()` to `all.go`.
3. Add a `<id>_test.go` with at least one table-driven test covering the
   threshold boundary.
4. Add a `testdata/<id>_fixture/` package if the metric needs a real Go source
   file to parse.

## Severity escalation

Every metric applies the same rule: a finding is `Warning` when the value
crosses the threshold; it escalates to `Error` when the value reaches or
exceeds **2×** the threshold. The threshold is taken from `MetricOptions`
(user config / CLI flag) and falls back to `DefaultThreshold()` when zero.

---

## Metric reference

### cyclomatic — Cyclomatic Complexity

| | |
|---|---|
| **ID** | `cyclomatic` |
| **Default threshold** | `> 7` |
| **Direction** | higher is worse |

**Formula:** `CC = 1 + decision_points`

A decision point is any `if`, `for`, `range`, non-default `case` or `select`
clause, and each short-circuit boolean operator (`&&`, `||`). The baseline of
1 represents the single straight-line path through the function.

**Why it matters:** McCabe showed that cyclomatic complexity predicts defect
density. Functions above 7 are statistically harder to test exhaustively —
full branch coverage requires at least one test case per path.

**Limitation:** cyclomatic counts independent paths but not *nesting depth*.
Seven flat `if` blocks and seven-deep nested `if` blocks score identically.
Use `cognitive` or `nesting` alongside `cyclomatic` to catch the nested case.

**References**
- McCabe, T. J. (1976). *A Complexity Measure*. IEEE Transactions on Software Engineering, SE-2(4).
  [Wikipedia overview](https://en.wikipedia.org/wiki/Cyclomatic_complexity)

---

### cognitive — Cognitive Complexity

| | |
|---|---|
| **ID** | `cognitive` |
| **Default threshold** | `> 15` |
| **Direction** | higher is worse |

**Formula:** recursive AST walk applying SonarSource increment rules — no
single closed-form expression.

**Rules (summarised):**
- Each `if`, `for`, `switch`, `select` adds `1 + current_nesting_level`.
- `else` and `else if` each add 1 (no nesting bonus).
- A sequence of identical boolean operators (`&&` or `||`) adds 1 per
  sequence; each *change* of operator inside the sequence adds 1 more.
- Labeled `break`, `continue`, and `goto` each add 1.
- Nested function literals increase the nesting counter for their bodies.

**Why it matters:** cyclomatic gives equal weight to flat and deeply nested
branching. Cognitive complexity penalises nesting, matching human intuition
that `if { if { if { } } }` is harder to follow than three sequential `if`
blocks with the same cyclomatic score.

**References**
- Campbell, G. A. (2018). *Cognitive Complexity — A new way of measuring understandability*.
  SonarSource white paper. [PDF](https://www.sonarsource.com/docs/CognitiveComplexity.pdf)

---

### npath — NPath Complexity

| | |
|---|---|
| **ID** | `npath` |
| **Default threshold** | `> 200` |
| **Direction** | higher is worse |

**Formula:** recursive AST walk applying Nejmeh's multiplication rules — no
single closed-form expression.

**Rules (summarised):**
- `if-else`: `paths(then) + paths(else)`.
- Sequential statements: multiply.
- `for`/`range`: `paths(body) + 1`.
- `switch`: sum of case-path counts.
- Each `&&` or `||` inside a condition adds 1.

**Why it matters:** cyclomatic counts decision points additively; NPath
multiplies them. A function with three sequential `if-else` blocks has
cyclomatic 4 but NPath 8. Seven blocks: cyclomatic 8, NPath 128. NPath
surfaces the combinatorial explosion that cyclomatic misses, catching
functions whose test matrix is implausibly large even if no single nesting
chain is deep.

**References**
- Nejmeh, B. A. (1988). *NPATH: A measure of execution path complexity and its applications*.
  Communications of the ACM, 31(2).
  [ACM DL](https://dl.acm.org/doi/10.1145/42372.42379)

---

### halstead — Halstead Volume

| | |
|---|---|
| **ID** | `halstead` |
| **Default threshold** | `> 1000` |
| **Direction** | higher is worse |

**Formula:** `V = N × log₂(n)`

- `n1` = unique operators, `n2` = unique operands, `n = n1 + n2` (vocabulary)
- `N1` = total operators, `N2` = total operands, `N = N1 + N2` (length)

**Why it matters:** Volume is roughly proportional to the information content
of the function. A long straight-line function with no branches scores 1 on
cyclomatic but can have very high Volume — the case the Halstead family catches
and cyclomatic misses entirely.

**References**
- Halstead, M. H. (1977). *Elements of Software Science*. Elsevier.
  [Wikipedia overview](https://en.wikipedia.org/wiki/Halstead_complexity_measures)

---

### difficulty — Halstead Difficulty

| | |
|---|---|
| **ID** | `difficulty` |
| **Default threshold** | `> 15` |
| **Direction** | higher is worse |

**Formula:** `D = (n1 / 2) × (N2 / n2)`

Uses the same token counts as Volume. High when there are many distinct
operators against few distinct operands — the program manipulates a small data
vocabulary in many ways.

**Why it matters:** two functions can have similar Volume but very different
Difficulty. High D with moderate V signals operator density; high V with
moderate D signals token sprawl. Together they triangulate where complexity
actually lives.

**References**
- Halstead, M. H. (1977). *Elements of Software Science*. Elsevier.
  [Wikipedia overview](https://en.wikipedia.org/wiki/Halstead_complexity_measures)

---

### effort — Halstead Effort

| | |
|---|---|
| **ID** | `effort` |
| **Default threshold** | `> 10000` |
| **Direction** | higher is worse |

**Formula:** `E = D × V`

Product of Difficulty and Volume. Halstead proposed it as a proxy for the
total mental effort required to write or comprehend a program.

**Why it matters:** Effort captures both density (D) and total content (V) in
one number. When triaging a package, Effort is the single most actionable
Halstead measure — a function with high E warrants attention regardless of
whether the cause is operator density or sheer length.

**References**
- Halstead, M. H. (1977). *Elements of Software Science*. Elsevier.
  [Wikipedia overview](https://en.wikipedia.org/wiki/Halstead_complexity_measures)

---

### maintainability — Maintainability Index

| | |
|---|---|
| **ID** | `maintainability` |
| **Default threshold** | `< 65` |
| **Direction** | lower is worse |

**Formula (Visual Studio / Coleman 1994 variant):**

```
raw = 171 - 5.2·ln(V) - 0.23·CC - 16.2·ln(LOC)
MI  = max(0, min(100, raw × 100 / 171))
```

- `V` = Halstead Volume, `CC` = cyclomatic complexity, `LOC` = effective line count.
- Clamped to 0–100; the `100/171` normalisation is Visual Studio's addition to
  the original 1994 formula.

**Why it matters:** MI is the only metric where higher is better. It composes
three orthogonal signals (information density, branching, length) into a single
0–100 score with a well-established traffic-light convention: green ≥ 65,
yellow 50–64, red < 50. A single function dipping below 65 is unremarkable; a
*package mean* below 65 warrants structural attention.

**References**
- Coleman, D., Ash, D., Lowther, B., & Oman, P. (1994). *Using metrics to evaluate software system maintainability*.
  IEEE Computer, 27(8).
- Visual Studio normalization:
  [Microsoft Docs — Maintainability Index](https://learn.microsoft.com/en-us/visualstudio/code-quality/code-metrics-maintainability-index-range-and-meaning)

---

### nesting — Maximum Nesting Depth

| | |
|---|---|
| **ID** | `nesting` |
| **Default threshold** | `> 4` |
| **Direction** | higher is worse |

**Formula:** walk all `*ast.BlockStmt` nodes; return the maximum depth observed.

Depth 1 is the function body itself. Each `if`, `for`, `switch case`,
anonymous block, or closure body adds one level.

**Why it matters:** deep nesting correlates strongly with high cognitive
complexity but is simpler to explain and gate. It is also a sub-signal of the
`readability` score; exposing it independently lets a project enforce a nesting
ceiling without pulling in the full readability composite.

**References**
- ESLint [max-depth rule](https://eslint.org/docs/latest/rules/max-depth) —
  depth 4 is the widely-cited default across languages and linters.

---

### funclen — Function Length

| | |
|---|---|
| **ID** | `funclen` |
| **Default threshold** | `> 40` |
| **Direction** | higher is worse |

**Formula:** count of `SourceLines` that are non-blank and whose first
non-whitespace token is not a `//` or `/*` comment.

The "effective" line count excludes blank lines and comment-only lines to
avoid penalising well-documented functions and to focus the signal on lines
a reviewer actually reads.

**Why it matters:** long functions are the root cause of most other quality
issues — they tend to have higher cyclomatic complexity, deeper nesting, and
lower readability. Function length is the bluntest metric but also the most
universally understood; it is often the right gate to add to CI first.

**References**
- Fowler, M. (2018). *Function Length*. martinfowler.com.
  [Article](https://martinfowler.com/bliki/FunctionLength.html)

---

### returns — Return Statement Count

| | |
|---|---|
| **ID** | `returns` |
| **Default threshold** | `> 4` |
| **Direction** | higher is worse |

**Formula:** count of `*ast.ReturnStmt` nodes reachable from `FuncDecl.Body`,
stopping at `*ast.FuncLit` boundaries (returns inside nested closures belong to
the closure, not the enclosing function).

**Why it matters:** many early returns can be intentional (guard clauses,
error handling) or accidental (a function trying to do too much). The metric
leaves the judgment to the configured threshold rather than baking a heuristic
in. High return counts often accompany high cyclomatic complexity; when a
function scores high on both, it is a strong refactoring signal.

**References**
- Dijkstra, E. W. (1970). *Notes on Structured Programming* (EWD249).
  The debate over multiple exits traces to his structured-programming arguments.
  [EWD249 transcript](https://www.cs.utexas.edu/~EWD/transcriptions/EWD02xx/EWD249/EWD249.html)

---

### readability — Readability Score

| | |
|---|---|
| **ID** | `readability` |
| **Default threshold** | `< 0.6` |
| **Direction** | lower is worse |

**Formula:** `score = (w_len·L + w_nest·N + w_ident·I + w_comment·C) / Σw`

All weights default to 1. Each sub-signal is normalised to 0–1:

| Sub-signal | Worst at | Cap |
|---|---|---|
| `L` — length | long functions | `max_lines` (default 40) |
| `N` — nesting | deep blocks | `max_nesting` (default 4) |
| `I` — identifier length | median ident ≤ 5 chars (excluding `i`, `j`, `k`, `_`, `ok`, `err`) | — |
| `C` — comment density | no comments | 20 % of lines |

**Why it matters:** the score is a proxy, not a ground truth. Short utility
functions can score low because they lack comments and use single-letter loop
variables. Trust the trend across a package more than any individual value.

**References**
- Buse, R. P. L., & Weimer, W. R. (2010). *A metric for software readability*.
  IEEE Transactions on Software Engineering.
  [Preprint PDF](https://web.eecs.umich.edu/~weimerw/p/weimer-tse2010-readability-preprint.pdf)

---

### testability — Testability Score

| | |
|---|---|
| **ID** | `testability` |
| **Default threshold** | `< 0.6` |
| **Direction** | lower is worse |

**Formula:** `score = (w_p·P + w_se·S + w_iface·F + w_len·L) / Σw`

All weights default to 1. Each sub-signal is normalised to 0–1:

| Sub-signal | Worst at | Cap |
|---|---|---|
| `P` — parameter count | many params | `max_params` (default 5) |
| `S` — side effects | calls into `os`/`log`/`http`/`net`/`time`/`fmt` (excluding pure `fmt` calls) + package-global reads | 3 |
| `F` — interface params | concrete-type params dominate | — |
| `L` — length | long functions | `max_lines` (default 40) |

Pure `fmt` calls (`Sprintf`, `Errorf`, `Sprint`, `Sprintln`, `Append*`,
`Sscan*`) are excluded from `S` because they are observably pure. Only calls
that perform I/O or produce side effects visible outside the function count.

**Why it matters:** a function that depends on concrete types, calls I/O
packages, and reads globals is hard to test in isolation without a real
environment. Testability flags these patterns and nudges toward dependency
injection and interface-based parameters.

**References**
- Bruntink, M., & van Deursen, A. (2006). *An empirical study into class testability*.
  Journal of Systems and Software, 79(9).
  [ResearchGate](https://www.researchgate.net/publication/220378137_An_empirical_study_into_class_testability)
  — closest published kin; kyber's metric is a Go function-level adaptation of
  the concepts, not a direct implementation.
