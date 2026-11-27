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

// 商品認證申請與審核的五個業務碼（19 號設計書 §11.1，#2755）。
//
// 沒有這張登錄，五個碼會**全部**塌縮成 409（Gateway 對未登錄業務碼的既有行為）。
// 那個失敗完全靜默：HTTP 有回、envelope 有 code、前端也拿得到，只是「這筆申請不存在」
// 與「版本衝突」在 status 層長得一模一樣 ⇒ ADM 分不出該回列表還是該重抓詳情。
// 這支測試就是那張登錄唯一的守門人。
func TestStatusForBizCode_ProductCertification(t *testing.T) {
	want := map[int32]int{
		bizcode.CertApplicationNotFound:         http.StatusNotFound,
		bizcode.CertApplicationVersionConflict:  http.StatusConflict,
		bizcode.CertApplicationStateInvalid:     http.StatusConflict,
		bizcode.CertReviewReasonRequired:        http.StatusBadRequest,
		bizcode.CertApplicationDuplicatePending: http.StatusConflict,
	}
	for code, status := range want {
		if got := StatusForBizCode(code); got != status {
			t.Errorf("業務碼 %d → HTTP %d，預期 %d", code, got, status)
		}
	}

	if StatusForBizCode(bizcode.CertApplicationNotFound) == StatusForBizCode(bizcode.CertApplicationVersionConflict) {
		t.Error("3025 與 3026 的 HTTP status 相同 —— 登錄表沒有產生任何分流效果")
	}

	if got := StatusForBizCode(3024); got != http.StatusConflict {
		t.Errorf("未登錄的 3024 → HTTP %d，預期 409 塌縮", got)
	}
}

// product-service 的業務碼**實際上就是**以 gRPC status ＋ "[NNNN] " 前綴回來的
// （EnvelopeGuardInterceptor 把「gRPC OK ＋ 非零 envelope code」轉成 gRPC error）。
// 那條路徑走 writeBizCodeEnvelopeMapped，必須吃到登錄表。
//
// ⚠️ **FailedPrecondition 是這裡最重要的一支**：interceptor 對沒有登錄在
// envelopeCodeToGRPC 的業務碼一律用它，所以 3025–3029 五個碼走的全是這一支。
// GrpcErr 原本只在 PermissionDenied／InvalidArgument 兩個分支查表 ⇒ 3025／3028
// 在 dev 上雙雙塌成 409（#2755）。下面每一個 gRPC 分支都必須查表，漏一個就是一條
// 繞過登錄表的暗路。
func TestGrpcErrRestoresProductCertificationBizCodes(t *testing.T) {
	cases := []struct {
		name     string
		grpcCode codes.Code
		message  string
		wantHTTP int
		wantCode int32
	}{
		{"InvalidArgument 帶 3025 走登錄表的 404", codes.InvalidArgument, "[3025] 認證申請不存在", http.StatusNotFound, bizcode.CertApplicationNotFound},
		{"InvalidArgument 帶 3028 維持 400", codes.InvalidArgument, "[3028] 請填寫駁回原因", http.StatusBadRequest, bizcode.CertReviewReasonRequired},
		{"PermissionDenied 帶 3026 走登錄表的 409", codes.PermissionDenied, "[3026] 版本已變更", http.StatusConflict, bizcode.CertApplicationVersionConflict},
		{"FailedPrecondition 帶 3025 走登錄表的 404", codes.FailedPrecondition, "[3025] 認證申請不存在", http.StatusNotFound, bizcode.CertApplicationNotFound},
		{"FailedPrecondition 帶 3028 走登錄表的 400", codes.FailedPrecondition, "[3028] 請填寫駁回原因", http.StatusBadRequest, bizcode.CertReviewReasonRequired},
		{"FailedPrecondition 帶 3027 維持 409", codes.FailedPrecondition, "[3027] 狀態不允許", http.StatusConflict, bizcode.CertApplicationStateInvalid},
		{"FailedPrecondition 帶未登錄的 3024 仍塌縮 409", codes.FailedPrecondition, "[3024] 其他認證錯誤", http.StatusConflict, 3024},
		{"NotFound 帶未登錄的 3021 維持 404", codes.NotFound, "[3021] 商品不存在", http.StatusNotFound, 3021},
		{"ResourceExhausted 帶未登錄的 4291 維持 429", codes.ResourceExhausted, "[4291] 本週額度已用完", http.StatusTooManyRequests, 4291},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/x", nil)
			GrpcErr(c, "product", grpcstatus.Error(tc.grpcCode, tc.message))

			var env struct {
				Code int32 `json:"code"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &env)
			if w.Code != tc.wantHTTP || env.Code != tc.wantCode {
				t.Fatalf("HTTP %d / code %d，預期 HTTP %d / code %d（body=%s）",
					w.Code, env.Code, tc.wantHTTP, tc.wantCode, w.Body.String())
			}
		})
	}
}
