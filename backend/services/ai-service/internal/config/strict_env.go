package config

import (
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"strings"
)

// envTagKey is the struct tag key Config's fields use to declare which
// environment variable(s) they are populated from (see config.go).
const envTagKey = "env"

// WarnUnknownEnv reflects over cfg's `env:"..."` tags (recursing into nested
// structs such as ServerConfig/RabbitMQConfig) and slog.Warns about every
// process environment variable that looks like it was meant for this
// struct's config but isn't one of the declared names — most likely a typo
// (e.g. GEMINI_APIKEY instead of GEMINI_API_KEY) or a stale/renamed variable
// left behind in k8s values / docker-compose-local.yml after a rename.
//
// It deliberately does NOT warn about every unrecognized process env var —
// that would be almost all noise: PATH, HOME, plus variables this process
// legitimately reads outside this struct entirely —
// OTEL_EXPORTER_OTLP_ENDPOINT/OTEL_RESOURCE_ATTRIBUTES, INTERNAL_AUTH_MODE,
// MQ_PASSIVE_DECLARE, REDIS_HOST/PORT/PASSWORD, GOPROXY/GONOSUMDB are all set
// on ai-service's pod per k8s/values/ai-service.yaml + overlays but read by
// shared pkg/ code directly, never by this Config struct.
//
// So the comparison is scoped to a set of "family" prefixes derived purely
// from the declared tag names themselves — e.g. DB_HOST and DB_PORT both
// declare a DB_-family tag, so an unrecognized DB_-prefixed variable is
// in-family and gets a warning, while an unrelated REDIS_-prefixed one is
// out of family and is silently ignored. This keeps both the known set AND
// the scoping boundary 100% derived from the struct's own tags — nothing
// here is a hand-written variable (or prefix) list that can go stale as
// fields are added, renamed or removed; see task-7-report.md for what this
// would warn about against dev's actual injected variable set.
//
// One family-scoped source of noise the chart cannot be told to suppress:
// Kubernetes injects Docker-links-style env vars for every Service object
// visible to the pod (enableServiceLinks defaults to true and is never set
// to false anywhere under k8s/ — checked), and this config's six upstream
// services (ORDER_SERVICE_HOST, PRODUCT_SERVICE_HOST, ...) share their
// family prefix with the k8s Service object of the same name (e.g. the
// "order-service" Service injects ORDER_SERVICE_SERVICE_HOST,
// ORDER_SERVICE_PORT, ORDER_SERVICE_PORT_9915_TCP_ADDR, ... — all in the
// "ORDER_" family this struct already owns). looksLikeK8sServiceLink
// recognizes these by their structural *shape* (the fixed suffixes
// Kubernetes always uses: _SERVICE_HOST, _SERVICE_PORT[_<NAME>], bare
// _PORT, and _PORT_<n>_TCP[_PROTO|_PORT|_ADDR]) rather than by enumerating
// the six service names, so a seventh upstream service added later needs no
// update here — and gets excluded before ever reaching the warning.
//
// A second, distinct source of family-scoped noise (final review Finding 2,
// 2026-09-19): "in the same family" is a much coarser test than "looks like
// a typo of a name we declare". RABBITMQ_HOST/VHOST/USERNAME/PASSWORD share
// the RABBITMQ_ family with the one RabbitMQ tag this struct actually
// declares (RABBITMQ_URL — this service dials Rabbit with a single DSN, it
// never reads the dissected host/vhost/user/pass; those four are only
// consumed by other, unrelated services' own configs that happen to share
// the same "hawkr-common" secret and the "MQ 連線慣例" documented in
// scripts/check-compose-k8s-align.sh). MEDIA_BASE_URL shares the MEDIA_
// family with MEDIA_SERVICE_HOST/MEDIA_SERVICE_GRPC_PORT (a downstream gRPC
// target this struct dials) purely by coincidence of English vocabulary — it
// is a display-domain constant with no reader anywhere in this service's own
// import graph (confirmed by repo-wide grep: `pkg/config.MediaBaseURLEnvKey`
// exists, but ai-service never imports or calls it; only review/social/
// gateway/notification/seller/buyer/im-service do, from their own configs).
// DB_POOL_MAX_CONNS shares the DB_ family with DB_HOST/DB_PORT/... but is
// read by pkg/database.DefaultPoolConfig via a bare os.Getenv, not through
// this struct at all — a real cross-package reader, just not a family
// sibling in the sense this check cares about.
//
// None of those four is a small edit away from the one declared name that
// put them in-family (RABBITMQ_HOST vs RABBITMQ_URL, MEDIA_BASE_URL vs
// MEDIA_SERVICE_HOST) the way a genuine typo is (GEMINI_APIKEY is exactly
// GEMINI_API_KEY minus one underscore; ORDER_SERVICE_GRPCPORT is exactly
// ORDER_SERVICE_GRPC_PORT minus one underscore). So the warning condition
// adds a second, independent test — isNearMissOfKnown, a small Levenshtein
// distance to *some* declared name — on top of the family test, instead of
// hand-listing RABBITMQ_HOST/VHOST/USERNAME/PASSWORD, MEDIA_BASE_URL and
// DB_POOL_MAX_CONNS as an exception set. This is strictly narrowing: nothing
// that used to be silent can start warning, and a real typo (which is by
// definition a near-miss of the name it typo'd) still warns. Proven against
// the real rendered dev and prod environments (helm template both overlays)
// in strict_env_test.go: zero warnings on either, while a deliberately
// typo'd variable added to that same real set still warns.
//
// DB_POOL_MAX_CONNS's genuine cross-package reader (pkg/database, not this
// struct) is not silenced by pretending it is a declared name — it is
// silenced because it fails the near-miss test the same way the other three
// do, which happens to also be true. Nothing here special-cases it by name.
func WarnUnknownEnv(cfg *Config) {
	known := collectEnvNames(reflect.ValueOf(cfg))
	families := familyPrefixes(known)

	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, ok := known[name]; ok {
			continue
		}
		if inAnyFamily(name, families) && !looksLikeK8sServiceLink(name) && isNearMissOfKnown(name, known) {
			slog.Warn("環境變數不在 config struct 的已知 env tag 內，可能是拼字錯誤或已棄用的變數",
				"variable", name)
		}
	}
}

