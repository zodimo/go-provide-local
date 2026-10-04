// Package plocal provides a blazing-fast, type-safe Context Registry Tree for Go 1.18+.
//
// # Overview
//
// plocal brings the elegance of React's Context API (or Jetpack Compose's
// CompositionLocal) to Go's [context.Context]. It solves two core problems with
// the standard [context.WithValue] approach:
//
//  1. No type safety — ctx.Value() returns any, forcing boilerplate type
//     assertions that bypass the compiler and can panic at runtime.
//  2. Performance — deeply nested standard context nodes increase GC pressure
//     and slow down value lookups.
//
// # The Registry Tree
//
// Instead of wrapping the context multiple times, plocal injects a single
// "fast-lane" linked list (a Prototype Chain) into the standard context. Each
// call to [ProvideAll] allocates exactly one lightweight [registryNode] and
// links it to the parent — no map copying, no GC spikes.
//
// Value lookups via [Use] traverse only the injected dependency nodes, skipping
// all standard library timeout/cancellation wrappers entirely. A node's entry
// storage is immutable after construction, so [Use] takes no lock: each step is
// a lock-free pointer compare over a pointer-keyed slice.
//
// # Quick Start
//
// Define a typed key with a fallback default:
//
//	var RequestIDKey = plocal.NewResourceKey[string]("unknown-req-id")
//
// Inject state into a new scope with [ProvideAll]:
//
//	plocal.ProvideAll(ctx, []plocal.Provider{
//	    plocal.Value(RequestIDKey, "req-abc-123"),
//	}, func(scopedCtx context.Context) {
//	    // use scopedCtx inside the closure
//	    handleRequest(scopedCtx)
//	})
//
// Retrieve the typed value anywhere downstream with [Use]:
//
//	id := plocal.Use(ctx, RequestIDKey) // type: string, no assertion needed
//
// # Context-Returning Injection
//
// [ProvideAll] (and its single-value wrapper [Provide]) evaluate a consumer
// closure and automatically release the scope when that closure returns:
//
//	plocal.ProvideAll(ctx, providers, func(scopedCtx context.Context) {
//	    // scope lives only for the duration of this closure
//	})
//
// [WithProvider] and [WithProviders] are the counterpart that returns the
// enriched [context.Context] directly instead of taking a consumer closure:
//
//	scopedCtx := plocal.WithProvider(ctx, RequestIDKey, "req-abc-123")
//	scopedCtx = plocal.WithProviders(scopedCtx, providers)
//
// [UpdateProvider] and [UpdateProviders] amend the current scope's node. They
// are copy-on-upsert: a new node is constructed and the context you pass in is
// left unmodified, so upsert never mutates state a concurrent reader may be
// traversing. Use the returned context — it is the one that reflects the
// change.
//
// The scope-lifetime difference matters. A closure scope is released the
// instant the consumer returns and can never outlive it. A context returned by
// [WithProvider]/[WithProviders] lives as long as the caller keeps it — it is
// the caller's responsibility to stop using it. Reach for the context-returning
// form when the enriched context must escape the point where it is built (for
// example, to stash a provider on a request context that a host framework hands
// to arbitrary downstream code), and for the closure form when the scope is
// strictly lexical.
//
// # Key Properties
//
//   - Type-safe: built on Go 1.18 generics; no manual type assertions.
//   - Zero-copy scoping: [ProvideAll] with N keys creates exactly one node.
//   - Zero-alloc reads: [Use] never touches the heap.
//   - Collision-proof: keys are pointer values, making cross-package key
//     collisions mathematically impossible.
//
// # Anti-Pattern Warning
//
// Do not use plocal (or context.Context in general) to pass application-wide
// core dependencies such as database connection pools or domain repositories.
// This creates a Service Locator anti-pattern. Use explicit struct fields for
// primary domain dependencies.
package plocal
