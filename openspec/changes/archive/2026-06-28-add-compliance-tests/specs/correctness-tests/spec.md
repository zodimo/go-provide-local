## ADDED Requirements

### Requirement: Fallback value returned when key not injected
`Use()` SHALL return the key's registered default fallback value when the key has never been injected into the context chain.

#### Scenario: Empty standard context returns fallback
- **WHEN** `Use()` is called with a plain `context.Background()` and a key with a known default
- **THEN** the default value is returned with no panic

#### Scenario: Non-plocal context returns fallback
- **WHEN** `Use()` is called on a context that has had values added via `context.WithValue` (not plocal)
- **THEN** the key's default value is returned with no panic

### Requirement: Injected value is returned by Use
`Use()` SHALL return the value most recently injected for a key in the nearest enclosing scope.

#### Scenario: Single Provide returns injected value
- **WHEN** a value is injected via `Provide()` and `Use()` is called inside the consumer
- **THEN** the injected value is returned

#### Scenario: ProvideAll with multiple providers returns all values
- **WHEN** multiple providers are passed to `ProvideAll()` and `Use()` is called for each key inside the consumer
- **THEN** each key resolves to its injected value

### Requirement: Scope inheritance — inner scope can read outer values
A child scope created with `Provide` or `ProvideAll` SHALL be able to read values injected by any ancestor scope.

#### Scenario: Two-level nesting reads parent value
- **WHEN** a value is injected at the outer scope and `Use()` is called from within an inner scope that does not re-inject that key
- **THEN** the outer value is returned

#### Scenario: Three-level nesting reads grandparent value
- **WHEN** a value is injected at the outermost scope and `Use()` is called two levels deeper
- **THEN** the outermost value is returned

### Requirement: Scope shadowing — inner value overrides outer
`Use()` SHALL return the innermost injected value for a key when the same key is injected at multiple nesting levels.

#### Scenario: Inner scope shadows outer scope
- **WHEN** the same key is injected at both an outer and an inner scope with different values
- **THEN** `Use()` inside the inner scope returns the inner value
- **AND** `Use()` inside the outer scope (before the inner scope) returns the outer value

### Requirement: Sibling scope isolation
Values injected in one sibling scope SHALL NOT be visible in another sibling scope.

#### Scenario: Two sibling scopes are independent
- **WHEN** two separate `Provide` calls are made from the same parent context for the same key with different values
- **THEN** each consumer sees only its own injected value

### Requirement: ProvideAll creates exactly one registry node
`ProvideAll()` with N providers SHALL create exactly one new `registryNode`, not N nodes.

#### Scenario: ProvideAll with three providers produces depth increment of one
- **WHEN** `ProvideAll()` is called with three providers on a context at depth D
- **THEN** the resulting context has registry depth D+1 (not D+3)
