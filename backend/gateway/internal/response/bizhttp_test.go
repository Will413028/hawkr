package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func runGrpcErr(t *testing.T, err error) (int, int32, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)
	GrpcErr(c, "product", err)

	var env struct {
		Code    int32  `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Code, env.Message
}

// 領域服務的 delegation.Validate 以 InvalidArgument / PermissionDenied ＋ "[NNNN] "
// 前綴回傳 9260 / 9262。不解析前綴，業務碼在 HTTP 層整個掉光，前端只剩一個沒有恢復
// 指引的通用 400／403。
func TestGrpcErrRestoresDelegationBizCodes(t *testing.T) {
	cases := []struct {
		name     string
		grpcCode codes.Code
		message  string
		wantHTTP int
		wantCode int32
		wantMsg  string
	}{
		{"InvalidArgument 帶 9260", codes.InvalidArgument, "[9260] 代行操作原因為必填", http.StatusBadRequest, bizcode.DelegationInvalidCommand, "代行操作原因為必填"},
		{"PermissionDenied 帶 9262", codes.PermissionDenied, "[9262] 此工作階段未取得該能力", http.StatusForbidden, bizcode.DelegationCapabilityDenied, "此工作階段未取得該能力"},
		{"PermissionDenied 帶 9275", codes.PermissionDenied, "[9275] 資源不屬於代行對象", http.StatusForbidden, bizcode.DelegationResourceNotOwned, "資源不屬於代行對象"},
		// 登錄表決定 status：同一個 gRPC 分支也可以吐 422，這正是 §6.5.2 要的分流。
		{"PermissionDenied 帶 9272 走登錄表的 422", codes.PermissionDenied, "[9272] 店家狀態不允許代行", http.StatusUnprocessableEntity, bizcode.DelegationStoreNotOperable, "店家狀態不允許代行"},
		// 無前綴時逐位元維持既有行為。
		{"InvalidArgument 無前綴", codes.InvalidArgument, "bad request", http.StatusBadRequest, 400, "bad request"},
		{"PermissionDenied 無前綴", codes.PermissionDenied, "forbidden", http.StatusForbidden, 403, "forbidden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, msg := runGrpcErr(t, grpcstatus.Error(tc.grpcCode, tc.message))
			if status != tc.wantHTTP || code != tc.wantCode || msg != tc.wantMsg {
				t.Fatalf("HTTP %d / code %d / msg %q，預期 HTTP %d / code %d / msg %q",
					status, code, msg, tc.wantHTTP, tc.wantCode, tc.wantMsg)
			}
		})
	}
}

func TestStatusForBizCode(t *testing.T) {
	want := map[int32]int{
		bizcode.DelegationInvalidCommand:       http.StatusBadRequest,
		bizcode.DelegationAdminSessionRequired: http.StatusUnauthorized,
		bizcode.DelegationCredentialInvalid:    http.StatusUnauthorized,
		bizcode.DelegationCapabilityDenied:     http.StatusForbidden,
		bizcode.DelegationTargetMismatch:       http.StatusForbidden,
		bizcode.DelegationAuthorizationRevoked: http.StatusForbidden,
		bizcode.DelegationStoreNotDelegable:    http.StatusForbidden,
		bizcode.DelegationResourceNotOwned:     http.StatusForbidden,
		bizcode.DelegationActorMismatch:        http.StatusForbidden,
		bizcode.DelegationSessionExpired:       http.StatusGone,
		bizcode.DelegationSessionRevoked:       http.StatusGone,
		bizcode.DelegationSessionNotFound:      http.StatusGone,
		bizcode.DelegationHandoffInvalid:       http.StatusGone,
		bizcode.DelegationStoreNotOperable:     http.StatusUnprocessableEntity,
		bizcode.DelegationRateLimited:          http.StatusTooManyRequests,
		bizcode.DelegationUpstreamUnavailable:  http.StatusFailedDependency,
		bizcode.InvalidPlanCode:                http.StatusBadRequest,
	}
	for code, status := range want {
		if got := StatusForBizCode(code); got != status {
			t.Errorf("業務碼 %d → HTTP %d，預期 %d", code, got, status)
		}
	}

	// 未登錄者塌縮 409（含刻意不登錄的四個衝突碼與 9279）。
	for _, code := range []int32{
		bizcode.DelegationResourceVersionConflict,
		bizcode.DelegationIdempotencyKeyReused,
		bizcode.DelegationConflictActorOtherStore,
		bizcode.DelegationConflictStoreOtherActor,
		bizcode.DelegationAuditIntentFailed,
		9999,
	} {
		if got := StatusForBizCode(code); got != http.StatusConflict {
			t.Errorf("未登錄業務碼 %d → HTTP %d，預期 409 塌縮", code, got)
		}
	}

	// 9276 的 HTTP 是 200 且走 response.Biz，刻意不進這張「業務拒絕」表。
	if _, listed := bizHTTPStatus[bizcode.DelegationBatchPartialFailed]; listed {
		t.Error("9276 不得登錄進 bizHTTPStatus")
	}
}

// TestBizHTTPStatus_NoServerErrorStatuses 守住這張表唯一的可用性硬前提：**登錄值一律
// 4xx**。
//
// 這不是風格偏好。Cloudflare 站在 Gateway 前面，origin 回 5xx 時它會把整個 body 換掉：
// 叢集內直打 gateway 拿到 `{"code":9280,…}` ＋ `application/json`，同一支端點經 CF 變成
// `error code: 502` ＋ `text/plain` ⇒ 前端 `response.json()` 直接拋錯，**業務碼永遠送不
// 到前端**，而這張表存在的全部理由就是把業務碼送到前端。9280 曾登錄成 502，正是這個形狀。
//
// 反向的顧慮（C12：404／503 被讀成「此環境尚未部署代行營運服務」並讓入口永久隱藏）只約束
// 那兩個碼，不構成回頭選 5xx 的理由——4xx 的語意是「這一次請求出了事」而不是「這個服務不
// 存在」。上游不可用要表達「依賴的下游失敗」時用 424 Failed Dependency。
func TestBizHTTPStatus_NoServerErrorStatuses(t *testing.T) {
	for code, status := range bizHTTPStatus {
		if status >= 500 {
			t.Errorf("業務碼 %d 登錄成 HTTP %d：5xx 的 body 會被 Cloudflare 換成自己的錯誤頁，"+
				"前端拿不到這個業務碼。要表達「上游不可用」請用 424 Failed Dependency", code, status)
		}
		if status < 400 {
			t.Errorf("業務碼 %d 登錄成 HTTP %d：這張表只收「業務拒絕 → 4xx」，"+
				"2xx／3xx 走 response.Biz 或 response.OK", code, status)
		}
	}
}

func TestBizStatusAndData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	BizStatus(c, bizcode.DelegationSessionExpired, "已到期")
	if w.Code != http.StatusGone {
		t.Fatalf("BizStatus 寫出 HTTP %d，預期 410", w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
	BizStatusData(c, bizcode.DelegationResourceVersionConflict, "版本衝突", gin.H{"currentVersion": "13"})
	if w.Code != http.StatusConflict {
		t.Fatalf("BizStatusData 寫出 HTTP %d，預期 409", w.Code)
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Data["currentVersion"] != "13" {
		t.Fatalf("BizStatusData 未帶出 data：%s", w.Body.String())
	}
}
