## Why

`go-provide-local` makes strong claims about type safety, zero-copy performance, and collision-proof key isolation — but has no tests to verify any of them. As a new library, the true developer experience is unknown, and without a compliance test suite, regressions could silently invalidate the library's core guarantees.

## What Changes

- Add `plocal/api_test.go`: correctness and safety tests covering all documented behaviours
- Add `plocal/api_bench_test.go`: benchmark tests that prove the performance claims with measurable alloc counts and timing
- Add `Makefile`: developer-facing targets for running tests, race detection, and benchmarks without needing to remember flags

## Capabilities

### New Capabilities

- `correctness-tests`: Tests that verify functional correctness — fallback values, value injection, scope inheritance, scope shadowing, sibling isolation, and multi-provider node collapsing
- `safety-tests`: Tests that verify safety guarantees — no panics on empty/non-plocal contexts, key collision-proofing across packages, and goroutine read safety
- `performance-benchmarks`: Benchmarks that prove the zero-copy and fast-lane traversal claims — `Use()` allocs, `ProvideAll()` allocs, depth-scaling behaviour, and direct comparison against `context.WithValue`

### Modified Capabilities

## Impact

- New test files only — no changes to `api.go` or `models.go`
- No new dependencies (uses standard `testing` package only)
- `Makefile` provides `test`, `test-race`, `bench`, and `bench-mem` targets as the canonical way to run the suite
- Benchmark results will serve as a baseline for future performance regression detection
