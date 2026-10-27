package seller

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
)

// PayUni 子商店註冊（ADR 2026-10-19-seller-kyc-payuni-consolidated「核准與 PayUni 綁定」）的公開型別與方法。
// 賣家的方法以帳號為身分；管理員的方法由呼叫方（admin 的 Connect 方法）先確認是管理員。綁定、同步與觀測的外部呼叫都在
// River job，不在任何 transaction 內。

// PayUniClient 是呼叫 PayUni 的 port（真正的實作是 platform/payuni 的 Client，測試用替身）。
type PayUniClient = payuni.Client

// PayUniBindStatus 是註冊的綁定狀態。
type PayUniBindStatus = model.PayUniBindStatus

// PayUniOutcome 是 PayUni 回報的原碼推導出的審核結果。
type PayUniOutcome = model.PayUniOutcome

// PayUniSource 是一筆觀測的來源。
type PayUniSource = model.PayUniSource

// PayUniRemedy 是使用者該採取的動作類別。
type PayUniRemedy = model.PayUniRemedy

// PayUniProblem 是要給使用者看的問題（碼、處置、補件步驟、PayUni 的審核意見）。
type PayUniProblem = model.PayUniProblem

// PayUniRegistration 是店家的 PayUni 子商店註冊。
type PayUniRegistration = model.PayUniRegistration

// AdminPayUniRegistration 是後台看到的註冊：註冊加上它所在的 PayUni 階段與此刻不能重新綁定的原因。
type AdminPayUniRegistration = model.AdminPayUniRegistration

// PayUniStatus 是 PayUni 目前回報的狀態。
type PayUniStatus = model.PayUniStatus

// PayUniObservation 是一筆觀測紀錄。
type PayUniObservation = model.PayUniObservation

// PayUniObservationCursor 是觀測紀錄的 keyset 位置。
type PayUniObservationCursor = model.PayUniObservationCursor

// PayUniObservationListInput 是店家觀測紀錄的分頁輸入。
type PayUniObservationListInput = model.PayUniObservationListInput

// PayUniObservationPage 是一頁觀測紀錄。
type PayUniObservationPage = model.PayUniObservationPage

// PayUniNotification 是 PayUni 的 AGENT_NOTIFY 通知。
type PayUniNotification = model.PayUniNotification

// RebindPayUniInput 是管理員重新綁定。
type RebindPayUniInput = model.RebindPayUniInput

// KYCFlowStatus 是 cert 軸與 PayUni 軸合成的流程狀態。
type KYCFlowStatus = model.KYCFlowStatus

// KYCFlow 是流程狀態與衍生旗標（CanResubmit、IdentityLocked）；PayUni 的事實在 PayUniRegistration，賣家看到的 PayUni 狀態是 SellerPayUni。
type KYCFlow = model.KYCFlow

// PayUniState 是賣家看到的 PayUni 收款狀態（SellerPayUniOf 由申請與註冊推導）。
type PayUniState = model.PayUniState

// SellerPayUni 是賣家看到的 PayUni 狀態與要處理的事（KYCApplication.PayUni）。
type SellerPayUni = model.SellerPayUni

// KYCResubmitUnavailableReason 是儲存草稿與重新送出此刻不能做的原因（KYCApplication.ResubmitReasons）；Text 是它的句子。
type KYCResubmitUnavailableReason = model.KYCResubmitUnavailableReason

// PayUniObserveArgs 是套用一則 PayUni 通知的 job 參數。
type PayUniObserveArgs = payuni.ObserveArgs

