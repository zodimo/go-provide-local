## Context

`plocal` stores a scope's values in a `registryNode`, a linked prototype chain
the library owns. Reads (`Use`) walk that chain from the leaf to the root and
return the first match; writes (`Provide*`, `Update*`, `WithProvider`) either
push a node or amend one.

The previous change (`fix-benchmark-truthfulness-and-injection-cost`) added a
`sync.RWMutex` to every node so that `UpdateProvider` / `UpdateProviders` could
upsert entries into an already-shared node's `map[any]any` without racing a
concurrent `Use`. That was a correct fix for the race it addressed, but it
placed a lock on the read path and kept an interface-keyed map as the storage.
Both are incidental costs, and this change removes them together.

All numbers are from this machine: Intel Core Ultra 9 185H, Go 1.27.0,
linux/amd64, GOMAXPROCS=22, `-benchmem`. Absolute values are advisory; ratios,
alloc counts, and level counts are the evidence.

### Measured: isolating the two costs

A standalone probe reproduces the node structure and walks a 100-deep chain,
varying one factor at a time:

| Read path (depth 100) | ns/op | B/op | allocs/op |
|---|---|---|---|
| today: `RWMutex` + `map[any]any` | 2633 | 0 | 0 |
| lock-free + `map[any]any` | 1144 | 0 | 0 |
| lock-free + pointer-keyed slice | **276** | 0 | 0 |
| stdlib `context.Value()` | 582 | 0 | 0 |

- **The lock costs 2.3x.** Removing it from the read path is the largest single
  win and does not depend on the storage decision.
- **The map costs another ~4x on top.** Interface-keyed hashing is pure
  overhead when keys are already pointer-unique addresses.
- The two compound to **9.5x**, which puts plocal ~2x ahead of stdlib at depth
  100 — a position unreachable by tuning either factor alone.

### Measured: the resulting depth profile

| Depth | plocal today | target | stdlib | vs stdlib |
|---|---|---|---|---|
| 1 | 43 ns | 6.4 ns | 11.9 ns | beats |
| 10 | 290 ns | 29.6 ns | 72.9 ns | beats |
| 100 | 2621 ns | 276 ns | 582 ns | beats |

Note that depth 1 also improves sharply (43 → ~6 ns), because a one-entry slice
scan avoids map hashing overhead entirely. Most real scopes are shallow, so this
is the case that matters most.

### Measured: injection improves too

Building one node with N entries, slice vs map:

| N | slice | map | note |
|---|---|---|---|
| 8 | 131 ns / 64 B / 8 allocs | 368 ns / 64 B / 8 allocs | 2.8x faster |
| 32 | 521 ns / 256 B / 32 allocs | 2115 ns / 2600 B / 35 allocs | 4x faster, 10x less memory |

The byte reduction is Go's map overflow-bucket overhead disappearing; the
alloc count tracks entry count in both shapes because each `Value[T]` boxes its
value. Injection was not the goal of this change, but it improves for free.

### Measured: the width regression

A pointer-keyed slice scans linearly. Within a *single* node, a worst-case read
(wanted entry last) grows with N, where the map is roughly flat:

| Entries in one node | slice (last) | slice (first) | map |
|---|---|---|---|
| 4 | 13.6 ns | ~7 ns | 21 ns |
| 8 | 16.7 ns | ~7 ns | 36 ns |
| 16 | 23.7 ns | ~7 ns | 16 ns |
| 32 | 48.6 ns | 6.9 ns | 24 ns |
| 64 | 86.3 ns | 7.0 ns | 17 ns |

The crossover is around N≈16; the scan is positional, so an entry found early
is ~7 ns regardless of N. This is the one behavior that gets worse, and it is
accepted rather than engineered around. Two reasons: the depth axis (which the
slice wins everywhere) is the axis that matters for nesting, and per-scope value
counts in practice are small. The README and the width requirement will state
the soft cap explicitly instead of claiming unconditional flatness.

## Goals / Non-Goals

### Goals

- Make a node's entry set immutable after construction, so no lock is needed on
  the read path and the concurrency guarantee holds by construction.
- Replace interface-keyed map storage with pointer-keyed slice storage.
- Preserve all public behavior: shadowing, inheritance, sibling isolation,
  fallback, one-node-per-scope, and upsert's no-new-level semantics.
- Preserve the race-free guarantee under `-race`, including concurrent
  upsert-vs-read.
- Re-measure and re-document honestly, including the width regression.

### Non-Goals

- Eliminating the `c.Value(registryKey)` walk that locates the leaf node.
- Hybrid slice→map storage or any storage that is adaptive in N.
- Any public API, signature, or semantic change.

## Decisions

### Decision: immutability replaces the lock, rather than a cheaper lock

