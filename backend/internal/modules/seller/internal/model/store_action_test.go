package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// storeIn 造一個店家：條件依 status 推導（與 seller.store_requirement_flags 的 active／not_pending_deletion 同義），suspended 決定停權。
func storeIn(status Status, suspended bool) Store {
	s := Store{Status: status, Sanctions: NoSanctions(), Conditions: Conditions{
		Derived: true, Active: status != Closed, NotPendingDeletion: status != PendingDeletion,
	}}
	if suspended {
		s.Sanctions = SanctionedBy(EffectiveMeasure{Kind: MeasureSuspension})
	}
	return s
}

var allStatuses = []Status{Active, PendingDeletion, Closed}

func reasonsEqual(got []StoreActionUnavailableReason, want ...StoreActionUnavailableReason) bool {
	return slices.Equal(got, want)
}

// 表上 blocking 的組合都有以「。」結尾的 zh-TW 句子；idempotent 與表外的組合是零值。
func TestStoreActionTextCoversExactlyTheBlockingCombinations(t *testing.T) {
	for _, a := range StoreActionValues() {
		listed := map[StoreActionUnavailableReason]bool{}
		for _, rule := range storeActionRules[a] {
			listed[rule.Reason] = true
			got := StoreActionUnavailableText(rule.Reason, a)
			switch {
			case rule.Text != nil:
				if got.Text == "" || !strings.HasSuffix(got.Text, "。") || got.Locale != uitext.ZhTW {
					t.Errorf("%s/%s 沒有完整的 zh-TW 句子：%+v", a, rule.Reason, got)
				}
			default:
				if got != (uitext.Message{}) {
					t.Errorf("%s/%s 是 idempotent，不該有句子：%+v", a, rule.Reason, got)
				}
			}
		}
		for _, r := range StoreActionUnavailableReasonValues() {
			if !listed[r] && StoreActionUnavailableText(r, a) != (uitext.Message{}) {
				t.Errorf("%s/%s 不在表上，卻有句子", a, r)
			}
		}
	}
	if StoreActionUnavailableText("WEIRD", StoreActionDelegate) != (uitext.Message{}) || StoreActionUnavailableText(StoreReasonStoreClosed, "WEIRD") != (uitext.Message{}) {
		t.Error("不認得的原因或動作不得有句子")
	}
	if !StoreActionIdempotent(StoreActionRebindPayUni, StoreReasonPayUniBindInProgress) || StoreActionIdempotent(StoreActionRebindPayUni, StoreReasonPayUniBound) {
		t.Error("只有重新綁定的 PAYUNI_BIND_IN_PROGRESS 是 idempotent")
	}
}

// DELEGATE 的條件與狀態不一致時（讀店家的條件說已關閉，Status 卻是營運中）：原因跟著能力缺的條件走，不是看 Status。
// 這個案例在「DELEGATE 只看 Status」的突變下轉紅（Status 是 Active 會得到空清單）。
func TestDelegateReasonsFollowTheConditionsNotTheStatus(t *testing.T) {
	s := Store{Status: Active, Sanctions: NoSanctions(), Conditions: Conditions{Derived: true, Active: false, NotPendingDeletion: true}}
	if got := s.ActionUnavailableReasons(StoreActionDelegate); !reasonsEqual(got, StoreReasonStoreClosed) || s.Can(CapStorefrontWrite) {
		t.Fatalf("Active 但條件說已關閉：%v", got)
	}
	s = Store{Status: Closed, Sanctions: NoSanctions(), Conditions: Conditions{Derived: true, Active: true, NotPendingDeletion: true}}
	if got := s.ActionUnavailableReasons(StoreActionDelegate); len(got) != 0 || !s.Can(CapStorefrontWrite) {
		t.Fatalf("Closed 但條件都滿足：%v", got)
	}
}

