//go:build integration

package procurement

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/rfqhash"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func wantKind(t *testing.T, what string, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Errorf("%s = %v，要 %v", what, err, kind)
	}
}

func wantField(t *testing.T, what string, err error, field string) {
	t.Helper()
	wantKind(t, what, err, apperr.KindInvalidInput)
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Errorf("%s：不是 apperr：%v", what, err)
		return
	}
	var got []string
	for _, v := range ae.FieldViolations() {
		got = append(got, v.Field)
	}
	if !slices.Equal(got, []string{field}) {
		t.Errorf("%s 的欄位 = %v，要 [%s]", what, got, field)
	}
}

// newConversation 為買家寫一個需求與它的對話，updated_at 是 at。
func newConversation(ctx context.Context, t *testing.T, d *db.DB, buyerID int64, at time.Time) uuid.UUID {
	t.Helper()
	q := store.New(d.Conn(ctx))
	req, conv := id(), id()
	if err := q.InsertRequest(ctx, store.InsertRequestParams{ID: req, BuyerID: &buyerID, Now: at}); err != nil {
		t.Fatal(err)
	}
	if err := q.InsertConversation(ctx, store.InsertConversationParams{ID: conv, BuyerID: buyerID, RequestID: req, Now: at}); err != nil {
		t.Fatal(err)
	}
	return conv
}

func TestReadMethodsAreNotFoundForAnotherBuyerOrUnknownIDs(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	const other = int64(8)
	_, err := m.GetSnapshot(ctx, other, c.request)
	wantKind(t, "GetSnapshot 別人的", err, apperr.KindNotFound)
	_, err = m.GetSnapshot(ctx, 7, id())
	wantKind(t, "GetSnapshot 不存在", err, apperr.KindNotFound)
	_, err = m.GetConversationMessages(ctx, other, c.conversation)
	wantKind(t, "GetConversationMessages 別人的", err, apperr.KindNotFound)
	wantKind(t, "RenameConversation 別人的", m.RenameConversation(ctx, other, c.conversation, "新名字"), apperr.KindNotFound)
	wantKind(t, "DeleteConversation 別人的", m.DeleteConversation(ctx, other, c.conversation), apperr.KindNotFound)
	wantKind(t, "DeleteConversation 不存在", m.DeleteConversation(ctx, 7, id()), apperr.KindNotFound)
	_, err = m.GetCandidateSearch(ctx, other, c.request, c.run, uitext.ZhTW)
	wantKind(t, "GetCandidateSearch 別人的", err, apperr.KindNotFound)
	if n := count(ctx, t, d, "conversations", "id = $1 AND deleted_at IS NULL", c.conversation); n != 1 {
		t.Errorf("別人的刪除不得生效：未刪除的對話 %d 筆", n)
	}
	if n := count(ctx, t, d, "agent_runs", "id = $1 AND status = 'QUEUED'", c.run); n != 1 {
		t.Errorf("別人的刪除不得取消 run")
	}
}

