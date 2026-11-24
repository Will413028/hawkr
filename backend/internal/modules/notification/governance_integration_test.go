//go:build integration

package notification

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/fcm"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// fakeReporters 取代 moderation 的檢舉人讀取：與 moderation.CaseReporters 同一個形狀（一邊、依檢舉人號碼由小到大、從游標之後讀）；afters 記下每次讀的游標。
type fakeReporters struct {
	items  []moderation.Reporter
	afters []int64
}

func (f *fakeReporters) CaseReporters(_ context.Context, _ uuid.UUID, side moderation.ReporterSide, after int64, size int32) (moderation.ReporterPage, error) {
	f.afters = append(f.afters, after)
	var ids []int64
	byID := map[int64]moderation.Reporter{}
	for _, r := range f.items {
		id := r.BuyerID
		if side == moderation.ReporterSideStore {
			id = r.StoreID
		}
		if id > after {
			ids = append(ids, id)
			byID[id] = r
		}
	}
	slices.Sort(ids)
	var page moderation.ReporterPage
	for i, id := range ids {
		if i == int(size) {
			page.Next = ids[i-1]
			break
		}
		page.Items = append(page.Items, byID[id])
	}
	return page, nil
}

// setupGovernance 組真的 buyer 與 seller、正式註冊表與正式的治理訂閱者：事件走 events.Publish，渠道 job 走真的 deliver（email 寄到 fake）。
func setupGovernance(t *testing.T, reporters CaseReporters) *fixture {
	t.Helper()
	t.Parallel()
	if reporters == nil {
		reporters = &fakeReporters{}
	}
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	clock := func() time.Time { return governanceAt }
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: clock, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: clock, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	q := nopReportingJobs{}
	sl := seller.New(seller.Deps{
		KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: dirNoOrders{}, ProductStores: dirNoProducts{}, Registry: dirNoRegistry{}, Now: clock, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, clock), Events: bus, Jobs: q, Now: clock, Logger: log}),
	})
	resolver := NewResolver(buyers, sl, reporters)
	rec := &recorder{}
	push, mail, contacts := fcm.NewFake(nil), resend.NewFake(nil), &fakeContacts{m: map[int64]identity.ContactEmail{}}
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: sl, Resolver: resolver, Validity: noVouchers{}, Jobs: rec, Now: clock, NewID: uuid.NewV7, Logger: log,
		Contacts: contacts, Locales: id, Push: push, Mail: mail, ImageURL: testImageURL, SuppressionKeys: testSuppressionKeys,
	}, registry.Registry{})
	m.Subscribe(bus)
	return &fixture{db: d, identity: id, buyer: buyers, sellerMod: sl, bus: bus, jobs: rec, module: m, logs: logs, push: push, mail: mail, contacts: contacts, resolver: resolver}
}

func (f *fixture) governanceIntake(chunk int) *intake.Service {
	return intake.New(intake.Deps{
		DB: f.db, Accounts: f.identity, Buyers: f.buyer, Stores: f.sellerMod, Registry: registry.Registry{}, Resolver: f.resolver, Jobs: f.jobs,
		FanoutChunk: chunk, Now: func() time.Time { return governanceAt }, NewID: uuid.NewV7, Logger: slog.New(slog.DiscardHandler),
	})
}

// emit 在一個 transaction 內發布事件（來源模組的寫入 transaction 的替身），再把排著的意圖展開。
func emit[E any](t *testing.T, f *fixture, e E) {
	t.Helper()
	if err := f.db.InTx(context.Background(), func(ctx context.Context) error { return events.Publish(ctx, f.bus, e) }); err != nil {
		t.Fatal(err)
	}
	f.governanceRun(t)
}

func (f *fixture) governanceRun(t *testing.T) {
	t.Helper()
	f.runAll(t, f.governanceIntake(2))
}

