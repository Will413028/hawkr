//go:build integration

package sellerside_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/quote"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/sellerside"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestAnotherStoreAndAnUninvitedStoreSeeNothingAndCannotWrite(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	if _, err := svc.Get(ctx, outsider, f.publication); err == nil {
		t.Fatal("沒被邀請的店讀得到詢價")
	} else {
		wantKind(t, err, apperr.KindNotFound)
	}
	_, err := svc.Get(ctx, invitedStore, uuid.NewV7())
	wantKind(t, err, apperr.KindNotFound)
	page, err := svc.List(ctx, outsider, 0, "")
	must(t, err)
	if len(page.Items) != 0 {
		t.Fatalf("沒被邀請的店列得到 %d 筆", len(page.Items))
	}
	_, err = svc.UpsertDraft(ctx, outsider, f.publication, "", f.validQuote())
	wantKind(t, err, apperr.KindNotFound)
	_, err = svc.Submit(ctx, f.submitIn(outsider, uuid.NewV7(), "", f.validQuote()))
	wantKind(t, err, apperr.KindNotFound)
	_, err = svc.Decline(ctx, outsider, f.publication, "")
	wantKind(t, err, apperr.KindNotFound)
	if n := f.count("procurement.seller_responses"); n != 0 {
		t.Fatalf("被擋下的寫入不該留下回價列：%d", n)
	}
}

func TestStoreCannotTouchAnotherStoresResponse(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	mine := f.submit(invitedStore, "", f.validQuote())
	svc := f.service()
	// 另一家被邀請的店拿著我的回價 ID 撤回：不是它的回價。
	if _, err := svc.Withdraw(ctx, otherInvitedStore, f.publication, mine.ResponseID, "1", ""); err == nil {
		t.Fatal("別家的回價被撤回")
	} else {
		wantKind(t, err, apperr.KindNotFound)
	}
	other := f.submit(otherInvitedStore, "", f.validQuote())
	_, err := svc.Withdraw(ctx, otherInvitedStore, f.publication, mine.ResponseID, "1", "")
	wantKind(t, err, apperr.KindNotFound)
	d, err := svc.Get(ctx, otherInvitedStore, f.publication)
	must(t, err)
	if d.Latest == nil || d.Latest.ResponseID != other.ResponseID || d.Latest.ResponseID == mine.ResponseID {
		t.Fatalf("Get 只能含自己的回價：%+v", d.Latest)
	}
}

func TestReadsCreateNoRows(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	if _, err := f.service().Get(ctx, invitedStore, f.publication); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service().List(ctx, invitedStore, 0, ""); err != nil {
		t.Fatal(err)
	}
	if n := f.count("procurement.seller_responses"); n != 0 {
		t.Fatalf("讀取建了 %d 列回價身分列", n)
	}
}

func TestGetReturnsTheSellerFacingContentAndSummary(t *testing.T) {
	f := newFixture(t)
	d, err := f.service().Get(t.Context(), invitedStore, f.publication)
	must(t, err)
	if d.Summary.PublicationID != f.publication || d.Summary.Version != 1 || d.Summary.SellerFacingHash != f.hash {
		t.Fatalf("摘要 = %+v", d.Summary)
	}
	p := d.Payload
	if p.Item != "小白菜" || p.BasisUnit == "" || p.LowerBoundAmount != 100 || p.UpperBoundAmount != 200 || d.Draft != nil || d.Latest != nil {
		t.Fatalf("內容 = %+v", d)
	}
}

func TestBuyerDeletedKeepsTheInviteReadableButClosesWrites(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	f.submit(invitedStore, "", f.validQuote())
	must(t, f.d.InTx(ctx, func(ctx context.Context) error { return retention.New(f.d).OnBuyerDeleted(ctx, f.buyer) }))
	if n := f.count("procurement.rfq_publications WHERE buyer_id IS NULL AND id = $1", f.publication); n != 1 {
		t.Fatal("買家刪除後發布列的 buyer_id 要是 NULL")
	}
	svc := f.service()
	d, err := svc.Get(ctx, invitedStore, f.publication)
	must(t, err)
	if d.Payload.Item != "小白菜" || d.Latest == nil || d.Summary.SellerFacingHash != f.hash {
		t.Fatalf("買家刪除後被邀請的店要讀得到完整內容：%+v", d)
	}
	page, err := svc.List(ctx, invitedStore, 0, "")
	must(t, err)
	if len(page.Items) != 1 {
		t.Fatalf("買家刪除後清單 %d 筆", len(page.Items))
	}
	_, err = svc.Get(ctx, outsider, f.publication)
	wantKind(t, err, apperr.KindNotFound)
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote())
	wantClosed(t, err)
	_, err = svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "1", f.validQuote()))
	wantClosed(t, err)
	_, err = svc.Withdraw(ctx, invitedStore, f.publication, d.Latest.ResponseID, "1", "")
	wantClosed(t, err)
	if _, err := svc.Decline(ctx, invitedStore, f.publication, "CLOSED"); err != nil {
		t.Fatalf("買家刪除後仍可婉拒：%v", err)
	}
}

