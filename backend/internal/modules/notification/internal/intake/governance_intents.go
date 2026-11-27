package intake

import (
	"fmt"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

func measureCreatedIntent(e seller.MeasureCreated) (name string, params any, ok bool) {
	switch e.Kind {
	case seller.MeasureSuspension:
		return "", nil, false
	case seller.MeasureContentTakedown:
		return "store_content_taken_down", registry.StoreContentTakenDownParams{
			StoreID: e.StoreID, MeasureID: e.MeasureID, ItemCount: len(e.Contents), OccurredAt: e.OccurredAt,
		}, true
	case seller.MeasureWarning, seller.MeasureTermination:
		return "store_measure_issued", registry.StoreMeasureIssuedParams{
			StoreID: e.StoreID, MeasureID: e.MeasureID, Kind: string(e.Kind), OccurredAt: e.OccurredAt,
		}, true
	}
	return "", nil, false
}

func requestDecidedIntent(e seller.MeasureRequestDecided) (name string, params any, err error) {
	if e.Outcome != seller.RequestAccepted && e.Outcome != seller.RequestRejected {
		return "", nil, fmt.Errorf("措施請求的裁定 %q（種類 %q）沒有對應的通知型別", e.Outcome, e.Kind)
	}
	return "store_measure_request_decided", registry.StoreMeasureRequestDecidedParams{
		StoreID: e.StoreID, MeasureID: e.MeasureID, RequestID: e.RequestID, RequestKind: string(e.Kind), Outcome: string(e.Outcome), OccurredAt: e.OccurredAt,
	}, nil
}

func measureRevokedNotified(e seller.MeasureRevoked) bool {
	return e.Reason != seller.RevokeAppealAccepted
}

func measureEndedNotified(e seller.MeasureEnded) bool {
	return e.RequestID == uuid.UUID{}
}

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

func socialAuthorBuyer(a social.Actor) (int64, bool) {
	return a.ID, a.Kind == social.ActorBuyer
}

func messageAuthorBuyer(kind im.PartyKind, id int64) (int64, bool) {
	return id, kind == im.PartyBuyer
}
