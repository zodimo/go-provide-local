### Requirement: ProvideAll allocates proportionally to providers, not to scope depth
`ProvideAll()` SHALL allocate memory proportional to the number of providers being injected, NOT to the depth of the existing scope chain. Creating a new scope at depth 100 SHALL allocate the same amount as creating one at depth 1. The node's entry storage SHALL be sized to the provider count at construction so that no growth reallocation occurs.

#### Scenario: ProvideAll alloc count is independent of existing chain depth
- **WHEN** `ProvideAll()` with one provider is benchmarked at depth 1 and at depth 100
- **THEN** both show the same number of allocations per operation

### Requirement: Use performs zero heap allocations per call
After a scope has been created, `Use()` SHALL perform zero heap allocations per invocation. The read path does not copy, normalize, or allocate per call; all construction cost is paid when the node is built, which is what allows reads to be a lock-free pointer walk over immutable storage.

#### Scenario: BenchmarkUse_Depth1 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with one plocal node
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

#### Scenario: BenchmarkUse_Depth10 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with 10 nested plocal nodes, reading a key from the deepest node
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

#### Scenario: BenchmarkUse_Depth100 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with 100 nested plocal nodes
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

### Requirement: Use lookup time scales linearly with depth (O(depth))
`Use()` SHALL scale linearly with the depth of the registry chain. It walks the
Prototype Chain of nodes the library owns, probing each node's values, so its
lookup time MUST NOT stay flat and MUST NOT grow faster than linearly. This
corrects an earlier framing that implied `Use()` was depth-independent; a
corrected harness shows the growth is real, and that stdlib is nonetheless
cheaper per step.

#### Scenario: Depth-scaling benchmark confirms linear growth
- **WHEN** `Use()` is benchmarked at depth 1, 10, and 100 on a correctly nested
  chain, seeking a key at the outermost node
- **THEN** the ns/op values increase approximately proportionally to depth
  (no plateau, and not superlinear)

### Requirement: plocal Use is faster than stdlib context.WithValue traversal for equivalent depth

Documentation and specs SHALL state the plocal-vs-stdlib lookup trade-off
honestly rather than asserting an unconditional win. For a single-key lookup, a
`plocal` step is a `map[any]any` probe, while a stdlib step is a pointer
comparison; measured at depth 10, stdlib `context.Value()` is faster (~61 ns/op)
than `Use()` (~139 ns/op). Stdlib wins on raw single-key lookup speed at shallow
depth, while `plocal` wins on type safety, zero-allocation reads at any depth, and
scope creation that is O(providers) rather than O(depth). No documentation SHALL
claim that `Use()` is unconditionally faster than `context.Value()`.

#### Scenario: Documentation states the trade-off rather than a blanket win
- **WHEN** a reader examines the benchmark results in `README.md`
- **THEN** the text does not claim that `Use()` is unconditionally faster than
  `context.Value()`
- **AND** it names stdlib's advantage (single-key lookup speed at shallow depth)
  and plocal's advantages (type safety, zero-alloc reads, O(providers) scope
  creation)

#### Scenario: Both implementations are benchmarked at the same depth for comparison
- **WHEN** a plocal benchmark is paired with a stdlib benchmark at depth N
- **THEN** both read the value from the root of a chain of depth N
- **AND** both report allocations via `b.ReportAllocs()`

### Requirement: Injecting N values for one scope creates one node and keeps reads flat
Injecting N values for one scope via `ProvideAll`/`WithProviders` SHALL create
exactly one registry level and a bounded, N-independent allocation count, in
contrast to N nested scopes which create N levels and N allocations. A read of a
key in the batched scope SHALL stay flat as N grows, while reading a key spread
across N scopes SHALL grow with N.

#### Scenario: One level for N providers
- **WHEN** N values are injected via a single `ProvideAll`/`WithProviders` call
- **THEN** the resulting registry depth is 1 for any N

#### Scenario: N scopes cost N levels
- **WHEN** N values are each injected into their own nested scope
- **THEN** the resulting registry depth is N

#### Scenario: Batched reads stay flat while spread reads grow
- **WHEN** the outermost-injected key is read after injecting N values batched
  into one node, and, separately, after spreading them across N scopes, for
  N ∈ {1, 4, 8, 16, 32}
- **THEN** the batched ns/op stays within a narrow band across all N
- **AND** the spread ns/op grows with N

#### Scenario: Allocation count is sublinear in N for batching
- **WHEN** injection is benchmarked for N ∈ {1, 4, 16, 32}
- **THEN** batched allocs/op stays within a small constant band while N-scope
  allocs/op grows proportionally to N

### Requirement: Batched injection is cheaper than spreading the same values
A single batched injection call SHALL cost fewer allocations and less time than
spreading the same N values across N nested scopes. Spreading SHALL be treated
as the worse strategy and MUST NOT be presented as equivalent to batching.

#### Scenario: Batched injection beats spread injection
- **WHEN** injecting N=32 values via one `WithProviders` call is benchmarked
  against 32 calls that each add a scope
- **THEN** the batched call reports fewer allocs/op and lower ns/op

### Requirement: The suite reports where stdlib wins, without asserting a fixed ns/op threshold
The spec and docs SHALL state plainly where a measurement favors stdlib — raw
single-key lookup at every depth, and raw injection ns/bytes for a single wrap.
No requirement SHALL assert a fixed ns/op threshold; assertions SHALL be on
structure (allocation counts, level counts, flat-vs-linear shape).

