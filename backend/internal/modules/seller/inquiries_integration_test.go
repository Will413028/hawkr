//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/jackc/pgx/v5/pgconn"
)

// 案件的詢問（step10e-3cb-design §7.4）。管理員 21 發詢問、22 記回覆、23 撤回。

const (
	inquiryCreator  = 21
	inquiryRecorder = 22
	inquiryWithdraw = 23
)

// foodCase 在 storeID 上開一個進行中的食安案件（一筆通報）。
func (f *fixture) foodCase(t *testing.T, storeID int64) AdminCase {
	t.Helper()
	c, _ := f.newCase(t, storeID, policy.FoodSafety, SeverityHigh, 1)
	return c
}

func (f *fixture) inquire(t *testing.T, caseID uuid.UUID, kind InquiryKind, message string) (CreateCaseInquiryResult, error) {
	t.Helper()
	return f.module.CreateCaseInquiry(context.Background(), CreateCaseInquiryInput{CaseID: caseID, AdminID: inquiryCreator, Kind: kind, Message: message})
}

func (f *fixture) mustInquire(t *testing.T, caseID uuid.UUID) CreateCaseInquiryResult {
	t.Helper()
	r, err := f.inquire(t, caseID, InquiryKindRecords, "請提供這批商品的來源與進貨紀錄")
	must(t, err)
	return r
}

// reply 記下回覆（管理員 22），收到的時間是 receivedAt。
func (f *fixture) reply(t *testing.T, inquiryID uuid.UUID, receivedAt time.Time, tokens ...string) (RecordCaseInquiryReplyResult, error) {
	t.Helper()
	return f.module.RecordCaseInquiryReply(context.Background(), RecordCaseInquiryReplyInput{
		InquiryID: inquiryID, AdminID: inquiryRecorder, Channel: InquiryChannelPhone, ReceivedAt: receivedAt, Body: "賣家電話回覆：已附上進貨單", EvidenceTokens: tokens,
	})
}

func (f *fixture) inquiriesOf(t *testing.T, caseID uuid.UUID) []AdminInquiry {
	t.Helper()
	var out []AdminInquiry
	var after *InquiryCursor
	for {
		page, err := f.module.CaseInquiries(context.Background(), CaseInquiryListInput{CaseID: caseID, PageSize: 2, After: after})
		must(t, err)
		out = append(out, page.Items...)
		if page.Next == nil {
			return out
		}
		after = page.Next
	}
}

// 發詢問：案件進入「等賣家」（不在待決定），期限預設 72 小時，案件的版本與 updated_at 前進，不發事件（沒有消費者）；
// 同時只能有一筆還沒結束的；輸入不合法什麼都不寫。
func TestCreateInquiryStartsTheWaitAndAllowsOnlyOneOpenInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	if !(c.Open() && !c.AwaitingSeller()) || c.AwaitingSeller() || len(c.Availability.CreateInquiry) != 0 {
		t.Fatalf("發詢問之前：%+v", c.Case)
	}
	f.now = f.now.Add(time.Hour)
	r, err := f.inquire(t, c.ID, InquiryKindExplanation, "  請說明這批商品的保存方式  ")
	must(t, err)
	i := r.Inquiry
	if r.Replayed || i.State() != InquiryStateOpen || i.Kind != InquiryKindExplanation || i.Message != "請說明這批商品的保存方式" || i.CaseID != c.ID || i.StoreID != s.StoreID ||
		i.CreatedByAdminID != inquiryCreator || !i.CreatedAt.Equal(f.now) || !i.DueAt.Equal(f.now.Add(InquiryDefaultDue)) ||
		len(i.Availability.RecordReply) != 0 || len(i.Availability.Withdraw) != 0 {
		t.Fatalf("inquiry=%+v", i)
	}
	got := r.Case
	if !got.AwaitingSeller() || got.OpenInquiryID == nil || *got.OpenInquiryID != i.ID || got.Version != c.Version+1 || !got.UpdatedAt.Equal(f.now) ||
		len(got.Availability.CreateInquiry) == 0 || len(got.Availability.Decide) == 0 {
		t.Fatalf("case=%+v", got.Case)
	}
	if got.DecideDeadline != nil {
		t.Fatalf("等賣家的案件沒有決定期限：%+v", got.DecideDeadline)
	}

	// 同時只能有一筆還沒結束的。
	_, err = f.inquire(t, c.ID, InquiryKindRecords, "再問一次")
	wantBlocked(t, err, "這個案件已有一筆詢問在等賣家回覆。")
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE case_id = $1", c.ID) != 1 {
		t.Fatal("被擋下的詢問寫入了")
	}

	// 輸入不合法：指到欄位，什麼都不寫。
	other := f.foodCase(t, f.anotherStore(t).StoreID)
	due := func(d time.Duration) *time.Time { v := f.now.Add(d); return &v }
	for name, tt := range map[string]struct {
		in    CreateCaseInquiryInput
		field string
	}{
		"沒選種類":      {CreateCaseInquiryInput{Message: "x"}, "Kind"},
		"種類不認得":     {CreateCaseInquiryInput{Kind: "PHONE", Message: "x"}, "Kind"},
		"說明是空白":     {CreateCaseInquiryInput{Kind: InquiryKindRecords, Message: " \n "}, "Message"},
		"說明太長":      {CreateCaseInquiryInput{Kind: InquiryKindRecords, Message: strings.Repeat("字", 2001)}, "Message"},
		"期限早於 1 天":  {CreateCaseInquiryInput{Kind: InquiryKindRecords, Message: "x", DueAt: due(24*time.Hour - time.Second)}, "DueAt"},
		"期限晚於 14 天": {CreateCaseInquiryInput{Kind: InquiryKindRecords, Message: "x", DueAt: due(14*24*time.Hour + time.Second)}, "DueAt"},
		"期限是過去":     {CreateCaseInquiryInput{Kind: InquiryKindRecords, Message: "x", DueAt: due(-time.Hour)}, "DueAt"},
	} {
		tt.in.CaseID, tt.in.AdminID = other.ID, inquiryCreator
		_, err := f.module.CreateCaseInquiry(ctx, tt.in)
		if fields := invalidFields(t, err); !slices.Equal(fields, []string{tt.field}) {
			t.Errorf("%s：欄位=%v err=%v", name, fields, err)
		}
	}
	if _, err := f.module.CreateCaseInquiry(ctx, CreateCaseInquiryInput{CaseID: other.ID, Kind: InquiryKindRecords, Message: "x"}); err == nil {
		t.Error("沒有管理員的指令要被擋下")
	}
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE case_id = $1", other.ID) != 0 {
		t.Fatal("不合法的輸入寫入了")
	}
	// 期限的邊界：剛好 1 天與剛好 14 天可以。
	for _, d := range []time.Duration{24 * time.Hour, 14 * 24 * time.Hour} {
		c2 := f.foodCase(t, f.anotherStore(t).StoreID)
		in := CreateCaseInquiryInput{CaseID: c2.ID, AdminID: inquiryCreator, Kind: InquiryKindRecords, Message: "x", DueAt: due(d)}
		r, err := f.module.CreateCaseInquiry(ctx, in)
		must(t, err)
		if !r.Inquiry.DueAt.Equal(*in.DueAt) {
			t.Errorf("期限 %v：%v", d, r.Inquiry.DueAt)
		}
	}
	// 案件不存在。
	_, err = f.inquire(t, uuid.NewV7(), InquiryKindRecords, "x")
	wantKind(t, err, apperr.KindNotFound)
	f.assertCaseInvariants(t)
}

