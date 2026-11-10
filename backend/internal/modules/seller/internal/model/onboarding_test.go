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
		name    string
		c       Conditions
		kyc     OnboardingKYCRead
		pay     OnboardingPaymentRead
		profile [2]any
		review  [2]any
		payment [2]any
	}{
		{"全部完成", conds(true, true, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}},
		{"全部完成但兩個來源都讀不到：完成的任務不需要讀取結果", conds(true, true, true), OnboardingKYCRead{Err: errRead}, OnboardingPaymentRead{Err: errRead},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}},
		{"商店資料缺項", conds(true, true, false), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingActionRequired, OnboardingActionEditProfile}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}},
		{"沒有 KYC 申請", conds(false, false, true), OnboardingKYCRead{}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 草稿", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusDraft}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 審核中", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusSubmitted}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 被退回", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusRejected}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 已取消", conds(false, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusCancelled}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionSubmitCertification}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 讀取失敗：只有平台審核是 UNKNOWN，收款帳戶仍是 LOCKED", conds(false, false, true), OnboardingKYCRead{Err: errRead}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingUnknown, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}},
		{"KYC 核准、還沒有收款帳戶列", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Configured: true},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}},
		{"收款帳戶建立中", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "NOT_CREATED", Configured: true},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}},
		{"收款帳戶建立失敗：等營運重試，賣家聯絡客服", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "NOT_CREATED", Configured: true, CreateFailed: true},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionContactSupport}},
		{"沒有設定收款開通、沒有帳戶列：沒有重新整理這個動作", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionNone}},
		{"沒有設定收款開通、帳戶列停在建立中", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "NOT_CREATED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionNone}},
		{"收款帳戶審核中", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "UNDER_REVIEW"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}},
		{"收款帳戶待設定、可以開通", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED", CanStartOnboarding: true},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionStartPaymentOnboarding}},
		{"收款帳戶待設定、沒有設定收款開通", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionNone}},
		{"收款帳戶被停用", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "DISABLED"},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingActionRequired, OnboardingActionContactSupport}},
		{"帳戶 ENABLED 但副本還沒跟上：不算完成", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionRefreshPaymentAccount}},
		{"收款帳戶讀取失敗：只有收款帳戶是 UNKNOWN", conds(true, false, true), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Err: errRead},
			[2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingCompleted, OnboardingActionNone}, [2]any{OnboardingUnknown, OnboardingActionNone}},
		{"沒有讀過條件：fail-closed", Conditions{}, OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, enabled,
			[2]any{OnboardingActionRequired, OnboardingActionNone}, [2]any{OnboardingInProgress, OnboardingActionNone}, [2]any{OnboardingLocked, OnboardingActionNone}},
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

// 店家沒有 CapStorefrontWrite（停權、等待刪除）時，後續 RPC 會被擋下的下一步是 NONE，狀態與句子不變；聯絡客服不需要能力，照舊。判斷與能力推導同一個 Store.Can。
// mutation 1：OnboardingTasksOf 不看 Store.Can（停權時照給下一步）→ 停權與等待刪除的列失敗。
// mutation 2：聯絡客服也被清成 NONE → 建立失敗與被停用的列失敗。
// mutation 3：把狀態與句子也改掉（例如清成 LOCKED）→ 與能完成寫入的同一份輸入比對失敗。
func TestOnboardingActionsAreNoneWhenTheStoreCannotWrite(t *testing.T) {
	t.Parallel()
	suspended := Store{Status: Active, Sanctions: SanctionedBy(EffectiveMeasure{}), Conditions: conds(false, false, false)}
	pending := Store{Status: PendingDeletion, Sanctions: NoSanctions(), Conditions: Conditions{Derived: true, Active: true, NotPendingDeletion: false, StoreName: false}}
	withKYC := func(s Store) Store { s.Conditions.KYCApproved = true; return s }
	cases := []struct {
		name  string
		store Store
		kyc   OnboardingKYCRead
		pay   OnboardingPaymentRead
	}{
		{"停權、缺資料、沒有申請", suspended, OnboardingKYCRead{}, OnboardingPaymentRead{}},
		{"停權、KYC 被退回", suspended, OnboardingKYCRead{Found: true, Status: KYCStatusRejected}, OnboardingPaymentRead{}},
		{"停權、帳戶待設定", withKYC(suspended), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED", CanStartOnboarding: true, Configured: true}},
		{"停權、帳戶審核中", withKYC(suspended), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "UNDER_REVIEW", Configured: true}},
		{"等待刪除、缺資料、沒有申請", pending, OnboardingKYCRead{}, OnboardingPaymentRead{}},
		{"等待刪除、帳戶待設定", withKYC(pending), OnboardingKYCRead{Found: true, Status: KYCStatusApproved}, OnboardingPaymentRead{Found: true, Status: "ACTION_REQUIRED", CanStartOnboarding: true, Configured: true}},
	}
	for _, tc := range cases {
		if tc.store.Can(CapStorefrontWrite) {
			t.Fatalf("%s：測試的店家其實有 CapStorefrontWrite", tc.name)
		}
		o := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: tc.store, KYC: tc.kyc, Payment: tc.pay})
		for _, task := range o.Tasks {
			if task.Action != OnboardingActionNone {
				t.Errorf("%s：%s 的下一步是 %s，要 NONE", tc.name, task.ID, task.Action)
			}
			if task.State == "" || task.Status.Text == "" {
				t.Errorf("%s：%s 的狀態或句子被清掉：%+v", tc.name, task.ID, task)
			}
		}
		// 狀態與句子與「能寫」的同一份輸入相同。
		writable := tc.store
		writable.Sanctions, writable.Status = NoSanctions(), Active
		writable.Conditions.Active, writable.Conditions.NotPendingDeletion = true, true
		if !writable.Can(CapStorefrontWrite) {
			t.Fatalf("%s：對照的店家沒有 CapStorefrontWrite", tc.name)
		}
		want := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: writable, KYC: tc.kyc, Payment: tc.pay})
		for i, task := range o.Tasks {
			if task.State != want.Tasks[i].State || task.Status != want.Tasks[i].Status {
				t.Errorf("%s：%s 的狀態或句子與能寫入時不同：%+v vs %+v", tc.name, task.ID, task, want.Tasks[i])
			}
		}
	}
	// 聯絡客服不需要賣家能力：停權的店家照舊拿得到。
	dis := OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: withKYC(suspended), KYC: OnboardingKYCRead{Found: true, Status: KYCStatusApproved},
		Payment: OnboardingPaymentRead{Found: true, Status: "DISABLED", Configured: true}})
	if got := taskOf(t, dis, OnboardingPaymentAccount).Action; got != OnboardingActionContactSupport {
		t.Errorf("停權、帳戶被停用：下一步 %s，要 CONTACT_SUPPORT", got)
	}
}

