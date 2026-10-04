## Context

The repository is a two-package Go module:

- `plocal` — the library. Public surface: `NewResourceKey`, `Value`, `Provide`,
  `ProvideAll`, `Use`, plus the recently added `WithProvider` and `WithProviders`.
- `examples/platform` — a lightly adapted copy of `google.golang.org/adk/v2`'s
  platform package. It exposes two seams over `plocal`: `WithTimeProvider`/`Now`
  (time) and `WithTaskRunner`/`RunTasks` (task fan-out).

The change is triggered by two gaps discovered while auditing the working tree:

**Gap 1 — undocumented capability.** `WithProvider`/`WithProviders` and the whole
`examples/platform` package appear in no doc. `examples/platform/doc.go` even
references `WithUUIDProvider` and `NewUUID`, symbols that were dropped in the
adaptation. The docs describe a package that does not exist.

**Gap 2 — untrustworthy benchmarks.** `plocal/api_bench_test.go` carries a
"measured baseline" comment block, and `README.md` mirrors it in a results table.
A fresh `make bench-mem` on the author's machine (Intel Core Ultra 9 185H,
Go 1.26.2, linux/amd64) produces:

| Scenario | Documented | Actual |
|---|---|---|
| `Use()` depth 1 | 11 ns/op | 24 ns/op |
| `Use()` depth 10 | 56 ns/op | 126 ns/op |
| `Use()` depth 100 | 524 ns/op | 1223 ns/op |
| `ProvideAll()` depth 1 | 209 ns/op | 339 ns/op |
| `ProvideAll()` depth 100 | 197 ns/op | 358 ns/op |
| plocal `Use()` depth 10 | 58 ns/op | 139 ns/op |
| stdlib `ctx.Value()` depth 10 | 29 ns/op | 61 ns/op |

Every figure is roughly 2x low, and only one scenario (depth 10) has a stdlib
comparison at all.

**Constraint:** the machine's clock is throttled or otherwise differs from the
original run, so absolute ns/op is not portable. The design must therefore treat
*relative* comparisons (same machine, same run) as the load-bearing evidence and
absolute numbers as advisory.

## Goals / Non-Goals

**Goals:**

- Make the `examples/platform` package and the `WithProvider`/`WithProviders` API
  discoverable from `README.md` and `plocal/doc.go`.
- Eliminate the phantom `WithUUIDProvider`/`NewUUID` references so every documented
  symbol resolves to real code.
- Give every benchmark scenario a stdlib counterpart, so plocal-vs-stdlib can be
  read at a glance at depths 1, 10, and 100 plus scope creation.
- Replace stale baselines with measured numbers and make the specs match reality.
- Correct the false spec requirement that claims an unconditional plocal win.

**Non-Goals:**

- No production code changes to `plocal/api.go` or `plocal/models.go`.
- No new dependencies (the stdlib comparison uses only `context`).
- No attempt to make `plocal` outperform stdlib on single-key lookup — that is not
  the library's value proposition.
- No CI wiring for benchmarks; this change improves what is measured, not when.
- No added UUID seam — the adapted package deliberately omits it.

## Decisions

### Decision 1: Fix `doc.go` by describing seams that exist, not by adding a UUID seam

`examples/platform/doc.go` names `WithUUIDProvider` and `NewUUID`. Two ways to
resolve: (a) implement the UUID seam so the docs become true, or (b) rewrite the
docs to describe `WithTimeProvider`/`Now` and `WithTaskRunner`/`RunTasks`.

**Chosen: (b).** The adaptation intentionally dropped UUID generation; re-adding it
would pull in a UUID dependency and expand the example's surface for no
documentation benefit. Documentation should follow the code here, not the reverse.
An alternative considered was deleting `doc.go`'s second paragraph entirely, but
that would lose the useful explanation of why providers ride on the context; a
rewrite preserves the rationale while making it accurate.

### Decision 2: Document `WithProvider`/`WithProviders` as a distinct injection style

The two new functions differ from `Provide`/`ProvideAll` in a way that matters and
that users will get wrong: `Provide`/`ProvideAll` take a consumer closure and
auto-release the scope when it returns, while `WithProvider`/`WithProviders` return
a derived `context.Context` whose lifetime is the caller's responsibility. Both
must be documented, and the README must state the difference explicitly rather than
listing the functions as synonyms.

### Decision 3: Pair benchmarks by scenario, not by a single depth-10 token comparison

The comparison is extended to every scenario: `Use()` at depths 1, 10, and 100, and
`ProvideAll()` scope creation at depth 1. Each pair uses an identically shaped
chain with the sought key at the root, so the two benchmarks measure equal
traversal distance. Naming convention: `BenchmarkVsStdlib_<impl>_Depth<N>`.

Rationale: a single depth-10 data point invited the false generalization that
already made it into the spec. A per-depth table makes the depth-dependence — and
the fact that the gap *widens* with depth — visible.

### Decision 4: Report the trade-off honestly instead of asserting a win

The measured result is that stdlib is faster at single-key lookup (depth 10: ~61 ns
vs ~139 ns). The prior spec asserted the opposite. The requirement is rewritten as
a MODIFIED requirement stating the real trade-off: stdlib wins raw lookup speed,
plocal wins type safety, zero-alloc reads, and O(providers) scope creation. This
keeps the spec testable — a reader can verify each claim against a run — and stops
the suite from documenting a claim it cannot pass.

### Decision 5: Treat absolute ns/op as advisory, relative comparison as evidence

Because absolute numbers vary with machine and thermal state, the design leans on
ratios within a single run. The README table will carry the measured absolute
values plus a machine note; the spec's verification scenarios assert *relationships*
(plocal allocs == 0 at every depth; ProvideAll allocs equal at depth 1 and 100;
both implementations benchmarked at the same depth) rather than fixed ns/op
thresholds.

## Risks / Trade-offs

- **Absolute numbers drift between runs and machines** → State the measurement
  context in the README; assert relative relationships in the spec, not fixed
  ns/op. Document that baselines must be regenerated with `make bench-mem`.
- **A benchmark passing on one machine may not on another** → The allocs/op
  scenarios (0 allocs for `Use`; equal allocs for `ProvideAll` at depth 1 and 100)
  are the stable assertions; ns/op scenarios carry tolerance wording.
- **Documentation can drift from code again** → The new
  `platform-integration-example` spec makes "documented symbol must exist" an
  explicit, checkable requirement, so a future drift is a spec violation rather
  than a silent bug.
- **Duplicating numbers in comments and README invites divergence** → A spec
  scenario requires both to match a fresh run; reviewers regenerate with
  `make bench-mem` rather than trusting either copy.
- **Example code is a vendored ADK copy and may diverge upstream** → The README
  and `doc.go` state the provenance (`google.golang.org/adk/v2`) and the nature of
  the adaptation, so the fork relationship is documented.