The previous change needed a lock because upsert mutated a shared map. Two
escape routes existed: keep mutation and use a cheaper synchronization
primitive, or remove mutation so there is nothing to synchronize. We take the
second.

- **Chosen: immutable nodes.** `Use` becomes a lock-free pointer chase. Upsert
  produces a new node reflecting the amended entries; the old node is never
  written. A reader traversing a chain therefore never observes a torn state and
  never needs to coordinate.
- **Rejected: cheaper lock (e.g. a per-node atomic or a sharded lock).** It
  keeps the cost on the read path and keeps map storage, forgoing the 4x
  storage win, and it leaves the reader/writer coupling in place.
- **Rejected: atomic pointer-swap of the map.** It would preserve in-place
  upsert observably, but a map behind an atomic pointer still cannot be written
  concurrently with readers — the writer must copy the map anyway, so it is
  immutability with extra steps and an extra indirection.

**Consequence:** an upsert that previously returned the *same* context now
returns a context whose leaf is a *new* node. This is invisible to callers,
since the returned context is used as before, and it preserves the
"no new level" property that the correctness tests assert.

### Decision: pointer-keyed slice over `map[any]any`

Keys are `*ResourceKey[T]` — already used as map keys by address. A slice of
`{key any, val any}` compared by pointer identity turns a hash into a compare,
and stores entries contiguously.

- **Chosen: `[]entry` linear scan.** 4x faster probing at depth, 3-4x faster and
  10x leaner at injection, and best for the common small-N case.
- **Rejected: keeping the map and only removing the lock.** Leaves ~4x on the
  table and keeps the injection-bytes overhead.

**Consequence:** worst-case read inside a large single node is linear, giving
the ~16 soft cap documented above.

### Decision: keep the read path allocation-free

`Use` must continue to allocate nothing. This is why the read path cannot copy
or normalize anything per call, and why the node is front-loaded with the work
(construction builds the slice; reading only walks it). The existing
zero-allocation requirement in the `performance-benchmarks` spec is retained
unchanged and is a load-bearing constraint on any future read-path work.

### Open question: the `c.Value(registryKey)` floor

`Use` first calls `c.Value(registryKey)` to locate the leaf node, which walks
the *standard* context chain (timeouts, cancellations, HTTP request contexts)
by interface assertion. A probe of a bare `ctx.Value` miss on
`context.Background()` measured 0.53 ns, but that is the trivial case; the cost
scales with the number of standard-library wrappers between the given context
and the node. After this change the node walk is ~6 ns at depth 1, so the
standard-context walk is no longer negligible by comparison.

This change does **not** address it. It is recorded as the next candidate and
should be measured under realistic `net/http` + `WithTimeout` composition before
any work is scoped. No design decision here depends on its outcome.

## Risks / Trade-offs

- **Width regression (accepted).** Worst-case read in a node with >~16 entries
  is slower than today's map (86 ns vs 50 ns at N=64). Mitigation is
  documentation: a soft cap in the README and an honestly-stated width
  requirement. Not engineered around.
- **All existing benchmark numbers become stale.** Every value in
  `api_bench_test.go`'s baseline block, the previous change's artifacts, and the
  README tables is invalidated by this change. They must be regenerated from a
  single fresh run, and the README narrative must be revised — not just
  re-numbered, since the depth conclusion flips from "plocal loses" to "plocal
  wins."
- **Test assertions coupled to in-place mutation.** `plocal/api_test.go` §4
  asserts upsert semantics; assertions phrased over map identity or pointer
  equality of the node will need restating in terms of returned values and
  level counts. This is a test-rewrite cost, not a behavior change.
- **`-race` must still pass.** Immutability should make this strictly easier,
  but the concurrent upsert-vs-read test (4.4) is the proof and must be re-run
  under `-race`.
- **Slice growth policy.** Constructing a node should size the slice exactly
  (`make([]entry, 0, len(providers))`) to avoid growth reallocations; this
  affects the injection alloc counts that the spec asserts.

## Migration Plan

No migration required. The public API and semantics are unchanged; only the
internal representation differs. Callers recompile against the same signatures.

Sequencing within the change:

1. Convert `registryNode` to an immutable, pointer-keyed slice and remove the
   lock, keeping behavior identical.
2. Rework `UpdateProvider` / `UpdateProviders` to copy-on-upsert.
3. Re-run the full `-race` test suite, including the concurrent upsert-vs-read
   test.
4. Regenerate benchmarks from one run; update the baseline block and README.
5. Apply the spec deltas for `performance-benchmarks`, `safety-tests`, and
   `correctness-tests`.

## Open Questions

- Should the width soft cap be enforced (a lint or a test that fails past N) or
  only documented? Current leaning: documented only, since the regression is
  graceful and per-scope counts are a caller choice.
- Is the `c.Value(registryKey)` floor worth a follow-up change? Depends on the
  composition measurement described above.
