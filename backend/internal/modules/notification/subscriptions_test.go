package notification

import (
	"slices"
	"sort"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
)

// 事件裁決表（docs/plans/2026-11-23-a8-notification/event-adjudication.md）裡每個沒標「不做」的單體事件，與它登記的型別。
// 覆蓋測試以這份清單對照 subscriptions() 這張表：表少一列、多一列、或某列的型別不同，都會紅。
var adjudicatedEvents = map[string][]string{
	"accountsuspension.Suspended":             {"account_suspended"},
	"accountsuspension.Unsuspended":           {"account_unsuspended"},
	"social.CommentCreated":                   {"comment_created"},
	"social.CommentReplied":                   {"comment_replied_buyer", "comment_replied_store"},
	"social.CommentLiked":                     {"comment_liked_buyer", "comment_liked_store"},
	"social.PostLiked":                        {"post_liked"},
	"social.PostPublished":                    {"post_published"},
	"buyer.SellerFollowed":                    {"store_followed"},
	"buyer.RestockTargeted":                   {"product_restocked"},
	"review.ReviewCreated":                    {"review_created"},
	"im.MessageSent":                          {"chat_message_buyer", "chat_message_store"},
	"coupon.VoucherExpiring":                  {"voucher_expiring"},
	"seller.StoreSuspended":                   {"store_suspended"},
	"seller.StoreUnsuspended":                 {"store_unsuspended"},
	"seller.MeasureCreated":                   {"store_content_taken_down", "store_measure_issued"},
	"seller.MeasureRevised":                   {"store_measure_revised"},
	"seller.MeasureEnded":                     {"store_measure_ended"},
	"seller.MeasureRevoked":                   {"store_measure_revoked"},
	"seller.MeasureRequestDecided":            {"store_measure_request_decided"},
	"seller.CaseInquiryCreated":               {"store_case_inquiry"},
	"seller.KYCApproved":                      {"kyc_approved"},
	"seller.KYCRejected":                      {"kyc_rejected"},
	"seller.StorePendingDeletion":             {"store_pending_deletion"},
	"seller.ReportReceived":                   {"report_received"},
	"seller.ReportResolved":                   {"seller_report_resolved"},
	"buyer.BuyerDeletionRequested":            {"buyer_deletion_requested"},
	"moderation.ReportReceived":               {"report_received", "store_report_received"},
	"moderation.CaseClosedEvent":              {"moderation_case_closed_buyers", "moderation_case_closed_stores"},
	"social.ContentHidden":                    {"content_hidden"},
	"social.ContentRestored":                  {"content_restored"},
	"social.BuyerWarned":                      {"author_warned"},
	"review.ReviewHidden":                     {"content_hidden"},
	"review.ReviewRestored":                   {"content_restored"},
	"im.MessageHidden":                        {"content_hidden"},
	"im.MessageRestored":                      {"content_restored"},
	"payment.PayoutAccountChanged":            {"payout_account_activated"},
	"payment.PayoutDeadlineApproaching":       {"payout_account_deadline"},
	"delegation.DelegationOperationCompleted": {"delegation_operation_completed"},
	"procurement.RfqPublished":                {"rfq_invited"},
}

// 裁決表標「不做」的單體事件：不能有登記通知型別的訂閱者（個資清除的訂閱者不算）。
var notNotifiedEvents = []string{
	"identity.AccountDeleted", "buyer.BuyerDeleted", "seller.StoreRetired",
	"buyer.FavoriteAdded", "buyer.FavoriteRemoved", "buyer.ProductViewed",
	"review.ReviewAggregateChanged", "seller.StoreExposureChanged", "seller.StoreProfileChanged",
	"seller.MeasureRequestSubmitted", "seller.MeasureRequestClosed", "seller.KYCSubmitted", "seller.KYCCancelled",
}

// 清除訂閱者訂閱的事件：刪帳、刪買家、店家退場、停權（裝置 token）。
var erasureEvents = []string{"identity.AccountDeleted", "buyer.BuyerDeleted", "seller.StoreRetired", "accountsuspension.Suspended"}