const (
	BindStatusPending PayUniBindStatus = model.BindPending
	BindStatusBound   PayUniBindStatus = model.BindBound
	BindStatusFailed  PayUniBindStatus = model.BindFailed

	PayUniOutcomeNone         PayUniOutcome = model.OutcomeNone
	PayUniOutcomeUnderReview  PayUniOutcome = model.OutcomeUnderReview
	PayUniOutcomePendingInfo  PayUniOutcome = model.OutcomePendingInfo
	PayUniOutcomeApproved     PayUniOutcome = model.OutcomeApproved
	PayUniOutcomeRejected     PayUniOutcome = model.OutcomeRejected
	PayUniOutcomeUnrecognized PayUniOutcome = model.OutcomeUnrecognized

	PayUniSourceWebhook PayUniSource = model.SourceWebhook
	PayUniSourcePoll    PayUniSource = model.SourcePoll
	PayUniSourceRefresh PayUniSource = model.SourceRefresh
	PayUniSourceBind    PayUniSource = model.SourceBind

	FlowNotStarted     KYCFlowStatus = model.FlowNotStarted
	FlowDraft          KYCFlowStatus = model.FlowDraft
	FlowSubmitted      KYCFlowStatus = model.FlowSubmitted
	FlowRejected       KYCFlowStatus = model.FlowRejected
	FlowBinding        KYCFlowStatus = model.FlowBinding
	FlowBindFailed     KYCFlowStatus = model.FlowBindFailed
	FlowForwarded      KYCFlowStatus = model.FlowForwarded
	FlowUnderReview    KYCFlowStatus = model.FlowUnderReview
	FlowPendingInfo    KYCFlowStatus = model.FlowPendingInfo
	FlowApproved       KYCFlowStatus = model.FlowApproved
	FlowPayUniRejected KYCFlowStatus = model.FlowPayUniRejected

	// 我們自己的錯誤碼（PayUni 的碼是 AGENT 開頭，原樣保存在 BindErrorCode、SyncErrorCode）。
	PayUniErrIncompleteData   = model.ErrIncompleteData
	PayUniErrNotConfigured    = model.ErrNotConfigured
	PayUniErrUnavailable      = model.ErrUnavailable
	PayUniErrMerchantTerminal = model.ErrMerchantTerminal
)

// PayUniStateValues 回傳全部賣家端的 PayUni 狀態。
func PayUniStateValues() []PayUniState { return model.PayUniStateValues() }

// KYCResubmitUnavailableReasonValues 回傳全部重新送出的不能原因。
func KYCResubmitUnavailableReasonValues() []KYCResubmitUnavailableReason {
	return model.KYCResubmitUnavailableReasonValues()
}

// PayUni 錯誤的處置類別（值與 ClassifyPayUniCode 回傳的一致）。
const (
	RemedySellerFix      = model.RemedySellerFix
	RemedyContactSupport = model.RemedyContactSupport
	RemedyRetryLater     = model.RemedyRetryLater
)

// KYCFlowStatusValues 回傳全部流程狀態。
func KYCFlowStatusValues() []KYCFlowStatus { return model.KYCFlowStatusValues() }

// ErrPayUniPermanent 標示重試也不會成功的 PayUni 通知處理失敗（未知的店家）：job 記錄後取消，consumer 不重送。
var ErrPayUniPermanent = payuni.ErrPermanent

// PayUniBindStatusValues 回傳全部綁定狀態。
func PayUniBindStatusValues() []PayUniBindStatus { return model.PayUniBindStatusValues() }

// PayUniOutcomeValues 回傳全部 PayUni outcome。
func PayUniOutcomeValues() []PayUniOutcome { return model.PayUniOutcomeValues() }

// PayUniSourceValues 回傳全部觀測來源。
func PayUniSourceValues() []PayUniSource { return model.PayUniSourceValues() }

// FormatPayUniMerID 由 store_code 組成 PayUni 的 MerID。
func FormatPayUniMerID(storeCode int64) string { return model.FormatMerID(storeCode) }

// PayUniOutcomeOf 把 PayUni 的 (MerStatus, MemberStatus) 推導成審核結果。
func PayUniOutcomeOf(merStatus, memberStatus string) PayUniOutcome {
	return model.PayUniOutcomeOf(merStatus, memberStatus)
}

