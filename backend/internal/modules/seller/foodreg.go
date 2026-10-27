package seller

import (
	"context"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// FoodRegistration 是店家的一筆食品業者登錄（核准中的那一筆是公開字號的唯一來源）。
type FoodRegistration = model.FoodRegistration

// FoodRegState 是食品業者登錄的狀態。
type FoodRegState = model.FoodRegState

// 食品業者登錄的狀態。
const (
	FoodRegPending   = model.FoodRegPending
	FoodRegApproved  = model.FoodRegApproved
	FoodRegRejected  = model.FoodRegRejected
	FoodRegWithdrawn = model.FoodRegWithdrawn
	FoodRegEnded     = model.FoodRegEnded
)

// FoodRegStateValues 回傳全部儲存的狀態。
func FoodRegStateValues() []FoodRegState { return model.FoodRegStateValues() }

// FoodRegInbox 是審核收件匣的隊列：現在可以裁定（店家的 KYC 已核准）與還在等 KYC 核准。
type FoodRegInbox = model.FoodRegInbox

// 審核收件匣的隊列。
const (
	FoodRegInboxDecidable   = model.FoodRegInboxDecidable
	FoodRegInboxAwaitingKYC = model.FoodRegInboxAwaitingKYC
)

// FoodRegInboxValues 回傳全部隊列。
func FoodRegInboxValues() []FoodRegInbox { return model.FoodRegInboxValues() }

// FoodRegEndCause 是核准的登錄結束的原因。
type FoodRegEndCause = model.FoodRegEndCause

// 核准的登錄結束的原因。
const (
	FoodRegSuperseded   = model.FoodRegSuperseded
	FoodRegCleared      = model.FoodRegCleared
	FoodRegRevoked      = model.FoodRegRevoked
	FoodRegStoreRetired = model.FoodRegStoreRetired
)

// FoodRegEndCauseValues 回傳全部結束原因。
func FoodRegEndCauseValues() []FoodRegEndCause { return model.FoodRegEndCauseValues() }

// FoodRegistrationSubject 是後台核對登錄時要比對的 KYC 主體。
type FoodRegistrationSubject = model.FoodRegistrationSubject

// FoodRegistrationReview 是後台看到的一筆登錄。
type FoodRegistrationReview = model.FoodRegistrationReview

// SubmitFoodRegistrationInput 是賣家送出食品業者登錄。
type SubmitFoodRegistrationInput = model.SubmitFoodRegistrationInput

// FoodRegistrationCursor 是後台清單的 keyset 位置。
type FoodRegistrationCursor = model.FoodRegistrationCursor

// FoodRegistrationListInput 是後台的清單輸入。
type FoodRegistrationListInput = model.FoodRegistrationListInput

// FoodRegistrationPage 是一頁後台清單。
type FoodRegistrationPage = model.FoodRegistrationPage

// DecideFoodRegistrationInput 是管理員裁定待審的登錄。
type DecideFoodRegistrationInput = model.DecideFoodRegistrationInput

// RevokeFoodRegistrationInput 是管理員撤銷核准中的登錄。
type RevokeFoodRegistrationInput = model.RevokeFoodRegistrationInput

// SubmitFoodRegistration 賣家送出食品業者登錄供平台核對；已有待審時取代它，目前核准的那一筆照常有效。回傳送出之後的店家。
func (m *Module) SubmitFoodRegistration(ctx context.Context, in SubmitFoodRegistrationInput) (Store, error) {
	return m.foodreg.Submit(ctx, in)
}

// WithdrawFoodRegistration 賣家撤回待審的登錄，核准的那一筆保留。
func (m *Module) WithdrawFoodRegistration(ctx context.Context, storeID int64) (Store, error) {
	return m.foodreg.Withdraw(ctx, storeID)
}

// ClearFoodRegistration 賣家清除登錄：結束核准的那一筆並撤回待審的，立即生效。
func (m *Module) ClearFoodRegistration(ctx context.Context, storeID int64) (Store, error) {
	return m.foodreg.Clear(ctx, storeID)
}

// FoodRegistrations 後台的清單：沒有指定狀態是收件匣，指定其他狀態是歷史。
func (m *Module) FoodRegistrations(ctx context.Context, in FoodRegistrationListInput) (FoodRegistrationPage, error) {
	return m.foodreg.List(ctx, in)
}

// FoodRegistrationReview 讀一筆登錄的後台檢視。
func (m *Module) FoodRegistrationReview(ctx context.Context, id uuid.UUID) (FoodRegistrationReview, error) {
	return m.foodreg.Get(ctx, id)
}

// DecideFoodRegistration 管理員核准或退回待審的登錄；核准要求店家的 KYC 已核准。
func (m *Module) DecideFoodRegistration(ctx context.Context, in DecideFoodRegistrationInput) (FoodRegistrationReview, error) {
	return m.foodreg.Decide(ctx, in)
}

// RevokeFoodRegistration 管理員撤銷核准中的登錄：店面與商品頁立即不再顯示這個字號，理由必填、給賣家看。
func (m *Module) RevokeFoodRegistration(ctx context.Context, in RevokeFoodRegistrationInput) (FoodRegistrationReview, error) {
	return m.foodreg.Revoke(ctx, in)
}