func subscribedTypes(subs []subscription) map[string][]string {
	out := map[string][]string{}
	for _, s := range subs {
		if s.erasure {
			continue
		}
		out[s.event] = append(out[s.event], s.types...)
	}
	for _, ts := range out {
		sort.Strings(ts)
	}
	return out
}

// 每個被訂閱的單體事件都對應至少一個已註冊的型別（清除訂閱者除外），型別名稱要在註冊表裡，而且只能是同步訂閱者可以登記的型別
// （公告與 fan-out 的子型別只由各自的派送建立）。
func TestEverySubscriptionMapsToRegisteredTypes(t *testing.T) {
	subs := (&Module{}).subscriptions()
	for _, s := range subs {
		if s.event == "" || s.register == nil {
			t.Errorf("訂閱 %+v 沒有事件或登記函式", s)
		}
		if s.erasure {
			if len(s.types) != 0 {
				t.Errorf("%s 的清除訂閱者不該登記型別：%v", s.event, s.types)
			}
			continue
		}
		if len(s.types) == 0 {
			t.Errorf("%s 的訂閱沒有對應任何通知型別，也不是清除訂閱者", s.event)
		}
		for _, name := range s.types {
			e, ok := registry.Lookup(name)
			if !ok {
				t.Errorf("%s 對應的型別 %q 不在註冊表", s.event, name)
				continue
			}
			if sp := e.Spec(); sp.Fanout || sp.Spawned {
				t.Errorf("%s 對應的型別 %q 是公告或 fan-out 子型別，不能由同步訂閱者登記", s.event, name)
			}
		}
	}
}

// 每個已註冊的型別都有訂閱者登記它；明列的例外只有公告（Fanout，後台建立）與 fan-out 的子型別（Spawned，由已被訂閱的母意圖展開）。
func TestEveryRegisteredTypeHasASubscriptionOrIsAnnouncementOrSpawned(t *testing.T) {
	subscribed := map[string]bool{}
	for _, s := range (&Module{}).subscriptions() {
		for _, name := range s.types {
			subscribed[name] = true
		}
	}
	for _, e := range registry.All() {
		sp := e.Spec()
		switch {
		case subscribed[sp.Name]:
		case sp.Fanout:
		case sp.Spawned:
			if !hasSubscribedParent(sp.Name, subscribed) {
				t.Errorf("fan-out 子型別 %q 的母意圖沒有任何訂閱者登記", sp.Name)
			}
		default:
			t.Errorf("型別 %q 沒有任何訂閱者登記它，也不是公告或 fan-out 子型別", sp.Name)
		}
	}
}

func hasSubscribedParent(child string, subscribed map[string]bool) bool {
	for _, e := range registry.All() {
		if sp := e.Spec(); sp.Broadcast && sp.Child == child && subscribed[sp.Name] {
			return true
		}
	}
	return false
}

// 裁決表的每個非「不做」事件都有訂閱，登記的型別與裁決一致；表裡沒有裁決的通知訂閱也會紅。
func TestSubscriptionsMatchTheEventAdjudication(t *testing.T) {
	got := subscribedTypes((&Module{}).subscriptions())
	for event, want := range adjudicatedEvents {
		w := slices.Clone(want)
		sort.Strings(w)
		if !slices.Equal(got[event], w) {
			t.Errorf("事件 %s 登記的型別 = %v，裁決表是 %v", event, got[event], w)
		}
	}
	for event := range got {
		if _, ok := adjudicatedEvents[event]; !ok {
			t.Errorf("事件 %s 有通知訂閱，但裁決表沒有它", event)
		}
	}
	for _, event := range notNotifiedEvents {
		if len(got[event]) != 0 {
			t.Errorf("事件 %s 裁決為不做，卻登記了型別 %v", event, got[event])
		}
	}
}

func TestErasureSubscriptionsCoverTheDeletionEvents(t *testing.T) {
	var got []string
	for _, s := range (&Module{}).subscriptions() {
		if s.erasure {
			got = append(got, s.event)
		}
	}
	for _, want := range erasureEvents {
		if !slices.Contains(got, want) {
			t.Errorf("沒有 %s 的清除訂閱者（有：%v）", want, got)
		}
	}
}
