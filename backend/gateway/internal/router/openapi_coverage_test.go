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

// backend/api/openapi/openapi.yaml 是舊 gateway 還在服務的 REST 的清單（ADR
// 2026-09-26-external-contract-protobuf-connect D5）：新的對外 API 一律用 Connect，REST 只會隨模組搬遷
// 縮小。這支測試讓 gin 實際註冊的路由與清單一一對上：
//   - 清單有、路由沒有 → 清單在描述不存在的端點，一律失敗（刪路由時一起刪清單）。
//   - 路由有、清單沒有 → 新增的 REST 路由，一律失敗；既有的例外登記在 routesNotInSpec。
//   - 清單的端點數只准變少（maxRESTOperations），新增路由時連同清單一起補也會失敗；例外見 ADR
//     2026-09-26-external-contract-protobuf-connect 文末修訂（逐條登記，目前只有店主讀單篇貼文）。
//   - 規格的 operationId 必須等於 handler 方法名（首字小寫）：scripts/gateway-json-contract-check.py
//     的 Check B 靠這個對應找到 handler，對不上的 handler 會被靜默跳過。
//
// 清單只准變短：登記了卻已經在規格裡的條目也會失敗，避免清單腐化。
const specPath = "../../../api/openapi/openapi.yaml"

// maxRESTOperations 是清單的端點數上限，只准變少：模組搬遷刪掉 REST 路由後，把它改成新的端點數鎖住新低點。
const maxRESTOperations = 437

// 分類理由；路由隨模組搬遷刪除時從清單移除，不再補進規格。
const (
	notAnnotatedDelegated = "代行營運（delegatedHandle 註冊）的 handler 沒有 swag 註解"
	binaryProxy           = "反向代理到下游的 binary 串流，沒有 swag 註解"
	notAnnotated          = "handler 沒有 swag 註解"
	swagSkippedAnnotation = "swag 註解後面接的是 type 宣告，swag 沒有收錄"
	payuniWebhookProxy    = "PayUni 的 webhook，反向代理到 payuni-webhook-service，沒有 swag 註解"
	localDevTool          = "本機開發工具（APP_ENV=local／dev 才註冊），不屬於對外 API"
)

// routesNotInSpec 是沒有寫進清單的既有路由（method + 以 {} 代表參數的 path），只准變短。
var routesNotInSpec = map[string]string{
	"GET /api/v1/admin/delegated-operations/{}/audit-logs":                      notAnnotatedDelegated,
	"GET /api/v1/admin/delegated-operations/{}/summary":                         notAnnotatedDelegated,
	"GET /api/v1/admin/delegated-operations/current/summary":                    notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/orders/{}":                                    notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/orders/{}/recipient-contact":                  notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/posts":                                        notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/posts/{}":                                     notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/products":                                     notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/products/{}":                                  notAnnotatedDelegated,
	"GET /api/v1/admin/sellers/{}/storefront":                                   notAnnotatedDelegated,
	"PATCH /api/v1/admin/sellers/{}/posts/{}":                                   notAnnotatedDelegated,
	"PATCH /api/v1/admin/sellers/{}/products/{}":                                notAnnotatedDelegated,
	"PATCH /api/v1/admin/sellers/{}/storefront":                                 notAnnotatedDelegated,
	"POST /api/v1/admin/delegated-operations/media/upload_sessions":             notAnnotatedDelegated,
	"POST /api/v1/admin/delegated-operations/media/upload_sessions/{}/complete": notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/posts":                                       notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/posts/{}/publish":                            notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/posts/{}/unpublish":                          notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products":                                    notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/{}/publish":                         notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/{}/unpublish":                       notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/products/batch-unpublish":                    notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/storefront/cover":                            notAnnotatedDelegated,
	"POST /api/v1/admin/sellers/{}/storefront/logo":                             notAnnotatedDelegated,
	"GET /api/v1/admin/invoices/documents/{}/file/content":                      binaryProxy,
	"GET /api/v1/admin/invoices/jobs/{}/manual_unlocks/{}/evidence/content":     binaryProxy,
	"GET /api/v1/admin/invoices/tax_reviews/{}/documents/{}/content":            binaryProxy,
	"GET /api/v1/app/media/secure_download/{}":                                  binaryProxy,
	"GET /api/v1/app/media/secure_preview/{}":                                   binaryProxy,
	"GET /api/v1/app/orders/{}/invoice_documents/{}/file/content":               binaryProxy,
	"GET /api/v1/app/seller/invoices/documents/{}/file/content":                 binaryProxy,
	"PUT /api/v1/app/media/secure_upload/{}":                                    binaryProxy,
	"GET /api/v1/admin/notification-templates":                                  notAnnotated,
	"GET /api/v1/app/hashtags":                                                  notAnnotated,
	"POST /webhook/payuni/notify":                                               payuniWebhookProxy,
	"GET /dev/webhook-simulator":                                                localDevTool,
	"GET /dev/webhook-simulator/lookup":                                         localDevTool,
	"GET /docs/{}":                                                              localDevTool,
	"POST /dev/webhook-simulator/send":                                          localDevTool,
	"POST /docs/login":                                                          localDevTool,
}

var pathParam = regexp.MustCompile(`/(?:[:*][^/]+|\{[^}]+\})`)

func routeKey(method, path string) string {
	return strings.ToUpper(method) + " " + pathParam.ReplaceAllString(path, "/{}")
}

// specRoutes 回傳規格裡每條路由的 operationId。
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

// holderMethod 取出 gin 記錄的 handler 名稱裡的方法名，例如
// ".../handler.(*Holder).ListAdminAccounts-fm" → "ListAdminAccounts"。
var holderMethod = regexp.MustCompile(`\.\(\*Holder\)\.([A-Za-z0-9_]+)-fm$`)

func TestEveryRouteIsInOpenAPISpec(t *testing.T) {
	// dev、local 才註冊的路由也要涵蓋：規格描述的是所有環境的聯集。
	t.Setenv("APP_ENV", "local")
	r := buildFullRouter(t)
	spec := specRoutes(t)

	routes := map[string]string{} // route → gin 記錄的 handler 名稱
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
