//go:build integration

package moderation

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// listWorld 開三個案件：貼文 A（進行中）、貼文 B（已結案，不違規）、留言 C（進行中）；每個案件的開案時間錯開（注入的時鐘）。
type listWorld struct {
	postOpen, postClosed, commentOpen Case
}

func (f *fixture) listWorld(t *testing.T) listWorld {
	t.Helper()
	w := f.world(t)
	reporter, _ := f.newBuyer(t)
	second := f.post(t, w.storeID, "第二篇")
	var lw listWorld
	f.advance(time.Minute)
	lw.postClosed = f.submit(t, reporter, Target{Kind: TargetPost, ContentID: second.ID}, policy.Spam).Case
	f.advance(time.Minute)
	lw.postOpen = f.submit(t, reporter, Target{Kind: TargetPost, ContentID: w.post.ID}, policy.Spam).Case
	f.advance(time.Minute)
	lw.commentOpen = f.submit(t, reporter, commentTarget(w.comment), policy.Spam).Case
	must(t, errOf(f.module.DecideCase(context.Background(), DecideInput{
		CaseID: lw.postClosed.ID, Version: lw.postClosed.Version, AdminID: 9, Procedure: "/test/Decide", Decision: DecisionNoViolation,
	})))
	return lw
}

func ids(p CasePage) []uuid.UUID {
	out := make([]uuid.UUID, len(p.Cases))
	for i, c := range p.Cases {
		out[i] = c.ID
	}
	return out
}

func sameIDs(got []uuid.UUID, want ...uuid.UUID) bool { return slices.Equal(got, want) }

// 沒有篩選列出全部，開案時間新的在前；狀態與對象種類各自篩選，也可以組合。
func TestCasesFilterByStateAndTargetKind(t *testing.T) {
	t.Parallel()
	f := setup(t)
	lw := f.listWorld(t)
	ctx := context.Background()
	list := func(fl CaseFilter) CasePage {
		return mustV[CasePage](t)(f.module.Cases(ctx, fl, nil, 0))
	}

	if got := ids(list(CaseFilter{})); !sameIDs(got, lw.commentOpen.ID, lw.postOpen.ID, lw.postClosed.ID) {
		t.Fatalf("全部（新到舊）= %v", got)
	}
	if got := ids(list(CaseFilter{State: CaseOpen})); !sameIDs(got, lw.commentOpen.ID, lw.postOpen.ID) {
		t.Fatalf("進行中 = %v", got)
	}
	closed := list(CaseFilter{State: CaseClosed})
	if got := ids(closed); !sameIDs(got, lw.postClosed.ID) || closed.Cases[0].ClosedAt == nil || closed.Cases[0].Decision == nil {
		t.Fatalf("已結案 = %+v", closed.Cases)
	}
	if got := ids(list(CaseFilter{TargetKind: TargetPost})); !sameIDs(got, lw.postOpen.ID, lw.postClosed.ID) {
		t.Fatalf("貼文 = %v", got)
	}
	if got := ids(list(CaseFilter{TargetKind: TargetComment})); !sameIDs(got, lw.commentOpen.ID) {
		t.Fatalf("留言 = %v", got)
	}
	if got := ids(list(CaseFilter{State: CaseOpen, TargetKind: TargetPost})); !sameIDs(got, lw.postOpen.ID) {
		t.Fatalf("進行中的貼文 = %v", got)
	}
	if got := ids(list(CaseFilter{State: CaseClosed, TargetKind: TargetComment})); len(got) != 0 {
		t.Fatalf("已結案的留言應該沒有：%v", got)
	}
	if got := ids(list(CaseFilter{TargetKind: TargetBuyer})); len(got) != 0 {
		t.Fatalf("買家案件應該沒有：%v", got)
	}
}

