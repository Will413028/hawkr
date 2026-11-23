//go:build integration

package moderation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const reverseAction = "/test/ReverseHide"

// reviewBy 讓買家對店家寫一則評價（訂單由替身提供，已完成一天）。
func (f *fixture) reviewBy(t *testing.T, b buyer.Profile, storeID int64, rating int, content string, tokens ...string) review.Review {
	t.Helper()
	n := f.next() + 1000
	no := fmt.Sprintf("N%d", n)
	f.orders.mu.Lock()
	f.orders.byNo[no] = review.OrderForReview{
		OrderNo: no, BuyerID: b.BuyerID, StoreID: storeID, Status: review.OrderStatusCompleted, CompletedAt: f.clock().Add(-24 * time.Hour),
		Items: []review.OrderItem{{OrderItemID: n, ProductID: 77, SkuName: "紅茶 100g", Price: 120, ImageKey: "order/img.jpg"}},
	}
	f.orders.mu.Unlock()
	r, err := f.review.Submit(context.Background(), review.SubmitInput{BuyerID: b.BuyerID, OrderNo: no, OrderItemID: n, Rating: rating, Content: content, StagedTokens: tokens})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *fixture) buyerProfile(t *testing.T) buyer.Profile {
	t.Helper()
	_, account := f.newBuyer(t)
	p, err := f.buyer.Profile(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// seedChat 直接寫一個對話與 n 則訊息（seq 1..n，單數是買家寄、雙數是店家寄，文字是 m<seq>）；imageSeq 大於 0 的那一則改成圖片訊息（引用真的 media 裡一筆已確認的資產，ID 放在 f.chatAsset，它是來源，快照指向的是 moderation 自己的複本，B1-D20）。
func (f *fixture) seedChat(t *testing.T, buyerID, storeID int64, n, imageSeq int) (conv uuid.UUID, ids map[int64]uuid.UUID) {
	t.Helper()
	conv = uuid.NewV7()
	base := f.clock()
	f.exec(t, `INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, last_message_at, created_at) VALUES ($1, $2, $3, $4, $5, $5)`,
		conv, buyerID, storeID, n, base.Add(time.Duration(n)*time.Second))
	ids = map[int64]uuid.UUID{}
	for seq := int64(1); seq <= int64(n); seq++ {
		id := uuid.NewV7()
		ids[seq] = id
		kind := "BUYER"
		if seq%2 == 0 {
			kind = "STORE"
		}
		at := base.Add(time.Duration(seq) * time.Second)
		if seq == int64(imageSeq) {
			var assetID uuid.UUID
			if err := f.db.InTx(context.Background(), func(ctx context.Context) error {
				a, err := f.media.Confirm(ctx, stagedUploader, f.stage(t), media.VisibilityPublic)
				assetID = a.ID
				return err
			}); err != nil {
				t.Fatal(err)
			}
			f.chatAsset = assetID
			f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, image_asset_id, image_object_key, created_at)
				VALUES ($1, $2, $3, $4, 'IMAGE', $6, 'chat/ctx.jpg', $5)`, id, conv, seq, kind, at, assetID)
			continue
		}
		f.exec(t, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at) VALUES ($1, $2, $3, $4, 'TEXT', $5, $6)`,
			id, conv, seq, kind, fmt.Sprintf("m%d", seq), at)
	}
	return conv, ids
}

func (f *fixture) hideMessage(t *testing.T, id uuid.UUID) {
	t.Helper()
	if _, err := f.im.Hide(context.Background(), im.HideInput{MessageID: id, AdminID: 1, Category: string(policy.Spam), Text: "先隱藏", SourceRef: "test"}); err != nil {
		t.Fatal(err)
	}
}

// openChatCase 直接建立一個進行中的聊天訊息案件與它的快照：買家單方檢舉下，聊天案件的作者全是店家（設計 §0.4.1 第 2 點），買家寄的訊息沒有檢舉入口（店家檢舉人在 A6-4c），
// 但處置路徑（作者取自快照）要被走到。
func (f *fixture) openChatCase(t *testing.T, messageID uuid.UUID, c snapshot.Content) Case {
	t.Helper()
	id := uuid.NewV7()
	text, sum := snapshot.Encode(c)
	now := f.clock()
	f.exec(t, `INSERT INTO moderation.cases (id, target_kind, content_id, opened_at) VALUES ($1, 'CHAT_MESSAGE', $2, $3)`, id, messageID, now)
	f.exec(t, `INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, asset_ids, captured_at) VALUES ($1, $2, $3, $4, '{}', $5)`, uuid.NewV7(), id, sum[:], text, now)
	return mustV[CaseDetail](t)(f.module.Case(context.Background(), id)).Case
}

// cloneCase 直接建立同一個內容的第二個案件並複製快照：內容已被隱藏，不能再經 SubmitReport 檢舉（§1.1／§1.2，已隱藏回 NotFound）。
func (f *fixture) cloneCase(t *testing.T, src Case) Case {
	t.Helper()
	id := uuid.NewV7()
	now := f.clock()
	f.exec(t, `INSERT INTO moderation.cases (id, target_kind, content_id, opened_at) SELECT $1, target_kind, content_id, $2 FROM moderation.cases WHERE id = $3`, id, now, src.ID)
	f.exec(t, `INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, asset_ids, captured_at) SELECT $1, $2, content_sha256, content, asset_ids, $3 FROM moderation.case_snapshots WHERE case_id = $4`, uuid.NewV7(), id, now, src.ID)
	return mustV[CaseDetail](t)(f.module.Case(context.Background(), id)).Case
}

func buyerChatSnapshot(buyerID int64, conv uuid.UUID, seq int64) snapshot.Content {
	return snapshot.Content{Kind: TargetChatMessage, Chat: &snapshot.Chat{
		AuthorKind: "BUYER", AuthorID: buyerID, Body: fmt.Sprintf("m%d", seq), ConversationID: conv.String(), Seq: seq, AuthoredAt: time.Date(2026, 11, 2, 4, 0, 0, 0, time.UTC),
	}}
}

func (f *fixture) reverse(id uuid.UUID) (CaseDetail, error) {
	return f.module.ReverseHide(context.Background(), ReverseInput{CaseID: id, Actor: adminaudit.Actor{AdminID: 9, Action: reverseAction}, Reason: "誤判"})
}

func (f *fixture) hidden(t *testing.T, party im.Party, conv uuid.UUID, seq int64) (hidden bool, text *string) {
	t.Helper()
	page := mustV[im.MessagePage](t)(f.im.ListMessages(context.Background(), conv, party, 0, 100))
	for _, m := range page.Items {
		if m.Seq == seq {
			return m.Hidden, m.Text
		}
	}
	t.Fatalf("對話裡沒有 seq %d", seq)
	return false, nil
}

// 8.1-1：檢舉 → 快照 → 隱藏 → 還原（評價）。公開讀取與統計隨隱藏消失、隨還原回來，隱藏與還原各發一次 ReviewAggregateChanged；案件維持結案、決定不改，撤銷紀錄與稽核在原案件旁。
func TestReviewReportHideAndReverse(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	author := f.buyerProfile(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, author, storeID, 2, "不好吃", f.stage(t))
	if len(r.Images) != 1 {
		t.Fatalf("評價要有一張圖：%+v", r.Images)
	}

	res := f.submit(t, reporter, Target{Kind: TargetReview, ContentID: r.ID}, policy.Spam)
	snap := caseSnapshot(t, f, res.Case.ID)
	if snap == nil || snap.Review == nil || snap.Kind != TargetReview {
		t.Fatalf("評價案件的快照：%+v", snap)
	}
	if snap.Review.Rating != 2 || snap.Review.Body != "不好吃" || snap.Review.AuthorID != author.BuyerID || snap.Review.SubjectStoreID != storeID || snap.Review.ProductID != 77 ||
		len(snap.Review.MediaRefs) != 1 || snap.Review.MediaRefs[0] == r.Images[0].AssetID.String() {
		t.Fatalf("快照欄位：%+v", snap.Review)
	}
	if snap.StoreID() != 0 {
		t.Fatal("評價所屬的店家不是作者，不能放進歸屬店家")
	}
	if got := f.count(t, `SELECT cardinality(asset_ids) FROM moderation.case_snapshots WHERE case_id = $1 AND $2::uuid = ANY(asset_ids)`, res.Case.ID, snap.Review.MediaRefs[0]); got != 1 {
		t.Fatalf("快照指向 moderation 自己的複本（B1-D20），複本要記在 asset_ids：%d", got)
	}

	before := f.aggs.Load()
	mustV[CaseDetail](t)(f.decide(t, res.Case, ActionHide, policy.Spam))
	if _, err := f.review.GetReview(ctx, r.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("隱藏後 GetReview：%v", err)
	}
	if st := mustV[review.ProductStats](t)(f.review.ProductStats(ctx, r.ProductID)); st.Count != 0 {
		t.Fatalf("隱藏後統計要不計：%+v", st)
	}
	if n := f.count(t, "SELECT count(*) FROM review.hides WHERE review_id = $1 AND source_ref = $2 AND restored_at IS NULL", r.ID, SourceRefPrefix+res.Case.ID.String()); n != 1 {
		t.Fatalf("隱藏紀錄 = %d", n)
	}
	if got := f.aggs.Load() - before; got != 1 {
		t.Fatalf("隱藏發了 %d 次 ReviewAggregateChanged", got)
	}

	before = f.aggs.Load()
	d := mustV[CaseDetail](t)(f.reverse(res.Case.ID))
	if d.Case.ClosedAt == nil || d.Case.Decision == nil || *d.Case.Decision != DecisionViolation || d.Case.Action == nil || *d.Case.Action != ActionHide {
		t.Fatalf("撤銷不改決定、案件維持結案：%+v", d.Case)
	}
	if d.Case.Reversal == nil || d.Case.Reversal.AdminID != 9 || d.Case.Reversal.Reason != "誤判" || d.Case.Reversal.At.IsZero() {
		t.Fatalf("撤銷紀錄：%+v", d.Case.Reversal)
	}
	if got := mustV[review.Review](t)(f.review.GetReview(ctx, r.ID, 0)); got.ID != r.ID {
		t.Fatalf("還原後 GetReview：%+v", got)
	}
	if st := mustV[review.ProductStats](t)(f.review.ProductStats(ctx, r.ProductID)); st.Count != 1 {
		t.Fatalf("還原後統計要回復：%+v", st)
	}
	if n := f.count(t, "SELECT count(*) FROM review.hides WHERE review_id = $1 AND restored_at IS NOT NULL AND restored_by = 9", r.ID); n != 1 {
		t.Fatalf("還原留列：%d", n)
	}
	if got := f.aggs.Load() - before; got != 1 {
		t.Fatalf("還原發了 %d 次 ReviewAggregateChanged", got)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND closed_at IS NOT NULL AND reversed_at IS NOT NULL AND reversed_by_admin_id = 9 AND reverse_reason = '誤判'", res.Case.ID); n != 1 {
		t.Fatal("cases 的 reversed_* 要有值")
	}
	if n := f.count(t, "SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_type = 'moderation_case' AND target_id = $2 AND admin_id = 9", reverseAction, res.Case.ID.String()); n != 1 {
		t.Fatalf("稽核紀錄 = %d", n)
	}

	if _, err := f.reverse(res.Case.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("撤銷兩次：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_id = $2", reverseAction, res.Case.ID.String()); n != 1 {
		t.Fatalf("失敗的撤銷不留稽核：%d", n)
	}
}

// ReverseHide 只對處置是 HIDE 的已結案案件：進行中、WARN、不違規都是 FailedPrecondition；不存在是 NotFound；缺案件、原因空白是 InvalidInput。
func TestReverseHideRequiresAClosedHideCase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	open := f.openCase(t, commentTarget(w.comment))
	if _, err := f.reverse(open.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("進行中的案件：%v", err)
	}
	warn := f.openCase(t, Target{Kind: TargetComment, ContentID: f.comment(t, w.post.ID, w.author, "另一則").ID})
	mustV[CaseDetail](t)(f.decide(t, warn, ActionWarn, policy.Harassment))
	if _, err := f.reverse(warn.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("WARN 案件：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND reversed_at IS NOT NULL", warn.ID); n != 0 {
		t.Fatal("WARN 案件不得有撤銷")
	}
	dismissed := f.openCase(t, Target{Kind: TargetComment, ContentID: f.comment(t, w.post.ID, w.author, "第三則").ID})
	mustV[CaseDetail](t)(f.module.DecideCase(context.Background(), DecideInput{CaseID: dismissed.ID, Version: dismissed.Version, Actor: adminaudit.Actor{AdminID: 9, Action: "/test/Decide"}, Decision: DecisionNoViolation}))
	if _, err := f.reverse(dismissed.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("不違規的案件：%v", err)
	}
	if _, err := f.reverse(uuid.NewV7()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的案件：%v", err)
	}
	for name, in := range map[string]ReverseInput{
		"沒有案件": {Actor: adminaudit.Actor{AdminID: 9, Action: reverseAction}, Reason: "x"},
		"原因空白": {CaseID: open.ID, Actor: adminaudit.Actor{AdminID: 9, Action: reverseAction}, Reason: " "},
	} {
		if _, err := f.module.ReverseHide(context.Background(), in); kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

// 撤銷社群留言的隱藏：同一個入口也還原 social（W1 讓社群內容一併受惠）。
func TestReverseHideRestoresACommentAndRecordsTheReversal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	c := f.openCase(t, commentTarget(w.comment))
	mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	if f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatal("留言要被隱藏")
	}
	d := mustV[CaseDetail](t)(f.reverse(c.ID))
	if !f.readable(t, social.TargetComment, w.comment.ID) {
		t.Fatal("撤銷後留言要看得到")
	}
	if d.Case.ClosedAt == nil || d.Case.Reversal == nil {
		t.Fatalf("%+v", d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM social.hides WHERE comment_id = $1 AND restored_at IS NOT NULL", w.comment.ID); n != 1 {
		t.Fatalf("social 的還原留列：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_id = $2 AND detail->>'restored' = 'true'", reverseAction, c.ID.String()); n != 1 {
		t.Fatalf("實際還原時稽核要記 restored=true：%d", n)
	}
}

// 8.1-2（檢舉面）：快照的 Context 恰好前 5、後 5、不含被檢舉的那則；被隱藏的前文是墓碑，隱藏的內容不進快照；前後文的圖片一併被證據保全；
// 店家寫的訊息在買家單方檢舉下 HIDE 仍是 FailedPrecondition（W2 在 A6-4c）。
func TestChatReportFreezesFiveBeforeAndAfterWithTombstones(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	reporter, _ := f.newBuyer(t)
	conv, ids := f.seedChat(t, reporter, storeID, 14, 9)
	f.hideMessage(t, ids[3])

	res := f.submit(t, reporter, Target{Kind: TargetChatMessage, ContentID: ids[6]}, policy.Harassment)
	snap := caseSnapshot(t, f, res.Case.ID)
	if snap == nil || snap.Chat == nil {
		t.Fatalf("聊天案件的快照：%+v", snap)
	}
	c := snap.Chat
	if c.AuthorKind != "STORE" || c.AuthorID != storeID || c.Seq != 6 || c.Body != "m6" || c.ConversationID != conv.String() || snap.StoreID() != storeID {
		t.Fatalf("被檢舉的訊息：%+v", c)
	}
	var seqs []int64
	for _, m := range c.Context {
		seqs = append(seqs, m.Seq)
	}
	want := []int64{1, 2, 3, 4, 5, 7, 8, 9, 10, 11}
	if fmt.Sprint(seqs) != fmt.Sprint(want) {
		t.Fatalf("前後文 seq = %v，預期 %v（前 5、後 5、不含被檢舉的）", seqs, want)
	}
	for _, m := range c.Context {
		switch m.Seq {
		case 3:
			if !m.Hidden || m.Body != "" || len(m.MediaRefs) != 0 {
				t.Fatalf("被隱藏的前文要是墓碑：%+v", m)
			}
		case 9:
			if m.Type != "IMAGE" || len(m.MediaRefs) != 1 || m.MediaRefs[0] == f.chatAsset.String() {
				t.Fatalf("圖片訊息：%+v", m)
			}
		default:
			if m.Body != fmt.Sprintf("m%d", m.Seq) || m.Hidden {
				t.Fatalf("一般訊息：%+v", m)
			}
		}
	}
	if strings.Contains(fmt.Sprintf("%+v", c.Context), "m3") {
		t.Fatalf("被隱藏的前文內容不得出現在快照前後文：%+v", c.Context)
	}
	var raw string
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT content FROM moderation.case_snapshots WHERE case_id = $1", res.Case.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, `"m3"`) {
		t.Fatal("被隱藏的訊息內容不得寫進證據快照")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.case_snapshots WHERE case_id = $1 AND $2::uuid = ANY(asset_ids)", res.Case.ID, f.chatAsset); n != 0 {
		t.Fatal("快照不得引用來源資產，要指向自己的複本（B1-D20）")
	}
	if n := f.count(t, "SELECT cardinality(asset_ids) FROM moderation.case_snapshots WHERE case_id = $1", res.Case.ID); n != 1 {
		t.Fatal("前後文的圖片也要被證據保全")
	}

	if _, err := f.decide(t, res.Case, ActionHide, policy.Harassment); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("店家寫的聊天訊息 HIDE：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1", ids[6]); n != 0 {
		t.Fatalf("失敗的處置不留隱藏：%d", n)
	}
}

// 8.1-2（處置面）：買家寄的聊天訊息（案件直接建立）HIDE → 讀取路徑回墓碑、對話預覽 Hidden 且不帶文字（ID1）→ 撤銷 → 內容與預覽回來；HIDE 只走 im，不走 review。
func TestChatHideAndReverseThroughIM(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	conv, ids := f.seedChat(t, b, storeID, 7, 0)
	party := im.Party{Kind: im.PartyBuyer, BuyerID: b}
	c := f.openChatCase(t, ids[7], buyerChatSnapshot(b, conv, 7))

	mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND source_ref = $2 AND restored_at IS NULL AND admin_id = 9", ids[7], SourceRefPrefix+c.ID.String()); n != 1 {
		t.Fatalf("im 的隱藏紀錄 = %d", n)
	}
	if hidden, text := f.hidden(t, party, conv, 7); !hidden || text != nil {
		t.Fatalf("ListMessages 要回墓碑：hidden=%v text=%v", hidden, text)
	}
	after := mustV[im.MessagePage](t)(f.im.MessagesAfter(ctx, conv, party, 6, 10))
	if len(after.Items) != 1 || !after.Items[0].Hidden || after.Items[0].Text != nil {
		t.Fatalf("MessagesAfter 要回墓碑：%+v", after.Items)
	}
	convs := mustV[im.ConversationPage](t)(f.im.ListConversations(ctx, party, nil, 10))
	if len(convs.Items) != 1 || !convs.Items[0].Preview.Hidden || convs.Items[0].Preview.Text != nil {
		t.Fatalf("對話預覽要是 Hidden 且不帶文字：%+v", convs.Items)
	}

	mustV[CaseDetail](t)(f.reverse(c.ID))
	if hidden, text := f.hidden(t, party, conv, 7); hidden || text == nil || *text != "m7" {
		t.Fatalf("還原後訊息要回來：hidden=%v text=%v", hidden, text)
	}
	convs = mustV[im.ConversationPage](t)(f.im.ListConversations(ctx, party, nil, 10))
	if convs.Items[0].Preview.Hidden || convs.Items[0].Preview.Text == nil || *convs.Items[0].Preview.Text != "m7" {
		t.Fatalf("還原後預覽回到內容：%+v", convs.Items[0].Preview)
	}
	if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND restored_at IS NOT NULL AND restored_by = 9", ids[7]); n != 1 {
		t.Fatalf("還原留列：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND closed_at IS NOT NULL AND reversed_at IS NOT NULL", c.ID); n != 1 {
		t.Fatal("案件維持結案並記下撤銷")
	}
}

// 8.1-3：非對話方檢舉訊息 → NotFound；檢舉已隱藏的訊息 → NotFound；檢舉自己的訊息或評價 → InvalidInput；評價被隱藏後不能再被檢舉。
func TestReportAuthorizationForChatAndReview(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)
	party, _ := f.newBuyer(t)
	stranger, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, party, storeID, 6, 0)
	submit := func(reporter int64, kind TargetKind, id uuid.UUID) error {
		_, err := f.module.SubmitReport(ctx, SubmitInput{ReporterBuyerID: reporter, Target: Target{Kind: kind, ContentID: id}, Category: policy.Harassment})
		return err
	}
	if err := submit(stranger, TargetChatMessage, ids[2]); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("非對話方：%v", err)
	}
	if err := submit(party, TargetChatMessage, ids[1]); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("檢舉自己的訊息：%v", err)
	}
	f.hideMessage(t, ids[4])
	if err := submit(party, TargetChatMessage, ids[4]); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("已隱藏的訊息：%v", err)
	}
	if err := submit(party, TargetChatMessage, uuid.NewV7()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的訊息：%v", err)
	}
	if err := submit(party, TargetChatMessage, ids[2]); err != nil {
		t.Fatalf("對話方檢舉店家寫的訊息：%v", err)
	}

	author := f.buyerProfile(t)
	r := f.reviewBy(t, author, storeID, 1, "很差")
	if err := submit(author.BuyerID, TargetReview, r.ID); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("檢舉自己的評價：%v", err)
	}
	if err := submit(stranger, TargetReview, uuid.NewV7()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在的評價：%v", err)
	}
	if _, err := f.review.Hide(ctx, review.HideInput{ReviewID: r.ID, Category: string(policy.Spam), Text: "先隱藏", SourceRef: "test", AdminID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := submit(stranger, TargetReview, r.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("已隱藏的評價：%v", err)
	}
	r2 := f.reviewBy(t, f.buyerProfile(t), storeID, 2, "普通")
	blocker, _ := f.newBuyer(t)
	if err := submit(blocker, TargetReview, r2.ID); err != nil {
		t.Fatalf("未封鎖時可檢舉：%v", err)
	}
	r3 := f.reviewBy(t, f.buyerProfile(t), storeID, 2, "普通")
	mustV[bool](t)(f.buyer.BlockBuyer(ctx, blocker, r3.BuyerID))
	if err := submit(blocker, TargetReview, r3.ID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("觀看者封鎖評價作者後不可檢舉（同 GetReview 的可讀規則）：%v", err)
	}
}

// 8.1-4：評價已被刪帳清除後 HIDE → 成功、沒有隱藏紀錄（對齊 TestHideOfDeletedBuyerContentSucceedsWithoutCallingSocial）；作者取自快照。
func TestHideOfAPurgedReviewSucceedsWithoutAHideRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	author := f.buyerProfile(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, author, storeID, 3, "普通")
	c := f.submit(t, reporter, Target{Kind: TargetReview, ContentID: r.ID}, policy.Spam).Case
	f.exec(t, "DELETE FROM review.reviews WHERE id = $1", r.ID)

	d := mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	if d.Case.ClosedAt == nil || d.Case.Decision == nil || *d.Case.Decision != DecisionViolation {
		t.Fatalf("案件要結為違規：%+v", d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM review.hides"); n != 0 {
		t.Fatalf("評價已清除不寫隱藏，卻有 %d 筆", n)
	}
}

// 8.1-5：作者已關閉（買家 CLOSED、評價作者刪帳中）時 HIDE 仍成功：隱藏與還原不取擁有者的守門。
func TestHideWorksWhileTheAuthorIsDeactivated(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, _ := f.exposableStore(t)

	closed := int64(900000 + f.next())
	f.exec(t, `INSERT INTO buyer.buyers (buyer_id, account_id, status, display_name, created_at, updated_at, deleted_at)
		OVERRIDING SYSTEM VALUE VALUES ($1, $2, 'CLOSED', 'b', now(), now(), now())`, closed, 1000000+closed)
	conv, ids := f.seedChat(t, closed, storeID, 3, 0)
	c := f.openChatCase(t, ids[3], buyerChatSnapshot(closed, conv, 3))
	mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND restored_at IS NULL", ids[3]); n != 1 {
		t.Fatalf("已關閉的買家寄的訊息仍要被隱藏：%d", n)
	}
	mustV[CaseDetail](t)(f.reverse(c.ID))
	if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND restored_at IS NOT NULL", ids[3]); n != 1 {
		t.Fatalf("已關閉的買家寄的訊息仍要能還原：%d", n)
	}

	author := f.buyerProfile(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, author, storeID, 2, "待刪帳")
	rc := f.submit(t, reporter, Target{Kind: TargetReview, ContentID: r.ID}, policy.Spam).Case
	if err := f.buyer.BeginDeletion(ctx, author.BuyerID); err != nil {
		t.Fatal(err)
	}
	mustV[CaseDetail](t)(f.decide(t, rc, ActionHide, policy.Spam))
	if n := f.count(t, "SELECT count(*) FROM review.hides WHERE review_id = $1 AND restored_at IS NULL", r.ID); n != 1 {
		t.Fatalf("刪帳中的作者的評價仍要被隱藏：%d", n)
	}
}

