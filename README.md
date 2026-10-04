# go-provide-local

A type-safe Context Registry Tree for Go 1.18+.

`go-provide-local` brings the elegance of React's Context API (or Jetpack Compose's `CompositionLocal`) to Go's `context.Context`. It solves the biggest architectural flaw of standard `context.WithValue`: the lack of compiler type safety — and it makes injecting *many* scoped values cheap, by storing a whole set of them in **one** node instead of one context level per value.

## The Problem

Relying on standard `context.WithValue` introduces significant friction:

1. **No Type Safety:** `ctx.Value()` returns `any`. You are forced to write boilerplate type assertions that bypass the compiler and can panic at runtime.
2. **One Level Per Value:** Every `context.WithValue` call adds a context level. Injecting N scoped values means N levels, N allocations, and a read whose cost grows linearly with N.
3. **The Slow Traversal Penalty:** Reading a value traverses through timeouts, cancellations, and HTTP request data one interface assertion at a time.
4. **Key Collisions:** Using strings as context keys can lead to silent overwrites across different packages.

## The Solution: The Registry Tree

`go-provide-local` bypasses map-copying and per-value context wraps by implementing a **Lexically Scoped Registry Tree** (a Prototype Chain).

Instead of wrapping the context once per value, it injects a single "Fast-Lane" linked list into the standard context, and a whole set of values for one scope goes into **one node**.

