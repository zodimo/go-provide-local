## 1. Correctness Tests (api_test.go)

- [x] 1.1 Create `plocal/api_test.go` with `package plocal` declaration and helper to measure registry chain depth
- [x] 1.2 Implement `TestUse_FallbackOnEmptyContext` — `Use()` on `context.Background()` returns key default
- [x] 1.3 Implement `TestUse_FallbackOnNonPlocal` — `Use()` on a `context.WithValue` context returns key default
- [x] 1.4 Implement `TestProvide_ReturnsInjectedValue` — single `Provide()` call, `Use()` returns injected value
- [x] 1.5 Implement `TestProvideAll_ReturnsAllInjectedValues` — multiple providers, all keys resolve correctly
- [x] 1.6 Implement `TestNesting_InheritanceFromParent` — inner scope reads key injected in outer scope
- [x] 1.7 Implement `TestNesting_ThreeLevels_ReadsGrandparent` — three-level nesting reads root value
- [x] 1.8 Implement `TestNesting_Shadowing` — inner scope overrides outer value; outer still sees its own
- [x] 1.9 Implement `TestSiblingScopes_AreIsolated` — two sibling `Provide` calls do not cross-contaminate
- [x] 1.10 Implement `TestProvideAll_CreatesExactlyOneNode` — verify registry depth increases by exactly 1 for N providers

## 2. Safety Tests (api_test.go)

- [x] 2.1 Implement `TestUse_NoPanic_BackgroundContext` — assert no panic via `defer/recover`
- [x] 2.2 Implement `TestUse_NoPanic_CancelledContext` — cancelled context does not cause panic
- [x] 2.3 Implement `TestUse_NoPanic_TimeoutContext` — timeout context does not cause panic
- [x] 2.4 Implement `TestKeyCollision_IndependentKeys_SameType` — two keys with same type and default are independent
- [x] 2.5 Implement `TestKeyCollision_BothInjected_ResolveIndependently` — both keys in same `ProvideAll`, each resolves to own value
- [x] 2.6 Implement `TestGoroutineSafety_ConcurrentUse` — N goroutines read same scoped context; run with `-race` flag

## 3. Performance Benchmarks (api_bench_test.go)

- [x] 3.1 Create `plocal/api_bench_test.go` with `package plocal` declaration and depth-builder helper
- [x] 3.2 Implement `BenchmarkUse_Depth1` with `b.ReportAllocs()` — expect 0 allocs/op
- [x] 3.3 Implement `BenchmarkUse_Depth10` with `b.ReportAllocs()` — expect 0 allocs/op
- [x] 3.4 Implement `BenchmarkUse_Depth100` with `b.ReportAllocs()` — expect 0 allocs/op
- [x] 3.5 Implement `BenchmarkProvideAll_Depth1_vs_Depth100` — verify same alloc count at both depths (proportional to providers, not depth)
- [x] 3.6 Implement `BenchmarkUse_DepthScaling` at depths 1, 10, 100 seeking root key — document linear scaling
- [x] 3.7 Implement `BenchmarkVsStdlib_Depth10` — compare plocal `Use()` ns/op against equivalent `context.Value()` traversal

## 4. Makefile

- [x] 4.1 Create `Makefile` at repo root with a `help` target that lists all targets
- [x] 4.2 Add `test` target: `go test ./plocal/...`
- [x] 4.3 Add `test-race` target: `go test -race ./plocal/...`
- [x] 4.4 Add `bench` target: `go test -bench=. -benchtime=5s ./plocal/...`
- [x] 4.5 Add `bench-mem` target: `go test -bench=. -benchmem -benchtime=5s ./plocal/...`
- [x] 4.6 Add `test-all` convenience target that runs `test-race` then `bench-mem`

## 5. Verification

- [x] 5.1 Run `make test` — all correctness and safety tests pass
- [x] 5.2 Run `make test-race` — no data race detected
- [x] 5.3 Run `make bench-mem` — review alloc counts and document baseline in a code comment
- [x] 5.4 Update `README.md` to reference the test suite and `make` targets as evidence for the library's claims