// 買家寄的聊天訊息與評價的 WARN、BAN 走作者（取自快照）：聊天案件的作者是店家時 WARN 回 FailedPrecondition，買家作者時警告記在 social。
func TestChatAndReviewWarnUseTheFrozenAuthor(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	reporter, _ := f.newBuyer(t)
	b, _ := f.newBuyer(t)
	_, ids := f.seedChat(t, reporter, storeID, 6, 0)
	storeCase := f.submit(t, reporter, Target{Kind: TargetChatMessage, ContentID: ids[2]}, policy.Harassment).Case
	if _, err := f.decide(t, storeCase, ActionWarn, policy.Harassment); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("店家寫的訊息不能警告買家：%v", err)
	}
	conv, ids2 := f.seedChat(t, b, storeID, 3, 0)
	c := f.openChatCase(t, ids2[3], buyerChatSnapshot(b, conv, 3))
	mustV[CaseDetail](t)(f.decide(t, c, ActionWarn, policy.Harassment))
	if n := f.count(t, "SELECT count(*) FROM social.sanctions WHERE buyer_id = $1 AND source_ref = $2", b, SourceRefPrefix+c.ID.String()); n != 1 {
		t.Fatalf("警告紀錄 = %d", n)
	}
}

// 8.1-6：同一則訊息上 Decide（HIDE）、im 的抹除、同對話的 Send 並行，各跑 N 次：沒有死鎖，鎖序檢查器（testdb）沒有違規；不假設先後。
func TestHideRacesWithEraseAndSendWithoutDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	const rounds = 8
	for i := range rounds {
		b, _ := f.newBuyer(t)
		conv, ids := f.seedChat(t, b, storeID, 3, 0)
		c := f.openChatCase(t, ids[3], buyerChatSnapshot(b, conv, 3))
		errs := make([]error, 3)
		var wg sync.WaitGroup
		ops := []func() error{
			func() error { _, err := f.decide(t, c, ActionHide, policy.Spam); return err },
			func() error { return f.im.ErasePartyMessages(context.Background(), im.PartyBuyer, b) },
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
		if n := f.count(t, "SELECT count(*) FROM im.hides WHERE message_id = $1 AND restored_at IS NULL", ids[3]); n != 1 {
			t.Errorf("第 %d 輪：HIDE 成功後訊息要有一筆未還原的隱藏：%d", i, n)
		}
	}
}

