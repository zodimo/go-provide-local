package plocal

import (
	"context"
	"fmt"
	"testing"
)

// buildChain creates a registry chain of exactly the given depth: one
// WithProviders call per level, so the returned context holds `depth` linked
// registryNodes. The sought key (rootKey) is injected at the outermost level,
// which makes it the worst case for Use: a read from the innermost context must
// walk the entire chain before it finds a match.
//
// Each filler level holds exactly one provider whose key is never sought, so a
// read for rootKey is guaranteed to traverse all `depth` nodes.
func buildChain(depth int) (context.Context, *ResourceKey[string]) {
	rootKey := NewResourceKey[string]("default")
	fillerKey := NewResourceKey[int](0)

	// Outermost node holds the sought key.
	ctx := WithProviders(context.Background(), []Provider{Value(rootKey, "root-value")})

	// Each remaining level adds one filler node, deepening the chain.
	for i := 1; i < depth; i++ {
		ctx = WithProviders(ctx, []Provider{Value(fillerKey, i)})
	}

	return ctx, rootKey
}

// buildBatchScope puts rootKey plus (width-1) filler values into a SINGLE
// registry node, using one WithProviders call. It is the "many values in one
// scope" shape — the library's headline case.
func buildBatchScope(width int) (context.Context, *ResourceKey[string]) {
	rootKey := NewResourceKey[string]("default")
	fillerKey := NewResourceKey[int](0)

	providers := make([]Provider, 0, width)
	providers = append(providers, Value(rootKey, "root-value"))
	for i := 1; i < width; i++ {
		providers = append(providers, Value(fillerKey, i))
	}

	return WithProviders(context.Background(), providers), rootKey
}

// buildSpreadChain puts rootKey at the outermost level and every other value in
// its own nested node, so `width` values cost `width` registry levels. It is the
// "one value per scope" shape — the cost of spreading N values across N scopes
// instead of batching them into one.
//
// It uses WithProviders (which always pushes a new node) rather than
// WithProvider, because WithProvider upserts: distinct keys are merged into the
// current node and would collapse the chain to a single level.
func buildSpreadChain(width int) (context.Context, *ResourceKey[string]) {
	rootKey := NewResourceKey[string]("default")
	fillerKey := NewResourceKey[int](0)

	ctx := WithProviders(context.Background(), []Provider{Value(rootKey, "root-value")})
	for i := 1; i < width; i++ {
		ctx = WithProviders(ctx, []Provider{Value(fillerKey, i)})
	}

	return ctx, rootKey
}

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

