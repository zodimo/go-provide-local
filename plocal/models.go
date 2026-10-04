package plocal

type registryContextKey struct{}

var registryKey = registryContextKey{}

// entry is one key/value pair stored in a node. Keys are *ResourceKey[T]
// addresses, so identity is a pointer compare — no interface hashing.
type entry struct {
	key any
	val any
}

// registryNode is a single node in the Prototype Chain.
// It holds only the values injected at one particular ProvideAll call site.
//
// Invariant: a node is immutable after construction. entries is fixed when the
// node is built and is never mutated afterwards. Upsert paths
// (UpdateProvider / UpdateProviders, and WithProvider when it merges into the
// current scope) construct a *new* node carrying the amended entry set rather
// than writing into an existing one.
//
// Because entries never change, [Use] needs no lock: a reader traversing a
// chain can never observe a partially written node, so concurrent reads and
// concurrent upserts are race-free by construction.
//
// Storage is a pointer-keyed slice scanned linearly rather than a
// map[any]any. Probes are pointer compares instead of interface hashes, and
// the entries are contiguous. The trade-off is that the worst-case read inside
// a single node grows with the node's entry count (crossover around ~16
// entries per node); see the package documentation for the soft cap.
type registryNode struct {
	parent  *registryNode
	entries []entry // Only holds values injected at this specific level
}

// get reads a value from this node. It scans entries by pointer compare and
// is lock-free: entries is immutable after construction.
func (n *registryNode) get(key any) (any, bool) {
	for i := range n.entries {
		if n.entries[i].key == key {
			return n.entries[i].val, true
		}
	}
	return nil, false
}

// has reports whether key is present in this node. Lock-free, as with [get].
func (n *registryNode) has(key any) bool {
	for i := range n.entries {
		if n.entries[i].key == key {
			return true
		}
	}
	return false
}

// with returns a new node carrying this node's entries plus the given ones.
// A given entry whose key is already present replaces the existing binding
// rather than being appended, so upsert cannot produce duplicate keys. The
// receiver is left unmodified — this is how upsert avoids mutating a node a
// concurrent reader may be traversing (copy-on-upsert).
func (n *registryNode) with(extra ...entry) *registryNode {
	entries := make([]entry, 0, len(n.entries)+len(extra))
	entries = append(entries, n.entries...)
	for _, e := range extra {
		replaced := false
		for i := range entries {
			if entries[i].key == e.key {
				entries[i] = e
				replaced = true
				break
			}
		}
		if !replaced {
			entries = append(entries, e)
		}
	}
	return &registryNode{
		parent:  n.parent,
		entries: entries,
	}
}

type providerImpl[T any] struct {
	key *ResourceKey[T]
	val T
}

// asEntry converts the provider into the key/value pair it contributes.
func (p providerImpl[T]) asEntry() entry {
	return entry{key: p.key, val: p.val}
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