func TestListPagesByPublishTimeNewestFirst(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	second := f.sibling(2, start.Add(time.Minute))
	third := f.sibling(3, start.Add(2*time.Minute))
	f.submit(invitedStore, "", f.validQuote())
	if _, err := f.service().UpsertDraft(ctx, invitedStore, second.publication, "", model.QuoteInput{SpecNote: "草稿"}); err != nil {
		t.Fatal(err)
	}
	svc := f.service()
	first, err := svc.List(ctx, invitedStore, 2, "")
	must(t, err)
	if len(first.Items) != 2 || first.Items[0].Summary.PublicationID != third.publication || first.Items[1].Summary.PublicationID != second.publication || first.NextPageToken == "" {
		t.Fatalf("第一頁 = %+v", first.Items)
	}
	if !first.Items[1].HasDraft || first.Items[1].LatestResponseStatus != nil || first.Items[0].HasDraft {
		t.Fatalf("草稿與回價狀態 = %+v", first.Items)
	}
	rest, err := svc.List(ctx, invitedStore, 2, first.NextPageToken)
	must(t, err)
	if len(rest.Items) != 1 || rest.Items[0].Summary.PublicationID != f.publication || rest.NextPageToken != "" {
		t.Fatalf("第二頁 = %+v", rest.Items)
	}
	if st := rest.Items[0].LatestResponseStatus; st == nil || *st != model.ResponseActive {
		t.Fatalf("已送出的詢價狀態 = %v", st)
	}
	_, err = svc.List(ctx, invitedStore, -1, "")
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = svc.List(ctx, invitedStore, 0, "not-a-token")
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestDraftCreateUpdateAndVersionConflicts(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	lead := int32(0)
	in := model.QuoteInput{SpecNote: "草稿", GoodsAmount: 10, LeadTimeDays: &lead}
	d1, err := svc.UpsertDraft(ctx, invitedStore, f.publication, "", in)
	must(t, err)
	if d1.Version != 1 {
		t.Fatalf("第一版 = %+v", d1)
	}
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", in)
	wantConflict(t, err, sellerside.TargetDraft)
	in.SpecNote = "草稿二"
	d2, err := svc.UpsertDraft(ctx, invitedStore, f.publication, "1", in)
	must(t, err)
	if d2.Version != 2 {
		t.Fatalf("第二版 = %+v", d2)
	}
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "1", in)
	wantConflict(t, err, sellerside.TargetDraft)
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "x", in)
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", model.QuoteInput{SpecNote: strings.Repeat("字", 1001)})
	wantKind(t, err, apperr.KindInvalidInput)
	got, err := svc.Get(ctx, invitedStore, f.publication)
	must(t, err)
	if got.Draft == nil || got.Draft.Version != 2 || got.Draft.Quote.SpecNote != "草稿二" || got.Draft.Quote.GoodsAmount != 10 || got.Draft.Quote.LeadTimeDays == nil || *got.Draft.Quote.LeadTimeDays != 0 {
		t.Fatalf("讀回的草稿 = %+v", got.Draft)
	}
	if got.Latest != nil {
		t.Fatal("草稿不是正式回價")
	}
}

