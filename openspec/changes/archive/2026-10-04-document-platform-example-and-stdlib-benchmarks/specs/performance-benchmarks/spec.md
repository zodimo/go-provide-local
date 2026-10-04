## MODIFIED Requirements

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

## ADDED Requirements

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
