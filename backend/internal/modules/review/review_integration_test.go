//go:build integration

package review_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func hide(t *testing.T, f *fixture, id uuid.UUID) review.Hide {
	t.Helper()
	return must[review.Hide](t)(f.module.Hide(context.Background(), review.HideInput{ReviewID: id, Category: "SPAM", Text: "廣告", SourceRef: "case-1", AdminID: 1}))
}

// RD1：封鎖的過濾在 SQL 述詞內、keyset 之前：被擋的列不佔頁，也不出現；統計不過濾；店家封鎖買家不影響。
func TestRD1ViewerBlocksFilterBeforeThePageIsCut(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	const product = 700
	viewer := f.newBuyer(t)
	var authors []review.Review
	var buyersList []int64
	for range 5 {
		b := f.newBuyer(t)
		buyersList = append(buyersList, b.BuyerID)
		authors = append(authors, f.submit(t, b, storeID, product, 4, "好"))
	}
	r1, r2, r3, r4, r5 := authors[0], authors[1], authors[2], authors[3], authors[4]
	must[bool](t)(f.buyers.BlockBuyer(ctx, viewer.BuyerID, buyersList[4]))
	must[bool](t)(f.buyers.BlockBuyer(ctx, buyersList[3], viewer.BuyerID))

	anon := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(product), 0, review.Filter{}, nil, 50))
	if !sameIDs(ids(anon), r5.ID, r4.ID, r3.ID, r2.ID, r1.ID) {
		t.Fatalf("匿名看到全部：%v", ids(anon))
	}
	p1 := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(product), viewer.BuyerID, review.Filter{}, nil, 2))
	if !sameIDs(ids(p1), r3.ID, r2.ID) || p1.Next == nil {
		t.Fatalf("第一頁要是 r3、r2（被擋的不佔頁）且有下一頁：%v next=%v", ids(p1), p1.Next)
	}
	p2 := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(product), viewer.BuyerID, review.Filter{}, p1.Next, 2))
	if !sameIDs(ids(p2), r1.ID) || p2.Next != nil {
		t.Fatalf("第二頁只有 r1 且沒有下一頁：%v next=%v", ids(p2), p2.Next)
	}
	sp := must[review.Page](t)(f.module.ListStoreReviews(ctx, storeID, viewer.BuyerID, review.Filter{}, nil, 50))
	if !sameIDs(ids(sp), r3.ID, r2.ID, r1.ID) {
		t.Fatalf("店家評價頁也要過濾：%v", ids(sp))
	}
	own := must[review.Page](t)(f.module.ListOwnStoreReviews(ctx, storeID, review.Filter{}, nil, 50))
	if len(own.Reviews) != 5 {
		t.Fatalf("店家後台看到全部：%d", len(own.Reviews))
	}
	if st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(product))); st.Count != 5 {
		t.Fatalf("統計不過濾：%+v", st)
	}
	f.exec(t, "INSERT INTO seller.store_blocks (store_id, buyer_id, created_at) VALUES ($1, $2, $3)", storeID, buyersList[0], f.clock())
	again := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(product), viewer.BuyerID, review.Filter{}, nil, 50))
	if !sameIDs(ids(again), r3.ID, r2.ID, r1.ID) {
		t.Fatalf("店家封鎖買家不影響可見性：%v", ids(again))
	}
	must[bool](t)(f.buyers.BlockStore(ctx, viewer.BuyerID, storeID))
	if got := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(product), viewer.BuyerID, review.Filter{}, nil, 50)); len(got.Reviews) != 0 {
		t.Fatalf("封鎖店之後看不到：%v", ids(got))
	}
	if got := must[review.Page](t)(f.module.ListStoreReviews(ctx, storeID, 0, review.Filter{}, nil, 50)); len(got.Reviews) != 5 {
		t.Fatalf("匿名不受影響：%d", len(got.Reviews))
	}
}

