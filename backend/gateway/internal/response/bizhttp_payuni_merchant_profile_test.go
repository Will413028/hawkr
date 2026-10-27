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

// PAYUni update_merchant 有限範圍寫入（issue #2806 Phase 2）的三個業務碼。
//
// spec `docs/superpowers/specs/2026-08-30-payuni-update-merchant-phase2-design.md`
// §6.2 明講：樂觀鎖版本衝突（9234）要回 412，跟「同 Idempotency-Key、不同
// payload」（9235）的 409 分開——409 保留給「你看到的畫面過時了，重新整理即可」，
// 412 才說得出「你手上這份草稿的版本號已經不是最新」。兩者塌縮成同一個 409，前端
// 就分不出該不該清空使用者尚未送出的草稿。
//
// 9235／9236 就算不登錄也會塌縮到 409（未登錄碼的既有預設值），仍明寫在
// bizHTTPStatus 裡——理由同表裡 9285/9290 的既有先例：不能讓一個碼靠一個跟它無關
// 的預設值苟活，讀表的人要能一眼看到「這三個碼是同一組治理語意，全部經過審視」。
func TestStatusForBizCode_PayuniMerchantProfile(t *testing.T) {
	want := map[int32]int{
		bizcode.PayuniMerchantProfileVersionConflict:      http.StatusPreconditionFailed, // 9234 → 412
		bizcode.PayuniMerchantProfileIdempotencyKeyReused: http.StatusConflict,           // 9235 → 409
		bizcode.PayuniMerchantProfileNotActive:            http.StatusConflict,           // 9236 → 409
	}
	for code, status := range want {
		if got := StatusForBizCode(code); got != status {
			t.Errorf("業務碼 %d → HTTP %d，預期 %d", code, got, status)
		}
	}

	// 鑑別力：9234 必須跟 9235/9236 是不同的 status，否則前端無法分辨「版本衝突該
	// 重抓最新值但保留草稿」跟「client bug 級別的異常，請重新操作」。
	if StatusForBizCode(bizcode.PayuniMerchantProfileVersionConflict) == StatusForBizCode(bizcode.PayuniMerchantProfileIdempotencyKeyReused) {
		t.Error("9234 與 9235 的 HTTP status 相同 —— 登錄表沒有產生任何分流效果")
	}
}

// 這三個業務碼實際上是以 gRPC FailedPrecondition ＋ "[NNNN] " 前綴回來的
// （envelopeCodeToGRPC 沒有登記它們，EnvelopeGuardInterceptor 一律走 FailedPrecondition
// 分支——跟商品認證的 3025-3029 走同一條暗路）。GrpcErr 必須真的查到表，不能只是
// StatusForBizCode 這個函式本身對，而 GrpcErr 沒接上它。
func TestGrpcErrRestoresPayuniMerchantProfileBizCodes(t *testing.T) {
	cases := []struct {
		name     string
		message  string
		wantHTTP int
		wantCode int32
	}{
		{"9234 版本衝突走登錄表的 412", "[9234] 資料已被更新，請重新整理後再試", http.StatusPreconditionFailed, bizcode.PayuniMerchantProfileVersionConflict},
		{"9235 Idempotency-Key 重用維持 409", "[9235] 這個操作已經處理過，但內容不同，請重新操作", http.StatusConflict, bizcode.PayuniMerchantProfileIdempotencyKeyReused},
		{"9236 尚未 active 維持 409", "[9236] 商家尚未完成 PayUni 綁定或審核中，暫不可修改聯絡資訊", http.StatusConflict, bizcode.PayuniMerchantProfileNotActive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
			GrpcErr(c, "seller", grpcstatus.Error(codes.FailedPrecondition, tc.message))

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
