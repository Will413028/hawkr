package turn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/extract"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/planner"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var now = time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)

type fakeResolver struct {
	res    resolver.Resolution
	byTerm map[string][]resolver.Match
	leaves map[uuid.UUID][]resolver.Match
	err    error
	calls  int
}

func (f *fakeResolver) Leaves(_ context.Context, rootID uuid.UUID) ([]resolver.Match, error) {
	return f.leaves[rootID], f.err
}

func (f *fakeResolver) Leaf(_ context.Context, id uuid.UUID) (resolver.Match, bool, error) {
	var all []resolver.Match
	all = append(all, f.res.Matches...)
	for _, ms := range f.byTerm {
		all = append(all, ms...)
	}
	for _, ms := range f.leaves {
		all = append(all, ms...)
	}
	for _, m := range all {
		if m.CategoryID == id && m.Leaf {
			return m, true, f.err
		}
	}
	return resolver.Match{}, false, f.err
}

func (f *fakeResolver) Resolve(_ context.Context, term string) (resolver.Resolution, error) {
	f.calls++
	r := f.res
	if ms, ok := f.byTerm[term]; ok {
		r.Matches = ms
	}
	r.BuyerTerm = term
	return r, f.err
}

func sp(s string) *string { return &s }
func ip(i int64) *int64   { return &i }

func resolved() slots.Set {
	return slots.Set{
		Item: sp("蘋果"), Resolution: &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: catID(5), Label: "蘋果"},
		Unit: sp("KG"), DeliveryCity: sp("臺北市"),
	}
}

func match(id int64, name, path string) resolver.Match {
	m := resolver.Match{CategoryID: catID(id), Name: name, Path: path, Via: resolver.ViaExact, Leaf: true}
	if path != "" {
		m.RootName = strings.SplitN(path, " > ", 2)[0]
		m.RootID = rootID(m.RootName)
	}
	return m
}

func nilIDMatch(name, path string) resolver.Match {
	m := match(1, name, path)
	m.CategoryID = uuid.Nil()
	return m
}

func rootID(name string) uuid.UUID {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-9000-%012d", h.Sum32()%100000+1))
}

func catID(n int64) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
}

func optID(n int64) string { return "category-" + catID(n).String() }

func decideOK(t *testing.T, rs Resolver, prior priorState, patch slots.Patch) decision {
	t.Helper()
	d, err := decide(context.Background(), rs, uitext.ZhTW, now, prior, patch)
	if err != nil {
		t.Fatalf("decide：%v", err)
	}
	return d
}

func TestSingleMatchConfirmsTheCategoryWithItsID(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(18, "小白菜", "葉菜 > 小白菜")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("小白菜"), Unit: sp("kg"), DeliveryCity: sp("台北")})
	r := d.next.Resolution
	if r == nil || r.Kind != slots.CategoryConfirmed || r.CategoryID != catID(18) || r.Label != "小白菜" {
		t.Fatalf("resolution = %+v", r)
	}
	if d.interrupt != nil || d.status != statusRequirementReady {
		t.Errorf("interrupt = %v, status = %s：品項已確認、必填齊全，不該中斷", d.interrupt, d.status)
	}
	var keys []string
	for _, r := range d.recorded {
		keys = append(keys, r.Key)
	}
	if !strings.Contains(strings.Join(keys, ","), "SLOT_KEY_ITEM_RESOLUTION") {
		t.Errorf("recorded = %v，解析結果要記為本輪記下的欄位", keys)
	}
}

