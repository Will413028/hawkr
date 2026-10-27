package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// plan 8f-1：通用的 admin 機敏下載與預覽已刪除。任何管理員以連號 asset ID 讀任何機敏檔、
// 不驗歸屬、不記是誰；現在每個擁有者各自授權自己的讀取。這支測試守「舊路由不存在」，
// 以及它唯一的使用者（稅籍審核頁）改走 invoice 的端點。
func TestGenericAdminSecureMediaRoutesAreGone(t *testing.T) {
	r := buildFullRouter(t)

	for _, ri := range r.Routes() {
		if ri.Method == http.MethodGet && strings.HasPrefix(ri.Path, adminPrefix+"/media/") {
			t.Errorf("通用 admin 機敏路由復活了：%s %s", ri.Method, ri.Path)
		}
	}

	for _, path := range []string{
		"/api/v1/admin/media/secure_download/1",
		"/api/v1/admin/media/secure_preview/x",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404（不得被路由到任何 handler）", path, w.Code)
		}
	}
}

// app 端的機敏下載與預覽（只有上傳者本人）不在 8f-1 的範圍，必須還在。
func TestAppSecureMediaRoutesStillMounted(t *testing.T) {
	r := buildFullRouter(t)
	want := map[string]bool{
		"GET " + appPrefix + "/media/secure_download/:asset_id":  false,
		"GET " + appPrefix + "/media/secure_preview/:session_id": false,
	}
	for _, ri := range r.Routes() {
		if _, ok := want[ri.Method+" "+ri.Path]; ok {
			want[ri.Method+" "+ri.Path] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("app 端路由不見了：%s", k)
		}
	}
}

// 稅籍佐證檔經 invoice 的端點提供，且掛在 admin 群組的身分檢查之後：
// 沒有 admin 身分得到的是 401／403，不是 404（404 代表根本沒有這條路由）。
func TestAdminInvoiceTaxDocumentContentRouteMounted(t *testing.T) {
	r := buildFullRouter(t)

	found := false
	for _, ri := range r.Routes() {
		if ri.Method == http.MethodGet &&
			ri.Path == adminPrefix+"/invoices/tax_reviews/:store_id/documents/:document_id/content" {
			found = true
		}
	}
	if !found {
		t.Fatal("GET /api/v1/admin/invoices/tax_reviews/:store_id/documents/:document_id/content 沒有註冊")
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/invoices/tax_reviews/1/documents/1/content", nil))
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Errorf("未帶身分 = %d, want 401/403（擋在 admin 群組的身分檢查）", w.Code)
	}
}

// handler 組給 ADM 的 fileDownloadPath 必須指向實際註冊的路由。
func TestAdminTaxDocumentPathMatchesHandlerLiteral(t *testing.T) {
	got := adminPrefix + "/invoices" + strings.NewReplacer(":store_id", "123", ":document_id", "1").Replace(AdminInvoiceTaxDocumentContent)
	if want := "/api/v1/admin/invoices/tax_reviews/123/documents/1/content"; got != want {
		t.Errorf("路由組出 %q，handler 的 fileDownloadPath 寫的是 %q", got, want)
	}
}