// UNKNOWN 只表示來源讀取失敗：沒有任何讀取失敗時，任何輸入都不會產生 UNKNOWN，也不會產生沒設的 state 或 action。
func TestOnboardingUnknownOnlyOnReadFailure(t *testing.T) {
	t.Parallel()
	kycs := []OnboardingKYCRead{{}, {Found: true, Status: KYCStatusDraft}, {Found: true, Status: KYCStatusSubmitted}, {Found: true, Status: KYCStatusApproved},
		{Found: true, Status: KYCStatusRejected}, {Found: true, Status: KYCStatusCancelled}, {Found: true, Status: "SOMETHING_NEW"}}
	pays := []OnboardingPaymentRead{{}, {Configured: true}, {Found: true, Status: "NOT_CREATED", CreateFailed: true, Configured: true}, {Found: true, Status: "NOT_CREATED"}, {Found: true, Status: "UNDER_REVIEW"}, {Found: true, Status: "ACTION_REQUIRED"},
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

// 公開曝光的每個條件都對應到恰好一種處理者：一個以上的開通任務，或公告頁（STORE_ACTIVE、NOT_SUSPENDED、NOT_PENDING_DELETION）；兩者不並存。
// 對照表 requirementCovers 就是商店資料任務挑條件用的表（profileTask 經 requirementsOf 讀它），不是平行的副本。
// mutation：CapPublicListing 的 needs 加一個沒有在 requirementCovers 的新條件（並列入 Requirements()）→ 這個測試失敗。
func TestEveryPublicListingRequirementIsCoveredByATaskOrTheNoticePage(t *testing.T) {
	t.Parallel()
	var notice []Requirement
	for _, r := range Needs(CapPublicListing) {
		cover, ok := requirementCovers[r]
		if !ok {
			t.Errorf("公開條件 %s 沒有對應到任務或公告頁", r)
			continue
		}
		if (len(cover.tasks) > 0) == cover.notice {
			t.Errorf("公開條件 %s 的處理者要是任務或公告頁其中之一：%+v", r, cover)
		}
		for _, id := range cover.tasks {
			if !slices.Contains(OnboardingTaskIDValues(), id) {
				t.Errorf("公開條件 %s 對應到不存在的任務 %s", r, id)
			}
		}
		if cover.notice {
			notice = append(notice, r)
		}
	}
	if want := []Requirement{RequirementActive, RequirementNotSuspended, RequirementNotPendingDeletion}; !slices.Equal(notice, want) {
		t.Errorf("公告頁處理的條件=%v，要 %v", notice, want)
	}
	for r := range requirementCovers {
		if !slices.Contains(Requirements(), r) {
			t.Errorf("對照表有不是公開條件的 %s", r)
		}
	}
	// 商店資料任務缺哪些條件就是表裡歸它的條件：缺任何一項都不能是 COMPLETED。
	for _, r := range requirementsOf(OnboardingProfile) {
		c := conds(true, true, true)
		clear, ok := map[Requirement]func(*Conditions){
			RequirementStoreName: func(c *Conditions) { c.StoreName = false },
			RequirementLogo:      func(c *Conditions) { c.Logo = false },
			RequirementTownCode:  func(c *Conditions) { c.TownCode = false },
		}[r]
		if !ok {
			t.Fatalf("商店資料任務的條件 %s 沒有對應的測試輸入", r)
		}
		clear(&c)
		if got := taskOf(t, OnboardingTasksOf(uitext.ZhTW, OnboardingInput{Store: storeWith(c)}), OnboardingProfile); got.State != OnboardingActionRequired {
			t.Errorf("缺 %s 時商店資料是 %s，要 ACTION_REQUIRED", r, got.State)
		}
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