// 草稿建立在已有草稿時由 ON CONFLICT 擋下（併發建立只有一個贏）：第二次插入沒有列回來，也不改原內容。
func TestInsertSellerDraftKeepsTheFirstPayloadOnConflict(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	in := model.QuoteInput{SpecNote: "第一份", GoodsAmount: 10}
	if _, err := f.service().UpsertDraft(ctx, invitedStore, f.publication, "", in); err != nil {
		t.Fatal(err)
	}
	var responseID uuid.UUID
	if err := f.d.Pool().QueryRow(ctx, `SELECT seller_response_id FROM procurement.seller_response_drafts`).Scan(&responseID); err != nil {
		t.Fatal(err)
	}
	q := store.New(f.d.Pool())
	_, err := q.InsertSellerDraft(ctx, store.InsertSellerDraftParams{SellerResponseID: responseID, Payload: []byte(`{"SpecNote":"第二份"}`), Now: start})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("已有草稿時要沒有列回來，得到 %v", err)
	}
	got, err := q.GetSellerDraft(ctx, responseID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || strings.Contains(string(got.Payload), "第二份") || !strings.Contains(string(got.Payload), "第一份") {
		t.Fatalf("草稿被覆蓋：version=%d payload=%s", got.Version, got.Payload)
	}
}

func TestSubmitCreatesAnImmutableVersionWhoseHashMatchesTheBuyersList(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	lead := int32(0)
	promised := f.now.Add(24*time.Hour + 123456789*time.Nanosecond)
	q := f.validQuote()
	q.LeadTimeDays, q.PromisedDelivery, q.OfferedQuantity = &lead, &promised, "12.5"
	v := f.submit(invitedStore, "", q)
	if v.Version != 1 || v.Replayed || v.Status != model.ResponseActive || v.SupersededByVersion != 0 || v.Quote.Currency != "TWD" || v.Quote.OfferedQuantity != "12.500" ||
		v.Quote.LeadTimeDays == nil || *v.Quote.LeadTimeDays != 0 || !v.SubmittedAt.Equal(start) {
		t.Fatalf("送出結果 = %+v", v)
	}
	page, err := f.buyerQuotes().List(ctx, quote.ListInput{BuyerID: f.buyer, RequestID: f.request, PublicationID: f.publication})
	must(t, err)
	if len(page.Responses) != 1 || page.Responses[0].PayloadHash != v.PayloadHash {
		t.Fatalf("買家端列出的 payload_hash %+v 要等於賣家送出時回的 %s", page.Responses, v.PayloadHash)
	}
	d, err := f.service().Get(ctx, invitedStore, f.publication)
	must(t, err)
	if d.Latest == nil || d.Latest.PayloadHash != v.PayloadHash || d.Latest.Version != 1 {
		t.Fatalf("Get 的 latest = %+v", d.Latest)
	}
	if !v.OutsideRange {
		t.Error("貨款 1000、數量 12.5 KG，區間 1250 到 2500，要標示在區間外")
	}
}

func TestSubmitOutsideRangeIsFlaggedFromTheApprovalRange(t *testing.T) {
	f := newFixture(t)
	q := f.validQuote()
	q.OfferedQuantity = "3"
	q.GoodsAmount, q.TotalAmount = 100, 170
	v := f.submit(invitedStore, "", q)
	if !v.OutsideRange {
		t.Fatalf("貨款 100 低於 3 KG 的區間 300 到 600：%+v", v)
	}
	q.GoodsAmount, q.TotalAmount = 400, 470
	if v2 := f.submit(invitedStore, "1", q); v2.OutsideRange {
		t.Fatalf("貨款 400 在區間內：%+v", v2)
	}
}

func TestSubmitReplayReturnsTheFirstVersionBeforeAnyOtherCheck(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	cid := uuid.NewV7()
	in := f.submitIn(invitedStore, cid, "", f.validQuote())
	first, err := svc.Submit(ctx, in)
	must(t, err)
	again, err := svc.Submit(ctx, in)
	must(t, err)
	if !again.Replayed || again.Version != 1 || again.ResponseID != first.ResponseID || again.PayloadHash != first.PayloadHash {
		t.Fatalf("重送 = %+v", again)
	}
	if n := f.count("procurement.seller_response_versions"); n != 1 {
		t.Fatalf("重送不該多一個版本：%d", n)
	}
	// 重送先於版本檢查：expected 版本已不是目前的（空字串卻已有正式回價）也回放，不判成已提交。
	// 重送先於停止寫入：婉拒之後、有效期限過後、需求成交之後的重送都回放。
	if _, err := svc.Decline(ctx, invitedStore, f.publication, ""); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(30 * 24 * time.Hour)
	f.exec(`UPDATE procurement.rfq_publications SET buyer_id = NULL WHERE id = $1`, f.publication)
	replayed, err := svc.Submit(ctx, in)
	must(t, err)
	if !replayed.Replayed || replayed.Version != 1 || replayed.Status != model.ResponseExpired {
		t.Fatalf("婉拒與過期之後的重送 = %+v", replayed)
	}
}