// 觀看者查詢要能把封鎖的 pub_* 函式內聯（規則 R4：擁有者發布、單一 SELECT、計畫測試鎖住）：計畫裡沒有 Function Scan。
func TestViewerBlockPredicateInlinesThePubFunctions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	raw, err := os.ReadFile("db/queries/public.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)AND NOT EXISTS \(\s*SELECT 1 FROM buyer\.pub_blocked_buyers\(\).*?r\.store_id\)`).FindString(string(raw))
	if m == "" {
		t.Fatal("在 public.sql 找不到觀看者的封鎖子句")
	}
	clause := strings.ReplaceAll(regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(m, ""), "sqlc.arg(viewer_buyer_id)::bigint", "1")
	var plan []string
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "EXPLAIN SELECT r.id FROM review.reviews AS r WHERE r.product_id = '"+pid(700).String()+"' "+clause)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if got := strings.Join(plan, "\n"); strings.Contains(got, "Function Scan") {
		t.Fatalf("pub_* 函式沒有被內聯：\n%s", got)
	}
}

// 公開述詞（testdata/public_predicate.sql）的 pub_inactive_buyers join 要被 planner 內聯：計畫裡沒有 Function Scan。
func TestInactiveBuyersPredicateInlinesThePubFunction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	raw, err := os.ReadFile("testdata/public_predicate.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "buyer.pub_inactive_buyers()") {
		t.Fatal("述詞沒有 join buyer.pub_inactive_buyers()")
	}
	var plan []string
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "EXPLAIN SELECT r.id FROM review.reviews AS r WHERE r.product_id = '"+pid(700).String()+"' AND "+string(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if got := strings.Join(plan, "\n"); strings.Contains(got, "Function Scan") {
		t.Fatalf("pub_inactive_buyers 沒有被內聯：\n%s", got)
	}
}

// RD2：內容與回覆以字元計：1000 個中文字可過、1001 個被擋。
func TestRD2LengthIsCountedInRunes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	long := strings.Repeat("字", 1000)
	r := f.submit(t, b, storeID, 700, 5, long)
	if got := must[review.Review](t)(f.module.GetReview(ctx, r.ID, 0)); len([]rune(got.Content)) != 1000 {
		t.Fatalf("存下 1000 個字：%d", len([]rune(got.Content)))
	}
	it := f.order(b, storeID, 701)
	_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, Content: long + "字"})
	if kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("1001 個字要被擋：%v", err)
	}
	st := f.store(t, storeID)
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: long}); err != nil {
		t.Fatalf("回覆 1000 個字可過：%v", err)
	}
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: long + "字"}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("回覆 1001 個字要被擋：%v", err)
	}
}

// RD3：隱藏之後可以還原，還原留列、之後可以再隱藏；隱藏與還原都登記評分待重算並排快取失效，worker 跑過才寫 catalog；重複隱藏冪等。
func TestRD3HideCanBeRestoredAndHiddenAgain(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 3, "普通")
	f.runRatings(t)
	f.jobs.reset()
	f.ratings.reset()

	h := hide(t, f, r.ID)
	if _, ok := f.queueRow(t, pid(700)); !ok || f.ratings.callCount() != 0 || f.jobs.count("revalidate") != 1 {
		t.Fatalf("新隱藏只登記待重算、不碰 catalog：%+v ratings=%d jobs=%v", h, f.ratings.callCount(), f.jobs.kinds())
	}
	f.runRatings(t)
	afterHide := f.ratings.snapshot()
	if !h.Created || len(afterHide) != 1 || afterHide[0].productID != pid(700) || afterHide[0].average != 0 || afterHide[0].count != 0 || f.jobs.count("revalidate") != 2 {
		t.Fatalf("新隱藏：%+v ratings=%+v jobs=%v", h, afterHide, f.jobs.kinds())
	}
	if _, err := f.module.GetReview(ctx, r.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("隱藏後單筆讀取是 NotFound：%v", err)
	}
	again := hide(t, f, r.ID)
	if _, ok := f.queueRow(t, pid(700)); ok || again.Created || again.ID != h.ID {
		t.Fatalf("重複隱藏冪等、不再登記：%+v", again)
	}
	if st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); st.Count != 0 {
		t.Fatalf("隱藏的不計分：%+v", st)
	}

	f.advance(time.Hour)
	restored := must[review.Hide](t)(f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "誤判"}))
	f.runRatings(t)
	afterRestore := f.ratings.snapshot()
	if restored.RestoredAt == nil || len(afterRestore) != 2 || afterRestore[1].average != 3 || afterRestore[1].count != 1 {
		t.Fatalf("還原：%+v ratings=%+v", restored, afterRestore)
	}
	if got := must[review.Review](t)(f.module.GetReview(ctx, r.ID, 0)); got.ID != r.ID {
		t.Fatal("還原後可讀")
	}
	if st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); st.Count != 1 {
		t.Fatalf("還原後計分：%+v", st)
	}
	if _, err := f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "再還原"}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("沒有未還原的隱藏：%v", err)
	}
	if again := hide(t, f, r.ID); !again.Created || again.ID == h.ID {
		t.Fatalf("還原後可以再隱藏，留下兩列歷史：%+v", again)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.hides WHERE review_id = $1", r.ID); n != 2 {
		t.Fatalf("hides 列數 = %d", n)
	}
}

