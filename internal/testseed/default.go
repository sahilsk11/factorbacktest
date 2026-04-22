package testseed

// Default is the package-level registry populated with all built-in
// fixtures. Callers should use this unless they explicitly want an isolated
// registry (for example in a unit test of the registry itself).
var Default = func() *Registry {
	r := NewRegistry()
	r.Register(baseUniverseFixture)
	r.Register(prices2020Fixture)
	r.Register(userBasicFixture)
	r.Register(strategyMomentumFixture)
	r.Register(investmentBasicFixture)
	return r
}()