func TestSubmitReplayWithDifferentExpectedFieldsIsRejected(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	cid := uuid.NewV7()
	in := f.submitIn(invitedStore, cid, "", f.validQuote())
	_, err := svc.Submit(ctx, in)
	must(t, err)
	for name, mutate := range map[string]func(*sellerside.SubmitInput){
		"expected_publication_version":        func(i *sellerside.SubmitInput) { i.ExpectedPublicationVersion = "2" },
		"expected_seller_facing_payload_hash": func(i *sellerside.SubmitInput) { i.ExpectedSellerFacingHash = "stale" },
		"expected_seller_response_version":    func(i *sellerside.SubmitInput) { i.ExpectedResponseVersion = "1" },
	} {
		changed := in
		mutate(&changed)
		_, err := svc.Submit(ctx, changed)
		wantKind(t, err, apperr.KindInvalidInput)
		if msg := err.Error(); !strings.Contains(msg, "client_request_id") {
			t.Errorf("%s：要指向 client_request_id：%v", name, err)
		}
	}
	if n := f.count("procurement.seller_response_versions"); n != 1 {
		t.Fatalf("不同的重送不該建版本：%d", n)
	}
}

// 草稿的大小只由 DB 的 CHECK（jsonb 文字的位元組數）量：Go 序列化未超過上限、jsonb 文字（鍵值之間多了空白）卻超過的草稿回 INVALID_ARGUMENT，不是 INTERNAL；
// 含大量 < > & 的草稿（Go 轉義成六位元組，jsonb 不轉義）在上限之內要被收下。
func TestDraftSizeIsMeasuredByTheDatabase(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	control := func(n int) string { return strings.Repeat("\x01", n) }
	var over model.QuoteInput
	for m := 1000; m >= 0; m-- {
		over = model.QuoteInput{SpecNote: control(m), TermsNote: control(1000), DeviationDisclosure: control(1000), ConversionBasis: control(500)}
		if b, err := json.Marshal(over); err != nil {
			t.Fatal(err)
		} else if len(b) <= 16384 {
			break
		}
	}
	var jsonbBytes int
	payload, _ := json.Marshal(over)
	if err := f.d.Pool().QueryRow(ctx, `SELECT octet_length($1::jsonb::text)`, string(payload)).Scan(&jsonbBytes); err != nil {
		t.Fatal(err)
	}
	if len(payload) > 16384 || jsonbBytes <= 16384 {
		t.Fatalf("測資要 Go 序列化在上限內（%d）、jsonb 文字超過（%d）", len(payload), jsonbBytes)
	}
	_, err := svc.UpsertDraft(ctx, invitedStore, f.publication, "", over)
	wantKind(t, err, apperr.KindInvalidInput)
	if !strings.Contains(err.Error(), "Draft") {
		t.Errorf("要指向 Draft：%v", err)
	}
	plain := model.QuoteInput{SpecNote: control(1000), TermsNote: control(1000), DeviationDisclosure: control(1000), ConversionBasis: control(500)}
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", plain)
	wantKind(t, err, apperr.KindInvalidInput)
	amp := model.QuoteInput{SpecNote: strings.Repeat("&", 1000), TermsNote: strings.Repeat("<", 1000), DeviationDisclosure: strings.Repeat(">", 1000), ConversionBasis: strings.Repeat("&", 500)}
	if b, _ := json.Marshal(amp); len(b) <= 16384 {
		t.Fatalf("測資要讓 Go 序列化超過 16384：%d", len(b))
	}
	if _, err := svc.UpsertDraft(ctx, invitedStore, f.publication, "", amp); err != nil {
		t.Fatalf("& 很多但 jsonb 在上限內的草稿要收下：%v", err)
	}
	// 草稿已存在時，帶空版本又太大的請求先是版本衝突：衝突先於大小。
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", over)
	wantConflict(t, err, sellerside.TargetDraft)
}

// 正式回價不受草稿大小限制：各欄位都在自己的上限內就收。
func TestSubmitIsNotCappedByTheDraftSize(t *testing.T) {
	f := newFixture(t)
	q := f.validQuote()
	q.SpecNote, q.TermsNote, q.DeviationDisclosure, q.ConversionBasis = strings.Repeat("&", 1000), strings.Repeat("<", 1000), strings.Repeat(">", 1000), strings.Repeat("&", 500)
	if v := f.submit(invitedStore, "", q); v.Version != 1 {
		t.Fatalf("送出 = %+v", v)
	}
}

