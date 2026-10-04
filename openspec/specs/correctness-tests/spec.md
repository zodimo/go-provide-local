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
`ProvideAll()` with N providers SHALL create exactly one new `registryNode`, not
N nodes. The node's entry storage SHALL be sized to N at construction, so the
single allocation for the value set happens once and does not grow.

#### Scenario: ProvideAll with three providers produces depth increment of one
- **WHEN** `ProvideAll()` is called with three providers on a context at depth D
- **THEN** the resulting context has registry depth D+1 (not D+3)

#### Scenario: Node storage holds exactly the injected entries
- **WHEN** `ProvideAll()` is called with N providers
- **THEN** the resulting node resolves all N keys to their injected values
- **AND** resolves no key that was not injected at that node

### Requirement: Upsert preserves scope semantics while replacing node storage
`UpdateProvider` and `UpdateProviders` SHALL upsert entries such that a key
already present in the current scope's node resolves to the new value, and a key
absent from it is added. The upsert SHALL NOT create a new registry level, and
SHALL NOT mutate the node it read from — the returned context carries a newly
constructed node with the amended entry set. When no registry node exists on the
given context, upsert SHALL create one.

#### Scenario: Upsert overwrites without adding a level
- **WHEN** `UpdateProvider` is called with a key already present in the current
  scope's node
- **THEN** `Use()` for that key returns the updated value
- **AND** the registry depth is unchanged

#### Scenario: Upsert adds a new key without adding a level
- **WHEN** `UpdateProviders` is called with keys not yet in the current scope's
  node
- **THEN** `Use()` resolves each added key to its upserted value
- **AND** the registry depth is unchanged

#### Scenario: Upsert on a node-less context creates one level
- **WHEN** `UpdateProvider` is called on a context with no registry node
- **THEN** the resulting context has registry depth 1 and resolves the upserted
  key

#### Scenario: Upsert leaves the source node intact
- **WHEN** a derived context is upserted and the parent context is read
  afterwards
- **THEN** the parent still resolves the original value for the upserted key

### Requirement: Shadowing precedence is nearest-scope-wins across all injection paths
`Use()` SHALL return the value from the nearest enclosing scope that holds the
key, regardless of which API injected it (`Provide`, `ProvideAll`,
`WithProvider`, `UpdateProvider`, `UpdateProviders`). Immutability MUST NOT allow
an earlier-injected value to shadow a later-injected one at the same or a nearer
scope.

#### Scenario: Later injection at a nearer scope shadows an earlier one
- **WHEN** a key is injected at an outer scope and then re-injected at an inner
  scope using a different injection API
- **THEN** `Use()` inside the inner scope returns the inner value

#### Scenario: Upsert then read reflects the upsert within its scope
- **WHEN** a key is injected, then upserted on the same context, and `Use()` is
  called with the upserted context
- **THEN** the upserted value is returned