func (f *fixture) inboxTypes(t *testing.T, account int64) []string {
	t.Helper()
	rows, err := f.db.Pool().Query(context.Background(), `SELECT type FROM notification.inbox_items WHERE account_id = $1 ORDER BY type`, account)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func (f *fixture) channels(t *testing.T, account int64) []string {
	t.Helper()
	rows, err := f.db.Pool().Query(context.Background(), `SELECT channel FROM notification.deliveries WHERE account_id = $1 ORDER BY channel`, account)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// storeOwner 開一間店並回傳 (店主帳號, store_id)。
func (f *fixture) storeOwner(t *testing.T) (account, storeID int64) {
	t.Helper()
	account, _ = f.newBuyer(t)
	return account, f.openStore(t, account)
}

var governanceAt = time.Date(2026, 11, 23, 4, 30, 0, 0, time.UTC)

// 店家面型別：每個來源事件在 worker 解析成店主帳號與店，寫進該店的收件匣，並依型別的渠道排推播與 email 投遞紀錄；停權措施不另發（由 StoreSuspended 通知）。
// mutation：拿掉 subscriptions.go 裡治理側的任何一列 → 對應列的收件匣是空的，這個測試紅。
func TestStoreSideEventsCreateOneInboxRowForTheStoreOwner(t *testing.T) {
	f := setupGovernance(t, nil)
	owner, storeID := f.storeOwner(t)
	measure, other := uuid.NewV7(), uuid.NewV7()
	for _, c := range []struct {
		name     string
		emit     func()
		wantType string
		channels []string
	}{
		{"停權", func() {
			emit(t, f, seller.StoreSuspended{StoreID: storeID, AccountID: owner, PeriodID: 1, EventID: uuid.NewV7(), OccurredAt: governanceAt})
		}, "store_suspended", []string{"EMAIL", "PUSH"}},
		{"解除停權", func() {
			emit(t, f, seller.StoreUnsuspended{StoreID: storeID, AccountID: owner, PeriodID: 1, EventID: uuid.NewV7(), OccurredAt: governanceAt})
		}, "store_unsuspended", []string{"EMAIL", "PUSH"}},
		{"內容下架", func() {
			emit(t, f, seller.MeasureCreated{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), Kind: seller.MeasureContentTakedown, Contents: make([]seller.ContentRef, 2), OccurredAt: governanceAt})
		}, "store_content_taken_down", []string{"EMAIL", "PUSH"}},
		{"警告", func() {
			emit(t, f, seller.MeasureCreated{StoreID: storeID, AccountID: owner, MeasureID: measure, Kind: seller.MeasureWarning, OccurredAt: governanceAt})
		}, "store_measure_issued", []string{"EMAIL", "PUSH"}},
		{"措施更新", func() {
			emit(t, f, seller.MeasureRevised{StoreID: storeID, AccountID: owner, MeasureID: measure, Revision: 2, OccurredAt: governanceAt})
		}, "store_measure_revised", []string{"PUSH"}},
		{"措施結束", func() {
			emit(t, f, seller.MeasureEnded{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), OccurredAt: governanceAt})
		}, "store_measure_ended", []string{"EMAIL", "PUSH"}},
		{"措施撤銷", func() {
			emit(t, f, seller.MeasureRevoked{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), OccurredAt: governanceAt})
		}, "store_measure_revoked", []string{"EMAIL", "PUSH"}},
		{"請求被駁回", func() {
			emit(t, f, seller.MeasureRequestDecided{StoreID: storeID, AccountID: owner, MeasureID: other, RequestID: uuid.NewV7(), Kind: seller.RequestKindAppeal, Outcome: seller.RequestRejected, OccurredAt: governanceAt})
		}, "store_measure_request_decided", []string{"EMAIL", "PUSH"}},
		{"KYC 核准", func() {
			emit(t, f, seller.KYCApproved{StoreID: storeID, AccountID: owner, ApplicationID: uuid.NewV7(), OccurredAt: governanceAt})
		}, "kyc_approved", []string{"EMAIL", "PUSH"}},
		{"KYC 退回", func() {
			emit(t, f, seller.KYCRejected{StoreID: storeID, AccountID: owner, ApplicationID: uuid.NewV7(), Reason: "自由文字理由", OccurredAt: governanceAt})
		}, "kyc_rejected", []string{"EMAIL", "PUSH"}},
		{"案件詢問", func() {
			emit(t, f, seller.CaseInquiryCreated{StoreID: storeID, AccountID: owner, InquiryID: uuid.NewV7(), CaseID: uuid.NewV7(), Kind: seller.InquiryKindRecords, DueAt: governanceAt.Add(72 * time.Hour), OccurredAt: governanceAt})
		}, "store_case_inquiry", []string{"EMAIL", "PUSH"}},
	} {
		before := f.inboxTypes(t, owner)
		beforeChannels := len(f.channels(t, owner))
		c.emit()
		got := f.inboxTypes(t, owner)
		if len(got) != len(before)+1 || !slices.Contains(got, c.wantType) {
			t.Errorf("%s：收件匣 %v → %v，預期多一列 %s", c.name, before, got, c.wantType)
		}
		if n := len(f.channels(t, owner)) - beforeChannels; n != len(c.channels) {
			t.Errorf("%s：新增 %d 筆投遞紀錄，預期 %v", c.name, n, c.channels)
		}
	}
	var audience string
	var rowStore *int64
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT audience, store_id FROM notification.inbox_items WHERE account_id = $1 LIMIT 1`, owner).Scan(&audience, &rowStore); err != nil || audience != "STORE" || rowStore == nil || *rowStore != storeID {
		t.Fatalf("收件匣列的受眾與店 = %s %v（%v）", audience, rowStore, err)
	}

	suspension := f.inboxTypes(t, owner)
	emit(t, f, seller.MeasureCreated{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), Kind: seller.MeasureSuspension, OccurredAt: governanceAt})
	if got := f.inboxTypes(t, owner); !slices.Equal(got, suspension) {
		t.Errorf("停權措施由 StoreSuspended 通知，不另發：%v → %v", suspension, got)
	}
	closedOwner, closedStore := f.storeOwner(t)
	if _, err := f.sellerMod.CloseCurrent(context.Background(), closedOwner); err != nil {
		t.Fatal(err)
	}
	emit(t, f, seller.StoreSuspended{StoreID: closedStore, AccountID: closedOwner, PeriodID: 9, EventID: uuid.NewV7(), OccurredAt: governanceAt})
	if got := f.inboxTypes(t, closedOwner); len(got) != 0 {
		t.Errorf("已關閉的店不寫收件匣：%v", got)
	}
}

// 接受申覆或解除請求時 seller 同 transaction 也發 MeasureRevoked（APPEAL_ACCEPTED）、MeasureEnded（帶 RequestID）：同一次處置只有
// store_measure_request_decided 一則（D3），文案依裁定；管理員直接撤銷或結束照常通知。
// mutation：OnMeasureRevoked 或 OnMeasureEnded 不略過接受請求的處置 → 收件匣多一列，這個測試紅。
func TestAcceptedDecisionIsTheOnlyNoticeForTheDisposition(t *testing.T) {
	f := setupGovernance(t, nil)
	ctx := context.Background()
	owner, storeID := f.storeOwner(t)
	appealed, lifted := uuid.NewV7(), uuid.NewV7()
	appealReq, liftReq := uuid.NewV7(), uuid.NewV7()
	emit(t, f, seller.MeasureRevoked{StoreID: storeID, AccountID: owner, MeasureID: appealed, Reason: seller.RevokeAppealAccepted, OccurredAt: governanceAt})
	emit(t, f, seller.MeasureRequestDecided{StoreID: storeID, AccountID: owner, MeasureID: appealed, RequestID: appealReq, Kind: seller.RequestKindAppeal, Outcome: seller.RequestAccepted, OccurredAt: governanceAt})
	emit(t, f, seller.MeasureEnded{StoreID: storeID, AccountID: owner, MeasureID: lifted, RequestID: liftReq, OccurredAt: governanceAt})
	emit(t, f, seller.MeasureRequestDecided{StoreID: storeID, AccountID: owner, MeasureID: lifted, RequestID: liftReq, Kind: seller.RequestKindRemediation, Outcome: seller.RequestAccepted, OccurredAt: governanceAt})
	if got := f.inboxTypes(t, owner); !slices.Equal(got, []string{"store_measure_request_decided", "store_measure_request_decided"}) {
		t.Fatalf("兩次接受的處置各一則裁定：%v", got)
	}
	var outcomes []string
	rows, err := f.db.Pool().Query(ctx, `SELECT params->>'outcome' FROM notification.inbox_items WHERE account_id = $1 ORDER BY params->>'request_kind'`, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			t.Fatal(err)
		}
		outcomes = append(outcomes, o)
	}
	if !slices.Equal(outcomes, []string{"ACCEPTED", "ACCEPTED"}) {
		t.Fatalf("裁定要帶進參數：%v", outcomes)
	}
	emit(t, f, seller.MeasureRevoked{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), Reason: seller.RevokeAdminCorrection, OccurredAt: governanceAt})
	emit(t, f, seller.MeasureEnded{StoreID: storeID, AccountID: owner, MeasureID: uuid.NewV7(), OccurredAt: governanceAt})
	if got := f.inboxTypes(t, owner); len(got) != 4 || !slices.Contains(got, "store_measure_revoked") || !slices.Contains(got, "store_measure_ended") {
		t.Fatalf("管理員直接撤銷與結束各一則：%v", got)
	}
}

// 確認信（店家等待刪除、買家刪帳申請）：只有 email 投遞紀錄、沒有收件匣列；信箱位址在寄送時才向 identity 讀，不在意圖參數、收件匣列與任何 job 參數裡。
// mutation：把位址放進 StorePendingDeletionParams 或 BuyerDeletionRequestedParams → 位址出現在意圖參數，這個測試紅。
func TestConfirmationEmailsNeverPutTheAddressInAnyJobParams(t *testing.T) {
	f := setupGovernance(t, nil)
	ctx := context.Background()
	owner, storeID := f.storeOwner(t)
	buyerAccount, buyerID := f.newBuyer(t)
	const storeAddress, buyerAddr = "store-owner@example.com", "deleting-buyer@example.com"
	f.contacts.set(owner, storeAddress, identity.ContactEmailVerified)
	f.contacts.set(buyerAccount, buyerAddr, identity.ContactEmailVerified)

	emit(t, f, seller.StorePendingDeletion{StoreID: storeID, StoreCode: 7, AccountID: owner, EventID: uuid.NewV7(), OccurredAt: governanceAt})
	emit(t, f, buyer.BuyerDeletionRequested{BuyerID: buyerID, AccountID: buyerAccount, EventID: uuid.NewV7(), OccurredAt: governanceAt})

	for _, account := range []int64{owner, buyerAccount} {
		if got := f.inboxTypes(t, account); len(got) != 0 {
			t.Errorf("確認信不寫收件匣：%d → %v", account, got)
		}
		if got := f.channels(t, account); !slices.Equal(got, []string{"EMAIL"}) {
			t.Fatalf("帳號 %d 的投遞紀錄 %v，預期只有 EMAIL", account, got)
		}
		if err := f.deliver(f.deliveryID(t, account, "EMAIL"), false); err != nil {
			t.Fatal(err)
		}
	}
	sent := f.mail.Sent()
	if len(sent) != 2 {
		t.Fatalf("寄出 %d 封，預期 2", len(sent))
	}
	for _, s := range sent {
		if (s.Email.To != storeAddress && s.Email.To != buyerAddr) || s.Email.Subject == "" {
			t.Errorf("寄出的信 %+v", s)
		}
	}
	var params []string
	rows, err := f.db.Pool().Query(ctx, `SELECT params::text FROM notification.intents`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		params = append(params, p)
	}
	deliveries, _ := f.jobs.deliverJobs()
	for _, d := range deliveries {
		raw, _ := json.Marshal(d)
		params = append(params, string(raw))
	}
	if len(params) < 4 {
		t.Fatalf("要檢查的參數只有 %d 筆", len(params))
	}
	for _, p := range params {
		if strings.Contains(p, "@") || strings.Contains(p, "example.com") {
			t.Errorf("job 或意圖參數含位址：%s", p)
		}
	}
}

// 檢舉確認（seller 來源的買家檢舉人、moderation 來源的店家檢舉人）：收件匣、推播之外寄確認信；位址在寄送時向 identity 讀，不在意圖參數與 job 參數裡。
// mutation：OnSellerReportReceived 把位址放進意圖參數 → 位址出現在意圖參數，這個測試紅。
func TestReportReceivedConfirmationEmailsReadTheAddressAtDelivery(t *testing.T) {
	f := setupGovernance(t, nil)
	ctx := context.Background()
	storeAccount, storeID := f.storeOwner(t)
	buyerAccount, buyerID := f.newBuyer(t)
	const storeAddress, buyerAddr = "reporting-store@example.com", "reporting-buyer@example.com"
	f.contacts.set(storeAccount, storeAddress, identity.ContactEmailVerified)
	f.contacts.set(buyerAccount, buyerAddr, identity.ContactEmailVerified)

	emit(t, f, seller.ReportReceived{ReportID: uuid.NewV7(), StoreID: storeID, ReporterBuyerID: buyerID, Category: policy.Spam, OccurredAt: governanceAt})
	emit(t, f, moderation.ReportReceived{EventID: uuid.NewV7(), OccurredAt: governanceAt, ReportID: uuid.NewV7(), CaseID: uuid.NewV7(), ReporterStoreID: storeID, Category: policy.Spam})

	for _, account := range []int64{storeAccount, buyerAccount} {
		if got := f.channels(t, account); !slices.Contains(got, "EMAIL") {
			t.Fatalf("帳號 %d 的投遞紀錄 %v，預期含 EMAIL", account, got)
		}
		if err := f.deliver(f.deliveryID(t, account, "EMAIL"), false); err != nil {
			t.Fatal(err)
		}
	}
	sent := f.mail.Sent()
	if len(sent) != 2 || sent[0].Email.To == sent[1].Email.To {
		t.Fatalf("寄出 %+v，預期兩位檢舉人各一封", sent)
	}
	for _, m := range sent {
		if (m.Email.To != storeAddress && m.Email.To != buyerAddr) || m.Email.Subject == "" {
			t.Errorf("寄出的信 %+v", m)
		}
	}
	var params []string
	rows, err := f.db.Pool().Query(ctx, `SELECT params::text FROM notification.intents`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		params = append(params, p)
	}
	deliveries, _ := f.jobs.deliverJobs()
	for _, d := range deliveries {
		raw, _ := json.Marshal(d)
		params = append(params, string(raw))
	}
	if len(params) < 2 {
		t.Fatalf("要檢查的參數只有 %d 筆", len(params))
	}
	for _, p := range params {
		if strings.Contains(p, "@") || strings.Contains(p, "example.com") {
			t.Errorf("job 或意圖參數含位址：%s", p)
		}
	}
}

// 作者通知只給買家作者：social、review、im 的隱藏與還原、social 的警告各寫一列；店家作者（social 的店家、im 的店家寄的訊息）不通知。
// mutation：messageAuthorBuyer 不檢查種類（店家作者也通知）→ 店家的 AuthorID 被當買家，這個測試紅。
func TestAuthorNoticesGoToBuyerAuthorsOnly(t *testing.T) {
	f := setupGovernance(t, nil)
	account, buyerID := f.newBuyer(t)
	strangerAccount, strangerBuyer := f.newBuyer(t)
	sanction := func() uuid.UUID { return uuid.NewV7() }
	emit(t, f, social.ContentHidden{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), Target: social.Target{Kind: social.TargetComment, ID: uuid.NewV7()}, Author: social.Actor{Kind: social.ActorBuyer, ID: buyerID}, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, social.ContentRestored{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), Target: social.Target{Kind: social.TargetComment, ID: uuid.NewV7()}, Author: social.Actor{Kind: social.ActorBuyer, ID: buyerID}, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, social.BuyerWarned{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), BuyerID: buyerID, Category: policy.Harassment, AppealPath: policy.AppealNone})
	emit(t, f, review.ReviewHidden{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), ReviewID: uuid.NewV7(), BuyerID: buyerID, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, review.ReviewRestored{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), ReviewID: uuid.NewV7(), BuyerID: buyerID, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, im.MessageHidden{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), ConversationID: uuid.NewV7(), MessageID: uuid.NewV7(), AuthorKind: im.PartyBuyer, AuthorID: buyerID, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, im.MessageRestored{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), ConversationID: uuid.NewV7(), MessageID: uuid.NewV7(), AuthorKind: im.PartyBuyer, AuthorID: buyerID, Category: policy.Spam, AppealPath: policy.AppealNone})
	if got, want := f.inboxTypes(t, account), []string{"author_warned", "content_hidden", "content_hidden", "content_hidden", "content_restored", "content_restored", "content_restored"}; !slices.Equal(got, want) {
		t.Fatalf("買家作者的收件匣 %v，預期 %v", got, want)
	}

	emit(t, f, social.ContentHidden{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), Target: social.Target{Kind: social.TargetComment, ID: uuid.NewV7()}, Author: social.Actor{Kind: social.ActorStore, ID: strangerBuyer}, Category: policy.Spam, AppealPath: policy.AppealNone})
	emit(t, f, im.MessageHidden{EventID: uuid.NewV7(), OccurredAt: governanceAt, SanctionID: sanction(), ConversationID: uuid.NewV7(), MessageID: uuid.NewV7(), AuthorKind: im.PartyStore, AuthorID: strangerBuyer, Category: policy.Spam, AppealPath: policy.AppealNone})
	if got := f.inboxTypes(t, strangerAccount); len(got) != 0 {
		t.Errorf("店家作者不通知：%v", got)
	}
}

// 檢舉人收到確認與結果：seller 的買家檢舉人、moderation 的買家與店家檢舉人各一則；檢舉結果只說結果類別。
func TestReporterNoticesCoverBuyersAndStores(t *testing.T) {
	f := setupGovernance(t, nil)
	ctx := context.Background()
	buyerAccount, buyerID := f.newBuyer(t)
	storeAccount, storeID := f.storeOwner(t)
	report := uuid.NewV7()

	emit(t, f, seller.ReportReceived{ReportID: report, StoreID: storeID, ReporterBuyerID: buyerID, Category: policy.Spam, OccurredAt: governanceAt})
	supplement := uuid.NewV7()
	emit(t, f, seller.ReportReceived{ReportID: report, SupplementID: &supplement, StoreID: storeID, ReporterBuyerID: buyerID, Category: policy.Spam, OccurredAt: governanceAt})
	emit(t, f, seller.ReportResolved{ReportID: report, StoreID: storeID, ReporterBuyerID: buyerID, Outcome: seller.OutcomeNoViolation, OccurredAt: governanceAt})
	if got := f.inboxTypes(t, buyerAccount); !slices.Equal(got, []string{"report_received", "report_received", "report_resolved"}) {
		t.Fatalf("買家檢舉人（seller 來源）%v", got)
	}

	modReport := uuid.NewV7()
	emit(t, f, moderation.ReportReceived{EventID: uuid.NewV7(), OccurredAt: governanceAt, ReportID: modReport, CaseID: uuid.NewV7(), ReporterStoreID: storeID, Category: policy.Spam})
	emit(t, f, moderation.ReportReceived{EventID: uuid.NewV7(), OccurredAt: governanceAt, ReportID: uuid.NewV7(), CaseID: uuid.NewV7(), ReporterBuyerID: buyerID, Category: policy.Spam})
	if got := f.inboxTypes(t, storeAccount); !slices.Equal(got, []string{"store_report_received"}) {
		t.Fatalf("店家檢舉人（moderation 來源）%v", got)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND audience = 'STORE' AND store_id = $2`, storeAccount, storeID); n != 1 {
		t.Fatalf("店家檢舉人的列要帶店：%d", n)
	}
	if got := f.inboxTypes(t, buyerAccount); len(got) != 4 {
		t.Fatalf("買家檢舉人累計 %v", got)
	}
	var params string
	if err := f.db.Pool().QueryRow(ctx, `SELECT params::text FROM notification.inbox_items WHERE account_id = $1 AND type = 'report_resolved'`, buyerAccount).Scan(&params); err != nil || !strings.Contains(params, `"outcome": "NO_VIOLATION"`) {
		t.Fatalf("結果類別 %s（%v）", params, err)
	}
}