func TestAmbiguousItemPausesWithAOneLevelCardCappedAtFive(t *testing.T) {
	var ms []resolver.Match
	for i := int64(1); i <= 8; i++ {
		ms = append(ms, match(i, "品項", "大類 > 品項"))
	}
	rs := &fakeResolver{res: resolver.Resolution{Matches: ms}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("品項"), Unit: sp("KG"), DeliveryCity: sp("臺北市")})
	if d.interrupt == nil {
		t.Fatal("多個命中要中斷等買家選")
	}
	if d.next.Resolution != nil {
		t.Errorf("還沒選，不得有解析：%+v", d.next.Resolution)
	}
	if d.interrupt.Stage != StageItem {
		t.Errorf("同一大類的命中不問大類：stage = %s", d.interrupt.Stage)
	}
	if got := len(d.interrupt.Candidates); got != planner.MaxItemOptions {
		t.Errorf("candidates = %d，要截成 %d", got, planner.MaxItemOptions)
	}
	if d.interrupt.Stage != StageItem || d.interrupt.Query != "品項" {
		t.Errorf("payload = %+v", d.interrupt)
	}
	q := d.interrupt.Question
	if q.SlotKey != "SLOT_KEY_ITEM_DISAMBIGUATION" || !q.AllowFreeText || q.CanSkip || len(q.Options) != planner.MaxItemOptions+1 {
		t.Errorf("question = %+v", q)
	}
	if !strings.Contains(q.Options[0].Label, "大類 > 品項") || q.Options[0].OptionID != optID(1) {
		t.Errorf("選項要帶大類路徑：%+v", q.Options[0])
	}
	if d.plan.Act != planner.ItemNeedsDisambiguation {
		t.Errorf("act = %s", d.plan.Act)
	}
	if d.status != statusClarifying {
		t.Errorf("status = %s：品項未定不算齊全", d.status)
	}
}

func TestNoMatchPausesWithTheDescribeOnlyCard(t *testing.T) {
	d := decideOK(t, &fakeResolver{}, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("神秘水果")})
	if d.interrupt == nil || len(d.interrupt.Candidates) != 0 || d.interrupt.Stage != StageDescribe {
		t.Fatalf("interrupt = %+v", d.interrupt)
	}
	if q := d.interrupt.Question; len(q.Options) != 0 || !q.AllowFreeText {
		t.Errorf("question = %+v", q)
	}
}

func TestResolverFailureFailsTheTurnInsteadOfGuessing(t *testing.T) {
	_, err := decide(context.Background(), &fakeResolver{err: errors.New("taxonomy down")}, uitext.ZhTW, now, priorState{}, slots.Patch{Item: sp("蘋果")})
	if err == nil {
		t.Fatal("分類樹讀不到要讓這一輪失敗")
	}
}

func TestResolvesOnlyWhenTheItemChangedOrIsUnresolved(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(5, "蘋果", "水果 > 蘋果")}}}
	prior := priorState{current: resolved(), requestOf: statusClarifying}
	decideOK(t, rs, prior, slots.Patch{Quantity: ip(5)})
	if rs.calls != 0 {
		t.Errorf("品項沒變且已解析，不該再查分類（%d 次）", rs.calls)
	}
	decideOK(t, rs, prior, slots.Patch{Item: sp("蘋果")})
	if rs.calls != 0 {
		t.Errorf("同樣的品項不算變更（%d 次）", rs.calls)
	}
	decideOK(t, rs, prior, slots.Patch{Item: sp("香蕉")})
	if rs.calls != 1 {
		t.Errorf("品項改了要重新解析（%d 次）", rs.calls)
	}
	unresolved := priorState{current: slots.Set{Item: sp("蘋果")}, requestOf: statusClarifying}
	decideOK(t, rs, unresolved, slots.Patch{Quantity: ip(5)})
	if rs.calls != 2 {
		t.Errorf("有品項但未解析要再解析（%d 次）", rs.calls)
	}
}

func TestOptionalProgressIsReadFromThePreviousTurnAndPlannedForThisOne(t *testing.T) {
	rs := &fakeResolver{}
	prior := priorState{current: resolved(), requestOf: statusRequirementReady, prevTurn: &prevTurn{optionalStreak: 1, skipped: []model.SlotKey{slots.Packaging}}}
	d := decideOK(t, rs, prior, slots.Patch{})
	if d.plan.Question == nil || d.plan.Question.Slot != slots.Quantity {
		t.Fatalf("question = %+v，必填齊全後問第一個選填", d.plan.Question)
	}
	if d.plan.OptionalStreak != 2 {
		t.Errorf("OptionalStreak = %d，要接著上一輪的 1 加 1", d.plan.OptionalStreak)
	}
	if len(d.plan.Skipped) != 1 || d.plan.Skipped[0] != slots.Packaging {
		t.Errorf("Skipped = %v，要帶上一輪買家跳過的欄位", d.plan.Skipped)
	}
	d = decideOK(t, rs, priorState{current: resolved(), requestOf: statusRequirementReady, prevTurn: &prevTurn{optionalStreak: planner.MaxConsecutiveOptionalQuestions}}, slots.Patch{})
	if d.plan.Question != nil || d.plan.OptionalStreak != planner.MaxConsecutiveOptionalQuestions {
		t.Errorf("問滿上限後不再問，計數原封不動：%+v", d.plan)
	}
}

