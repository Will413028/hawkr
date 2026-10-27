package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// TestEnvelopeAndPageDataAreEnvelopePkgAliases 鎖住 2026-08-14 pkg/envelope
// 抽取計畫的核心不變式：response.Envelope／response.PageData 必須是
// pkg/envelope 對應型別的「型別別名」（type alias），不是結構相同但名字不同
// 的獨立型別——否則 gateway 既有 80+ 處 response.Envelope{...} /
// response.PageData{...} 字面值建構、以及 swag `@Success ... response.Envelope`
// 標註會全部要求逐一改寫。若改成非別名的具名型別，本測試會直接編譯失敗
// （cannot use response.Envelope as envelope.Envelope），而不是斷言失敗。
//
//nolint:staticcheck
func TestEnvelopeAndPageDataAreEnvelopePkgAliases(t *testing.T) {
	var e Envelope = envelope.Envelope{Code: 0, Message: "success"}
	var _ envelope.Envelope = e
	if e.Message != "success" {
		t.Fatalf("Message = %q, want success", e.Message)
	}

	var p PageData = envelope.PageData{Total: "1", Page: "1", PageSize: "20"}
	var _ envelope.PageData = p
	if p.Total != "1" {
		t.Fatalf("Total = %q, want 1", p.Total)
	}
}

// TestGrpcDomainErr_StaysEquivalentToGrpcErr 鎖住 GrpcDomainErr 的別名契約。
//
// 它的 doc comment 向 19 個既有呼叫點保證「兩者行為完全一致」。本地 GrpcErr 比
// pkg/envelope 版多了代行業務碼的前綴還原（PermissionDenied／InvalidArgument 兩個
// 分支），所以只要有人把 GrpcDomainErr 改回直接委派 envelope，這個保證就在無聲中
// 失效：既有 19 個呼叫點今天都不是代行路徑，看不出任何差別，而下一個把代行
// handler 接上這個別名的人會拿到塌掉業務碼的 403。
func TestGrpcDomainErr_StaysEquivalentToGrpcErr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 挑代行專屬碼：這正是 envelope 版不會還原、本地版會還原的那一類。
	err := grpcstatus.Error(codes.PermissionDenied, "[9262] 這個能力不在授權範圍")

	run := func(fn func(*gin.Context, string, error)) (int, Envelope) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/delegated-operations/x", nil)
		fn(c, "account", err)
		var env Envelope
		if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
			t.Fatalf("unmarshal: %v", e)
		}
		return w.Code, env
	}

	wantStatus, wantEnv := run(GrpcErr)
	gotStatus, gotEnv := run(GrpcDomainErr)

	if gotStatus != wantStatus {
		t.Errorf("HTTP status: GrpcDomainErr=%d, GrpcErr=%d —— 別名已與本尊分家", gotStatus, wantStatus)
	}
	if gotEnv.Code != wantEnv.Code {
		t.Errorf("envelope code: GrpcDomainErr=%d, GrpcErr=%d", gotEnv.Code, wantEnv.Code)
	}
	if gotEnv.Message != wantEnv.Message {
		t.Errorf("message: GrpcDomainErr=%q, GrpcErr=%q", gotEnv.Message, wantEnv.Message)
	}
	if wantEnv.Code != 9262 {
		t.Fatalf("前置條件不成立：GrpcErr 沒有還原 9262（拿到 %d），本測試失去鑑別力", wantEnv.Code)
	}
}

// TestGrpcErr_DelegatesToPkgEnvelope 確認 response.GrpcErr 轉發後行為不變——
// 完整的業務碼透傳／HTTP status 映射案例已搬進
// pkg/envelope/envelope_test.go（TestGrpcErr_*），這裡只驗證轉發本身沒漏接。
func TestGrpcErr_DelegatesToPkgEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/app/checkout/validate", nil)

	err := grpcstatus.Error(codes.FailedPrecondition, "[4015] 不可購買自己上架的商品")
	GrpcErr(c, "order", err)

	if w.Code != http.StatusConflict {
		t.Fatalf("HTTP status = %d, want 409", w.Code)
	}
	var env Envelope
	if e := json.Unmarshal(w.Body.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal: %v", e)
	}
	if env.Code != 4015 {
		t.Errorf("code = %d, want 4015", env.Code)
	}
	if env.Message != "不可購買自己上架的商品" {
		t.Errorf("message = %q, want prefix stripped", env.Message)
	}
}
