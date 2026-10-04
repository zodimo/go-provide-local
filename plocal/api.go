package plocal

import (
	"context"
)

// Provider is implemented by any value that contributes an entry to a new
// scope node. Use [Value] to create a Provider, and pass one or more Providers
// to [ProvideAll] or [Provide] to inject them into the context.
type Provider interface {
	// asEntry returns the key/value pair this Provider contributes.
	asEntry() entry
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
// exists, returning a context whose leaf is a **newly constructed** node
// carrying the amended entry set. The receiver node is never mutated, and no
// new registry level is added. If no registry node exists yet, it creates one
// via [WithProviders].
//
// Because upsert copies rather than mutates, an upserted context is safe to
// share: a concurrent [Use] on the original context never observes a
// partially written node. Note that the *returned* context is the one that
// reflects the change — the context passed in is left as it was.
func UpdateProvider(c context.Context, provider Provider) context.Context {
	if node, ok := c.Value(registryKey).(*registryNode); ok {
		return context.WithValue(c, registryKey, node.with(provider.asEntry()))
	}
	return WithProviders(c, []Provider{provider})
}

// UpdateProviders upserts several providers into the current scope's node if one
// exists, returning a context whose leaf is a **newly constructed** node
// carrying the amended entry set. As with [UpdateProvider], no node is mutated
// and no new registry level is added; when no registry node exists yet, one is
// created via [WithProviders].
func UpdateProviders(c context.Context, providers []Provider) context.Context {
	if node, ok := c.Value(registryKey).(*registryNode); ok {
		entries := make([]entry, 0, len(providers))
		for _, p := range providers {
			entries = append(entries, p.asEntry())
		}
		return context.WithValue(c, registryKey, node.with(entries...))
	}
	return WithProviders(c, providers)
}

// Use retrieves the typed value associated with key from the nearest enclosing
// scope. It walks the Prototype Chain — from the current leaf node up to the
// root — and returns the first matching value it finds.
//
// Precedence is "nearest scope wins": a value injected in a deeper scope shadows
// one injected in a shallower scope for the same key, regardless of whether it
// was injected via [Provide], [ProvideAll], [WithProvider], [UpdateProvider], or
// [UpdateProviders].
//
// If key was never injected into any ancestor scope, Use returns the key's
// default fallback (set when the key was created via [NewResourceKey]).
//
// Use never allocates heap memory and takes no lock. Node entry storage is
// immutable after construction, so a shared context is safe to read
// concurrently from multiple goroutines — and safe to read while another
// goroutine upserts a derived context.
//
// Example:
//
//	theme := plocal.Use(ctx, ThemeKey) // type: Theme — no assertion needed
func Use[T any](c context.Context, key *ResourceKey[T]) T {
	if c == nil {
		return key.Default()
	}

	// Walk the registry tree from the current leaf up to the root. Each step is
	// a lock-free pointer chase over immutable entries.
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
// merged into that scope (no new level) — so a run of WithProvider calls for
// distinct keys stays at one registry level. If key is already present, a new
// shadowing node is pushed so the override is scoped. When no registry node
// exists yet, a new node is created.
//
// Merging is copy-on-upsert: a new node carrying the current entries plus the
// new one is constructed, and the node on the given context is left unmodified.
// The returned context is the one that reflects the merge.
func WithProvider[T any](c context.Context, key *ResourceKey[T], val T) context.Context {
	if parentNode, ok := c.Value(registryKey).(*registryNode); ok {
		if !parentNode.has(key) {
			return UpdateProvider(c, Value(key, val))
		}
	}
	return WithProviders(c, []Provider{Value(key, val)})
}

// WithProviders constructs a new registry node holding the given providers and
// returns a context whose leaf is that node. The entry slice is sized to the
// provider count at construction, so it never grows.
func WithProviders(c context.Context, providers []Provider) context.Context {
	// Size the entry storage exactly: it is fixed at construction and never
	// appended to afterwards.
	entries := make([]entry, 0, len(providers))
	for _, p := range providers {
		entries = append(entries, p.asEntry())
	}

	// Create the new tree node
	node := &registryNode{
		entries: entries,
	}

	// If a parent node exists, link to it (The Prototype Chain)
	if parentNode, ok := c.Value(registryKey).(*registryNode); ok {
		node.parent = parentNode
	}

	// Wrap the context once with our new leaf node
	localCtx := context.WithValue(c, registryKey, node)

	return localCtx
}
