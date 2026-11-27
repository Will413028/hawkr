package notification

import (
	"context"
	"reflect"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

type subscription struct {
	event    string
	types    []string
	erasure  bool
	register func(*events.Bus)
}

func eventName[E any]() string {
	t := reflect.TypeFor[E]()
	const marker = "/internal/modules/"
	path := t.PkgPath()
	i := strings.Index(path, marker)
	if i < 0 {
		return t.String()
	}
	module, _, _ := strings.Cut(path[i+len(marker):], "/")
	return module + "." + t.Name()
}

func notify[E any](h func(context.Context, E) error, types ...string) subscription {
	return subscription{event: eventName[E](), types: types, register: func(b *events.Bus) { events.Subscribe(b, h) }}
}

func notifyNamed[E any](name string, h func(context.Context, E) error, types ...string) subscription {
	return subscription{event: eventName[E](), types: types, register: func(b *events.Bus) { events.SubscribeNamed(b, name, h) }}
}

func erase[E any](h func(context.Context, E) error) subscription {
	return subscription{event: eventName[E](), erasure: true, register: func(b *events.Bus) { events.Subscribe(b, h) }}
}

func (m *Module) subscriptions() []subscription {
	in := m.intake
	return []subscription{
		notify(m.OnSuspended, "account_suspended"),
		erase(m.PurgeTokensOnSuspended),
		notify(m.OnUnsuspended, "account_unsuspended"),

		notify(m.OnCommentCreated, "comment_created"),
		notify(m.OnCommentRepliedToBuyer, "comment_replied_buyer"),
		notify(m.OnCommentRepliedToStore, "comment_replied_store"),
		notify(m.OnCommentLikedBuyerAuthor, "comment_liked_buyer"),
		notify(m.OnCommentLikedStoreAuthor, "comment_liked_store"),
		notify(m.OnPostLiked, "post_liked"),
		notify(m.OnPostPublished, "post_published"),
		notify(m.OnSellerFollowed, "store_followed"),
		notify(m.OnRestockTargeted, "product_restocked"),
		notify(m.OnReviewCreated, "review_created"),
		notifyNamed(coupon.ExpiringHandler, m.OnVoucherExpiring, "voucher_expiring"),
		notify(m.OnMessageSentToBuyer, "chat_message_buyer"),
		notify(m.OnMessageSentToStore, "chat_message_store"),

		erase(m.OnAccountDeleted),
		erase(m.PurgeTokensOnAccountDeleted),
		erase(m.OnBuyerDeleted),
		erase(m.OnStoreRetired),

		notify(in.OnStoreSuspended, "store_suspended"),
		notify(in.OnStoreUnsuspended, "store_unsuspended"),
		notify(in.OnMeasureCreated, "store_content_taken_down", "store_measure_issued"),
		notify(in.OnMeasureRevised, "store_measure_revised"),
		notify(in.OnMeasureEnded, "store_measure_ended"),
		notify(in.OnMeasureRevoked, "store_measure_revoked"),
		notify(in.OnMeasureRequestDecided, "store_measure_request_decided"),
		notify(in.OnKYCApproved, "kyc_approved"),
		notify(in.OnKYCRejected, "kyc_rejected"),
		notify(in.OnCaseInquiryCreated, "store_case_inquiry"),
		notify(in.OnStorePendingDeletion, "store_pending_deletion"),
		notify(in.OnSellerReportReceived, "report_received"),
		notify(in.OnSellerReportResolved, "seller_report_resolved"),
		notify(in.OnBuyerDeletionRequested, "buyer_deletion_requested"),
		notify(in.OnModerationReportReceived, "report_received", "store_report_received"),
		notify(in.OnCaseClosed, "moderation_case_closed_buyers", "moderation_case_closed_stores"),
		notify(in.OnContentHidden, "content_hidden"),
		notify(in.OnContentRestored, "content_restored"),
		notify(in.OnBuyerWarned, "author_warned"),
		notify(in.OnReviewHidden, "content_hidden"),
		notify(in.OnReviewRestored, "content_restored"),
		notify(in.OnMessageHidden, "content_hidden"),
		notify(in.OnMessageRestored, "content_restored"),
		notify(in.OnPayoutAccountChanged, "payout_account_activated"),
		notify(in.OnPayoutDeadlineApproaching, "payout_account_deadline"),
		notify(in.OnDelegationOperationCompleted, "delegation_operation_completed"),
		notify(in.OnRfqPublished, "rfq_invited"),
	}
}

// Subscribe 把來源事件登記成同步訂閱者：每個只在發布的 transaction 內登記意圖並排 job（intake 的 On*，規則見 internal/intake/AGENTS.md），
// 刪除與退場事件排個資清除 job，停權與刪帳號另在同一個 transaction 刪掉帳號的裝置 token。serve 與 worker 的組裝都呼叫（cmd/hawkr 的 subscribeNotification）。
func (m *Module) Subscribe(bus *events.Bus) {
	for _, s := range m.subscriptions() {
		s.register(bus)
	}
}
