## MODIFIED Requirements

### Requirement: Use lookup time is independent of registry depth
`Use()` SHALL traverse only the nodes the library owns, so its lookup cost is
independent of how many scopes are nested. Nesting more scopes MUST NOT increase
`Use()` cost. This is a correction of the prior requirement, which claimed
`Use()` scales linearly with depth — that description matches stdlib
`context.Value()`, not `Use()`.

#### Scenario: Use cost does not grow with scope depth
- **WHEN** `Use()` is benchmarked on correctly nested chains of depth 1, 10, and 100, seeking a key at the root
- **THEN** the ns/op values at all three depths fall within a narrow tolerance band of one another (no proportional growth)
- **AND** allocations per operation remain 0 at every depth

#### Scenario: Stdlib lookup grows linearly while plocal stays flat
- **WHEN** plocal `Use()` and stdlib `context.Value()` are benchmarked at the same depths on shape-equivalent chains
- **THEN** stdlib ns/op grows approximately proportionally to depth while plocal ns/op does not

### Requirement: plocal and stdlib lookup trade off by depth, with a crossover
For equivalent-depth, worst-case lookup, plocal and stdlib SHALL be compared
honestly: stdlib is faster for shallow contexts and plocal is faster for deeper
ones, with a crossover at a small depth. This replaces the prior unconditional
claim that plocal is faster, and the README's inverse claim that stdlib is faster
at every depth; both are false.

#### Scenario: Stdlib wins at depth 1
- **WHEN** both implementations are benchmarked at depth 1 reading the sole/root key
- **THEN** stdlib `context.Value()` ns/op is lower than plocal `Use()` ns/op

#### Scenario: plocal wins at depth 100
- **WHEN** both implementations are benchmarked at depth 100 reading the root key
- **THEN** plocal `Use()` ns/op is lower than stdlib `context.Value()` ns/op
- **AND** the gap is material (plocal is flat while stdlib is linear)

### Requirement: ProvideAll amortizes injection into one context level
Injecting N values for one scope SHALL create exactly one context level and a
bounded, N-independent allocation count, in contrast to N sequential
`context.WithValue` wraps which create N levels and N allocations. Raw ns/op and
raw bytes/op for a single stdlib wrap MAY be lower than plocal's node creation;
the asserted advantage is level count and allocation count, not raw speed.

#### Scenario: One level for N providers, N levels for N stdlib wraps
- **WHEN** N values are injected via `ProvideAll`/`WithProviders` and, separately, via N sequential `context.WithValue` calls
- **THEN** the plocal result has registry depth 1 and the stdlib result has context depth N

#### Scenario: Allocation count is sublinear in N for plocal
- **WHEN** injection is benchmarked for N ∈ {1, 4, 16, 32}
- **THEN** plocal allocs/op stays within a small constant band while stdlib allocs/op grows proportionally to N

### Requirement: WithProviders is measurably cheaper than sequential WithProvider
A single `WithProviders` call SHALL cost fewer allocations and less time than
N sequential `WithProvider` calls when injecting the same N values into one
scope. The sequential path SHALL be treated as the worst of the three strategies
and MUST NOT be presented as equivalent to batching.

#### Scenario: Batched injection beats sequential injection
- **WHEN** injecting N=32 values via one `WithProviders` call is benchmarked against 32 sequential `WithProvider` calls
- **THEN** the batched call reports fewer allocs/op and lower ns/op

### Requirement: Scope override is local, not a permanent replacement
An inner scope's value for a key SHALL shadow only within that scope; the outer
scope's value MUST be unchanged once the inner scope is exited. This is the
"local override, not permanent replace" semantic, and the spec SHALL record that
the equivalent with stdlib requires re-wrapping the context.

#### Scenario: Inner override leaves outer scope intact
- **WHEN** a key is injected at an outer scope, overridden at an inner scope, and read again from the outer context after the inner scope
- **THEN** the inner read returns the inner value
- **AND** the post-inner read on the outer context returns the original outer value

#### Scenario: Override works with the context-returning API too
- **WHEN** `WithProviders` derives an inner context that overrides a key, and both the parent and derived contexts remain available
- **THEN** reading via the derived context returns the override while reading via the parent context returns the original value
