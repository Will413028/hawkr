package visibility

import (
	"strings"
	"testing"
	"time"

	. "github.com/go-jet/jet/v2/postgres"
)

var at = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func render(t *testing.T, cond BoolExpression, from ReadableTable) (string, []any) {
	t.Helper()
	q, args := SELECT(STAR).FROM(from).WHERE(cond).Sql()
	return q, args
}

// 固定代碼是 SQL 常數：以 `$1` 傳 'PUBLISHED' 時 generic plan 證明不了 partial index 的述詞（量測 59 ms 對 0.7 ms）。
func TestPostPublishedUsesInlineConstants(t *testing.T) {
	src := ReadablePosts("p", at, Anonymous)
	q, args := render(t, src.Where(), src.T)
	if !strings.Contains(q, "p.status = 'PUBLISHED'") || !strings.Contains(q, "p.deleted_at IS NULL") {
		t.Errorf("PostPublished 要是常數述詞：%s", q)
	}
	for _, a := range args {
		if s, ok := a.(string); ok {
			t.Errorf("不該有字串參數：%q", s)
		}
	}
}

func TestReadablePostsRequireTheStoreToBeExposableAtTheRequestClock(t *testing.T) {
	src := ReadablePosts("p", at, Anonymous)
	q, args := render(t, src.Where(), src.T)
	if !strings.Contains(q, "seller.pub_exposable_stores($1::timestamp with time zone) AS pub WHERE pub.store_id = p.store_id") {
		t.Errorf("店家可曝光的條件不對：%s", q)
	}
	if len(args) != 1 || args[0] != at {
		t.Errorf("時間要是請求的 clock：%v", args)
	}
}

func TestOwnerPostsOnlyRequireOwnershipAndNotDeleted(t *testing.T) {
	src := OwnerPosts("p", 7)
	q, _ := render(t, src.Where(), src.T)
	if strings.Contains(q, "pub_exposable_stores") || strings.Contains(q, "PUBLISHED") {
		t.Errorf("店主讀自己的貼文不要求公開：%s", q)
	}
	if !strings.Contains(q, "p.store_id =") || !strings.Contains(q, "p.deleted_at IS NULL") {
		t.Errorf("店主的貼文要限定店家且沒有刪除：%s", q)
	}
}

// 計入只看儲存的旗標：不重新推導隱藏、刪除或作者狀態（那些由 reevaluate_* 維護旗標，讀取與重算直接用旗標）；觀看者不影響，封鎖不併進 Counted。
func TestCountedIsTheStoredFlagAndNothingElse(t *testing.T) {
	for name, c := range map[string]Comments{"匿名": CommentsOf("c", Anonymous), "有觀看者": CommentsOf("c", Viewer{BuyerID: 42})} {
		q, _ := render(t, c.Counted(), c.T)
		if !strings.Contains(q, "c.counted") {
			t.Errorf("%s：Counted 要看旗標：%s", name, q)
		}
		for _, banned := range []string{"social.hides", "inactive_authors", "pub_inactive_buyers", "c.deleted_at", "pub_blocked_"} {
			if strings.Contains(q, banned) {
				t.Errorf("%s：Counted 不得重新推導或帶封鎖（%s）：%s", name, banned, q)
			}
		}
	}
	e := ForumEntriesOf("e", Viewer{BuyerID: 42})
	q, _ := render(t, e.Counted(), e.T)
	if !strings.Contains(q, "e.counted") || strings.Contains(q, "social.hides") || strings.Contains(q, "pub_blocked_") {
		t.Errorf("討論區留言的 Counted 也只看旗標：%s", q)
	}
}

// 匿名讀取的語句文字不含任何封鎖子句（不用 `$v IS NULL OR …` 的開關）；有觀看者的語句才多出封鎖。
func TestAnonymousReadsCarryNoBlockClause(t *testing.T) {
	p, c, e := ReadablePosts("p", at, Anonymous), CommentsOf("c", Anonymous), ForumEntriesOf("e", Anonymous)
	for name, q := range map[string]string{
		"貼文": rendered(t, p.Where(), p.T), "留言": rendered(t, c.Where(), c.T), "討論區留言": rendered(t, e.Where(), e.T),
		"討論區": rendered(t, ReadableForum(at, Anonymous, Int64(7)), nil),
	} {
		if strings.Contains(q, "pub_blocked_") || strings.Contains(q, "IS NULL OR") {
			t.Errorf("%s：匿名讀取不得有封鎖子句：%s", name, q)
		}
	}
	if c.Admits(String("x")) != nil || e.Admits(String("x")) != nil {
		t.Error("匿名沒有被排除的列，Admits 要是 nil")
	}
}

// 觀看者封鎖的店：貼文與討論區查 pub_blocked_stores()；留言作者是買家時雙向查 pub_blocked_buyers()。兩種對象是兩個函式，不靠 kind 欄位區分。
func TestViewerReadsCarryTheBlockPredicates(t *testing.T) {
	v := Viewer{BuyerID: 42}
	p, c := ReadablePosts("p", at, v), CommentsOf("c", v)
	pq := rendered(t, p.Where(), p.T)
	if !strings.Contains(pq, "buyer.pub_blocked_stores()") || strings.Contains(pq, "pub_blocked_buyers") || !strings.Contains(pq, "NOT") {
		t.Errorf("貼文要排除觀看者封鎖的店：%s", pq)
	}
	cq := rendered(t, c.Where(), c.T)
	if strings.Count(cq, "buyer.pub_blocked_buyers()") != 2 || strings.Contains(cq, "pub_blocked_stores") || !strings.Contains(cq, "c.actor_kind != 'BUYER'") {
		t.Errorf("留言要雙向排除與觀看者有封鎖的買家作者：%s", cq)
	}
	if strings.Contains(cq, "'STORE'") {
		t.Errorf("店家作者不在留言入口排除（容器條件擋下整家店）：%s", cq)
	}
	fq := rendered(t, ReadableForum(at, v, Int64(7)), nil)
	if !strings.Contains(fq, "buyer.pub_blocked_stores()") || !strings.Contains(fq, "pub_exposable_stores") {
		t.Errorf("討論區要店家可曝光而且沒被觀看者封鎖：%s", fq)
	}
	if c.Admits(String("x")) == nil {
		t.Error("有觀看者時 Admits 要檢查父列")
	}
	for _, q := range []string{pq, cq, fq} {
		if !strings.Contains(q, "$") || strings.Contains(q, "IS NULL OR") {
			t.Errorf("觀看者是 bind 參數，沒有開關：%s", q)
		}
	}
}

func rendered(t *testing.T, cond BoolExpression, from ReadableTable) string {
	t.Helper()
	if from == nil {
		q, _ := SELECT(cond).Sql()
		return q
	}
	q, _ := render(t, cond, from)
	return q
}

// 佔位項目的 state 才讀 inactive_authors（只對輸出的頁內的列求值），作者種類的條件在子查詢裡。
func TestGoneReadsTheLocalCopyOfTheBuyerFact(t *testing.T) {
	c := CommentsOf("c", Anonymous)
	q, _ := render(t, c.Gone(), c.T)
	if !strings.Contains(q, "c.deleted_at IS NOT NULL") || !strings.Contains(q, "social.inactive_authors AS ia") ||
		!strings.Contains(q, "(ia.buyer_id = c.actor_id) AND (c.actor_kind = 'BUYER')") {
		t.Errorf("Gone 的條件不對：%s", q)
	}
	if strings.Contains(q, "buyer.pub_") {
		t.Errorf("social 不再讀 buyer 的 pub：%s", q)
	}
}