func TestGetSnapshotIsKeyedByRequestAndRecomputesTheHash(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, true)
	irq := ask(ctx, t, d, c)
	s, err := m.GetSnapshot(ctx, 7, c.request)
	if err != nil {
		t.Fatal(err)
	}
	if s.RequestID != c.request || s.RevisionID != c.revision || s.Status != "CLARIFYING" {
		t.Errorf("識別與狀態：%+v", s)
	}
	if s.Slots.Item == nil || *s.Slots.Item != "外套" || s.Slots.Quantity == nil || *s.Slots.Quantity != 3 || s.Slots.BudgetPerUnit == nil || *s.Slots.BudgetPerUnit != 500 {
		t.Errorf("欄位：%+v", s.Slots)
	}
	if s.ActiveRunID != nil {
		t.Errorf("等待輸入的 run 不是進行中：%v", s.ActiveRunID)
	}
	if s.MissingRequired == nil || len(s.MissingRequired) != 0 {
		t.Errorf("必填都填了，MissingRequired 要是空的：%v", s.MissingRequired)
	}
	if s.OpenConfirmation == nil || s.OpenConfirmation.InterruptID != irq || !slices.Equal(s.ItemDisambiguation, []string{"蘋果"}) {
		t.Errorf("待回答的中斷與消歧卡：%+v %v", s.OpenConfirmation, s.ItemDisambiguation)
	}
	if len(s.Runs) != 1 || s.Runs[0].RunID != c.run {
		t.Errorf("runs：%+v", s.Runs)
	}
	want, err := rfqhash.Snapshot(rfqhash.Input{
		SchemaVersion: rfqhash.SchemaV1, ConfirmedRfqSnapshotID: c.snapshot.String(), RequestRevisionID: c.revision.String(), PricePreviewID: c.preview.String(), BuyerNote: "請盡快",
		Requirement: rfqhash.Slots{Item: ptr("外套"), Quantity: ptr(int32(3)), Unit: ptr("件"), BudgetPerUnit: ptr(int64(500)), DeliveryCity: ptr("台北市"),
			DeliveryDate: ptr(epoch.Format(time.DateOnly)), ExactMatchRequired: ptr(false)},
		Price: rfqhash.Price{RangeLowerAmount: 400, RangeUpperAmount: 600, BasisQuantity: 1, BasisUnit: "件",
			TotalLowerAmount: ptr(int64(1200)), TotalUpperAmount: ptr(int64(1800)), TotalQuantity: ptr(int32(3)), TotalUnit: ptr("件")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := s.PublishedRfq
	if p == nil || p.PublicationID != c.publication || p.RfqSnapshotHash != want {
		t.Errorf("發布結果與重算的 hash：%+v，要 hash %s", p, want)
	}
	rt, err := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if err != nil || rt.PublishedRfq == nil || rt.PublishedRfq.RfqSnapshotHash != want {
		t.Errorf("GetRuntimeSnapshot 的 hash 要與 GetSnapshot 相同：%+v %v", rt.PublishedRfq, err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.conversations SET deleted_at = $2 WHERE id = $1`, c.conversation, epoch); err != nil {
		t.Fatal(err)
	}
	_, err = m.GetSnapshot(ctx, 7, c.request)
	wantKind(t, "對話已刪除的需求", err, apperr.KindNotFound)
}

// partialRequest 寫一個只有品項的需求與它的對話，必填的城市與單位還缺。
func partialRequest(ctx context.Context, t *testing.T, d *db.DB, buyerID int64) uuid.UUID {
	t.Helper()
	q := store.New(d.Conn(ctx))
	req := id()
	if err := q.InsertRequest(ctx, store.InsertRequestParams{ID: req, BuyerID: &buyerID, Now: epoch}); err != nil {
		t.Fatal(err)
	}
	if n, err := q.InsertRevision(ctx, store.InsertRevisionParams{ID: id(), RequestID: req, BuyerID: &buyerID, RevisionNo: 1, Item: ptr("外套"), Now: epoch}); err != nil || n != 1 {
		t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	if err := q.InsertConversation(ctx, store.InsertConversationParams{ID: id(), BuyerID: buyerID, RequestID: req, Now: epoch}); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestGetSnapshotListsTheMissingRequiredSlotsAndHasNoPublicationOrInterruptYet(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	req := partialRequest(ctx, t, d, 7)
	s, err := m.GetSnapshot(ctx, 7, req)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.MissingRequired, []model.SlotKey{"DELIVERY_CITY", "UNIT"}) {
		t.Errorf("MissingRequired = %v，要 [DELIVERY_CITY UNIT]", s.MissingRequired)
	}
	if s.PublishedRfq != nil {
		t.Errorf("還沒發布，PublishedRfq 要是 nil：%+v", s.PublishedRfq)
	}
	if s.OpenConfirmation != nil || len(s.ItemDisambiguation) != 0 || s.ActiveRunID != nil || len(s.Runs) != 0 {
		t.Errorf("還沒有中斷與 run：%+v", s)
	}
}

const quantityCard = `{"stage":"","query":"","candidates":[{"categoryId":"21","label":"蘋果","path":"水果 > 蘋果"}],` +
	`"question":{"slotKey":"SLOT_KEY_QUANTITY","prompt":"要幾件？","allowFreeText":true}}`

func TestGetSnapshotOpenInterruptOutsideTheItemCardHasNoDisambiguation(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'WAITING_FOR_INPUT' WHERE id = $1`, c.run); err != nil {
		t.Fatal(err)
	}
	irq := id()
	if err := store.New(d.Conn(ctx)).InsertInterrupt(ctx, store.InsertInterruptParams{ID: irq, RunID: c.run, Payload: []byte(quantityCard), Now: epoch}); err != nil {
		t.Fatal(err)
	}
	s, err := m.GetSnapshot(ctx, 7, c.request)
	if err != nil {
		t.Fatal(err)
	}
	if s.OpenConfirmation == nil || s.OpenConfirmation.InterruptID != irq || s.OpenConfirmation.Question.SlotKey != "QUANTITY" {
		t.Errorf("待回答的中斷：%+v", s.OpenConfirmation)
	}
	if len(s.ItemDisambiguation) != 0 {
		t.Errorf("不是品項卡就沒有消歧：%v", s.ItemDisambiguation)
	}
	if s.PublishedRfq != nil {
		t.Errorf("seed 沒發布：%+v", s.PublishedRfq)
	}
}

func TestGetConversationMessagesMatchesTheRuntimeSnapshot(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	got, err := m.GetConversationMessages(ctx, 7, c.conversation)
	if err != nil {
		t.Fatal(err)
	}
	rt, _ := m.GetRuntimeSnapshot(ctx, 7, c.conversation)
	if got.ConversationID != c.conversation || got.RequestID != c.request || got.Status != "CLARIFYING" || len(got.Messages) != 2 ||
		got.Messages[0].Role != "USER" || got.Messages[1].Role != "ASSISTANT" || got.Messages[1].ID != c.turn || len(rt.Messages) != len(got.Messages) {
		t.Errorf("訊息：%+v", got)
	}
}

func TestListConversationsPageSizeRules(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	for i := range 55 {
		newConversation(ctx, t, d, 7, epoch.Add(time.Duration(i)*time.Second))
	}
	other := newConversation(ctx, t, d, 8, epoch.Add(time.Hour))
	for size, want := range map[int32]int{0: 20, 1: 1, 51: 50, 50: 50, 1000: 50} {
		p, err := m.ListConversations(ctx, 7, size, "")
		if err != nil || len(p.Conversations) != want {
			t.Errorf("page_size %d：%d 筆 %v，要 %d", size, len(p.Conversations), err, want)
		}
		for _, c := range p.Conversations {
			if c.ID == other {
				t.Errorf("別的買家的對話出現在列表")
			}
		}
	}
	_, err := m.ListConversations(ctx, 7, -1, "")
	wantField(t, "page_size -1", err, "page_size")
	for _, bad := range []string{"%%%", "bm90LWpzb24", "e30", "e3siaSI6IiJ9"} {
		_, err = m.ListConversations(ctx, 7, 10, bad)
		wantField(t, "壞 token "+bad, err, "page_token")
	}
}

func TestListConversationsTokenPagingHasNoGapsOrDuplicatesEvenWithEqualUpdatedAt(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	var all []uuid.UUID
	same := epoch.Add(time.Minute)
	for i := range 9 {
		at := same // 一半的列有相同的 updated_at，翻頁邊界落在同時間戳之間
		if i%2 == 0 {
			at = epoch.Add(time.Duration(i) * time.Second)
		}
		all = append(all, newConversation(ctx, t, d, 7, at))
	}
	gone := newConversation(ctx, t, d, 7, same)
	newConversation(ctx, t, d, 8, same) // 別的買家、相同的 updated_at：翻頁不得帶出
	if err := m.DeleteConversation(ctx, 7, gone); err != nil {
		t.Fatal(err)
	}
	var seen []uuid.UUID
	var last time.Time
	token := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("翻頁沒有結束")
		}
		p, err := m.ListConversations(ctx, 7, 2, token)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range p.Conversations {
			if len(seen) > 0 && c.UpdateTime.After(last) {
				t.Errorf("順序：%v 在 %v 之後", c.UpdateTime, last)
			}
			last = c.UpdateTime
			seen = append(seen, c.ID)
		}
		if token = p.NextPageToken; token == "" {
			break
		}
		if len(p.Conversations) != 2 {
			t.Errorf("非最後一頁要滿頁：%d", len(p.Conversations))
		}
	}
	if len(seen) != len(all) {
		t.Fatalf("共 %d 筆，要 %d（不漏）", len(seen), len(all))
	}
	byString := func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) }
	slices.SortFunc(all, byString)
	got := slices.Clone(seen)
	slices.SortFunc(got, byString)
	if !slices.Equal(all, got) {
		t.Errorf("翻出的對話集合與寫入的不同（漏或重）：%v vs %v", got, all)
	}
	for _, s := range seen {
		if s == gone {
			t.Errorf("已刪除的對話不得列出")
		}
	}
}

