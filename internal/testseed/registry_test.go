package testseed

import (
	"context"
	"database/sql"
	"testing"
)

func makeFixture(t *testing.T, name string, deps []string, appliedOrder *[]string) Fixture {
	t.Helper()
	return Fixture{
		Name:         name,
		Dependencies: deps,
		Apply: func(ctx context.Context, db *sql.DB, got map[string]Result) (Result, error) {
			for _, d := range deps {
				if _, ok := got[d]; !ok {
					t.Errorf("fixture %q ran before dep %q", name, d)
				}
			}
			*appliedOrder = append(*appliedOrder, name)
			return Result{"name": name}, nil
		},
	}
}

func TestRegistry_AppliesDependenciesInOrder(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var order []string
	// c -> b -> a ; d -> a. Asking for c and d should yield a,b,c,d (a
	// appears once even though two fixtures depend on it).
	r.Register(makeFixture(t, "a", nil, &order))
	r.Register(makeFixture(t, "b", []string{"a"}, &order))
	r.Register(makeFixture(t, "c", []string{"b"}, &order))
	r.Register(makeFixture(t, "d", []string{"a"}, &order))

	res, err := r.Apply(context.Background(), nil, []string{"c", "d"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got, want := len(res), 4; got != want {
		t.Fatalf("results: got %d, want %d", got, want)
	}
	if got, want := len(order), 4; got != want {
		t.Fatalf("applied: got %d, want %d (%v)", got, want, order)
	}
	// a must come first. b must come before c. d must come after a.
	idx := map[string]int{}
	for i, n := range order {
		idx[n] = i
	}
	if idx["a"] > idx["b"] || idx["a"] > idx["c"] || idx["a"] > idx["d"] {
		t.Fatalf("a should come first, got %v", order)
	}
	if idx["b"] > idx["c"] {
		t.Fatalf("b must come before c, got %v", order)
	}
}

func TestRegistry_CycleDetected(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var order []string
	r.Register(makeFixture(t, "a", []string{"b"}, &order))
	r.Register(makeFixture(t, "b", []string{"a"}, &order))

	if _, err := r.Apply(context.Background(), nil, []string{"a"}); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestRegistry_UnknownFixture(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if _, err := r.Apply(context.Background(), nil, []string{"nope"}); err == nil {
		t.Fatal("expected unknown-fixture error, got nil")
	}
}

func TestRegistry_ListIsSorted(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var order []string
	r.Register(makeFixture(t, "b", nil, &order))
	r.Register(makeFixture(t, "a", nil, &order))
	list := r.List()
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("List not sorted: %v", list)
	}
}