// RD6：評價的 product_id、sku_name、price、商品圖都取自訂單項目；之後訂單項目改變也不影響已存的快照。
func TestRD6SnapshotComesFromTheOrderItem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	r := must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5}))
	if r.ProductID != pid(700) || r.StoreID != storeID || r.SkuName != "紅茶 100g" || r.Price != 120 || r.ItemImageKey == nil || *r.ItemImageKey != "order/img.jpg" {
		t.Fatalf("快照要等於訂單項目：%+v", r)
	}
	f.orders.mu.Lock()
	o := f.orders.byNo[it.orderNo]
	o.Items[0].Price, o.Items[0].SkuName = 1, "改過"
	f.orders.mu.Unlock()
	got := must[review.Review](t)(f.module.GetReview(ctx, r.ID, 0))
	if got.Price != 120 || got.SkuName != "紅茶 100g" {
		t.Fatalf("快照建立後不變：%+v", got)
	}
}

// 沒有商品的訂單項目（RFQ 成交，product_id 全零）不能評價，不能走到 reviews_product_check 才失敗。
func TestSubmitRefusesAnOrderItemWithoutAProduct(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	b := f.newBuyer(t)
	it := f.order(b, f.exposableStore(t), 0)
	_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5})
	if kindOf(err) != apperr.KindFailedPrecondition || review.ReasonOf(err) != review.ReasonOrderNotReviewable {
		t.Fatalf("沒有商品的項目要回 ORDER_NOT_REVIEWABLE：%v", err)
	}
}

// RD7：最多 4 張圖：第 5 個 token 被擋（不是靜默截斷），DB 的 CHECK 是後備。
func TestRD7AtMostFourImages(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	tokens, assets := f.stage(t, b, 5)
	_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: tokens})
	if kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("5 個 token 要被擋：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.reviews"); n != 0 {
		t.Fatalf("被擋的提交不留評價：%d", n)
	}
	if got := f.assetState(t, assets[0]); got != "STAGED" {
		t.Fatalf("被擋的提交不得確認圖片：%s", got)
	}
	r := must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: tokens[:4]}))
	if len(r.Images) != 4 || r.Images[0].AssetID != assets[0] || r.Images[3].AssetID != assets[3] {
		t.Fatalf("四張圖依序存下：%+v", r.Images)
	}
	for _, a := range assets[:4] {
		if got := f.assetState(t, a); got != "ACTIVE" {
			t.Fatalf("提交後資產 state=%s，預期 ACTIVE", got)
		}
	}
	_, err = f.db.Conn(ctx).Exec(ctx, "INSERT INTO review.review_images (review_id, position, asset_id, object_key) VALUES ($1, 4, $2, 'y')", r.ID, uuid.NewV7())
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "review_images_position_check" {
		t.Fatalf("第 5 張圖要被 CHECK 擋下：%v", err)
	}
}

// RD9：訂單服務的暫時性故障是 Unavailable，不是「訂單不存在」；訂單真的不存在才是 NotFound。
func TestRD9OrderOutageIsUnavailableNotNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	in := review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5}

	f.orders.mu.Lock()
	f.orders.err = fmt.Errorf("order-service：connection refused")
	f.orders.mu.Unlock()
	_, err := f.module.Submit(ctx, in)
	if kindOf(err) != apperr.KindUnavailable || review.ReasonOf(err) != "" {
		t.Fatalf("故障要是 Unavailable：%v", err)
	}
	f.orders.mu.Lock()
	f.orders.err = nil
	f.orders.mu.Unlock()
	_, err = f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: "不存在", OrderItemID: it.itemID, Rating: 5})
	if kindOf(err) != apperr.KindNotFound || review.ReasonOf(err) != review.ReasonOrderNotFound {
		t.Fatalf("訂單不存在要是 NotFound：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.reviews"); n != 0 {
		t.Fatalf("失敗的提交不留評價：%d", n)
	}
}

