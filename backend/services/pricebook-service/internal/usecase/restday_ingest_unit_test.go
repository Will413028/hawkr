package usecase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// TestRestDayIngestHandleRejectsEmptyResponse 驗證 P4 review followup 補的
// guard：MOA 對空 Data 回 200（壅塞時的已知行為，姊妹路徑 PriceIngest.Handle
// 已有同樣的 guard），RestDayIngest.Handle 必須回錯誤，不能靜默回 (0, nil)
// 讓稽核表看起來像一次「成功但什麼都沒 ingest 到」的執行。
//
// 不需要真的 Postgres：guard 在任何 DB 呼叫之前就 return，pool 傳 nil 安全。
func TestRestDayIngestHandleRejectsEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"RS":"OK","Next":false,"Data":[]}`))
	}))
	t.Cleanup(srv.Close)

	u := usecase.NewRestDayIngest(moa.NewClient(srv.URL, srv.URL), nil, func() int64 { return 1 }, zap.NewNop())

	written, err := u.Handle(context.Background())
	if err == nil {
		t.Fatal("Handle 對空 Data 回應應該回錯誤，不能靜默回 (0, nil)")
	}
	if written != 0 {
		t.Errorf("written = %d，預期 0", written)
	}
}