// 跨頁不重不漏：一頁一筆走完，順序與一次讀全部相同；最後一頁沒有下一頁的游標。
func TestCasesKeysetPagingHasNoDuplicatesOrGaps(t *testing.T) {
	t.Parallel()
	f := setup(t)
	lw := f.listWorld(t)
	ctx := context.Background()
	var got []uuid.UUID
	var after *CaseCursor
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("分頁沒有結束")
		}
		p := mustV[CasePage](t)(f.module.Cases(ctx, CaseFilter{}, after, 1))
		got = append(got, ids(p)...)
		if p.Next == nil {
			break
		}
		if len(p.Cases) != 1 {
			t.Fatalf("有下一頁時這頁要滿：%d", len(p.Cases))
		}
		after = p.Next
	}
	if !sameIDs(got, lw.commentOpen.ID, lw.postOpen.ID, lw.postClosed.ID) {
		t.Fatalf("分頁走完 = %v", got)
	}
}

// 開案時間相同時依案件 ID 由大到小，翻頁仍然不重不漏。
func TestCasesWithTheSameOpenTimePageByID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	lw := f.listWorld(t)
	f.exec(t, "UPDATE moderation.cases SET opened_at = $1", f.clock())
	all := mustV[CasePage](t)(f.module.Cases(context.Background(), CaseFilter{}, nil, 0))
	if len(all.Cases) != 3 {
		t.Fatalf("cases = %d", len(all.Cases))
	}
	for i := 1; i < len(all.Cases); i++ {
		if all.Cases[i-1].ID.String() < all.Cases[i].ID.String() {
			t.Fatalf("同時間要依 ID 由大到小：%v", ids(all))
		}
	}
	var got []uuid.UUID
	var after *CaseCursor
	for range 3 {
		p := mustV[CasePage](t)(f.module.Cases(context.Background(), CaseFilter{}, after, 1))
		got = append(got, ids(p)...)
		after = p.Next
	}
	if !sameIDs(got, ids(all)...) || after != nil {
		t.Fatalf("同時間分頁 = %v，全部 = %v，after=%v（lw=%v）", got, ids(all), after, lw.postOpen.ID)
	}
}

func TestCasesRejectsInvalidFilterAndPageSize(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, fl := range map[string]CaseFilter{
		"狀態不合法":   {State: "REOPENED"},
		"對象種類不合法": {TargetKind: "CHAT"},
	} {
		if _, err := f.module.Cases(ctx, fl, nil, 0); kindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err = %v", name, err)
		}
	}
	if _, err := f.module.Cases(ctx, CaseFilter{}, nil, -1); kindOf(err) != apperr.KindInvalidInput {
		t.Errorf("負的 page size：err = %v", err)
	}
	// 超過上限以上限計，不是錯誤。
	if _, err := f.module.Cases(ctx, CaseFilter{}, nil, 10_000); err != nil {
		t.Errorf("超過上限：err = %v", err)
	}
}

// 決定與後台動作紀錄在同一個 transaction：成功寫一筆（動作是 Procedure、對象是案件），失敗（版本衝突）不留紀錄。
func TestDecideWritesTheAdminAuditRecordInTheSameTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := f.world(t)
	c := f.openCase(t, commentTarget(w.comment))
	const audit = "SELECT count(*) FROM adminaudit.records WHERE action = '/test/Decide' AND target_type = 'moderation_case' AND target_id = $1 AND admin_id = 9"

	f.bumpVersion(t, c.ID)
	if _, err := f.decide(t, c, ActionHide, policy.Spam); kindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("過期版本：err = %v", err)
	}
	if n := f.count(t, audit, c.ID.String()); n != 0 {
		t.Fatalf("衝突回滾後不該有紀錄：%d", n)
	}
	cur := mustV[CaseDetail](t)(f.module.Case(context.Background(), c.ID))
	mustV[CaseDetail](t)(f.decide(t, cur.Case, ActionHide, policy.Spam))
	if n := f.count(t, audit, c.ID.String()); n != 1 {
		t.Fatalf("結案要留一筆紀錄：%d", n)
	}
	if _, err := f.module.DecideCase(context.Background(), DecideInput{CaseID: c.ID, Version: 1, AdminID: 9, Decision: DecisionNoViolation}); kindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("沒有 Procedure：err = %v", err)
	}
}
