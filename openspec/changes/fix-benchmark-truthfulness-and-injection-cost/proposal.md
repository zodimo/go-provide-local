## Why

The benchmark suite measures the wrong axis, and its committed numbers do not
reproduce. Both problems were found by running the suite on the author's own
machine (Intel Core Ultra 9 185H, Go 1.27.0, linux/amd64, GOMAXPROCS=22).

**Gap 1 — `buildChain` does not build a chain.** The helper claims to create "a
registry chain of the given depth" but calls `WithProviders` exactly once:

```go
providers := []Provider{Value(rootKey, "root-value")}
for i := 1; i < depth; i++ {
    providers = append(providers, Value(fillerKey, i))   // all in ONE node
}
return WithProviders(context.Background(), providers), rootKey
```

One `WithProviders` call creates **one** node holding N values. So
`BenchmarkUse_Depth100` traverses a linked list of length 1 and measures a
single `map[any]any` probe — not a 100-deep traversal. The committed baseline
(26 → 132 → 1213 ns/op) is therefore not a measurement of plocal at all.

**Gap 2 — the committed numbers are a stale artifact.** A fresh `make bench-mem`
reports plocal `Use()` at **~26 ns/op at every depth** (depth 1/10/100 →
27.6 / 25.8 / 26.4), stable across `-count=3` (24.9–27.6 ns). The committed
block claims 26 / 132 / 1213. The stdlib column, by contrast, reproduces
faithfully (8 / 53 / 441 → 7.6 / 54.3 / 439.7). The suite fixed the stdlib half
of the comparison and then certified a plocal half that was never measuring a
chain.

**Gap 3 — the resulting narrative is inverted.** Because the false ramp made
plocal look O(depth) and stdlib look cheaper, `README.md` concluded:

> "stdlib `context.Value()` is faster at every depth (~3x at depth 1, ~2.5x at
> depth 10, ~2.7x at depth 100)"

Measured at depth 100: plocal **26 ns** vs stdlib **440 ns** — plocal is ~17x
faster, not 2.7x slower. `Use()` is flat in depth because it walks only its own
nodes; `context.Value()` is linear because it walks every node. The library's
one genuinely strong result is being documented as a loss.

**Gap 4 — the real performance axis is unmeasured.** The library's actual claim
is about **injection shape**, not lookup depth: it stores N values for a scope in
**one flat node**, where stdlib needs **N context levels**. The existing
`ScopeCreation` benchmark only tests N=1, the one case where the stdlib approach
is cheaper. Measured for N=32:

| Strategy | ns/op | B/op | allocs/op | context levels |
|---|---|---|---|---|
| `WithProviders(ctx, 32 provs)` | 2049 | 2456 | **6** | **1** |
| 32× `WithProvider` | 9955 | 13312 | 160 | 32 |
| 32× `context.WithValue` | **1452** | **1536** | 32 | 32 |

Two honest findings fall out: batching via `WithProviders` is ~5x faster and
~27x fewer allocs than the sequential `WithProvider` loop; and plain
`context.WithValue` is cheaper in *raw ns and bytes* than plocal even at N=32 —
plocal's advantage is **alloc count** (6 vs 32) and, above all, the **flat
lookup shape** it buys. Naming the axis honestly means naming that trade.

**Constraint:** absolute ns/op is machine- and thermal-dependent and is
advisory. The load-bearing evidence is (a) alloc counts, which are stable, and
(b) ratios measured within a single run.

## What Changes

- **Rewrite `buildChain`** so it actually nests: `depth` sequential scope
  creations, one node per level, with the sought key at the root. Fix its doc
  comment and the false `// Build the chain iteratively...` comment.
- **Re-measure every benchmark** and regenerate both the `api_bench_test.go`
  baseline comment block and the `README.md` results tables from the same run.
- **Add an injection-cost sweep**: `WithProviders(N)` vs N× `WithProvider` vs
  N× `context.WithValue` for N ∈ {1, 2, 4, 8, 16, 32}, reporting ns/op, B/op,
  allocs/op, and resulting context depth.
- **Add a lookup-after-injection pair**: worst-case read of the first-injected
  key after injecting N values with each strategy, so the flat-vs-linear
  consequence of the injection shape is visible (crossover at N≈4).
- **Add a scope-override demonstration**: a test proving an inner scope
  overrides a key locally without mutating the outer scope — the "local
  override, not permanent replace" semantic — contrasted with the stdlib
  equivalent that requires re-wrapping to scope.
- **Invert and correct the narrative** in `README.md`: lead with flat
  O(1)-in-depth lookup vs stdlib's O(depth), state the crossover (N≈4), and
  state the N=1 and raw-ns cases where stdlib wins.
- **Correct the `performance-benchmarks` spec**: the requirement "`Use()` lookup
  time scales linearly with depth (O(depth))" is false for plocal (it is flat;
  linearity describes stdlib) and must be rewritten. The requirement "plocal `Use`
  is faster than stdlib for equivalent depth" must become conditional on depth.

## Capabilities

### Modified Capabilities

- `performance-benchmarks`: correct the depth-scaling requirement (plocal is
  depth-*independent*, not linear), make the stdlib comparison conditional on
  depth and state the crossover, and add requirements covering the injection-cost
  comparison, the allocation-per-scope claim at N>1, and the scope-override
  semantic.

## Impact

- **Tests:** `plocal/api_bench_test.go` — rewrite `buildChain`, correct baseline
  comments, add injection sweep, lookup-after-injection, and scope-override
  benchmarks.
- **Docs:** `README.md` — regenerate results tables; correct the "Key findings"
  and the "Performance"/"Problem" narrative to match measurement.
- **Specs:** `openspec/specs/performance-benchmarks/spec.md` (via delta).
- **No production code changes.** `plocal/api.go` and `plocal/models.go` are
  correct; their behavior was mis-measured, not wrong.
- **No new dependencies.** The comparison uses only `context`.
- **Supersedes a claim** in the archived change
  `document-platform-example-and-stdlib-benchmarks`, which marked the
  performance spec complete on numbers this change shows to be artifacts.
