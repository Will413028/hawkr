//go:build integration

package turn_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"uuid"

	adkmodel "google.golang.org/adk/v2/model"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/turn"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func (f *fixture) direct() turn.Direct {
	return turn.Direct{DB: f.d, Resolver: resolver.New(taxonomy{}, resolver.Synonyms{}, func() time.Time { return now }), Now: func() time.Time { return now }}
}

// seed 建第一個版本：已解析的品項（小白菜）、單位 KG、縣市 city（空字串是沒填），需求狀態 status。
func (f *fixture) seed(city, status string) uuid.UUID {
	f.t.Helper()
	ctx := context.Background()
	q := store.New(f.d.Conn(ctx))
	id := uuid.NewV7()
	item, kind, label, cat, unit := "小白菜", "CATEGORY_CONFIRMED", "小白菜", int64(18), "KG"
	p := store.InsertRevisionParams{
		ID: id, RevisionNo: 1, Item: &item, ItemResolutionKind: &kind, ItemResolutionLabel: &label, ItemResolutionCategoryID: &cat, Unit: &unit,
		Now: now, RequestID: f.request, BuyerID: ptr(buyer),
	}
	if city != "" {
		p.DeliveryCity = &city
	}
	if n, err := q.InsertRevision(ctx, p); err != nil || n != 1 {
		f.t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	f.setStatus("CLARIFYING", status)
	return id
}

func (f *fixture) setStatus(from, to string) {
	f.t.Helper()
	if from == to {
		return
	}
	if n, err := store.New(f.d.Conn(context.Background())).SetRequestStatus(context.Background(), store.SetRequestStatusParams{Status: to, Now: now, ID: f.request, BuyerID: ptr(buyer), FromStatus: from}); err != nil || n != 1 {
		f.t.Fatalf("SetRequestStatus %s→%s n=%d err=%v", from, to, n, err)
	}
}

func (f *fixture) status() string {
	f.t.Helper()
	r, err := store.New(f.d.Conn(context.Background())).GetRequestForBuyer(context.Background(), store.GetRequestForBuyerParams{ID: f.request, BuyerID: ptr(buyer)})
	if err != nil {
		f.t.Fatal(err)
	}
	return r.Status
}

func (f *fixture) active() store.ProcurementRequestRevision {
	f.t.Helper()
	r, err := store.New(f.d.Conn(context.Background())).GetActiveRevision(context.Background(), store.GetActiveRevisionParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) revisions() []store.ProcurementRequestRevision {
	f.t.Helper()
	r, err := store.New(f.d.Conn(context.Background())).ListRevisions(context.Background(), store.ListRevisionsParams{RequestID: f.request, BuyerID: ptr(buyer)})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *fixture) answerCity(expected uuid.UUID, option string, cid uuid.UUID) ([]byte, error) {
	return f.direct().Answer(context.Background(), turn.AnswerSlotInput{
		BuyerID: buyer, RequestID: f.request, Slot: slots.DeliveryCity, OptionID: option, ExpectedRevisionID: expected, ClientRequestID: cid, Locale: uitext.ZhTW,
	})
}

func (f *fixture) rewind(expected uuid.UUID, slot string, cid uuid.UUID) ([]byte, error) {
	return f.direct().Rewind(context.Background(), turn.RewindInput{
		BuyerID: buyer, RequestID: f.request, Slot: model.SlotKey(slot), ExpectedRevisionID: expected, ClientRequestID: cid, Locale: uitext.ZhTW,
	})
}

func errInfo(err error) (kind apperr.Kind, reason, target string, fields []string) {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return apperr.KindOf(err), "", "", nil
	}
	for _, v := range ae.FieldViolations() {
		fields = append(fields, v.Field)
	}
	return ae.Kind(), ae.Reason.String(), ae.Metadata["conflictTarget"], fields
}

func wantErr(t *testing.T, err error, kind apperr.Kind, reason string) (target string, fields []string) {
	t.Helper()
	k, r, target, fields := errInfo(err)
	if err == nil || k != kind || r != reason {
		t.Fatalf("要 %v／%q，得到 %v", kind, reason, err)
	}
	return target, fields
}

type resultView struct {
	RevisionID string `json:"revisionId"`
	RunID      string `json:"runId"`
	Slots      struct {
		DeliveryCity string `json:"deliveryCity"`
		Item         string `json:"item"`
	} `json:"slots"`
	Question *struct {
		SlotKey string `json:"slotKey"`
	} `json:"question"`
	Rejected []struct {
		Key string `json:"key"`
	} `json:"rejected"`
}

func view(t *testing.T, raw []byte) resultView {
	t.Helper()
	var v resultView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("turn_result 不是 JSON：%s %v", raw, err)
	}
	return v
}

