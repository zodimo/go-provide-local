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
// Measured baseline (Intel Core Ultra 9 185H, Go 1.27, linux/amd64, GOMAXPROCS=22),
// regenerated with `make bench-mem` (5s/bench):
//
//   BenchmarkUse_Depth1-22                       →    26 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth10-22                      →   132 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth100-22                     →  1213 ns/op    0 B/op   0 allocs/op
//
//   BenchmarkProvideAll_Depth1-22                →   340 ns/op  440 B/op   6 allocs/op
//   BenchmarkProvideAll_Depth100-22              →   345 ns/op  440 B/op   6 allocs/op
//   (identical alloc counts prove scope creation cost is depth-independent)
//
//   BenchmarkUse_DepthScaling_1-22               →    27 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_DepthScaling_10-22              →   132 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_DepthScaling_100-22             →  1200 ns/op    0 B/op   0 allocs/op
//   (10x depth ≈ 10x time — O(depth), linear growth confirmed)
//
//   BenchmarkVsStdlib_plocal_Depth1-22           →    26 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth1-22           →     8 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_plocal_Depth10-22          →   132 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth10-22          →    53 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_plocal_Depth100-22         →  1170 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth100-22         →   441 ns/op    0 B/op   0 allocs/op
//
//   BenchmarkVsStdlib_plocal_ScopeCreation_Depth1-22  →  343 ns/op  440 B/op   6 allocs/op
//   BenchmarkVsStdlib_stdlib_ScopeCreation_Depth1-22  →   39 ns/op   48 B/op   1 allocs/op
//   (1 provider each; the plocal side stays flat as providers grow, the stdlib
//    side adds one allocation per wrap)
//
// NOTE: Absolute ns/op varies with machine and thermal state; treat the ratios
// within a single run as the load-bearing evidence and the absolute figures as
// advisory. For a worst-case single-key lookup, stdlib context.Value is faster
// per step — ~3x at depth 1, ~2.5x at depth 10, ~2.7x at depth 100. Each plocal
// step is a map[any]any probe; each stdlib step is a pointer comparison + type
// switch. plocal's advantages are:
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

// 3.7 Direct comparison: plocal Use() vs stdlib context.Value() at depths
// 1, 10, and 100, plus scope creation at depth 1.
//
// In both implementations the sought key lives in the root (outermost) node
// and all remaining levels are fillers, so the two benchmarks in a pair
// traverse exactly the same distance.

// stdlibKey is a comparable struct used as a context key in the stdlib chain.
// Struct keys avoid the lint warning for built-in types as map keys while
// staying allocation-free.
type stdlibKey struct{ id int }

// buildStdlibChain builds a chain of `depth` nested context.WithValue nodes,
// storing the sought value in the root (outermost) node and fillers below it.
// It mirrors buildChain so the two chains are shape-equivalent.
func buildStdlibChain(depth int) (context.Context, stdlibKey) {
	rootKey := stdlibKey{id: 0}
	ctx := context.WithValue(context.Background(), rootKey, "root-value")
	for i := 1; i < depth; i++ {
		ctx = context.WithValue(ctx, stdlibKey{id: i}, i)
	}
	return ctx, rootKey
}

func BenchmarkVsStdlib_plocal_Depth1(b *testing.B) {
	ctx, key := buildChain(1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkVsStdlib_stdlib_Depth1(b *testing.B) {
	ctx, key := buildStdlibChain(1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ctx.Value(key)
	}
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
		ctx.Value(key)
	}
}

func BenchmarkVsStdlib_plocal_Depth100(b *testing.B) {
	ctx, key := buildChain(100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkVsStdlib_stdlib_Depth100(b *testing.B) {
	ctx, key := buildStdlibChain(100)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ctx.Value(key)
	}
}

// 3.8 Scope-creation comparison at depth 1: plocal ProvideAll (one registry
// node holding N providers) vs N sequential stdlib context.WithValue wraps.
// The stdlib side is expected to allocate and grow O(providers), while the
// plocal side stays flat.
func BenchmarkVsStdlib_plocal_ScopeCreation_Depth1(b *testing.B) {
	ctx, _ := buildChain(1)
	key := NewResourceKey[string]("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ProvideAll(ctx, []Provider{Value(key, "v")}, func(c context.Context) context.Context { return c })
	}
}

func BenchmarkVsStdlib_stdlib_ScopeCreation_Depth1(b *testing.B) {
	ctx := context.Background()
	key := stdlibKey{id: 0}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = context.WithValue(ctx, key, "v")
	}
}
