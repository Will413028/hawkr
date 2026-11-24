package intake

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// 這個檔案放治理訂閱者用的純函式：由事件決定「登記哪個型別、什麼參數」，沒有任何 I/O（subscribers_governance.go 只准呼叫它們與 Accept）。
// 事件的列舉值在這裡轉成註冊表的字串（註冊表不 import 來源模組）；兩邊的值由 governance_intents_test.go 對照。

// measureCreatedIntent 決定措施建立事件的型別：內容下架、警告與終止各自通知；停權措施由 StoreSuspended 通知（不重複）。
func measureCreatedIntent(e seller.MeasureCreated) (name string, params any, ok bool) {
	switch e.Kind {
	case seller.MeasureSuspension:
		return "", nil, false
	case seller.MeasureContentTakedown:
		return "store_content_taken_down", registry.StoreContentTakenDownParams{
			StoreID: e.StoreID, MeasureID: e.MeasureID, ItemCount: len(e.Contents), OccurredAt: e.OccurredAt,
		}, true
	}
	return "store_measure_issued", registry.StoreMeasureIssuedParams{
		StoreID: e.StoreID, MeasureID: e.MeasureID, Kind: string(e.Kind), OccurredAt: e.OccurredAt,
	}, true
}

// requestDecidedIntent 決定請求裁定事件的型別：接受申覆時同 transaction 也發 MeasureRevoked、接受解除請求時也發 MeasureEnded，所以接受的裁定
// 登記成 store_measure_revoked、store_measure_ended（同一個 MeasureID 的冪等鍵，同一次處置只通知一則）；駁回才是 store_measure_request_decided。
func requestDecidedIntent(e seller.MeasureRequestDecided) (name string, params any, err error) {
	state := registry.StoreMeasureStateParams{StoreID: e.StoreID, MeasureID: e.MeasureID, OccurredAt: e.OccurredAt}
	switch {
	case e.Outcome == seller.RequestAccepted && e.Kind == seller.RequestKindAppeal:
		return "store_measure_revoked", state, nil
	case e.Outcome == seller.RequestAccepted && e.Kind == seller.RequestKindRemediation:
		return "store_measure_ended", state, nil
	case e.Outcome == seller.RequestRejected:
		return "store_measure_request_decided", registry.StoreMeasureRequestDecidedParams{
			StoreID: e.StoreID, MeasureID: e.MeasureID, RequestID: e.RequestID, RequestKind: string(e.Kind), OccurredAt: e.OccurredAt,
		}, nil
	}
	return "", nil, fmt.Errorf("措施請求的裁定 %q（種類 %q）沒有對應的通知型別", e.Outcome, e.Kind)
}

// moderationReportReceivedIntent 決定 moderation 檢舉確認的型別：買家檢舉人走 report_received，店家檢舉人走 store_report_received；恰好一個為正。
func moderationReportReceivedIntent(e moderation.ReportReceived) (name string, params any, err error) {
	p := registry.ReportReceivedParams{
		ReportSource: registry.ReportSourceModeration, ReportID: e.ReportID, BuyerID: e.ReporterBuyerID, StoreID: e.ReporterStoreID, OccurredAt: e.OccurredAt,
	}
	switch {
	case e.ReporterBuyerID > 0 && e.ReporterStoreID == 0:
		return "report_received", p, nil
	case e.ReporterStoreID > 0 && e.ReporterBuyerID == 0:
		return "store_report_received", p, nil
	}
	return "", nil, fmt.Errorf("moderation 檢舉 %s 的檢舉人要恰好有買家或店家一個（%d、%d）", e.ReportID, e.ReporterBuyerID, e.ReporterStoreID)
}

// socialAuthorBuyer 回傳社群內容作者的買家號碼；店家作者的內容走 seller 的措施，不在作者通知的範圍。
func socialAuthorBuyer(a social.Actor) (int64, bool) {
	return a.ID, a.Kind == social.ActorBuyer
}

// messageAuthorBuyer 回傳聊天訊息作者的買家號碼；店家寄的訊息 im 不隱藏，這裡同樣不通知店家作者。
func messageAuthorBuyer(kind im.PartyKind, id int64) (int64, bool) {
	return id, kind == im.PartyBuyer
}
