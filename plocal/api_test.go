package plocal

import (
	"context"
	"sync"
	"testing"
	"time"
)

// chainDepth walks the registry tree rooted at c and returns its depth.
// A context with no plocal node returns 0; each ProvideAll/Provide call
// that creates a new node increments the depth by exactly 1.
func chainDepth(c context.Context) int {
	node, ok := c.Value(registryKey).(*registryNode)
	if !ok {
		return 0
	}
	depth := 0
	for curr := node; curr != nil; curr = curr.parent {
		depth++
	}
	return depth
}

// ─────────────────────────────────────────────────────────────────────────────
// §1 Correctness Tests
// ─────────────────────────────────────────────────────────────────────────────

// 1.2 Use() returns the key default when nothing has been injected.
func TestUse_FallbackOnEmptyContext(t *testing.T) {
	key := NewResourceKey[string]("default-value")
	got := Use(context.Background(), key)
	if got != "default-value" {
		t.Errorf("expected fallback %q, got %q", "default-value", got)
	}
}

// 1.3 Use() returns the key default when the context was modified only by
// stdlib context.WithValue — no plocal node is present.
func TestUse_FallbackOnNonPlocal(t *testing.T) {
	type stdKey struct{}
	ctx := context.WithValue(context.Background(), stdKey{}, "stdlib-val")
	key := NewResourceKey[int](42)
	got := Use(ctx, key)
	if got != 42 {
		t.Errorf("expected fallback 42, got %d", got)
	}
}

// 1.4 A single Provide() call makes the injected value available via Use().
func TestProvide_ReturnsInjectedValue(t *testing.T) {
	key := NewResourceKey[string]("fallback")
	result := Provide(context.Background(), key, "injected", func(ctx context.Context) string {
		return Use(ctx, key)
	})
	if result != "injected" {
		t.Errorf("expected %q, got %q", "injected", result)
	}
}

// 1.5 ProvideAll() with multiple providers makes all keys available.
func TestProvideAll_ReturnsAllInjectedValues(t *testing.T) {
	keyA := NewResourceKey[string]("a-default")
	keyB := NewResourceKey[int](0)
	keyC := NewResourceKey[bool](false)

	type result struct {
		a string
		b int
		c bool
	}

	got := ProvideAll(context.Background(), []Provider{
		Value(keyA, "hello"),
		Value(keyB, 99),
		Value(keyC, true),
	}, func(ctx context.Context) result {
		return result{
			a: Use(ctx, keyA),
			b: Use(ctx, keyB),
			c: Use(ctx, keyC),
		}
	})

	if got.a != "hello" {
		t.Errorf("keyA: expected %q, got %q", "hello", got.a)
	}
	if got.b != 99 {
		t.Errorf("keyB: expected 99, got %d", got.b)
	}
	if !got.c {
		t.Errorf("keyC: expected true, got false")
	}
}

// 1.6 An inner scope can read a value injected in the outer scope.
func TestNesting_InheritanceFromParent(t *testing.T) {
	key := NewResourceKey[string]("default")

	Provide(context.Background(), key, "outer", func(outerCtx context.Context) struct{} {
		// inner scope does NOT re-inject key
		Provide(outerCtx, NewResourceKey[int](0), 1, func(innerCtx context.Context) struct{} {
			got := Use(innerCtx, key)
			if got != "outer" {
				t.Errorf("inner scope: expected %q from parent, got %q", "outer", got)
			}
			return struct{}{}
		})
		return struct{}{}
	})
}

// 1.7 A scope three levels deep can read a value injected at the root.
func TestNesting_ThreeLevels_ReadsGrandparent(t *testing.T) {
	rootKey := NewResourceKey[string]("default")
	mid := NewResourceKey[int](0)
	leaf := NewResourceKey[bool](false)

	Provide(context.Background(), rootKey, "root-value", func(ctx1 context.Context) struct{} {
		Provide(ctx1, mid, 1, func(ctx2 context.Context) struct{} {
			Provide(ctx2, leaf, true, func(ctx3 context.Context) struct{} {
				got := Use(ctx3, rootKey)
				if got != "root-value" {
					t.Errorf("3 levels deep: expected %q, got %q", "root-value", got)
				}
				return struct{}{}
			})
			return struct{}{}
		})
		return struct{}{}
	})
}

