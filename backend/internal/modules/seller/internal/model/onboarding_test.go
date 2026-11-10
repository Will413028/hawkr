package model

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var errRead = errors.New("來源讀取失敗")

// conds 是店家讀出的條件：kyc 與 transfers 是閘門的兩個來源欄，profile 決定店名、logo 與鄉鎮市區代碼是否都有。
func conds(kyc, transfers, profile bool) Conditions {
	return Conditions{Derived: true, Active: true, NotPendingDeletion: true, KYCApproved: kyc, TransfersActive: transfers, StoreName: profile, Logo: profile, TownCode: profile}
}

func storeWith(c Conditions) Store {
	return Store{Status: Active, Sanctions: NoSanctions(), Conditions: c}
}

func taskOf(t *testing.T, o Onboarding, id OnboardingTaskID) OnboardingTask {
	t.Helper()
	for _, task := range o.Tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("清單裡沒有任務 %s：%+v", id, o.Tasks)
	return OnboardingTask{}
}

// 每個任務的每個狀態與下一步：完成看店家讀出的條件，讀取結果只決定狀態句與下一步；讀取失敗只讓需要它的任務是 UNKNOWN。
// mutation 1：PLATFORM_REVIEW 的完成改看 KYC 讀取結果（Found 且 APPROVED）而不是 Conditions.KYCApproved → KYC 已核准但讀取失敗的列變成 UNKNOWN，失敗。
// mutation 2：PAYMENT_ACCOUNT 的完成改看帳戶狀態 ENABLED 而不是 TransfersActive 副本 → 「帳戶 ENABLED 但副本 false」的列變成 COMPLETED，失敗。
// mutation 3：KYC 讀取失敗時整個函式回空清單（模擬讓 RPC 失敗）→ 其他任務消失，失敗。
func TestOnboardingTaskStatesAndActions(t *testing.T) {
	t.Parallel()
	enabled := OnboardingPaymentRead{Found: true, Status: "ENABLED", CanStartOnboarding: true}
	cases := []struct {
		name       string
		c          Conditions
		kyc        OnboardingKYCRead
		pay        OnboardingPaymentRead
		profile    [2]any
		review     [2]any
		payment    [2]any
		wantOthers bool
	}{
		{"全部完成", conds(true, true, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, false},
		{"全部完成但兩個來源都讀不到：完成的任務不需要讀取結果", conds(true, true, true), OnboardingKYCRead{Err: errRead}, OnboardingPaymentRead{Err: errRead},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, false},
		{"商店資料缺項", conds(true, true, false), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingActionRequired, OnboardingActionEditProfile}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, false},
		{"沒有 KYC 申請", conds(false, false, true), OnboardingKYCRead{}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 草稿", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusDraft}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 審核中", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusSubmitted}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 被退回", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusRejected}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 已取消", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusCancelled}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 讀取失敗：只有平台審核是 UNKNOWN，收款帳戶仍是 LOCKED", conds(false, false, true), OnboardingKYCRead{Err: errRead}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingUnknown, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
		{"KYC 核准、還沒有收款帳戶列", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}, false},
		{"收款帳戶建立中", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "NOT_CREATED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}, false},
		{"收款帳戶審核中", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "UNDER_REVIEW"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}, false},
		{"收款帳戶待設定、可以開通", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED", CanStartOnboarding: true},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionStartPaymentOnboarding}, false},
		{"收款帳戶待設定、沒有設定收款開通", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionNone}, false},
		{"收款帳戶被停用", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "DISABLED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionContactSupport}, false},
		{"帳戶 ENABLED 但副本還沒跟上：不算完成", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}, false},
		{"收款帳戶讀取失敗：只有收款帳戶是 UNKNOWN", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Err: errRead},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingUnknown, OnboardingActionNone}, false},
		{"沒有讀過條件：fail-closed", Conditions{}, OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingActionRequired, OnboardingActionEditProfile}, [2]any{OnboardingInProgress, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: storeWith(tc.c), KYC: tc.kyc, Payment: tc.pay})
			wantIDs := []OnboardingTaskID{OnboardingProfile, OnboardingPlatformReview, OnboardingPaymentAccount}
			var gotIDs []OnboardingTaskID
			for _, task := range o.Tasks {
				gotIDs = append(gotIDs, task.ID)
				if task.State == "" || task.Action == "" || task.Title.Text == "" || task.Status.Text == "" {
					t.Errorf("任務 %s 有沒設的欄位：%+v", task.ID, task)
				}
			}
			if !slices.Equal(gotIDs, wantIDs) {
				t.Fatalf("任務與順序=%v，要 %v", gotIDs, wantIDs)
			}
			for _, w := range []struct {
				id   OnboardingTaskID
				want [2]any
			}{{OnboardingProfile, tc.profile}, {OnboardingPlatformReview, tc.review}, {OnboardingPaymentAccount, tc.payment}} {
				got := taskOf(t, o, w.id)
				if got.State != w.want[0] || got.Action != w.want[1] {
					t.Errorf("%s：state=%s action=%s，要 %v", w.id, got.State, got.Action, w.want)
				}
			}
			if o.Complete != tc.c.OnboardingComplete() {
				t.Errorf("complete=%v，閘門=%v", o.Complete, tc.c.OnboardingComplete())
			}
		})
	}
}