// 已結案的案件不能發詢問。
func TestCreateInquiryIsBlockedOnAClosedCase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	_, err := f.decide(t, c.ID, DecisionNoAction, "")
	must(t, err)
	_, err = f.inquire(t, c.ID, InquiryKindRecords, "x")
	wantBlocked(t, err, "案件已結案。")
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries") != 0 {
		t.Fatal("已結案的案件寫入了詢問")
	}
}

// 關店不擋詢問（帳號還在，條款第 11.3 條的義務與案件都還在）；只有店家的個資已依保留期限清除（沒有聯絡資料可以問）才擋。
// 已開著的詢問在關店與清理時都不動，照常可以記回覆或撤回。
func TestCreateInquiryIsBlockedOnlyOnceTheStoreWasPurged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	waiting := f.foodCase(t, s.StoreID)
	open := f.mustInquire(t, waiting.ID)
	idle, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)

	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if got := f.caseOf(t, idle.ID); len(got.Availability.CreateInquiry) != 0 || !f.caseOf(t, waiting.ID).AwaitingSeller() {
		t.Fatalf("只是關店不擋：%v", got.Availability.CreateInquiry)
	}
	r, err := f.inquire(t, idle.ID, InquiryKindRecords, "店家已關閉，帳號還在")
	must(t, err)
	_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: r.Inquiry.ID, AdminID: inquiryWithdraw})
	must(t, err)

	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 || !f.purged(t, s.StoreID) {
		t.Fatalf("purged=%d", n)
	}
	_, err = f.inquire(t, idle.ID, InquiryKindRecords, "x")
	wantBlocked(t, err, "店家資料已依保留期限清除，沒有聯絡資料可以詢問賣家。")
	if got := f.caseOf(t, idle.ID); len(got.Availability.CreateInquiry) == 0 {
		t.Fatal("已清理的店家，案件不能發詢問（can_create_inquiry 要是 false）")
	}
	// 已開著的詢問不動，照常逾期、可以記回覆或撤回。
	still := f.inquiriesOf(t, waiting.ID)
	if len(still) != 1 || still[0].State() != InquiryStateOpen || len(still[0].Availability.RecordReply) != 0 || len(still[0].Availability.Withdraw) != 0 {
		t.Fatalf("清理之後仍開著的詢問：%+v", still)
	}
	if open.Inquiry.DueAt.Add(time.Second).Before(still[0].DueAt) {
		t.Fatal("逾期照常")
	}
	_, err = f.reply(t, open.Inquiry.ID, f.now.Add(-time.Hour))
	must(t, err)
	f.assertCaseInvariants(t)
}

// 冪等重送（AIP-155）：同一位管理員、同一個 request_id、同樣的內容回原本那一筆（目前的狀態），內容不同回 invalid_argument；範圍是 (管理員, request_id)，
// 另一位管理員用同一個值是另一個請求；重送先於有狀態的驗證（原本那一筆已回覆、案件已結案之後，重送仍回原本那一筆）；期限不在重送時重新檢查。
func TestCreateInquiryRequestIDReplaysTheSameInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	req := uuid.NewV4()
	due := f.now.Add(25 * time.Hour)
	in := CreateCaseInquiryInput{CaseID: c.ID, AdminID: inquiryCreator, Kind: InquiryKindRecords, Message: "提供紀錄", DueAt: &due, RequestID: &req}
	first, err := f.module.CreateCaseInquiry(ctx, in)
	must(t, err)
	if first.Replayed {
		t.Fatal("第一次不是重送")
	}
	version := first.Case.Version

	f.now = f.now.Add(2 * time.Hour) // 期限只剩 23 小時：重送不重新檢查範圍
	again, err := f.module.CreateCaseInquiry(ctx, in)
	must(t, err)
	if !again.Replayed || again.Inquiry.ID != first.Inquiry.ID || again.Case.Version != version || f.count(t, "SELECT count(*) FROM seller.case_inquiries") != 1 {
		t.Fatalf("again=%+v", again)
	}
	changed := in
	changed.Message = "提供紀錄與說明"
	_, err = f.module.CreateCaseInquiry(ctx, changed)
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"RequestID"}) {
		t.Fatalf("內容不同：%v", err)
	}
	changed = in
	changed.CaseID = f.foodCase(t, f.anotherStore(t).StoreID).ID
	_, err = f.module.CreateCaseInquiry(ctx, changed)
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"RequestID"}) {
		t.Fatalf("同一個 request_id 帶別的案件：%v", err)
	}
	// 另一位管理員用同一個 request_id：另一個請求，不會讀回別人建立的詢問。
	other := f.foodCase(t, f.anotherStore(t).StoreID)
	mine := CreateCaseInquiryInput{CaseID: other.ID, AdminID: inquiryCreator + 1, Kind: InquiryKindRecords, Message: "另一位管理員", RequestID: &req}
	got, err := f.module.CreateCaseInquiry(ctx, mine)
	must(t, err)
	if got.Replayed || got.Inquiry.ID == first.Inquiry.ID || got.Inquiry.CreatedByAdminID != inquiryCreator+1 {
		t.Fatalf("範圍是 (管理員, request_id)：%+v", got.Inquiry)
	}
	// 原本那一筆已回覆、案件已結案：重送仍回原本那一筆（目前的狀態），不是「案件已結案」。
	_, err = f.reply(t, first.Inquiry.ID, f.now.Add(-time.Hour))
	must(t, err)
	_, err = f.decide(t, c.ID, DecisionNoAction, "")
	must(t, err)
	late, err := f.module.CreateCaseInquiry(ctx, in)
	must(t, err)
	if !late.Replayed || late.Inquiry.ID != first.Inquiry.ID || late.Inquiry.State() != InquiryStateReplied || late.Case.Open() {
		t.Fatalf("late=%+v", late)
	}
}

