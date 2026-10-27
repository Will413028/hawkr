package middleware

import (
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
)

// delegatedRouteRegistry records every route of 56 號 ADM 代行營運 (§6.1.1 的 38 支),
// filled at mount time by the five helpers in router/delegated_capability.go.
//
// It exists because the limiter has to answer "is this a delegated endpoint" BEFORE
// any handler runs, and no cheaper answer is correct. A path predicate cannot be one:
// the 20 resource endpoints sit under /api/v1/admin/sellers/:store_id/, and that same
// :store_id segment can carry non-delegated endpoints too. A prefix rule cannot tell
// them apart: it counts a non-delegated endpoint that shares the segment as delegated,
// and it misses a delegated endpoint whose path falls outside the shape it matches.
// Both directions fail silently (the endpoint still answers 429, only the business
// code is wrong, and nothing turns red), so membership is registered at mount time
// and matched exactly.
//
// Registration rather than a hand-kept list, for the same reason delegatedHandle takes
// the capability as a required parameter: 宣告＝掛載合一. A delegated endpoint added
// next year is covered the day it is mounted, instead of the day someone remembers a
// second list exists.
//
// It lives in middleware and not in router because router imports middleware; the
// reverse would be a cycle.
var (
	delegatedRouteMu       sync.Mutex
	delegatedRouteRegistry = map[string]struct{}{}
)

// RegisterDelegatedRoute records one delegated route by (method, matched path). The
// path must be gin's route pattern (":store_id" kept as-is), because that is what
// c.FullPath() returns at request time.
func RegisterDelegatedRoute(method, fullPath string) {
	delegatedRouteMu.Lock()
	delegatedRouteRegistry[method+" "+fullPath] = struct{}{}
	delegatedRouteMu.Unlock()
}

// RegisteredDelegatedRoutes returns the registered routes sorted, so the router-side
// inventory test can compare it against the mounted route table.
func RegisteredDelegatedRoutes() []string {
	delegatedRouteMu.Lock()
	out := make([]string, 0, len(delegatedRouteRegistry))
	for k := range delegatedRouteRegistry {
		out = append(out, k)
	}
	delegatedRouteMu.Unlock()
	sort.Strings(out)
	return out
}

// IsDelegatedRoute reports whether this request matched a delegated route.
//
// c.FullPath() and not c.Request.URL.Path: the raw URL carries a concrete store id
// ("/sellers/2685914682/products") which matches nothing in the registry, and a
// request that matched no route at all yields "" — correctly not delegated.
func IsDelegatedRoute(c *gin.Context) bool {
	full := c.FullPath()
	if full == "" {
		return false
	}
	delegatedRouteMu.Lock()
	_, ok := delegatedRouteRegistry[c.Request.Method+" "+full]
	delegatedRouteMu.Unlock()
	return ok
}