// RD10：單筆讀取與列表同一個公開可讀述詞：隱藏的、作者刪帳中的、觀看者封鎖的都是 NotFound（不洩漏存在性）；作者的歷史不含隱藏的。
func TestRD10SingleReadUsesThePublicPredicate(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	author := f.newBuyer(t)
	viewer := f.newBuyer(t)
	r := f.submit(t, author, storeID, 700, 5, "好")
	if _, err := f.module.GetReview(ctx, r.ID, viewer.BuyerID); err != nil {
		t.Fatal(err)
	}

	must[bool](t)(f.buyers.BlockBuyer(ctx, viewer.BuyerID, author.BuyerID))
	if _, err := f.module.GetReview(ctx, r.ID, viewer.BuyerID); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("觀看者封鎖作者：%v", err)
	}
	if _, err := f.module.GetReview(ctx, r.ID, 0); err != nil {
		t.Fatalf("匿名照常可讀：%v", err)
	}

	hide(t, f, r.ID)
	if _, err := f.module.GetReview(ctx, r.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("隱藏的評價單筆讀取：%v", err)
	}
	if mine := must[review.Page](t)(f.module.ListMyReviews(ctx, author.BuyerID, 0, nil, 50)); len(mine.Reviews) != 0 {
		t.Fatalf("作者的歷史不含隱藏的（舊行為）：%d", len(mine.Reviews))
	}
	if _, err := f.module.GetReview(ctx, uuid.UUID{}, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}

	r2 := f.submit(t, f.newBuyer(t), storeID, 700, 1, "差")
	if err := f.buyers.BeginDeletion(ctx, r2.BuyerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.GetReview(ctx, r2.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("作者刪帳中：%v", err)
	}
	if st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); st.Count != 0 {
		t.Fatalf("刪帳中的作者不計分：%+v", st)
	}
}

func TestSubmitRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)

	it := f.order(b, storeID, 700)
	in := review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, Content: "  好喝  "}
	f.jobs.reset()
	r := must[review.Review](t)(f.module.Submit(ctx, in))
	if r.Content != "好喝" || len(f.created) != 1 || f.created[0].ReviewID != r.ID || f.created[0].ProductID != pid(700) || f.created[0].BuyerID != b.BuyerID || len(revalidations(f.jobs)) != 1 {
		t.Fatalf("提交：%+v events=%v jobs=%v", r, f.created, f.jobs.kinds())
	}
	if _, err := f.module.Submit(ctx, in); kindOf(err) != apperr.KindAlreadyExists || review.ReasonOf(err) != review.ReasonAlreadyExists {
		t.Fatalf("重複評價：%v", err)
	}
	if len(f.created) != 1 {
		t.Fatalf("失敗不發事件：%d", len(f.created))
	}

	other := f.newBuyer(t)
	it2 := f.order(b, storeID, 700)
	if _, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: other.BuyerID, OrderNo: it2.orderNo, OrderItemID: it2.itemID, Rating: 5}); kindOf(err) != apperr.KindNotFound || review.ReasonOf(err) != review.ReasonOrderNotFound {
		t.Fatalf("別人的訂單：%v", err)
	}
	if _, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it2.orderNo, OrderItemID: it2.itemID + 999, Rating: 5}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("項目不在訂單：%v", err)
	}
	f.orders.mu.Lock()
	o := f.orders.byNo[it2.orderNo]
	o.Status = "40"
	f.orders.byNo[it2.orderNo] = o
	f.orders.mu.Unlock()
	if _, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it2.orderNo, OrderItemID: it2.itemID, Rating: 5}); review.ReasonOf(err) != review.ReasonOrderNotReviewable {
		t.Fatalf("未完成：%v", err)
	}
	o.Status = "50"
	f.orders.mu.Lock()
	f.orders.byNo[it2.orderNo] = o
	f.orders.mu.Unlock()
	f.advance(30 * 24 * time.Hour)
	if _, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it2.orderNo, OrderItemID: it2.itemID, Rating: 5}); review.ReasonOf(err) != review.ReasonWindowExpired {
		t.Fatalf("超過期限：%v", err)
	}
	f.advance(-30 * 24 * time.Hour)

	if err := f.buyers.BeginDeletion(ctx, b.BuyerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it2.orderNo, OrderItemID: it2.itemID, Rating: 5}); kindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("刪帳中的買家不能再評價：%v", err)
	}
}

