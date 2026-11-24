package notification

import (
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// SubscribeGovernance 把治理與交易側的來源事件登記成同步訂閱者：每個只在發布的 transaction 內登記意圖並排 job（intake 的 On*，規則見 internal/intake/AGENTS.md）。
// serve 與 worker 的組裝都呼叫（cmd/hawkr 的 subscribeNotification）；每個事件一個訂閱者。
func (m *Module) SubscribeGovernance(bus *events.Bus) {
	in := m.intake
	events.Subscribe[seller.StoreSuspended](bus, in.OnStoreSuspended)
	events.Subscribe[seller.StoreUnsuspended](bus, in.OnStoreUnsuspended)
	events.Subscribe[seller.MeasureCreated](bus, in.OnMeasureCreated)
	events.Subscribe[seller.MeasureRevised](bus, in.OnMeasureRevised)
	events.Subscribe[seller.MeasureEnded](bus, in.OnMeasureEnded)
	events.Subscribe[seller.MeasureRevoked](bus, in.OnMeasureRevoked)
	events.Subscribe[seller.MeasureRequestDecided](bus, in.OnMeasureRequestDecided)
	events.Subscribe[seller.KYCApproved](bus, in.OnKYCApproved)
	events.Subscribe[seller.KYCRejected](bus, in.OnKYCRejected)
	events.Subscribe[seller.CaseInquiryCreated](bus, in.OnCaseInquiryCreated)
	events.Subscribe[seller.StorePendingDeletion](bus, in.OnStorePendingDeletion)
	events.Subscribe[seller.ReportReceived](bus, in.OnSellerReportReceived)
	events.Subscribe[seller.ReportResolved](bus, in.OnSellerReportResolved)
	events.Subscribe[buyer.BuyerDeletionRequested](bus, in.OnBuyerDeletionRequested)
	events.Subscribe[moderation.ReportReceived](bus, in.OnModerationReportReceived)
	events.Subscribe[moderation.CaseClosedEvent](bus, in.OnCaseClosed)
	events.Subscribe[social.ContentHidden](bus, in.OnContentHidden)
	events.Subscribe[social.ContentRestored](bus, in.OnContentRestored)
	events.Subscribe[social.BuyerWarned](bus, in.OnBuyerWarned)
	events.Subscribe[review.ReviewHidden](bus, in.OnReviewHidden)
	events.Subscribe[review.ReviewRestored](bus, in.OnReviewRestored)
	events.Subscribe[im.MessageHidden](bus, in.OnMessageHidden)
	events.Subscribe[im.MessageRestored](bus, in.OnMessageRestored)
	events.Subscribe[payment.PayoutAccountChanged](bus, in.OnPayoutAccountChanged)
	events.Subscribe[payment.PayoutDeadlineApproaching](bus, in.OnPayoutDeadlineApproaching)
	events.Subscribe[delegation.DelegationOperationCompleted](bus, in.OnDelegationOperationCompleted)
	events.Subscribe[procurement.RfqPublished](bus, in.OnRfqPublished)
}