// 撤銷時內容已被清除（評價刪除）：同 Hide 的規則視為成功，撤銷仍記在案件上。殺死的變異：ReverseHide 在擁有者回 NotFound 時回錯。
func TestReverseHideOfAPurgedReviewStillRecordsTheReversal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	author := f.buyerProfile(t)
	reporter, _ := f.newBuyer(t)
	r := f.reviewBy(t, author, storeID, 3, "普通")
	c := f.submit(t, reporter, Target{Kind: TargetReview, ContentID: r.ID}, policy.Spam).Case
	mustV[CaseDetail](t)(f.decide(t, c, ActionHide, policy.Spam))
	f.exec(t, "DELETE FROM review.reviews WHERE id = $1", r.ID)

	d, err := f.reverse(c.ID)
	if err != nil {
		t.Fatalf("內容已清除的撤銷要成功：%v", err)
	}
	if d.Case.Reversal == nil || d.Case.ClosedAt == nil {
		t.Fatalf("案件維持結案並記下撤銷：%+v", d.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM adminaudit.records WHERE action = $1 AND target_id = $2 AND detail->>'restored' = 'false'", reverseAction, c.ID.String()); n != 1 {
		t.Fatalf("內容已清除時稽核要記 restored=false：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND reversed_at IS NOT NULL AND reversed_by_admin_id = 9", c.ID); n != 1 {
		t.Fatalf("reversed_* 要寫入：%d", n)
	}
}

// 冪等的 Hide 讓後到的案件沿用先到案件的那一筆：撤銷後到的案件不能還原別案的隱藏（FailedPrecondition，內容仍隱藏），撤銷先到的案件才還原。
// 殺死的變異：Restore 忽略 source_ref（B 的撤銷會還原 A 的隱藏）。
func TestReverseOfALaterCaseDoesNotRestoreAnEarlierCasesHide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	b, _ := f.newBuyer(t)
	conv, ids := f.seedChat(t, b, storeID, 7, 0)
	party := im.Party{Kind: im.PartyBuyer, BuyerID: b}
	a := f.openChatCase(t, ids[7], buyerChatSnapshot(b, conv, 7))
	mustV[CaseDetail](t)(f.decide(t, a, ActionHide, policy.Spam))
	bc := f.openChatCase(t, ids[7], buyerChatSnapshot(b, conv, 7))
	mustV[CaseDetail](t)(f.decide(t, bc, ActionHide, policy.Spam))

	if _, err := f.reverse(bc.ID); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("後到案件的撤銷：%v", err)
	}
	if hidden, _ := f.hidden(t, party, conv, 7); !hidden {
		t.Fatal("別案的隱藏不能被還原")
	}
	if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND reversed_at IS NOT NULL", bc.ID); n != 0 {
		t.Fatal("失敗的撤銷不能留下紀錄")
	}
	mustV[CaseDetail](t)(f.reverse(a.ID))
	if hidden, _ := f.hidden(t, party, conv, 7); hidden {
		t.Fatal("撤銷先到的案件要還原")
	}
}

