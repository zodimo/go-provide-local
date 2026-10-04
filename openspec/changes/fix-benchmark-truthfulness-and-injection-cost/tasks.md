# Tasks

## 1. Fix the harness

- [ ] 1.1 Rewrite `buildChain` in `plocal/api_bench_test.go` to nest `depth` scopes, one node per level, with the sought key injected at the root (worst-case read)
- [ ] 1.2 Fix the `buildChain` doc comment (it claims "chain of the given depth" — make that true) and delete/fix the false `// Build the chain iteratively...` comment
- [ ] 1.3 Verify the corrected chain has registry depth == `depth` (temporary assertion or a debug test) before trusting any number

## 2. Fix the depth benchmarks and re-measure

- [ ] 2.1 Re-run `BenchmarkUse_Depth1/10/100` and `BenchmarkUse_DepthScaling_*` against the corrected chain
- [ ] 2.2 Confirm flat vs linear against `BenchmarkVsStdlib_stdlib_Depth*`; record the crossover depth in a range (e.g. ~4–8)
- [ ] 2.3 Replace the committed baseline comment block (lines ~36–74) with freshly measured values from one run

## 3. Add the injection-cost sweep (the real axis)

- [ ] 3.1 Add `BenchmarkInject_WithProviders_N` for N ∈ {1, 2, 4, 8, 16, 32}
- [ ] 3.2 Add `BenchmarkInject_WithProviderSeq_N` for the same N (sequential `WithProvider`)
- [ ] 3.3 Add `BenchmarkInject_StdlibWithValueSeq_N` for the same N (sequential `context.WithValue`)
- [ ] 3.4 Report ns/op, B/op, allocs/op; assert allocs and level count, not ns

## 4. Add lookup-after-injection pairing

- [ ] 4.1 Add `BenchmarkLookupAfterInject_plocal_N` and `_stdlib_N` (worst case: first-injected key) for N ∈ {1, 4, 8, 16, 32}
- [ ] 4.2 Confirm and document the crossover (stdlib wins N ≤ ~4, plocal wins above)

## 5. Add the scope-override test

- [ ] 5.1 Add a correctness test in `plocal/api_test.go`: inner `Provide`/`ProvideAll` override leaves the outer scope's value intact
- [ ] 5.2 Add a test for the context-returning API: `WithProviders` override is local; the parent context still reads the original

## 6. Correct the narrative

- [ ] 6.1 Rewrite `README.md` "Benchmark Results" tables from the single fresh run
- [ ] 6.2 Invert the "Key findings": lead with flat O(1)-in-depth lookup; state crossover; state where stdlib wins (N ≤ ~4, raw ns/bytes at injection)
- [ ] 6.3 Fix the "The Problem"/"The Solution" prose so "massive performance penalty of deeply nested stdlib nodes" is backed by the corrected (flat-vs-linear) data, not the phantom ramp
- [ ] 6.4 Add the `WithProviders` vs N×`WithProvider` finding as a measured recommendation

## 7. Patch the spec and verify

- [ ] 7.1 Apply the `performance-benchmarks` delta; ensure the old linear-scaling and unconditional-faster requirements are replaced
- [ ] 7.2 Confirm no remaining requirement asserts a fixed ns/op threshold
- [ ] 7.3 Run `make test`, `make test-race`, `make bench-mem`; confirm the README, comments, and spec all match one run
- [ ] 7.4 Run `openspec validate fix-benchmark-truthfulness-and-injection-cost --strict`
