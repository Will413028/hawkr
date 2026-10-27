package model

import (
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

var reportCaseEpoch = time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

func outcomePtr(o Outcome) *Outcome { return &o }

func openCase(category policy.Category) Case {
	return Case{ID: uuid.NewV7(), Category: category, Severity: SeverityLow, Version: 1, OpenedAt: reportCaseEpoch, UpdatedAt: reportCaseEpoch}
}

func closedCase(category policy.Category, reason CaseCloseReason) Case {
	c := openCase(category)
	closedAt := reportCaseEpoch.Add(time.Hour)
	c.ClosedAt, c.CloseReason = &closedAt, &reason
	if reason == CaseClosedDecided {
		d := DecisionNoAction
		c.Decision = &d
	}
	return c
}

// 通報的處理狀態只由事實推導：有結果就是已有結果（不論有沒有案件）；沒有結果而有案件是在案件中；兩者都沒有是待分案。
func TestReportStateIsDerivedFromTheFacts(t *testing.T) {
	caseID := uuid.NewV7()
	for name, tt := range map[string]struct {
		report Report
		want   ReportState
	}{
		"沒有案件也沒有結果":  {Report{}, ReportStateAwaitingTriage},
		"在案件裡、還沒有結果": {Report{CaseID: &caseID}, ReportStateInCase},
		"有結果、沒有案件":   {Report{Outcome: outcomePtr(OutcomeNoViolation)}, ReportStateResolved},
		"有結果、在案件裡":   {Report{CaseID: &caseID, Outcome: outcomePtr(OutcomeActionTaken)}, ReportStateResolved},
		"結果是重複、沒有案件": {Report{Outcome: outcomePtr(OutcomeDuplicate)}, ReportStateResolved},
		"結果是濫報、沒有案件": {Report{Outcome: outcomePtr(OutcomeAbusive)}, ReportStateResolved},
		"案件裡、結果是未違規": {Report{CaseID: &caseID, Outcome: outcomePtr(OutcomeNoViolation)}, ReportStateResolved},
	} {
		if got := tt.report.State(); got != tt.want {
			t.Errorf("%s：State=%s，預期 %s", name, got, tt.want)
		}
	}
	if got := ReportStateValues(); !slices.Equal(got, []ReportState{ReportStateAwaitingTriage, ReportStateInCase, ReportStateResolved}) {
		t.Errorf("ReportStateValues=%v", got)
	}
}

// 結果是否否定通報：未違規、內容不足、濫報否定；沒有結果、已採取措施、重複不否定（rb R2 的 3b 規則）。
func TestSupportingFollowsTheOutcome(t *testing.T) {
	if !(Report{}).Supporting() {
		t.Error("沒有結果的通報可以當來源")
	}
	for outcome, want := range map[Outcome]bool{
		OutcomeActionTaken: true, OutcomeDuplicate: true,
		OutcomeNoViolation: false, OutcomeInsufficient: false, OutcomeAbusive: false,
	} {
		if got := (Report{Outcome: outcomePtr(outcome)}).Supporting(); got != want {
			t.Errorf("%s：Supporting=%v，預期 %v", outcome, got, want)
		}
		if outcome.Negates() == want {
			t.Errorf("%s：Negates 要和 Supporting 相反", outcome)
		}
	}
	if got := OutcomeValues(); !slices.Equal(got, []Outcome{OutcomeActionTaken, OutcomeNoViolation, OutcomeInsufficient, OutcomeDuplicate, OutcomeAbusive}) {
		t.Errorf("OutcomeValues 的順序是契約的順序：%v", got)
	}
}

// 結案帶入的結果：採取措施是已採取措施，不採取措施是未違規。
func TestDecisionDefaultOutcome(t *testing.T) {
	if got := DecisionActionTaken.DefaultOutcome(); got != OutcomeActionTaken {
		t.Errorf("採取措施：%s", got)
	}
	if got := DecisionNoAction.DefaultOutcome(); got != OutcomeNoViolation {
		t.Errorf("不採取措施：%s", got)
	}
	if got := DecisionValues(); !slices.Equal(got, []Decision{DecisionActionTaken, DecisionNoAction}) {
		t.Errorf("DecisionValues 的順序：%v", got)
	}
}

// 「待分案」是沒有案件也沒有結果，與通報建立時選的類別無關。分案的開始時間與期限在 SQL（seller.review_report_triage），
// 資料集測試拿它的成員比對 State()。
func TestAwaitingTriageStateIsNoCaseAndNoOutcomeRegardlessOfCategory(t *testing.T) {
	created := time.Date(2026, 10, 18, 3, 4, 5, 123456000, time.UTC)
	caseID := uuid.NewV7()
	for name, r := range map[string]Report{
		"食安":      {Category: policy.FoodSafety, CreatedAt: created},
		"其他類別":    {Category: policy.Fraud, CreatedAt: created},
		"檢舉人選了其他": {Category: policy.Other, CreatedAt: created},
	} {
		if r.State() != ReportStateAwaitingTriage {
			t.Errorf("%s：沒有案件也沒有結果就是待分案", name)
		}
	}
	for name, r := range map[string]Report{
		"食安但已在案件裡":   {Category: policy.FoodSafety, CreatedAt: created, CaseID: &caseID},
		"食安但已有結果":    {Category: policy.FoodSafety, CreatedAt: created, Outcome: outcomePtr(OutcomeNoViolation)},
		"其他類別已有結果":   {Category: policy.Privacy, CreatedAt: created, Outcome: outcomePtr(OutcomeDuplicate)},
		"食安、在案件並有結果": {Category: policy.FoodSafety, CreatedAt: created, CaseID: &caseID, Outcome: outcomePtr(OutcomeActionTaken)},
	} {
		if r.State() == ReportStateAwaitingTriage {
			t.Errorf("%s：不是待分案", name)
		}
	}
}

// 通報「有新的補充」是存在編號大於已看過編號（SupplementsSeenSeq）的補充：最新一則的編號等於已看過的編號就不算，更大就算；沒有補充永遠不算；有了結果不再提醒。
// 比的是最新一則的編號，不是則數：編號不連號時（則數 2、最新編號 5、看過 3）兩者答案不同，以編號為準（與 SQL 的 EXISTS seq > supplements_seen_seq 一致）。
// 它不看案件的 UpdatedAt，也不看任何時間：對別的成員做處理不會把這筆的補充標成看過。
func TestReportHasUnseenSupplementsComparesWithTheSeenMarker(t *testing.T) {
	done := OutcomeNoViolation
	for name, tt := range map[string]struct {
		count, latest, seen int
		outcome             *Outcome
		want                bool
	}{
		"沒有補充":           {0, 0, 0, nil, false},
		"有補充、從沒看過":       {1, 1, 0, nil, true},
		"看過最新一則":         {3, 3, 3, nil, false},
		"最新一則還沒看過":       {4, 4, 3, nil, true},
		"看過 0 則、有 10 則":  {10, 10, 0, nil, true},
		"有結果不再提醒":        {2, 2, 0, &done, false},
		"編號不連號、最新一則還沒看過": {2, 5, 3, nil, true},
		"編號不連號、看過最新一則":   {2, 5, 5, nil, false},
	} {
		r := Report{SupplementCount: tt.count, LatestSupplementSeq: tt.latest, SupplementsSeenSeq: tt.seen, Outcome: tt.outcome}
		if got := r.HasUnseenSupplements(); got != tt.want {
			t.Errorf("%s：HasUnseenSupplements=%v，預期 %v", name, got, tt.want)
		}
	}
}

// 生效的類別：在案件裡是平台判定類別，其他是檢舉人選的類別。
func TestReportEffectiveCategory(t *testing.T) {
	assessed := policy.Fraud
	if got := (Report{Category: policy.FoodSafety}).EffectiveCategory(); got != policy.FoodSafety {
		t.Errorf("沒有判定：EffectiveCategory=%v", got)
	}
	if got := (Report{Category: policy.Other, AssessedCategory: &assessed}).EffectiveCategory(); got != policy.Fraud {
		t.Errorf("已判定：EffectiveCategory=%v", got)
	}
}

func TestCaseStateAndDecisionClock(t *testing.T) {
	if open, closed := openCase(policy.Fraud), closedCase(policy.Fraud, CaseClosedDecided); !open.Open() || open.State() != CaseStateOpen || closed.Open() || closed.State() != CaseStateClosed {
		t.Error("案件的狀態由 closed_at 推導")
	}
}

func blocksOf(bs ...CaseBlock) []CaseBlock { return bs }

// 案件每個動作不能做的原因，逐格斷言內容與順序（API 的 can_*／decision_options 與指令的前置檢查都只看這個函式）。
func TestCaseAvailabilityListsEveryReasonInOrder(t *testing.T) {
	type want struct {
		takeAction, noAction   []CaseBlock
		severity, link, create []CaseBlock
		options                []Decision
	}
	food, fraud := policy.FoodSafety, policy.Fraud
	for name, tt := range map[string]struct {
		facts CaseFacts
		want  want
	}{
		"進行中的案件，什麼作為都沒有": {
			CaseFacts{Case: openCase(food)},
			want{takeAction: blocksOf(CaseBlockNoActionBasis), options: []Decision{DecisionNoAction}},
		},
		"進行中、有一個未撤銷的導出措施": {
			CaseFacts{Case: openCase(fraud), ResultingMeasures: 1, UnrevokedMeasures: 1},
			want{noAction: blocksOf(CaseBlockUnrevokedMeasureExists), options: []Decision{DecisionActionTaken}},
		},
		"進行中、導出的措施都撤銷了（做過就是採取過措施）": {
			CaseFacts{Case: openCase(fraud), ResultingMeasures: 1},
			want{options: []Decision{DecisionActionTaken, DecisionNoAction}},
		},
		"進行中、有成員的結果是已採取措施": {
			CaseFacts{Case: openCase(fraud), ActionTakenReports: 1},
			want{noAction: blocksOf(CaseBlockReportActionTaken), options: []Decision{DecisionActionTaken}},
		},
		"進行中、未撤銷的措施與已採取措施的成員都有（兩個原因依序列出）": {
			CaseFacts{Case: openCase(fraud), ResultingMeasures: 2, UnrevokedMeasures: 1, ActionTakenReports: 3},
			want{noAction: blocksOf(CaseBlockUnrevokedMeasureExists, CaseBlockReportActionTaken), options: []Decision{DecisionActionTaken}},
		},
		"已結案、什麼作為都沒有": {
			CaseFacts{Case: closedCase(fraud, CaseClosedDecided)},
			want{
				takeAction: blocksOf(CaseBlockClosed, CaseBlockNoActionBasis), noAction: blocksOf(CaseBlockClosed),
				severity: blocksOf(CaseBlockClosed), link: blocksOf(CaseBlockClosed), create: blocksOf(CaseBlockClosed),
			},
		},
		"已結案、有未撤銷的措施與已採取措施的成員（已結案永遠在最前面）": {
			CaseFacts{Case: closedCase(fraud, CaseClosedDecided), ResultingMeasures: 1, UnrevokedMeasures: 1, ActionTakenReports: 1},
			want{
				takeAction: blocksOf(CaseBlockClosed), noAction: blocksOf(CaseBlockClosed, CaseBlockUnrevokedMeasureExists, CaseBlockReportActionTaken),
				severity: blocksOf(CaseBlockClosed), link: blocksOf(CaseBlockClosed), create: blocksOf(CaseBlockClosed),
			},
		},
		"進行中、店家已關閉：只有從案件建立措施被擋": {
			CaseFacts{Case: openCase(food), StoreClosed: true},
			want{takeAction: blocksOf(CaseBlockNoActionBasis), create: blocksOf(CaseBlockStoreClosed), options: []Decision{DecisionNoAction}},
		},
		"已結案、店家已關閉：建立措施依序是已結案、店家已關閉": {
			CaseFacts{Case: closedCase(food, CaseClosedDecided), StoreClosed: true},
			want{
				takeAction: blocksOf(CaseBlockClosed, CaseBlockNoActionBasis), noAction: blocksOf(CaseBlockClosed),
				severity: blocksOf(CaseBlockClosed), link: blocksOf(CaseBlockClosed), create: blocksOf(CaseBlockClosed, CaseBlockStoreClosed),
			},
		},
		"以沒有通報關閉": {
			CaseFacts{Case: closedCase(food, CaseClosedNoReports)},
			want{
				takeAction: blocksOf(CaseBlockClosed, CaseBlockNoActionBasis), noAction: blocksOf(CaseBlockClosed),
				severity: blocksOf(CaseBlockClosed), link: blocksOf(CaseBlockClosed), create: blocksOf(CaseBlockClosed),
			},
		},
	} {
		a := tt.facts.Availability()
		check := func(what string, got, want []CaseBlock) {
			t.Helper()
			if !slices.Equal(got, want) {
				t.Errorf("%s／%s：%v，預期 %v", name, what, got, want)
			}
		}
		check("決定採取措施", a.Decide[DecisionActionTaken], tt.want.takeAction)
		check("決定不採取措施", a.Decide[DecisionNoAction], tt.want.noAction)
		check("改嚴重度", a.ChangeSeverity, tt.want.severity)
		check("連結措施", a.LinkMeasure, tt.want.link)
		check("建立措施", a.CreateMeasure, tt.want.create)
		if got := a.DecisionOptions(); !slices.Equal(got, tt.want.options) {
			t.Errorf("%s／decision_options：%v，預期 %v", name, got, tt.want.options)
		}
	}
}

// I5：每個進行中的案件至少有一個可以選的決定。事實來自同一個導出集合（未撤銷的措施是導出措施的子集），所以窮舉所有一致的組合都成立；
// 沒有任何作為時只有「不採取措施」，有作為（措施做過、或有成員已採取措施）時「採取措施」一定可選。
func TestEveryOpenCaseHasADecisionOptionForAllConsistentFacts(t *testing.T) {
	for resulting := range 4 {
		for unrevoked := 0; unrevoked <= resulting; unrevoked++ {
			for actionTaken := range 4 {
				for _, category := range []policy.Category{policy.FoodSafety, policy.Fraud} {
					for _, storeClosed := range []bool{false, true} {
						f := CaseFacts{Case: openCase(category), StoreClosed: storeClosed, ResultingMeasures: resulting, UnrevokedMeasures: unrevoked, ActionTakenReports: actionTaken}
						options := f.Availability().DecisionOptions()
						if len(options) == 0 {
							t.Fatalf("導出措施 %d（未撤銷 %d）、已採取措施的成員 %d：沒有任何可以選的決定", resulting, unrevoked, actionTaken)
						}
						if has := slices.Contains(options, DecisionActionTaken); has != (resulting > 0 || actionTaken > 0) {
							t.Errorf("導出措施 %d、已採取措施的成員 %d：採取措施可選=%v", resulting, actionTaken, has)
						}
						if has := slices.Contains(options, DecisionNoAction); has != (unrevoked == 0 && actionTaken == 0) {
							t.Errorf("未撤銷 %d、已採取措施的成員 %d：不採取措施可選=%v", unrevoked, actionTaken, has)
						}
						if f.ActionTakenSupported() != slices.Contains(options, DecisionActionTaken) {
							t.Errorf("ActionTakenSupported 要和「採取措施可選」一致（案件與成員共用）：%+v", f)
						}
					}
				}
			}
		}
	}
	// 已結案的案件沒有任何可以選的決定。
	if got := (CaseFacts{Case: closedCase(policy.Fraud, CaseClosedDecided), ActionTakenReports: 1, ResultingMeasures: 1}).Availability().DecisionOptions(); len(got) != 0 {
		t.Errorf("已結案：%v", got)
	}
}

func reportBlocksOf(bs ...ReportBlock) []ReportBlock { return bs }

// 通報每個動作不能做的原因，逐格斷言內容與順序。
func TestReportAvailabilityListsEveryReasonInOrder(t *testing.T) {
	type want struct {
		triage, move, cite []ReportBlock
		takeAction         []ReportBlock // 結果「已採取措施」不能設的原因
		otherOutcomes      []ReportBlock // 其餘結果不能設的原因（都相同）
		outcomes           []Outcome
	}
	noActionBasis := &CaseFacts{Case: openCase(policy.Fraud), StoreClosed: false}
	withBasis := &CaseFacts{Case: openCase(policy.Fraud), ResultingMeasures: 1}
	closedWithOutcome := &CaseFacts{Case: closedCase(policy.Fraud, CaseClosedDecided), ActionTakenReports: 1}
	caseID := uuid.NewV7()
	awaiting := Report{Category: policy.Fraud}
	inCase := Report{Category: policy.Fraud, CaseID: &caseID}
	resolvedInCase := Report{Category: policy.Fraud, CaseID: &caseID, Outcome: outcomePtr(OutcomeActionTaken)}
	resolvedAlone := Report{Category: policy.Fraud, Outcome: outcomePtr(OutcomeNoViolation)}
	allOutcomes := OutcomeValues()
	withoutActionTaken := OutcomeValues()[1:]
	lastWithMeasure := func(c *CaseFacts) *CaseFacts {
		v := *c
		v.Case.ReportCount = 1
		return &v
	}
	moreMembers := func(c *CaseFacts) *CaseFacts {
		v := *c
		v.Case.ReportCount = 2
		return &v
	}
	for name, tt := range map[string]struct {
		facts ReportFacts
		want  want
	}{
		"待分案": {
			ReportFacts{Report: awaiting},
			want{move: reportBlocksOf(ReportBlockNotInOpenCase), takeAction: reportBlocksOf(ReportBlockNeedsCase), outcomes: withoutActionTaken},
		},
		"在進行中的案件裡、案件還沒有可以說已採取措施的作為": {
			ReportFacts{Report: inCase, Case: noActionBasis},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), takeAction: reportBlocksOf(ReportBlockNoActionBasis), outcomes: withoutActionTaken},
		},
		"在進行中的案件裡、案件有導出的措施": {
			ReportFacts{Report: inCase, Case: withBasis},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), outcomes: allOutcomes},
		},
		"在進行中的案件裡、是認定的來源": {
			ReportFacts{Report: inCase, Case: withBasis, FindingSource: true},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), move: reportBlocksOf(ReportBlockFindingSource), outcomes: allOutcomes},
		},
		"是案件唯一的成員而案件有導出的措施": {
			ReportFacts{Report: inCase, Case: lastWithMeasure(withBasis)},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), move: reportBlocksOf(ReportBlockLastReportWithMeasure), outcomes: allOutcomes},
		},
		"案件還有別的成員時可以移動": {
			ReportFacts{Report: inCase, Case: moreMembers(withBasis)},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), outcomes: allOutcomes},
		},
		"唯一的成員但案件沒有導出的措施可以移動": {
			ReportFacts{Report: inCase, Case: lastWithMeasure(noActionBasis)},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), takeAction: reportBlocksOf(ReportBlockNoActionBasis), outcomes: withoutActionTaken},
		},
		"既是認定的來源、又是有導出措施的案件唯一的成員（兩個原因依序列出）": {
			ReportFacts{Report: inCase, Case: lastWithMeasure(withBasis), FindingSource: true},
			want{triage: reportBlocksOf(ReportBlockAlreadyTriaged), move: reportBlocksOf(ReportBlockFindingSource, ReportBlockLastReportWithMeasure), outcomes: allOutcomes},
		},
		"已有結果、在已結案的案件裡": {
			ReportFacts{Report: resolvedInCase, Case: closedWithOutcome},
			want{
				triage:     reportBlocksOf(ReportBlockResolved, ReportBlockAlreadyTriaged),
				move:       reportBlocksOf(ReportBlockResolved, ReportBlockNotInOpenCase),
				takeAction: reportBlocksOf(ReportBlockResolved), otherOutcomes: reportBlocksOf(ReportBlockResolved),
			},
		},
		"已有結果（未違規）、沒有案件": {
			ReportFacts{Report: resolvedAlone},
			want{
				triage: reportBlocksOf(ReportBlockResolved), move: reportBlocksOf(ReportBlockResolved, ReportBlockNotInOpenCase),
				cite:       reportBlocksOf(ReportBlockNegated),
				takeAction: reportBlocksOf(ReportBlockResolved, ReportBlockNeedsCase), otherOutcomes: reportBlocksOf(ReportBlockResolved),
			},
		},
		"店家已關閉、通報沒有被否定": {
			ReportFacts{Report: awaiting, StoreClosed: true},
			want{move: reportBlocksOf(ReportBlockNotInOpenCase), cite: reportBlocksOf(ReportBlockStoreClosed), takeAction: reportBlocksOf(ReportBlockNeedsCase), outcomes: withoutActionTaken},
		},
		"店家已關閉、通報也被否定（依序）": {
			ReportFacts{Report: resolvedAlone, StoreClosed: true},
			want{
				triage: reportBlocksOf(ReportBlockResolved), move: reportBlocksOf(ReportBlockResolved, ReportBlockNotInOpenCase),
				cite:       reportBlocksOf(ReportBlockNegated, ReportBlockStoreClosed),
				takeAction: reportBlocksOf(ReportBlockResolved, ReportBlockNeedsCase), otherOutcomes: reportBlocksOf(ReportBlockResolved),
			},
		},
	} {
		a := tt.facts.Availability()
		check := func(what string, got, want []ReportBlock) {
			t.Helper()
			if !slices.Equal(got, want) {
				t.Errorf("%s／%s：%v，預期 %v", name, what, got, want)
			}
		}
		check("分案", a.Triage, tt.want.triage)
		check("移動", a.Move, tt.want.move)
		check("當作認定的來源", a.CiteAsSource, tt.want.cite)
		check("結果「已採取措施」", a.Resolve[OutcomeActionTaken], tt.want.takeAction)
		for _, o := range OutcomeValues()[1:] {
			check("結果 "+string(o), a.Resolve[o], tt.want.otherOutcomes)
		}
		if got := a.AvailableOutcomes(); !slices.Equal(got, tt.want.outcomes) {
			t.Errorf("%s／available_outcomes：%v，預期 %v", name, got, tt.want.outcomes)
		}
	}
}

