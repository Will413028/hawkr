package registry

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func entryOf(t *testing.T, name string) Entry {
	t.Helper()
	e, ok := Lookup(name)
	if !ok {
		t.Fatalf("沒有型別 %s", name)
	}
	return e
}

// 結案的檢舉人 fan-out：買家與店家各走自己的母意圖、只讀自己那一邊，Target.ID 與游標是檢舉人號碼，下一頁從游標之後讀（不從頭重走）。
func TestCaseClosedFanoutReadsOnlyItsSideAndPagesFromTheCursor(t *testing.T) {
	caseID := exampleCaseID
	r := fakeResolver{reporters: map[uuid.UUID]map[model.Audience][]ReporterRef{caseID: {
		model.AudienceBuyer: {
			{ID: 7, ReportID: uuid.MustParse("0190a000-0000-7000-8000-0000000000a1")},
			{ID: 8, ReportID: uuid.MustParse("0190a000-0000-7000-8000-0000000000a4")},
			{ID: 12, ReportID: uuid.MustParse("0190a000-0000-7000-8000-0000000000a6")},
		},
		model.AudienceStore: {
			{ID: 21, ReportID: uuid.MustParse("0190a000-0000-7000-8000-0000000000a2")},
			{ID: 22, ReportID: uuid.MustParse("0190a000-0000-7000-8000-0000000000a3")},
		},
	}}}
	params := mustJSON(t, ModerationCaseClosedParams{CaseID: caseID, Outcome: ModerationOutcomeViolation, OccurredAt: exampleAt})
	collect := func(parent string, store bool) (ids []int64, cursors []int64) {
		e := entryOf(t, parent)
		var after int64
		for {
			cursors = append(cursors, after)
			page, err := e.Spawn(context.Background(), r, params, PageRequest{After: after, Limit: 2, OccurredAt: exampleAt})
			if err != nil {
				t.Fatalf("%s：%v", parent, err)
			}
			for _, it := range page.Items {
				ids = append(ids, it.ID)
				var p ReportResolvedParams
				if err := json.Unmarshal(it.Params, &p); err != nil {
					t.Fatal(err)
				}
				if p.ReportSource != ReportSourceModeration || p.Outcome != ReportOutcomeActionTaken || (store && p.StoreID != it.ID) || (!store && p.BuyerID != it.ID) {
					t.Fatalf("%s 的子參數 %+v", parent, p)
				}
			}
			if page.Done {
				return ids, cursors
			}
			after = page.Next
		}
	}
	if ids, cursors := collect("moderation_case_closed_buyers", false); !slices.Equal(ids, []int64{7, 8, 12}) || !slices.Equal(cursors, []int64{0, 8}) {
		t.Errorf("買家檢舉人 %v，游標 %v", ids, cursors)
	}
	if ids, cursors := collect("moderation_case_closed_stores", true); !slices.Equal(ids, []int64{21, 22}) || !slices.Equal(cursors, []int64{0, 22}) {
		t.Errorf("店家檢舉人 %v，游標 %v", ids, cursors)
	}
}

func TestCaseClosedFanoutRejectsAnUnknownOutcomeAndANonIncreasingReporter(t *testing.T) {
	caseID := exampleCaseID
	r := fakeResolver{reporters: map[uuid.UUID]map[model.Audience][]ReporterRef{caseID: {model.AudienceBuyer: {{ID: 7, ReportID: exampleReportID}}}}}
	e := entryOf(t, "moderation_case_closed_buyers")
	bad := mustJSON(t, ModerationCaseClosedParams{CaseID: caseID, Outcome: "OTHER", OccurredAt: exampleAt})
	if _, err := e.Spawn(context.Background(), r, bad, PageRequest{Limit: 10}); !errors.Is(err, ErrBadParams) {
		t.Errorf("不認得的案件結果要是永久錯誤：%v", err)
	}
	good := mustJSON(t, ModerationCaseClosedParams{CaseID: caseID, Outcome: ModerationOutcomeViolation, OccurredAt: exampleAt})
	if _, err := e.Spawn(context.Background(), r, good, PageRequest{Limit: 10}); err != nil {
		t.Errorf("第一頁：%v", err)
	}
	if _, err := e.Spawn(context.Background(), staleReporters{r}, good, PageRequest{After: 7, Limit: 10}); err == nil {
		t.Error("檢舉人號碼沒有大於游標要回錯誤")
	}
}

type staleReporters struct{ fakeResolver }

func (s staleReporters) CaseReporters(ctx context.Context, caseID uuid.UUID, audience model.Audience, _ int64, limit int) ([]ReporterRef, error) {
	return s.fakeResolver.CaseReporters(ctx, caseID, audience, 0, limit)
}

