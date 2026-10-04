## Why

`Use()` is slow for reasons that are incidental to the library's design, not
inherent to it. A probe on the author's machine (Intel Core Ultra 9 185H,
Go 1.27.0, linux/amd64, GOMAXPROCS=22) isolates two independent costs on the
read path, both introduced by the previous change's fix for the upsert data
race:

1. **A `sync.RWMutex` on every node read.** Added to make in-place upsert
   race-free, it charges an atomic CAS pair to every `get()` — including the
   overwhelming majority of nodes that are written once at creation and then
   only read.
2. **A `map[any]any` per node.** Interface-keyed maps hash a `runtime.iface`
   (type pointer + data pointer) on every probe, where a pointer compare would
   do — the keys are already `*ResourceKey[T]` addresses.

Isolated on a 100-deep chain, each fix measured independently:

| Read path (depth 100) | ns/op | vs today |
|---|---|---|
| today: `RWMutex` + `map[any]any` | 2633 | 1.0x |
| drop the read lock (immutable nodes) | 1144 | 2.3x |
| pointer-keyed slice, no lock | **276** | **9.5x** |
| stdlib `context.Value()` @ depth 100 | 582 | — |

The gains **compound**, and the resulting shape is faster than stdlib at every
depth — reversing the previous change's honest finding that "stdlib is cheaper
at every depth":

| Depth | plocal today | probe target | stdlib | result |
|---|---|---|---|---|
| 1 | 43 ns | **6.4 ns** | 11.9 ns | beats stdlib |
| 10 | 290 ns | **29.6 ns** | 72.9 ns | beats stdlib |
| 100 | 2621 ns | **276 ns** | 582 ns | beats stdlib |

Injection improves as a side effect: a contiguous slice avoids Go's map
overflow buckets, so building one node with 32 entries drops from 2115 ns /
2600 B to **521 ns / 256 B**.

**Known regression (accepted).** A pointer-keyed slice is scanned linearly, so
a *single* node holding many values regresses once N is large: at N=64 a
worst-case read costs ~86 ns where the current map costs ~50 ns. The crossover
is around N≈16. This is accepted as a minor tradeoff — the gains at every depth
and at injection dominate, and per-scope value counts are typically small. The
README and spec will document N-per-scope as a soft cap rather than claim
flat-in-width unconditionally.

**This supersedes the previous change.** `fix-benchmark-truthfulness-and-injection-cost`
chose the `RWMutex` deliberately (task 2.1) to make in-place upsert race-free.
This change reverses that choice by removing the reason for it: nodes become
immutable, so there is nothing to race against.

### Constraint

Absolute ns/op is machine- and thermal-dependent and is advisory. The
load-bearing evidence is (a) alloc counts and registry-level counts, which are
stable, (b) the flat-vs-linear *shape* of a walk, and (c) ratios measured within
a single run.

## What Changes

- **Make `registryNode` immutable after construction.** Drop `sync.RWMutex` and
  the `get`/`has`/`put` lock wrappers. A node's entry set is fixed when the node
  is created.
- **Replace `values map[any]any` with a pointer-keyed slice** (`[]entry`), so a
  lookup is a pointer compare rather than an interface hash. Keys stay the
  `*ResourceKey[T]` addresses already in use.
- **Remove the lock from the read path in `Use()`**, turning each step into a
  lock-free pointer chase.
- **Re-express upsert without mutation.** `UpdateProvider` / `UpdateProviders`
  build a new node containing the amended entry set (preserving the no-new-level
  semantics and `ProvideAll`'s "exactly one node" guarantee) instead of writing
  into a shared map. `WithProvider`'s merge-vs-shadow logic is preserved.
- **Keep the concurrency guarantee by construction.** Immutability means a
  shared context is safe to read concurrently without a lock, and an upsert
  never mutates memory a reader may be traversing.
- **Re-measure every benchmark** and regenerate the `api_bench_test.go` baseline
  comment block and the `README.md` results tables from one run.
- **Document the width soft cap.** State that per-scope value counts above
  ~16 are the case the slice storage does not optimize for.
- **Correct the `performance-benchmarks` spec**: replace "linear in depth,
  stdlib is faster" with the measured post-change position, and restate the
  width requirement so it no longer claims unconditional flatness.
- **Correct the `safety-tests` spec** so the concurrency requirement is phrased
  over immutability rather than over a lock.

## Capabilities

### Modified Capabilities

- `performance-benchmarks`: depth requirement updated to the post-change
  result (plocal beats stdlib at every depth); width requirement restated with
  the ~16-per-node soft cap; injection requirement updated with the slice's
  lower time and bytes.
- `safety-tests`: concurrency requirement rephrased to be guaranteed by node
  immutability rather than by a read lock, and extended to cover concurrent
  upsert-vs-read.
- `correctness-tests`: upsert semantics requirement added for the
  copy-on-upsert behavior, replacing the in-place-mutation wording.

## Impact

- **Production code:** `plocal/models.go` (node struct, entry slice, lock
  removal, `get`/`has`/`put` removal), `plocal/api.go` (`Use` walk, `WithProviders`
  construction, `UpdateProvider`/`UpdateProviders`, `WithProvider`).
- **Tests:** `plocal/api_test.go` — the in-place-upsert assertions in §4 must be
  restated in terms of resulting values and level counts rather than map
  identity; race test in 4.4 must still pass under `-race`.
  `plocal/api_bench_test.go` — regenerate the baseline block.
- **Docs:** `README.md` — regenerate results tables; revise the performance
  narrative now that plocal beats stdlib on depth.
- **Specs:** `openspec/specs/performance-benchmarks/spec.md`,
  `openspec/specs/safety-tests/spec.md`, `openspec/specs/correctness-tests/spec.md`
  (via deltas).
- **No new dependencies.** The change removes a `sync` import rather than adding
  one.
- **Supersedes** the `RWMutex` decision (task 2.1) of the change
  `fix-benchmark-truthfulness-and-injection-cost`, and the depth disclosure in
  the archived `document-platform-example-and-stdlib-benchmarks`.

## Non-goals

- No public API change. Function names, signatures, and semantics of `Value`,
  `Provide`, `ProvideAll`, `Provide`-family, `Use`, `UpdateProvider`,
  `UpdateProviders`, and `WithProvider` are unchanged from the caller's view.
- No hybrid slice→map storage. The width regression is accepted, not engineered
  around.
- No attempt to eliminate the `c.Value(registryKey)` interface walk that
  locates the leaf node. It remains the fixed floor of `Use()`.
- No change to `ResourceKey` construction, key identity, or fallback behavior.