func TestListPageTokensAreBoundToTheirList(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	if _, err := svc.List(ctx, invitedStore, 0, quote.EncodeToken(quote.TokenSellerResponses, f.now, uuid.NewV7())); err == nil {
		t.Fatal("買家回價列表的 token 不能給賣家的詢價列表用")
	} else {
		wantKind(t, err, apperr.KindInvalidInput)
	}
	f.sibling(buyerID+1, f.now.Add(time.Hour))
	first, err := svc.List(ctx, invitedStore, 1, "")
	must(t, err)
	if first.NextPageToken == "" {
		t.Fatal("要有下一頁")
	}
	_, err = f.buyerQuotes().List(ctx, quote.ListInput{BuyerID: f.buyer, RequestID: f.request, PublicationID: f.publication, PageToken: first.NextPageToken})
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestSubmitReplayWithDifferentContentIsRejected(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	cid := uuid.NewV7()
	_, err := f.service().Submit(ctx, f.submitIn(invitedStore, cid, "", f.validQuote()))
	must(t, err)
	changed := f.validQuote()
	changed.GoodsAmount, changed.TotalAmount = 1001, 1071
	_, err = f.service().Submit(ctx, f.submitIn(invitedStore, cid, "", changed))
	wantKind(t, err, apperr.KindInvalidInput)
	if n := f.count("procurement.seller_response_versions"); n != 1 {
		t.Fatalf("內容不同的重送不該建版本：%d", n)
	}
	// 規格說明是 hash 的一部分：只改它也算內容不同。
	changed = f.validQuote()
	changed.SpecNote = "另一份規格"
	_, err = f.service().Submit(ctx, f.submitIn(invitedStore, cid, "", changed))
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestSubmitVersionAndPublicationChecks(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	in := f.submitIn(invitedStore, uuid.NewV7(), "", f.validQuote())
	bad := in
	bad.ExpectedSellerFacingHash = "stale"
	_, err := svc.Submit(ctx, bad)
	wantConflict(t, err, quote.TargetPublication)
	bad = in
	bad.ExpectedPublicationVersion = "2"
	_, err = svc.Submit(ctx, bad)
	wantConflict(t, err, quote.TargetPublication)
	bad = in
	bad.ExpectedResponseVersion = "1" // 還沒有正式回價
	_, err = svc.Submit(ctx, bad)
	wantConflict(t, err, quote.TargetSellerResponse)
	if _, err := svc.Submit(ctx, in); err != nil {
		t.Fatal(err)
	}
	// 已有正式回價卻帶空字串、帶舊版本。
	_, err = svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "", f.validQuote()))
	wantConflict(t, err, quote.TargetSellerResponse)
	v2, err := svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "1", f.validQuote()))
	must(t, err)
	if v2.Version != 2 {
		t.Fatalf("第二版 = %+v", v2)
	}
	_, err = svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "1", f.validQuote()))
	wantConflict(t, err, quote.TargetSellerResponse)
	d, err := svc.Get(ctx, invitedStore, f.publication)
	must(t, err)
	if d.Latest == nil || d.Latest.Version != 2 {
		t.Fatalf("目前版本 = %+v", d.Latest)
	}
	// 每一次被擋下的送出都沒有留下版本。
	if n := f.count("procurement.seller_response_versions"); n != 2 {
		t.Fatalf("版本數 = %d", n)
	}
}

func TestSubmitValidatesTheQuote(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	cases := map[string]func(*model.QuoteInput){
		"金額恆等式不成立":    func(q *model.QuoteInput) { q.TotalAmount = 1 },
		"規格說明 1001 字": func(q *model.QuoteInput) { q.SpecNote = strings.Repeat("字", 1001) },
		"有效期限已過":      func(q *model.QuoteInput) { past := f.now.Add(-time.Second); q.ValidUntil = &past },
		"缺少單位":        func(q *model.QuoteInput) { q.OfferedQuantityUnit = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			q := f.validQuote()
			mutate(&q)
			_, err := f.service().Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "", q))
			wantKind(t, err, apperr.KindInvalidInput)
		})
	}
	if n := f.count("procurement.seller_response_versions"); n != 0 {
		t.Fatalf("驗證失敗的送出留下 %d 個版本", n)
	}
	longNote := f.validQuote()
	longNote.SpecNote = strings.Repeat("字", 1000)
	if _, err := f.service().Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "", longNote)); err != nil {
		t.Fatalf("1000 字的規格說明要通過：%v", err)
	}
}