// 兩個並行的發詢問（沒有 request_id）：鎖住案件之後 Availability 擋下後到的，只有一筆。
func TestConcurrentInquiriesOnOneCaseLeaveExactlyOneOpen(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			_, errs[i] = f.inquire(t, c.ID, InquiryKindRecords, fmt.Sprintf("並行 %d", i))
		})
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		wantBlocked(t, err, "這個案件已有一筆詢問在等賣家回覆。")
	}
	if ok != 1 || f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE case_id = $1", c.ID) != 1 {
		t.Fatalf("成功 %d 筆", ok)
	}
	// 同一個 request_id 的並行重送：只有一筆，其餘回同一筆。
	other := f.foodCase(t, f.anotherStore(t).StoreID)
	req := uuid.NewV4()
	ids := make([]uuid.UUID, n)
	begin := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-begin
			r, err := f.module.CreateCaseInquiry(context.Background(), CreateCaseInquiryInput{CaseID: other.ID, AdminID: inquiryCreator, Kind: InquiryKindRecords, Message: "同一個請求", RequestID: &req})
			if err != nil {
				t.Errorf("並行重送：%v", err)
				return
			}
			ids[i] = r.Inquiry.ID
		})
	}
	close(begin)
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("並行重送要回同一筆：%v", ids)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE case_id = $1", other.ID) != 1 {
		t.Fatal("並行重送寫了多筆")
	}
}

// 收到的時間的精確度是秒：與詢問建立同一秒、但早於建立時間（建立有微秒）的收到時間記為建立當下（資料庫保證收到不早於建立），不被擋；早於建立那一秒的還是被擋。
// 後台預設填入的「現在」是秒，剛建立就開對話框的回覆不能因此失敗。
// mutation：下限改回以微秒比較（received.Before(CreatedAt)）→ 同一秒內的回覆被擋，斷言失敗；拿掉 received = CreatedAt → 資料庫的 CHECK 失敗，斷言失敗。
func TestInquiryReplyReceivedInTheCreationSecondIsRecordedAsCreationTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	f.now = f.now.Truncate(time.Second).Add(640*time.Millisecond + 123*time.Microsecond)
	i := f.mustInquire(t, c.ID).Inquiry
	if !i.CreatedAt.Equal(f.now) {
		t.Fatalf("建立時間要帶微秒：%s", i.CreatedAt)
	}
	f.now = f.now.Add(time.Hour)
	_, err := f.reply(t, i.ID, i.CreatedAt.Truncate(time.Second).Add(-time.Nanosecond))
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"ReceivedAt"}) {
		t.Fatalf("早於建立那一秒要被擋：%v", err)
	}
	r, err := f.reply(t, i.ID, i.CreatedAt.Truncate(time.Second))
	must(t, err)
	if got := r.Inquiry.Reply; got == nil || !got.ReceivedAt.Equal(i.CreatedAt) {
		t.Fatalf("同一秒的收到時間要記為建立時間：%+v", r.Inquiry.Reply)
	}
}

