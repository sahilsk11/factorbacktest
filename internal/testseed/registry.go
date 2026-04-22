package testseed

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

// Registry holds a set of named fixtures and knows how to apply them in
// dependency order. Registry is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	fixtures map[string]Fixture
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{fixtures: map[string]Fixture{}}
}

// Register adds a fixture to the registry. Registering a fixture whose name
// already exists panics; fixture names are intended to be compile-time
// constants.
func (r *Registry) Register(f Fixture) {
	if f.Name == "" {
		panic("testseed: fixture name is required")
	}
	if f.Apply == nil {
		panic(fmt.Sprintf("testseed: fixture %q has nil Apply", f.Name))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.fixtures[f.Name]; ok {
		panic(fmt.Sprintf("testseed: fixture %q already registered", f.Name))
	}
	r.fixtures[f.Name] = f
}

// Fixture looks up a fixture by name.
func (r *Registry) Fixture(name string) (Fixture, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.fixtures[name]
	return f, ok
}

// FixtureInfo is a lightweight view of a fixture used for listing.
type FixtureInfo struct {
	Name         string   `json:"name"`
	Dependencies []string `json:"dependencies"`
}

// List returns every registered fixture sorted by name.
func (r *Registry) List() []FixtureInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FixtureInfo, 0, len(r.fixtures))
	for _, f := range r.fixtures {
		deps := append([]string(nil), f.Dependencies...)
		sort.Strings(deps)
		out = append(out, FixtureInfo{Name: f.Name, Dependencies: deps})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Apply resolves the transitive dependencies of names, runs each required
// fixture exactly once in dependency order, and returns the Result of every
// fixture that ran (including dependencies) keyed by fixture name. The order
// in which fixtures run is deterministic: dependencies come first, ties are
// broken by fixture name.
func (r *Registry) Apply(ctx context.Context, db *sql.DB, names []string) (map[string]Result, error) {
	order, err := r.resolve(names)
	if err != nil {
		return nil, err
	}

	results := make(map[string]Result, len(order))
	for _, name := range order {
		f, _ := r.Fixture(name)
		res, err := f.Apply(ctx, db, results)
		if err != nil {
			return results, fmt.Errorf("fixture %q: %w", name, err)
		}
		if res == nil {
			res = Result{}
		}
		results[name] = res
	}
	return results, nil
}

// resolve returns the list of fixture names (including transitive deps) in
// the order they should be applied. It fails if any name is unknown or if
// the dependency graph contains a cycle.
func (r *Registry) resolve(names []string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	order := []string{}

	var visit func(name string, stack []string) error
	visit = func(name string, stack []string) error {
		switch color[name] {
		case black:
			return nil
		case gray:
			return fmt.Errorf("testseed: dependency cycle: %v -> %s", stack, name)
		}
		f, ok := r.fixtures[name]
		if !ok {
			return fmt.Errorf("testseed: unknown fixture %q", name)
		}
		color[name] = gray
		deps := append([]string(nil), f.Dependencies...)
		sort.Strings(deps)
		for _, d := range deps {
			if err := visit(d, append(stack, name)); err != nil {
				return err
			}
		}
		color[name] = black
		order = append(order, name)
		return nil
	}

	requested := append([]string(nil), names...)
	sort.Strings(requested)
	for _, n := range requested {
		if err := visit(n, nil); err != nil {
			return nil, err
		}
	}
	return order, nil
}
