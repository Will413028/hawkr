package seller

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
)

// 主管機關來函（step10e-reports-brief R5、設計 step10e-3cb-design）的公開型別與方法。

type (
	// AuthorityOrderKind 是來函要平台做的事（限制或移除商品、調取資料、暫停帳號、其他）。
	AuthorityOrderKind = model.AuthorityOrderKind
	// AuthorityOrderState 是來函此刻的狀態（處理中、已完成、已撤銷登記），讀取時推導。
	AuthorityOrderState = model.AuthorityOrderState
	// AuthorityNoticeKind 是被指名者的告知方式（已告知、暫不告知、由措施告知）。
	AuthorityNoticeKind = model.AuthorityNoticeKind
	// HoldExtensionCause 是保存期限被延長的原因（管理員、提供資料之後）。
	HoldExtensionCause = model.HoldExtensionCause
	// StoreCoverage 是保存對指名範圍內某一家店的涵蓋狀態。
	StoreCoverage = model.StoreCoverage
	// AuthorityOrderBlock 是來函此刻不能做某個動作的一個原因。
	AuthorityOrderBlock = model.AuthorityOrderBlock
	// AuthorityNotice 是一個對象的告知。
	AuthorityNotice = model.AuthorityNotice
	// AuthorityHoldExtension 是保存期限的一次延長。
	AuthorityHoldExtension = model.AuthorityHoldExtension
	// AuthorityTargetStore 是被指名範圍內的一家店與它的涵蓋狀態。
	AuthorityTargetStore = model.AuthorityTargetStore
	// AuthorityOrderTarget 是來函指名的對象。
	AuthorityOrderTarget = model.AuthorityOrderTarget
	// AuthorityCompletion 是來函的完成。
	AuthorityCompletion = model.AuthorityCompletion
	// AuthorityRevocation 是來函的撤銷登記（登記錯誤）。
	AuthorityRevocation = model.AuthorityRevocation
	// TerminationTarget 是建立措施的目標店家的事實（是否關閉、是不是帳號最近的 stint）。
	TerminationTarget = model.TerminationTarget
	// AuthorityOrder 是主管機關的一份來函。
	AuthorityOrder = model.AuthorityOrder
	// AuthorityOrderAvailability 是來函每個動作不能做的原因。
	AuthorityOrderAvailability = model.AuthorityOrderAvailability
	// AdminAuthorityOrder 是後台看到的一份來函（含此刻可以做的動作）。
	AdminAuthorityOrder = model.AdminAuthorityOrder
	// AuthorityTargetInput 是登記來函時指名的一個對象。
	AuthorityTargetInput = model.AuthorityTargetInput
	// CreateAuthorityOrderInput 是登記來函的輸入。
	CreateAuthorityOrderInput = model.CreateAuthorityOrderInput
	// CreateAuthorityOrderResult 是登記來函的結果。
	CreateAuthorityOrderResult = model.CreateAuthorityOrderResult
	// AuthorityNoticeInput 是一個對象的告知輸入。
	AuthorityNoticeInput = model.AuthorityNoticeInput
	// TargetNoticeInput 是完成來函時一個對象的告知。
	TargetNoticeInput = model.TargetNoticeInput
	// CompleteAuthorityOrderInput 是完成來函的輸入。
	CompleteAuthorityOrderInput = model.CompleteAuthorityOrderInput
	// ExtendAuthorityOrderHoldInput 是延長保存期限的輸入。
	ExtendAuthorityOrderHoldInput = model.ExtendAuthorityOrderHoldInput
	// RevokeAuthorityOrderInput 是撤銷登記的輸入。
	RevokeAuthorityOrderInput = model.RevokeAuthorityOrderInput
	// RevokeAuthorityOrderResult 是撤銷登記的結果。
	RevokeAuthorityOrderResult = model.RevokeAuthorityOrderResult
	// RecordAuthorityTargetNoticeInput 是記下暫不告知的對象後來的告知的輸入。
	RecordAuthorityTargetNoticeInput = model.RecordAuthorityTargetNoticeInput
	// AuthorityCursor 是來函清單的 keyset 位置。
	AuthorityCursor = model.AuthorityCursor
	// AuthorityOrderListInput 是來函清單的輸入。
	AuthorityOrderListInput = model.AuthorityOrderListInput
	// AdminAuthorityOrderPage 是一頁來函。
	AdminAuthorityOrderPage = model.AdminAuthorityOrderPage
)