func TestReplyRules(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID, otherStore := f.exposableStore(t), f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 5, "好")
	f.runRatings(t)
	f.jobs.reset()
	f.ratings.reset()

	got := must[review.Review](t)(f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, storeID), ReviewID: r.ID, Content: " 謝謝 "}))
	_, queued := f.queueRow(t, pid(700))
	if got.Reply == nil || got.Reply.Content != "謝謝" || len(f.jobs.kinds()) != 0 || queued {
		t.Fatalf("回覆不登記評分重算、不排失效：%+v jobs=%v queued=%v", got.Reply, f.jobs.kinds(), queued)
	}
	got = must[review.Review](t)(f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, storeID), ReviewID: r.ID, Content: "再謝謝"}))
	if got.Reply.Content != "再謝謝" {
		t.Fatalf("可以覆蓋：%+v", got.Reply)
	}
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, otherStore), ReviewID: r.ID, Content: "x"}); kindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("不是這家店：%v", err)
	}
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, storeID), ReviewID: uuid.UUID{}, Content: "x"}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, storeID), ReviewID: r.ID, Content: "   "}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("空回覆：%v", err)
	}
	hide(t, f, r.ID)
	if _, err := f.module.Reply(ctx, review.ReplyInput{Store: f.store(t, storeID), ReviewID: r.ID, Content: "x"}); review.ReasonOf(err) != review.ReasonNotReplyable {
		t.Fatalf("已隱藏不可回覆：%v", err)
	}
}

func TestStatsAndStoreRatings(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	s1, s2, s3 := f.exposableStore(t), f.exposableStore(t), f.exposableStore(t)
	f.submit(t, f.newBuyer(t), s1, 700, 5, "")
	f.submit(t, f.newBuyer(t), s1, 700, 4, "")
	hidden := f.submit(t, f.newBuyer(t), s1, 701, 1, "")
	f.submit(t, f.newBuyer(t), s2, 702, 3, "")
	hide(t, f, hidden.ID)

	if p := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(700))); p.Count != 2 || p.Average != 4.5 {
		t.Fatalf("商品：%+v", p)
	}
	st := must[review.StoreStats](t)(f.module.StoreStats(ctx, s1))
	if st.Count != 2 || st.Average != 4.5 || st.Distribution != [5]int64{0, 0, 0, 1, 1} {
		t.Fatalf("店家：%+v", st)
	}
	got := must[map[int64]review.StoreRating](t)(f.module.StoreRatingStats(ctx, []int64{s1, s2, s3, s1}))
	if len(got) != 2 || got[s1].Count != 2 || got[s2].Average != 3 {
		t.Fatalf("批次（沒有評價的店不回、重複只算一次）：%+v", got)
	}
	for _, bad := range [][]int64{nil, {0}, {-1}, make([]int64, review.MaxStoreRatingStatsBatch+1)} {
		if _, err := f.module.StoreRatingStats(ctx, bad); kindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%v 要被擋：%v", bad, err)
		}
	}
	empty := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(999)))
	if empty.Count != 0 || empty.Average != 0 {
		t.Fatalf("沒有評價是 0：%+v", empty)
	}
}

