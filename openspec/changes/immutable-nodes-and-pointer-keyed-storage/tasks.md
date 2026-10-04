# Tasks

## 1. Convert node storage to immutable, pointer-keyed slices

- [x] 1.1 Replace `registryNode.values map[any]any` with an `entries []entry` slice (`entry{key any; val any}`) in `plocal/models.go`
- [x] 1.2 Remove `sync.RWMutex` from `registryNode` and delete the `get`/`has`/`put` lock wrappers; replace with lock-free `get(k any) (any, bool)` and `has(k any) bool` that scan `entries` by pointer compare
- [x] 1.3 Construct nodes with `make([]entry, 0, len(providers))` so entry storage is sized exactly and never grows
- [x] 1.4 Drop the now-unused `sync` import from `plocal/models.go`
- [x] 1.5 Add a doc comment on `registryNode` stating the immutability invariant: entries are fixed at construction and never mutated

## 2. Rewrite the construction and read paths

- [x] 2.1 Update `WithProviders` (`plocal/api.go`) to build the `entries` slice from providers and construct the node without locking
- [x] 2.2 Update `Use()` to walk the chain with the lock-free `get`, preserving nearest-scope-wins, nil-context handling, and the fallback return
- [x] 2.3 Confirm `Use()` still allocates 0 B/op at depths 1/10/100 (`-benchmem`)
- [x] 2.4 Update the `Provider` interface doc and `providerImpl.apply` to write into the entry slice rather than a map (or replace `apply` with an entry-returning form, per the chosen construction shape)

## 3. Re-express upsert without mutation

- [x] 3.1 Rework `UpdateProvider` to construct a new node with the amended entry set, preserving the no-new-level property and falling back to `WithProviders` when no node exists
- [x] 3.2 Rework `UpdateProviders` the same way for multiple entries
- [x] 3.3 Verify `WithProvider`'s merge-vs-shadow logic still holds: a new key merges into the current node (no new level), an existing key pushes a shadowing node
- [x] 3.4 Update the `UpdateProvider`/`UpdateProviders`/`WithProvider` doc comments to describe copy-on-upsert rather than in-place mutation

## 4. Restate the correctness tests for copy-on-upsert

- [x] 4.1 Update `plocal/api_test.go` §4 assertions that depend on in-place mutation or node pointer identity; assert on resolved values and registry level counts instead
- [x] 4.2 Keep and re-verify the concurrent upsert-vs-`Use` race test (4.4) under `-race`
- [x] 4.3 Add a test asserting the parent context still resolves the original value after a derived context is upserted
- [x] 4.4 Run the full suite with `go test ./... -race` and confirm zero races

## 5. Re-measure and regenerate the baseline

- [x] 5.1 Re-run the full benchmark suite in a single run and capture ns/op, B/op, allocs/op, and level counts
- [x] 5.2 Replace the baseline comment block in `plocal/api_bench_test.go` with the fresh values
- [x] 5.3 Verify the depth benchmarks now show plocal faster than stdlib at depths 1/10/100; record the comparison table
- [x] 5.4 Verify injection benchmarks show reduced ns and B/op vs the pre-change numbers; record them
- [x] 5.5 Measure and record the width sweep, including the per-node count at which a worst-case batched read crosses over (expected ~16)

## 6. Correct the specs and narrative

- [x] 6.1 Confirm the `performance-benchmarks` delta is applied (depth requirement now asserts a plocal advantage; width requirement states the ~16 soft cap; injection requirement notes the byte reduction)
- [x] 6.2 Confirm the `safety-tests` delta is applied (concurrency guaranteed by immutability; concurrent upsert-vs-read required race-free)
- [x] 6.3 Confirm the `correctness-tests` delta is applied (copy-on-upsert semantics; nearest-scope-wins across all injection paths)
- [x] 6.4 Rewrite `README.md` benchmark results tables from the single fresh run
- [x] 6.5 Revise the README performance narrative: plocal now beats stdlib on depth; state the per-node width soft cap plainly; remove any claim invalidated by the new numbers
- [x] 6.6 Note in the README (or a changelog entry) that this change supersedes the `RWMutex` decision of `fix-benchmark-truthfulness-and-injection-cost`

## 7. Verification and close-out

- [x] 7.1 Run `go vet ./...` and the project lint target
- [x] 7.2 Run `go test ./... -race -count=2` to catch order-dependent flakiness
- [x] 7.3 Confirm no public API or behavior change: `Value`, `Provide`, `ProvideAll`, `Use`, `UpdateProvider`, `UpdateProviders`, `WithProvider` signatures and semantics unchanged
- [x] 7.4 Validate the change with `openspec validate --change "immutable-nodes-and-pointer-keyed-storage"`
- [x] 7.5 Record the `c.Value(registryKey)` composition question as a follow-up candidate; do not implement it here
