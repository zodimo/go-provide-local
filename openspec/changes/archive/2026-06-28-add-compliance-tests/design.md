## Context

`go-provide-local` is a new Go library implementing a Lexically Scoped Registry Tree on top of `context.Context`. It makes three categories of documented claims — correctness, safety, and performance — none of which are currently verified by tests. The library has two source files (`api.go`, `models.go`, ~88 lines total) and a standard `go.mod`.

## Goals / Non-Goals

**Goals:**
- Prove every documented library claim with a corresponding test or benchmark
- Establish alloc-count baselines for `Use()` and `ProvideAll()` to catch future regressions
- Keep tests in the same package (`plocal`) using only the standard `testing` library
- Make tests readable as living documentation of the library's guarantees

**Non-Goals:**
- Integration tests with external systems
- Fuzzing or property-based testing (future work)
- Modifying `api.go` or `models.go` to make tests pass

## Decisions

### Decision: Two test files, not one

Split into `api_test.go` (correctness + safety) and `api_bench_test.go` (benchmarks).

**Rationale:** Benchmarks have different run semantics (`go test -bench=.`) and mixing them with correctness tests clutters both. Separation also makes the intent of each file self-documenting.

**Alternative considered:** Single `api_test.go` — rejected because benchmark noise obscures test output and vice versa.

---

### Decision: White-box testing (same package, not `plocal_test`)

Tests live in `package plocal`, not `package plocal_test`.

**Rationale:** The key safety claim — that the `map[any]any` type assertion `val.(T)` cannot panic in practice — requires inspecting the `registryNode` structure. White-box access lets us write a targeted test that verifies the internal invariant, not just the public API surface.

**Alternative considered:** Black-box `plocal_test` package — partially used for collision-proof key tests (which must simulate two separate packages), but insufficient for internal invariant checks.

---

### Decision: Key collision test uses two synthetic "package" variables

The collision-proof claim requires demonstrating that two `*ResourceKey[string]` values created independently (simulating two different packages) do not interfere.

**Rationale:** In real usage, keys are package-level `var` declarations. The pointer address is the map key. Two distinct `NewResourceKey` calls produce distinct pointers, even with identical types and defaults. The test simply creates two keys and proves they resolve independently.

---

### Decision: Benchmark uses `b.ReportAllocs()` + `runtime.ReadMemStats` for zero-copy verification

**Rationale:** `b.ReportAllocs()` reports allocations per operation measured by the testing harness. A `Use()` call on an already-set-up scope should show `0 allocs/op` — this is the primary machine-verifiable proof of the "zero-copy" claim. `ProvideAll()` will show allocations (the node + map), but these should be bounded to exactly the providers being injected.

---

### Decision: Depth-scaling benchmark at 1, 10, 100 levels

**Rationale:** The O(depth) claim for `Use()` is verifiable by showing that benchmark time scales linearly with depth. Three data points (1, 10, 100) are sufficient to demonstrate the trend without excessive test runtime.

## Risks / Trade-offs

- **Benchmark flakiness** → Run with `-benchtime=5s` and `-count=3` in CI to smooth variance. Document the expected alloc count in comments, not as assertions (alloc assertions are fragile across Go versions).
- **White-box tests coupling to internals** → Acceptable: the `registryNode` struct is the library's core data structure and is unlikely to change shape without intentional redesign.
- **The `val.(T)` assertion** → Cannot be tested for panic without triggering undefined behaviour intentionally. Instead, we document the invariant (only `providerImpl[T]` can write to the map via `apply()`) and test that `Use()` never panics under all normal and edge-case inputs.