func TestMatchingRegressesOnlyWhenASearchFieldChanges(t *testing.T) {
	base := resolved()
	base.Quantity = func() *int32 { v := int32(10); return &v }()
	d0 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	base.DeliveryDate = &d0
	cases := []struct {
		name   string
		mutate func(*slots.Set)
		want   string
	}{
		{"沒改", func(*slots.Set) {}, statusMatching},
		{"包裝", func(s *slots.Set) { s.Packaging = sp("箱") }, statusMatching},
		{"預算", func(s *slots.Set) { s.BudgetPerUnit = ip(30) }, statusMatching},
		{"精確比對", func(s *slots.Set) { v := false; s.ExactMatchRequired = &v }, statusMatching},
		{"品項解析", func(s *slots.Set) {
			s.Resolution = &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: catID(6), Label: "香蕉"}
		}, statusRequirementReady},
		{"數量", func(s *slots.Set) { v := int32(11); s.Quantity = &v }, statusMatching},
		{"單位", func(s *slots.Set) { s.Unit = sp("CATTY") }, statusMatching},
		{"縣市", func(s *slots.Set) { s.DeliveryCity = sp("高雄市") }, statusMatching},
		{"交期", func(s *slots.Set) { d := d0.AddDate(0, 0, 1); s.DeliveryDate = &d }, statusMatching},
	}
	for _, c := range cases {
		next := base
		c.mutate(&next)
		if got := nextStatus(statusMatching, base, next, true); got != c.want {
			t.Errorf("%s：MATCHING → %s，要 %s", c.name, got, c.want)
		}
	}
	if got := nextStatus(statusMatching, base, base, false); got != statusMatching {
		t.Errorf("搜尋欄位沒改就維持 MATCHING（%s）", got)
	}
	next := base
	next.Resolution = &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: catID(6), Label: "香蕉"}
	if got := nextStatus(statusMatching, base, next, false); got != statusClarifying {
		t.Errorf("退回時依齊全與否：未齊全是 CLARIFYING（%s）", got)
	}
	if got := nextStatus(statusClarifying, slots.Set{}, base, true); got != statusRequirementReady {
		t.Errorf("CLARIFYING 齊全 → %s", got)
	}
	if got := nextStatus(statusRequirementReady, base, base, false); got != statusClarifying {
		t.Errorf("REQUIREMENT_READY 變得不齊全 → %s", got)
	}
}

func TestAutoTitle(t *testing.T) {
	long := strings.Repeat("菜", 60)
	for name, c := range map[string]struct {
		in   slots.Set
		want string
	}{
		"品項加縣市":      {slots.Set{Item: sp("小白菜"), DeliveryCity: sp("桃園市")}, "小白菜｜桃園市"},
		"只有品項":       {slots.Set{Item: sp("小白菜")}, "小白菜"},
		"品項未知留空":     {slots.Set{DeliveryCity: sp("桃園市")}, ""},
		"截斷 40 rune": {slots.Set{Item: &long}, strings.Repeat("菜", 40)},
	} {
		if got := autoTitle(c.in); got != c.want {
			t.Errorf("%s：autoTitle = %q，要 %q", name, got, c.want)
		}
	}
}