func TestEveryWriteBlockConditionClosesDraftSubmitAndWithdraw(t *testing.T) {
	type blocker struct {
		name  string
		apply func(f *fixture)
	}
	blockers := []blocker{
		{"賣家已婉拒", func(f *fixture) {
			if _, err := f.service().Decline(context.Background(), invitedStore, f.publication, "NO"); err != nil {
				f.t.Fatal(err)
			}
		}},
		{"需求有進行中的成交", func(f *fixture) {
			// 另一家店的報價被買家接受。
			r := f.submit(otherInvitedStore, "", f.validQuote())
			if _, err := f.buyerQuotes().Accept(context.Background(), quote.AcceptInput{
				BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ResponseID: r.ResponseID, ExpectedVersion: 1, ExpectedSellerFacingHash: f.hash,
			}); err != nil {
				f.t.Fatal(err)
			}
		}},
		{"買家已刪除", func(f *fixture) {
			f.exec(`UPDATE procurement.rfq_publications SET buyer_id = NULL WHERE id = $1`, f.publication)
		}},
		{"回價期限已過", func(f *fixture) {
			// f.now 是台北 2026-11-23 11:00；期限 11-22 已過。
			f.exec(`UPDATE procurement.rfq_publication_approvals SET response_deadline = DATE '2026-11-22'`)
		}},
	}
	for _, b := range blockers {
		t.Run(b.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := t.Context()
			mine := f.submit(invitedStore, "", f.validQuote())
			b.apply(f)
			svc := f.service()
			// 期限改過核准之後賣家可見內容的 hash 會變，讀目前的值再送。
			d, err := svc.Get(ctx, invitedStore, f.publication)
			must(t, err)
			f.hash = d.Summary.SellerFacingHash
			_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote())
			wantClosed(t, err)
			_, err = svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "1", f.validQuote()))
			wantClosed(t, err)
			_, err = svc.Withdraw(ctx, invitedStore, f.publication, mine.ResponseID, "1", "")
			wantClosed(t, err)
			if n := f.count("procurement.seller_response_versions WHERE seller_response_id = $1", mine.ResponseID); n != 1 {
				t.Fatalf("被擋下的寫入留下版本：%d", n)
			}
			if n := f.count("procurement.seller_response_drafts"); n != 0 {
				t.Fatalf("被擋下的寫入留下草稿：%d", n)
			}
			if got, err := svc.Get(ctx, invitedStore, f.publication); err != nil || got.Latest == nil {
				t.Fatalf("停止寫入後讀取照常：%v", err)
			}
		})
	}
}

func TestADeadlineOfTodayStillAllowsWritesAndAFailedOrderReopensThem(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	f.exec(`UPDATE procurement.rfq_publication_approvals SET response_deadline = DATE '2026-11-23'`) // 台北的今天：不算過
	d, err := f.service().Get(ctx, invitedStore, f.publication)
	must(t, err)
	f.hash = d.Summary.SellerFacingHash
	mine := f.submit(otherInvitedStore, "", f.validQuote())
	a, err := f.buyerQuotes().Accept(ctx, quote.AcceptInput{BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ResponseID: mine.ResponseID, ExpectedVersion: 1, ExpectedSellerFacingHash: f.hash})
	must(t, err)
	_, err = f.service().UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote())
	wantClosed(t, err)
	// 建單重試用盡：接受單轉 ORDER_FAILED、live 鎖解除，賣家又能寫入。
	f.exec(`UPDATE procurement.buyer_quote_acceptances SET state = 'ORDER_FAILED', order_provisioning_state = 'ORDER_FAILED', order_failure_reason = 'RETRY_EXHAUSTED' WHERE id = $1`, a.ID)
	if _, err := f.service().UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote()); err != nil {
		t.Fatalf("建單失敗後賣家要能再寫入：%v", err)
	}
}

