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
	// Wrap the context once with our new leaf node
	localCtx := WithProviders(c, providers)

	return consumer(localCtx)
}

// Provide injects a single key/value pair into a new scope and calls consumer
// with the enriched context. It is equivalent to:
//
//	plocal.ProvideAll(ctx, []plocal.Provider{plocal.Value(key, val)}, consumer)
//
// Like [ProvideAll], the value is stored in a new registry node, so it shadows
// shallower values for the same key and is itself shadowed by deeper ones
// (nearest scope wins). Use [ProvideAll] directly when injecting several values
// at once to avoid creating one node per key.
func Provide[T, U any](c context.Context, key *ResourceKey[T], val T, consumer func(c context.Context) U) U {
	return ProvideAll(c, []Provider{Value(key, val)}, consumer)
}

// UpdateProvider upserts a single provider into the current scope's node if one
// exists, mutating that node in place (no new registry level) and returning the
// same context. If no registry node exists yet, it creates one via
// [WithProviders].
//
// The write is guarded by the node's lock, so it is safe to call concurrently
// with [Use] on the same context. Note that upserting mutates a context that
// may already have been shared: prefer [Provide]/[ProvideAll] for scope-local
// injection and reach for Update* only when you intend to amend a live scope.
func UpdateProvider(c context.Context, provider Provider) context.Context {
	if node, ok := c.Value(registryKey).(*registryNode); ok {
		node.mu.Lock()
		provider.apply(node.values)
		node.mu.Unlock()
		return c
	}
	return WithProviders(c, []Provider{provider})
}

// UpdateProviders upserts several providers into the current scope's node if one
// exists, mutating that node in place (no new registry level) and returning the
// same context. If no registry node exists yet, it creates one via
// [WithProviders]. Writes are lock-guarded, as with [UpdateProvider].
func UpdateProviders(c context.Context, providers []Provider) context.Context {
	if node, ok := c.Value(registryKey).(*registryNode); ok {
		node.mu.Lock()
		for _, p := range providers {
			p.apply(node.values)
		}
		node.mu.Unlock()
		return c
	}
	return WithProviders(c, providers)
}

// Use retrieves the typed value associated with key from the nearest enclosing
// scope. It walks the Prototype Chain — from the current leaf node up to the
// root — and returns the first matching value it finds.
//
// Precedence is "nearest scope wins": a value injected in a deeper scope shadows
// one injected in a shallower scope for the same key, regardless of whether it
// was injected via [Provide], [ProvideAll], or [WithProviders].
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
	if c == nil {
		return key.Default()
	}

	// Walk the registry tree from the current leaf up to the root.
	if node, ok := c.Value(registryKey).(*registryNode); ok {
		for curr := node; curr != nil; curr = curr.parent {
			if val, exists := curr.get(key); exists {
				return val.(T)
			}
		}
	}

	return key.Default()
}

// WithProvider injects a single key/value pair and returns the enriched context.
//
// If the current scope already exists and does not yet contain key, the pair is
// merged into that scope's node in place (no new level) — so a run of
// WithProvider calls for distinct keys stays at one registry level. If key is
// already present, a new shadowing node is pushed so the override is scoped.
// When no registry node exists yet, a new node is created.
func WithProvider[T any](c context.Context, key *ResourceKey[T], val T) context.Context {
	if parentNode, ok := c.Value(registryKey).(*registryNode); ok {
		if !parentNode.has(key) {
			return UpdateProvider(c, Value(key, val))
		}
	}
	return WithProviders(c, []Provider{Value(key, val)})
}

func WithProviders(c context.Context, providers []Provider) context.Context {
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

	return localCtx
}