func (f *fixture) noRunNoQuota(why string) {
	f.t.Helper()
	if n := f.count(`SELECT count(*) FROM procurement.agent_runs WHERE request_id = $1`, f.request); n != 0 {
		f.t.Errorf("%s：不該建 run，找到 %d 個", why, n)
	}
	if f.used() != 0 {
		f.t.Errorf("%s：不該扣額度，used = %d", why, f.used())
	}
}

func (f *fixture) turnCount() int {
	return f.count(`SELECT count(*) FROM procurement.turns WHERE request_id = $1`, f.request)
}

func TestAnswerWritesANewRevisionAndTurnWithoutARunOrQuota(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	cid := uuid.NewV7()
	raw, err := f.answerCity(rev1, "city-taichung", cid)
	if err != nil {
		t.Fatal(err)
	}
	v := view(t, raw)
	active := f.active()
	if v.RevisionID != active.ID.String() || v.Slots.DeliveryCity != "臺中市" || v.RunID != "" || active.RevisionNo != 2 || active.ParentRevisionID == nil || *active.ParentRevisionID != rev1 {
		t.Fatalf("result = %+v active = %+v", v, active)
	}
	if got := f.revisions()[0]; got.Status != "SUPERSEDED" || got.ID != rev1 {
		t.Fatalf("舊版本要被取代：%+v", got)
	}
	var source, message string
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT source, buyer_message FROM procurement.turns WHERE client_request_id = $1`, cid).Scan(&source, &message); err != nil || source != "ANSWER" || message != "臺中市" {
		t.Fatalf("turn = %s %q %v", source, message, err)
	}
	f.noRunNoQuota("SubmitAnswer")
	conv, _ := store.New(f.d.Conn(context.Background())).GetConversationForBuyer(context.Background(), store.GetConversationForBuyerParams{ID: f.conversation, BuyerID: buyer})
	if conv.LastMessagePreview == "" || conv.Title != "小白菜｜臺中市" {
		t.Errorf("conversation = %+v", conv)
	}
}

func TestAnswerTextAndQuantityAndSkipBranches(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	d := f.direct()
	run := func(in turn.AnswerSlotInput) (uuid.UUID, resultView) {
		t.Helper()
		in.BuyerID, in.RequestID, in.ExpectedRevisionID, in.ClientRequestID, in.Locale = buyer, f.request, rev, uuid.NewV7(), uitext.ZhTW
		raw, err := d.Answer(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}
		rev = f.active().ID
		return rev, view(t, raw)
	}
	run(turn.AnswerSlotInput{Slot: slots.Quantity, FreeText: "20", UnitOptionID: "unit-catty"})
	if a := f.active(); a.Quantity == nil || *a.Quantity != 20 || a.Unit == nil || *a.Unit != "CATTY" {
		t.Fatalf("數量與單位 = %+v", a)
	}
	run(turn.AnswerSlotInput{Slot: slots.Packaging, FreeText: "  箱  "})
	if a := f.active(); a.Packaging == nil || *a.Packaging != "箱" {
		t.Fatalf("包裝 = %+v", a)
	}
	run(turn.AnswerSlotInput{Slot: slots.BudgetPerUnit, Skip: true})
	var skipped []string
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT skipped_slots FROM procurement.turns ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&skipped); err != nil || len(skipped) != 1 || skipped[0] != "BUDGET_PER_UNIT" {
		t.Fatalf("略過 = %v %v", skipped, err)
	}
	_, v := run(turn.AnswerSlotInput{Slot: slots.BudgetPerUnit, FreeText: "0"})
	if len(v.Rejected) != 1 || v.Rejected[0].Key != "SLOT_KEY_BUDGET_PER_UNIT" {
		t.Fatalf("超出範圍的值記在 rejected，不是錯誤：%+v", v)
	}
	f.noRunNoQuota("SubmitAnswer")
}

func TestAnswerItemTextNeedsExactlyOneLeafCategory(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	in := func(text string) turn.AnswerSlotInput {
		return turn.AnswerSlotInput{BuyerID: buyer, RequestID: f.request, Slot: slots.Item, FreeText: text, ExpectedRevisionID: rev, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW}
	}
	for name, text := range map[string]string{"多個": "蘋果", "零個": "火星蔬菜"} {
		_, err := f.direct().Answer(context.Background(), in(text))
		_, fields := wantErr(t, err, apperr.KindInvalidInput, "")
		if len(fields) != 1 || fields[0] != "FreeText" {
			t.Fatalf("%s：欄位 %v，要 [FreeText]", name, fields)
		}
	}
	if f.turnCount() != 0 || len(f.revisions()) != 1 {
		t.Fatal("被拒絕的 ITEM 回答不得寫入")
	}
	f.noRunNoQuota("ITEM 對不到唯一分類")
	if n := f.count(`SELECT count(*) FROM procurement.agent_run_interrupts`); n != 0 {
		t.Fatalf("SubmitAnswer 不建中斷：%d", n)
	}
	raw, err := f.direct().Answer(context.Background(), in("小白菜"))
	if err != nil || view(t, raw).Slots.Item != "小白菜" {
		t.Fatalf("唯一對到時直接收下：%s %v", raw, err)
	}
}

func TestAnswerMissingRequiredFieldsAndOptionsOffTheCardWriteNothing(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	for name, in := range map[string]turn.AnswerSlotInput{
		"沒有任何回答":   {Slot: slots.DeliveryCity},
		"選項不在卡上":   {Slot: slots.DeliveryCity, OptionID: "city-mars"},
		"沒有欄位":     {OptionID: "city-taipei"},
		"必填欄位不能略過": {Slot: slots.DeliveryCity, Skip: true},
	} {
		in.BuyerID, in.RequestID, in.ExpectedRevisionID, in.ClientRequestID, in.Locale = buyer, f.request, rev, uuid.NewV7(), uitext.ZhTW
		_, err := f.direct().Answer(context.Background(), in)
		if k, _, _, _ := errInfo(err); err == nil || k != apperr.KindInvalidInput {
			t.Fatalf("%s：要 INVALID_ARGUMENT，得到 %v", name, err)
		}
	}
	if f.turnCount() != 0 || len(f.revisions()) != 1 {
		t.Fatal("形狀不合不得寫入")
	}
}

func TestAnswerStaleExpectedRevisionIsAbortedWithRequestRevisionTarget(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	if _, err := f.answerCity(rev1, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	_, err := f.answerCity(rev1, "city-kaohsiung", uuid.NewV7())
	if target, _ := wantErr(t, err, apperr.KindVersionConflict, "VERSION_CONFLICT"); target != "request_revision" {
		t.Fatalf("conflictTarget = %q", target)
	}
	if f.turnCount() != 1 || len(f.revisions()) != 2 {
		t.Fatal("過時的請求不得寫入")
	}
}

func TestAnswerAndRewindWithAPendingInterruptAreFailedPreconditionApprovalPending(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	_, _ = f.ask(ctx)
	rev := f.active().ID
	turnsBefore := f.turnCount()
	_, err := f.answerCity(rev, "city-taipei", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "APPROVAL_PENDING")
	_, err = f.rewind(rev, "ITEM", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "APPROVAL_PENDING")
	if f.turnCount() != turnsBefore {
		t.Fatal("有待回覆的中斷時不得寫入")
	}
}

func TestAnswerAndRewindWhileARunIsInProgressAreRefused(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	_ = f.submit("另一則", uitext.ZhTW)
	_, err := f.answerCity(rev, "city-taichung", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "PROCUREMENT_RUN_IN_PROGRESS")
	_, err = f.rewind(rev, "DELIVERY_CITY", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "PROCUREMENT_RUN_IN_PROGRESS")
	if f.turnCount() != 0 {
		t.Fatal("進行中的 run 擋下時不得寫入")
	}
}

func TestAnswerAndRewindOfSomeoneElsesRequestAreNotFound(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	_, err := f.direct().Answer(context.Background(), turn.AnswerSlotInput{
		BuyerID: buyer + 1, RequestID: f.request, Slot: slots.DeliveryCity, OptionID: "city-taipei", ExpectedRevisionID: rev, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW,
	})
	wantErr(t, err, apperr.KindNotFound, "")
	_, err = f.direct().Rewind(context.Background(), turn.RewindInput{
		BuyerID: buyer + 1, RequestID: f.request, Slot: slots.DeliveryCity, ExpectedRevisionID: rev, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW,
	})
	wantErr(t, err, apperr.KindNotFound, "")
	_, err = f.direct().Answer(context.Background(), turn.AnswerSlotInput{
		BuyerID: buyer, RequestID: uuid.NewV7(), Slot: slots.DeliveryCity, OptionID: "city-taipei", ExpectedRevisionID: rev, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW,
	})
	wantErr(t, err, apperr.KindNotFound, "")
}

func TestAnswerReplayReturnsTheStoredResultByteForByteEvenAfterTheStateMoved(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	cid := uuid.NewV7()
	first, err := f.answerCity(rev1, "city-taichung", cid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.answerCity(f.active().ID, "city-kaohsiung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	revs, turns := len(f.revisions()), f.turnCount()
	again, err := f.answerCity(rev1, "city-taichung", cid)
	if err != nil {
		t.Fatalf("重送要回放，即使讀到的版本已過時、需求已被改過：%v", err)
	}
	if !bytes.Equal(first, again) {
		t.Fatalf("重送要逐位元組等於第一次的結果：\n%s\n%s", first, again)
	}
	if len(f.revisions()) != revs || f.turnCount() != turns {
		t.Fatal("重送不得再寫入")
	}
	f.noRunNoQuota("重送")
}

func TestAnswerReplayWithDifferentContentOrRequestIsInvalidArgument(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	cid := uuid.NewV7()
	if _, err := f.answerCity(rev1, "city-taichung", cid); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"不同的選項": func() error { _, err := f.answerCity(rev1, "city-kaohsiung", cid); return err },
		"不同的讀到版本": func() error {
			_, err := f.answerCity(f.active().ID, "city-taichung", cid)
			return err
		},
		"當成倒回": func() error { _, err := f.rewind(rev1, "DELIVERY_CITY", cid); return err },
	} {
		_, fields := wantErr(t, run(), apperr.KindInvalidInput, "")
		if len(fields) != 1 || fields[0] != "client_request_id" {
			t.Fatalf("%s：欄位 %v", name, fields)
		}
	}
}

func TestAnswerReplayOfAKeyUsedByARunIsInvalidArgument(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	id := f.submit("某則訊息", uitext.ZhTW)
	r := f.run(id)
	_, err := f.answerCity(rev, "city-taichung", r.ClientRequestID)
	_, fields := wantErr(t, err, apperr.KindInvalidInput, "")
	if len(fields) != 1 || fields[0] != "client_request_id" {
		t.Fatalf("欄位 %v", fields)
	}
}

func TestAnswerMovesAMatchingRequestBackOnlyWhenTheItemResolutionChanges(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "MATCHING")
	if _, err := f.answerCity(rev, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	if f.status() != "MATCHING" {
		t.Fatalf("縣市不影響搜尋，status = %s", f.status())
	}
	_, err := f.direct().Answer(context.Background(), turn.AnswerSlotInput{
		BuyerID: buyer, RequestID: f.request, Slot: slots.Item, FreeText: "蘋果", ExpectedRevisionID: f.active().ID, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW,
	})
	wantErr(t, err, apperr.KindInvalidInput, "") // 蘋果有兩個分類：SubmitAnswer 不出卡
	if _, err := f.direct().Answer(context.Background(), turn.AnswerSlotInput{
		BuyerID: buyer, RequestID: f.request, Slot: slots.Item, FreeText: "  小白菜 ", ExpectedRevisionID: f.active().ID, ClientRequestID: uuid.NewV7(), Locale: uitext.ZhTW,
	}); err != nil {
		t.Fatal(err)
	}
	if f.status() != "MATCHING" {
		t.Fatalf("品項解析沒變（同一個分類）要維持 MATCHING，status = %s", f.status())
	}
}

func TestRewindStepsBackAlongTheAncestorChainAndRunsOut(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	if _, err := f.answerCity(rev1, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	rev2 := f.active().ID
	if _, err := f.answerCity(rev2, "city-kaohsiung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	rev3 := f.active().ID

	cid := uuid.NewV7()
	raw, err := f.rewind(rev3, "DELIVERY_CITY", cid)
	if err != nil {
		t.Fatal(err)
	}
	rev4 := f.active()
	if v := view(t, raw); rev4.RevisionNo != 4 || rev4.ID.String() != v.RevisionID || v.Slots.DeliveryCity != "臺中市" || v.Question == nil || v.Question.SlotKey != "SLOT_KEY_DELIVERY_CITY" || v.RunID != "" {
		t.Fatalf("第一次倒回：%+v active = %+v", v, rev4)
	}
	if rev4.ParentRevisionID == nil || *rev4.ParentRevisionID != rev2 || *rev4.DeliveryCity != "臺中市" {
		t.Fatalf("倒回的新版本祖先是目標版、內容同目標版：%+v", rev4)
	}
	var source string
	if err := f.d.Pool().QueryRow(context.Background(), `SELECT source FROM procurement.turns WHERE client_request_id = $1`, cid).Scan(&source); err != nil || source != "REWIND" {
		t.Fatalf("source = %q %v", source, err)
	}
	if got := f.revisions()[2]; got.ID != rev3 || got.Status != "SUPERSEDED" {
		t.Fatalf("舊版本不重新啟用：%+v", got)
	}

	if _, err := f.rewind(rev4.ID, "DELIVERY_CITY", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	rev5 := f.active()
	if rev5.RevisionNo != 5 || rev5.ParentRevisionID == nil || *rev5.ParentRevisionID != rev1 || *rev5.DeliveryCity != "臺北市" {
		t.Fatalf("第二次倒回要再退一步到第一版，不是回到高雄：%+v", rev5)
	}

	_, err = f.rewind(rev5.ID, "DELIVERY_CITY", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "")
	if len(f.revisions()) != 5 {
		t.Fatal("沒有可倒回的內容時不得寫入")
	}
	f.noRunNoQuota("Rewind")
}

func TestRewindOfASlotThatWasNeverWrittenIsFailedPrecondition(t *testing.T) {
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	_, err := f.rewind(rev, "PACKAGING", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "")
	_, err = f.rewind(rev, "EXACT_MATCH_REQUIRED", uuid.NewV7())
	_, fields := wantErr(t, err, apperr.KindInvalidInput, "")
	if len(fields) != 1 || fields[0] != "SlotKey" {
		t.Fatalf("欄位 %v", fields)
	}
	if f.turnCount() != 0 {
		t.Fatal("不得寫入")
	}
}

func TestRewindFirstValueOfASlotCanLandOnAVersionWhereItWasUnset(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("", "CLARIFYING")
	if _, err := f.answerCity(rev1, "city-taipei", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	raw, err := f.rewind(f.active().ID, "DELIVERY_CITY", uuid.NewV7())
	if err != nil {
		t.Fatal(err)
	}
	a := f.active()
	if a.DeliveryCity != nil || a.ParentRevisionID == nil || *a.ParentRevisionID != rev1 || view(t, raw).Question.SlotKey != "SLOT_KEY_DELIVERY_CITY" {
		t.Fatalf("倒回到還沒填的第一版：%+v %s", a, raw)
	}
	_, err = f.rewind(a.ID, "DELIVERY_CITY", uuid.NewV7())
	wantErr(t, err, apperr.KindFailedPrecondition, "")
}

func TestRewindStaleExpectedRevisionIsAborted(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	if _, err := f.answerCity(rev1, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	_, err := f.rewind(rev1, "DELIVERY_CITY", uuid.NewV7())
	if target, _ := wantErr(t, err, apperr.KindVersionConflict, "VERSION_CONFLICT"); target != "request_revision" {
		t.Fatalf("conflictTarget = %q", target)
	}
}

func TestRewindReplayReturnsTheStoredResultAndDetectsADifferentSlot(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "REQUIREMENT_READY")
	if _, err := f.answerCity(rev1, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	rev2 := f.active().ID
	cid := uuid.NewV7()
	first, err := f.rewind(rev2, "DELIVERY_CITY", cid)
	if err != nil {
		t.Fatal(err)
	}
	revs, turns := len(f.revisions()), f.turnCount()
	again, err := f.rewind(rev2, "DELIVERY_CITY", cid)
	if err != nil || !bytes.Equal(first, again) {
		t.Fatalf("重送要回放第一次的結果：%v\n%s\n%s", err, first, again)
	}
	if len(f.revisions()) != revs || f.turnCount() != turns {
		t.Fatal("重送不得再寫入")
	}
	_, err = f.rewind(rev2, "QUANTITY", cid)
	_, fields := wantErr(t, err, apperr.KindInvalidInput, "")
	if len(fields) != 1 || fields[0] != "client_request_id" {
		t.Fatalf("不同欄位的欄位違規 %v", fields)
	}
}

func TestRewindItemFromMatchingGoesBackToReadyButRewindingTheCityStaysMatching(t *testing.T) {
	f := newFixture(t)
	rev1 := f.seed("臺北市", "MATCHING")
	// 第二版把品項換成另一個分類：結果與第一版的解析不同（用資料列直接寫，模擬較早的 TURN 結果）。
	q := store.New(f.d.Conn(context.Background()))
	item, kind, label, cat, unit, city := "梨", "CATEGORY_CONFIRMED", "梨", int64(30), "KG", "臺北市"
	if _, err := q.SupersedeActiveRevision(context.Background(), store.SupersedeActiveRevisionParams{RequestID: f.request, BuyerID: ptr(buyer)}); err != nil {
		t.Fatal(err)
	}
	rev2 := uuid.NewV7()
	if n, err := q.InsertRevision(context.Background(), store.InsertRevisionParams{
		ID: rev2, RevisionNo: 2, ParentRevisionID: &rev1, Item: &item, ItemResolutionKind: &kind, ItemResolutionLabel: &label, ItemResolutionCategoryID: &cat,
		Unit: &unit, DeliveryCity: &city, Now: now, RequestID: f.request, BuyerID: ptr(buyer),
	}); err != nil || n != 1 {
		t.Fatalf("InsertRevision n=%d err=%v", n, err)
	}
	raw, err := f.rewind(rev2, "ITEM", uuid.NewV7())
	if err != nil {
		t.Fatal(err)
	}
	if a := f.active(); *a.Item != "小白菜" || *a.ItemResolutionCategoryID != 18 || f.status() != "REQUIREMENT_READY" {
		t.Fatalf("品項解析變了要退回：active = %+v status = %s", a, f.status())
	}
	if view(t, raw).Question.SlotKey != "SLOT_KEY_ITEM" {
		t.Fatalf("問題要是品項：%s", raw)
	}
	f.setStatus("REQUIREMENT_READY", "MATCHING")
	if _, err := f.answerCity(f.active().ID, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.rewind(f.active().ID, "DELIVERY_CITY", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	if f.status() != "MATCHING" {
		t.Fatalf("縣市不影響搜尋，status = %s", f.status())
	}
}

func TestADirectTurnIsTheBaseOfTheNextRunBasedTurn(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	if _, err := f.answerCity(rev, "city-taichung", uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	id := f.submit("加上包裝", uitext.ZhTW)
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"packaging":"箱"}`)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	a := f.active()
	if a.Packaging == nil || *a.DeliveryCity != "臺中市" || a.RevisionNo != 3 {
		t.Fatalf("run 的新版本要以不經 run 的版本為基礎：%+v", a)
	}
	f.requireStoredTurnResult(id)
}

