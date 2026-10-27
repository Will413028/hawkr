package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func writeDiagnosticConfig(t *testing.T, yamlContent string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "diagnostic-body-logging.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	orig := diagnosticBodyLoggingConfigPath
	diagnosticBodyLoggingConfigPath = path
	t.Cleanup(func() { diagnosticBodyLoggingConfigPath = orig })
}

func TestLoadDiagnosticBodyLoggingRules_MissingFileReturnsNil(t *testing.T) {
	orig := diagnosticBodyLoggingConfigPath
	diagnosticBodyLoggingConfigPath = filepath.Join(t.TempDir(), "does-not-exist.yaml")
	defer func() { diagnosticBodyLoggingConfigPath = orig }()

	rules := loadDiagnosticBodyLoggingRules()
	if rules != nil {
		t.Fatalf("missing config file must yield nil rules, got: %v", rules)
	}
}

func TestLoadDiagnosticBodyLoggingRules_ParsesValidConfig(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "POST /api/v1/app/buyers/me/addresses"
    fields: ["city", "district"]
    owner: "will"
    expires_at: "`+future+`"
`)
	rules := loadDiagnosticBodyLoggingRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d: %v", len(rules), rules)
	}
	r := rules[0]
	if r.Route != "POST /api/v1/app/buyers/me/addresses" || r.Owner != "will" {
		t.Fatalf("rule parsed wrong: %+v", r)
	}
	if len(r.Fields) != 2 || r.Fields[0] != "city" || r.Fields[1] != "district" {
		t.Fatalf("fields parsed wrong: %v", r.Fields)
	}
}

func TestDiagnosticOverrideActive_ExpiredRuleIsInactive(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "GET /api/v1/app/ping"
    fields: ["foo"]
    owner: "will"
    expires_at: "`+past+`"
`)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/app/ping", nil)

	if diagnosticOverrideActive(c) {
		t.Fatal("expired rule must not be active")
	}
}

// #2506 whole-branch review Fix E.1: syntactically broken YAML must fail
// closed — loadDiagnosticBodyLoggingRules returns nil, not a partial parse.
func TestLoadDiagnosticBodyLoggingRules_MalformedYAMLReturnsNil(t *testing.T) {
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "GET /api/v1/app/ping"
    fields: ["foo"
    owner: "will"
    expires_at: "2026-01-01T00:00:00Z"
`)
	rules := loadDiagnosticBodyLoggingRules()
	if rules != nil {
		t.Fatalf("malformed YAML must yield nil rules, got: %v", rules)
	}
}

// #2506 whole-branch review Fix E.2 (proves Fix D.1): an unknown/misspelled
// key ("field:" instead of "fields:") is syntactically valid YAML that would
// have parsed successfully — with Fields left empty — under the old
// non-strict yaml.Unmarshal. UnmarshalStrict must turn it into a parse error
// instead, so this asserts nil, not a rule with empty Fields.
func TestLoadDiagnosticBodyLoggingRules_UnknownKeyReturnsNil(t *testing.T) {
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "POST /api/v1/app/ping"
    field: ["foo"]
    owner: "will"
    expires_at: "2026-01-01T00:00:00Z"
`)
	rules := loadDiagnosticBodyLoggingRules()
	if rules != nil {
		t.Fatalf("unknown/misspelled key must yield nil rules (fail closed), got: %v", rules)
	}
}

// #2506 whole-branch review Fix E.3: route matching must be METHOD+path, not
// path-only — a rule scoped to POST must not activate for a GET to the same
// path.
func TestDiagnosticOverrideActive_MethodMismatchDoesNotMatch(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "POST /api/v1/app/ping"
    fields: ["foo"]
    owner: "will"
    expires_at: "`+future+`"
`)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var active bool
	var extra map[string]struct{}
	r.GET("/api/v1/app/ping", func(c *gin.Context) {
		active = diagnosticOverrideActive(c)
		extra = diagnosticExtraAllowed(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if active {
		t.Fatal("a rule scoped to POST must not activate for a GET to the same path")
	}
	if extra != nil {
		t.Fatalf("expected nil extraAllowed on method mismatch, got: %v", extra)
	}
}

// #2506 whole-branch review Fix E.4 (proves Fix D.2): a rule with no fields
// scoped ("fields: []") must not activate the diagnostic override at all —
// otherwise it would flip prod onto the full dev/staging body-buffering path
// while claiming to scope nothing, exposing the entire base allowedKeys set.
func TestDiagnosticOverrideActive_EmptyFieldsRuleDoesNotActivate(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	writeDiagnosticConfig(t, `
diagnostic_body_logging:
  - route: "GET /api/v1/app/ping"
    fields: []
    owner: "will"
    expires_at: "`+future+`"
`)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var active bool
	var extra map[string]struct{}
	r.GET("/api/v1/app/ping", func(c *gin.Context) {
		active = diagnosticOverrideActive(c)
		extra = diagnosticExtraAllowed(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/app/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if active {
		t.Fatal("a rule with no fields scoped must not activate the diagnostic override")
	}
	if extra != nil {
		t.Fatalf("expected nil extraAllowed for an empty-fields rule, got: %v", extra)
	}
}
