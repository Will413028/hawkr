package router

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Will413028/hawkr/gateway/internal/handler"
)

func adminProductCertificationRoutes() []string {
	return []string{
		"GET " + adminPrefix + "/product_certification_applications",
		"GET " + adminPrefix + "/product_certification_applications/:id",
		"GET " + adminPrefix + "/product_certification_applications/:id/audit_logs",
		"POST " + adminPrefix + "/product_certification_applications/:id/approve",
		"POST " + adminPrefix + "/product_certification_applications/:id/reject",
		"POST " + adminPrefix + "/product_certification_applications/:id/revoke",
		"POST " + adminPrefix + "/product_certification_applications/:id/reverify",
	}
}

func TestAdminProductCertificationRoutesMountedUnderRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})

	mounted := map[string]struct{}{}
	for _, ri := range r.Routes() {
		mounted[ri.Method+" "+ri.Path] = struct{}{}
	}
	noAuth := map[string]struct{}{}
	for _, k := range registeredAdminNoAuth() {
		noAuth[k] = struct{}{}
	}

	for _, k := range adminProductCertificationRoutes() {
		if _, ok := mounted[k]; !ok {
			t.Errorf("認證審核端點未掛載：%s", k)
		}
		if _, ok := noAuth[k]; ok {
			t.Errorf("認證審核端點被掛到 credential-only 群組：%s —— 代行憑證持有者將可核准／撤銷全平台的法規標章", k)
		}
	}
}

// reverify 必須是 POST 而且**只有** POST：它會打政府開放資料平台（有每日配額）並寫 DB。
// 做成 GET 會被瀏覽器預抓、代理快取與 prefetch 重放，配額在沒有人按下任何按鈕的情況下
// 被燒光（19 §9.3）。
func TestReverifyRouteIsPostOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})

	path := adminPrefix + "/product_certification_applications/:id/reverify"
	methods := map[string]bool{}
	for _, ri := range r.Routes() {
		if ri.Path == path {
			methods[ri.Method] = true
		}
	}
	if !methods["POST"] {
		t.Fatalf("%s 沒有 POST 掛載（實際 methods=%v）", path, methods)
	}
	for m := range methods {
		if m != "POST" {
			t.Errorf("%s 不得掛 %s —— 這個動作有副作用，非 POST 會被重放", path, m)
		}
	}
}

// /admin/certifications* 與 GET /admin/sellers/:store_id 是商家實名認證（KYC，`docs/modules/seller.md` §13）的前綴，KYC 審核在單體
// （SellerAdminService.GetKYCApplication、ApproveKYC、RejectKYC…），商品認證刻意不佔用同一段（路由前綴共用會讓 gin 的
// param 衝突在啟動時才爆，或安靜地讓其中一組蓋住另一組）。這條測試守「不會復活」：gateway 不得註冊 KYC 的後台路由
// （一旦有人在 /admin/certifications 底下加路由，不論是不是商品認證，同一份申請就有兩個審核入口），
// 商品認證只在自己的 product_certification_applications 底下。
func TestLegacyKYCAdminRoutesStayDeleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRoutes(r, &handler.Holder{})

	for _, ri := range r.Routes() {
		if strings.HasPrefix(ri.Path, adminPrefix+"/certifications") {
			t.Errorf("KYC 的 /admin/certifications* 已搬進單體，gateway 不得再註冊：%s %s", ri.Method, ri.Path)
		}
		if ri.Method == "GET" && ri.Path == adminPrefix+"/sellers/:store_id" {
			t.Errorf("GET /admin/sellers/:store_id（KYC 與 PayUni 的詳情）已搬進單體，gateway 不得再註冊：%s %s", ri.Method, ri.Path)
		}
	}

	for _, k := range adminProductCertificationRoutes() {
		if strings.Contains(k, adminPrefix+"/certifications") {
			t.Errorf("商品認證端點佔用了 /admin/certifications 的前綴：%s", k)
		}
	}
}
