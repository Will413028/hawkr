// services/pricebook-service/internal/domain/mapping/seed_test_helper.go
package mapping

// NewSeedForTest builds a Seed directly from entries, bypassing CSV/manifest
// loading and validation entirely — for tests in other packages (e.g.
// internal/usecase) that need a resolvable Seed without a real seed file.
// Mirrors LoadSeed's index-construction line exactly (index keyed by literal
// AliasName, no normalization at insert time — Resolve normalizes the
// lookup name instead, see seed.go); callers must pass already-normalized
// AliasName values (no brackets, no quantity specs) for Resolve to find them.
func NewSeedForTest(entries []SeedEntry) Seed {
	index := make(map[string]SeedEntry, len(entries))
	for _, e := range entries {
		index[e.AliasName] = e
	}
	return Seed{entries: entries, index: index}
}