// k8sServiceLinkSuffixes are the fixed suffix shapes Kubernetes uses for its
// Docker-links-compatibility env vars
// (https://kubernetes.io/docs/concepts/services-networking/service/#environment-variables),
// injected for every Service object visible to the pod, keyed by
// <SERVICE_NAME>_ where <SERVICE_NAME> is the k8s Service's name
// upper-cased with "-" turned into "_". Matched as a suffix so it applies to
// any service name, not just this config's own six upstream services.
var k8sServiceLinkSuffixes = []*regexp.Regexp{
	regexp.MustCompile(`_SERVICE_HOST$`),
	regexp.MustCompile(`_SERVICE_PORT(_[A-Z0-9]+)?$`),
	regexp.MustCompile(`_PORT$`),
	regexp.MustCompile(`_PORT_[0-9]+_(TCP|UDP)(_(PROTO|PORT|ADDR))?$`),
}

// looksLikeK8sServiceLink reports whether name has the shape of one of
// Kubernetes' own auto-injected Service-discovery env vars, regardless of
// which Service it names.
func looksLikeK8sServiceLink(name string) bool {
	for _, re := range k8sServiceLinkSuffixes {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// collectEnvNames walks v (following pointers, recursing into nested
// structs) and returns the set of every name declared by an `env:"..."` tag.
// A tag may list more than one name comma-separated for a fallback chain
// (DBPort reads DB_POOL_PORT, falling back to DB_PORT — both are genuinely
// consulted by Load, so both must count as known).
func collectEnvNames(v reflect.Value) map[string]struct{} {
	names := make(map[string]struct{})

	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if tag, ok := field.Tag.Lookup(envTagKey); ok {
				for _, name := range strings.Split(tag, ",") {
					if name = strings.TrimSpace(name); name != "" {
						names[name] = struct{}{}
					}
				}
			}
			if fv := v.Field(i); fv.Kind() == reflect.Struct || fv.Kind() == reflect.Pointer {
				walk(fv)
			}
		}
	}
	walk(v)

	return names
}

// familyPrefixes derives "<first token>_" from every known name that
// contains an underscore (e.g. "DB_HOST" ⇒ "DB_"). A name with no
// underscore (e.g. "ENV") contributes no family — there is no natural
// sub-namespace to scope by, so it is only ever matched exactly.
func familyPrefixes(known map[string]struct{}) map[string]struct{} {
	families := make(map[string]struct{})
	for name := range known {
		if idx := strings.Index(name, "_"); idx > 0 {
			families[name[:idx+1]] = struct{}{}
		}
	}
	return families
}

func inAnyFamily(name string, families map[string]struct{}) bool {
	for prefix := range families {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// nearMissThreshold is the maximum Levenshtein distance at which an
// undeclared name still counts as "probably a typo of" a declared one. 1
// covers exactly a missing/extra separator (GEMINI_APIKEY vs
// GEMINI_API_KEY, ORDER_SERVICE_GRPCPORT vs ORDER_SERVICE_GRPC_PORT); 2
// leaves a little room for a genuine single-character slip (e.g.
// DB_POOL_PROT for DB_POOL_PORT) without opening the door to names that
// merely share a family prefix but denote something else entirely (see
// WarnUnknownEnv's doc comment for the three real examples this excludes).
const nearMissThreshold = 2

// isNearMissOfKnown reports whether name is within nearMissThreshold edits
// of at least one declared name. This is the second, independent test
// WarnUnknownEnv applies on top of family scoping — see its doc comment for
// why family scoping alone (shared prefix) is too coarse a proxy for "looks
// like a typo of something we declare".
func isNearMissOfKnown(name string, known map[string]struct{}) bool {
	for k := range known {
		if levenshtein(name, k) <= nearMissThreshold {
			return true
		}
	}
	return false
}

// levenshtein computes the classic edit distance (insert/delete/substitute,
// each cost 1) between a and b, operating on bytes — env var names are
// ASCII (uppercase letters, digits, underscore) so byte-wise is equivalent
// to rune-wise here and avoids a []rune conversion.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