func TestListFiltersAndKeysetAcrossTimestamps(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	oneToken, _ := f.stage(t, b, 1)
	withImg := must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 2, StagedTokens: oneToken}))
	f.advance(time.Minute)
	plain := f.submit(t, f.newBuyer(t), storeID, 700, 5, "")

	if got := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{WithImages: true}, nil, 50)); !sameIDs(ids(got), withImg.ID) {
		t.Fatalf("只看有圖：%v", ids(got))
	}
	if got := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{Rating: 5}, nil, 50)); !sameIDs(ids(got), plain.ID) {
		t.Fatalf("只看 5 星：%v", ids(got))
	}
	if _, err := f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{Rating: 6}, nil, 50); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("評分篩選 6：%v", err)
	}
	all := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(700), 0, review.Filter{}, nil, 0))
	if len(all.Reviews) != 2 || len(all.Reviews[1].Images) != 1 || all.Reviews[0].Images == nil {
		t.Fatalf("預設頁大小與附圖：%+v", all.Reviews)
	}
}

// 同一個 asset 不能在同一則評價出現兩次：服務層擋重複 token，DB 的 UNIQUE (review_id, asset_id) 是後備。
func TestReviewImageAssetIsUniquePerReview(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	tokens, _ := f.stage(t, b, 1)
	_, err := f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: []string{tokens[0], tokens[0]}})
	if kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("重複 token 要被擋：%v", err)
	}
	r := must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: tokens}))
	_, err = f.db.Conn(ctx).Exec(ctx, "INSERT INTO review.review_images (review_id, position, asset_id, object_key) VALUES ($1, 1, $2, 'y')", r.ID, r.Images[0].AssetID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "review_images_asset_key" {
		t.Fatalf("同一 asset 重複要被 UNIQUE 擋下：%v", err)
	}
}

// 回覆與隱藏並發：隱藏的 transaction 還沒 commit 時回覆要等它（先鎖評價列），commit 後看得到隱藏，回覆被拒而不是落在已隱藏的評價上。
func TestReplyWaitsForAConcurrentHide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 5, "好")
	st := f.store(t, storeID)

	replyErr := make(chan error, 1)
	err := f.db.InTx(ctx, func(txCtx context.Context) error {
		must[review.Hide](t)(f.module.Hide(txCtx, review.HideInput{ReviewID: r.ID, Category: "SPAM", Text: "廣告", AdminID: 1}))
		go func() {
			_, err := f.module.Reply(ctx, review.ReplyInput{Store: st, ReviewID: r.ID, Content: "謝謝"})
			replyErr <- err
		}()
		deadline := time.Now().Add(10 * time.Second)
		for f.scalar(t, "SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()") == 0 {
			if time.Now().After(deadline) {
				return errors.New("回覆沒有等隱藏的 transaction")
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-replyErr; review.ReasonOf(err) != review.ReasonNotReplyable {
		t.Fatalf("並發隱藏後回覆要被拒：%v", err)
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.reviews WHERE id = $1 AND reply_content IS NOT NULL", r.ID); n != 0 {
		t.Fatal("回覆不得落在已隱藏的評價上")
	}
}

// 時鐘倒退（多個實例的時差）時還原的 restored_at 不得早於 hidden_at（hides_restore_check）。
func TestRestoreNeverPrecedesHiddenAt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 5, "好")
	h := hide(t, f, r.ID)
	f.advance(-time.Hour)
	restored, err := f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "誤判"})
	if err != nil {
		t.Fatalf("時鐘倒退不得讓還原失敗：%v", err)
	}
	if restored.RestoredAt == nil || restored.RestoredAt.Before(h.HiddenAt) {
		t.Fatalf("restored_at %v 早於 hidden_at %v", restored.RestoredAt, h.HiddenAt)
	}
}

// Restore 帶來源參照：沒有未還原的隱藏（not-hidden 句子）與隱藏是別案造成的（source-mismatch 句子）是兩種不同的 FailedPrecondition，後者不動隱藏。
// 殺死的變異：Restore 忽略 SourceRef（來源不符仍還原）；兩種情況回同一句。
func TestRestoreSourceMismatchIsDistinctFromNotHidden(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 3, "普通")
	if _, err := f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "誤判", SourceRef: "case-1"}); kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "沒有被隱藏") {
		t.Fatalf("沒有隱藏：%v", err)
	}
	hide(t, f, r.ID)
	_, err := f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "誤判", SourceRef: "case-2"})
	if kindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "不是由這個案件造成") {
		t.Fatalf("來源不符：%v", err)
	}
	if _, err := f.module.GetReview(ctx, r.ID, 0); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("來源不符時隱藏仍在：%v", err)
	}
	if _, err := f.module.Restore(ctx, review.RestoreInput{ReviewID: r.ID, AdminID: 2, Reason: "誤判", SourceRef: "case-1"}); err != nil {
		t.Fatalf("來源相符可還原：%v", err)
	}
}

