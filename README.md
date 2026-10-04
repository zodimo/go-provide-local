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
* **Flat in Width:** Injecting N values for a scope creates **one** node, so a read stays flat as N grows (up to a per-node soft cap of ~16 values) — where N separate `context.WithValue` wraps give N levels and linearly slower reads. See [Benchmark Results](#benchmark-results).
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

Injects multiple providers into the context simultaneously. This creates a single new node in the Registry Tree pointing to the parent scope, holding all N values — so scope creation is O(providers), not O(providers) nodes. The node's entry storage is sized to the provider count at construction, and a read over it stays flat as the number of values grows (up to the per-node soft cap of ~16 values).

### `WithProvider[T any](ctx context.Context, key *ResourceKey[T], val T) context.Context`

Injects a key/value pair and returns the enriched `context.Context` directly. Unlike `Provide`, it does not take a consumer closure — the caller owns the returned context.

If the current scope already exists and does not yet contain `key`, the pair is **merged into that scope** (no new level) — so a run of `WithProvider` calls for distinct keys stays at one registry level. If `key` is already present, a new shadowing node is pushed so the override is scoped. When no registry node exists yet, a new node is created. Merging is **copy-on-upsert**: a new node is constructed and the node on the given context is left unmodified.

### `WithProviders(ctx context.Context, providers []Provider) context.Context`

Injects multiple providers at once and returns the enriched `context.Context` directly, creating one Registry Tree node for the whole batch. The node's entry storage is sized to the provider count at construction.

### `UpdateProvider(ctx context.Context, provider Provider) context.Context`

Upserts a single provider into the **current** scope if one exists, returning a context whose leaf is a **newly constructed node** carrying the amended entry set. No node is mutated, and no new registry level is added. If no registry node exists yet, it creates one via `WithProviders`.

### `UpdateProviders(ctx context.Context, providers []Provider) context.Context`

Upserts several providers into the current scope in one call, with the same semantics as `UpdateProvider`.

> **Upsert is copy-on-upsert, not in-place mutation.** `UpdateProvider`/`UpdateProviders` build a new node and return a new context — the context you pass in is left exactly as it was, so it stays safe to read concurrently and its value never changes underneath other holders. **Use the returned context**; the one you passed does not reflect the upsert. Prefer `Provide`/`ProvideAll` for scope-local injection; reach for `Update*` when you deliberately want to amend a scope and thread the amended context onward.

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

Measured on Intel Core Ultra 9 185H, Go 1.27, linux/amd64, GOMAXPROCS=22, after
node storage was made immutable and pointer-keyed (regenerated from one run with
`make bench-mem`). Absolute ns/op varies with machine and thermal state — the
ratios within one run, and the alloc/level counts, are the load-bearing
evidence; the absolute figures are advisory.

Two independent axes are measured below. Reading one as the other is the mistake
this section exists to prevent.

#### Axis 1 — Width: N values for ONE scope (the library's headline)

Batching N values into a single scope (`WithProviders` / `ProvideAll`) stores
them in **one node**, so a read stays **flat in N** up to the per-node soft cap.
Spreading the same N values across N scopes creates N nodes, so reads grow
**linearly in N**. Both shapes read the outermost-injected key (worst case).

| Values (N) | Batched: 1 node, `Use()` ns/op | Spread: N scopes, `Use()` ns/op | Batched levels | Spread levels |
|---|---|---|---|---|
| 1 | 12.6 | 12.0 | 1 | 1 |
| 4 | 12.7 | 21.3 | 1 | 4 |
| 8 | 12.7 | 30.2 | 1 | 8 |
| 16 | 13.1 | 67.1 | 1 | 16 |
| 32 | **12.6** | **107.8** | **1** | **32** |

> **Per-node soft cap (~16 values).** Node storage is a pointer-keyed slice
> scanned linearly, so the **worst-case** read *inside a single node* grows with
> that node's entry count — while reading an entry stored early stays ~12 ns
> regardless of N. Measured worst case (wanted entry last) vs best case (first):
>
> | Entries in one node | 4 | 8 | 16 | 32 | 64 |
> |---|---|---|---|---|---|
> | worst case (last) | 17 ns | 18 ns | 25 ns | 55 ns | 96 ns |
> | best case (first) | 12 ns | 13 ns | 12 ns | 12 ns | 12 ns |
>
> Beyond roughly **16 values per node** the worst-case read grows with the count.
> For very wide scopes, split them into nested scopes — that stays linear in
> depth but with a much smaller constant. The batched-read column above reads a
> key stored near the front, which is why it stays flat to N=32.

#### Injection cost to build those N values (N = 32)

| Strategy | ns/op | B/op | allocs/op | context/registry levels |
|---|---|---|---|---|
| plocal `WithProviders(ctx, 32)` | 651 | 1232 | **3** | **1** |
| plocal 32× `WithProviders` (spread) | 3832 | 3584 | 96 | 32 |
| plocal 32× `UpdateProvider` (upsert) | 6339 | 6112 | 128 | 1 |
| stdlib 32× `context.WithValue` | **1359** | **1536** | 32 | 32 |

> The upsert row is measured with each call's returned context threaded into the
> next (the natural accumulating shape), so the Nth call walks an N-deep chain to
> find its leaf before copying — which is why its per-op figure grows with N and
> is not a flat per-call cost. As a single isolated call, upsert is **~136 ns
> into a 1-entry node** and **~561 ns into a 32-entry node**, 3 allocs/op each
> and flat in chain depth, because it always targets the leaf.

#### Axis 2 — Depth: one value per nested scope

When each value gets its own scope, both plocal and stdlib are linear in the
number of levels — and **plocal is now cheaper than stdlib at depth 10 and 100**:

| Depth (nodes) | plocal `Use()` ns/op | stdlib `ctx.Value()` ns/op | plocal allocs/op | stdlib allocs/op |
|---|---|---|---|---|
| 1 | 13.1 | **8.8** | 0 | 0 |
| 10 | **44.1** | 52.2 | 0 | 0 |
| 100 | **286.8** | 456.5 | 0 | 0 |

#### Scope creation (1 provider injected)

| Implementation | ns/op | B/op | allocs/op |
|---|---|---|---|
| plocal `ProvideAll()` at depth 1 | 169 | 152 | 5 |
| plocal `ProvideAll()` at depth 100 | 174 | **152** | **5** ← same as depth 1 |
| stdlib `context.WithValue` at depth 1 | **39** | **48** | **1** |

**Key findings:**

- ✅ **plocal wins on width: reads stay flat as one scope holds more values.**
  Injecting N values for a scope is **one node**, so `Use()` is ~12.6 ns whether
  N is 1 or 32. Spreading the same values across N scopes makes reads grow
  linearly (12 → 108 ns) and forces N levels. This is the axis the library is
  built around: **inject related values together with `ProvideAll`, not one call
  per value.** (Mind the per-node soft cap above.)
- ✅ **plocal wins zero-alloc reads.** `Use()` allocates **0 bytes at every
  width and depth**, and takes no lock.
- ✅ **plocal now wins on depth too.** `Use()` is faster than `context.Value()`
  at depth 10 (44 vs 52 ns) and depth 100 (287 vs 457 ns). Because nodes are
  immutable, each step is a lock-free pointer compare rather than a lock-guarded
  `map[any]any` probe — removing that cost flipped this axis.
- ✅ **Batching beats sequential injection within plocal.** One
  `WithProviders(..., 32)` costs 3 allocs / 1 level and keeps reads flat, versus
  96 allocs / 32 levels for 32 separate `WithProviders` calls (≈6× the time and
  32× the allocs). Use `ProvideAll`/`WithProviders` for values that share a
  scope.
- ✅ **plocal wins type safety.** `Use()` is fully generic — no `any`, no manual
  type assertions, no runtime panics from a bad assertion.
- ⚠️ **stdlib still wins raw single-value scope creation, and depth-1 lookup.**
  `context.WithValue` costs 48 B / 1 alloc versus a registry node's 152 B /
  5 allocs, and at depth 1 its single wrap (8.8 ns) is marginally cheaper than
  locating the plocal leaf via `c.Value(registryKey)` (13.1 ns). plocal's
  advantage is the **shape** it buys — one level, a bounded alloc count, flat
  reads — plus its now-real depth advantage once the chain is more than a couple
  of levels deep.
- ⚠️ **Keep a single scope's value count small.** Past ~16 values in one node,
  the worst-case read inside that node grows linearly (see the soft cap table).
  Nested scopes avoid the wide-single-node case.

### Superseded decisions

This result set supersedes the `sync.RWMutex` decision made in the change
`fix-benchmark-truthfulness-and-injection-cost`. That change added a per-node
read-write mutex so in-place upsert could not race a concurrent `Use` — a correct
fix for the race it addressed, but it put a lock on the read path and kept a
`map[any]any` per node. Both are gone: nodes are now **immutable after
construction** and store a **pointer-keyed slice**, so reads are lock-free and
upsert is copy-on-upsert. The concurrency guarantee is unchanged in force but now
holds **by construction** rather than by locking, and the depth conclusion is
reversed: plocal is no longer slower than stdlib at every depth. This also
supersedes the depth disclosure in the archived
`document-platform-example-and-stdlib-benchmarks`.