func TestRenameConversationTitleRules(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	conv := newConversation(ctx, t, d, 7, epoch)
	title := func() (string, string) {
		var ti, src string
		if err := d.Pool().QueryRow(ctx, `SELECT title, title_source FROM procurement.conversations WHERE id = $1`, conv).Scan(&ti, &src); err != nil {
			t.Fatal(err)
		}
		return ti, src
	}
	ok40 := strings.Repeat("字", 40)
	if err := m.RenameConversation(ctx, 7, conv, ok40); err != nil {
		t.Errorf("40 字：%v", err)
	}
	if ti, src := title(); ti != ok40 || src != "USER" {
		t.Errorf("標題 = %q（%s）", ti, src)
	}
	for name, bad := range map[string]string{"41 字": strings.Repeat("字", 41), "換行": "a\nb", "歸位": "a\rb", "空白": "   ", "空": ""} {
		wantField(t, name, m.RenameConversation(ctx, 7, conv, bad), "title")
	}
	if ti, _ := title(); ti != ok40 {
		t.Errorf("被拒的改名不得改動標題：%q", ti)
	}
	if err := m.RenameConversation(ctx, 7, conv, "  新名字  "); err != nil {
		t.Fatal(err)
	}
	if ti, _ := title(); ti != "新名字" {
		t.Errorf("頭尾空白要去掉：%q", ti)
	}
	if err := m.RenameConversation(ctx, 7, conv, "新名字"); err != nil {
		t.Errorf("重送同一個標題：%v", err)
	}
}

