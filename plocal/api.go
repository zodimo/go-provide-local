package plocal

import (
	"context"
)

// Provider is implemented by any value that can apply itself to the internal
// registry map of a new scope node. Use [Value] to create a Provider, and pass
// one or more Providers to [ProvideAll] or [Provide] to inject them into the
// context.
type Provider interface {
	apply(m map[any]any)
}

// Value creates a [Provider] that binds val to key. The returned Provider is
// consumed by [Provide] or [ProvideAll]; it has no effect until passed to one
// of those functions.
//
// Example:
//
//	p := plocal.Value(ThemeKey, DarkTheme)
//	plocal.ProvideAll(ctx, []plocal.Provider{p}, func(ctx context.Context) { ... })
func Value[T any](key *ResourceKey[T], val T) Provider {
	return providerImpl[T]{
		key: key,
		val: val,
	}
}

// ProvideAll injects all providers into a new lexically-scoped registry node
// and calls consumer with the enriched context. The new node points to the
// parent node (if any), forming the Prototype Chain used by [Use].
//
// All N providers are stored in a single new node — [ProvideAll] always
// increments the registry depth by exactly 1, regardless of how many providers
// are supplied. The scope is automatically released when consumer returns.
//
// T is the return type of consumer, allowing ProvideAll to be composed in
// expression position.
//
// Example — HTTP middleware:
//
//	providers := []plocal.Provider{
//	    plocal.Value(TraceIDKey, traceID),
//	    plocal.Value(UserRoleKey, role),
//	}
//	plocal.ProvideAll(r.Context(), providers, func(ctx context.Context) {
//	    next.ServeHTTP(w, r.WithContext(ctx))
//	})
func ProvideAll[T any](c context.Context, providers []Provider, consumer func(ctx context.Context) T) T {

	// Create a map just for the new providers at this level
	localVals := make(map[any]any, len(providers))
	for _, p := range providers {
		p.apply(localVals)
	}

	// Create the new tree node
	node := &registryNode{
		values: localVals,
	}

	// If a parent node exists, link to it (The Prototype Chain)
	if parentNode, ok := c.Value(registryKey).(*registryNode); ok {
		node.parent = parentNode
	}

	// Wrap the context once with our new leaf node
	localCtx := context.WithValue(c, registryKey, node)

	return consumer(localCtx)
}

// Provide is a convenience wrapper around [ProvideAll] for injecting a single
// key/value pair into a new scope. It is equivalent to:
//
//	plocal.ProvideAll(ctx, []plocal.Provider{plocal.Value(key, val)}, consumer)
//
// Use [ProvideAll] directly when injecting multiple values to avoid creating
// one node per key.
func Provide[T, U any](c context.Context, key *ResourceKey[T], val T, consumer func(c context.Context) U) U {
	return ProvideAll(c, []Provider{Value(key, val)}, consumer)
}

// Use retrieves the typed value associated with key from the nearest enclosing
// scope in the registry tree. It walks the Prototype Chain — from the current
// leaf node up to the root — and returns the first matching value it finds.
//
// If key was never injected into any ancestor scope, Use returns the key's
// default fallback (set when the key was created via [NewResourceKey]).
//
// Use never allocates heap memory and is safe to call concurrently from
// multiple goroutines sharing the same context.
//
// Example:
//
//	theme := plocal.Use(ctx, ThemeKey) // type: Theme — no assertion needed
func Use[T any](c context.Context, key *ResourceKey[T]) T {

	// Find the leaf node in the standard context
	if node, ok := c.Value(registryKey).(*registryNode); ok {

		// Walk up our fast-lane registry tree
		for curr := node; curr != nil; curr = curr.parent {
			if val, exists := curr.values[key]; exists {
				return val.(T)
			}
		}
	}

	return key.Default
}