// 重複與已採取措施的通報可以當認定的來源，未違規、內容不足、濫報不行（和 Supporting 一致）。
func TestCiteAsSourceFollowsSupporting(t *testing.T) {
	for _, outcome := range OutcomeValues() {
		a := ReportFacts{Report: Report{Outcome: outcomePtr(outcome)}}.Availability()
		if blocked := len(a.CiteAsSource) > 0; blocked != outcome.Negates() {
			t.Errorf("%s：CiteAsSource 被擋=%v，Negates=%v", outcome, blocked, outcome.Negates())
		}
	}
	if a := (ReportFacts{Report: Report{}}).Availability(); len(a.CiteAsSource) != 0 {
		t.Errorf("沒有結果的通報可以當來源：%v", a.CiteAsSource)
	}
}

// 每個原因都有 zh-TW 的句子；不同原因是不同的句子（錯誤取第一個原因的句子，不能讓兩個原因說同一句）；不認得的原因沒有句子。
func TestEveryBlockHasADistinctSentence(t *testing.T) {
	seen := map[string]string{}
	for _, b := range ReportBlockValues() {
		msg := b.Text().Text
		if msg == "" {
			t.Errorf("ReportBlock %s 沒有句子", b)
		}
		// 店家已關閉：通報、案件與 CreateMeasure 是同一句（StoreActionUnavailableText），這是設計（建立措施被擋下時錯誤就是 CreateMeasure 的那一句）。
		if b == ReportBlockStoreClosed {
			if want := StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure).Text; msg != want {
				t.Errorf("ReportBlock STORE_CLOSED 是 %q，要與 CreateMeasure 的 STORE_CLOSED 同一句 %q", msg, want)
			}
			continue
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("ReportBlock %s 與 %s 是同一句：%q", b, prev, msg)
		}
		seen[msg] = "ReportBlock " + string(b)
	}
	for _, b := range CaseBlockValues() {
		for _, action := range []CaseAction{CaseActionDecide, CaseActionChangeSeverity, CaseActionLinkMeasure, CaseActionCreateMeasure, CaseActionCreateInquiry} {
			if b.Text(action).Text == "" {
				t.Errorf("CaseBlock %s（%s）沒有句子", b, action)
			}
		}
		msg := b.Text(CaseActionDecide).Text
		if b == CaseBlockStoreClosed {
			for _, action := range []CaseAction{CaseActionDecide, CaseActionCreateMeasure} {
				if want := StoreActionUnavailableText(StoreReasonStoreClosed, StoreActionCreateMeasure).Text; b.Text(action).Text != want {
					t.Errorf("CaseBlock STORE_CLOSED（%s）是 %q，要與 CreateMeasure 的 STORE_CLOSED 同一句 %q", action, b.Text(action).Text, want)
				}
			}
			continue
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("CaseBlock %s 與 %s 是同一句：%q", b, prev, msg)
		}
		seen[msg] = "CaseBlock " + string(b)
	}
	// 已結案：連結措施（從案件建立措施）的句子說明不能再從案件建立措施（#S31），與其他動作的「案件已結案。」不同。
	if link, other := CaseBlockClosed.Text(CaseActionLinkMeasure).Text, CaseBlockClosed.Text(CaseActionDecide).Text; link != "案件已結案，不能再從它建立措施。" || link == other {
		t.Errorf("CaseBlock CLOSED：連結措施 %q，其他動作 %q", link, other)
	}
	if ReportBlock("").Text().Text != "" || ReportBlock("UNKNOWN").Text().Text != "" || CaseBlock("").Text(CaseActionDecide).Text != "" || CaseBlock("UNKNOWN").Text(CaseActionCreateInquiry).Text != "" {
		t.Error("不認得的原因不得有句子")
	}
}