// 記下回覆：收到的時間不早於詢問建立、不晚於現在；附件以管理員的身分轉正、必須是機敏檔，引用登記在詢問 ID；記下之後詢問結束、案件的版本前進；
// 重送回原本那一則（已記過）；已撤回回錯誤。
func TestRecordInquiryReply(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	i := f.mustInquire(t, c.ID).Inquiry
	f.now = f.now.Add(2 * time.Hour)

	for name, tt := range map[string]struct {
		in    RecordCaseInquiryReplyInput
		field string
	}{
		"沒選管道":          {RecordCaseInquiryReplyInput{ReceivedAt: f.now, Body: "x"}, "Channel"},
		"管道不認得":         {RecordCaseInquiryReplyInput{Channel: "SMS", ReceivedAt: f.now, Body: "x"}, "Channel"},
		"沒有收到的時間":       {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, Body: "x"}, "ReceivedAt"},
		"早於詢問建立（秒）":     {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: i.CreatedAt.Truncate(time.Second).Add(-time.Nanosecond), Body: "x"}, "ReceivedAt"},
		"晚於現在":          {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: f.now.Add(time.Microsecond), Body: "x"}, "ReceivedAt"},
		"內容是空白":         {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: f.now, Body: "  "}, "Body"},
		"內容太長":          {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: f.now, Body: strings.Repeat("字", 4001)}, "Body"},
		"附件超過 5 份":      {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: f.now, Body: "x", EvidenceTokens: []string{"a", "b", "c", "d", "e", "f"}}, "EvidenceTokens"},
		"附件的 token 是空白": {RecordCaseInquiryReplyInput{Channel: InquiryChannelPhone, ReceivedAt: f.now, Body: "x", EvidenceTokens: []string{"a", " "}}, "EvidenceTokens[1]"},
	} {
		tt.in.InquiryID, tt.in.AdminID = i.ID, inquiryRecorder
		_, err := f.module.RecordCaseInquiryReply(ctx, tt.in)
		if fields := invalidFields(t, err); !slices.Equal(fields, []string{tt.field}) {
			t.Errorf("%s：欄位=%v err=%v", name, fields, err)
		}
	}
	_, err := f.reply(t, uuid.NewV7(), f.now)
	wantKind(t, err, apperr.KindNotFound)
	// 詢問不存在的句子是 Connect code NOT_FOUND 的預設文字，不是空字串。
	if ae := new(apperr.Error); !errors.As(err, &ae) || strings.TrimSpace(ae.Message) == "" {
		t.Fatalf("NotFound 要帶預設文字：%v", err)
	}
	if got := f.inquiriesOf(t, c.ID)[0]; got.State() != InquiryStateOpen || f.count(t, "SELECT count(*) FROM seller.case_inquiry_reply_evidence") != 0 {
		t.Fatalf("被拒絕的回覆寫入了：%+v", got)
	}

	// 附件必須是機敏檔：media 回報不是機敏就在寫入之前被拒絕，指到那一張。
	f.media.NotSecure = true
	_, err = f.reply(t, i.ID, f.now, "plain")
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"EvidenceTokens[0]"}) {
		t.Fatalf("非機敏的附件：%v", err)
	}
	f.media.NotSecure = false
	if f.inquiriesOf(t, c.ID)[0].State() != InquiryStateOpen {
		t.Fatal("被拒絕的附件讓詢問結束了")
	}

	before := f.caseOf(t, c.ID)
	uploaders := len(f.media.uploaders)
	got, err := f.reply(t, i.ID, f.now.Add(-30*time.Minute), "doc1", "doc2")
	must(t, err)
	r := got.Inquiry
	if got.AlreadyRecorded || r.State() != InquiryStateReplied || r.Reply == nil || r.Reply.Channel != InquiryChannelPhone || r.Reply.Body != "賣家電話回覆：已附上進貨單" ||
		!r.Reply.ReceivedAt.Equal(f.now.Add(-30*time.Minute)) || !r.Reply.RecordedAt.Equal(f.now) || r.Reply.RecordedByAdminID != inquiryRecorder || len(r.Reply.Evidence) != 2 ||
		len(r.Availability.RecordReply) == 0 || len(r.Availability.Withdraw) == 0 {
		t.Fatalf("reply=%+v", r)
	}
	if got.Case.Version != before.Version+1 || !got.Case.UpdatedAt.Equal(f.now) || got.Case.AwaitingSeller() || !got.Case.Open() {
		t.Fatalf("case=%+v", got.Case.Case)
	}
	// 附件以管理員的身分轉正（Admin: true）、ref type 是治理佐證。
	newUploaders := f.media.uploaders[uploaders:]
	if len(newUploaders) != 2 || newUploaders[0] != (media.Uploader{ID: inquiryRecorder, Admin: true}) || newUploaders[1] != (media.Uploader{ID: inquiryRecorder, Admin: true}) {
		t.Fatalf("uploaders=%+v", newUploaders)
	}
	if last := f.media.finalized[len(f.media.finalized)-2:]; last[0] != media.RefTypeGovernanceEvidence || last[1] != media.RefTypeGovernanceEvidence {
		t.Fatalf("ref type=%v", last)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.case_inquiry_reply_evidence WHERE inquiry_id = $1", i.ID); n != 2 {
		t.Fatalf("附件列=%d", n)
	}
	// 引用登記在詢問 ID（同步之後 media 看得到兩個資產）。
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: i.ID.String()}
	f.syncMedia(t, ref)
	if got := f.media.Referenced(ref); len(got) != 2 {
		t.Fatalf("引用=%v", got)
	}

	// 重送：回原本那一則，不寫入、不動版本、不比較內容。
	f.now = f.now.Add(time.Hour)
	again, err := f.module.RecordCaseInquiryReply(ctx, RecordCaseInquiryReplyInput{
		InquiryID: i.ID, AdminID: inquiryRecorder + 1, Channel: InquiryChannelEmail, ReceivedAt: f.now, Body: "別的內容",
	})
	must(t, err)
	if !again.AlreadyRecorded || again.Inquiry.Reply.Body != r.Reply.Body || again.Inquiry.Reply.Channel != InquiryChannelPhone || again.Case.Version != got.Case.Version {
		t.Fatalf("again=%+v", again.Inquiry)
	}
	if _, err := f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: i.ID, AdminID: inquiryWithdraw}); err == nil {
		t.Fatal("已回覆的詢問不能撤回")
	} else {
		wantBlocked(t, err, "這筆詢問已記下回覆。")
	}
	f.assertCaseInvariants(t)
}

// 撤回：管理員撤回還沒回覆的詢問（案件版本前進、回到待決定）；重送回 already_withdrawn；撤回之後不能記回覆。
func TestWithdrawInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	i := f.mustInquire(t, c.ID).Inquiry
	before := f.caseOf(t, c.ID)
	f.now = f.now.Add(time.Hour)
	got, err := f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: i.ID, AdminID: inquiryWithdraw})
	must(t, err)
	w := got.Inquiry.Withdrawn
	if got.AlreadyWithdrawn || got.Inquiry.State() != InquiryStateWithdrawn || w == nil || w.Cause != InquiryWithdrawAdmin || w.AdminID != inquiryWithdraw || !w.At.Equal(f.now) ||
		got.Case.Version != before.Version+1 || got.Case.AwaitingSeller() || !decisionStartOf(t, got.Case).Equal(f.now) {
		t.Fatalf("withdrawn=%+v case=%+v", got.Inquiry, got.Case.Case)
	}
	f.now = f.now.Add(time.Hour)
	again, err := f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: i.ID, AdminID: inquiryWithdraw + 1})
	must(t, err)
	if !again.AlreadyWithdrawn || again.Inquiry.Withdrawn.AdminID != inquiryWithdraw || again.Case.Version != got.Case.Version {
		t.Fatalf("again=%+v", again.Inquiry)
	}
	_, err = f.reply(t, i.ID, f.now)
	wantBlocked(t, err, "這筆詢問已撤回。")
	_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: uuid.NewV7(), AdminID: inquiryWithdraw})
	wantKind(t, err, apperr.KindNotFound)
	// 撤回之後可以再發一筆。
	f.mustInquire(t, c.ID)
	f.assertCaseInvariants(t)
}

