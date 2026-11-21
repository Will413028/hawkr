//go:build integration

package moderation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) takedownInput(c Case) DecideInput {
	return DecideInput{
		CaseID: c.ID, Version: c.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionViolation, Category: policy.Spam, Action: ActionTakedown,
		Text: "違反規範", Severity: string(seller.SeverityValues()[0]),
	}
}

func (f *fixture) revokeMeasure(t *testing.T, id uuid.UUID) {
	t.Helper()
	m := mustV[seller.Measure](t)(f.seller.CurrentMeasure(context.Background(), id))
	mustV[seller.Measure](t)(f.seller.RevokeMeasure(context.Background(), seller.RevokeMeasureInput{MeasureID: id, ExpectedVersion: m.Version, AdminID: 7, Reason: seller.RevokeAdminCorrection}))
}

func (f *fixture) storeParty(t *testing.T, storeID int64) im.Party {
	t.Helper()
	st := mustV[seller.Store](t)(chatStores{f.seller}.Store(context.Background(), storeID))
	return im.Party{Kind: im.PartyStore, Store: &st}
}

// 店家寫的聊天訊息被檢舉並下架：措施與案件綁定，列表與預覽對雙方都是墓碑，im 沒有隱藏紀錄；撤銷措施後訊息回來。
func TestStoreChatTakedownTombstonesAndRevokeRestores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	conv, ids := f.seedChat(t, b, storeID, 2, 0)
	buyerParty := im.Party{Kind: im.PartyBuyer, BuyerID: b}
	storeParty := f.storeParty(t, storeID)

	res := f.submit(t, b, Target{Kind: TargetChatMessage, ContentID: ids[2]}, policy.Harassment)
	d := mustV[CaseDetail](t)(f.module.DecideCase(ctx, f.takedownInput(res.Case)))
	if d.Case.MeasureID == nil || d.Case.StoreID == nil || *d.Case.StoreID != storeID {
		t.Fatalf("案件要綁定措施與店家：%+v", d.Case)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measure_contents WHERE measure_id = $1 AND content_kind = 'CHAT_MESSAGE' AND content_id = $2`, d.Case.MeasureID, ids[2]); n != 1 {
		t.Fatalf("措施的對象 = %d", n)
	}
	if n := f.count(t, `SELECT count(*) FROM im.hides WHERE message_id = $1`, ids[2]); n != 0 {
		t.Fatalf("下架不寫 im 的隱藏紀錄：%d", n)
	}
	for name, p := range map[string]im.Party{"買家": buyerParty, "店家": storeParty} {
		if hidden, text := f.hidden(t, p, conv, 2); !hidden || text != nil {
			t.Fatalf("%s的列表要回墓碑：hidden=%v text=%v", name, hidden, text)
		}
		convs := mustV[im.ConversationPage](t)(f.im.ListConversations(ctx, p, nil, 10))
		if len(convs.Items) != 1 || !convs.Items[0].Preview.Hidden || convs.Items[0].Preview.Text != nil {
			t.Fatalf("%s的預覽要是墓碑：%+v", name, convs.Items)
		}
	}
	if hidden, _ := f.hidden(t, buyerParty, conv, 1); hidden {
		t.Fatal("同對話的其他訊息不受影響")
	}

	f.revokeMeasure(t, *d.Case.MeasureID)
	if hidden, text := f.hidden(t, buyerParty, conv, 2); hidden || text == nil || *text != "m2" {
		t.Fatalf("撤銷後訊息回來：hidden=%v text=%v", hidden, text)
	}
	convs := mustV[im.ConversationPage](t)(f.im.ListConversations(ctx, buyerParty, nil, 10))
	if convs.Items[0].Preview.Hidden {
		t.Fatalf("撤銷後預覽回到內容：%+v", convs.Items[0].Preview)
	}
}

// 評價回覆被下架只遮回覆，評價照常；撤銷措施後回覆回來；回覆被下架後不能再被檢舉。
func TestReviewReplyTakedownHidesOnlyTheReply(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	author := f.buyerProfile(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, author, storeID, 3, "普通")
	st := mustV[seller.Store](t)(f.sellerRd.Store(ctx, storeID))
	mustV[review.Review](t)(f.review.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: "不實的回覆"}))

	res := f.submit(t, reporter, Target{Kind: TargetReviewReply, ContentID: r.ID}, policy.Harassment)
	snap := caseSnapshot(t, f, res.Case.ID)
	if snap == nil || snap.ReviewReply == nil || snap.ReviewReply.StoreID != storeID || snap.ReviewReply.Body != "不實的回覆" || snap.ReviewReply.ReviewID != r.ID.String() {
		t.Fatalf("回覆快照：%+v", snap)
	}
	d := mustV[CaseDetail](t)(f.module.DecideCase(ctx, f.takedownInput(res.Case)))
	if d.Case.MeasureID == nil {
		t.Fatal("要建立措施")
	}
	got := mustV[review.Review](t)(f.review.GetReview(ctx, r.ID, 0))
	if got.Reply != nil || got.Content != "普通" {
		t.Fatalf("只遮回覆：%+v", got)
	}
	if st := mustV[review.ProductStats](t)(f.review.ProductStats(ctx, r.ProductID)); st.Count != 1 {
		t.Fatalf("評價照常計入：%+v", st)
	}
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: Target{Kind: TargetReviewReply, ContentID: r.ID}, Category: policy.Harassment}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("已下架的回覆不能再檢舉：%v", err)
	}

	f.revokeMeasure(t, *d.Case.MeasureID)
	if got := mustV[review.Review](t)(f.review.GetReview(ctx, r.ID, 0)); got.Reply == nil || got.Reply.Content != "不實的回覆" {
		t.Fatalf("撤銷後回覆回來：%+v", got.Reply)
	}
}

// 沒有回覆的評價不能被檢舉回覆；買家與評價作者的 WARN、HIDE 不適用於回覆。
func TestReviewReplyReportRequiresAReplyAndRejectsBuyerActions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, f.buyerProfile(t), storeID, 3, "普通")
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: Target{Kind: TargetReviewReply, ContentID: r.ID}, Category: policy.Harassment}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("沒有回覆：%v", err)
	}
	st := mustV[seller.Store](t)(f.sellerRd.Store(ctx, storeID))
	mustV[review.Review](t)(f.review.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: "回覆"}))
	res := f.submit(t, reporter, Target{Kind: TargetReviewReply, ContentID: r.ID}, policy.Harassment)
	for _, a := range []Action{ActionHide, ActionWarn, ActionBan} {
		if _, err := f.decide(t, res.Case, a, policy.Spam); kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s 對回覆：%v", a, kindOf(err))
		}
	}
}

// 店家檢舉人只收聊天訊息、而且要是那段對話的一方、不能檢舉自己；去重、冪等與額度依檢舉人種類各自計。
func TestStoreReporterIsAcceptedOnlyForChatMessages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 4, 0)
	submit := func(store int64, kind TargetKind, id uuid.UUID, rid *uuid.UUID) (SubmitResult, error) {
		return f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: store, Target: Target{Kind: kind, ContentID: id}, Category: policy.Harassment, RequestID: rid})
	}
	rid := uuid.New()
	first, err := submit(storeID, TargetChatMessage, ids[1], &rid)
	if err != nil || !first.Created || first.Report.ReporterStoreID != storeID || first.Report.ReporterBuyerID != 0 {
		t.Fatalf("店家檢舉買家寄的訊息：%+v %v", first, err)
	}
	if n := f.count(t, `SELECT count(*) FROM moderation.reports WHERE id = $1 AND reporter_store_id = $2 AND reporter_buyer_id IS NULL`, first.Report.ID, storeID); n != 1 {
		t.Fatalf("檢舉列的檢舉人 = %d", n)
	}
	again, err := submit(storeID, TargetChatMessage, ids[1], nil)
	if err != nil || again.Created || again.Report.ID != first.Report.ID {
		t.Fatalf("同店家重送回既有：%+v %v", again, err)
	}
	replay, err := submit(storeID, TargetChatMessage, ids[1], &rid)
	if err != nil || replay.Report.ID != first.Report.ID {
		t.Fatalf("同 request_id 重送回第一次：%+v %v", replay, err)
	}
	if _, err := submit(storeID, TargetChatMessage, ids[2], nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("店家檢舉自己寄的訊息：%v", err)
	}
	if _, err := submit(otherStore, TargetChatMessage, ids[3], nil); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("非對話方的店家：%v", err)
	}
	r := f.reviewBy(t, f.buyerProfile(t), storeID, 2, "普通")
	if _, err := submit(storeID, TargetReview, r.ID, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("店家檢舉評價：%v", err)
	}
	if _, err := submit(storeID, TargetReviewReply, r.ID, nil); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("店家檢舉評價回覆：%v", err)
	}
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: b, ReporterStoreID: storeID, Target: Target{Kind: TargetChatMessage, ContentID: ids[3]}, Category: policy.Harassment}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("買家與店家同時：%v", err)
	}
}

// 店家的每日額度 20，與買家各自計數：店家用完後買家仍能檢舉，反之亦然。
func TestStoreDailyQuotaIsCountedSeparatelyFromTheBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 44, 0)
	for i := range DailyReportLimit {
		seq := int64(2*i + 1)
		if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: storeID, Target: Target{Kind: TargetChatMessage, ContentID: ids[seq]}, Category: policy.Harassment}); err != nil {
			t.Fatalf("第 %d 筆：%v", i+1, err)
		}
	}
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterStoreID: storeID, Target: Target{Kind: TargetChatMessage, ContentID: ids[41]}, Category: policy.Harassment}); kindOf(err) != apperr.KindResourceExhausted {
		t.Fatalf("第 21 筆：%v", err)
	}
	if _, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: b, Target: Target{Kind: TargetChatMessage, ContentID: ids[2]}, Category: policy.Harassment}); err != nil {
		t.Fatalf("買家的額度不受店家影響：%v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM moderation.reporter_daily_counts WHERE reporter_store_id = $1 AND submitted = $2`, storeID, DailyReportLimit); n != 1 {
		t.Fatalf("店家的計數列 = %d", n)
	}
}

