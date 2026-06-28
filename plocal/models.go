package plocal

type registryContextKey struct{}

var registryKey = registryContextKey{}

// 1. The Registry Tree Node
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

type ResourceKey[T any] struct {
	Default T
}

func NewResourceKey[T any](fallback T) *ResourceKey[T] {
	return &ResourceKey[T]{Default: fallback}
}