func TestWithdrawIsATargetStateMethod(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	_, err := svc.Withdraw(ctx, invitedStore, f.publication, uuid.NewV7(), "1", "")
	wantKind(t, err, apperr.KindNotFound) // 還沒有回價身分列
	r := f.submit(invitedStore, "", f.validQuote())
	_, err = svc.Withdraw(ctx, invitedStore, f.publication, r.ResponseID, "", "")
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = svc.Withdraw(ctx, invitedStore, f.publication, r.ResponseID, "1", strings.Repeat("a", 65))
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = svc.Withdraw(ctx, invitedStore, f.publication, r.ResponseID, "2", "")
	wantConflict(t, err, quote.TargetSellerResponse)
	c, err := svc.Withdraw(ctx, invitedStore, f.publication, r.ResponseID, "1", "  PRICE_CHANGED ")
	must(t, err)
	if c.WithdrawnVersion != 1 || c.LatestVersion != 1 || c.WriteClosedAt != nil || c.ResponseID != r.ResponseID {
		t.Fatalf("結案狀態 = %+v", c)
	}
	var code string
	var at time.Time
	must(t, f.d.Pool().QueryRow(ctx, `SELECT withdrawn_reason_code, withdrawn_at FROM procurement.seller_responses WHERE id = $1`, r.ResponseID).Scan(&code, &at))
	if code != "PRICE_CHANGED" {
		t.Fatalf("原因代碼 = %q", code)
	}
	f.now = f.now.Add(time.Hour)
	again, err := svc.Withdraw(ctx, invitedStore, f.publication, r.ResponseID, "1", "OTHER")
	must(t, err)
	var code2 string
	var at2 time.Time
	must(t, f.d.Pool().QueryRow(ctx, `SELECT withdrawn_reason_code, withdrawn_at FROM procurement.seller_responses WHERE id = $1`, r.ResponseID).Scan(&code2, &at2))
	if !reflect.DeepEqual(again, c) || code2 != code || !at2.Equal(at) {
		t.Fatalf("重複撤回要原樣回第一次的結果：%+v %q %v", again, code2, at2)
	}
	d, err := svc.Get(ctx, invitedStore, f.publication)
	must(t, err)
	if d.Latest == nil || d.Latest.Status != model.ResponseWithdrawn {
		t.Fatalf("撤回後的狀態 = %+v", d.Latest)
	}
	// 撤回之後可以用目前的版本（已撤回）當 expected 送出新版本。
	v2 := f.submit(invitedStore, "1", f.validQuote())
	if v2.Version != 2 || v2.Status != model.ResponseActive {
		t.Fatalf("撤回後重新報價 = %+v", v2)
	}
}

func TestDeclineIsIdempotentAndKeepsTheFirstClosure(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	r := f.submit(invitedStore, "", f.validQuote())
	c, err := svc.Decline(ctx, invitedStore, f.publication, " BUSY ")
	must(t, err)
	if c.WriteClosedAt == nil || c.WriteClosedReason != "DECLINED" || c.ResponseID != r.ResponseID || c.LatestVersion != 1 {
		t.Fatalf("結案狀態 = %+v", c)
	}
	f.now = f.now.Add(time.Hour)
	again, err := svc.Decline(ctx, invitedStore, f.publication, "OTHER")
	must(t, err)
	if again.ResponseID != c.ResponseID || again.PublicationID != c.PublicationID || again.LatestVersion != c.LatestVersion ||
		again.WithdrawnVersion != c.WithdrawnVersion || again.WriteClosedReason != c.WriteClosedReason ||
		again.WriteClosedAt == nil || !again.WriteClosedAt.Equal(*c.WriteClosedAt) {
		t.Fatalf("重複婉拒要回第一次的結果：%+v 與 %+v", again, c)
	}
	var code string
	must(t, f.d.Pool().QueryRow(ctx, `SELECT write_closed_reason_code FROM procurement.seller_responses WHERE id = $1`, r.ResponseID).Scan(&code))
	if code != "BUSY" {
		t.Fatalf("要保留第一次的原因：%q", code)
	}
	_, err = svc.Decline(ctx, invitedStore, f.publication, strings.Repeat("a", 65))
	wantKind(t, err, apperr.KindInvalidInput)
	// 已送出的正式回價保留，買家不能再接受。
	page, err := f.buyerQuotes().List(ctx, quote.ListInput{BuyerID: f.buyer, RequestID: f.request, PublicationID: f.publication})
	must(t, err)
	if len(page.Responses) != 1 || page.Responses[0].Acceptability != model.AcceptabilityIneligible {
		t.Fatalf("婉拒後買家端 = %+v", page.Responses)
	}
	// 另一家店沒婉拒：不受影響；沒填原因存 NULL。
	d, err := svc.Decline(ctx, otherInvitedStore, f.publication, "")
	must(t, err)
	var n int
	must(t, f.d.Pool().QueryRow(ctx, `SELECT count(*) FROM procurement.seller_responses WHERE id = $1 AND write_closed_reason_code IS NULL`, d.ResponseID).Scan(&n))
	if n != 1 {
		t.Fatal("沒填原因要存 NULL")
	}
}