func TestDatabaseRejectsATitleOver40RunesAndKeepsTheWhitespaceCheck(t *testing.T) {
	ctx := context.Background()
	_, d := runtimeModule(t)
	conv := newConversation(ctx, t, d, 7, epoch)
	var def string
	if err := d.Pool().QueryRow(ctx, `SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'procurement' AND t.relname = 'conversations' AND c.conname = 'conversations_title_check'`).Scan(&def); err != nil {
		t.Fatalf("讀不到 conversations_title_check：%v", err)
	}
	if !strings.Contains(def, "<= 40") || strings.Contains(def, "<= 200") {
		t.Errorf("約束定義沒有收緊到 40：%s", def)
	}
	set := func(title string) error {
		_, err := d.Pool().Exec(ctx, `UPDATE procurement.conversations SET title = $2 WHERE id = $1`, conv, title)
		return err
	}
	if err := set(strings.Repeat("字", 40)); err != nil {
		t.Errorf("40 字要過：%v", err)
	}
	if err := set(strings.Repeat("字", 41)); err == nil {
		t.Error("DB 要拒絕 41 字的標題")
	}
	if err := set(" 前導空白"); err == nil {
		t.Error("要保留的頭尾空白約束被刪掉了")
	}
	if err := set("尾端空白 "); err == nil {
		t.Error("要保留的頭尾空白約束被刪掉了（尾端）")
	}
}

func setJob(ctx context.Context, t *testing.T, d *db.DB, run uuid.UUID, job int64) {
	t.Helper()
	if err := store.New(d.Conn(ctx)).SetRunJob(ctx, store.SetRunJobParams{ID: run, JobID: &job}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteConversationCancelsRunsIsIdempotentAndKeepsPublishedRfqs(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, true)
	setJob(ctx, t, d, c.run, 1)
	if err := m.DeleteConversation(ctx, 7, c.conversation); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, d, "conversations", "id = $1 AND deleted_at IS NOT NULL", c.conversation); n != 1 {
		t.Error("對話要被標為已刪除")
	}
	if n := count(ctx, t, d, "agent_runs", "id = $1 AND status = 'CANCEL_REQUESTED'", c.run); n != 1 {
		t.Error("進行中的 run 要在同一個 transaction 轉 CANCEL_REQUESTED（與 CancelRun 相同）")
	}
	if n := count(ctx, t, d, "rfq_publications", "id = $1", c.publication); n != 1 {
		t.Error("已發布的詢價不受影響")
	}
	if n := count(ctx, t, d, "buyer_quote_acceptances", "id = $1", c.acceptance); n != 1 {
		t.Error("接受單不受影響")
	}
	if err := m.DeleteConversation(ctx, 7, c.conversation); err != nil {
		t.Errorf("重送已刪除的對話要回成功：%v", err)
	}
	if n := count(ctx, t, d, "agent_runs", "id = $1 AND status = 'CANCEL_REQUESTED'", c.run); n != 1 {
		t.Error("重送不得再動 run")
	}
	_, err := m.GetConversationMessages(ctx, 7, c.conversation)
	wantKind(t, "刪除後讀訊息", err, apperr.KindNotFound)
	wantKind(t, "刪除後改名", m.RenameConversation(ctx, 7, c.conversation, "x"), apperr.KindNotFound)
	p, err := m.ListConversations(ctx, 7, 10, "")
	if err != nil || len(p.Conversations) != 0 {
		t.Errorf("刪除後不列出：%+v %v", p, err)
	}
}