func (f *fixture) answer(expected uuid.UUID, in turn.AnswerSlotInput, cid uuid.UUID) ([]byte, error) {
	in.BuyerID, in.RequestID, in.ExpectedRevisionID, in.ClientRequestID, in.Locale = buyer, f.request, expected, cid, uitext.ZhTW
	return f.direct().Answer(context.Background(), in)
}

func TestAnswerSkipStoresNoRejectedInput(t *testing.T) {
	for _, slot := range []model.SlotKey{slots.Packaging, slots.BudgetPerUnit} {
		f := newFixture(t)
		rev := f.seed("臺北市", "REQUIREMENT_READY")
		cid := uuid.NewV7()
		raw, err := f.answer(rev, turn.AnswerSlotInput{Slot: slot, Skip: true}, cid)
		if err != nil {
			t.Fatal(err)
		}
		if v := view(t, raw); len(v.Rejected) != 0 {
			t.Fatalf("%s：略過不該有 rejected：%s", slot, raw)
		}
		var stored []byte
		if err := f.d.Pool().QueryRow(context.Background(), `SELECT turn_result FROM procurement.turns WHERE client_request_id = $1`, cid).Scan(&stored); err != nil || len(view(t, stored).Rejected) != 0 {
			t.Fatalf("%s：存下的 turn_result 不該有 rejected：%s %v", slot, stored, err)
		}
	}
}