// UNKNOWN 只表示來源讀取失敗：沒有任何讀取失敗時，任何輸入都不會產生 UNKNOWN，也不會產生沒設的 state 或 action。
func TestOnboardingUnknownOnlyOnReadFailure(t *testing.T) {
	t.Parallel()
	kycs := []OnboardingKYCRead{{}, {Found: true, Status: KYCStatusDraft}, {Found: true, Status: KYCStatusSubmitted}, {Found: true, Status: KYCStatusApproved},
		{Found: true, Status: KYCStatusRejected}, {Found: true, Status: KYCStatusCancelled}, {Found: true, Status: "SOMETHING_NEW"}}
	pays := []OnboardingPaymentRead{{}, {Found: true, Status: "NOT_CREATED"}, {Found: true, Status: "UNDER_REVIEW"}, {Found: true, Status: "ACTION_REQUIRED"},
		{Found: true, Status: "ACTION_REQUIRED", CanStartOnboarding: true}, {Found: true, Status: "ENABLED"}, {Found: true, Status: "DISABLED"}, {Found: true, Status: "SOMETHING_NEW"}}
	for _, kyc := range []bool{false, true} {
		for _, transfers := range []bool{false, true} {
			for _, profile := range []bool{false, true} {
				for _, k := range kycs {
					for _, p := range pays {
						o := OnboardingTasksOf(uitext.EN, OnboardingInput{Store: storeWith(conds(kyc, transfers, profile)), KYC: k, Payment: p})
						for _, task := range o.Tasks {
							if task.State == OnboardingUnknown {
								t.Fatalf("沒有讀取失敗卻是 UNKNOWN：%+v（kyc=%v transfers=%v）", task, kyc, transfers)
							}
							if !slices.Contains(OnboardingTaskStateValues(), task.State) || !slices.Contains(OnboardingActionValues(), task.Action) {
								t.Fatalf("不在值域的 state 或 action：%+v", task)
							}
						}
					}
				}
			}
		}
	}
}