func TestDeleteConversationCancelsAWaitingRunAndItsInterrupt(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	irq := ask(ctx, t, d, c)
	if err := m.DeleteConversation(ctx, 7, c.conversation); err != nil {
		t.Fatal(err)
	}
	if n := count(ctx, t, d, "agent_runs", "id = $1 AND status = 'CANCELLED'", c.run); n != 1 {
		t.Error("等待輸入的 run 直接轉 CANCELLED")
	}
	if n := count(ctx, t, d, "agent_run_interrupts", "id = $1 AND status = 'CANCELLED'", irq); n != 1 {
		t.Error("PENDING 中斷要隨之取消")
	}
}

func TestGetCandidateSearchHasThreeShapes(t *testing.T) {
	ctx := context.Background()
	m, d := runtimeModule(t)
	c := seed(ctx, t, store.New(d.Conn(ctx)), 7, false)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'SUCCEEDED', completed_at = $2 WHERE id = $1`, c.run, epoch); err != nil {
		t.Fatal(err)
	}
	searchRun := id()
	q := store.New(d.Conn(ctx))
	if n, err := q.InsertRun(ctx, store.InsertRunParams{ID: searchRun, BuyerID: 7, RequestID: c.request, ConversationID: &c.conversation, CandidateSetID: &c.candidateSet,
		RunType: "CANDIDATE_SEARCH", ClientRequestID: id(), Now: epoch}); err != nil || n != 1 {
		t.Fatalf("InsertRun：%d %v", n, err)
	}
	get := func() (CandidateSearch, error) {
		return m.GetCandidateSearch(ctx, 7, c.request, searchRun, uitext.ZhTW)
	}

	got, err := get()
	if err != nil || got.Status != "QUEUED" || got.CandidateSetID != c.candidateSet || len(got.Candidates) != 0 || got.FailureReason != "" {
		t.Errorf("QUEUED：%+v %v", got, err)
	}
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'FAILED', failure_reason = 'SEARCH_FAILED', completed_at = $2 WHERE id = $1`, searchRun, epoch); err != nil {
		t.Fatal(err)
	}
	got, err = get()
	if err != nil || got.Status != "FAILED" || got.FailureReason != "SEARCH_FAILED" || len(got.Candidates) != 0 {
		t.Errorf("FAILED：%+v %v", got, err)
	}
	done := epoch.Add(time.Hour)
	if _, err := d.Pool().Exec(ctx, `UPDATE procurement.agent_runs SET status = 'SUCCEEDED', failure_reason = NULL, completed_at = $2 WHERE id = $1`, searchRun, done); err != nil {
		t.Fatal(err)
	}
	got, err = get()
	if err != nil || got.Status != "SUCCEEDED" || got.FailureReason != "" || len(got.Candidates) != 1 {
		t.Fatalf("SUCCEEDED：%+v %v", got, err)
	}
	cand := got.Candidates[0]
	if cand.ID != c.candidate || cand.StoreID != storeID || cand.Selected || !cand.FactsAsOf.Equal(done) {
		t.Errorf("候選：%+v", cand)
	}
	if len(cand.MatchedFacts) != 1 || cand.MatchedFacts[0].SlotKey != "ITEM" || len(cand.UnverifiableFacts) != 4 {
		t.Errorf("事實由需求現行版本組出（品項、預算、數量、縣市、交期）：%+v %+v", cand.MatchedFacts, cand.UnverifiableFacts)
	}
	if _, err := q.SetCandidateSelected(ctx, store.SetCandidateSelectedParams{ID: c.candidate, CandidateSetID: c.candidateSet, RequestID: c.request, BuyerID: 7, Selected: true, Now: epoch}); err != nil {
		t.Fatal(err)
	}
	if got, _ = get(); len(got.Candidates) != 1 || !got.Candidates[0].Selected {
		t.Errorf("勾選要反映：%+v", got.Candidates)
	}

	_, err = m.GetCandidateSearch(ctx, 7, c.request, c.run, uitext.ZhTW)
	wantKind(t, "TURN 類型的 run", err, apperr.KindNotFound)
	_, err = m.GetCandidateSearch(ctx, 7, id(), searchRun, uitext.ZhTW)
	wantKind(t, "不屬於這個需求", err, apperr.KindNotFound)
	_, err = m.GetCandidateSearch(ctx, 7, c.request, id(), uitext.ZhTW)
	wantKind(t, "不存在的 run", err, apperr.KindNotFound)
}