// 晚記下的回覆讓案件回到「待決定」時，起算點是收到的時間，不是記下的時間（B11）：期限算的是平台拿到資訊之後的處理時間，不因晚登記而延長。
// 管理員撤回從撤回的時間起算；系統撤回（結案）不重新起算。
func TestALateRecordedReplyStartsTheDecisionClockAtTheReceiveTime(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	i := f.mustInquire(t, c.ID).Inquiry
	received := f.now.Add(24 * time.Hour)
	f.now = f.now.Add(3 * 24 * time.Hour)
	got, err := f.reply(t, i.ID, received)
	must(t, err)
	if start := decisionStartOf(t, got.Case); !start.Equal(received) || start.Equal(f.now) {
		t.Fatalf("待決定的開始=%v，預期收到的時間 %v（不是記下的 %v）", start, received, f.now)
	}
	// 兩筆詢問取最晚的重新起算時間：第二筆被管理員撤回，比第一筆的收到時間晚。
	f.now = f.now.Add(time.Hour)
	second := f.mustInquire(t, c.ID).Inquiry
	f.now = f.now.Add(time.Hour)
	w, err := f.module.WithdrawCaseInquiry(context.Background(), WithdrawCaseInquiryInput{InquiryID: second.ID, AdminID: inquiryWithdraw})
	must(t, err)
	if start := decisionStartOf(t, w.Case); !start.Equal(f.now) {
		t.Fatalf("待決定的開始=%v，預期最晚的撤回時間 %v", start, f.now)
	}
	// 清單與單筆的值相同（兩條 SQL 路徑）。
	open := CaseStateOpen
	page, err := f.module.AdminCases(context.Background(), CaseListInput{State: &open, StoreID: s.StoreID, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || !decisionStartOf(t, page.Items[0]).Equal(f.now) {
		t.Fatalf("清單的待決定開始=%+v", page.Items)
	}
}

// 結案時還沒結束的詢問由系統撤回（CASE_CLOSED，作者是結案的管理員）：決定，或通報都移走而以 NO_REPORTS 關閉。已回覆的詢問不動；結案不重新起算待決定。
func TestClosingTheCaseWithdrawsItsOpenInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)

	decided := f.foodCase(t, s.StoreID)
	open := f.mustInquire(t, decided.ID).Inquiry
	f.now = f.now.Add(time.Hour)
	closed, err := f.decide(t, decided.ID, DecisionNoAction, "")
	must(t, err)
	inquiries := f.inquiriesOf(t, decided.ID)
	w := inquiries[0].Withdrawn
	if len(inquiries) != 1 || inquiries[0].ID != open.ID || w == nil || w.Cause != InquiryWithdrawCaseClosed || w.AdminID != 10 || !w.At.Equal(f.now) || closed.Open() || closed.AwaitingSeller() || closed.OpenInquiryID != nil {
		t.Fatalf("inquiries=%+v case=%+v", inquiries, closed.Case)
	}
	if closed.DecideDeadline != nil {
		t.Fatalf("已結案的案件沒有決定期限（系統撤回不重新起算）：%+v", closed.DecideDeadline)
	}

	// 已回覆的詢問不動。
	replied := f.foodCase(t, f.anotherStore(t).StoreID)
	r := f.mustInquire(t, replied.ID).Inquiry
	_, err = f.reply(t, r.ID, f.now)
	must(t, err)
	_, err = f.decide(t, replied.ID, DecisionNoAction, "")
	must(t, err)
	if got := f.inquiriesOf(t, replied.ID)[0]; got.State() != InquiryStateReplied {
		t.Fatalf("已回覆的詢問被動了：%+v", got)
	}

	// 通報都移走：原案件以 NO_REPORTS 關閉，詢問由系統撤回（作者是移動的管理員 9）。
	other := f.anotherStore(t)
	food, members := f.newCase(t, other.StoreID, policy.FoodSafety, SeverityHigh, 1)
	moved := f.mustInquire(t, food.ID).Inquiry
	f.now = f.now.Add(time.Hour)
	res, err := f.move(t, members[0].ID, policy.Fraud, severityPtr(SeverityLow))
	must(t, err)
	if !res.SourceCaseClosed {
		t.Fatal("原案件要以 NO_REPORTS 關閉")
	}
	got := f.inquiriesOf(t, food.ID)
	if len(got) != 1 || got[0].ID != moved.ID || got[0].Withdrawn == nil || got[0].Withdrawn.Cause != InquiryWithdrawCaseClosed || got[0].Withdrawn.AdminID != 9 || !got[0].Withdrawn.At.Equal(f.now) {
		t.Fatalf("NO_REPORTS 結案：%+v", got)
	}
	f.assertCaseInvariants(t)
}

// 結案與記下回覆並行：不會留下已結案而還沒結束的詢問（J1），回覆與系統撤回只會有一個。
func TestDecideAndAConcurrentReplyNeverLeaveAnOpenInquiryOnAClosedCase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	for range 200 {
		c := f.foodCase(t, s.StoreID)
		i := f.mustInquire(t, c.ID).Inquiry
		version := f.caseOf(t, c.ID).Version
		var wg sync.WaitGroup
		var decideErr, replyErr error
		wg.Go(func() {
			_, decideErr = f.module.DecideCase(context.Background(), DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: version})
		})
		wg.Go(func() { _, replyErr = f.reply(t, i.ID, f.now) })
		wg.Wait()
		got := f.inquiriesOf(t, c.ID)[0]
		switch got.State() {
		case InquiryStateOpen:
			t.Fatalf("結束之後詢問還開著：%+v decide=%v reply=%v", got, decideErr, replyErr)
		case InquiryStateReplied:
			if replyErr != nil {
				t.Fatalf("回覆成功了卻回錯誤：%v", replyErr)
			}
			// 回覆先寫入，決定在案件版本前進之後以舊 etag 失敗，或在回覆之前結案——後者不可能：結案會先撤回。
			if decideErr == nil {
				t.Fatalf("回覆之後案件的版本前進，帶舊 etag 的決定要失敗")
			}
			wantKind(t, decideErr, apperr.KindVersionConflict)
		case InquiryStateWithdrawn:
			must(t, decideErr)
			wantBlocked(t, replyErr, "這筆詢問已撤回。")
		}
		// 使用過的案件收尾，讓同一家店可以再開同類別的案件。
		if f.caseOf(t, c.ID).Open() {
			_, err := f.decide(t, c.ID, DecisionNoAction, "")
			must(t, err)
		}
	}
	f.assertCaseInvariants(t)
}