const (
	AuthorityRestrictProduct = model.AuthorityRestrictProduct
	AuthorityDataRequest     = model.AuthorityDataRequest
	AuthoritySuspendAccount  = model.AuthoritySuspendAccount
	AuthorityOther           = model.AuthorityOther

	AuthorityStateOpen      = model.AuthorityStateOpen
	AuthorityStateCompleted = model.AuthorityStateCompleted
	AuthorityStateRevoked   = model.AuthorityStateRevoked

	AuthorityNoticeNotified = model.AuthorityNoticeNotified
	AuthorityNoticeWithheld = model.AuthorityNoticeWithheld
	AuthorityNoticeMeasure  = model.AuthorityNoticeMeasure

	HoldExtensionAdmin        = model.HoldExtensionAdmin
	HoldExtensionDataProvided = model.HoldExtensionDataProvided

	CoverageHeld                     = model.CoverageHeld
	CoveragePurgedBeforeRegistration = model.CoveragePurgedBeforeRegistration
	CoverageHoldEnded                = model.CoverageHoldEnded
	CoveragePurgedAfterHoldEnded     = model.CoveragePurgedAfterHoldEnded
	CoverageHoldReleasedByRevocation = model.CoverageHoldReleasedByRevocation

	AuthorityBlockRevoked           = model.AuthorityBlockRevoked
	AuthorityBlockCompleted         = model.AuthorityBlockCompleted
	AuthorityBlockNoticeNotWithheld = model.AuthorityBlockNoticeNotWithheld

	// AuthorityHold 是來函保存期限的預設長度：收文後 184 天。
	AuthorityHold = model.AuthorityHold
	// MaxSourceAuthorityOrders 是一個認定最多引用的來函數。
	MaxSourceAuthorityOrders = model.MaxSourceAuthorityOrders
	// MaxAuthorityTargets 與 MaxAuthorityTargetProducts 是一份來函最多指名的對象數與每個對象最多的商品數。
	MaxAuthorityTargets        = model.MaxAuthorityTargets
	MaxAuthorityTargetProducts = model.MaxAuthorityTargetProducts
)

// AuthorityOrderKindValues 回傳全部來函種類。
func AuthorityOrderKindValues() []AuthorityOrderKind { return model.AuthorityOrderKindValues() }

// AuthorityOrderStateValues 回傳全部來函狀態。
func AuthorityOrderStateValues() []AuthorityOrderState { return model.AuthorityOrderStateValues() }

// AuthorityNoticeKindValues 回傳全部告知方式。
func AuthorityNoticeKindValues() []AuthorityNoticeKind { return model.AuthorityNoticeKindValues() }

// HoldExtensionCauseValues 回傳全部保存延長的原因。
func HoldExtensionCauseValues() []HoldExtensionCause { return model.HoldExtensionCauseValues() }

// AuthorityOrderBlockValues 回傳全部來函不能做動作的原因。
func AuthorityOrderBlockValues() []AuthorityOrderBlock { return model.AuthorityOrderBlockValues() }

// DefaultAuthorityDue 是來函沒有指定期限時，收文後的期限。
func DefaultAuthorityDue(kind AuthorityOrderKind) time.Duration {
	return model.DefaultAuthorityDue(kind)
}

// CreateAuthorityOrder 登記主管機關的一份來函：對象是任何帳號（必須存在），保存期限定為收文後 184 天。帶 request_id 重送回傳原本那一份。
func (m *Module) CreateAuthorityOrder(ctx context.Context, in CreateAuthorityOrderInput) (CreateAuthorityOrderResult, error) {
	return m.governance.CreateAuthorityOrder(ctx, in)
}

// CompleteAuthorityOrder 標記來函完成並記下每個對象的告知；調取資料的來函依提供時間延長保存期限。
func (m *Module) CompleteAuthorityOrder(ctx context.Context, in CompleteAuthorityOrderInput) (AdminAuthorityOrder, error) {
	return m.governance.CompleteAuthorityOrder(ctx, in)
}

// ExtendAuthorityOrderHold 延長來函的保存期限（只能延長，留下前後值與原因）。
func (m *Module) ExtendAuthorityOrderHold(ctx context.Context, in ExtendAuthorityOrderHoldInput) (AdminAuthorityOrder, error) {
	return m.governance.ExtendAuthorityOrderHold(ctx, in)
}

// RecordAuthorityTargetNotice 記下暫不告知的對象後來的告知（改成已告知，或寫新的確認日期）。
func (m *Module) RecordAuthorityTargetNotice(ctx context.Context, in RecordAuthorityTargetNoticeInput) (AdminAuthorityOrder, error) {
	return m.governance.RecordAuthorityTargetNotice(ctx, in)
}

// RevokeAuthorityOrder 撤銷登記錯誤的來函（只在完成之前，附原因）：保存解除、紀錄保留。同一位管理員重送回傳目前的紀錄（AlreadyRevoked）。
func (m *Module) RevokeAuthorityOrder(ctx context.Context, in RevokeAuthorityOrderInput) (RevokeAuthorityOrderResult, error) {
	return m.governance.RevokeAuthorityOrder(ctx, in)
}

// AuthorityOrder 讀一份來函的後台檢視。只由 admin 的 handler 呼叫，自己開讀取快照（不是跨模組入口）。
func (m *Module) AuthorityOrder(ctx context.Context, id uuid.UUID) (AdminAuthorityOrder, error) {
	return m.governance.AuthorityOrder(ctx, id)
}

// AuthorityOrders 是來函清單。只由 admin 的 handler 呼叫，自己開讀取快照（不是跨模組入口）。
func (m *Module) AuthorityOrders(ctx context.Context, in AuthorityOrderListInput) (AdminAuthorityOrderPage, error) {
	return m.governance.AuthorityOrders(ctx, in)
}