// ─────────────────────────────────────────────────────────────────────────────
// §3 Performance Benchmarks
//
// Measured baseline (Intel Core Ultra 9 185H, Go 1.27, linux/amd64, GOMAXPROCS=22),
// regenerated in a single run (2s/bench unless noted):
//
// Two independent axes are measured. Do not read one as the other.
//
// AXIS 1 — depth (one value per nested scope). plocal Use() cost grows with
// the number of registry nodes it must walk, so per-level nesting is LINEAR and
// stdlib context.Value() is cheaper at every depth:
//
//   BenchmarkUse_Depth1-22                       →    38 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth10-22                      →   292 ns/op    0 B/op   0 allocs/op
//   BenchmarkUse_Depth100-22                     →  2652 ns/op    0 B/op   0 allocs/op
//
//   BenchmarkVsStdlib_plocal_Depth1-22           →    37 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth1-22           →     8 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_plocal_Depth10-22          →   279 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth10-22          →    54 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_plocal_Depth100-22         →  2671 ns/op    0 B/op   0 allocs/op
//   BenchmarkVsStdlib_stdlib_Depth100-22         →   439 ns/op    0 B/op   0 allocs/op
//   (both are O(nodes); stdlib wins at every depth because each plocal step is a
//    lock-guarded map[any]any probe plus a pointer hop, while each stdlib step is
//    a pointer compare + type switch)
//
// AXIS 2 — width (N values for ONE scope). This is the library's headline. One
// WithProviders call stores all N values in one node, so a read stays FLAT in N,
// while spreading the same N values across N scopes makes reads LINEAR in N:
//
//   BenchmarkRead_Batched_N1 ... N32-22          →   37 ns/op at every N (flat)
//   BenchmarkRead_Spread_N1-22                   →    37 ns/op
//   BenchmarkRead_Spread_N4-22                   →   112 ns/op
//   BenchmarkRead_Spread_N8-22                   →   213 ns/op
//   BenchmarkRead_Spread_N16-22                  →   429 ns/op
//   BenchmarkRead_Spread_N32-22                  →   859 ns/op
//
//   BenchmarkInject_Batched_N32-22               →  2079 ns/op  2488 B/op   6 allocs/op   (1 level)
//   BenchmarkInject_SeqPlocal_N32-22             →  9519 ns/op 13824 B/op 128 allocs/op   (32 levels)
//   BenchmarkInject_SeqStdlib_N32-22             →  1418 ns/op  1536 B/op  32 allocs/op   (32 levels)
//   BenchmarkInject_Upsert_N32-22                →  2896 ns/op   512 B/op  32 allocs/op   (1 level)
//   (stdlib's raw injection ns/bytes stay lower even at N=32; plocal's win is the
//    one-node shape — 1 level, constant allocs, and flat reads)
//
// NOTE: Absolute ns/op varies with machine and thermal state; treat the ratios
// within a single run, and the alloc/level counts, as the load-bearing evidence
// and the absolute figures as advisory. The library's advantages are:
//   1. Zero heap allocations on Use() at every depth and every width.
//   2. Batching N values into one scope costs ONE node, a near-constant alloc
//      count, and keeps reads flat in N — versus N nodes, N allocs, and linear
//      reads for the same values spread across scopes. (Raw injection ns/bytes
//      for a single stdlib WithValue remain lower; the win is shape, not speed.)
//   3. Full compile-time type safety — no manual type assertions.
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildChain_RegistryDepth guards the harness: buildChain(depth) must
// produce a registry chain of exactly `depth` nodes, or every depth benchmark
// below is measuring the wrong thing. A prior version of buildChain called
// WithProviders once for any depth, so "depth 100" was really a 1-node chain.
func TestBuildChain_RegistryDepth(t *testing.T) {
	for _, depth := range []int{1, 2, 4, 10, 100} {
		ctx, _ := buildChain(depth)
		if got := chainDepth(ctx); got != depth {
			t.Errorf("buildChain(%d): registry depth = %d, want %d", depth, got, depth)
		}
	}
}

// TestReadShapes_LevelCounts guards the width harness: the batched shape must
// collapse N values into one level, and the spread shape must spend one level
// per value.
func TestReadShapes_LevelCounts(t *testing.T) {
	for _, width := range []int{1, 4, 32} {
		batched, _ := buildBatchScope(width)
		if got := chainDepth(batched); got != 1 {
			t.Errorf("buildBatchScope(%d): registry depth = %d, want 1", width, got)
		}
		spread, _ := buildSpreadChain(width)
		if got := chainDepth(spread); got != width {
			t.Errorf("buildSpreadChain(%d): registry depth = %d, want %d", width, got, width)
		}
	}
}

// ─── §3.1 Depth benchmarks (one value per nested scope) ──────────────────────

// BenchmarkUse_Depth1/10/100 — worst-case read of the outermost (root) key.
func BenchmarkUse_Depth1(b *testing.B)   { benchmarkUseDepth(b, 1) }
func BenchmarkUse_Depth10(b *testing.B)  { benchmarkUseDepth(b, 10) }
func BenchmarkUse_Depth100(b *testing.B) { benchmarkUseDepth(b, 100) }

