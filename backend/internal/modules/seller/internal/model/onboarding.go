package model

import (
	"slices"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// OnboardingTaskID 是開通任務的種類，值＝proto OnboardingTaskId 的值名去掉型別前綴。
type OnboardingTaskID string

const (
	// OnboardingProfile 商店資料：店名、logo 與鄉鎮市區代碼都有值。
	OnboardingProfile OnboardingTaskID = "PROFILE"
	// OnboardingPlatformReview 平台審核：KYC 申請核准。
	OnboardingPlatformReview OnboardingTaskID = "PLATFORM_REVIEW"
	// OnboardingPaymentAccount 收款帳戶：轉帳能力啟用（seller 自存的副本）。
	OnboardingPaymentAccount OnboardingTaskID = "PAYMENT_ACCOUNT"
)

// OnboardingTaskIDValues 回傳全部任務種類，依顯示順序。
func OnboardingTaskIDValues() []OnboardingTaskID {
	return []OnboardingTaskID{OnboardingProfile, OnboardingPlatformReview, OnboardingPaymentAccount}
}

// OnboardingTaskState 是開通任務的狀態（只供 client 分支），值＝proto OnboardingTaskState 的值名去掉型別前綴。
type OnboardingTaskState string

const (
	// OnboardingCompleted 已完成。
	OnboardingCompleted OnboardingTaskState = "COMPLETED"
	// OnboardingActionRequired 需要賣家動作。
	OnboardingActionRequired OnboardingTaskState = "ACTION_REQUIRED"
	// OnboardingInProgress 進行中，賣家沒有要做的事。
	OnboardingInProgress OnboardingTaskState = "IN_PROGRESS"
	// OnboardingLocked 前一項還沒完成，這一項尚未開放。
	OnboardingLocked OnboardingTaskState = "LOCKED"
	// OnboardingUnknown 這一項需要的來源讀取失敗。只有這一種情況會是 UNKNOWN。
	OnboardingUnknown OnboardingTaskState = "UNKNOWN"
)

// OnboardingTaskStateValues 回傳全部狀態。
func OnboardingTaskStateValues() []OnboardingTaskState {
	return []OnboardingTaskState{OnboardingCompleted, OnboardingActionRequired, OnboardingInProgress, OnboardingLocked, OnboardingUnknown}
}

// OnboardingAction 是開通任務的下一步，值＝proto OnboardingAction 的值名去掉型別前綴。client 對到自己的路由；按鈕文字在共用文案目錄。
type OnboardingAction string

const (
	// OnboardingActionNone 沒有賣家能做的事。
	OnboardingActionNone OnboardingAction = "NONE"
	// OnboardingActionEditProfile 補齊店名、logo 或所在地。
	OnboardingActionEditProfile OnboardingAction = "EDIT_PROFILE"
	// OnboardingActionSubmitCertification 送出或重新送出商家認證。
	OnboardingActionSubmitCertification OnboardingAction = "SUBMIT_CERTIFICATION"
	// OnboardingActionStartPaymentOnboarding 開始或繼續金流商的收款帳戶設定。
	OnboardingActionStartPaymentOnboarding OnboardingAction = "START_PAYMENT_ONBOARDING"
	// OnboardingActionRefreshPaymentAccount 重新整理收款帳戶的狀態。
	OnboardingActionRefreshPaymentAccount OnboardingAction = "REFRESH_PAYMENT_ACCOUNT"
	// OnboardingActionContactSupport 聯絡客服。
	OnboardingActionContactSupport OnboardingAction = "CONTACT_SUPPORT"
)

// OnboardingActionValues 回傳全部下一步。
func OnboardingActionValues() []OnboardingAction {
	return []OnboardingAction{
		OnboardingActionNone, OnboardingActionEditProfile, OnboardingActionSubmitCertification, OnboardingActionStartPaymentOnboarding,
		OnboardingActionRefreshPaymentAccount, OnboardingActionContactSupport,
	}
}

// OnboardingKYCRead 是讀 KYC 申請的結果：Err 非 nil 是讀取失敗；沒有失敗時 Found 表示店家有申請（Status 是它的狀態）。
type OnboardingKYCRead struct {
	Err    error
	Found  bool
	Status KYCStatus
}

// OnboardingPaymentRead 是讀收款帳戶的結果：Err 非 nil 是讀取失敗；沒有失敗時 Found 表示有帳戶列（Status 是 payment 的狀態值：NOT_CREATED、
// UNDER_REVIEW、ACTION_REQUIRED、ENABLED、DISABLED）。CanStartOnboarding 是這個環境能不能產生開通 session（有設定收款開通）。
type OnboardingPaymentRead struct {
	Err                error
	Found              bool
	Status             string
	CanStartOnboarding bool
}

// OnboardingInput 是開通任務清單的輸入：讀店家時的條件（Store.Conditions，閘門的兩個來源欄在其中）、停權（Store.Sanctions）、KYC 與收款帳戶的讀取結果。
// KYC 與收款帳戶的讀取只決定狀態句與下一步，完成與否只看 Store.Conditions。
type OnboardingInput struct {
	Store   Store
	KYC     OnboardingKYCRead
	Payment OnboardingPaymentRead
}

// OnboardingTask 是一個開通任務；Title 與 Status 是給賣家看的句子。
type OnboardingTask struct {
	ID     OnboardingTaskID
	State  OnboardingTaskState
	Action OnboardingAction
	Title  uitext.Message
	Status uitext.Message
}

// Onboarding 是開通任務清單與開通閘門。
type Onboarding struct {
	// Tasks 是必要任務，依 OnboardingTaskIDValues 的順序。
	Tasks []OnboardingTask
	// Complete 是開通閘門 ONBOARDING_COMPLETE（與 CapabilitiesOf 的 CapPublicListing 與 CapShippingSetup 缺不缺這一項同一個值）。
	Complete bool
}

// OnboardingTasksOf 是開通任務清單的唯一產生點（語系 l）：任務的完成由 in.Store.Conditions 決定（與能力推導讀同一組欄位，D31 Q2、Q3）：
// PROFILE 看店名、logo 與鄉鎮市區代碼；PLATFORM_REVIEW 看 KYCApproved；PAYMENT_ACCOUNT 看 TransfersActive 副本（收款帳戶本身的讀取只決定狀態句與下一步），還沒完成而且平台審核也沒完成時是 LOCKED。
// 停權、關店與等待刪除不是任務（由公告頁處理）。讀取失敗（in.KYC.Err、in.Payment.Err）只讓需要那個讀取來決定狀態句與下一步的任務是 UNKNOWN：
// 已經完成的任務不需要讀取結果，仍是 COMPLETED。
func OnboardingTasksOf(l uitext.Locale, in OnboardingInput) Onboarding {
	return Onboarding{
		Tasks: []OnboardingTask{
			profileTask(l, in.Store),
			platformReviewTask(l, in.Store.Conditions, in.KYC),
			paymentAccountTask(l, in.Store.Conditions, in.Payment),
		},
		Complete: in.Store.Conditions.OnboardingComplete(),
	}
}

// onboardingCover 是一個公開條件由誰處理：tasks 是處理它的開通任務，notice 為 true 表示由公告頁處理（不是任務）；兩者恰好有一個。
type onboardingCover struct {
	tasks  []OnboardingTaskID
	notice bool
}

// requirementCovers 是每個公開條件（Requirements()）的處理者，也是任務函式的輸入：商店資料任務缺什麼由這張表挑出歸它的條件。
// 平台審核與收款帳戶一起處理 ONBOARDING_COMPLETE（兩者的 AND）。新增公開條件時必須在這裡有一列，測試
// TestEveryPublicListingRequirementIsCoveredByATaskOrTheNoticePage 守住。
var requirementCovers = map[Requirement]onboardingCover{
	RequirementStoreName:          {tasks: []OnboardingTaskID{OnboardingProfile}},
	RequirementLogo:               {tasks: []OnboardingTaskID{OnboardingProfile}},
	RequirementTownCode:           {tasks: []OnboardingTaskID{OnboardingProfile}},
	RequirementOnboardingComplete: {tasks: []OnboardingTaskID{OnboardingPlatformReview, OnboardingPaymentAccount}},
	RequirementActive:             {notice: true},
	RequirementNotSuspended:       {notice: true},
	RequirementNotPendingDeletion: {notice: true},
}

// requirementsOf 回傳歸任務 id 處理的公開條件，依 Requirements() 的順序。
func requirementsOf(id OnboardingTaskID) []Requirement {
	var out []Requirement
	for _, r := range Requirements() {
		if slices.Contains(requirementCovers[r].tasks, id) {
			out = append(out, r)
		}
	}
	return out
}

// profileTask：歸商店資料處理的條件（requirementCovers）全部滿足才完成；沒有讀過條件時 met 一律不滿足（fail-closed）。
func profileTask(l uitext.Locale, s Store) OnboardingTask {
	t := OnboardingTask{ID: OnboardingProfile, Title: text.OnboardingTitleProfile(l)}
	var missing []string
	for _, r := range requirementsOf(OnboardingProfile) {
		if met(r, s) {
			continue
		}
		noun, _ := uitext.For(l).SellerSellerRequirement(string(r))
		missing = append(missing, noun)
	}
	if len(missing) == 0 {
		t.State, t.Action, t.Status = OnboardingCompleted, OnboardingActionNone, text.OnboardingProfileCompleted(l)
		return t
	}
	t.State, t.Action = OnboardingActionRequired, OnboardingActionEditProfile
	t.Status = text.OnboardingProfileMissing(l, text.OnboardingProfileMissingArgs{Items: strings.Join(missing, text.OnboardingProfileListSeparator(l).Text)})
	return t
}

func platformReviewTask(l uitext.Locale, c Conditions, k OnboardingKYCRead) OnboardingTask {
	t := OnboardingTask{ID: OnboardingPlatformReview, Title: text.OnboardingTitlePlatformReview(l)}
	set := func(state OnboardingTaskState, action OnboardingAction, status uitext.Message) OnboardingTask {
		t.State, t.Action, t.Status = state, action, status
		return t
	}
	switch {
	case c.Derived && c.KYCApproved:
		return set(OnboardingCompleted, OnboardingActionNone, text.OnboardingPlatformReviewCompleted(l))
	case k.Err != nil:
		return set(OnboardingUnknown, OnboardingActionNone, text.OnboardingPlatformReviewUnknown(l))
	case !k.Found:
		return set(OnboardingActionRequired, OnboardingActionSubmitCertification, text.OnboardingPlatformReviewNotSubmitted(l))
	}
	switch k.Status {
	case KYCStatusSubmitted:
		return set(OnboardingInProgress, OnboardingActionNone, text.OnboardingPlatformReviewSubmitted(l))
	case KYCStatusRejected:
		return set(OnboardingActionRequired, OnboardingActionSubmitCertification, text.OnboardingPlatformReviewRejected(l))
	case KYCStatusCancelled:
		return set(OnboardingActionRequired, OnboardingActionSubmitCertification, text.OnboardingPlatformReviewCancelled(l))
	case KYCStatusDraft:
		return set(OnboardingActionRequired, OnboardingActionSubmitCertification, text.OnboardingPlatformReviewDraft(l))
	case KYCStatusApproved:
		// 申請已核准，閘門的欄位卻不是（沒有讀過條件，或兩次讀取之間改變）：以閘門為準，視為進行中。
		return set(OnboardingInProgress, OnboardingActionNone, text.OnboardingPlatformReviewSubmitted(l))
	}
	return set(OnboardingActionRequired, OnboardingActionSubmitCertification, text.OnboardingPlatformReviewNotSubmitted(l))
}

func paymentAccountTask(l uitext.Locale, c Conditions, p OnboardingPaymentRead) OnboardingTask {
	t := OnboardingTask{ID: OnboardingPaymentAccount, Title: text.OnboardingTitlePaymentAccount(l)}
	set := func(state OnboardingTaskState, action OnboardingAction, status uitext.Message) OnboardingTask {
		t.State, t.Action, t.Status = state, action, status
		return t
	}
	switch {
	case c.Derived && c.TransfersActive:
		return set(OnboardingCompleted, OnboardingActionNone, text.OnboardingPaymentAccountCompleted(l))
	case !c.Derived || !c.KYCApproved:
		return set(OnboardingLocked, OnboardingActionNone, text.OnboardingPaymentAccountLocked(l))
	case p.Err != nil:
		return set(OnboardingUnknown, OnboardingActionNone, text.OnboardingPaymentAccountUnknown(l))
	case !p.Found:
		return set(OnboardingInProgress, OnboardingActionRefreshPaymentAccount, text.OnboardingPaymentAccountPreparing(l))
	}
	switch p.Status {
	case "ACTION_REQUIRED":
		if p.CanStartOnboarding {
			return set(OnboardingActionRequired, OnboardingActionStartPaymentOnboarding, text.OnboardingPaymentAccountActionRequired(l))
		}
		return set(OnboardingActionRequired, OnboardingActionNone, text.OnboardingPaymentAccountActionRequiredUnavailable(l))
	case "DISABLED":
		return set(OnboardingActionRequired, OnboardingActionContactSupport, text.OnboardingPaymentAccountDisabled(l))
	case "ENABLED":
		// 帳戶已啟用，seller 的副本還沒有跟上：進行中，重新整理。
		return set(OnboardingInProgress, OnboardingActionRefreshPaymentAccount, text.OnboardingPaymentAccountSyncing(l))
	case "UNDER_REVIEW":
		return set(OnboardingInProgress, OnboardingActionRefreshPaymentAccount, text.OnboardingPaymentAccountUnderReview(l))
	}
	return set(OnboardingInProgress, OnboardingActionRefreshPaymentAccount, text.OnboardingPaymentAccountPreparing(l))
}