// 句子逐字等於核可的文案（N1–N8、N10、N11、N13–N15、N17、N18 與 E 類加句號）。
func TestStoreActionTextIsTheApprovedCopy(t *testing.T) {
	for _, tc := range []struct {
		reason StoreActionUnavailableReason
		action StoreAction
		want   string
	}{
		{StoreReasonStoreClosed, StoreActionCreateMeasure, "店家已關閉，不能建立警告、停權或風險措施。"},
		{StoreReasonStoreClosed, StoreActionDelegate, "店家已關閉，無法代行。"},
		{StoreReasonStorePendingDeletion, StoreActionDelegate, "店家等待刪除，無法代行。"},
		{StoreReasonAccountSuspended, StoreActionDelegate, "帳號停權中，無法代行。"},
		{StoreReasonStoreClosed, StoreActionApproveKYC, "店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。"},
		{StoreReasonStoreClosed, StoreActionRejectKYC, "店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。"},
		{StoreReasonStoreClosed, StoreActionRequeryKYC, "店家已關閉，這份商家認證申請暫停審核，店家恢復營業後才能審核或重新查詢。"},
		{StoreReasonStorePendingDeletion, StoreActionApproveKYC, "店家等待刪除，這份商家認證申請不再審核。"},
		{StoreReasonStorePendingDeletion, StoreActionRejectKYC, "店家等待刪除，這份商家認證申請不再審核。"},
		{StoreReasonStorePendingDeletion, StoreActionRequeryKYC, "店家等待刪除，這份商家認證申請不再審核。"},
		{StoreReasonAccountSuspended, StoreActionApproveKYC, "帳號停權中，停權解除之後才能核准商家認證。"},
		{StoreReasonKYCNotSubmitted, StoreActionApproveKYC, "只有審核中的申請能核准或退回。"},
		{StoreReasonKYCNotSubmitted, StoreActionRejectKYC, "只有審核中的申請能核准或退回。"},
		{StoreReasonKYCNotSubmitted, StoreActionRequeryKYC, "只有審核中的申請能重新查詢登記資料。"},
		{StoreReasonKYCNotBusiness, StoreActionRequeryKYC, "只有公司型的申請有登記資料可以查詢。"},
		{StoreReasonPayUniMerchantTerminal, StoreActionRebindPayUni, "PayUni 端的商店已是終止狀態，這家店無法再綁定；賣家要重新入駐才能收款。"},
		{StoreReasonStoreClosed, StoreActionRebindPayUni, "店家已關閉，不能重新綁定 PayUni。"},
		{StoreReasonStorePendingDeletion, StoreActionRebindPayUni, "店家等待刪除，不能重新綁定 PayUni。"},
		{StoreReasonKYCNotApproved, StoreActionRebindPayUni, "賣家正在修改商家認證，重新核准後會自動重新綁定。"},
		{StoreReasonPayUniBound, StoreActionRebindPayUni, "只有綁定失敗的註冊能重新綁定。"},
	} {
		if got := StoreActionUnavailableText(tc.reason, tc.action).Text; got != tc.want {
			t.Errorf("%s/%s = %q，期望 %q", tc.reason, tc.action, got, tc.want)
		}
	}
}

// DELEGATE 的原因與 CapStorefrontWrite 缺的條件在每個狀態組合上等價：能力缺什麼，原因就是什麼，順序同 Requirements()。
// 期望值由能力表（needs）逐條對應，不經 DELEGATE 的規則表，所以把規則表改成不看停權時這個測試會失敗；「只看 Status」的突變由 TestDelegateReasonsFollowTheConditionsNotTheStatus 抓（這裡 storeIn 的條件由 Status 推導，兩者一致）。
func TestDelegateReasonsAreTheMissingStorefrontWriteRequirements(t *testing.T) {
	for _, status := range allStatuses {
		for _, suspended := range []bool{false, true} {
			s := storeIn(status, suspended)
			var want []StoreActionUnavailableReason
			for _, r := range missingFor(CapStorefrontWrite, s) {
				switch r {
				case RequirementActive:
					want = append(want, StoreReasonStoreClosed)
				case RequirementNotPendingDeletion:
					want = append(want, StoreReasonStorePendingDeletion)
				case RequirementNotSuspended:
					want = append(want, StoreReasonAccountSuspended)
				default:
					t.Fatalf("CapStorefrontWrite 多了條件 %s，DELEGATE 沒有對應的原因", r)
				}
			}
			got := s.ActionUnavailableReasons(StoreActionDelegate)
			if (len(got) == 0) != s.Can(CapStorefrontWrite) {
				t.Errorf("%s suspended=%v：原因 %v 與 Can(CapStorefrontWrite)=%v 不一致", status, suspended, got, s.Can(CapStorefrontWrite))
			}
			// 集合相同即可：原因的先後是 DELEGATE 規則表的順序（已關閉、等待刪除、停權），不是 Requirements() 的順序。
			if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
				t.Errorf("%s suspended=%v：原因 %v，缺的條件對應 %v", status, suspended, got, want)
			}
		}
	}
	// 停權的已關閉店家：第一個是 STORE_CLOSED。
	if got := storeIn(Closed, true).ActionUnavailableReasons(StoreActionDelegate); got[0] != StoreReasonStoreClosed {
		t.Errorf("已關閉而且停權：%v", got)
	}
}