func benchmarkUseDepth(b *testing.B, depth int) {
	ctx, key := buildChain(depth)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// ProvideAll alloc should be proportional to providers, not to the depth of the
// chain it is attached to.
func BenchmarkProvideAll_Depth1(b *testing.B)   { benchmarkProvideAllDepth(b, 1) }
func BenchmarkProvideAll_Depth100(b *testing.B) { benchmarkProvideAllDepth(b, 100) }

func benchmarkProvideAllDepth(b *testing.B, depth int) {
	ctx, _ := buildChain(depth)
	key := NewResourceKey[string]("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		WithProviders(ctx, []Provider{Value(key, "v")})
	}
}

// Depth scaling, run explicitly so the linear growth is visible side by side.
func BenchmarkUse_DepthScaling_1(b *testing.B)   { benchmarkUseDepth(b, 1) }
func BenchmarkUse_DepthScaling_10(b *testing.B)  { benchmarkUseDepth(b, 10) }
func BenchmarkUse_DepthScaling_100(b *testing.B) { benchmarkUseDepth(b, 100) }

// ─── §3.2 Direct plocal vs stdlib at matched depth ───────────────────────────
//
// In both implementations the sought key lives in the root (outermost) node and
// all remaining levels are fillers, so the two benchmarks in a pair traverse
// exactly the same distance.

func BenchmarkVsStdlib_plocal_Depth1(b *testing.B)   { benchmarkUseDepth(b, 1) }
func BenchmarkVsStdlib_plocal_Depth10(b *testing.B)  { benchmarkUseDepth(b, 10) }
func BenchmarkVsStdlib_plocal_Depth100(b *testing.B) { benchmarkUseDepth(b, 100) }

func BenchmarkVsStdlib_stdlib_Depth1(b *testing.B)   { benchmarkStdlibDepth(b, 1) }
func BenchmarkVsStdlib_stdlib_Depth10(b *testing.B)  { benchmarkStdlibDepth(b, 10) }
func BenchmarkVsStdlib_stdlib_Depth100(b *testing.B) { benchmarkStdlibDepth(b, 100) }

func benchmarkStdlibDepth(b *testing.B, depth int) {
	ctx, key := buildStdlibChain(depth)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = ctx.Value(key)
	}
}

// ─── §3.3 Width benchmarks: many values in one scope (the real axis) ─────────
//
// Read the outermost-injected key after injecting `width` values with each
// shape. Batched reads are flat in width; spread reads grow linearly.

func BenchmarkRead_Batched_N1(b *testing.B)  { benchmarkReadBatched(b, 1) }
func BenchmarkRead_Batched_N4(b *testing.B)  { benchmarkReadBatched(b, 4) }
func BenchmarkRead_Batched_N8(b *testing.B)  { benchmarkReadBatched(b, 8) }
func BenchmarkRead_Batched_N16(b *testing.B) { benchmarkReadBatched(b, 16) }
func BenchmarkRead_Batched_N32(b *testing.B) { benchmarkReadBatched(b, 32) }

func benchmarkReadBatched(b *testing.B, width int) {
	ctx, key := buildBatchScope(width)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

func BenchmarkRead_Spread_N1(b *testing.B)  { benchmarkReadSpread(b, 1) }
func BenchmarkRead_Spread_N4(b *testing.B)  { benchmarkReadSpread(b, 4) }
func BenchmarkRead_Spread_N8(b *testing.B)  { benchmarkReadSpread(b, 8) }
func BenchmarkRead_Spread_N16(b *testing.B) { benchmarkReadSpread(b, 16) }
func BenchmarkRead_Spread_N32(b *testing.B) { benchmarkReadSpread(b, 32) }

func benchmarkReadSpread(b *testing.B, width int) {
	ctx, key := buildSpreadChain(width)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = Use(ctx, key)
	}
}

// ─── §3.4 Injection cost: batched vs sequential plocal vs sequential stdlib ──
//
// Each benchmark injects `width` values into a fresh context. Levels and allocs
// are the stable assertions; ns/op is advisory.

func BenchmarkInject_Batched_N1(b *testing.B)  { benchmarkInjectBatched(b, 1) }
func BenchmarkInject_Batched_N4(b *testing.B)  { benchmarkInjectBatched(b, 4) }
func BenchmarkInject_Batched_N8(b *testing.B)  { benchmarkInjectBatched(b, 8) }
func BenchmarkInject_Batched_N16(b *testing.B) { benchmarkInjectBatched(b, 16) }
func BenchmarkInject_Batched_N32(b *testing.B) { benchmarkInjectBatched(b, 32) }

func benchmarkInjectBatched(b *testing.B, width int) {
	key := NewResourceKey[int](0)
	providers := make([]Provider, width)
	for i := range providers {
		providers[i] = Value(key, i)
	}
	bg := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = WithProviders(bg, providers)
	}
}

func BenchmarkInject_SeqPlocal_N1(b *testing.B)  { benchmarkInjectSeqPlocal(b, 1) }
func BenchmarkInject_SeqPlocal_N4(b *testing.B)  { benchmarkInjectSeqPlocal(b, 4) }
func BenchmarkInject_SeqPlocal_N8(b *testing.B)  { benchmarkInjectSeqPlocal(b, 8) }
func BenchmarkInject_SeqPlocal_N16(b *testing.B) { benchmarkInjectSeqPlocal(b, 16) }
func BenchmarkInject_SeqPlocal_N32(b *testing.B) { benchmarkInjectSeqPlocal(b, 32) }

