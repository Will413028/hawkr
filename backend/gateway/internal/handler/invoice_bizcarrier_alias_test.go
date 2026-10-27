package handler

import (
	"net/http"
	"testing"

	rsp "github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
)

// 繞道 ①：把 internal/response 取別名 import。
//
// 這是 9 種繞道裡最惡毒的一種，因為它**完全不需要惡意**——純風格 refactor，
// 編譯過、gofmt 過、go vet 過、既有測試全綠。而舊 AST gate 的
// isPkgCall(call, "response", …) 硬寫識別字 "response"，於是整個檔找不到任何成功
// 出口、每個呼叫點都命中 `if len(after) == 0 { continue }`（per-site fail-OPEN）、
// 報零 finding。此後任何人刪掉端點 44 的守門，CI 全綠上 prod。
//
// 本檔整支 handler 只用別名 `rsp`，一次 import（同檔重複 import 同一個 package 會被
// staticcheck ST1019 擋下）。斷言仍然成立：rsp.OK 與 response.OK 是**同一個函式**，
// 而檢查發生在那個函式體內，不在呼叫端的寫法上。
func TestBypass01_ImportAlias(t *testing.T) {
	w := runProbe(t, func(h *Holder) gin.HandlerFunc {
		return func(c *gin.Context) {
			resp, err := h.Invoice.HealthCheckProviderAccountForAdmin(c.Request.Context(),
				&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 1, StoreId: "2685914682"})
			if err != nil {
				rsp.GrpcErr(c, "invoice", err)
				return
			}
			// 完全沒有 resp.GetCode() 檢查，且成功出口以別名呼叫。
			rsp.OK(c, convInvSellerProviderAccountData(resp.GetData()))
		}
	})
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	assertNotSwallowed(t, w)
}
