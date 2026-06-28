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
// all standard library timeout/cancellation wrappers entirely.
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
