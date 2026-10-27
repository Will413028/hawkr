package authz

import (
	"sort"
	"sync"
)

// The route registry records `(method, path, capability)` for every seller route
// declared via requireSeller(cap) (46 §3.2 / §3.8). Registration is driven by the
// router mount helper (sellerHandle) so declaration and mounting are one act — a
// route that carries requireSeller is, by construction, in this registry. The boot
// assertion (router.assertSellerRouteCoverage) then uses this registry to verify
// deny-by-default coverage: every /api/v1/app route must resolve to a declared
// capability, an explicit exemption, or the buyer-facing default.

var (
	registryMu sync.Mutex
	// registry maps "METHOD PATH" → capability. A map (not a slice) so re-building
	// the router in tests is idempotent rather than accumulating duplicates.
	registry = map[string]Capability{}
)

func routeKey(method, path string) string { return method + " " + path }

// Register records that (method, path) declared cap. Called by the router mount
// helper. Panics if cap is not a known token — a typo in a capability constant
// must fail loudly at boot, not silently under-gate.
func Register(method, path string, cap Capability) {
	if !IsKnown(cap) {
		panic("authz.Register: unknown capability " + string(cap) + " for " + routeKey(method, path))
	}
	registryMu.Lock()
	registry[routeKey(method, path)] = cap
	registryMu.Unlock()
}

// LookupRoute returns the declared capability for (method, path), if any.
func LookupRoute(method, path string) (Capability, bool) {
	registryMu.Lock()
	cap, ok := registry[routeKey(method, path)]
	registryMu.Unlock()
	return cap, ok
}

// The admin registry is the same idea for ADM 代行營運 (56 §6.1.0 / §8.1.2): the
// delegatedHandle mount helper records (method, path, adminCapability) so that
// declaration and mounting are again one act, and the boot assertion can then walk
// the registry rather than a path prefix. Prefix would be the wrong scope — S1
// preflight lives under the same /api/v1/admin/sellers/:store_id/… shape yet has no
// capability to declare (deciding whether one can be granted IS its job), so a
// prefix-based check would panic the Gateway at boot.
//
// It is a SEPARATE map from the seller registry, not a merged one: the two token
// vocabularies must not be able to satisfy each other's Register call.

var (
	adminRegistryMu sync.Mutex
	adminRegistry   = map[string]AdminCapability{}
)

// RegisterAdmin records that (method, path) declared cap. Called by the
// delegatedHandle mount helper. Panics if cap is not one of the nine tokens —
// including the empty string, which is what makes "forgot to declare a capability"
// a boot failure instead of an unguarded delegated route.
func RegisterAdmin(method, path string, cap AdminCapability) {
	if !IsKnownAdmin(cap) {
		panic("authz.RegisterAdmin: unknown admin capability " + string(cap) + " for " + routeKey(method, path))
	}
	adminRegistryMu.Lock()
	adminRegistry[routeKey(method, path)] = cap
	adminRegistryMu.Unlock()
}

// LookupAdminRoute returns the declared admin capability for (method, path), if any.
func LookupAdminRoute(method, path string) (AdminCapability, bool) {
	adminRegistryMu.Lock()
	cap, ok := adminRegistry[routeKey(method, path)]
	adminRegistryMu.Unlock()
	return cap, ok
}

// RegisteredAdminRoute is one declared delegated-operation route.
type RegisteredAdminRoute struct {
	Method string
	Path   string
	Cap    AdminCapability
}

// RegisteredAdmin returns all declared delegated routes, sorted by path then method
// for stable test and panic-message output.
func RegisteredAdmin() []RegisteredAdminRoute {
	adminRegistryMu.Lock()
	out := make([]RegisteredAdminRoute, 0, len(adminRegistry))
	for k, cap := range adminRegistry {
		for i := 0; i < len(k); i++ {
			if k[i] == ' ' {
				out = append(out, RegisteredAdminRoute{Method: k[:i], Path: k[i+1:], Cap: cap})
				break
			}
		}
	}
	adminRegistryMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// RegisteredRoute is one declared seller route.
type RegisteredRoute struct {
	Method string
	Path   string
	Cap    Capability
}

// Registered returns all declared seller routes, sorted by path then method for
// stable test output.
func Registered() []RegisteredRoute {
	registryMu.Lock()
	out := make([]RegisteredRoute, 0, len(registry))
	for k, cap := range registry {
		// k = "METHOD PATH"; split on first space.
		for i := 0; i < len(k); i++ {
			if k[i] == ' ' {
				out = append(out, RegisteredRoute{Method: k[:i], Path: k[i+1:], Cap: cap})
				break
			}
		}
	}
	registryMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}
