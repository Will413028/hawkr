package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Page-trap 收斂（10 檔 §2 統一 Envelope）：以下 7 支端點曾把 records/total/page/pageSize
// 掛在 envelope 頂層與 data 並列，前端得先分辨「這支是哪一種形狀」才讀得到資料。
// 收斂後一律 {code,message,data:{records,total,page,pageSize}}，其中 total/page/pageSize
// 為字串（§4 對外數字一律 string）。
//
// 本測試對七支各打一次真實路由，逐支斷言：
//   - envelope 頂層不得再出現任何分頁欄位（pageData 內建的回歸守門）
//   - data.records 是陣列且筆數正確
//   - data.total / data.page / data.pageSize 是字串且值正確
//
// 兩支帶額外欄位的端點（member-cards 明細的 totalSpending/totalRefund、商品評論的 stats）
// 額外斷言那些欄位也在 data 之下、且沒有在收斂過程中被弄丟。
func assertPageEnvelope(t *testing.T, body map[string]interface{}, wantRecords int, wantTotal, wantPage, wantPageSize string) map[string]interface{} {
	t.Helper()
	assertCode(t, body, 0)
	if got := len(pageRecords(t, body)); got != wantRecords {
		t.Errorf("data.records len = %d, want %d", got, wantRecords)
	}
	data := pageData(t, body)
	for _, kv := range []struct{ key, want string }{
		{"total", wantTotal}, {"page", wantPage}, {"pageSize", wantPageSize},
	} {
		if got := data[kv.key]; got != kv.want {
			t.Errorf("data.%s = %v, want %q", kv.key, got, kv.want)
		}
	}
	return data
}

func doGet(t *testing.T, r *gin.Engine, path string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusOK)
	return parseJSON(t, w.Body)
}
