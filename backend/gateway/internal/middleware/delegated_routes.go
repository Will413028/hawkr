package middleware

import (
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
)

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