// 結案的 fan-out：moderation.CaseClosedEvent 的母意圖分頁讀自己那一邊的檢舉人，買家與店家檢舉人各自收到 report_resolved；每頁 2 位、共多頁，
// 下一頁從游標接著讀，重跑整條鏈不重複。
// mutation：resolver.CaseReporters 不從游標讀（每頁從頭）→ 這個測試紅。
// mutation：店家的母意圖 moderation_case_closed_stores 不登記（ModerationCaseClosedParents 拿掉第二個）→ 店家檢舉人沒有列，這個測試紅。
func TestCaseClosedFansOutToBuyerAndStoreReportersAcrossPages(t *testing.T) {
	var items []moderation.Reporter
	reporters := &fakeReporters{}
	f := setupGovernance(t, reporters)
	type account struct{ account, id int64 }
	var buyers, stores []account
	for range 3 {
		a, b := f.newBuyer(t)
		buyers = append(buyers, account{a, b})
	}
	for range 2 {
		a, s := f.storeOwner(t)
		stores = append(stores, account{a, s})
	}
	order := []struct {
		isStore bool
		idx     int
	}{{false, 0}, {true, 0}, {false, 1}, {true, 1}, {false, 2}}
	for _, o := range order {
		r := moderation.Reporter{ReportID: uuid.NewV7()}
		if o.isStore {
			r.StoreID = stores[o.idx].id
		} else {
			r.BuyerID = buyers[o.idx].id
		}
		items = append(items, r)
	}
	reporters.items = items

	closed := moderation.CaseClosedEvent{EventID: uuid.NewV7(), OccurredAt: governanceAt, CaseID: uuid.NewV7(), TargetKind: moderation.TargetComment, Outcome: moderation.OutcomeViolation}
	if err := f.db.InTx(context.Background(), func(ctx context.Context) error { return events.Publish(ctx, f.bus, closed) }); err != nil {
		t.Fatal(err)
	}
	svc := f.governanceIntake(2)
	pages, _ := f.runAll(t, svc)
	if len(pages) < 4 {
		t.Fatalf("兩個母意圖各 3 頁以上才能涵蓋 5 位檢舉人（每頁 2 位）：%d 頁", len(pages))
	}
	for _, b := range buyers {
		if got := f.inboxTypes(t, b.account); !slices.Equal(got, []string{"report_resolved"}) {
			t.Errorf("買家檢舉人 %d：%v", b.account, got)
		}
	}
	for _, s := range stores {
		if got := f.inboxTypes(t, s.account); !slices.Equal(got, []string{"store_report_resolved"}) {
			t.Errorf("店家檢舉人 %d：%v", s.account, got)
		}
	}
	total := f.count(t, `SELECT count(*) FROM notification.inbox_items`)
	if total != 5 {
		t.Fatalf("共 %d 列，預期 5", total)
	}
	fromStart := 0
	for _, a := range reporters.afters {
		if a == 0 {
			fromStart++
		}
	}
	if fromStart != 2 || len(reporters.afters) < 4 {
		t.Fatalf("每個母意圖只有第一頁從頭讀，之後從上一頁最後一位的號碼接著讀：%v", reporters.afters)
	}
	if err := f.db.InTx(context.Background(), func(ctx context.Context) error { return events.Publish(ctx, f.bus, closed) }); err != nil {
		t.Fatal(err)
	}
	f.runAll(t, svc)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items`); n != 5 {
		t.Fatalf("事件重送後共 %d 列，不得重複", n)
	}
}

// 撥款：只有第一次啟用通知（重送、再啟用都不再寫）；補件期限的提醒一個檔位一則，同一個事件重跑不重複，期限改了是新的提醒，檔位不同是另一則。
// mutation：payout_account_deadline 的冪等鍵拿掉檔位 → 7 天與 1 天只剩一則，這個測試紅。
func TestPayoutNoticesFireOncePerActivationAndPerDeadlineTier(t *testing.T) {
	f := setupGovernance(t, nil)
	owner, storeID := f.storeOwner(t)
	changed := func(first bool) payment.PayoutAccountChanged {
		return payment.PayoutAccountChanged{OccurredAt: governanceAt, StoreID: storeID, Provider: "stripe", TransfersActive: true, FirstActivation: first}
	}
	emit(t, f, changed(false))
	if got := f.inboxTypes(t, owner); len(got) != 0 {
		t.Fatalf("不是第一次啟用不通知：%v", got)
	}
	emit(t, f, changed(true))
	emit(t, f, changed(true))
	if got := f.inboxTypes(t, owner); !slices.Equal(got, []string{"payout_account_activated"}) {
		t.Fatalf("第一次啟用只通知一次：%v", got)
	}

	deadline := governanceAt.Add(5 * 24 * time.Hour)
	approaching := func(at time.Time, tier payment.DeadlineTier) payment.PayoutDeadlineApproaching {
		return payment.PayoutDeadlineApproaching{OccurredAt: at, StoreID: storeID, Deadline: deadline, Tier: tier}
	}
	emit(t, f, approaching(governanceAt, payment.DeadlineSevenDays))
	emit(t, f, approaching(governanceAt.Add(24*time.Hour), payment.DeadlineSevenDays))
	emit(t, f, approaching(governanceAt.Add(4*24*time.Hour+time.Hour), payment.DeadlineOneDay))
	emit(t, f, approaching(governanceAt.Add(4*24*time.Hour+2*time.Hour), payment.DeadlineOneDay))
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'payout_account_deadline'`, owner); n != 2 {
		t.Fatalf("七天與一天各一則共 2 列，實際 %d", n)
	}
	moved := approaching(governanceAt, payment.DeadlineSevenDays)
	moved.Deadline = deadline.Add(48 * time.Hour)
	emit(t, f, moved)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'payout_account_deadline'`, owner); n != 3 {
		t.Fatalf("期限改了是新的提醒，共 3 列，實際 %d", n)
	}
}

// 代行完成：參數只有稽核列 ID、目標店與帳號、權限與動作（列舉）和時間；代行者名稱、標籤、資源名稱、事由與案件參照都不進參數。
// mutation：OnDelegationOperationCompleted 把 ActionLabel 放進參數 → 參數多一個欄位與自由文字，這個測試紅。
func TestDelegationNoticeParamsHaveNoNamesOrFreeText(t *testing.T) {
	f := setupGovernance(t, nil)
	owner, storeID := f.storeOwner(t)
	resourceID, resourceName, reason, caseRef := "post-1", "秘密商品名稱", "客訴處理事由", "CASE-77"
	emit(t, f, delegation.DelegationOperationCompleted{
		AuditLogID: uuid.NewV7(), SessionID: 5, SessionDisplayCode: "S-0005", ActorAdminID: 9, ActorDisplayName: "管理員小明", TargetStoreID: storeID, TargetSellerAccountID: owner,
		Capability: delegation.CapabilityProductsWrite, Action: delegation.AuditProductUpdate, ActionLabel: "更新商品（標籤）", RiskLevel: "LOW", ResourceType: "PRODUCT",
		ResourceID: &resourceID, ResourceName: &resourceName, OperationReason: &reason, CaseReference: &caseRef, CorrelationID: "corr-1", OccurredAt: governanceAt, NotifiedAt: governanceAt,
	})
	if got := f.inboxTypes(t, owner); !slices.Equal(got, []string{"delegation_operation_completed"}) {
		t.Fatalf("收件匣 %v", got)
	}
	var raw string
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT params::text FROM notification.inbox_items WHERE account_id = $1`, owner).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"action", "audit_log_id", "capability", "occurred_at", "target_seller_account_id", "target_store_id"}; !slices.Equal(keys, want) {
		t.Fatalf("參數欄位 %v，預期 %v", keys, want)
	}
	for _, secret := range []string{"管理員小明", "更新商品（標籤）", resourceName, reason, caseRef, "corr-1", "S-0005"} {
		if strings.Contains(raw, secret) {
			t.Errorf("參數含 %q：%s", secret, raw)
		}
	}
}

// 詢價發布：每個被邀請的店家一則 rfq_invited（邀請 ID 是冪等鍵），事件重送不重複。
func TestRfqPublishedInvitesEachStoreOnce(t *testing.T) {
	f := setupGovernance(t, nil)
	var invites []procurement.RfqInvite
	var accounts []int64
	for range 3 {
		a, s := f.storeOwner(t)
		accounts = append(accounts, a)
		invites = append(invites, procurement.RfqInvite{InviteID: uuid.NewV7(), StoreID: s})
	}
	e := procurement.RfqPublished{PublicationID: uuid.NewV7(), RequestID: uuid.NewV7(), ApprovalID: uuid.NewV7(), PublishedAt: governanceAt, Invites: invites}
	emit(t, f, e)
	emit(t, f, e)
	for _, a := range accounts {
		if got := f.inboxTypes(t, a); !slices.Equal(got, []string{"rfq_invited"}) {
			t.Errorf("店主 %d：%v", a, got)
		}
	}
}
