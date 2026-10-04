## Why

The benchmark suite measures the wrong axis, and the harness it measures with
does not build the structure it claims. Both were found by running the suite on
the author's own machine (Intel Core Ultra 9 185H, Go 1.27.0, linux/amd64,
GOMAXPROCS=22).

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
single `map[any]any` probe — not a 100-deep traversal. Every depth benchmark,
and the README text built on it, describes a structure the helper never built.

**Gap 2 — with the harness fixed, depth is disclosed as a plocal *loss*.**
Once `buildChain` nests one node per level (verified: registry depth == `depth`
for 1/2/4/10/100, and a never-present key forces full traversal), plocal `Use()`
is linear in depth and stdlib `context.Value()` is cheaper at **every** depth:

| Depth | plocal `Use()` | stdlib `ctx.Value()` |
|---|---|---|
| 1 | 37 ns | **8 ns** |
| 10 | 256 ns | **54 ns** |
| 100 | 2554 ns | **448 ns** |

plocal walks its whole prototype chain with a (lock-guarded) `map[any]any` probe
per node; stdlib does a pointer compare + type switch per node. Depth is not an
axis plocal wins, and the docs must say so instead of implying otherwise.

**Gap 3 — the real axis (injection width) was unmeasured.** The library's actual
claim is that N values for a scope go into **one node**, whereas stdlib needs
**N context levels**. The consequence is visible in reads: a key injected
alongside N-1 other values reads in ~37 ns **regardless of N** when the values
are batched into one node, but grows linearly when spread across N scopes:

| Values (N) | batched: 1 node | spread: N scopes |
|---|---|---|
| 1 | 37 ns | 37 ns |
| 4 | 37 ns | 112 ns |
| 8 | 38 ns | 213 ns |
| 16 | 37 ns | 429 ns |
| 32 | **37 ns** | **859 ns** |

And the cost of *building* those N values (N=32):

| Strategy | ns/op | B/op | allocs/op | levels |
|---|---|---|---|---|
| `WithProviders(ctx, 32)` | 2079 | 2488 | **6** | **1** |
| 32× `WithProviders` (spread) | 9519 | 13824 | 128 | 32 |
| 32× `UpdateProvider` (upsert) | 2896 | 512 | 32 | 1 |
| 32× `context.WithValue` | **1418** | **1536** | 32 | 32 |

Three honest findings: batching keeps reads flat in N (one node, ~37 ns
forever); batching beats spreading within plocal by ~5x in time and ~20x in
allocs; and stdlib's **raw** injection ns/bytes remain lower even at N=32 —
plocal's win is the **shape** (one level, bounded allocs, flat reads), not raw
speed.

**Gap 4 — the optimization that enabled the fast path introduced two defects.**
The single-value `Provide` had been moved onto `context.WithValue(c, key, val)`
with a `c.Value(key)` fast path in `Use()`, and `WithProvider`/`UpdateProvider`
were made to mutate an existing node in place. Both changes are defects:

1. **Data race.** Upserting into a node that was already handed out mutates a
   shared `map[any]any`. Concurrent `Use(base, k)` + `WithProvider(base, …)`
   races under `-race` (and concurrent map read/write can panic), violating the
   documented concurrency guarantee of `Use()`.
2. **Broken shadowing.** Because `Provide` stored its value as a direct stdlib
   key, `Use()`'s fast path (checked first) made that value permanently shadow
   any later `ProvideAll`/`WithProviders` override of the same key. No ordering
   of the two lookups fixes both directions: a direct key and a registry node
   live in two context chains with no relative-position information.

Both defects are fixed in production code as part of this change: a
`sync.RWMutex` guards each node, `Provide` goes back onto the registry, and
`Use()` walks the registry only (no fast path). All injection orders then shadow
correctly ("nearest scope wins").

**Constraint:** absolute ns/op is machine- and thermal-dependent and is
advisory. The load-bearing evidence is (a) alloc counts and level counts, which
are stable, (b) the flat-vs-linear *shape* of reads as N grows, and (c) ratios
measured within a single run.

## What Changes

- **Rewrite `buildChain`** so it actually nests: `depth` sequential scope
  creations, one node per level, with the sought key at the outer root. Fix its
  doc comment and delete the false `// Build the chain iteratively...` comment.
- **Fix the concurrency defect:** add a `sync.RWMutex` to `registryNode`,
  locking reads in `Use()` and upsert writes in `WithProvider` /
  `UpdateProvider` / `UpdateProviders`.
- **Fix the shadowing defect:** put `Provide` back on the registry (one node per
  call) and drop the `c.Value(key)` fast path from `Use()`, so all injection
  paths compose into correct "nearest scope wins" shadowing.
- **Reframe the story around injection width** (the library's real axis): N
  values in one node keep reads flat; N scopes make them linear. Depth is
  retained as an honest disclosure, not a headline.
- **Re-measure every benchmark** and regenerate both the `api_bench_test.go`
  baseline comment block and the `README.md` results tables from the same run.
- **Add a width/read-after-injection pair** (`Read_Batched` vs `Read_Spread`) and
  an **injection-cost sweep** (`WithProviders(N)` vs N× `WithProviders` vs N×
  `UpdateProvider` vs N× `context.WithValue`) for N ∈ {1, 4, 8, 16, 32}.
- **Add correctness tests**: scope-override semantics (inner shadows outer,
  outer intact) for both the closure and derived-context APIs; upsert semantics
  for `UpdateProvider`/`UpdateProviders`; and a concurrent upsert-vs-read race
  test.
- **Correct the `performance-benchmarks` spec**: the "linear in depth (flat for
  plocal)" and unconditional-faster requirements are replaced with truthful ones
  (depth is linear for plocal and stdlib is cheaper at every depth; the width
  axis is where plocal wins; assert allocs/levels/shape, not fixed ns).

## Capabilities

### Modified Capabilities

- `performance-benchmarks`: state the depth requirement truthfully (plocal is
  linear in depth; stdlib is faster raw at every depth), add the width-axis
  requirement (one node per scope; flat reads as N grows; batch beats spread),
  add the injection-cost requirement, and add the scope-override requirement.

## Impact

- **Production code:** `plocal/api.go`, `plocal/models.go` — mutex on
  `registryNode`; `Use()` registry-only walk; `Provide` back on the registry;
  documented `UpdateProvider`/`UpdateProviders`.
- **Tests:** `plocal/api_bench_test.go` — rewrite `buildChain`, correct baseline
  comments, add width/read-after-injection/injection-cost benchmarks and harness
  guards. `plocal/api_test.go` — scope-override, upsert, and race tests.
- **Docs:** `README.md` — regenerate results tables; rewrite the pitch, Problem/
  Solution prose, API reference, and Key findings to the corrected width thesis.
- **Specs:** `openspec/specs/performance-benchmarks/spec.md` (via delta).
- **No new dependencies.** The comparison uses only `context`; the guard uses
  `sync`.
- **Supersedes a claim** in the archived change
  `document-platform-example-and-stdlib-benchmarks`, which marked the
  performance spec complete on a depth-based narrative.
