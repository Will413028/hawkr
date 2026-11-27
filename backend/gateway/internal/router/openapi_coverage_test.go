package router

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v2"
)

const specPath = "../../../api/openapi/openapi.yaml"

const maxRESTOperations = 228

const (
	notAnnotatedDelegated = "代行營運（delegatedHandle 註冊）的 handler 沒有 swag 註解"
	binaryProxy           = "反向代理到下游的 binary 串流，沒有 swag 註解"
	notAnnotated          = "handler 沒有 swag 註解"
	swagSkippedAnnotation = "swag 註解後面接的是 type 宣告，swag 沒有收錄"
	localDevTool          = "本機開發工具（APP_ENV=local／dev 才註冊），不屬於對外 API"
)

var routesNotInSpec = map[string]string{
	"GET /api/v1/admin/delegated-operations/{}/audit-logs":                  notAnnotatedDelegated,
	"GET /api/v1/admin/delegated-operations/{}/summary":                     notAnnotatedDelegated,
	"GET /api/v1/admin/delegated-operations/current/summary":                notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/orders/{}":                                notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/orders/{}/recipient-contact":              notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/products":                                 notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/products/{}":                              notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/storefront":                               notAnnotatedDelegated,
	"PATCH /api/v1/admin/sellers/{}/products/{}":                            notAnnotatedDelegated,
	"PATCH /api/v1/admin/sellers/{}/storefront":                             notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products":                                notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/{}/publish":                     notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/{}/unpublish":                   notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/batch-unpublish":                notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/storefront/cover":                        notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/storefront/logo":                         notAnnotatedDelegated,
	"GET /api/v1/admin/invoices/documents/{}/file/content":                  binaryProxy,
	"GET /api/v1/admin/invoices/jobs/{}/manual_unlocks/{}/evidence/content": binaryProxy,
	"GET /api/v1/admin/invoices/tax_reviews/{}/documents/{}/content":        binaryProxy,
	"GET /api/v1/app/orders/{}/invoice_documents/{}/file/content":           binaryProxy,
	"GET /api/v1/app/seller/invoices/documents/{}/file/content":             binaryProxy,
	"GET /api/v1/app/hashtags":                                              notAnnotated,
	"GET /docs/{}":                                                          localDevTool,
	"POST /docs/login":                                                      localDevTool,
}

var pathParam = regexp.MustCompile(`/(?:[:*][^/]+|\{[^}]+\})`)

func routeKey(method, path string) string {
	return strings.ToUpper(method) + " " + pathParam.ReplaceAllString(path, "/{}")
}

func specRoutes(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for path, ops := range doc.Paths {
		for method, op := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				out[routeKey(method, path)] = op.OperationID
			}
		}
	}
	return out
}

var holderMethod = regexp.MustCompile(`\.\(\*Holder\)\.([A-Za-z0-9_]+)-fm$`)

func TestEveryRouteIsInOpenAPISpec(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	r := buildFullRouter(t)
	spec := specRoutes(t)

	routes := map[string]string{}
	for _, ri := range r.Routes() {
		routes[routeKey(ri.Method, ri.Path)] = ri.Handler
	}

	var missing, ghost, stale, misnamed []string
	for k, handler := range routes {
		opID, inSpec := spec[k]
		if inSpec {
			m := holderMethod.FindStringSubmatch(handler)
			if m == nil || strings.ToLower(m[1][:1])+m[1][1:] != opID {
				misnamed = append(misnamed, fmt.Sprintf("%s：operationId %q，handler %s", k, opID, handler))
			}
			continue
		}
		if _, ok := routesNotInSpec[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range spec {
		if _, ok := routes[k]; !ok {
			ghost = append(ghost, k)
		}
	}
	for k := range routesNotInSpec {
		_, inSpec := spec[k]
		if _, routed := routes[k]; inSpec || !routed {
			stale = append(stale, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(ghost)
	sort.Strings(stale)
	sort.Strings(misnamed)
	for _, k := range missing {
		t.Errorf("新增的 REST 路由：%s —— 舊 gateway 不再新增 REST，新的對外 API 用 Connect（ADR 2026-09-26-external-contract-protobuf-connect）", k)
	}
	for _, k := range ghost {
		t.Errorf("規格裡的端點沒有對應的路由：%s", k)
	}
	for _, k := range stale {
		t.Errorf("routesNotInSpec 的 %s 已經在規格裡或路由已不存在，從清單移除", k)
	}
	switch n := len(spec); {
	case n > maxRESTOperations:
		t.Errorf("清單有 %d 個端點，超過上限 %d：舊 gateway 不再新增 REST，新的對外 API 用 Connect", n, maxRESTOperations)
	case n < maxRESTOperations:
		t.Errorf("清單剩 %d 個端點：把 maxRESTOperations 改成 %d，鎖住新低點", n, n)
	}
	for _, k := range misnamed {
		t.Errorf("operationId 與 handler 方法名不一致（Check B 會跳過這支 handler）：%s", k)
	}
}
