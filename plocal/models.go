package plocal

type registryContextKey struct{}

var registryKey = registryContextKey{}

// registryNode is a single node in the Prototype Chain.
// It holds only the values injected at one particular ProvideAll call site.
type registryNode struct {
	parent *registryNode
	values map[any]any // Only holds values injected at this specific level
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
	Default T
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
	return &ResourceKey[T]{Default: fallback}
}