func TestAnswerReplayWithADifferentSlotIsInvalidArgument(t *testing.T) {
	text := func(slot model.SlotKey) turn.AnswerSlotInput {
		return turn.AnswerSlotInput{Slot: slot, FreeText: "100"}
	}
	skip := func(slot model.SlotKey) turn.AnswerSlotInput { return turn.AnswerSlotInput{Slot: slot, Skip: true} }
	for name, tc := range map[string]struct{ first, second turn.AnswerSlotInput }{
		"同一段文字回答不同欄位": {text(slots.Quantity), text(slots.BudgetPerUnit)},
		"略過不同欄位":      {skip(slots.Packaging), skip(slots.BudgetPerUnit)},
	} {
		f := newFixture(t)
		rev := f.seed("臺北市", "REQUIREMENT_READY")
		cid := uuid.NewV7()
		first, err := f.answer(rev, tc.first, cid)
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		revs, turns := len(f.revisions()), f.turnCount()
		_, err = f.answer(rev, tc.second, cid)
		if _, fields := wantErr(t, err, apperr.KindInvalidInput, ""); len(fields) != 1 || fields[0] != "client_request_id" {
			t.Fatalf("%s：欄位 %v", name, fields)
		}
		again, err := f.answer(rev, tc.first, cid)
		if err != nil || !bytes.Equal(first, again) {
			t.Fatalf("%s：同欄位重送要回放第一次的結果：%v\n%s\n%s", name, err, first, again)
		}
		if len(f.revisions()) != revs || f.turnCount() != turns {
			t.Fatalf("%s：重送不得再寫入", name)
		}
	}
}