// 發詢問、記下回覆與撤回都讓案件的 etag 前進：用舊 etag 的決定要失敗（它們改變「可以決定」與決定的依據）。
func TestInquiryCommandsAdvanceTheCaseVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	staleDecision := func(name string, c AdminCase) {
		t.Helper()
		_, err := f.module.DecideCase(ctx, DecideCaseInput{CaseID: c.ID, AdminID: 10, Decision: DecisionNoAction, ExpectedVersion: c.Version})
		if apperr.KindOf(err) != apperr.KindVersionConflict {
			t.Errorf("%s：用舊 etag 的決定要回版本衝突：%v", name, err)
		}
	}
	fresh := func() AdminCase { return f.foodCase(t, f.anotherStore(t).StoreID) }

	created := fresh()
	f.mustInquire(t, created.ID)
	staleDecision("發詢問", created)

	replied := fresh()
	ir := f.mustInquire(t, replied.ID)
	_, err := f.reply(t, ir.Inquiry.ID, f.now)
	must(t, err)
	staleDecision("記下回覆", ir.Case)

	withdrawn := fresh()
	iw := f.mustInquire(t, withdrawn.ID)
	_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: iw.Inquiry.ID, AdminID: inquiryWithdraw})
	must(t, err)
	staleDecision("撤回", iw.Case)
}

// 詢問附件的讀取：先寫讀取紀錄（誰、哪一張）再向 media 讀；以詢問 ID 定址，位置沒有附件、詢問不存在、種類與 ID 對不上都是 NotFound，而且不留紀錄、不問 media；
// 紀錄寫不進去就不讀。
func TestInquiryEvidenceReadIsRecordedBeforeTheRead(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	i := f.mustInquire(t, c.ID).Inquiry
	got, err := f.reply(t, i.ID, f.now, "m0", "m1")
	must(t, err)
	ref := media.Ref{Type: media.RefTypeGovernanceEvidence, ID: i.ID.String()}
	for n, e := range got.Inquiry.Reply.Evidence {
		f.media.PutSecure(e.ObjectKey, ref, media.SecureAsset{Data: []byte(fmt.Sprintf("m%d", n)), ContentType: "application/pdf"})
	}
	for name, in := range map[string]ReadEvidenceInput{
		"位置沒有附件":     {AdminID: 9, Kind: EvidenceInquiry, RefID: i.ID, Position: 2},
		"負的位置":       {AdminID: 9, Kind: EvidenceInquiry, RefID: i.ID, Position: -1},
		"位置超出上限":     {AdminID: 9, Kind: EvidenceInquiry, RefID: i.ID, Position: 5},
		"不存在的詢問":     {AdminID: 9, Kind: EvidenceInquiry, RefID: uuid.NewV7(), Position: 0},
		"以案件 ID 讀":   {AdminID: 9, Kind: EvidenceInquiry, RefID: c.ID, Position: 0},
		"以詢問 ID 讀通報": {AdminID: 9, Kind: EvidenceReport, RefID: i.ID, Position: 0},
		"以詢問 ID 讀申覆": {AdminID: 9, Kind: EvidenceRequest, RefID: i.ID, Position: 0},
	} {
		if _, err := f.module.ReadEvidence(ctx, in); apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("%s：%v", name, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.evidence_accesses") != 0 || f.readSecureCalls() != 0 {
		t.Fatal("找不到的附件不能留下紀錄或問 media")
	}
	read, err := f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceInquiry, RefID: i.ID, Position: 1})
	must(t, err)
	if string(read.Data) != "m1" || read.ContentType != "application/pdf" {
		t.Fatalf("read=%q %s", read.Data, read.ContentType)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses WHERE inquiry_id = $1 AND report_id IS NULL AND request_id IS NULL AND position = 1 AND admin_id = 9 AND accessed_at = $2", i.ID, f.now); n != 1 {
		t.Fatalf("讀取紀錄=%d", n)
	}
	// 讀取紀錄寫不進去：不讀、不回傳。
	_, err = f.db.Conn(ctx).Exec(ctx, "ALTER TABLE seller.evidence_accesses ADD CONSTRAINT test_no_writes CHECK (false) NOT VALID")
	must(t, err)
	calls := f.readSecureCalls()
	read, err = f.module.ReadEvidence(ctx, ReadEvidenceInput{AdminID: 9, Kind: EvidenceInquiry, RefID: i.ID, Position: 0})
	if err == nil || len(read.Data) != 0 || f.readSecureCalls() != calls {
		t.Fatalf("紀錄寫不進去就不能讀：err=%v data=%q", err, read.Data)
	}
}