// moderation 的結果對到統一的類別：違規是 ACTION_TAKEN，不違規同名；子型別的文字只說結果類別。
func TestModerationOutcomesMapToTheReportOutcomeCategories(t *testing.T) {
	caseID := exampleCaseID
	r := fakeResolver{reporters: map[uuid.UUID]map[model.Audience][]ReporterRef{caseID: {model.AudienceBuyer: {{ID: 7, ReportID: exampleReportID}}}}}
	e := entryOf(t, "moderation_case_closed_buyers")
	for outcome, want := range map[string]string{ModerationOutcomeViolation: ReportOutcomeActionTaken, ModerationOutcomeNoViolation: ReportOutcomeNoViolation} {
		page, err := e.Spawn(context.Background(), r, mustJSON(t, ModerationCaseClosedParams{CaseID: caseID, Outcome: outcome, OccurredAt: exampleAt}), PageRequest{Limit: 10})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("%s：%+v %v", outcome, page, err)
		}
		var p ReportResolvedParams
		if err := json.Unmarshal(page.Items[0].Params, &p); err != nil || p.Outcome != want {
			t.Errorf("%s 對到 %q，預期 %q（%v）", outcome, p.Outcome, want, err)
		}
	}
}

// seller 的通報結果：母意圖只展開一位買家檢舉人，重跑同一頁（游標已越過）不再產生子意圖。
func TestSellerReportResolvedSpawnsExactlyTheReporter(t *testing.T) {
	e := entryOf(t, "seller_report_resolved")
	params := mustJSON(t, SellerReportResolvedParams{ReportID: exampleReportID, BuyerID: 42, Outcome: ReportOutcomeDuplicate, OccurredAt: exampleAt})
	page, err := e.Spawn(context.Background(), fakeResolver{}, params, PageRequest{Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != 42 || !page.Done {
		t.Fatalf("第一頁：%+v %v", page, err)
	}
	var p ReportResolvedParams
	if err := json.Unmarshal(page.Items[0].Params, &p); err != nil || p.ReportSource != ReportSourceSeller || p.BuyerID != 42 || p.ReportID != exampleReportID || p.Outcome != ReportOutcomeDuplicate {
		t.Errorf("子參數 %+v（%v）", p, err)
	}
	again, err := e.Spawn(context.Background(), fakeResolver{}, params, PageRequest{After: 42, Limit: 10})
	if err != nil || len(again.Items) != 0 || !again.Done {
		t.Errorf("游標越過後不再展開：%+v %v", again, err)
	}
	bad := mustJSON(t, SellerReportResolvedParams{ReportID: exampleReportID, OccurredAt: exampleAt})
	if _, err := e.Spawn(context.Background(), fakeResolver{}, bad, PageRequest{Limit: 10}); !errors.Is(err, ErrBadParams) {
		t.Errorf("沒有檢舉人要是永久錯誤：%v", err)
	}
}

// 店家面型別由 worker 以店家解析收件人：存活的店家得到 (所屬帳號, 店)，已關閉或不存在的店沒有收件人。
func TestStoreSideTypesResolveTheStoreOwner(t *testing.T) {
	r := fakeResolver{stores: map[int64]int64{11: 501}}
	e := entryOf(t, "store_suspended")
	x, ok, err := e.Expand(context.Background(), r, mustJSON(t, StoreSuspendedParams{StoreID: 11, PeriodID: 5, OccurredAt: exampleAt}))
	if err != nil || !ok || x.Recipient != (Recipient{AccountID: 501, StoreID: 11}) {
		t.Fatalf("存活的店：%+v %v %v", x, ok, err)
	}
	if _, ok, err := e.Expand(context.Background(), r, mustJSON(t, StoreSuspendedParams{StoreID: 12, PeriodID: 5, OccurredAt: exampleAt})); err != nil || ok {
		t.Errorf("已關閉或不存在的店沒有收件人：ok=%v %v", ok, err)
	}
	if _, _, err := e.Expand(context.Background(), r, mustJSON(t, StoreSuspendedParams{StoreID: 0, PeriodID: 5, OccurredAt: exampleAt})); !errors.Is(err, ErrBadParams) {
		t.Errorf("店家號碼不合法要是永久錯誤：%v", err)
	}
}

// 列舉值在 worker 端就被擋下（永久錯誤，不寫出文案壞掉的收件匣列）。
func TestExpandRejectsValuesThatHaveNoText(t *testing.T) {
	r := fakeResolver{buyers: map[int64]int64{42: 4}, stores: map[int64]int64{11: 1}}
	for name, c := range map[string]struct {
		typ    string
		params any
	}{
		"措施種類":   {"store_measure_issued", StoreMeasureIssuedParams{StoreID: 11, MeasureID: exampleMeasureID, Kind: MeasureKindSuspension, OccurredAt: exampleAt}},
		"請求種類":   {"store_measure_request_decided", StoreMeasureRequestDecidedParams{StoreID: 11, MeasureID: exampleMeasureID, RequestID: exampleRequestID, RequestKind: "OTHER", OccurredAt: exampleAt}},
		"詢問種類":   {"store_case_inquiry", StoreCaseInquiryParams{StoreID: 11, InquiryID: exampleRequestID, Kind: "OTHER", DueAt: exampleDueAt, OccurredAt: exampleAt}},
		"補件檔位":   {"payout_account_deadline", PayoutAccountDeadlineParams{StoreID: 11, Deadline: exampleDueAt, Tier: "OTHER", OccurredAt: exampleAt}},
		"代行動作":   {"delegation_operation_completed", DelegationOperationCompletedParams{AuditLogID: exampleGovernanceEventID, TargetStoreID: 11, Action: "OTHER", OccurredAt: exampleAt}},
		"檢舉來源":   {"report_received", ReportReceivedParams{ReportSource: "OTHER", ReportID: exampleReportID, BuyerID: 42, OccurredAt: exampleAt}},
		"結果類別":   {"report_resolved", ReportResolvedParams{ReportSource: ReportSourceSeller, ReportID: exampleReportID, BuyerID: 42, Outcome: "OTHER", OccurredAt: exampleAt}},
		"買家型別帶店": {"report_received", ReportReceivedParams{ReportSource: ReportSourceSeller, ReportID: exampleReportID, BuyerID: 42, StoreID: 11, OccurredAt: exampleAt}},
		"店型別帶買家": {"store_report_received", ReportReceivedParams{ReportSource: ReportSourceModeration, ReportID: exampleReportID, BuyerID: 42, StoreID: 11, OccurredAt: exampleAt}},
	} {
		if _, _, err := entryOf(t, c.typ).Expand(context.Background(), r, mustJSON(t, c.params)); !errors.Is(err, ErrBadParams) {
			t.Errorf("%s：%v", name, err)
		}
	}
}

// 作者通知的申訴途徑：目前每個途徑（只有 NONE）都組得出文字，不認得的途徑組字失敗而不是悄悄略過。
func TestEveryAppealPathRendersAndAnUnknownOneFails(t *testing.T) {
	hidden := entryOf(t, "content_hidden")
	for _, path := range policy.AppealPaths() {
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			raw := mustJSON(t, ContentHiddenParams{SanctionID: exampleSanctionID, BuyerID: 42, Source: ContentSourceMessage, Category: policy.Spam, AppealPath: path, OccurredAt: exampleAt})
			if _, err := hidden.Render(l, raw); err != nil {
				t.Errorf("申訴途徑 %s/%s：%v", path, l, err)
			}
			warned := mustJSON(t, AuthorWarnedParams{SanctionID: exampleSanctionID, BuyerID: 42, Category: policy.Spam, AppealPath: path, OccurredAt: exampleAt})
			if _, err := entryOf(t, "author_warned").Render(l, warned); err != nil {
				t.Errorf("author_warned 申訴途徑 %s/%s：%v", path, l, err)
			}
		}
	}
	bad := mustJSON(t, ContentHiddenParams{SanctionID: exampleSanctionID, BuyerID: 42, Source: ContentSourceSocial, Category: policy.Spam, AppealPath: "PORTAL", OccurredAt: exampleAt})
	if _, err := hidden.Render(uitext.EN, bad); err == nil {
		t.Error("不認得的申訴途徑應該組字失敗")
	}
}

// 來源與結果的每個值都有 zh-TW 與 en 文字，en 不含中文。
func TestEverySourceAndOutcomeHasText(t *testing.T) {
	for _, source := range []string{ContentSourceSocial, ContentSourceReview, ContentSourceMessage} {
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			if m, err := contentSourceLabel(l, source); err != nil || m.Locale != l || m.Text == "" {
				t.Errorf("內容來源 %s/%s：%+v %v", source, l, m, err)
			}
		}
	}
	for _, outcome := range []string{ReportOutcomeActionTaken, ReportOutcomeNoViolation, ReportOutcomeInsufficient, ReportOutcomeDuplicate, ReportOutcomeAbusive} {
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			if m, err := reportOutcomeLabel(l, outcome); err != nil || m.Locale != l || m.Text == "" {
				t.Errorf("檢舉結果 %s/%s：%+v %v", outcome, l, m, err)
			}
		}
	}
	if _, err := contentSourceLabel(uitext.EN, "OTHER"); err == nil {
		t.Error("不認得的來源要失敗")
	}
}

// 確認信型別只有 email 渠道，參數裡沒有任何位址欄位；其餘治理型別都有收件匣。
func TestConfirmationTypesAreEmailOnly(t *testing.T) {
	for _, name := range []string{"store_pending_deletion", "buyer_deletion_requested"} {
		spec := entryOf(t, name).Spec()
		if !slices.Equal(spec.Channels, []model.Channel{model.ChannelEmail}) || spec.Category != model.CategoryAccount {
			t.Errorf("%s 的渠道或分類：%+v", name, spec)
		}
	}
	for _, name := range []string{"store_suspended", "kyc_approved", "content_hidden", "report_received"} {
		if !slices.Contains(entryOf(t, name).Spec().Channels, model.ChannelInbox) {
			t.Errorf("%s 沒有收件匣", name)
		}
	}
}