func benchmarkInjectSeqPlocal(b *testing.B, width int) {
	bg := context.Background()
	// One WithProviders call per value: each pushes a new node. (WithProvider
	// would upsert distinct keys into one node instead of spreading them.)
	providers := make([]Provider, width)
	keys := make([]*ResourceKey[int], width)
	for i := range keys {
		keys[i] = NewResourceKey[int](i)
		providers[i] = Value(keys[i], i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ctx := bg
		for i := 0; i < width; i++ {
			ctx = WithProviders(ctx, []Provider{providers[i]})
		}
	}
}

func BenchmarkInject_SeqStdlib_N1(b *testing.B)  { benchmarkInjectSeqStdlib(b, 1) }
func BenchmarkInject_SeqStdlib_N4(b *testing.B)  { benchmarkInjectSeqStdlib(b, 4) }
func BenchmarkInject_SeqStdlib_N8(b *testing.B)  { benchmarkInjectSeqStdlib(b, 8) }
func BenchmarkInject_SeqStdlib_N16(b *testing.B) { benchmarkInjectSeqStdlib(b, 16) }
func BenchmarkInject_SeqStdlib_N32(b *testing.B) { benchmarkInjectSeqStdlib(b, 32) }

func benchmarkInjectSeqStdlib(b *testing.B, width int) {
	bg := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ctx := bg
		for i := 0; i < width; i++ {
			ctx = context.WithValue(ctx, stdlibKey{id: i}, i)
		}
	}
}

// BenchmarkInject_Upsert_N* measures the upsert path: one base node created up
// front, then N values applied in place. This is only valid for N values that
// share the base node's scope; it is the cheapest plocal injection shape.
func BenchmarkInject_Upsert_N1(b *testing.B)  { benchmarkInjectUpsert(b, 1) }
func BenchmarkInject_Upsert_N4(b *testing.B)  { benchmarkInjectUpsert(b, 4) }
func BenchmarkInject_Upsert_N8(b *testing.B)  { benchmarkInjectUpsert(b, 8) }
func BenchmarkInject_Upsert_N16(b *testing.B) { benchmarkInjectUpsert(b, 16) }
func BenchmarkInject_Upsert_N32(b *testing.B) { benchmarkInjectUpsert(b, 32) }

func benchmarkInjectUpsert(b *testing.B, width int) {
	bg := WithProviders(context.Background(), []Provider{Value(NewResourceKey[int](-1), -1)})
	key := NewResourceKey[int](0)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ctx := bg
		for i := 0; i < width; i++ {
			ctx = UpdateProvider(ctx, Value(key, i))
		}
	}
}

// ─── §3.5 Scope-creation comparison at depth 1 ───────────────────────────────

func BenchmarkVsStdlib_plocal_ScopeCreation_Depth1(b *testing.B) {
	ctx := context.Background()
	key := NewResourceKey[string]("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		WithProviders(ctx, []Provider{Value(key, "v")})
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

// ─── §3.6 Cross-check: the two shapes at equal N, printed side by side ───────

// TestReadShapes_Crossover prints the batched-vs-spread read cost and level
// counts at each N so the flat-vs-linear shape is visible in one place.
func TestReadShapes_Crossover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shape summary in short mode")
	}
	for _, width := range []int{1, 4, 8, 16, 32} {
		batched, bkey := buildBatchScope(width)
		spread, skey := buildSpreadChain(width)
		bLevels, sLevels := chainDepth(batched), chainDepth(spread)

		bRead := testing.Benchmark(func(b *testing.B) {
			for range b.N {
				_ = Use(batched, bkey)
			}
		})
		sRead := testing.Benchmark(func(b *testing.B) {
			for range b.N {
				_ = Use(spread, skey)
			}
		})

		fmt.Printf("N=%2d  batched: %6d ns/op  levels=%d   |   spread: %6d ns/op  levels=%d\n",
			width, bRead.NsPerOp(), bLevels, sRead.NsPerOp(), sLevels)
	}
}
