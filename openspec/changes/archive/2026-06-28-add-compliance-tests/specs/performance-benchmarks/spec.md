## ADDED Requirements

### Requirement: Use allocates zero heap memory per call after scope setup
After a scope has been created, `Use()` SHALL perform zero heap allocations per invocation.

#### Scenario: BenchmarkUse_Depth1 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with one plocal node
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

#### Scenario: BenchmarkUse_Depth10 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with 10 nested plocal nodes, reading a key from the deepest node
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

#### Scenario: BenchmarkUse_Depth100 reports 0 allocs/op
- **WHEN** `Use()` is benchmarked on a context with 100 nested plocal nodes
- **THEN** `b.ReportAllocs()` reports 0 allocations per operation

### Requirement: ProvideAll allocates proportionally to providers, not to scope depth
`ProvideAll()` SHALL allocate memory proportional to the number of providers being injected, NOT to the depth of the existing scope chain. Creating a new scope at depth 100 SHALL allocate the same amount as creating one at depth 1.

#### Scenario: ProvideAll alloc count is independent of existing chain depth
- **WHEN** `ProvideAll()` with one provider is benchmarked at depth 1 and at depth 100
- **THEN** both show the same number of allocations per operation

### Requirement: Use lookup time scales linearly with depth (O(depth))
`Use()` lookup time SHALL scale linearly with the depth of the registry chain, not geometrically or logarithmically.

#### Scenario: Depth-scaling benchmark confirms linear growth
- **WHEN** `Use()` is benchmarked at depth 1, 10, and 100 seeking a key at the root
- **THEN** the ns/op values increase proportionally (10x depth ≈ 10x time, within a 2x tolerance margin)

### Requirement: plocal Use is faster than stdlib context.WithValue traversal for equivalent depth
For equivalent depth, `Use()` SHALL be measurably faster than an equivalent `context.Value()` traversal through a chain of standard `context.WithValue` nodes.

#### Scenario: BenchmarkVsStdlib_Depth10 shows plocal advantage
- **WHEN** both plocal and stdlib are benchmarked at depth 10 reading a value from the root
- **THEN** plocal `Use()` ns/op is lower than stdlib `context.Value()` ns/op
