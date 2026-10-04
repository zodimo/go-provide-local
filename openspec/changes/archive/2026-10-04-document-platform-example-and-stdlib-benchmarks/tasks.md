## 1. Fix the platform example documentation

- [x] 1.1 Rewrite `examples/platform/doc.go` package comment: remove all references to `WithUUIDProvider` and `NewUUID`; describe the seams that exist (`WithTimeProvider`/`Now`, `WithTaskRunner`/`RunTasks`)
- [x] 1.2 In the same package comment, state the provenance (`google.golang.org/adk/v2`) and that the context seam is backed by `plocal` rather than `context.WithValue`
- [x] 1.3 Grep the package for any other phantom symbol references and fix them (e.g. the `WithUUIDProvider` mention in `examples/platform/exec.go`)
- [x] 1.4 Verify every `With...`/exported symbol named in `examples/platform` docs resolves to a declared symbol

## 2. Add stdlib comparison benchmarks

- [x] 2.1 In `plocal/api_bench_test.go`, add a stdlib chain depth parameter to the existing comparison helpers so any depth can be built
- [x] 2.2 Add `BenchmarkVsStdlib_plocal_Depth1` and `BenchmarkVsStdlib_stdlib_Depth1`
- [x] 2.3 Add `BenchmarkVsStdlib_plocal_Depth100` and `BenchmarkVsStdlib_stdlib_Depth100`
- [x] 2.4 Rename/keep the depth-10 pair so the naming convention `BenchmarkVsStdlib_<impl>_Depth<N>` is consistent across depths 1/10/100
- [x] 2.5 Add a scope-creation comparison (`ProvideAll` vs stdlib `context.WithValue`) at depth 1
- [x] 2.6 Ensure each stdlib benchmark builds a chain of the same depth as its plocal counterpart with the sought key at the root, and calls `b.ReportAllocs()`

## 3. Regenerate and record measured baselines

- [x] 3.1 Run `make bench-mem` and capture the ns/op, B/op, and allocs/op for every benchmark
- [x] 3.2 Replace the stale "Measured baseline" comment block in `plocal/api_bench_test.go` with the freshly measured values, keeping the machine/Go-version note
- [x] 3.3 Replace the benchmark table in `README.md` with the fresh values, adding a plocal-vs-stdlib row pair for each scenario (Use depths 1/10/100 and scope creation)
- [x] 3.4 Rewrite the README "Key findings" bullets to state the measured trade-off honestly: stdlib faster on single-key lookup, plocal faster on type safety, zero-alloc reads, and O(providers) scope creation
- [x] 3.5 Confirm the README table and the in-code baseline comments agree with each other and with the run output

## 4. Document `WithProvider` / `WithProviders`

- [x] 4.1 Add a "Context-Returning Injection" section to `plocal/doc.go` describing `WithProvider` and `WithProviders` and how they differ from `Provide`/`ProvideAll`
- [x] 4.2 Add `WithProvider[T any](ctx context.Context, key *ResourceKey[T], val T) context.Context` to the README API reference
- [x] 4.3 Add `WithProviders(ctx context.Context, providers []Provider) context.Context` to the README API reference
- [x] 4.4 Add a README note explaining closure-scoped injection (`Provide`/`ProvideAll`) vs derived-context injection (`WithProvider`/`WithProviders`), including the scope-lifetime difference

## 5. Document the platform example in the README

- [x] 5.1 Add a README section presenting `examples/platform` as an adoption proof: an ADK-v2 platform seam reimplemented on `plocal.WithProvider`
- [x] 5.2 Show both seams (`WithTimeProvider`/`Now` and `WithTaskRunner`/`RunTasks`) with a short usage snippet
- [x] 5.3 Link the section to the `examples/platform` directory

## 6. Verify

- [x] 6.1 Run `go build ./...` and `go vet ./...` — both clean
- [x] 6.2 Run `go test ./...` and `make test-race` — all pass
- [x] 6.3 Run `make bench-mem` one more time and confirm every documented number still matches within normal variance
- [x] 6.4 Run `openspec validate document-platform-example-and-stdlib-benchmarks` and resolve any validation errors
- [x] 6.5 Re-read `examples/platform/doc.go`, `README.md`, and `plocal/doc.go` and confirm no documented symbol is missing from the code