#### Scenario: Raw injection cost favors stdlib
- **WHEN** the injection-cost benchmarks report ns/op and B/op for N values
  injected with plocal and with N sequential `context.WithValue` calls
- **THEN** the recorded result notes stdlib's lower raw ns/bytes at equal N
- **AND** records plocal's advantage as level and allocation count, not speed

### Requirement: Scope override is local, not a permanent replacement
An inner scope's value for a key SHALL shadow only within that scope; the outer
scope's value MUST be unchanged once the inner scope is exited. This SHALL hold
regardless of which injection API is used, and for both the closure-scoped
(`Provide`/`ProvideAll`) and derived-context
(`WithProvider`/`WithProviders`/`Update*`) APIs.

#### Scenario: Inner override leaves outer scope intact
- **WHEN** a key is injected at an outer scope, overridden at an inner scope, and
  read again from the outer context after the inner scope
- **THEN** the inner read returns the inner value
- **AND** the post-inner read on the outer context returns the original value

#### Scenario: Override composes across API forms
- **WHEN** an outer value is injected with any one API and overridden at an inner
  scope with any other (`Provide`, `ProvideAll`, `WithProvider`, `WithProviders`)
- **THEN** the inner read returns the override
- **AND** the outer context still reads its original value

### Requirement: Upsert writes are race-free against concurrent reads
Upsert SHALL be safe to call concurrently with `Use()` on the same context. This
covers `UpdateProvider`/`UpdateProviders` and the `WithProvider` path that merges
a new key into the current scope. The suite SHALL include a test that exercises
upsert and read concurrently under the race detector.

#### Scenario: Concurrent upsert and read
- **WHEN** one goroutine repeatedly calls `Use(base, key)` while another
  upserts into `base`
- **THEN** the test passes under `go test -race` with no data race reported

### Requirement: Node read path performs no locking
Reading from a `registryNode` SHALL NOT acquire a lock or perform any
synchronization. Node entry storage is immutable after construction, so a reader
traversing a chain can never observe a partially written node, and no read-time
coordination is required.

#### Scenario: Read path contains no lock acquisition
- **WHEN** `Use()` is called on a context with any number of plocal nodes
- **THEN** no mutex lock or unlock occurs on the read path

#### Scenario: Immutability is preserved across upsert
- **WHEN** `UpdateProvider` or `UpdateProviders` is called on a context whose
  leaf node already exists
- **THEN** the existing node is left unmodified and the resulting context
  carries a newly constructed node with the amended entries
- **AND** the registry level count is unchanged

### Requirement: Every benchmark scenario has a paired stdlib comparison

`plocal/api_bench_test.go` SHALL give every benchmark scenario a paired
standard-library comparison, so each plocal benchmark can be read side by side
with an equivalent `context.WithValue` benchmark over a chain of the same depth.
Every such scenario SHALL have a plocal benchmark and a stdlib benchmark at the
same depth, including `Use()` at depths 1, 10, and 100, and scope creation at
depth 1.

#### Scenario: Use at depth 1 has a stdlib pair
- **WHEN** the benchmark suite is listed
- **THEN** a plocal `Use()` benchmark at depth 1 and a stdlib `ctx.Value()`
  benchmark at depth 1 both exist

#### Scenario: Use at depth 10 has a stdlib pair
- **WHEN** the benchmark suite is listed
- **THEN** a plocal `Use()` benchmark at depth 10 and a stdlib `ctx.Value()`
  benchmark at depth 10 both exist

#### Scenario: Use at depth 100 has a stdlib pair
- **WHEN** the benchmark suite is listed
- **THEN** a plocal `Use()` benchmark at depth 100 and a stdlib `ctx.Value()`
  benchmark at depth 100 both exist

#### Scenario: Scope creation has a stdlib pair
- **WHEN** the benchmark suite is listed
- **THEN** a `ProvideAll()` benchmark at depth 1 compares against an equivalent
  stdlib `context.WithValue` scope creation at depth 1

### Requirement: Stdlib comparison benchmarks use an equivalent chain shape

The stdlib benchmarks SHALL build a chain of the same depth as their plocal
counterpart and place the sought value at the root, so the comparison measures the
same traversal distance.

#### Scenario: stdlib chain depth matches the plocal chain depth
- **WHEN** a stdlib comparison benchmark runs at depth N
- **THEN** it constructs N nested `context.WithValue` nodes
- **AND** the sought key is stored in the outermost (root) node

### Requirement: Documented baseline numbers SHALL be reproducible on demand

The baseline numbers in code comments and in the `README.md` results table SHALL be
regenerated from an actual `make bench-mem` run and SHALL agree with what the suite
reports within normal run-to-run variance. Stale or extrapolated numbers SHALL NOT
be published as measured results; the `README.md` table SHALL list only numbers
produced by a real run.
#### Scenario: README table matches a fresh benchmark run
- **WHEN** a maintainer runs `make bench-mem` after a change
- **THEN** the numbers in the README table match the reported ns/op, B/op, and
  allocs/op within normal variance
- **AND** no figure is present that was not produced by a real run

#### Scenario: Committed baseline comments match measurement
- **WHEN** the baseline block in `api_bench_test.go` is compared with a fresh run
- **THEN** the recorded ns/op values reflect the current machine's measurements
- **AND** the recorded allocs/op values match
