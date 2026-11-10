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
	// OnboardingOtherRequirement 其他公開條件：公開曝光還缺其他任務與公告頁都沒有涵蓋的條件。
	OnboardingOtherRequirement OnboardingTaskID = "OTHER_REQUIREMENT"
)

// OnboardingTaskIDValues 回傳全部任務種類，依顯示順序。
func OnboardingTaskIDValues() []OnboardingTaskID {
	return []OnboardingTaskID{OnboardingProfile, OnboardingPlatformReview, OnboardingPaymentAccount, OnboardingOtherRequirement}
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
	// Tasks 是必要任務，依 OnboardingTaskIDValues 的順序；OTHER_REQUIREMENT 只在需要時出現。
	Tasks []OnboardingTask
	// Complete 是開通閘門 ONBOARDING_COMPLETE（與 CapabilitiesOf 的 CapPublicListing 與 CapShippingSetup 缺不缺這一項同一個值）。
	Complete bool
}

// OnboardingTasksOf 是開通任務清單的唯一產生點（語系 l）：任務的完成由 in.Store.Conditions 決定（與能力推導讀同一組欄位，D31 Q2、Q3）：
// PROFILE 看店名、logo 與鄉鎮市區代碼；PLATFORM_REVIEW 看 KYCApproved；PAYMENT_ACCOUNT 看 TransfersActive 副本（收款帳戶本身的讀取只決定狀態句與下一步），還沒完成而且平台審核也沒完成時是 LOCKED。
// 停權、關店與等待刪除不是任務（由公告頁處理）。讀取失敗（in.KYC.Err、in.Payment.Err）只讓需要那個讀取來決定狀態句與下一步的任務是 UNKNOWN：
// 已經完成的任務不需要讀取結果，仍是 COMPLETED。
func OnboardingTasksOf(l uitext.Locale, in OnboardingInput) Onboarding {
	return onboardingTasks(l, in, MissingExposure(in.Store))
}

// onboardingHandled 是公開曝光的條件裡，開通任務或公告頁已經處理的：其餘缺的條件才出現在 OTHER_REQUIREMENT。
var onboardingHandled = []Requirement{
	RequirementStoreName, RequirementLogo, RequirementTownCode, RequirementOnboardingComplete,
	RequirementActive, RequirementNotPendingDeletion, RequirementNotSuspended,
}

// onboardingTasks 以已算好的公開曝光缺項 missing 組清單（測試以人工的缺項涵蓋「沒有任務處理的條件」）。
func onboardingTasks(l uitext.Locale, in OnboardingInput, missing []Requirement) Onboarding {
	c := in.Store.Conditions
	tasks := []OnboardingTask{
		profileTask(l, c),
		platformReviewTask(l, c, in.KYC),
		paymentAccountTask(l, c, in.Payment),
	}
	for _, r := range missing {
		if !slices.Contains(onboardingHandled, r) {
			tasks = append(tasks, OnboardingTask{
				ID: OnboardingOtherRequirement, State: OnboardingActionRequired, Action: OnboardingActionContactSupport,
				Title: text.OnboardingTitleOtherRequirement(l), Status: text.OnboardingOtherRequirementMissing(l),
			})
			break
		}
	}
	return Onboarding{Tasks: tasks, Complete: c.OnboardingComplete()}
}

func profileTask(l uitext.Locale, c Conditions) OnboardingTask {
	t := OnboardingTask{ID: OnboardingProfile, Title: text.OnboardingTitleProfile(l)}
	if c.Derived && c.StoreName && c.Logo && c.TownCode {
		t.State, t.Action, t.Status = OnboardingCompleted, OnboardingActionNone, text.OnboardingProfileCompleted(l)
		return t
	}
	// 沒有讀過條件（Derived 為 false）時三項都算缺（fail-closed）。
	var missing []string
	for _, m := range []struct {
		ok bool
		r  Requirement
	}{{c.StoreName, RequirementStoreName}, {c.Logo, RequirementLogo}, {c.TownCode, RequirementTownCode}} {
		if m.ok && c.Derived {
			continue
		}
		noun, _ := uitext.For(l).SellerSellerRequirement(string(m.r))
		missing = append(missing, noun)
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