func TestCreateMeasureIsBlockedOnlyByAClosedStore(t *testing.T) {
	for _, status := range allStatuses {
		for _, suspended := range []bool{false, true} {
			got := storeIn(status, suspended).ActionUnavailableReasons(StoreActionCreateMeasure)
			if status == Closed && !reasonsEqual(got, StoreReasonStoreClosed) || status != Closed && len(got) != 0 {
				t.Errorf("%s suspended=%v：%v", status, suspended, got)
			}
		}
	}
	if len(storeIn(Active, false).ActionUnavailableReasons(StoreActionApproveKYC)) != 0 {
		t.Error("Store.ActionUnavailableReasons 只回答 CREATE_MEASURE 與 DELEGATE")
	}
}

// 核准、退回、重新查詢：每個狀態組合的第一個原因，以及停權只擋核准。
func TestKYCReviewReasonsPerState(t *testing.T) {
	submitted := KYCApplicationState{Status: KYCStatusSubmitted, SellerType: KYCCompany}
	for _, tc := range []struct {
		name         string
		store        Store
		app          KYCApplicationState
		approve      []StoreActionUnavailableReason
		reject       []StoreActionUnavailableReason
		requery      []StoreActionUnavailableReason
		wantApproved bool
	}{
		{"營運中、審核中", storeIn(Active, false), submitted, nil, nil, nil, true},
		{"已關閉", storeIn(Closed, false), submitted,
			[]StoreActionUnavailableReason{StoreReasonStoreClosed}, []StoreActionUnavailableReason{StoreReasonStoreClosed}, []StoreActionUnavailableReason{StoreReasonStoreClosed}, false},
		{"等待刪除", storeIn(PendingDeletion, false), submitted,
			[]StoreActionUnavailableReason{StoreReasonStorePendingDeletion}, []StoreActionUnavailableReason{StoreReasonStorePendingDeletion}, []StoreActionUnavailableReason{StoreReasonStorePendingDeletion}, false},
		{"停權：核准不行、退回與重新查詢照舊", storeIn(Active, true), submitted,
			[]StoreActionUnavailableReason{StoreReasonAccountSuspended}, nil, nil, false},
		{"已關閉而且停權：第一個是已關閉，停權不擋退回", storeIn(Closed, true), submitted,
			[]StoreActionUnavailableReason{StoreReasonStoreClosed, StoreReasonAccountSuspended}, []StoreActionUnavailableReason{StoreReasonStoreClosed}, []StoreActionUnavailableReason{StoreReasonStoreClosed}, false},
		{"申請不在審核中：結構性原因在前", storeIn(Closed, false), KYCApplicationState{Status: KYCStatusApproved, SellerType: KYCIndividual},
			[]StoreActionUnavailableReason{StoreReasonKYCNotSubmitted, StoreReasonStoreClosed}, []StoreActionUnavailableReason{StoreReasonKYCNotSubmitted, StoreReasonStoreClosed},
			[]StoreActionUnavailableReason{StoreReasonKYCNotSubmitted, StoreReasonKYCNotBusiness, StoreReasonStoreClosed}, false},
		{"個人型：不能重新查詢", storeIn(Active, false), KYCApplicationState{Status: KYCStatusSubmitted, SellerType: KYCIndividual},
			nil, nil, []StoreActionUnavailableReason{StoreReasonKYCNotBusiness}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := KYCActionUnavailableReasons(tc.store, tc.app, StoreActionApproveKYC); !slices.Equal(got, tc.approve) {
				t.Errorf("approve=%v 期望 %v", got, tc.approve)
			}
			if got := KYCActionUnavailableReasons(tc.store, tc.app, StoreActionRejectKYC); !slices.Equal(got, tc.reject) {
				t.Errorf("reject=%v 期望 %v", got, tc.reject)
			}
			if got := KYCActionUnavailableReasons(tc.store, tc.app, StoreActionRequeryKYC); !slices.Equal(got, tc.requery) {
				t.Errorf("requery=%v 期望 %v", got, tc.requery)
			}
		})
	}
	if KYCActionUnavailableReasons(storeIn(Active, false), submitted, StoreActionDelegate) != nil {
		t.Error("不是 KYC 的動作回 nil")
	}
}

