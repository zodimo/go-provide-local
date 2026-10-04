package plocal

import "sync"

type registryContextKey struct{}

var registryKey = registryContextKey{}

// registryNode is a single node in the Prototype Chain.
// It holds only the values injected at one particular ProvideAll call site.
//
// mu guards values against concurrent access. Most nodes are written once at
// creation and then only read, but the upsert paths (UpdateProvider /
// UpdateProviders, and WithProvider when it merges into the current scope)
// mutate an existing node's map in place, so readers take RLock and writers
// Lock. This preserves the "safe to call concurrently" guarantee of [Use]
// even when a derived context upserts into a shared parent.
type registryNode struct {
	mu     sync.RWMutex
	parent *registryNode
	values map[any]any // Only holds values injected at this specific level
}

// get reads a value from this node under the read lock.
func (n *registryNode) get(key any) (any, bool) {
	n.mu.RLock()
	val, ok := n.values[key]
	n.mu.RUnlock()
	return val, ok
}

// has reports whether key is present in this node under the read lock.
func (n *registryNode) has(key any) bool {
	n.mu.RLock()
	_, ok := n.values[key]
	n.mu.RUnlock()
	return ok
}

// put writes a value into this node under the write lock.
func (n *registryNode) put(key, val any) {
	n.mu.Lock()
	n.values[key] = val
	n.mu.Unlock()
}

type providerImpl[T any] struct {
	key *ResourceKey[T]
	val T
}

func (p providerImpl[T]) apply(m map[any]any) {
	m[p.key] = p.val
}

// ResourceKey is a typed, collision-proof context key for values of type T.
// Each key is uniquely identified by its pointer address, so two independently
// created keys for the same type T will never collide, even across packages.
//
// Always create keys via [NewResourceKey]; never construct ResourceKey literals
// directly, because the zero value lacks a meaningful default.
type ResourceKey[T any] struct {
	// Default is the fallback value returned by [Use] when the key has not
	// been injected into any ancestor scope. Set via [NewResourceKey].
	defaultValue T
}

func (r *ResourceKey[T]) Default() T {
	if r == nil {
		var zero T
		return zero
	}
	return r.defaultValue
}

// NewResourceKey creates a new [ResourceKey] for values of type T with the
// given fallback default. The fallback is returned by [Use] whenever the key
// has not been injected into the context.
//
// Keys are typically declared as package-level variables so they can be shared
// across call sites:
//
//	var RequestIDKey = plocal.NewResourceKey[string]("unknown-req-id")
//	var ThemeKey     = plocal.NewResourceKey[Theme](DefaultTheme)
func NewResourceKey[T any](fallback T) *ResourceKey[T] {
	return &ResourceKey[T]{defaultValue: fallback}
}