func TestSuspendedAndRetiredStoresAreRefused(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	svc := f.service()
	f.stores.set(true, false)
	_, err := svc.List(ctx, invitedStore, 0, "")
	wantKind(t, err, apperr.KindPermissionDenied)
	_, err = svc.Get(ctx, invitedStore, f.publication)
	wantKind(t, err, apperr.KindPermissionDenied)
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote())
	wantKind(t, err, apperr.KindPermissionDenied)
	_, err = svc.Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "", f.validQuote()))
	wantKind(t, err, apperr.KindPermissionDenied)
	_, err = svc.Decline(ctx, invitedStore, f.publication, "")
	wantKind(t, err, apperr.KindPermissionDenied)
	f.stores.set(false, true)
	_, err = svc.UpsertDraft(ctx, invitedStore, f.publication, "", f.validQuote())
	wantKind(t, err, apperr.KindNotFound)
	if n := f.count("procurement.seller_responses"); n != 0 {
		t.Fatalf("被拒絕的寫入留下 %d 列", n)
	}
}

func TestSubmitAndAcceptSerializeOnTheResponseRow(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	first := f.submit(invitedStore, "", f.validQuote())
	var wg sync.WaitGroup
	var submitErr, acceptErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, submitErr = f.service().Submit(ctx, f.submitIn(invitedStore, uuid.NewV7(), "1", f.validQuote()))
	}()
	go func() {
		defer wg.Done()
		_, acceptErr = f.buyerQuotes().Accept(ctx, quote.AcceptInput{
			BuyerID: f.buyer, RequestID: f.request, ClientRequestID: uuid.NewV7(), ResponseID: first.ResponseID, ExpectedVersion: 1, ExpectedSellerFacingHash: f.hash,
		})
	}()
	wg.Wait()
	versions := f.count("procurement.seller_response_versions")
	accepts := f.count("procurement.buyer_quote_acceptances")
	switch {
	case submitErr == nil && acceptErr != nil:
		wantConflict(t, acceptErr, quote.TargetSellerResponse)
		if versions != 2 || accepts != 0 {
			t.Fatalf("賣家先改價：版本 %d、接受單 %d", versions, accepts)
		}
	case submitErr != nil && acceptErr == nil:
		wantClosed(t, submitErr)
		if versions != 1 || accepts != 1 {
			t.Fatalf("買家先接受：版本 %d、接受單 %d", versions, accepts)
		}
	default:
		t.Fatalf("兩邊要恰好一個成功：送出 %v、接受 %v", submitErr, acceptErr)
	}
}

// 賣家寫入交易持有發布列的 FOR SHARE 時，買家刪除的 UPDATE 要等它提交：檢查與寫入之間不會插進買家刪除。
// 用 pg_stat_activity 的 Lock 等待判斷「被擋住」，不靠時間長短。
func TestBuyerDeletionWaitsForTheSellerWriteTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	seller, err := f.d.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = seller.Rollback(context.Background()) }()
	buyerID, err := store.New(seller).LockPublicationBuyer(ctx, f.publication)
	must(t, err)
	if buyerID == nil {
		t.Fatal("買家還在，buyer_id 不該是 NULL")
	}
	deleted := make(chan error, 1)
	go func() {
		deleted <- f.d.InTx(ctx, func(ctx context.Context) error { return retention.New(f.d).OnBuyerDeleted(ctx, f.buyer) })
	}()
	blocked := func() bool {
		return f.count("pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%rfq_publications%'") > 0
	}
	deadline := time.Now().Add(10 * time.Second)
	for !blocked() {
		select {
		case err := <-deleted:
			t.Fatalf("賣家寫入交易還沒提交，買家刪除就完成了：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("買家刪除沒有在發布列上等鎖")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n := f.count("procurement.rfq_publications WHERE buyer_id IS NULL AND id = $1", f.publication); n != 0 {
		t.Fatal("賣家寫入交易提交前，buyer_id 不該被改")
	}
	must(t, seller.Commit(ctx))
	if err := <-deleted; err != nil {
		t.Fatalf("買家刪除：%v", err)
	}
	if n := f.count("procurement.rfq_publications WHERE buyer_id IS NULL AND id = $1", f.publication); n != 1 {
		t.Fatal("賣家提交後買家刪除要完成")
	}
}
