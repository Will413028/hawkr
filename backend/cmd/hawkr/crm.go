package main

import (
	"log/slog"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// newCRM 組裝 crm 模組給對外入口（CrmAppService）。訂閱者（買家刪帳、店家退場）、worker 與訂單投影的 consumer 不在這裡接：
// 它們的切換是 A6-6e，現在客戶資料還由舊 crm-service 擁有，所以這裡只提供讀寫的入口。
func newCRM(d *db.DB, jc jobs.Enqueuer, buyers *buyer.Module, sellers *seller.Module, log *slog.Logger) *crm.Module {
	return crm.New(crm.Deps{DB: d, Jobs: jc, Buyers: buyers, Stores: sellers, NewID: uuid.NewV7, Now: time.Now, Logger: log})
}
