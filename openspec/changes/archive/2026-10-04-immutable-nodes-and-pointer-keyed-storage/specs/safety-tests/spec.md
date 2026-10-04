## MODIFIED Requirements

### Requirement: Goroutine read safety — concurrent Use on a shared context is safe
`Use()` on a shared, read-only context MUST be safe to call concurrently from
multiple goroutines without data races. This guarantee SHALL hold *by
construction*: node entry storage is immutable after the node is constructed,
so no read-time lock is taken and no reader can observe a partially written
node. The guarantee SHALL NOT depend on a lock being held by readers.

#### Scenario: Multiple goroutines read from the same scoped context concurrently
- **WHEN** a context with injected values is shared across N goroutines and each
  calls `Use()` simultaneously
- **THEN** all goroutines receive the correct value with no data race (verified
  with the `-race` flag)

## ADDED Requirements

### Requirement: Concurrent upsert against shared context is race-free without mutating shared node state
`UpdateProvider` and `UpdateProviders` MUST be safe to call concurrently with
`Use()` on a context that is already shared, without data races and without
mutating any node a concurrent reader may be traversing. Upsert SHALL produce a
newly constructed node reflecting the amended entries, leaving the prior node's
storage untouched.

#### Scenario: Concurrent upsert and read on a shared parent
- **WHEN** one goroutine repeatedly calls `UpdateProvider` on a context while
  other goroutines concurrently call `Use()` on the same context
- **THEN** no data race is reported under the `-race` flag
- **AND** every `Use()` call returns either the pre-upsert or post-upsert value
  for the key, never a corrupted or partially written value

#### Scenario: Upsert does not mutate the node it derived from
- **WHEN** a context is derived from a parent that already holds a node, and
  `UpdateProvider` is called on the derived context
- **THEN** the parent context's node still resolves the original value
- **AND** the derived context resolves the updated value