// 檢舉人恰好一欄：三張表的 CHECK 都擋掉兩欄皆空與兩欄皆有。
func TestReporterKindCheckRejectsNoneAndBoth(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 2, 0)
	res := f.submit(t, b, Target{Kind: TargetChatMessage, ContentID: ids[2]}, policy.Harassment)
	caseArg := map[string]bool{"reports 皆空": true, "reports 皆有": true, "requests 皆空": true, "requests 皆有": true}
	for name, sql := range map[string]string{
		"reports 皆空":      `INSERT INTO moderation.reports (id, case_id, category, created_at) VALUES (gen_random_uuid(), $1, 'SPAM', now())`,
		"reports 皆有":      `INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, reporter_store_id, category, created_at) VALUES (gen_random_uuid(), $1, 8, 9, 'SPAM', now())`,
		"requests 皆空":     `INSERT INTO moderation.report_requests (request_id, report_id, request_hash) VALUES (gen_random_uuid(), (SELECT id FROM moderation.reports WHERE case_id = $1 LIMIT 1), '\x0000000000000000000000000000000000000000000000000000000000000000')`,
		"requests 皆有":     `INSERT INTO moderation.report_requests (reporter_buyer_id, reporter_store_id, request_id, report_id, request_hash) VALUES (8, 9, gen_random_uuid(), (SELECT id FROM moderation.reports WHERE case_id = $1 LIMIT 1), '\x0000000000000000000000000000000000000000000000000000000000000000')`,
		"daily counts 皆空": `INSERT INTO moderation.reporter_daily_counts (day, submitted) VALUES (current_date, 1)`,
		"daily counts 皆有": `INSERT INTO moderation.reporter_daily_counts (reporter_buyer_id, reporter_store_id, day, submitted) VALUES (8, 9, current_date, 1)`,
	} {
		var args []any
		if caseArg[name] {
			args = []any{res.Case.ID}
		}
		_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s：預期 CHECK 違反，得到 %v", name, err)
		}
	}
}

