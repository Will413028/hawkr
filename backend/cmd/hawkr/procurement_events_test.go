package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// ExecuteRfqPublication 在同 transaction 發布 RfqPublished，沒有事件匯流排就無法發布：採購模組沒給匯流排時 New 就失敗（TestNewRejectsMissingRequiredDeps），
// 組裝函式（newProcurement、newProcurementWorker）的簽章要求匯流排，所以漏接在編譯時就擋下；這個測試釘住 serve 路徑的組裝函式把匯流排交給模組。
func TestProcurementAssemblyPassesTheEventBus(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if newProcurement(new(db.DB), nil, nil, nil, events.NewBus(), log) == nil {
		t.Fatal("serve 角色的採購模組沒組出來")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("沒有事件匯流排，組裝要失敗")
		}
	}()
	newProcurement(new(db.DB), nil, nil, nil, nil, log)
}
