# go-provide-local

A blazing-fast, type-safe Context Registry Tree for Go 1.18+.

`go-provide-local` brings the elegance of React's Context API (or Jetpack Compose's `CompositionLocal`) to Go's `context.Context`. It solves the biggest architectural flaws of standard `context.WithValue`: the lack of compiler type safety, and the massive performance penalty of deeply nested standard context nodes.

## The Problem

Relying on standard `context.WithValue` introduces significant friction:

1. **No Type Safety:** `ctx.Value()` returns `any`. You are forced to write boilerplate type assertions that bypass the compiler and can panic at runtime.
2. **The Garbage Collection Chokehold:** Copying large `map[string]any` registries every time you want to add a scoped variable puts immense pressure on the Go garbage collector.
3. **The Slow Traversal Penalty:** Every time you call `context.WithValue`, Go creates a new node. Reading a value requires traversing through timeouts, cancellations, and HTTP request data one interface assertion at a time.
4. **Key Collisions:** Using strings as context keys can lead to silent overwrites across different packages.

## The Solution: The Registry Tree

`go-provide-local` completely bypasses map-copying and bloated context traversal by implementing a **Lexically Scoped Registry Tree** (a Prototype Chain).

Instead of wrapping the context multiple times, it injects a single "Fast-Lane" linked list into the standard context.

* **Type-Safe:** Built purely on Go 1.18 Generics. No `any`, no manual type assertions.
* **Zero-Copy Scoping:** Creating a new scope (`ProvideAll`) allocates exactly one lightweight node and points it at the parent. Zero map copying, zero GC spikes.
* **Fast-Lane Lookups:** `Use()` traverses *only* your injected dependency nodes, completely skipping standard library timeouts and cancellations.
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
	// Type-safe, O(Depth) traversal through ONLY your injected nodes
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

A convenience wrapper for injecting a single value into a new scope.

### `ProvideAll[T any](ctx context.Context, providers []Provider, consumer func(ctx context.Context) T) T`

Injects multiple providers into the context simultaneously. This creates a single new node in the Registry Tree pointing to the parent scope. Highly optimized for rapid scope creation without map-copying allocations.

### `WithProvider[T any](ctx context.Context, key *ResourceKey[T], val T) context.Context`

Injects a single key/value pair and returns the enriched `context.Context` directly. Unlike `Provide`, it does not take a consumer closure — the caller owns the returned context.

### `WithProviders(ctx context.Context, providers []Provider) context.Context`

Injects multiple providers at once and returns the enriched `context.Context` directly, creating one Registry Tree node for the whole batch.

### `Use[T any](ctx context.Context, key *ResourceKey[T]) T`

Retrieves the typed value from the nearest node in the registry tree. Returns the key's default fallback value if the key does not exist in the prototype chain.

> **Closure-scoped vs derived-context injection.** `Provide`/`ProvideAll` take a consumer closure and **auto-release the scope** when that closure returns — the enriched context can never outlive it. `WithProvider`/`WithProviders` return a derived `context.Context` whose lifetime is the **caller's responsibility**. Use the closure form for strictly lexical scopes; use `WithProvider`/`WithProviders` when the enriched context must escape the point where it is built (for example, stashing a provider on a request context that a host framework hands to arbitrary downstream code).

## Ideal Use Cases

`go-provide-local` is designed for **highly-scoped environmental data**.

🟢 **Immediate Mode GUIs & Declarative UIs:** The zero-cost scoping makes it the perfect vehicle for passing Themes, Fonts, Window Bounds, or routing data down a massive UI tree rendering at 60 FPS.

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
Regenerate with `make bench-mem` (5s/bench). Absolute ns/op varies with machine
and thermal state — the plocal-vs-stdlib *ratios within one run* are the
load-bearing evidence; the absolute figures are advisory.

**Lookup: `plocal.Use()` vs stdlib `context.Value()`**

Both implementations read the sought key from the root of an identically shaped
chain of depth N.

| Depth | plocal `Use()` ns/op | stdlib `ctx.Value()` ns/op | plocal allocs/op | stdlib allocs/op |
|---|---|---|---|---|
| 1 | 26 | **8** | 0 | 0 |
| 10 | 132 | **53** | 0 | 0 |
| 100 | 1170 | **441** | 0 | 0 |

**Scope creation (1 provider injected)**

| Implementation | ns/op | B/op | allocs/op |
|---|---|---|---|
| plocal `ProvideAll()` at depth 1 | 343 | 440 | 6 |
| plocal `ProvideAll()` at depth 100 | 345 | **440** | **6** ← same as depth 1 |
| stdlib `context.WithValue` at depth 1 | **39** | **48** | **1** |

**Depth scaling (`Use()`)**

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Use()` depth 1 | 26 | 0 | **0** |
| `Use()` depth 10 | 132 | 0 | **0** |
| `Use()` depth 100 | 1213 | 0 | **0** |

**Key findings:**

- ⚠️ **stdlib wins raw single-key lookup speed.** For a worst-case single-key
  lookup, `context.Value()` is faster at every depth (~3x at depth 1, ~2.5x at
  depth 10, ~2.7x at depth 100). Each plocal step is a `map[any]any` probe; each
  stdlib step is a pointer comparison + type switch. `Use()` is *not*
  unconditionally faster than `context.Value()`, and this project no longer
  claims it is.
- ✅ **plocal wins type safety.** `Use()` is fully generic — no `any`, no manual
  type assertions, no runtime panics from a bad assertion.
- ✅ **plocal wins zero-alloc reads.** `Use()` allocates **0 bytes at every
  depth** — the fast-lane traversal never touches the heap.
- ✅ **plocal wins scope creation asymptotically.** `ProvideAll` is O(providers),
  not O(depth): injecting N keys creates **one** node (6 allocs, 440 B) whether
  the chain is 1 or 100 deep, while N sequential `context.WithValue` wraps cost
  one allocation each. stdlib is cheaper for a *single* wrap (1 alloc vs 6); the
  advantage reverses as providers accumulate in one scope.
- ✅ **`Use()` lookup scales linearly** with depth (O(depth)), not geometrically.