// ClassifyPayUniCode 回傳 PayUni 錯誤碼的處置類別與賣家要補的申請欄位（KYCFieldNames 的名稱）、店家欄位（StoreFieldNames 的名稱）；
// 不在目錄裡的碼 known 為 false（fail closed：聯絡客服）。
func ClassifyPayUniCode(code string) (remedy PayUniRemedy, fields, storeFields []string, known bool) {
	return model.ClassifyPayUniCode(code)
}

// PayUniProblemOf 回傳註冊目前要讓賣家處理的問題（綁定失敗、PayUni 退件或要求補件），沒有時是 nil；與申請的流程狀態用
// 同一個推導。
func PayUniProblemOf(reg PayUniRegistration) *PayUniProblem { return model.ProblemOf(reg) }

// RebindPayUni 管理員對綁定失敗（FAILED）的註冊重新綁定：回到 PENDING 並排入新的 job，不是再核准一次。
func (m *Module) RebindPayUni(ctx context.Context, in RebindPayUniInput) (AdminPayUniRegistration, error) {
	return m.payuni.Rebind(ctx, in)
}

// PayUniRegistrationOf 回傳店家的 PayUni 註冊（管理員）；沒有時回 NotFound。
func (m *Module) PayUniRegistrationOf(ctx context.Context, storeID int64) (AdminPayUniRegistration, error) {
	return m.payuni.Registration(ctx, storeID)
}

// PayUniObservations 回傳店家的 PayUni 狀態觀測紀錄的一頁，新到舊的 keyset（管理員；預設 20、上限 100）。
func (m *Module) PayUniObservations(ctx context.Context, in PayUniObservationListInput) (PayUniObservationPage, error) {
	return m.payuni.Observations(ctx, in)
}

// RefreshPayUniStatus 管理員手動刷新：排入查詢 PayUni 狀態的 job，套用走和 webhook、輪詢同一個函式。
func (m *Module) RefreshPayUniStatus(ctx context.Context, storeID int64) error {
	return m.payuni.Refresh(ctx, storeID)
}

// ReceivePayUniNotification 接收 PayUni 的 AGENT_NOTIFY（legacy 的 RabbitMQ consumer `sellerconsumer` 呼叫）：驗證形狀後排入
// 以 event_key 為唯一鍵的 job，回傳時 job 已持久化，consumer 此後才 ack。形狀不合回 InvalidInput（重送也不會好）。
func (m *Module) ReceivePayUniNotification(ctx context.Context, n PayUniNotification) error {
	return m.payuni.Receive(ctx, n)
}

// SyncPayUni 是 seller.payuni_sync job 的內容：PENDING 的註冊向 PayUni 綁定，BOUND 而有資料沒送出的送聯絡欄位與地址。
// finalAttempt 為 true（River 的最後一次嘗試）時暫時性的失敗改為記錄結果。again 為 true 表示稍後要再跑一輪。給 worker
// 與測試呼叫。
func (m *Module) SyncPayUni(ctx context.Context, storeID int64, finalAttempt bool) (again bool, err error) {
	return m.payuni.Sync(ctx, storeID, finalAttempt)
}

// ApplyPayUniNotification 是 seller.payuni_observe job 的內容：追加觀測並套用。未知的店家回 ErrPayUniPermanent。
func (m *Module) ApplyPayUniNotification(ctx context.Context, a PayUniObserveArgs) error {
	return m.payuni.Observe(ctx, a)
}

// PollPayUni 是 seller.payuni_poll job 的內容：向 PayUni 查一次審核狀態並套用。
func (m *Module) PollPayUni(ctx context.Context, storeID int64, source PayUniSource) error {
	return m.payuni.Poll(ctx, storeID, source)
}

// SweepPayUni 是 seller.payuni_sweep job 的內容：補排有資料沒送出又閒置的 sync、排超過間隔沒問過的輪詢；回傳各補排的數量。
func (m *Module) SweepPayUni(ctx context.Context) (resynced, polled int, err error) {
	return m.payuni.Sweep(ctx)
}
