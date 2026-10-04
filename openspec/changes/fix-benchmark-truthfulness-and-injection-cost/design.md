## Context

`plocal/api.go` is correct. `Use()` walks the Prototype Chain of
`registryNode`s the library owns; `context.WithValue` wraps one node per key.
The benchmarks mis-measured that structure, and the docs inherited the error.

All numbers below are from this machine: Intel Core Ultra 9 185H, Go 1.27.0,
linux/amd64, GOMAXPROCS=22, `-benchmem`, `-benchtime=1–2s`. They are advisory in
absolute terms; ratios within a run and alloc counts are the evidence.

### Measured: the committed depth ramp is an artifact

| Benchmark | Committed docs | Fresh run | Stable? |
|---|---|---|---|
| `Use` depth 1 | 26 ns | 27.6 ns | ✅ |
| `Use` depth 10 | 132 ns | 25.8 ns | ✅ |
| `Use` depth 100 | 1213 ns | 26.4 ns | ✅ (`-count=3`: 24.9–27.6) |
| stdlib `Value` depth 1 | 8 ns | 7.6 ns | ✅ |
| stdlib `Value` depth 10 | 53 ns | 54.3 ns | ✅ |
| stdlib `Value` depth 100 | 441 ns | 439.7 ns | ✅ |

plocal is **flat** at ~26 ns; the committed 26/132/1213 is a phantom from
`buildChain` creating one node regardless of `depth`.

### Measured: injection cost and shape (the real axis)

N=32, injecting N values into one scope:

| Strategy | ns/op | B/op | allocs/op | context levels |
|---|---|---|---|---|
| `WithProviders(ctx, n)` | 2049 | 2456 | **6** | **1** |
| N× `WithProvider` | 9955 | 13312 | 160 | 32 |
| N× `context.WithValue` | **1452** | **1536** | 32 | 32 |

Full sweep (ns/op | allocs/op):

| N | `WithProviders` | N× `WithProvider` | N× `context.WithValue` |
|---|---|---|---|
| 1 | 309 \| 4 | 301 \| 5 | 43 \| 1 |
| 2 | 355 \| 4 | 679 \| 10 | 86 \| 2 |
| 4 | 407 \| 4 | 1336 \| 20 | 168 \| 4 |
| 8 | 535 \| 4 | 3016 \| 40 | 344 \| 8 |
| 16 | 1242 \| 6 | 5121 \| 80 | 683 \| 16 |
| 32 | 2049 \| 6 | 9955 \| 160 | 1452 \| 32 |

### Measured: lookup after injection (worst case — first-injected key)

| N | plocal `Use` | stdlib `Value` |
|---|---|---|
| 1 | 27.5 ns | **6.1 ns** |
| 4 | 29.4 ns | 24.6 ns (crossover) |
| 8 | 30.6 ns | 43.9 ns |
| 16 | 29.5 ns | 79.5 ns |
| 32 | **26.3 ns** | 164.3 ns |

plocal is flat; stdlib is linear; crossover at N≈4.

## Goals / Non-Goals

**Goals:**

- Make `buildChain` measure what its name and comment claim.
- Replace every committed number with a freshly measured one, and make the file
  and the README agree with a single run.
- Measure the library's actual axis: injection shape (levels and allocs vs N keys).
- State the trade honestly, including where stdlib wins.
- Correct the false spec requirements so the suite stops asserting things it
  cannot pass.

**Non-Goals:**

- No production code changes to `api.go` / `models.go`.
- No new dependencies.
- No CI benchmark gating; this change fixes what is measured, not when.
- No attempt to make plocal beat stdlib on shallow single-key lookup — measured
  crossover is N≈4 and that is the honest boundary.
- No commitment to specific ns/op thresholds in the spec; assert structure
  (allocs, flatness, ordering) not timing.

## Decisions

### Decision 1: Fix the harness before touching a single number

Rewriting the baseline numbers without fixing `buildChain` would enshrine a new
set of phantoms. Order is: fix the helper, then re-measure, then update docs and
spec. The corrected `buildChain` nests `depth` times:

```
for i := 0; i < depth; i++ { ctx = <one-node scope> }   // depth nodes, root first
```

with the sought key injected at the root so a read is worst-case traversal.

### Decision 2: Make injection shape a first-class measured axis

The comparison that matters is not "lookup at depth N" (a symptom) but "how many
context levels and allocations does injecting N values cost, and what does that
do to reads" (the cause). The sweep reports levels and allocs, because those are
the stable, machine-independent facts; ns/op is advisory.

### Decision 3: Report the `WithProviders` vs N×`WithProvider` gap explicitly

Measured: ~5x faster and ~27x fewer allocs at N=32. This is a *within-library*
finding the docs never made. It turns "use `ProvideAll` for multiple values"
from a stylistic note into a measured recommendation.

### Decision 4: Do not claim plocal wins on raw injection speed

At N=32, plain `context.WithValue` is 1452 ns / 1536 B vs plocal's 2049 ns /
2456 B — stdlib is cheaper in raw cost even at high N. plocal's injection win is
**alloc count and resulting shape** (6 allocs, 1 level vs 32 allocs, 32 levels),
not nanoseconds. The spec and README must say this, because the previous spec's
unconditional "faster" requirement is exactly the kind of claim that cannot
survive a re-run.

### Decision 5: Express the lookup comparison as depth-conditional with a crossover

The requirement becomes: for N ≤ ~4 stdlib is faster in raw ns; for larger N
plocal is faster and the gap widens; plocal's cost is independent of depth while
stdlib's is linear. A test can verify flatness (allocs == 0 at all depths;
ns/op within a tolerance band across depths) — it cannot verify a fixed ns.

### Decision 6: Demonstrate scope override with a test, not a benchmark

"Local override, not permanent replace" is a semantic, not a timing. It belongs
in the correctness suite (`api_test.go`) as a done-observable assertion: an
inner scope overriding a key leaves the outer scope's value intact, and the
same guarantee via stdlib requires re-wrapping. Benchmarking it adds no
information.

## Risks / Trade-offs

- **Re-measured numbers drift again** → Report alloc counts (stable) and
  structural relationships as the assertions; keep ns/op advisory and dated with
  a machine note.
- **The corrected depth benchmark may still be flat** → If flat, that *is* the
  finding: state flat plocal vs linear stdlib and delete the linearity claim.
  Do not re-introduce a ramp to make the old story true.
- **`-benchtime` used by the docs may not match the run** → Standardize the
  Makefile target the docs cite, or drop ns/op from the committed block.
- **Crossover at N≈4 is noise-adjacent** (24.6 vs 29.4 ns) → State it as a
  range ("roughly 4–8 depending on machine"), not a precise integer, and let
  the flat-vs-linear shape carry the argument.
- **README narrative is load-bearing for the library's pitch** → The inverted
  claim ("stdlib faster at every depth") is the most visible error; correct it
  first, before the tables.
