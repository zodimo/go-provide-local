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
`Use()` SHALL scale at most linearly with the depth of the registry chain. It
walks the Prototype Chain of nodes the library owns, probing each node's values,
so its lookup time MUST NOT stay flat and MUST NOT grow faster than linearly.
With immutable, pointer-keyed nodes the per-step cost is a pointer compare
rather than a lock-guarded hash, so linear growth is retained with a materially
lower constant factor.

#### Scenario: Depth-scaling benchmark confirms linear growth
- **WHEN** `Use()` is benchmarked at depth 1, 10, and 100 on a correctly nested
  chain, seeking a key at the outermost node
- **THEN** the ns/op values increase approximately proportionally to depth
  (no plateau, and not superlinear)

### Requirement: plocal Use is faster than stdlib context.WithValue traversal for equivalent depth
`Use()` SHALL be measurably faster than an equivalent `context.Value()`
traversal through a chain of standard `context.WithValue` nodes at equivalent
depth. This **supersedes** the previously withdrawn version of this
requirement, which asserted the opposite was true: that withdrawal was accurate
for the lock-guarded `map[any]any` node representation, and stops being true
once nodes are immutable and pointer-keyed. The spec SHALL require that the
benchmark suite measures both and records the result.

#### Scenario: BenchmarkVsStdlib shows plocal advantage at every depth
- **WHEN** plocal and stdlib are both benchmarked at depths 1, 10, and 100
  reading a value from the outermost node
- **THEN** plocal `Use()` ns/op is lower than stdlib `context.Value()` ns/op at
  every measured depth
- **AND** the recorded result and docs state the margin

### Requirement: Injecting N values for one scope creates one node and keeps reads flat
Injecting N values for one scope via `ProvideAll`/`WithProviders` SHALL create
exactly one registry level and a bounded, N-independent allocation count, in
contrast to N nested scopes which create N levels and N allocations. A read of a
key in the batched scope SHALL stay flat in N for per-scope value counts within
the supported range, while reading a key spread across N scopes SHALL grow with
N.

Because node storage is a pointer-keyed slice scanned linearly, a batched read
is flat only while the number of values stored in a **single** node stays small;
beyond roughly 16 values per node the read grows with the count, and the spec
SHALL NOT claim unconditional flatness.

#### Scenario: One level for N providers
- **WHEN** N values are injected via a single `ProvideAll`/`WithProviders` call
- **THEN** the resulting registry depth is 1 for any N

#### Scenario: N scopes cost N levels
- **WHEN** N values are each injected into their own nested scope
- **THEN** the resulting registry depth is N

#### Scenario: Batched reads stay flat within the supported per-node count
- **WHEN** the outermost-injected key is read after injecting N values batched
  into one node, for N up to the documented per-node soft cap
- **THEN** ns/op stays approximately constant across N

#### Scenario: Per-node soft cap is documented
- **WHEN** the documentation states the batched-read behavior
- **THEN** it states the per-node value count beyond which a batched read
  becomes linear, rather than claiming flatness for all N

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
