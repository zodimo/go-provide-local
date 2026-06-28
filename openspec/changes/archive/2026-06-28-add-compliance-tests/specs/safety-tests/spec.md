## ADDED Requirements

### Requirement: Use never panics on any valid context input
`Use()` SHALL never panic regardless of the context passed in — including empty contexts, contexts with no plocal nodes, and contexts modified only by standard library functions.

#### Scenario: Use on background context does not panic
- **WHEN** `Use()` is called with `context.Background()` and any `ResourceKey`
- **THEN** no panic occurs and the fallback value is returned

#### Scenario: Use on cancelled context does not panic
- **WHEN** `Use()` is called on a cancelled `context.WithCancel` context
- **THEN** no panic occurs and the fallback value is returned

#### Scenario: Use on context with timeout does not panic
- **WHEN** `Use()` is called on a `context.WithTimeout` context
- **THEN** no panic occurs and the fallback value is returned

### Requirement: Keys from independent NewResourceKey calls never collide
Two `*ResourceKey[T]` values created by separate `NewResourceKey` calls SHALL resolve independently, even if they share the same type and default value — simulating keys from two different packages.

#### Scenario: Two independent string keys with same default do not cross-contaminate
- **WHEN** two keys are created with `NewResourceKey[string]("default")` and one is injected with a distinct value
- **THEN** `Use()` for the uninjected key returns its own default, not the other key's value

#### Scenario: Injecting one key does not affect a sibling key of the same type
- **WHEN** both keys are injected with different values in the same `ProvideAll` call
- **THEN** each key resolves to its own value independently

### Requirement: Goroutine read safety — concurrent Use on a shared context is safe
`Use()` on a shared, read-only context MUST be safe to call concurrently from multiple goroutines without data races.

#### Scenario: Multiple goroutines read from the same scoped context concurrently
- **WHEN** a context with injected values is shared across N goroutines and each calls `Use()` simultaneously
- **THEN** all goroutines receive the correct value with no data race (verified with `-race` flag)
