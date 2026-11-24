package registry

import (
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 治理與交易側型別在每個語系的固定輸出（含深連結）：改文案、參數格式或路徑都會在這裡紅，要改就連預期一起改。
func TestGoldenGovernanceRendering(t *testing.T) {
	cases := []struct {
		typ, params       string
		locale            uitext.Locale
		title, body, link string
	}{
		{"store_suspended", `{"store_id":11,"period_id":5,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家已被停權", "你的店家已於 2026/11/23 12:30 因平台措施被停權，部分功能受到限制。請到賣家中心查看詳情。", "/seller-center/suspended"},
		{"store_suspended", `{"store_id":11,"period_id":5,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your store has been suspended", "Your store was suspended on 2026/11/23 12:30 because of a platform measure, and some features are restricted. See the seller center for details.", "/en/seller-center/suspended"},
		{"store_unsuspended", `{"store_id":11,"period_id":5,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家已解除停權", "你的店家已於 2026/11/23 12:30 解除停權，可以恢復經營。", ""},
		{"store_unsuspended", `{"store_id":11,"period_id":5,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your store suspension has been lifted", "Your store suspension was lifted on 2026/11/23 12:30. You can run your store normally again.", ""},
		{"store_content_taken_down", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","item_count":2,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家內容已被下架", "平台已於 2026/11/23 12:30 下架你店家的 2 項內容。請到賣家中心查看措施詳情與申覆方式。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_content_taken_down", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","item_count":2,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Content from your store was taken down", "The platform took down 2 item(s) of your store's content on 2026/11/23 12:30. See the seller center for the measure and how to appeal.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_issued", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","kind":"WARNING","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家收到警告", "平台已於 2026/11/23 12:30 對你的店家發出警告。請到賣家中心查看事由與後續處理。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_issued", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","kind":"WARNING","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your store received a warning", "The platform issued a warning to your store on 2026/11/23 12:30. See the seller center for the grounds and next steps.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_revised", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","revision":2,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家措施內容已更新", "平台已於 2026/11/23 12:30 更新對你的店家的措施內容（第 2 版）。請到賣家中心查看。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_revised", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","revision":2,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "A measure on your store was updated", "The platform updated a measure on your store on 2026/11/23 12:30 (revision 2). See the seller center.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_ended", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家措施已結束", "對你的店家的一項措施已於 2026/11/23 12:30 結束。請到賣家中心查看。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_ended", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "A measure on your store has ended", "A measure on your store ended on 2026/11/23 12:30. See the seller center.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_revoked", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家措施已撤銷", "對你的店家的一項措施已於 2026/11/23 12:30 撤銷。請到賣家中心查看。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_revoked", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "A measure on your store has been revoked", "A measure on your store was revoked on 2026/11/23 12:30. See the seller center.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_request_decided", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","request_id":"0190a000-0000-7000-8000-000000000003","request_kind":"APPEAL","outcome":"REJECTED","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "申覆未獲通過", "你對店家措施提出的申覆已於 2026/11/23 12:30 審理完畢，未獲通過。請到賣家中心查看。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_request_decided", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","request_id":"0190a000-0000-7000-8000-000000000003","request_kind":"APPEAL","outcome":"REJECTED","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your appeal was not accepted", "Your appeal against a measure on your store was reviewed on 2026/11/23 12:30 and was not accepted. See the seller center.", "/en/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"store_measure_request_decided", `{"store_id":11,"measure_id":"0190a000-0000-7000-8000-000000000002","request_id":"0190a000-0000-7000-8000-000000000003","request_kind":"REMEDIATION","outcome":"ACCEPTED","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "解除請求已獲通過", "你提出的措施解除請求已於 2026/11/23 12:30 審理完畢，獲得通過，措施已結束。請到賣家中心查看。", "/seller/measures/0190a000-0000-7000-8000-000000000002"},
		{"kyc_approved", `{"store_id":11,"application_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家審核已通過", "你的店家審核已於 2026/11/23 12:30 通過。請到賣家中心繼續設定。", "/seller/onboarding"},
		{"kyc_approved", `{"store_id":11,"application_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your store verification was approved", "Your store verification was approved on 2026/11/23 12:30. Continue the setup in the seller center.", "/en/seller/onboarding"},
		{"kyc_rejected", `{"store_id":11,"application_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "店家審核被退回", "你的店家審核已於 2026/11/23 12:30 被退回。請到賣家中心查看退回原因並重新送出。", "/seller/onboarding"},
		{"kyc_rejected", `{"store_id":11,"application_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your store verification was returned", "Your store verification was returned on 2026/11/23 12:30. See the seller center for the reason and resubmit.", "/en/seller/onboarding"},
		{"store_case_inquiry", `{"store_id":11,"inquiry_id":"0190a000-0000-7000-8000-000000000003","kind":"RECORDS","due_at":"2026-11-26T04:30:00Z","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "平台對你的店家發出詢問", "平台請你的店家提供相關紀錄，請於 2026/11/26 12:30 前回覆。詳情請到賣家中心查看。", "/seller/inquiries/0190a000-0000-7000-8000-000000000003"},
		{"store_case_inquiry", `{"store_id":11,"inquiry_id":"0190a000-0000-7000-8000-000000000003","kind":"RECORDS","due_at":"2026-11-26T04:30:00Z","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "The platform has sent your store an inquiry", "The platform asked your store to provide records. Please reply by 2026/11/26 12:30. See the seller center for details.", "/en/seller/inquiries/0190a000-0000-7000-8000-000000000003"},
		{"store_pending_deletion", `{"store_id":11,"event_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "已收到帳號刪除申請，店家等待刪除", "我們於 2026/11/23 12:30 收到你的帳號刪除申請，你的店家已停止經營並進入等待刪除，此申請無法撤回。若這不是你的操作，請盡快聯絡客服。", ""},
		{"store_pending_deletion", `{"store_id":11,"event_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "We received your account deletion request", "We received your account deletion request on 2026/11/23 12:30. Your store has stopped operating and is waiting to be deleted. This request cannot be withdrawn. If this was not you, please contact support as soon as possible.", ""},
		{"buyer_deletion_requested", `{"buyer_id":42,"event_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "已收到帳號刪除申請", "我們於 2026/11/23 12:30 收到你的帳號刪除申請，待未完成的訂單處理完畢後會刪除帳號，此申請無法撤回。若這不是你的操作，請盡快聯絡客服。", ""},
		{"buyer_deletion_requested", `{"buyer_id":42,"event_id":"0190a000-0000-7000-8000-000000000007","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "We received your account deletion request", "We received your account deletion request on 2026/11/23 12:30. Your account will be deleted once any open orders are settled, and this request cannot be withdrawn. If this was not you, please contact support as soon as possible.", ""},
		{"payout_account_activated", `{"store_id":11,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "收款帳戶已啟用", "你的收款帳戶已於 2026/11/23 12:30 啟用，之後的訂單款項可以撥入。", "/seller-center/wallet"},
		{"payout_account_activated", `{"store_id":11,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your payout account is active", "Your payout account became active on 2026/11/23 12:30. Order proceeds can now be paid out to it.", "/en/seller-center/wallet"},
		{"payout_account_deadline", `{"store_id":11,"deadline":"2026-11-26T04:30:00Z","tier":"SEVEN_DAYS","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "收款帳戶補件期限將至", "你的收款帳戶還有資料待補，請在 2026/11/26 12:30 前補齊，逾期收款功能可能受到限制。", "/seller-center/wallet"},
		{"payout_account_deadline", `{"store_id":11,"deadline":"2026-11-26T04:30:00Z","tier":"SEVEN_DAYS","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your payout account documents are due soon", "Your payout account still needs information. Please provide it by 2026/11/26 12:30; after that, payouts may be restricted.", "/en/seller-center/wallet"},
		{"delegation_operation_completed", `{"audit_log_id":"0190a000-0000-7000-8000-000000000007","target_store_id":11,"target_seller_account_id":42,"capability":"products.write","action":"PRODUCT_UPDATE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "平台人員已代行操作你的店家", "平台人員於 2026/11/23 12:30 代行你的店家，更新商品。", ""},
		{"delegation_operation_completed", `{"audit_log_id":"0190a000-0000-7000-8000-000000000007","target_store_id":11,"target_seller_account_id":42,"capability":"products.write","action":"PRODUCT_UPDATE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "A platform operator acted on your store", "On 2026/11/23 12:30, a platform operator acting on your behalf updated a product.", ""},
		{"rfq_invited", `{"store_id":11,"publication_id":"0190a000-0000-7000-8000-000000000004","invite_id":"0190a000-0000-7000-8000-000000000003","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "有買家邀請你的店家報價", "有買家發出新的詢價並邀請你的店家回覆。", ""},
		{"rfq_invited", `{"store_id":11,"publication_id":"0190a000-0000-7000-8000-000000000004","invite_id":"0190a000-0000-7000-8000-000000000003","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "A buyer invited your store to quote", "A buyer published a new request for quotation and invited your store to respond.", ""},
		{"content_hidden", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"source":"SOCIAL","category":"SPAM","appeal_path":"NONE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "你的內容已被隱藏", "你的留言於 2026/11/23 12:30 因「垃圾訊息」被隱藏。", ""},
		{"content_hidden", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"source":"SOCIAL","category":"SPAM","appeal_path":"NONE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your content has been hidden", "Your comment was hidden on 2026/11/23 12:30 for the following reason: Spam.", ""},
		{"content_restored", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"source":"REVIEW","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "你的內容已恢復顯示", "你的評價已於 2026/11/23 12:30 恢復顯示。", ""},
		{"content_restored", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"source":"REVIEW","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your content has been restored", "Your review was restored on 2026/11/23 12:30.", ""},
		{"author_warned", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"category":"HARASSMENT","appeal_path":"NONE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "你收到一則警告", "你於 2026/11/23 12:30 因「騷擾或霸凌」收到平台的警告。請留意社群規範。", ""},
		{"author_warned", `{"sanction_id":"0190a000-0000-7000-8000-000000000006","buyer_id":42,"category":"HARASSMENT","appeal_path":"NONE","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "You received a warning", "You received a warning from the platform on 2026/11/23 12:30 for the following reason: Harassment or bullying. Please review the community guidelines.", ""},
		{"report_received", `{"report_source":"SELLER","report_id":"0190a000-0000-7000-8000-000000000004","buyer_id":42,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "已收到你的檢舉", "我們已收到你提交的檢舉資料，會依規定處理，處理結果會再通知你。", "/account/reports/0190a000-0000-7000-8000-000000000004"},
		{"report_received", `{"report_source":"SELLER","report_id":"0190a000-0000-7000-8000-000000000004","buyer_id":42,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "We received your report", "We received the report you submitted and will handle it according to our policies. We will notify you of the outcome.", "/en/account/reports/0190a000-0000-7000-8000-000000000004"},
		{"store_report_received", `{"report_source":"MODERATION","report_id":"0190a000-0000-7000-8000-000000000004","store_id":11,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "已收到你的檢舉", "我們已收到你提交的檢舉資料，會依規定處理，處理結果會再通知你。", ""},
		{"store_report_received", `{"report_source":"MODERATION","report_id":"0190a000-0000-7000-8000-000000000004","store_id":11,"occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "We received your report", "We received the report you submitted and will handle it according to our policies. We will notify you of the outcome.", ""},
		{"report_resolved", `{"report_source":"SELLER","report_id":"0190a000-0000-7000-8000-000000000004","buyer_id":42,"outcome":"ACTION_TAKEN","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "你的檢舉有結果了", "你的檢舉已處理完畢：平台已依規定處理。", "/account/reports/0190a000-0000-7000-8000-000000000004"},
		{"report_resolved", `{"report_source":"SELLER","report_id":"0190a000-0000-7000-8000-000000000004","buyer_id":42,"outcome":"ACTION_TAKEN","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your report has an outcome", "Your report has been handled: the platform has taken action.", "/en/account/reports/0190a000-0000-7000-8000-000000000004"},
		{"store_report_resolved", `{"report_source":"MODERATION","report_id":"0190a000-0000-7000-8000-000000000004","store_id":11,"outcome":"NO_VIOLATION","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.ZhTW, "你的檢舉有結果了", "你的檢舉已處理完畢：經審查，未發現違規。", ""},
		{"store_report_resolved", `{"report_source":"MODERATION","report_id":"0190a000-0000-7000-8000-000000000004","store_id":11,"outcome":"NO_VIOLATION","occurred_at":"2026-11-23T04:30:00Z"}`, uitext.EN, "Your report has an outcome", "Your report has been handled: after review, no violation was found.", ""},
	}
	for _, c := range cases {
		e, ok := Lookup(c.typ)
		if !ok {
			t.Fatalf("沒有型別 %s", c.typ)
		}
		got, err := e.Render(c.locale, []byte(c.params))
		if err != nil {
			t.Errorf("%s/%s：%v", c.typ, c.locale, err)
			continue
		}
		if got.Title != c.title || got.Body != c.body || got.DeepLink != c.link || got.Locale != c.locale || got.ImageKey != "" || got.AggregationKey != "" || len(got.ActorAccounts) != 0 {
			t.Errorf("%s/%s 的輸出\n got  %+v\n want %q %q %q", c.typ, c.locale, got, c.title, c.body, c.link)
		}
	}
}

// 冪等鍵的固定輸出：事件來源身分變了，同一個事件會被當成新事件；補件期限的鍵含檔位，期限或檔位不同就是新的提醒。
func TestGoldenGovernanceIdempotencyKeys(t *testing.T) {
	const id = "0190a000-0000-7000-8000-000000000002"
	const sup = "0190a000-0000-7000-8000-000000000009"
	at := "2026-11-23T04:30:00Z"
	for _, c := range []struct{ typ, params, want string }{
		{"store_suspended", `{"store_id":11,"period_id":5,"occurred_at":"` + at + `"}`, "store_suspended:11/5"},
		{"store_unsuspended", `{"store_id":11,"period_id":5,"occurred_at":"` + at + `"}`, "store_unsuspended:11/5"},
		{"store_content_taken_down", `{"store_id":11,"measure_id":"` + id + `","item_count":1,"occurred_at":"` + at + `"}`, "store_content_taken_down:" + id},
		{"store_measure_issued", `{"store_id":11,"measure_id":"` + id + `","kind":"WARNING","occurred_at":"` + at + `"}`, "store_measure_issued:" + id},
		{"store_measure_revised", `{"store_id":11,"measure_id":"` + id + `","revision":3,"occurred_at":"` + at + `"}`, "store_measure_revised:" + id + "/3"},
		{"store_measure_ended", `{"store_id":11,"measure_id":"` + id + `","occurred_at":"` + at + `"}`, "store_measure_ended:" + id},
		{"store_measure_revoked", `{"store_id":11,"measure_id":"` + id + `","occurred_at":"` + at + `"}`, "store_measure_revoked:" + id},
		{"store_measure_request_decided", `{"store_id":11,"measure_id":"` + sup + `","request_id":"` + id + `","request_kind":"APPEAL","outcome":"REJECTED","occurred_at":"` + at + `"}`, "store_measure_request_decided:" + id},
		{"kyc_approved", `{"store_id":11,"application_id":"` + id + `","occurred_at":"` + at + `"}`, "kyc_approved:" + id},
		{"kyc_rejected", `{"store_id":11,"application_id":"` + id + `","occurred_at":"` + at + `"}`, "kyc_rejected:" + id},
		{"store_case_inquiry", `{"store_id":11,"inquiry_id":"` + id + `","kind":"RECORDS","due_at":"` + at + `","occurred_at":"` + at + `"}`, "store_case_inquiry:" + id},
		{"store_pending_deletion", `{"store_id":11,"event_id":"` + id + `","occurred_at":"` + at + `"}`, "store_pending_deletion:" + id},
		{"buyer_deletion_requested", `{"buyer_id":42,"event_id":"` + id + `","occurred_at":"` + at + `"}`, "buyer_deletion_requested:" + id},
		{"payout_account_activated", `{"store_id":11,"occurred_at":"` + at + `"}`, "payout_account_activated:11"},
		{"payout_account_deadline", `{"store_id":11,"deadline":"2026-11-26T04:30:00Z","tier":"SEVEN_DAYS","occurred_at":"` + at + `"}`, "payout_account_deadline:11/1795667400/SEVEN_DAYS"},
		{"payout_account_deadline", `{"store_id":11,"deadline":"2026-11-26T04:30:00Z","tier":"ONE_DAY","occurred_at":"` + at + `"}`, "payout_account_deadline:11/1795667400/ONE_DAY"},
		{"delegation_operation_completed", `{"audit_log_id":"` + id + `","target_store_id":11,"target_seller_account_id":42,"capability":"products.write","action":"PRODUCT_UPDATE","occurred_at":"` + at + `"}`, "delegation_operation_completed:" + id},
		{"rfq_invited", `{"store_id":11,"publication_id":"` + sup + `","invite_id":"` + id + `","occurred_at":"` + at + `"}`, "rfq_invited:" + id},
		{"content_hidden", `{"sanction_id":"` + id + `","buyer_id":42,"source":"SOCIAL","category":"SPAM","appeal_path":"NONE","occurred_at":"` + at + `"}`, "content_hidden:" + id},
		{"content_restored", `{"sanction_id":"` + id + `","buyer_id":42,"source":"SOCIAL","occurred_at":"` + at + `"}`, "content_restored:" + id},
		{"author_warned", `{"sanction_id":"` + id + `","buyer_id":42,"category":"SPAM","appeal_path":"NONE","occurred_at":"` + at + `"}`, "author_warned:" + id},
		{"report_received", `{"report_source":"SELLER","report_id":"` + id + `","buyer_id":42,"occurred_at":"` + at + `"}`, "report_received:" + id},
		{"report_received", `{"report_source":"SELLER","report_id":"` + id + `","supplement_id":"` + sup + `","buyer_id":42,"occurred_at":"` + at + `"}`, "report_received:" + id + "/" + sup},
		{"store_report_received", `{"report_source":"MODERATION","report_id":"` + id + `","store_id":11,"occurred_at":"` + at + `"}`, "store_report_received:" + id},
		{"seller_report_resolved", `{"report_id":"` + id + `","buyer_id":42,"outcome":"ACTION_TAKEN","occurred_at":"` + at + `"}`, "seller_report_resolved:" + id},
		{"moderation_case_closed_buyers", `{"case_id":"` + id + `","outcome":"VIOLATION","occurred_at":"` + at + `"}`, "moderation_case_closed_buyers:" + id},
		{"moderation_case_closed_stores", `{"case_id":"` + id + `","outcome":"VIOLATION","occurred_at":"` + at + `"}`, "moderation_case_closed_stores:" + id},
	} {
		e, ok := Lookup(c.typ)
		if !ok {
			t.Fatalf("沒有型別 %s", c.typ)
		}
		got, err := e.IdempotencyKey([]byte(c.params))
		if err != nil || got != c.want {
			t.Errorf("%s 的冪等鍵 = %q（%v），預期 %q", c.typ, got, err, c.want)
		}
	}
}

// fan-out 子型別沒有自己的冪等鍵，鍵由母意圖與名次（或買家號碼）決定。
func TestGovernanceSpawnedKeysFollowTheParentAndTheTarget(t *testing.T) {
	parent := uuid.MustParse("0190a000-0000-7000-8000-000000000002").String()
	if got, want := SpawnedKey("report_resolved", "moderation_case_closed_buyers:"+parent, 3), "report_resolved:moderation_case_closed_buyers:"+parent+"/3"; got != want {
		t.Fatalf("子意圖的鍵 = %q，預期 %q", got, want)
	}
	for _, name := range []string{"report_resolved", "store_report_resolved"} {
		e, _ := Lookup(name)
		if !e.Spec().Spawned || e.Spec().Audience() != map[string]model.Audience{"report_resolved": model.AudienceBuyer, "store_report_resolved": model.AudienceStore}[name] {
			t.Errorf("%s 要是對應受眾的 Spawned 型別：%+v", name, e.Spec())
		}
	}
}