// 案件清單的 awaiting_seller 篩選、「待決定」的判定與開始時間：SQL 的結果等於 Go 的判定式（同一個定義兩處實作，資料集測試鎖住一致）。
func TestCaseListAwaitingSellerAgreesWithTheGoPredicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var waiting, idle, replied, withdrawn, closed []uuid.UUID
	for range 3 {
		s := f.anotherStore(t)
		w := f.foodCase(t, s.StoreID)
		f.mustInquire(t, w.ID)
		waiting = append(waiting, w.ID)
		d, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
		idle = append(idle, d.ID)
		r, _ := f.newCase(t, s.StoreID, policy.Counterfeit, SeverityLow, 1)
		ri := f.mustInquire(t, r.ID).Inquiry
		f.now = f.now.Add(2 * time.Hour)
		_, err := f.reply(t, ri.ID, f.now.Add(-time.Hour))
		must(t, err)
		replied = append(replied, r.ID)
		x, _ := f.newCase(t, s.StoreID, policy.Privacy, SeverityLow, 1)
		xi := f.mustInquire(t, x.ID).Inquiry
		f.now = f.now.Add(time.Minute)
		_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: xi.ID, AdminID: inquiryWithdraw})
		must(t, err)
		withdrawn = append(withdrawn, x.ID)
		z, _ := f.newCase(t, s.StoreID, policy.Impersonation, SeverityLow, 1)
		f.mustInquire(t, z.ID)
		_, err = f.decide(t, z.ID, DecisionNoAction, "")
		must(t, err)
		closed = append(closed, z.ID)
	}
	list := func(in CaseListInput) []AdminCase {
		var out []AdminCase
		var after *CaseCursor
		for {
			in.PageSize, in.After = 4, after
			page, err := f.module.AdminCases(ctx, in)
			must(t, err)
			out = append(out, page.Items...)
			if page.Next == nil {
				return out
			}
			after = page.Next
		}
	}
	idsOf := func(cases []AdminCase) []uuid.UUID {
		var out []uuid.UUID
		for _, c := range cases {
			out = append(out, c.ID)
		}
		slices.SortFunc(out, uuid.UUID.Compare)
		return out
	}
	sorted := func(lists ...[]uuid.UUID) []uuid.UUID {
		out := slices.Concat(lists...)
		slices.SortFunc(out, uuid.UUID.Compare)
		return out
	}
	all := list(CaseListInput{})
	var wantWaiting, wantDecision []AdminCase
	for _, c := range all {
		if c.AwaitingSeller() {
			wantWaiting = append(wantWaiting, c)
		}
		if c.Open() && !c.AwaitingSeller() {
			wantDecision = append(wantDecision, c)
		}
	}
	got := list(CaseListInput{AwaitingSeller: true})
	if !slices.Equal(idsOf(got), sorted(waiting)) || !slices.Equal(idsOf(got), idsOf(wantWaiting)) {
		t.Fatalf("awaiting_seller=%v，預期 %v", idsOf(got), sorted(waiting))
	}
	open := CaseStateOpen
	openCases := list(CaseListInput{State: &open})
	var notWaiting []AdminCase
	waitingSet := map[uuid.UUID]bool{}
	for _, id := range waiting {
		waitingSet[id] = true
	}
	for _, c := range openCases {
		if !waitingSet[c.ID] {
			notWaiting = append(notWaiting, c)
		}
	}
	if !slices.Equal(idsOf(notWaiting), idsOf(wantDecision)) || !slices.Equal(idsOf(wantDecision), sorted(idle, replied, withdrawn)) {
		t.Fatalf("進行中減去等賣家應等於待決定：%v vs %v", idsOf(notWaiting), idsOf(wantDecision))
	}
	if got := list(CaseListInput{AwaitingSeller: true, State: &open}); len(got) != len(waiting) {
		t.Fatalf("進行中加等賣家=%d", len(got))
	}
	closedState := CaseStateClosed
	_, err := f.module.AdminCases(ctx, CaseListInput{AwaitingSeller: true, State: &closedState})
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"AwaitingSeller"}) {
		t.Fatalf("等賣家與已結案：%v", err)
	}
	// 待決定的開始時間（seller.review_case_decision，清單與單筆）等於由 fixture 的事實（開案、詢問回覆收到、管理員撤回）算出的 max；
	// 等賣家的案件沒有期限。
	for _, c := range all {
		if !(c.Open() && !c.AwaitingSeller()) {
			if c.DecideDeadline != nil {
				t.Errorf("案件 %s 不是待決定，不該有決定期限：%+v", c.ID, c.DecideDeadline)
			}
			continue
		}
		want := c.OpenedAt
		for _, i := range f.inquiriesOf(t, c.ID) {
			if r := inquiryClockRestart(i); r != nil && r.After(want) {
				want = *r
			}
		}
		if got := decisionStartOf(t, c); !got.Equal(want) {
			t.Errorf("案件 %s：待決定的開始=%v，由詢問算出 %v", c.ID, got, want)
		}
		if one := f.caseOf(t, c.ID); !decisionStartOf(t, one).Equal(want) {
			t.Errorf("案件 %s 單筆：%v，預期 %v", c.ID, decisionStartOf(t, one), want)
		}
	}
	// 已回覆的案件起算點是收到的時間（比開案晚的話）；已撤回（管理員）的是撤回時間；系統撤回（結案）的案件已不是待決定，沒有期限。
	for _, id := range closed {
		if f.caseOf(t, id).DecideDeadline != nil {
			t.Errorf("已結案不該有決定期限：%s", id)
		}
	}
	for _, id := range withdrawn {
		if c := f.caseOf(t, id); !decisionStartOf(t, c).After(c.OpenedAt) {
			t.Errorf("管理員撤回要重新起算：%s", id)
		}
	}
	f.assertCaseInvariants(t)
}

// 詢問清單舊到新、分頁不漏不重；案件不存在回 NotFound。
func TestListInquiriesIsOldestFirstAndPaged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	var want []uuid.UUID
	for n := range 5 {
		f.now = f.now.Add(time.Minute)
		r := f.mustInquire(t, c.ID)
		want = append(want, r.Inquiry.ID)
		f.now = f.now.Add(time.Minute)
		_, err := f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: r.Inquiry.ID, AdminID: inquiryWithdraw})
		must(t, err)
		_ = n
	}
	got := f.inquiriesOf(t, c.ID) // 每頁 2 筆
	var ids []uuid.UUID
	for _, i := range got {
		ids = append(ids, i.ID)
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("ids=%v want=%v", ids, want)
	}
	_, err := f.module.CaseInquiries(ctx, CaseInquiryListInput{CaseID: uuid.NewV7()})
	wantKind(t, err, apperr.KindNotFound)
}