// ListMyReviews 帶 order_item_id 只回那個訂單項目的評價；不存在的項目回空；不帶時回全部。
func TestListMyReviewsFiltersByOrderItem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	author := f.newBuyer(t)
	r1 := f.submit(t, author, storeID, 700, 5, "好")
	r2 := f.submit(t, author, storeID, 701, 4, "不錯")

	one := must[review.Page](t)(f.module.ListMyReviews(ctx, author.BuyerID, r1.OrderItemID, nil, 50))
	if !sameIDs(ids(one), r1.ID) {
		t.Fatalf("帶 r1 的訂單項目只回 r1：%v", ids(one))
	}
	two := must[review.Page](t)(f.module.ListMyReviews(ctx, author.BuyerID, r2.OrderItemID, nil, 50))
	if !sameIDs(ids(two), r2.ID) {
		t.Fatalf("帶 r2 的訂單項目只回 r2：%v", ids(two))
	}
	if none := must[review.Page](t)(f.module.ListMyReviews(ctx, author.BuyerID, r2.OrderItemID+999999, nil, 50)); len(none.Reviews) != 0 {
		t.Fatalf("不存在的訂單項目回空：%v", ids(none))
	}
	if all := must[review.Page](t)(f.module.ListMyReviews(ctx, author.BuyerID, 0, nil, 50)); !sameIDs(ids(all), r2.ID, r1.ID) {
		t.Fatalf("不帶篩選回全部（新到舊）：%v", ids(all))
	}
}

// ReviewStore 不套公開可讀述詞：隱藏與作者刪帳中的評價仍回所屬的店（Public 為 false），不存在才是 NotFound。
func TestReviewStoreIgnoresVisibilityAndReportsIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	r := f.submit(t, f.newBuyer(t), storeID, 700, 5, "好")
	got := must[review.ReviewStore](t)(f.module.ReviewStore(ctx, r.ID))
	if got.StoreID != storeID || !got.Public {
		t.Fatalf("公開的評價：%+v", got)
	}
	hide(t, f, r.ID)
	got = must[review.ReviewStore](t)(f.module.ReviewStore(ctx, r.ID))
	if got.StoreID != storeID || got.Public {
		t.Fatalf("隱藏的評價仍回店家、Public=false：%+v", got)
	}
	r2 := f.submit(t, f.newBuyer(t), storeID, 700, 1, "差")
	if err := f.buyers.BeginDeletion(ctx, r2.BuyerID); err != nil {
		t.Fatal(err)
	}
	got = must[review.ReviewStore](t)(f.module.ReviewStore(ctx, r2.ID))
	if got.StoreID != storeID || got.Public {
		t.Fatalf("作者刪帳中的評價仍回店家、Public=false：%+v", got)
	}
	if _, err := f.module.ReviewStore(ctx, uuid.UUID{}); kindOf(err) != apperr.KindNotFound {
		t.Fatalf("不存在：%v", err)
	}
}

// 商品統計與公開列表同一個述詞：隱藏、作者刪帳中的評價不計，其他商品的評價不混入，沒有評價是 0。
func TestProductStatsExcludeWhatThePublicListExcludes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	store := f.exposableStore(t)
	f.submit(t, f.newBuyer(t), store, 710, 5, "")
	f.submit(t, f.newBuyer(t), store, 710, 4, "")
	hidden := f.submit(t, f.newBuyer(t), store, 710, 1, "")
	leaving := f.submit(t, f.newBuyer(t), store, 710, 2, "")
	f.submit(t, f.newBuyer(t), store, 711, 1, "")
	hide(t, f, hidden.ID)
	if err := f.buyers.BeginDeletion(ctx, leaving.BuyerID); err != nil {
		t.Fatal(err)
	}

	st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(710)))
	if st.Count != 2 || st.Average != 4.5 || st.Distribution != [5]int64{0, 0, 0, 1, 1} {
		t.Fatalf("只計公開可讀的兩則（分佈同一個述詞）：%+v", st)
	}
	page := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(710), 0, review.Filter{}, nil, 50))
	if int64(len(page.Reviews)) != st.Count {
		t.Fatalf("統計與公開列表不一致：%d vs %d", len(page.Reviews), st.Count)
	}
	if none := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(712))); none.Count != 0 || none.Average != 0 || none.Distribution != [5]int64{} {
		t.Fatalf("沒有評價是 0：%+v", none)
	}
}

