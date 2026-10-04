# Tasks

## 1. Fix the harness

- [x] 1.1 Rewrite `buildChain` in `plocal/api_bench_test.go` to nest `depth` scopes, one node per level, with the sought key injected at the outermost node (worst-case read)
- [x] 1.2 Fix the `buildChain` doc comment (it claims "chain of the given depth" — make that true) and delete the false `// Build the chain iteratively...` comment
- [x] 1.3 Add `TestBuildChain_RegistryDepth` asserting registry depth == `depth` for 1/2/4/10/100

## 2. Fix the two defects introduced by the fast path

- [x] 2.1 Add a `sync.RWMutex` to `registryNode` (`models.go`) with `get`/`has`/`put` accessors
- [x] 2.2 Lock reads in `Use()` (`RLock`) and upsert writes (`Lock`) in `WithProvider`, `UpdateProvider`, `UpdateProviders`
- [x] 2.3 Put `Provide` back on the registry (equivalent to `ProvideAll` with one provider)
- [x] 2.4 Drop the `c.Value(key)` fast path from `Use()`; walk the registry only, so all injection orders shadow correctly
- [x] 2.5 Document `UpdateProvider`/`UpdateProviders` and the `WithProvider` upsert/merge semantics

## 3. Re-measure the depth axis and correct the baseline

- [x] 3.1 Re-run `BenchmarkUse_Depth1/10/100` and `BenchmarkUse_DepthScaling_*` against the corrected chain
- [x] 3.2 Confirm plocal is linear in depth and stdlib is cheaper at every depth; record the depth table
- [x] 3.3 Replace the committed baseline comment block with freshly measured values from one run

## 4. Add the width axis (the real story)

- [x] 4.1 Add `BenchmarkRead_Batched_N*` and `BenchmarkRead_Spread_N*` for N ∈ {1, 4, 8, 16, 32} (worst-case, outermost key)
- [x] 4.2 Add `BenchmarkInject_Batched_N*`, `BenchmarkInject_SeqPlocal_N*`, `BenchmarkInject_SeqStdlib_N*`, `BenchmarkInject_Upsert_N*`
- [x] 4.3 Report ns/op, B/op, allocs/op, and level counts; assert shape (flat vs linear), not fixed ns
- [x] 4.4 Add `TestReadShapes_LevelCounts` guard (batched = 1 level; spread = N levels)

## 5. Add correctness tests

- [x] 5.1 Scope-override tests: inner `Provide`/`ProvideAll` leaves the outer scope intact; `WithProviders` override is local
- [x] 5.2 `WithProvider` override of an existing key scopes to a new node; new key merges without a level
- [x] 5.3 `UpdateProvider`/`UpdateProviders` upsert tests (in-place, no new level; and fallback when no node exists)
- [x] 5.4 Concurrent upsert-vs-`Use` race test (passes under `-race`)

## 6. Correct the narrative

- [x] 6.1 Rewrite `README.md` "Benchmark Results" tables from the single fresh run
- [x] 6.2 Lead with width (flat reads as N grows); state plainly that stdlib wins raw lookup at every depth and raw injection ns/bytes
- [x] 6.3 Fix the intro/"Problem"/"Solution" prose so nothing implies a depth performance win
- [x] 6.4 Document `UpdateProvider`/`UpdateProviders` and the `WithProvider` merge behavior in the API reference

## 7. Patch the spec and verify

- [x] 7.1 Apply the `performance-benchmarks` delta; replace the false linear-scaling and unconditional-faster requirements
- [x] 7.2 Confirm no remaining requirement asserts a fixed ns/op threshold
- [x] 7.3 Run `make test`, `make test-race`, `make bench-mem`; confirm the README, comments, and spec all match one run
- [x] 7.4 Run `openspec validate fix-benchmark-truthfulness-and-injection-cost --strict`
