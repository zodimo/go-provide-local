## MODIFIED Requirements

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
This requirement is **withdrawn as false**, and the spec SHALL NOT assert a
plocal speed advantage here. For a matched-depth chain of single-value nodes,
stdlib `context.Value()` is cheaper than `Use()` at every depth (each plocal step
is a lock-guarded map probe plus a pointer hop; each stdlib step is a pointer
compare + type switch). The spec SHALL instead require that the benchmark suite
measures both and records the result honestly.

#### Scenario: Stdlib advantage is measured and disclosed
- **WHEN** both plocal and stdlib are benchmarked at depths 1, 10, and 100
  reading a value from the outermost node
- **THEN** stdlib `context.Value()` ns/op is lower than plocal `Use()` ns/op at
  every depth
- **AND** the recorded result and docs state this is a disclosed trade-off

## ADDED Requirements

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