// 訂閱者核對歸屬：別家店的訊息、買家寄的訊息不能被這家店的措施下架，措施整個回滾。
func TestTakedownRejectsAMessageThatIsNotTheMeasureStores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	otherStore, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, b, storeID, 2, 0)
	create := func(store int64, id uuid.UUID) error {
		_, err := f.seller.CreateMeasure(ctx, seller.CreateMeasureInput{
			AdminID: 3, StoreID: store, Kind: seller.MeasureContentTakedown, StatementToSeller: "下架", Note: "測試",
			Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.Spam, Severity: seller.Severity(seller.SeverityValues()[0])}},
			Takedown: &seller.ContentTakedown{Contents: []seller.TakedownContent{{
				ContentRef: seller.ContentRef{Kind: contentkind.ChatMessage, ID: id},
				Snapshot:   &seller.ContentSnapshot{Body: "m", AuthoredAt: f.clock()},
			}}},
		})
		return err
	}
	var bad *im.TakedownContentError
	if err := create(otherStore, ids[2]); !errors.As(err, &bad) {
		t.Fatalf("別家店的訊息：%v", err)
	}
	if err := create(storeID, ids[1]); !errors.As(err, &bad) {
		t.Fatalf("買家寄的訊息：%v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM seller.measures WHERE kind = 'CONTENT_TAKEDOWN'`); n != 0 {
		t.Fatalf("措施要回滾：%d", n)
	}
	if err := create(storeID, ids[2]); err != nil {
		t.Fatalf("自家店的訊息：%v", err)
	}
	if err := create(storeID, uuid.NewV7()); err != nil {
		t.Fatalf("已不存在的訊息成立（同 social）：%v", err)
	}
}