func TestRevisionMapperKeepsEveryFieldAndTheCategoryID(t *testing.T) {
	d := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	q := int32(7)
	exact := false
	in := slots.Set{
		Item: sp("蘋果"), Resolution: &slots.Resolution{Kind: slots.CategoryConfirmed, CategoryID: catID(5), Label: "蘋果"}, Quantity: &q, Unit: sp("KG"),
		Packaging: sp("箱"), BudgetPerUnit: ip(30), DeliveryCity: sp("臺北市"), DeliveryDate: &d, ExactMatchRequired: &exact,
	}
	p := revisionParams(in, store.InsertRevisionParams{})
	rev := store.ProcurementRequestRevision{
		Item: p.Item, ItemResolutionKind: p.ItemResolutionKind, ItemResolutionLabel: p.ItemResolutionLabel, ItemResolutionCategoryID: p.ItemResolutionCategoryID,
		Quantity: p.Quantity, Unit: p.Unit, Packaging: p.Packaging, BudgetPerUnit: p.BudgetPerUnit, DeliveryCity: p.DeliveryCity,
		DeliveryDate: p.DeliveryDate, ExactMatchRequired: p.ExactMatchRequired,
	}
	if out := setFromRevision(rev); !sameContent(out, in) {
		t.Errorf("往返後內容不同：%+v vs %+v", out, in)
	}
	if p.ItemResolutionCategoryID == nil || *p.ItemResolutionCategoryID != catID(5) {
		t.Errorf("category id = %v", p.ItemResolutionCategoryID)
	}
	described := slots.Set{Item: sp("x"), Resolution: &slots.Resolution{Kind: slots.BuyerDescribed, Label: "x"}}
	if p := revisionParams(described, store.InsertRevisionParams{}); p.ItemResolutionCategoryID != nil || p.ItemResolutionKind == nil {
		t.Errorf("BUYER_DESCRIBED 沒有分類 ID：%+v", p)
	}
	if p := revisionParams(slots.Set{}, store.InsertRevisionParams{ItemResolutionKind: sp("stale")}); p.ItemResolutionKind != nil {
		t.Error("沒有解析時要清掉解析欄位")
	}
}

func TestTurnResultEventShape(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "a", "x > a"), match(2, "b", "y > b")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("a"), BudgetPerUnit: ip(30)})
	intr := uuid.NewV7()
	id := ids{request: uuid.NewV7(), conversation: uuid.NewV7(), revision: uuid.NewV7(), turn: uuid.NewV7(), run: uuid.NewV7()}
	b, err := turnResultEvent(d, id, now, intr)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	tr := got[run.EventTurnResult]
	if tr == nil {
		t.Fatalf("事件鍵不是 %s：%s", run.EventTurnResult, b)
	}
	if tr["interrupt"].(map[string]any)["interruptId"] != intr.String() {
		t.Errorf("interrupt = %v，要等於預先產生的中斷 ID", tr["interrupt"])
	}
	if tr["revisionId"] != id.revision.String() || tr["messageId"] != id.turn.String() || tr["runId"] != id.run.String() {
		t.Errorf("ID 欄位不對：%s", b)
	}
	if tr["slots"].(map[string]any)["budgetPerUnit"] != "30" {
		t.Errorf("int64 在 proto JSON 是字串：%v", tr["slots"])
	}
	if tr["textFallback"] != tr["reply"] || tr["reply"] == "" {
		t.Errorf("reply 與 textFallback：%v / %v", tr["reply"], tr["textFallback"])
	}
	if _, has := tr["quota"]; has {
		t.Error("quota 不填（Q7）")
	}
	d.interrupt = nil
	b, _ = turnResultEvent(d, id, now, uuid.UUID{})
	if strings.Contains(string(b), `"interrupt"`) {
		t.Errorf("沒有中斷時不帶 interrupt：%s", b)
	}
}

func TestClassifyExtractErr(t *testing.T) {
	ctx := context.Background()
	reason := func(err error) string {
		var f *run.FailureError
		if !errors.As(err, &f) {
			return "<none>"
		}
		return f.Reason
	}
	if got := reason(classifyExtractErr(ctx, &extract.Error{Unavailable: true, Err: errors.New("x")})); got != run.FailureModelUnavailable {
		t.Errorf("服務不可用 → %s", got)
	}
	if got := reason(classifyExtractErr(ctx, &extract.Error{Err: errors.New("x")})); got != run.FailureTurnPipelineErr {
		t.Errorf("其他 → %s", got)
	}
	if got := reason(classifyExtractErr(ctx, errors.New("plain"))); got != run.FailureTurnPipelineErr {
		t.Errorf("未分類 → %s", got)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := classifyExtractErr(cctx, &extract.Error{Unavailable: true, Err: errors.New("x")}); !errors.Is(err, context.Canceled) {
		t.Errorf("ctx 已結束要原樣回 ctx 的錯誤：%v", err)
	}
}

func TestSingleMatchWithAnInvalidLabelIsTreatedAsNoMatch(t *testing.T) {
	for name, m := range map[string]resolver.Match{
		"blank label":    match(5, "  ", "水果 > 蘋果"),
		"zero id":        nilIDMatch("蘋果", "水果 > 蘋果"),
		"empty label":    match(5, "", "水果 > 蘋果"),
		"whitespace tab": match(5, "\t", "水果 > 蘋果"),
	} {
		t.Run(name, func(t *testing.T) {
			rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{m}}}
			d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果")})
			if d.interrupt == nil || len(d.interrupt.Candidates) != 0 || d.next.Resolution != nil || d.patch.Resolution != nil {
				t.Fatalf("interrupt = %+v, resolution = %+v, patch resolution = %+v，無效的單一命中要走零候選的描述卡", d.interrupt, d.next.Resolution, d.patch.Resolution)
			}
			if d.plan.Act != planner.ItemNeedsDisambiguation || d.status != statusClarifying || d.plan.Question.Prompt != planner.PromptItemDescribe {
				t.Errorf("act = %v, status = %q, prompt = %v", d.plan.Act, d.status, d.plan.Question.Prompt)
			}
		})
	}
}