* **Type-Safe:** Built purely on Go 1.18 Generics. No `any`, no manual type assertions.
* **Zero-Copy Scoping:** Creating a scope (`ProvideAll`) allocates one lightweight node and points it at the parent. Zero map copying, zero GC spikes.
* **Flat in Width:** Injecting N values for a scope creates **one** node, so a read stays flat as N grows — where N separate `context.WithValue` wraps give N levels and linearly slower reads. See [Benchmark Results](#benchmark-results).
* **Collision-Proof:** Uses pointer memory addresses for context keys, making cross-package collisions mathematically impossible.

## Installation

```bash
go get github.com/zodimo/go-provide-local/plocal

```

## Quick Start

### 1. Define your Keys

Define your resource keys as package-level variables. You must provide a fallback/default value.

```go
package state

import "github.com/zodimo/go-provide-local/plocal"

// The default is returned if the context doesn't contain the value.
var RequestIDKey = plocal.NewResourceKey[string]("unknown-req-id")
var ThemeKey = plocal.NewResourceKey[Theme](DefaultTheme)

```

### 2. Injecting State (`ProvideAll`)

Use `ProvideAll` to inject multiple dependencies into a new lexical scope. This creates a lightweight tree node and evaluates your callback.

```go
package ui

import (
	"context"
	"github.com/zodimo/go-provide-local/plocal"
	"myapp/state"
)

func RenderApp(ctx context.Context) {
	// Inject dependencies safely
	providers := []plocal.Provider{
		plocal.Value(state.ThemeKey, DarkTheme),
	}

	// ProvideAll pushes the state onto the registry tree
	plocal.ProvideAll(ctx, providers, func(scopedCtx context.Context) {
		
		// Everything inside this closure uses the DarkTheme
		RenderSidebar(scopedCtx)
		RenderBody(scopedCtx)

		// When this closure ends, scopedCtx goes out of scope. 
		// The state is automatically "popped" without manual cleanup!
	})
}

```

### 3. Consuming State (`Use`)

Downstream functions can safely extract the typed values. The lookup walks up your fast-lane registry tree, ignoring all standard context bloat.

```go
package ui

import (
	"context"
	"github.com/zodimo/go-provide-local/plocal"
	"myapp/state"
)

func RenderSidebar(ctx context.Context) {
	// 100% Type-Safe. No type assertions required.
	// Walks up the prototype chain to find the nearest injected Theme.
	theme := plocal.Use(ctx, state.ThemeKey)

	DrawRect(theme.BackgroundColor)
}

```

### 4. HTTP Middleware (Backend Use Case)

The Registry Tree is perfectly optimized for HTTP middleware. It allows you to inject request-scoped data (like User IDs or Trace spans) sequentially down the handler chain without the heavy garbage-collection penalty of copying maps.

```go
package middleware

import (
	"context"
	"net/http"
	"github.com/zodimo/go-provide-local/plocal"
)

// Define your keys
var TraceIDKey = plocal.NewResourceKey[string]("no-trace")
var UserRoleKey = plocal.NewResourceKey[string]("guest")

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		
		// 1. Gather your request-scoped state
		traceID := generateTraceID()
		role := determineUserRole(r)

		providers := []plocal.Provider{
			plocal.Value(TraceIDKey, traceID),
			plocal.Value(UserRoleKey, role),
		}

		// 2. ProvideAll pushes a new node onto the request's Registry Tree
		plocal.ProvideAll(r.Context(), providers, func(ctx context.Context) {
			// 3. Pass the enriched context to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
}

// Downstream Handler
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Type-safe, no assertion needed; walks only your injected registry nodes
	role := plocal.Use(r.Context(), UserRoleKey)
	
	if role == "guest" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	w.Write([]byte("Welcome to your profile"))
}

```

## API Reference

### `NewResourceKey[T any](fallback T) *ResourceKey[T]`

Creates a collision-proof key for a specific type `T`. The `fallback` value is returned if `Use` is called on a context that does not contain the key.

### `Value[T any](key *ResourceKey[T], val T) Provider`

Creates a `Provider` that binds a specific value to a key, ready to be injected via `Provide` or `ProvideAll`.

### `Provide[T, U any](ctx context.Context, key *ResourceKey[T], val T, consumer func(ctx context.Context) U) U`

Injects a single value into a new lexical scope and evaluates the consumer. Equivalent to `ProvideAll` with one provider: it creates one registry node, so depth increases by one. Prefer `ProvideAll` when injecting several values at once — that keeps them in a single node.

### `ProvideAll[T any](ctx context.Context, providers []Provider, consumer func(ctx context.Context) T) T`

Injects multiple providers into the context simultaneously. This creates a single new node in the Registry Tree pointing to the parent scope, holding all N values — so scope creation is O(providers), not O(providers) nodes. Highly optimized for injecting many values without map-copying allocations, and it is what keeps read cost flat as the number of values grows.

### `WithProvider[T any](ctx context.Context, key *ResourceKey[T], val T) context.Context`

Injects a key/value pair and returns the enriched `context.Context` directly. Unlike `Provide`, it does not take a consumer closure — the caller owns the returned context.

If the current scope already exists and does not yet contain `key`, the pair is **merged into that scope's node in place** (no new level) — so a run of `WithProvider` calls for distinct keys stays at one registry level. If `key` is already present, a new shadowing node is pushed so the override is scoped. When no registry node exists yet, a new node is created.

### `WithProviders(ctx context.Context, providers []Provider) context.Context`

Injects multiple providers at once and returns the enriched `context.Context` directly, creating one Registry Tree node for the whole batch.

### `UpdateProvider(ctx context.Context, provider Provider) context.Context`

Upserts a single provider into the **current** scope's node if one exists, mutating that node in place (no new registry level) and returning the same context. If no registry node exists yet, it creates one via `WithProviders`.

### `UpdateProviders(ctx context.Context, providers []Provider) context.Context`

Upserts several providers into the current scope's node in one call, with the same semantics as `UpdateProvider`.

> **Upsert mutates a live context.** `UpdateProvider`/`UpdateProviders` write into a node that may already have been shared with other goroutines or scopes. The write is lock-guarded (so it is race-free against concurrent `Use`), but the *value* of the key changes for every holder of that context. Prefer `Provide`/`ProvideAll` for scope-local injection; reach for `Update*` only when you deliberately intend to amend a live scope.

### `Use[T any](ctx context.Context, key *ResourceKey[T]) T`

Retrieves the typed value from the nearest node in the registry tree. Returns the key's default fallback value if the key does not exist in the prototype chain.

> **Closure-scoped vs derived-context injection.** `Provide`/`ProvideAll` take a consumer closure and **auto-release the scope** when that closure returns — the enriched context can never outlive it. `WithProvider`/`WithProviders`/`UpdateProvider`/`UpdateProviders` return a derived `context.Context` whose lifetime is the **caller's responsibility**. Use the closure form for strictly lexical scopes; use the derived-context form when the enriched context must escape the point where it is built (for example, stashing a provider on a request context that a host framework hands to arbitrary downstream code).

## Ideal Use Cases

`go-provide-local` is designed for **highly-scoped environmental data**.

🟢 **Immediate Mode GUIs & Declarative UIs:** Batching a scope's Theme, Font, Window Bounds, and routing data into a single `ProvideAll` node keeps lookups flat as the UI tree grows, rather than paying one context level per value.

🟢 **HTTP Middleware:** Extracting JWT claims, Correlation IDs, or Feature Flags and passing them cleanly to downstream handlers.

🟢 **Contextual Logging/Tracing:** Passing trace spans or logger instances decorated with request-scoped fields.

**Anti-Pattern Warning:** Do not use this package (or `context.Context` in general) to pass application-wide core dependencies like Database connection pools or domain repositories. This creates a "Service Locator" anti-pattern, making function signatures dishonest and tests difficult to write. Explicit struct fields remain the idiomatic Go approach for primary domain dependencies.

## Testing & Benchmarks

The library ships with a compliance test suite that verifies every documented claim. Run it with:

```bash
make test        # correctness + safety tests
make test-race   # same, with Go race detector
make bench-mem   # performance benchmarks with allocation reporting
make test-all    # race tests then full benchmark run
```

### Benchmark Results

Measured on Intel Core Ultra 9 185H, Go 1.27, linux/amd64, GOMAXPROCS=22.
Regenerate with `make bench-mem`. Absolute ns/op varies with machine and thermal
state — the ratios within one run, and the alloc/level counts, are the
load-bearing evidence; the absolute figures are advisory.

Two independent axes are measured below. Reading one as the other is the mistake
this section exists to prevent.

#### Axis 1 — Width: N values for ONE scope (the library's headline)

Batching N values into a single scope (`WithProviders` / `ProvideAll`) stores
them in **one node**, so a read stays **flat in N**. Spreading the same N values
across N scopes creates N nodes, so reads grow **linearly in N**. Both shapes
read the outermost-injected key (worst case).

| Values (N) | Batched: 1 node, `Use()` ns/op | Spread: N scopes, `Use()` ns/op | Batched levels | Spread levels |
|---|---|---|---|---|
| 1 | 37 | 37 | 1 | 1 |
| 4 | 37 | 112 | 1 | 4 |
| 8 | 38 | 213 | 1 | 8 |
| 16 | 37 | 429 | 1 | 16 |
| 32 | **37** | **859** | **1** | **32** |

#### Injection cost to build those N values (N = 32)

| Strategy | ns/op | B/op | allocs/op | context/registry levels |
|---|---|---|---|---|
| plocal `WithProviders(ctx, 32)` | 2079 | 2488 | **6** | **1** |
| plocal 32× `WithProviders` (spread) | 9519 | 13824 | 128 | 32 |
| plocal 32× `UpdateProvider` (upsert) | 2896 | 512 | 32 | 1 |
| stdlib 32× `context.WithValue` | **1418** | **1536** | 32 | 32 |

#### Axis 2 — Depth: one value per nested scope (disclosed, not the headline)

When each value gets its own scope, both plocal and stdlib are linear in the
number of levels — and **stdlib is cheaper at every depth**:

| Depth (nodes) | plocal `Use()` ns/op | stdlib `ctx.Value()` ns/op | plocal allocs/op | stdlib allocs/op |
|---|---|---|---|---|
| 1 | 37 | **8** | 0 | 0 |
| 10 | 256 | **54** | 0 | 0 |
| 100 | 2554 | **448** | 0 | 0 |

#### Scope creation (1 provider injected)

| Implementation | ns/op | B/op | allocs/op |
|---|---|---|---|
| plocal `ProvideAll()` at depth 1 | 363 | 472 | 6 |
| plocal `ProvideAll()` at depth 100 | 354 | **472** | **6** ← same as depth 1 |
| stdlib `context.WithValue` at depth 1 | **39** | **48** | **1** |

**Key findings:**

- ✅ **plocal wins on width: reads stay flat as one scope holds more values.**
  Injecting N values for a scope is **one node**, so `Use()` is ~37 ns whether
  N is 1 or 32. Spreading the same values across N scopes makes reads grow
  linearly (37 → 859 ns) and forces N levels. This is the axis the library is
  built around, and the one to optimize for: **inject related values together
  with `ProvideAll`, not one call per value.**
- ✅ **plocal wins zero-alloc reads.** `Use()` allocates **0 bytes at every
  width and depth** — the registry walk never touches the heap.
- ✅ **Batching beats sequential injection within plocal.** One
  `WithProviders(…, 32)` costs 6 allocs / 1 level and keeps reads flat, versus
  128 allocs / 32 levels for 32 separate `WithProviders` calls (5×+ the time and
  20×+ the allocs). Use `ProvideAll`/`WithProviders` for values that share a
  scope.
- ✅ **plocal wins type safety.** `Use()` is fully generic — no `any`, no manual
  type assertions, no runtime panics from a bad assertion.
- ⚠️ **stdlib wins raw single-key lookup speed at every depth.** For a
  worst-case read, `context.Value()` is faster at depth 1 (8 vs 37 ns), depth 10
  (54 vs 256 ns) and depth 100 (448 vs 2554 ns). Each plocal step is a
  lock-guarded `map[any]any` probe plus a pointer hop; each stdlib step is a
  pointer compare + type switch. `Use()` is *not* faster than `context.Value()`
  for a deep chain of single-value scopes, and this project does not claim it is.
- ⚠️ **stdlib also wins raw injection ns/bytes.** A single `context.WithValue`
  is cheaper than creating a registry node (48 B/1 alloc vs ~472 B/6 allocs).
  plocal's injection advantage is the **shape** it buys — one level, a bounded
  alloc count, and flat reads — not raw nanoseconds or bytes.