// 鎖序 seller(4) → im(13) → moderation(24)：同一則店家訊息上 TAKEDOWN 的 Decide、店家身分的抹除與買家 Send 並行，無死鎖、無鎖序違規。
func TestTakedownRacesWithEraseAndSendWithoutDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	const rounds = 8
	for i := range rounds {
		b, _ := f.newBuyer(t)
		conv, ids := f.seedChat(t, b, storeID, 2, 0)
		_ = conv
		res := f.submit(t, b, Target{Kind: TargetChatMessage, ContentID: ids[2]}, policy.Harassment)
		errs := make([]error, 3)
		var wg sync.WaitGroup
		ops := []func() error{
			func() error {
				_, err := f.module.DecideCase(context.Background(), f.takedownInput(res.Case))
				return err
			},
			func() error { return f.im.ErasePartyMessages(context.Background(), im.PartyStore, storeID) },
			func() error {
				_, err := f.im.Send(context.Background(), im.SendInput{Sender: im.Party{Kind: im.PartyBuyer, BuyerID: b}, CounterpartID: storeID, Type: im.TypeText, Text: "並行送出"})
				return err
			},
		}
		for j, op := range ops {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[j] = op()
			}()
		}
		wg.Wait()
		for j, err := range errs {
			var pgErr *pgconn.PgError
			switch {
			case err == nil:
			case errors.As(err, &pgErr) && (pgErr.Code == "40P01" || pgErr.Code == "40001"):
				t.Errorf("第 %d 輪操作 %d 死鎖或序列化失敗：%v", i, j, err)
			case j != 2 && err != nil:
				t.Errorf("第 %d 輪操作 %d 失敗：%v", i, j, err)
			case apperr.KindOf(err) == apperr.KindInternal:
				t.Errorf("第 %d 輪的 Send 回了內部錯誤：%v", i, err)
			}
		}
		if n := f.count(t, `SELECT count(*) FROM seller.measure_contents WHERE content_kind = 'CHAT_MESSAGE' AND content_id = $1`, ids[2]); n != 1 {
			t.Errorf("第 %d 輪：TAKEDOWN 成功後訊息要有一筆措施對象：%d", i, n)
		}
	}
}

// 鎖序 seller(4) → review(12) → moderation(24)：同一則評價上回覆下架的 Decide 與評價的 Hide 並行，無死鎖。
func TestReplyTakedownRacesWithReviewHideWithoutDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	st := mustV[seller.Store](t)(f.sellerRd.Store(ctx, storeID))
	const rounds = 8
	for i := range rounds {
		reporter, _ := f.newBuyer(t)
		r := f.reviewBy(t, f.buyerProfile(t), storeID, 3, "普通")
		mustV[review.Review](t)(f.review.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: "回覆"}))
		res := f.submit(t, reporter, Target{Kind: TargetReviewReply, ContentID: r.ID}, policy.Harassment)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		ops := []func() error{
			func() error { _, err := f.module.DecideCase(ctx, f.takedownInput(res.Case)); return err },
			func() error {
				_, err := f.review.Hide(ctx, review.HideInput{ReviewID: r.ID, Category: string(policy.Spam), Text: "先隱藏", SourceRef: "test", AdminID: 1})
				return err
			},
		}
		for j, op := range ops {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[j] = op()
			}()
		}
		wg.Wait()
		for j, err := range errs {
			if err != nil {
				t.Errorf("第 %d 輪操作 %d 失敗：%v", i, j, err)
			}
		}
	}
}
