// Package testseed is a shared registry of named database fixtures used by
// both the Go integration tests and the test-api admin router (which exposes
// seeding over HTTP to Playwright).
//
// A fixture is a unit of seed data. Fixtures declare dependencies on other
// fixtures; callers ask the registry to apply a set of fixtures and the
// registry resolves the dependency graph, runs each required fixture at most
// once, and returns the IDs created by each one so tests can reference the
// rows they just seeded.
package testseed

import (
	"context"
	"database/sql"
)

// Result is the payload returned by a single fixture's Apply call. Keys are
// stable, fixture-local names (for example "aapl_ticker_id" or
// "investment_id") and values are whatever the fixture wants to expose
// (typically uuid.UUID values, but any JSON-serialisable type works).
type Result map[string]any

// Fixture is one unit of seed data.
type Fixture struct {
	// Name is the stable identifier callers use to request this fixture.
	Name string

	// Dependencies lists the names of other fixtures that must run before
	// this one. Order inside the slice does not matter; the registry
	// topologically sorts the full set of requested fixtures.
	Dependencies []string

	// Apply inserts the fixture's rows. It receives a context, the database
	// handle, and the already-computed results of every transitive
	// dependency (keyed by fixture name) so a fixture can reference IDs
	// created by its dependencies without re-querying.
	Apply func(ctx context.Context, db *sql.DB, deps map[string]Result) (Result, error)
}