func caseSnapshot(t *testing.T, f *fixture, id uuid.UUID) *Snapshot {
	t.Helper()
	d, snap, err := f.module.CaseWithSnapshot(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if d.Case.ID != id {
		t.Fatalf("案件與快照要是同一個案件：%+v", d.Case)
	}
	return snap
}

// CaseWithSnapshot 對真實資料庫：回案件（含檢舉）與同案件的快照；快照被清除後回 nil 而案件仍在；案件不存在是 NotFound。
// 殺死的變異：快照用別的案件 ID 查、案件不存在不回 NotFound、沒有快照時回錯。
func TestCaseWithSnapshotReturnsTheCaseAndItsSnapshot(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	w := f.world(t)
	other := f.openCase(t, Target{Kind: TargetPost, ContentID: w.post.ID})
	c := f.openCase(t, commentTarget(w.comment))
	d, snap, err := f.module.CaseWithSnapshot(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Case.ID != c.ID || len(d.Reports) != 1 || snap == nil || snap.Social == nil || snap.Kind != TargetComment || snap.Social.Body != "留言內容" {
		t.Fatalf("案件與快照：%+v %+v", d, snap)
	}
	if o := caseSnapshot(t, f, other.ID); o == nil || o.Kind != TargetPost {
		t.Fatalf("別的案件有自己的快照：%+v", o)
	}
	f.exec(t, "DELETE FROM moderation.case_snapshots WHERE case_id = $1", c.ID)
	d, snap, err = f.module.CaseWithSnapshot(ctx, c.ID)
	if err != nil || snap != nil || d.Case.ID != c.ID {
		t.Fatalf("快照被清除後案件仍在、快照是 nil：%+v %+v %v", d, snap, err)
	}
	if _, _, err := f.module.CaseWithSnapshot(ctx, uuid.New()); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("案件不存在：%v", err)
	}
}

// 第二個案件沿用第一個案件的隱藏（冪等 Hide）後，撤銷第二個回 FailedPrecondition、內容仍隱藏、不留紀錄；撤銷第一個才還原。
// 殺死的變異：review／social 的 Restore 忽略 source_ref。
func TestReverseOfALaterCaseDoesNotRestoreAnEarlierCasesHideForReviewAndComment(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID, _ := f.exposableStore(t)
	author := f.buyerProfile(t)
	r := f.reviewBy(t, author, storeID, 3, "普通")
	w := f.world(t)
	cases := []struct {
		name    string
		target  Target
		visible func() bool
	}{
		{"review", Target{Kind: TargetReview, ContentID: r.ID}, func() bool {
			_, err := f.review.GetReview(context.Background(), r.ID, 0)
			return err == nil
		}},
		{"comment", commentTarget(w.comment), func() bool { return f.readable(t, social.TargetComment, w.comment.ID) }},
	}
	for _, tc := range cases {
		a := f.openCase(t, tc.target)
		mustV[CaseDetail](t)(f.decide(t, a, ActionHide, policy.Spam))
		b := f.cloneCase(t, a)
		mustV[CaseDetail](t)(f.decide(t, b, ActionHide, policy.Spam))
		if _, err := f.reverse(b.ID); kindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("%s：後到案件的撤銷：%v", tc.name, err)
		}
		if tc.visible() {
			t.Fatalf("%s：別案的隱藏不能被還原", tc.name)
		}
		if n := f.count(t, "SELECT count(*) FROM moderation.cases WHERE id = $1 AND reversed_at IS NOT NULL", b.ID); n != 0 {
			t.Fatalf("%s：失敗的撤銷不能留下紀錄", tc.name)
		}
		mustV[CaseDetail](t)(f.reverse(a.ID))
		if !tc.visible() {
			t.Fatalf("%s：撤銷先到的案件要還原", tc.name)
		}
	}
}
