## Context

The library stores a scope's values in a `registryNode` — a linked prototype
chain the library owns — whereas `context.WithValue` wraps one node per key.
The benchmark harness built the wrong structure (one node regardless of depth),
the docs narrated depth (the one axis plocal loses), and the performance
optimization that added a `Provide` fast path introduced a race and a shadowing
bug. This change fixes the harness, the defects, and the narrative.

All numbers below are from this machine: Intel Core Ultra 9 185H, Go 1.27.0,
linux/amd64, GOMAXPROCS=22, `-benchmem`, `-benchtime=1–2s`. They are advisory in
absolute terms; ratios within a run, alloc counts, and level counts are the
evidence.

### Corrected: the harness bug and the depth axis

`buildChain` called `WithProviders` once for any `depth`, so every depth
benchmark measured a single node. Fixed, `buildChain(depth)` produces exactly
`depth` linked nodes (verified by `chainDepth` and by forcing a full traversal
with a never-present key). The committed ramp was **not** a phantom — it
reproduces once the chain is real:

| Depth | plocal `Use()` | stdlib `ctx.Value()` |
|---|---|---|
| 1 | 37 ns | **8 ns** |
| 10 | 256 ns | **54 ns** |
| 100 | 2554 ns | **448 ns** |

Both are linear; stdlib is cheaper at every depth. Depth is disclosed as a
plocal loss, not a headline.

### Measured: the real axis — injection width

Read the outermost-injected key after injecting N values with each shape:

| N | batched (1 node) | spread (N scopes) |
|---|---|---|
| 1 | 37 ns | 37 ns |
| 4 | 37 ns | 112 ns |
| 8 | 38 ns | 213 ns |
| 16 | 37 ns | 429 ns |
| 32 | **37 ns** | **859 ns** |

Batched reads are **flat in N**; spread reads are **linear in N**.

### Measured: injection cost and shape (N = 32)

| Strategy | ns/op | B/op | allocs/op | levels |
|---|---|---|---|---|
| `WithProviders(ctx, 32)` | 2079 | 2488 | **6** | **1** |
| 32× `WithProviders` (spread) | 9519 | 13824 | 128 | 32 |
| 32× `UpdateProvider` (upsert) | 2896 | 512 | 32 | 1 |
| 32× `context.WithValue` | **1418** | **1536** | 32 | 32 |

### Measured: the two defects the fast path introduced

- **Race:** upsert mutated a `registryNode.values` map that may already be
  shared; concurrent `Use` + `WithProvider` fails `-race`.
- **Shadowing:** a `Provide` direct-context value permanently shadowed later
  registry injections of the same key; no lookup ordering fixes both directions.

Both are fixed by (a) a `sync.RWMutex` per node, (b) `Provide` back on the
registry, (c) `Use()` registry-only (no fast path). All injection orders then
follow "nearest scope wins" (verified across five orderings).

## Goals / Non-Goals

**Goals:**

- Make `buildChain` measure what its name and comment claim.
- Fix the race and the shadowing defect in production code.
- Replace every committed number with a freshly measured one; make the file and
  the README agree with a single run.
- Reframe the performance story around injection **width** (N values → one node
  → flat reads), the axis the library is actually built around.
- State the trade honestly, including where stdlib wins (raw single-key lookup
  at every depth; raw injection ns/bytes).
- Correct the spec so it asserts stable structure (allocs, levels,
  flat-vs-linear shape), not timing.

**Non-Goals:**

- No change to the *intended* public API surface beyond documenting the new
  `UpdateProvider`/`UpdateProviders`.
- No new dependencies (only `sync` and `context`).
- No CI benchmark gating; this change fixes what is measured, not when.
- No attempt to make plocal beat stdlib on deep single-value chains or raw
  injection ns — the measurements say it does not, and the docs must not claim
  it does.
- No commitment to specific ns/op thresholds in the spec; assert structure.

## Decisions

### Decision 1: Fix the harness before touching a single number

Rewriting the baseline numbers without fixing `buildChain` would keep measuring
a 1-node chain. Order: fix the helper, re-measure, then update docs and spec.
The corrected `buildChain` nests `depth` times with the sought key at the
outermost node, so a read is a worst-case traversal. Verified by
`TestBuildChain_RegistryDepth`.

### Decision 2: Inject N values, then measure reads — not depth

The comparison that matters is not "read a key at depth N" (plocal loses that at
every N and it is not what the library optimizes) but "inject N values for one
scope, then read: batched into one node vs spread across N scopes". The sweep
reports levels and allocs (stable facts); ns/op is advisory.

### Decision 3: Report the batch-vs-spread gap explicitly

Measured: ~5x faster and ~20x fewer allocs at N=32, plus flat-vs-linear reads.
This turns "use `ProvideAll` for multiple values" from a stylistic note into a
measured recommendation: batching is cheaper to build *and* keeps reads flat.

### Decision 4: Do not claim plocal wins on raw injection speed

At N=32, plain `context.WithValue` is 1418 ns / 1536 B vs plocal's 2079 ns /
2488 B — stdlib is cheaper in raw cost even at high N. plocal's injection win is
level count and alloc count (6 allocs, 1 level vs 32 allocs, 32 levels), which is
what buys the flat reads. The spec and README must say this.

### Decision 5: State the depth result truthfully

With the harness fixed, plocal `Use()` is linear in depth and stdlib is cheaper
at every depth. The spec requirement is therefore *not* "flat plocal vs linear
stdlib"; it is: plocal lookup grows with the number of nodes, and depth is
disclosed as a non-advantage. A test can verify structure (0 allocs; monotonic
growth with node count) — it cannot verify a fixed ns.

### Decision 6: Fix the two defects rather than document around them

The race and the shadowing bug are correctness problems in shared code, so they
are fixed in production, not worked around:

- **Race:** `registryNode` gains a `sync.RWMutex`. `Use()` reads under `RLock`;
  `UpdateProvider`/`UpdateProviders` and the `WithProvider` merge path write
  under `Lock`. Cost is small (depth-1 read unchanged; the deep walk pays a
  modest per-node lock, on the already-losing path).
- **Shadowing:** `Provide` returns to the registry (equivalent to `ProvideAll`
  with one provider) and `Use()` drops the `c.Value(key)` fast path. A direct
  stdlib key and a registry node cannot encode relative position, so the only
  correct design keeps all values in one structure.

### Decision 7: Demonstrate scope override with a test, not a benchmark

"Local override, not permanent replace" is a semantic. It belongs in the
correctness suite as a done-observable assertion: an inner scope shadows a key,
the outer scope's value is unchanged, for both the closure and derived-context
APIs. Benchmarking it adds no information.

## Risks / Trade-offs

- **Re-measured numbers drift again** → Assert alloc/level counts and the
  flat-vs-linear shape; keep ns/op advisory and dated with a machine note.
- **The mutex costs on the deep walk** → Measured ~35% on a 32-level walk, on the
  path that already loses to stdlib; the width axis (the win) is unaffected.
  Accept the cost for race-freedom.
- **`WithProvider` upsert surprises** → Merging distinct keys into the current
  node means `WithProvider` cannot build a deep chain and repeated calls stay at
  one level. Documented in the API reference and asserted by a test.
- **`Update*` mutates a live context** → Documented as a deliberate upsert with
  a race guard; the README warns to prefer `Provide`/`ProvideAll` for scope-local
  injection.
- **README narrative is load-bearing for the pitch** → Move the pitch off depth
  (where it loses) onto width and type safety (where it wins); correct the
  headline before the tables.
