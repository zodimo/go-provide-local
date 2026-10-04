## ADDED Requirements

### Requirement: Platform example documents its ADK-v2 provenance and adaptation

The `examples/platform` package SHALL document that it is adapted from
`google.golang.org/adk/v2`'s platform package and that its seams are backed by
`plocal` rather than `context.WithValue`.

#### Scenario: Package doc states provenance
- **WHEN** a reader opens `examples/platform/doc.go`
- **THEN** the package documentation names `google.golang.org/adk/v2` as the origin
- **AND** states that the context seam is implemented with `plocal`

### Requirement: Platform example documents only seams that exist

Documentation in `examples/platform` SHALL reference only APIs that are defined in
the package. No documentation SHALL reference symbols absent from the package, such
as `WithUUIDProvider` or `NewUUID`.

#### Scenario: doc.go references defined symbols only
- **WHEN** every `With...` or exported function named in `examples/platform/doc.go`
  is looked up in the package
- **THEN** each one resolves to a declared symbol
- **AND** no reference to `WithUUIDProvider` or `NewUUID` remains

#### Scenario: Documented seams match the implemented seams
- **WHEN** the README or package docs describe the platform example
- **THEN** the described seams are `WithTimeProvider`/`Now` and
  `WithTaskRunner`/`RunTasks`
- **AND** each named function exists in the package

### Requirement: README documents the WithProvider and WithProviders API

`README.md` SHALL document `plocal.WithProvider` and `plocal.WithProviders` in its
API reference, explaining that they return an enriched `context.Context` directly
rather than evaluating a consumer closure.

#### Scenario: API reference lists both context-returning helpers
- **WHEN** a reader searches the README API reference for injecting a single value
- **THEN** `WithProvider[T any](ctx context.Context, key *ResourceKey[T], val T) context.Context`
  is documented
- **AND** `WithProviders(ctx context.Context, providers []Provider) context.Context`
  is documented

#### Scenario: README explains the closure-vs-context distinction
- **WHEN** a reader compares `Provide`/`ProvideAll` with `WithProvider`/`WithProviders`
- **THEN** the README states that `Provide`/`ProvideAll` scope a consumer closure
  and auto-release the scope
- **AND** states that `WithProvider`/`WithProviders` return a derived context

### Requirement: plocal package documentation covers context-returning injection

`plocal/doc.go` SHALL document the context-returning injection path
(`WithProvider`/`WithProviders`) alongside `Provide`/`ProvideAll`.

#### Scenario: doc.go describes both injection styles
- **WHEN** a reader opens `plocal/doc.go`
- **THEN** the documentation describes injecting via `ProvideAll` with a consumer
  closure
- **AND** describes injecting via `WithProvider`/`WithProviders` that return a
  context

### Requirement: README documents the platform example as an adoption proof

`README.md` SHALL include a section describing `examples/platform` as a worked
integration example showing a real library adopting `plocal` in place of
`context.WithValue`.

#### Scenario: README links the platform example
- **WHEN** a reader looks for a real-world usage example
- **THEN** the README points to `examples/platform`
- **AND** explains that the ADK platform seam was reimplemented on top of
  `plocal.WithProvider`