func TestCommandSlotIsSetOnlyForSubmitAnswerAndRewindTurns(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rev := f.seed("臺北市", "REQUIREMENT_READY")
	answerCid, rewindCid := uuid.NewV7(), uuid.NewV7()
	if _, err := f.answerCity(rev, "city-taichung", answerCid); err != nil {
		t.Fatal(err)
	}
	if _, err := f.rewind(f.active().ID, "DELIVERY_CITY", rewindCid); err != nil {
		t.Fatal(err)
	}
	id := f.submit("加上包裝", uitext.ZhTW)
	l := &scripted{steps: []func(context.Context) (*adkmodel.LLMResponse, error){say(`{"packaging":"箱"}`)}}
	if err := f.runWith(ctx, id, f.executor(llmExtractor(l))); err != nil {
		t.Fatal(err)
	}
	slotOf := func(where string, arg any) *string {
		var s *string
		if err := f.d.Pool().QueryRow(ctx, `SELECT command_slot FROM procurement.turns WHERE `+where, arg).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := slotOf(`client_request_id = $1`, answerCid); s == nil || *s != "DELIVERY_CITY" {
		t.Errorf("SubmitAnswer 的 command_slot = %v", s)
	}
	if s := slotOf(`client_request_id = $1`, rewindCid); s == nil || *s != "DELIVERY_CITY" {
		t.Errorf("Rewind 的 command_slot = %v", s)
	}
	if s := slotOf(`source = $1`, "LLM_EXTRACT"); s != nil {
		t.Errorf("TURN 的 command_slot 要是 NULL：%v", *s)
	}
	for name, stmt := range map[string]string{
		"倒回沒有欄位":     `UPDATE procurement.turns SET command_slot = NULL WHERE source = 'REWIND'`,
		"TURN 帶了欄位":  `UPDATE procurement.turns SET command_slot = 'ITEM' WHERE source = 'LLM_EXTRACT'`,
		"欄位不在可回答的範圍": `UPDATE procurement.turns SET command_slot = 'UNIT' WHERE source = 'ANSWER'`,
	} {
		if _, err := f.d.Pool().Exec(ctx, stmt); err == nil {
			t.Errorf("%s：CHECK 要拒絕", name)
		}
	}
}