// 沒有任務處理的公開條件才出現 OTHER_REQUIREMENT；任務與公告頁處理的條件（含停權、關店、等待刪除）都不出現。
// mutation：已處理的清單拿掉 NOT_SUSPENDED → 停權時出現「其他公開條件」，失敗。
func TestOnboardingOtherRequirementOnlyForUnhandledRequirements(t *testing.T) {
	t.Parallel()
	in := OnboardingInput{Store: storeWith(conds(true, true, true)), KYC: OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, Payment: OnboardingPaymentRead{Found: true, Status: "ENABLED"}}
	if o := onboardingTasks(uitext.ZhTW, in, Requirements()); len(o.Tasks) != 3 {
		t.Fatalf("已處理的條件全缺時任務是 %d 項，要 3 項（沒有其他公開條件）", len(o.Tasks))
	}
	o := onboardingTasks(uitext.ZhTW, in, []Requirement{RequirementNotSuspended, "SOMETHING_NEW"})
	if len(o.Tasks) != 4 {
		t.Fatalf("有沒處理的條件時任務是 %d 項，要 4 項", len(o.Tasks))
	}
	other := taskOf(t, o, OnboardingOtherRequirement)
	if other.State != OnboardingActionRequired || other.Action != OnboardingActionContactSupport || o.Tasks[3].ID != OnboardingOtherRequirement {
		t.Fatalf("其他公開條件：%+v，要 ACTION_REQUIRED、CONTACT_SUPPORT、排在最後", other)
	}
	suspended := Store{Status: Active, Sanctions: sanctionsOf(true), Conditions: conds(true, true, true)}
	if o := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: suspended}); len(o.Tasks) != 3 {
		t.Fatalf("停權的店家出現了其他公開條件：%d 項", len(o.Tasks))
	}
}

// 開通閘門與能力推導同一組來源：兩項閘門任務都 COMPLETED ⇔ Complete ⇔ CapabilitiesOf 的 PUBLIC_LISTING 與 SHIPPING_SETUP 都沒缺 ONBOARDING_COMPLETE。
// （SQL 的 view onboarding_complete 是同一個 AND，由 integration 的 TestOnboardingTasksAgreeWithTheGateAndTheView 鎖住。）
// mutation：Complete 改成只看 KYCApproved → transfers 為 false 的列失敗。
func TestOnboardingGateTasksAgreeWithCapabilities(t *testing.T) {
	t.Parallel()
	for _, kyc := range []bool{false, true} {
		for _, transfers := range []bool{false, true} {
			s := storeWith(conds(kyc, transfers, true))
			o := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: s, KYC: OnboardingKYCRead{Found: kyc, Status: KYCStatusApproved}, Payment: OnboardingPaymentRead{Found: true, Status: "ENABLED"}})
			gateTasksDone := taskOf(t, o, OnboardingPlatformReview).State == OnboardingCompleted && taskOf(t, o, OnboardingPaymentAccount).State == OnboardingCompleted
			missingGate := false
			for _, st := range CapabilitiesOf(s) {
				if st.Capability == CapShippingSetup {
					missingGate = slices.Contains(st.Missing, RequirementOnboardingComplete)
				}
			}
			if gateTasksDone != o.Complete || o.Complete == missingGate {
				t.Errorf("kyc=%v transfers=%v：兩項閘門任務完成=%v、complete=%v、能力缺閘門=%v", kyc, transfers, gateTasksDone, o.Complete, missingGate)
			}
		}
	}
}

// 句子依語系：en 取英文，缺字時由產生器補 zh-TW（Locale 標示實際用的語系）；商店資料缺項的名詞取共用目錄、以語系的分隔詞連接。
func TestOnboardingProfileMissingNamesTheMissingItems(t *testing.T) {
	t.Parallel()
	c := conds(true, true, false)
	zh := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: storeWith(c)})
	got := taskOf(t, zh, OnboardingProfile).Status.Text
	if want := "還缺少：店名、logo、所在地。"; got != want {
		t.Errorf("zh-TW 商店資料缺項句=%q，要 %q", got, want)
	}
	c.Logo, c.StoreName, c.TownCode = true, true, false
	partial := taskOf(t, OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: storeWith(c)}), OnboardingProfile).Status.Text
	if strings.Contains(partial, "店名") || strings.Contains(partial, "logo") || !strings.Contains(partial, "所在地") {
		t.Errorf("只缺所在地時的句子=%q", partial)
	}
	en := taskOf(t, OnboardingTasksOf(uitext.EN, OnboardingInput{Store: storeWith(conds(true, true, false))}), OnboardingProfile)
	if en.Status.Locale != uitext.EN || !strings.HasPrefix(en.Status.Text, "Still missing: ") || en.Title.Text != "Store profile" {
		t.Errorf("en 的任務：%+v", en)
	}
}