func TestInvalidCandidatesNeverAppearOnTheCard(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{
		match(1, "蘋果", "水果 > 蘋果"), match(2, " ", "水果 > 空白"), nilIDMatch("壞 ID", "水果 > 壞"), match(3, "蘋果乾", "加工品 > 蘋果乾"),
	}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果")})
	if d.interrupt == nil || len(d.interrupt.Candidates) != 2 || d.interrupt.Candidates[0].CategoryID != catID(1) || d.interrupt.Candidates[1].CategoryID != catID(3) {
		t.Fatalf("interrupt = %+v，卡上只該有有效的兩個命中", d.interrupt)
	}
	if got := len(d.interrupt.Question.Options); got != 3 {
		t.Errorf("options = %d，要 2 個品項加「其他」", got)
	}
}

func TestInvalidMatchLeavingOneValidStillConfirmsIt(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "", "水果 > 空白")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果")})
	if d.interrupt != nil || d.next.Resolution == nil || d.next.Resolution.CategoryID != catID(1) {
		t.Fatalf("interrupt = %+v, resolution = %+v，剩一個有效命中就是單一命中", d.interrupt, d.next.Resolution)
	}
}

func TestTruncatedLabelIsAnAdjustmentNotARejection(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(7, strings.Repeat("果", slots.MaxTextLen+10), "水果 > 長")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果")})
	if d.interrupt != nil || d.next.Resolution == nil || d.next.Resolution.CategoryID != catID(7) || len([]rune(d.next.Resolution.Label)) > slots.MaxTextLen {
		t.Fatalf("interrupt = %+v, resolution = %+v，被截斷但保留的標籤仍是確認的分類", d.interrupt, d.next.Resolution)
	}
}

func TestNotReadyWhileTheItemHasNoResolution(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果"), Unit: sp("KG"), DeliveryCity: sp("臺北市")})
	if len(d.missing) != 0 || d.next.Resolution != nil || d.status != statusClarifying {
		t.Errorf("missing = %v, resolution = %+v, status = %q：必填齊全但品項未解析，不能進 REQUIREMENT_READY", d.missing, d.next.Resolution, d.status)
	}
	if got := decideOK(t, &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "")}}}, priorState{requestOf: statusClarifying},
		slots.Patch{Item: sp("蘋果"), Unit: sp("KG"), DeliveryCity: sp("臺北市")}); got.status != statusRequirementReady {
		t.Errorf("已解析且必填齊全 status = %q", got.status)
	}
}

func TestRejectedInputWhileDisambiguatingKeepsTheActButAsksTheQuestion(t *testing.T) {
	rs := &fakeResolver{res: resolver.Resolution{Matches: []resolver.Match{match(1, "蘋果", "水果 > 蘋果"), match(2, "蘋果乾", "加工品 > 蘋果乾")}}}
	d := decideOK(t, rs, priorState{requestOf: statusClarifying}, slots.Patch{Item: sp("蘋果"), DeliveryDate: sp("2020-01-01")})
	if d.interrupt == nil || d.plan.Act != planner.InputRejected {
		t.Fatalf("interrupt = %+v, act = %v，被拒要維持 INPUT_REJECTED 且仍中斷", d.interrupt, d.plan.Act)
	}
	if !strings.Contains(d.replyText, "沒辦法直接採用") || !strings.Contains(d.replyText, "找到幾個相符的品項") {
		t.Errorf("replyText = %q，要同時有被拒警告與消歧問句", d.replyText)
	}
}
