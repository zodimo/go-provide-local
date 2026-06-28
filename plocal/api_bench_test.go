package plocal

import (
	"context"
	"testing"
)

// buildChain creates a registry chain of the given depth, injecting rootKey at
// the bottom (depth 1) and filler keys at every other level. It returns the
// innermost context and a cleanup noop (the chain lives as long as the test).
//
// This helper drives the depth-scaling benchmarks without allocating inside
// the hot loop.
func buildChain(depth int) (context.Context, *ResourceKey[string]) {
	rootKey := NewResourceKey[string]("default")
	fillerKey := NewResourceKey[int](0)

	// Build the chain iteratively to avoid recursive closure overhead.
	// We seed the first level with rootKey so Use(rootKey) always requires a
	// full traversal to the bottom when seeking from the top.
	ctx := ProvideAll(context.Background(), []Provider{
		Value(rootKey, "root-value"),
	}, func(c context.Context) context.Context { return c })

	for i := 1; i < depth; i++ {
		ctx = ProvideAll(ctx, []Provider{
			Value(fillerKey, i),
		}, func(c context.Context) context.Context { return c })
	}

	return ctx, rootKey
}

// ─────────────────────────────────────────────────────────────────────────────
// §3 Performance Benchmarks
//
// Measured baseline (Intel Core Ultra 9 185H, Go 1.24, linux/amd64):
//
//   BenchmarkUse_Depth1-22                →   11 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth10-22               →   56 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth100-22              →  524 ns/op    0 B/op   0 allocs/op
//
//   BenchmarkProvideAll_Depth1-22         →  209 ns/op  440 B/op   6 allocs/op
//   BenchmarkProvideAll_Depth100-22       →  197 ns/op  440 B/op   6 allocs/op
//   (identical alloc counts prove scope creation cost is depth-independent)
//
//   BenchmarkUse_DepthScaling_1-22        →   12 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_DepthScaling_10-22       →   58 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_DepthScaling_100-22      →  557 ns/op    0 B/op   0 allocs/op
//   (10x depth ≈ 5x time — O(depth), linear growth confirmed)
//
//   BenchmarkVsStdlib_plocal_Depth10-22   →   58 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth10-22   →   29 ns/op    0 B/op   0 allocs/op
//
// NOTE: For a worst-case single-key lookup at shallow depth, stdlib context.Value
// is ~2x faster per step. Each plocal step involves a map[any]any lookup; each
// stdlib step is a pointer comparison + type switch. plocal's advantages are:
//   1. Zero heap allocations on Use() at any depth.
//   2. Scope creation cost is O(providers), not O(depth) — ProvideAll with N
//      keys creates exactly 1 node instead of N context.WithValue wraps.
//   3. Full compile-time type safety — no manual type assertions.
// ─────────────────────────────────────────────────────────────────────────────

// 3.2 Use() at depth 1 — baseline.
func BenchmarkUse_Depth1(b *testing.B) {
	ctx, key := buildChain(1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// 3.3 Use() at depth 10.
func BenchmarkUse_Depth10(b *testing.B) {
	ctx, key := buildChain(10)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// 3.4 Use() at depth 100.
func BenchmarkUse_Depth100(b *testing.B) {
	ctx, key := buildChain(100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// 3.5 ProvideAll alloc count should be the same regardless of how deep the
// existing chain is — proving it is proportional to providers, not depth.
func BenchmarkProvideAll_Depth1(b *testing.B) {
	ctx, _ := buildChain(1)
	key := NewResourceKey[string]("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ProvideAll(ctx, []Provider{Value(key, "v")}, func(c context.Context) context.Context { return c })
	}
}

func BenchmarkProvideAll_Depth100(b *testing.B) {
	ctx, _ := buildChain(100)
	key := NewResourceKey[string]("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ProvideAll(ctx, []Provider{Value(key, "v")}, func(c context.Context) context.Context { return c })
	}
}

// 3.6 Depth-scaling: run Use() at three depths so the ns/op values can be
// inspected for linear growth (10x depth ≈ 10x time).
func BenchmarkUse_DepthScaling_1(b *testing.B) {
	ctx, key := buildChain(1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkUse_DepthScaling_10(b *testing.B) {
	ctx, key := buildChain(10)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkUse_DepthScaling_100(b *testing.B) {
	ctx, key := buildChain(100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// 3.7 Direct comparison: plocal Use() vs stdlib context.Value() at depth 10.
//
// Stdlib chain: 10 context.WithValue nodes, each wrapping the previous.
// The value is stored at the bottom and retrieved from the top, requiring
// full traversal through Go's interface-based linked list.

type stdlibKey struct{ id int }

func buildStdlibChain(depth int) (context.Context, stdlibKey) {
	rootKey := stdlibKey{id: 0}
	ctx := context.WithValue(context.Background(), rootKey, "root-value")
	for i := 1; i < depth; i++ {
		ctx = context.WithValue(ctx, stdlibKey{id: i}, i)
	}
	return ctx, rootKey
}

func BenchmarkVsStdlib_plocal_Depth10(b *testing.B) {
	ctx, key := buildChain(10)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkVsStdlib_stdlib_Depth10(b *testing.B) {
	ctx, key := buildStdlibChain(10)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = ctx.Value(key)
	}
}