// 1.8 An inner scope shadows an outer value; the outer scope still sees its own.
func TestNesting_Shadowing(t *testing.T) {
	key := NewResourceKey[string]("default")

	Provide(context.Background(), key, "outer", func(outerCtx context.Context) struct{} {
		// sanity-check outer value
		if got := Use(outerCtx, key); got != "outer" {
			t.Errorf("outer scope: expected %q, got %q", "outer", got)
		}

		Provide(outerCtx, key, "inner", func(innerCtx context.Context) struct{} {
			if got := Use(innerCtx, key); got != "inner" {
				t.Errorf("inner scope: expected shadow %q, got %q", "inner", got)
			}
			return struct{}{}
		})

		// outer scope is unaffected after inner scope returns
		if got := Use(outerCtx, key); got != "outer" {
			t.Errorf("outer scope after inner: expected %q, got %q", "outer", got)
		}
		return struct{}{}
	})
}

// 1.9 Two sibling scopes created from the same parent do not contaminate each other.
func TestSiblingScopes_AreIsolated(t *testing.T) {
	key := NewResourceKey[string]("default")
	parent := context.Background()

	var siblingAGot, siblingBGot string

	Provide(parent, key, "sibling-A", func(ctxA context.Context) struct{} {
		siblingAGot = Use(ctxA, key)
		return struct{}{}
	})

	Provide(parent, key, "sibling-B", func(ctxB context.Context) struct{} {
		siblingBGot = Use(ctxB, key)
		return struct{}{}
	})

	if siblingAGot != "sibling-A" {
		t.Errorf("sibling A: expected %q, got %q", "sibling-A", siblingAGot)
	}
	if siblingBGot != "sibling-B" {
		t.Errorf("sibling B: expected %q, got %q", "sibling-B", siblingBGot)
	}
}

// 1.10 ProvideAll() with N providers increments registry depth by exactly 1 (not N).
func TestProvideAll_CreatesExactlyOneNode(t *testing.T) {
	k1 := NewResourceKey[string]("")
	k2 := NewResourceKey[int](0)
	k3 := NewResourceKey[bool](false)

	ctx := context.Background()
	depthBefore := chainDepth(ctx) // 0

	ProvideAll(ctx, []Provider{
		Value(k1, "a"),
		Value(k2, 1),
		Value(k3, true),
	}, func(inner context.Context) struct{} {
		depthAfter := chainDepth(inner)
		expected := depthBefore + 1
		if depthAfter != expected {
			t.Errorf("ProvideAll with 3 providers: expected depth %d, got %d", expected, depthAfter)
		}
		return struct{}{}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// §2 Safety Tests
// ─────────────────────────────────────────────────────────────────────────────

// 2.1 Use() must not panic on a plain background context.
func TestUse_NoPanic_BackgroundContext(t *testing.T) {
	key := NewResourceKey[string]("default")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Use() panicked on background context: %v", r)
		}
	}()
	_ = Use(context.Background(), key)
}

// 2.2 Use() must not panic on a cancelled context.
func TestUse_NoPanic_CancelledContext(t *testing.T) {
	key := NewResourceKey[string]("default")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Use() panicked on cancelled context: %v", r)
		}
	}()
	_ = Use(ctx, key)
}

// 2.3 Use() must not panic on a timed-out context.
func TestUse_NoPanic_TimeoutContext(t *testing.T) {
	key := NewResourceKey[string]("default")
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure it has expired

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Use() panicked on timeout context: %v", r)
		}
	}()
	_ = Use(ctx, key)
}

// 2.4 Two independently created keys with the same type and default do not
// collide — injecting one does not affect the other.
func TestKeyCollision_IndependentKeys_SameType(t *testing.T) {
	keyA := NewResourceKey[string]("default")
	keyB := NewResourceKey[string]("default")

	// Only inject keyA
	Provide(context.Background(), keyA, "value-for-A", func(ctx context.Context) struct{} {
		gotA := Use(ctx, keyA)
		gotB := Use(ctx, keyB) // keyB was never injected — should return its own default

		if gotA != "value-for-A" {
			t.Errorf("keyA: expected %q, got %q", "value-for-A", gotA)
		}
		if gotB != "default" {
			t.Errorf("keyB: expected fallback %q, got %q", "default", gotB)
		}
		return struct{}{}
	})
}