// 商品統計的附圖筆數與公開列表的 with_images 篩選同一個條件：隱藏、作者刪帳中的附圖評價不計，其他商品的不混入，沒有附圖是 0。
func TestProductStatsWithImagesCountMatchesTheWithImagesFilter(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	store := f.exposableStore(t)
	submitImg := func(product int) review.Review {
		b := f.newBuyer(t)
		it := f.order(b, store, product)
		tokens, _ := f.stage(t, b, 1)
		return must[review.Review](t)(f.module.Submit(ctx, review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 4, StagedTokens: tokens}))
	}
	submitImg(730)
	submitImg(730)
	hidden := submitImg(730)
	leaving := submitImg(730)
	submitImg(731)
	f.submit(t, f.newBuyer(t), store, 730, 5, "")
	f.submit(t, f.newBuyer(t), store, 732, 5, "")
	hide(t, f, hidden.ID)
	if err := f.buyers.BeginDeletion(ctx, leaving.BuyerID); err != nil {
		t.Fatal(err)
	}

	st := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(730)))
	if st.Count != 3 || st.WithImagesCount != 2 {
		t.Fatalf("公開可讀 3 則、其中附圖 2 則：%+v", st)
	}
	page := must[review.Page](t)(f.module.ListProductReviews(ctx, pid(730), 0, review.Filter{WithImages: true}, nil, 50))
	if int64(len(page.Reviews)) != st.WithImagesCount {
		t.Fatalf("附圖筆數與 with_images 列表不一致：%d vs %d", len(page.Reviews), st.WithImagesCount)
	}
	if none := must[review.ProductStats](t)(f.module.ProductStats(ctx, pid(732))); none.Count != 1 || none.WithImagesCount != 0 {
		t.Fatalf("沒有附圖是 0：%+v", none)
	}
}

// 回滾：提交的 transaction 在 Confirm 之後失敗，沒有評價、附圖仍是暫存，同一組 token 重送成功。
func TestRolledBackSubmitLeavesTheImagesStaged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	storeID := f.exposableStore(t)
	b := f.newBuyer(t)
	it := f.order(b, storeID, 700)
	tokens, assets := f.stage(t, b, 2)
	fail := true
	events.Subscribe(f.bus, func(context.Context, review.ReviewCreated) error {
		if fail {
			return errors.New("boom")
		}
		return nil
	})
	in := review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: tokens}
	if _, err := f.module.Submit(ctx, in); err == nil {
		t.Fatal("訂閱者失敗應讓提交回滾")
	}
	for _, a := range assets {
		if got := f.assetState(t, a); got != "STAGED" {
			t.Fatalf("回滾後資產 state=%s，預期 STAGED", got)
		}
	}
	if n := f.scalar(t, "SELECT count(*) FROM review.reviews"); n != 0 {
		t.Fatalf("回滾後不得有評價：%d", n)
	}
	fail = false
	if _, err := f.module.Submit(ctx, in); err != nil {
		t.Fatalf("回滾後同一組 token 重送應成功：%v", err)
	}
}

// 別人的 token 不能確認：回錯誤，沒有評價，資產維持暫存。
func TestSubmitRefusesATokenStagedByAnotherAccount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	storeID := f.exposableStore(t)
	b, other := f.newBuyer(t), f.newBuyer(t)
	it := f.order(b, storeID, 700)
	tokens, assets := f.stage(t, other, 1)
	if _, err := f.module.Submit(context.Background(), review.SubmitInput{BuyerID: b.BuyerID, OrderNo: it.orderNo, OrderItemID: it.itemID, Rating: 5, StagedTokens: tokens}); err == nil {
		t.Fatal("別的帳號的 token 應被拒絕")
	}
	if got := f.assetState(t, assets[0]); got != "STAGED" {
		t.Fatalf("state=%s，預期 STAGED", got)
	}
}
