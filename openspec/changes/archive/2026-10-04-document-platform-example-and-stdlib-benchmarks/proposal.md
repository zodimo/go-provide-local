## Why

Two recent additions to the repository are invisible to users and to the compliance
suite:

1. **The `examples/platform` package and the new `plocal.WithProvider` /
   `plocal.WithProviders` API are undocumented.** `examples/platform` is a lightly
   adapted copy of `google.golang.org/adk/v2`'s platform package that replaces
   `context.WithValue` with `plocal.WithProvider`. It is the project's most
   convincing real-world adoption proof, yet neither `README.md` nor
   `plocal/doc.go` mentions it. A concrete integration bug also shipped: the new
   `examples/platform/doc.go` references `WithUUIDProvider` and `NewUUID`, which
   do not exist in the adapted package.

2. **Benchmark comparisons against the standard library are incomplete and
   stale.** Only the depth-10 `Use()` scenario has a stdlib counterpart; depth 1,
   depth 100, and scope creation have none. Worse, the committed baseline numbers
   in `api_bench_test.go` and the README table are wrong by roughly 2x: a fresh
   `make bench-mem` run on this machine reports 24/126/1223 ns for depth 1/10/100,
   while the docs claim 11/56/524 ns. A benchmark suite that documents numbers the
   code does not reproduce cannot serve its purpose.

## What Changes

- Document the `examples/platform` package in `README.md` as a worked
  integration example (time seam and task-runner seam, ADK-v2 provenance,
  `context.WithValue` → `plocal` adaptation).
- Document the new `WithProvider` / `WithProviders` API in both `README.md`'s API
  reference and `plocal/doc.go`.
- Fix `examples/platform/doc.go`: remove references to the non-existent
  `WithUUIDProvider` and `NewUUID`, and describe the seams that actually exist
  (`WithTimeProvider`/`Now`, `WithTaskRunner`/`RunTasks`).
- Add a stdlib comparison benchmark to **every** scenario in
  `plocal/api_bench_test.go`: `Use()` at depths 1, 10, and 100, and `ProvideAll()`
  scope creation at depths 1 and 100.
- Replace the stale committed baseline numbers (in `api_bench_test.go` comments
  and the `README.md` results table) with freshly measured values, and present a
  plocal-vs-stdlib row pair per scenario.
- Correct and extend the `performance-benchmarks` spec so its requirements match
  measured behavior. The current requirement "plocal Use is faster than stdlib
  context.WithValue traversal for equivalent depth" is **factually false** at
  depth 10 (139 ns vs 61 ns) and must be reworded to describe the real trade-off.

## Capabilities

### New Capabilities

- `platform-integration-example`: Documentation contract for the
  `examples/platform` package, the `WithProvider`/`WithProviders` API, and the
  requirement that documented seams match the code.

### Modified Capabilities

- `performance-benchmarks`: Add a requirement that every benchmark scenario has a
  paired stdlib comparison, and correct the false "plocal is faster than stdlib"
  requirement to a measured, honest trade-off statement.

## Impact

- **Docs:** `README.md`, `plocal/doc.go`, `examples/platform/doc.go`.
- **Tests:** `plocal/api_bench_test.go` (new benchmark functions + corrected
  baseline comments).
- **Specs:** `openspec/specs/performance-benchmarks/spec.md` (via delta),
  new `openspec/specs/platform-integration-example/spec.md`.
- **No production code changes.** `plocal/api.go` and `plocal/models.go` are
  already implemented; this change is documentation, benchmarks, and specs only.
- **No new dependencies.** The stdlib comparison uses only `context`.