// 2.5 Both keys injected in the same ProvideAll resolve to their own values.
func TestKeyCollision_BothInjected_ResolveIndependently(t *testing.T) {
	keyA := NewResourceKey[string]("default")
	keyB := NewResourceKey[string]("default")

	ProvideAll(context.Background(), []Provider{
		Value(keyA, "A"),
		Value(keyB, "B"),
	}, func(ctx context.Context) struct{} {
		if got := Use(ctx, keyA); got != "A" {
			t.Errorf("keyA: expected %q, got %q", "A", got)
		}
		if got := Use(ctx, keyB); got != "B" {
			t.Errorf("keyB: expected %q, got %q", "B", got)
		}
		return struct{}{}
	})
}

// 2.6 Concurrent Use() calls on a shared scoped context must be data-race free.
// Run this test suite with -race to exercise the detector.
func TestGoroutineSafety_ConcurrentUse(t *testing.T) {
	key := NewResourceKey[string]("default")
	const goroutines = 50

	Provide(context.Background(), key, "shared-value", func(ctx context.Context) struct{} {
		var wg sync.WaitGroup
		wg.Add(goroutines)
		for range goroutines {
			go func() {
				defer wg.Done()
				got := Use(ctx, key)
				if got != "shared-value" {
					// t.Errorf is goroutine-safe
					t.Errorf("goroutine: expected %q, got %q", "shared-value", got)
				}
			}()
		}
		wg.Wait()
		return struct{}{}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// §3 Scope-Override Semantics
//
// "Local override, not permanent replace": an inner scope may shadow a key, but
// the outer scope's value is untouched once the inner scope is left. With stdlib
// context.WithValue the equivalent requires re-wrapping the context.
// ─────────────────────────────────────────────────────────────────────────────

// 3.1 An inner ProvideAll override leaves the outer scope's value intact.
func TestScopeOverride_ProvideAll_LeavesOuterIntact(t *testing.T) {
	key := NewResourceKey[string]("default")

	Provide(context.Background(), key, "outer", func(outerCtx context.Context) struct{} {
		if got := Use(outerCtx, key); got != "outer" {
			t.Fatalf("outer scope: expected %q, got %q", "outer", got)
		}

		// ProvideAll re-injects the same key at an inner scope.
		ProvideAll(outerCtx, []Provider{
			Value(key, "inner"),
			Value(NewResourceKey[int](0), 1),
		}, func(innerCtx context.Context) struct{} {
			if got := Use(innerCtx, key); got != "inner" {
				t.Errorf("inner scope: expected override %q, got %q", "inner", got)
			}
			return struct{}{}
		})

		// Outer must still read its own value — the override was local.
		if got := Use(outerCtx, key); got != "outer" {
			t.Errorf("outer after inner ProvideAll: expected %q, got %q", "outer", got)
		}
		return struct{}{}
	})
}

// 3.2 The context-returning API: WithProviders override is local; the parent
// context still reads the original.
func TestScopeOverride_WithProviders_ParentUnaffected(t *testing.T) {
	key := NewResourceKey[string]("default")

	parent := WithProviders(context.Background(), []Provider{Value(key, "parent-value")})
	if got := Use(parent, key); got != "parent-value" {
		t.Fatalf("parent before: expected %q, got %q", "parent-value", got)
	}

	child := WithProviders(parent, []Provider{Value(key, "child-value")})

	if got := Use(child, key); got != "child-value" {
		t.Errorf("child: expected override %q, got %q", "child-value", got)
	}
	// The parent context value is unchanged: the override lived only in child.
	if got := Use(parent, key); got != "parent-value" {
		t.Errorf("parent after deriving child: expected %q, got %q", "parent-value", got)
	}
}

// 3.3 WithProvider override (existing key) is likewise scoped to the new node.
func TestScopeOverride_WithProvider_ExistingKeyScopes(t *testing.T) {
	key := NewResourceKey[string]("default")

	base := WithProviders(context.Background(), []Provider{Value(key, "base")})
	derived := WithProvider(base, key, "derived") // key exists -> new shadowing node

	if got := Use(derived, key); got != "derived" {
		t.Errorf("derived: expected %q, got %q", "derived", got)
	}
	if got := Use(base, key); got != "base" {
		t.Errorf("base after WithProvider override: expected %q, got %q", "base", got)
	}
}

// 3.4 WithProvider with a NEW key merges into the current node without adding a
// registry level, and both keys remain readable.
func TestWithProvider_NewKey_MergesIntoCurrentScope(t *testing.T) {
	k1 := NewResourceKey[string]("d1")
	k2 := NewResourceKey[string]("d2")

	base := WithProviders(context.Background(), []Provider{Value(k1, "v1")})
	before := chainDepth(base)

	merged := WithProvider(base, k2, "v2")

	if got := chainDepth(merged); got != before {
		t.Errorf("WithProvider(new key): registry depth = %d, want %d (should merge, not nest)", got, before)
	}
	if got := Use(merged, k1); got != "v1" {
		t.Errorf("k1 after merge: expected %q, got %q", "v1", got)
	}
	if got := Use(merged, k2); got != "v2" {
		t.Errorf("k2 after merge: expected %q, got %q", "v2", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §4 Upsert (UpdateProvider / UpdateProviders)
// ─────────────────────────────────────────────────────────────────────────────

// 4.1 UpdateProvider overwrites the value in the current scope (no new level)
// and returns a context that resolves the new value. The upsert is
// copy-on-upsert: the returned context carries a newly constructed node, and
// the context passed in keeps resolving its original value.
func TestUpdateProvider_OverwritesWithoutMutatingSource(t *testing.T) {
	key := NewResourceKey[string]("default")

	ctx := WithProviders(context.Background(), []Provider{Value(key, "original")})
	depthBefore := chainDepth(ctx)

	updated := UpdateProvider(ctx, Value(key, "updated"))

	if got := chainDepth(updated); got != depthBefore {
		t.Errorf("UpdateProvider: registry depth = %d, want %d (no new level)", got, depthBefore)
	}
	if got := Use(updated, key); got != "updated" {
		t.Errorf("after UpdateProvider: expected %q, got %q", "updated", got)
	}
	// The source node is never mutated, so the context we upserted from still
	// resolves its original value (copy-on-upsert semantics).
	if got := Use(ctx, key); got != "original" {
		t.Errorf("source context after upsert: expected %q, got %q", "original", got)
	}
}

// 4.2 UpdateProvider with no existing registry node creates one.
func TestUpdateProvider_NoNode_FallsBackToNewNode(t *testing.T) {
	key := NewResourceKey[string]("default")

	ctx := UpdateProvider(context.Background(), Value(key, "created"))

	if got := chainDepth(ctx); got != 1 {
		t.Errorf("UpdateProvider on empty context: registry depth = %d, want 1", got)
	}
	if got := Use(ctx, key); got != "created" {
		t.Errorf("expected %q, got %q", "created", got)
	}
}

// 4.3 UpdateProviders upserts several keys into the current scope at once.
// A key already in the node is overwritten; one absent from it is added.
func TestUpdateProviders_UpsertsMultiple(t *testing.T) {
	k1 := NewResourceKey[int](0)
	k2 := NewResourceKey[string]("d")

	ctx := WithProviders(context.Background(), []Provider{Value(k1, 1)})
	depthBefore := chainDepth(ctx)

	updated := UpdateProviders(ctx, []Provider{
		Value(k1, 42),
		Value(k2, "added"),
	})

	if got := chainDepth(updated); got != depthBefore {
		t.Errorf("UpdateProviders: registry depth = %d, want %d (no new level)", got, depthBefore)
	}
	if got := Use(updated, k1); got != 42 {
		t.Errorf("k1 after upsert: expected 42, got %d", got)
	}
	if got := Use(updated, k2); got != "added" {
		t.Errorf("k2 after upsert: expected %q, got %q", "added", got)
	}
}

// 4.4 Upsert must be data-race free against concurrent Use() on the same
// shared parent context. Run with -race. Because upsert is copy-on-upsert,
// the shared base node is never mutated, so concurrent reads cannot observe
// a torn state.
func TestUpsert_ConcurrentWithUse_NoRace(t *testing.T) {
	readKey := NewResourceKey[int](-1)
	base := WithProviders(context.Background(), []Provider{Value(readKey, 7)})

	const iters = 5000
	var wg sync.WaitGroup
	wg.Add(2)

	// Reader goroutine: repeatedly read the shared base context. The base node
	// is immutable, so this must remain 7 throughout.
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			if got := Use(base, readKey); got != 7 {
				t.Errorf("concurrent read: expected 7, got %d", got)
				return
			}
		}
	}()

	// Writer goroutine: upsert new distinct keys into the same scope, each
	// time deriving a fresh context from the shared base.
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			_ = WithProvider(base, NewResourceKey[int](i), i)
		}
	}()

	wg.Wait()
}

// 4.5 Upserting a derived context leaves the parent context resolving its
// original value (copy-on-upsert, not mutation of shared node state).
func TestUpdateProvider_DerivedUpsertLeavesParentIntact(t *testing.T) {
	keyA := NewResourceKey[string]("a-default")
	keyB := NewResourceKey[string]("b-default")

	parent := WithProviders(context.Background(), []Provider{Value(keyA, "parent-a")})
	parentDepth := chainDepth(parent)

	// Derive a scope from the parent and upsert onto it.
	derived := WithProviders(parent, []Provider{Value(keyB, "derived-b")})
	upserted := UpdateProviders(derived, []Provider{Value(keyA, "derived-a")})

	if got := Use(upserted, keyA); got != "derived-a" {
		t.Errorf("upserted derived scope: expected %q for keyA, got %q", "derived-a", got)
	}
	if got := Use(upserted, keyB); got != "derived-b" {
		t.Errorf("upserted derived scope: expected %q for keyB, got %q", "derived-b", got)
	}
	// Upsert added no new level beyond the derived scope's own.
	if got := chainDepth(upserted); got != parentDepth+1 {
		t.Errorf("upserted derived scope: registry depth = %d, want %d", got, parentDepth+1)
	}

	// The parent resolves its original value for the upserted key.
	if got := Use(parent, keyA); got != "parent-a" {
		t.Errorf("parent after derived upsert: expected %q, got %q", "parent-a", got)
	}
	// And it never saw keyB, which was injected only in the derived scope.
	if got := Use(parent, keyB); got != "b-default" {
		t.Errorf("parent after derived upsert: expected fallback %q for keyB, got %q", "b-default", got)
	}
	// The derived (pre-upsert) context is likewise unchanged.
	if got := Use(derived, keyA); got != "parent-a" {
		t.Errorf("derived context before upsert still expected %q for keyA, got %q", "parent-a", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// §5 Immutability of node storage
// ─────────────────────────────────────────────────────────────────────────────

// 5.1 Construction sizes entry storage exactly: a node built from N providers
// holds exactly N entries, and Lookup never reports a key that was not injected
// at that node.
func TestNodeStorage_HoldsExactlyInjectedEntries(t *testing.T) {
	k1 := NewResourceKey[string]("d1")
	k2 := NewResourceKey[int](0)
	k3 := NewResourceKey[bool](false)
	absent := NewResourceKey[string]("absent")

	ctx := WithProviders(context.Background(), []Provider{
		Value(k1, "one"),
		Value(k2, 2),
		Value(k3, true),
	})

	node := ctx.Value(registryKey).(*registryNode)
	if len(node.entries) != 3 {
		t.Errorf("node entries = %d, want 3", len(node.entries))
	}
	if cap(node.entries) != 3 {
		t.Errorf("node entries capacity = %d, want 3 (sized exactly, must not grow)", cap(node.entries))
	}
	if node.has(absent) {
		t.Errorf("node reports a key that was never injected at this node")
	}
	if got := Use(ctx, absent); got != "absent" {
		t.Errorf("uninjected key: expected fallback %q, got %q", "absent", got)
	}
}

// 5.2 Upsert replaces rather than appends: re-upserting an existing key does not
// grow the entry set, and a merged new key adds exactly one entry.
func TestNodeStorage_UpsertDoesNotDuplicateEntries(t *testing.T) {
	k1 := NewResourceKey[int](0)
	k2 := NewResourceKey[string]("d")

	ctx := WithProviders(context.Background(), []Provider{Value(k1, 1)})

	same := UpdateProvider(ctx, Value(k1, 2))
	if got := len(same.Value(registryKey).(*registryNode).entries); got != 1 {
		t.Errorf("re-upserting an existing key: entries = %d, want 1", got)
	}
	if got := Use(same, k1); got != 2 {
		t.Errorf("re-upserted key: expected 2, got %d", got)
	}

	merged := UpdateProvider(ctx, Value(k2, "added"))
	if got := len(merged.Value(registryKey).(*registryNode).entries); got != 2 {
		t.Errorf("upserting a new key: entries = %d, want 2", got)
	}
	if got := Use(merged, k1); got != 1 {
		t.Errorf("pre-existing key after merge: expected 1, got %d", got)
	}
	if got := Use(merged, k2); got != "added" {
		t.Errorf("merged key: expected %q, got %q", "added", got)
	}
}