// 詢問資料表的約束：每個案例只違反一條，而且指名是哪一條（wantViolates）。
func TestCaseInquirySchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, other := f.open(t)
	c := f.foodCase(t, s.StoreID)
	busy := f.foodCase(t, other.StoreID)
	f.mustInquire(t, busy.ID)
	repliedCase := f.foodCase(t, f.anotherStore(t).StoreID)
	repliedInquiry := f.mustInquire(t, repliedCase.ID).Inquiry
	_, err := f.reply(t, repliedInquiry.ID, f.now, "e0")
	must(t, err)

	// 一筆有效的詢問（案件 c，店家 s）；每個案例只換掉一部分。
	insert := func(caseID uuid.UUID, storeID int64, admin int, due string, extraColumns, extraValues string) string {
		return fmt.Sprintf("INSERT INTO seller.case_inquiries (id, case_id, store_id, kind, message, created_at, created_by_admin_id, due_at%s) VALUES (uuidv7(), '%s', %d, 'RECORDS', 'x', now(), %d, %s%s)",
			extraColumns, caseID, storeID, admin, due, extraValues)
	}
	valid := func(extraColumns, extraValues string) string {
		return insert(c.ID, s.StoreID, 1, "now() + interval '1 day'", extraColumns, extraValues)
	}
	replyColumns := ", reply_body, reply_channel, reply_received_at, reply_recorded_at, reply_recorded_by_admin_id"
	withdrawColumns := ", withdrawn_at, withdrawn_by_admin_id, withdraw_cause"
	for _, v := range []struct{ name, constraint, sql string }{
		{"店家不是案件的店家", "case_inquiries_case_fkey", insert(c.ID, other.StoreID, 1, "now() + interval '1 day'", "", "")},
		{"種類不認得", "case_inquiries_kind_check", strings.Replace(valid("", ""), "'RECORDS'", "'PHONE'", 1)},
		{"說明是空字串", "case_inquiries_message_check", strings.Replace(valid("", ""), "'x'", "''", 1)},
		{"說明超過 2000 字", "case_inquiries_message_check", strings.Replace(valid("", ""), "'x'", "repeat('字', 2001)", 1)},
		{"期限不晚於建立", "case_inquiries_due_check", insert(c.ID, s.StoreID, 1, "now()", "", "")},
		{"管理員是 0", "case_inquiries_admin_check", insert(c.ID, s.StoreID, 0, "now() + interval '1 day'", "", "")},
		{"只有 request_id", "case_inquiries_request_check", valid(", request_id", ", uuidv7()")},
		{"只有 request_hash", "case_inquiries_request_check", valid(", request_hash", ", '\\x01'")},
		{"回覆只有內容", "case_inquiries_reply_check", valid(", reply_body", ", 'x'")},
		{"回覆管道不認得", "case_inquiries_reply_check", valid(replyColumns, ", 'x', 'SMS', now(), now(), 1")},
		{"回覆內容超過 4000 字", "case_inquiries_reply_check", valid(replyColumns, ", repeat('字', 4001), 'PHONE', now(), now(), 1")},
		{"收到早於建立", "case_inquiries_reply_check", valid(replyColumns, ", 'x', 'PHONE', now() - interval '1 day', now(), 1")},
		{"記下早於收到", "case_inquiries_reply_check", valid(replyColumns, ", 'x', 'PHONE', now() + interval '1 hour', now(), 1")},
		{"記下的管理員是 0", "case_inquiries_reply_check", valid(replyColumns, ", 'x', 'PHONE', now(), now(), 0")},
		{"撤回只有時間", "case_inquiries_withdraw_check", valid(", withdrawn_at", ", now()")},
		{"撤回原因不認得", "case_inquiries_withdraw_check", valid(withdrawColumns, ", now(), 1, 'OTHER'")},
		{"撤回的管理員是 0", "case_inquiries_withdraw_check", valid(withdrawColumns, ", now(), 0, 'ADMIN'")},
		{"撤回早於建立", "case_inquiries_withdraw_check", valid(withdrawColumns, ", now() - interval '1 day', 1, 'ADMIN'")},
		{"回覆與撤回同時存在", "case_inquiries_terminal_check", valid(replyColumns+withdrawColumns, ", 'x', 'PHONE', now(), now(), 1, now(), 1, 'ADMIN'")},
		{"同一個案件第二筆還沒結束的詢問", "case_inquiries_one_open", insert(busy.ID, other.StoreID, 1, "now() + interval '1 day'", "", "")},
		// 附件：位置、資產、外鍵與主鍵。
		{"附件位置超過 4", "case_inquiry_reply_evidence_position_check", fmt.Sprintf("INSERT INTO seller.case_inquiry_reply_evidence (inquiry_id, position, asset_id, object_key) VALUES ('%s', 5, 'a', 'k')", repliedInquiry.ID)},
		{"附件的資產是空字串", "case_inquiry_reply_evidence_asset_check", fmt.Sprintf("INSERT INTO seller.case_inquiry_reply_evidence (inquiry_id, position, asset_id, object_key) VALUES ('%s', 4, '', 'k')", repliedInquiry.ID)},
		{"附件掛在不存在的詢問", "case_inquiry_reply_evidence_inquiry_id_fkey", "INSERT INTO seller.case_inquiry_reply_evidence (inquiry_id, position, asset_id, object_key) VALUES (uuidv7(), 0, 'a', 'k')"},
		// 讀取紀錄：擁有者剛好一個，附件要存在。
		{"讀取紀錄沒有擁有者", "evidence_accesses_owner_check", "INSERT INTO seller.evidence_accesses (access_id, position, admin_id, accessed_at) VALUES (uuidv7(), 0, 1, now())"},
		{"讀取紀錄指向不存在的附件", "evidence_accesses_inquiry_fkey", fmt.Sprintf("INSERT INTO seller.evidence_accesses (access_id, inquiry_id, position, admin_id, accessed_at) VALUES (uuidv7(), '%s', 4, 1, now())", repliedInquiry.ID)},
	} {
		f.wantViolates(t, v.name, v.constraint, v.sql)
	}
	// 附件的主鍵：同一筆詢問同一個位置不能重複（已有位置 0）。
	_, err = f.db.Conn(ctx).Exec(ctx, fmt.Sprintf("INSERT INTO seller.case_inquiry_reply_evidence (inquiry_id, position, asset_id, object_key) VALUES ('%s', 0, 'a', 'k')", repliedInquiry.ID))
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.ConstraintName != "case_inquiry_reply_evidence_pkey" {
		t.Errorf("附件位置重複要被主鍵擋下：%v", err)
	}
	// 冪等鍵的範圍是 (管理員, request_id)：同一位管理員重複用同一個 request_id 被擋下，另一位管理員可以。
	// 第一筆佔用案件 c 的名額；重複的放在已撤回的列（不佔名額），才只違反冪等鍵。
	req := "'0199aaaa-0000-4000-8000-000000000002'"
	_, err = f.db.Conn(ctx).Exec(ctx, valid(", request_id, request_hash", ", "+req+", '\\x01'"))
	must(t, err)
	duplicate := func(admin int) string {
		return insert(c.ID, s.StoreID, admin, "now() + interval '1 day'", withdrawColumns+", request_id, request_hash", ", now(), 1, 'ADMIN', "+req+", '\\x01'")
	}
	f.wantViolates(t, "同一位管理員重複 request_id", "case_inquiries_request_key", duplicate(1))
	_, err = f.db.Conn(ctx).Exec(ctx, duplicate(2))
	must(t, err)
}

// decisionStartOf 是案件待決定的開始時間（決定期限的 Start，來源是 seller.review_case_decision）；案件沒有決定期限時記錯並回零值。
func decisionStartOf(t *testing.T, c AdminCase) time.Time {
	t.Helper()
	if c.DecideDeadline == nil {
		t.Errorf("案件 %s 沒有決定期限", c.ID)
		return time.Time{}
	}
	return c.DecideDeadline.Start
}