// 重新綁定：第一個原因的順序（結構性的在後、店家狀態在前）、idempotent、申請不是已核准。
func TestRebindReasonsFirstReasonPerState(t *testing.T) {
	reg := func(bind PayUniBindStatus, terminal bool) PayUniRegistration {
		return PayUniRegistration{BindStatus: bind, NeedsReonboarding: terminal}
	}
	for _, tc := range []struct {
		name  string
		store Store
		reg   PayUniRegistration
		app   KYCStatus
		want  []StoreActionUnavailableReason
	}{
		{"FAILED、已核准、營運中：可以", storeIn(Active, false), reg(BindFailed, false), KYCStatusApproved, nil},
		{"已關閉而且 PENDING：第一個是已關閉，不是 idempotent", storeIn(Closed, false), reg(BindPending, false), KYCStatusApproved, []StoreActionUnavailableReason{StoreReasonStoreClosed, StoreReasonPayUniBindInProgress}},
		{"PENDING：只有 idempotent 的原因", storeIn(Active, false), reg(BindPending, false), KYCStatusApproved, []StoreActionUnavailableReason{StoreReasonPayUniBindInProgress}},
		{"終止而且已關閉：第一個是終止", storeIn(Closed, false), reg(BindFailed, true), KYCStatusApproved, []StoreActionUnavailableReason{StoreReasonPayUniMerchantTerminal, StoreReasonStoreClosed}},
		{"等待刪除", storeIn(PendingDeletion, false), reg(BindFailed, false), KYCStatusApproved, []StoreActionUnavailableReason{StoreReasonStorePendingDeletion}},
		{"BOUND 而且申請是草稿：第一個是結構性的 PAYUNI_BOUND", storeIn(Active, false), reg(BindBound, false), KYCStatusDraft, []StoreActionUnavailableReason{StoreReasonPayUniBound, StoreReasonKYCNotApproved}},
		{"FAILED 而且申請是草稿：只有 KYC_NOT_APPROVED", storeIn(Active, false), reg(BindFailed, false), KYCStatusDraft, []StoreActionUnavailableReason{StoreReasonKYCNotApproved}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RebindUnavailableReasons(tc.store, tc.reg, tc.app); !slices.Equal(got, tc.want) {
				t.Errorf("got %v 期望 %v", got, tc.want)
			}
		})
	}
	for _, app := range []KYCStatus{KYCStatusDraft, KYCStatusSubmitted, KYCStatusRejected, KYCStatusCancelled} {
		if got := RebindUnavailableReasons(storeIn(Active, false), reg(BindFailed, false), app); !reasonsEqual(got, StoreReasonKYCNotApproved) {
			t.Errorf("申請 %s：%v", app, got)
		}
	}
}

// 錯誤取第一個原因的句子；清單是空或第一個原因沒有句子是內部錯誤（不是 failed_precondition，也沒有通用句）。
func TestErrStoreActionUnavailable(t *testing.T) {
	err := ErrStoreActionUnavailable([]StoreActionUnavailableReason{StoreReasonStoreClosed, StoreReasonStorePendingDeletion}, StoreActionDelegate)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || err == nil || !strings.Contains(err.Error(), "店家已關閉，無法代行。") {
		t.Fatalf("err=%v", err)
	}
	for _, bad := range [][]StoreActionUnavailableReason{nil, {StoreReasonPayUniBindInProgress}} {
		got := ErrStoreActionUnavailable(bad, StoreActionRebindPayUni)
		if got == nil || apperr.KindOf(got) == apperr.KindFailedPrecondition {
			t.Errorf("%v：%v", bad, got)
		}
	}
}
