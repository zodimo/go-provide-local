package plocal

import (
	"context"
)

type Provider interface {
	apply(m map[any]any)
}

func Value[T any](key *ResourceKey[T], val T) Provider {
	return providerImpl[T]{
		key: key,
		val: val,
	}
}

// ProvideAll creates a new node and points it at the parent. Zero copying.
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

// Provide is just a convenience wrapper around ProvideAll now for an adhoc provider
func Provide[T, U any](c context.Context, key *ResourceKey[T], val T, consumer func(c context.Context) U) U {
	return ProvideAll(c, []Provider{Value(key, val)}, consumer)
}

// Use walks up the registry tree
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
