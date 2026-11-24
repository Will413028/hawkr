package model

import (
	"time"

	"uuid"
)

// PaymentStatus 是付款的狀態（payments_status_check），只由 provider 的快照推導。
type PaymentStatus string

// 付款的狀態。
const (
	// PaymentCreating 已寫列，provider 那邊的 PaymentIntent 還沒確認建立（含呼叫過、結果不明）。
	PaymentCreating PaymentStatus = "CREATING"
	// PaymentAwaitingPayment 已建立，等買家付款（含需要 3DS 的額外驗證）。
	PaymentAwaitingPayment PaymentStatus = "AWAITING_PAYMENT"
	// PaymentProcessing 買家已送出付款，provider 處理中。
	PaymentProcessing PaymentStatus = "PROCESSING"
	// PaymentSucceeded 已入帳（終局）。
	PaymentSucceeded PaymentStatus = "SUCCEEDED"
	// PaymentCanceled 已取消（provider 的成功仍會勝出）。
	PaymentCanceled PaymentStatus = "CANCELED"
	// PaymentCreateFailed 建立的終局：provider 永久拒絕，或查回證明 provider 沒有這筆。
	PaymentCreateFailed PaymentStatus = "CREATE_FAILED"
	// PaymentAnomaly 建立的結果與 provider 對不起來或查回連續失敗：佔住這張訂單、補掃不再重試，由後台人工結案。
	PaymentAnomaly PaymentStatus = "ANOMALY"
)

// PaymentStatusValues 回傳全部付款狀態，與 payments_status_check 相同。
func PaymentStatusValues() []PaymentStatus {
	return []PaymentStatus{PaymentCreating, PaymentAwaitingPayment, PaymentProcessing, PaymentSucceeded, PaymentCanceled, PaymentCreateFailed, PaymentAnomaly}
}

// TransferStatus 是轉帳的狀態（transfers_status_check）。
type TransferStatus string

// 轉帳的狀態。
const (
	// TransferPending 已寫列，尚未呼叫或結果不明。
	TransferPending TransferStatus = "PENDING"
	// TransferSucceeded provider 已建立轉帳。
	TransferSucceeded TransferStatus = "SUCCEEDED"
	// TransferFailed 終局失敗，不佔「一張訂單應收最多一筆有效轉帳」。
	TransferFailed TransferStatus = "FAILED"
	// TransferAnomaly 與 provider 對不起來或前置條件不成立：不能確定沒有搬錢，所以仍佔位，補掃不再重試，由後台人工結案。
	TransferAnomaly TransferStatus = "ANOMALY"
)

// TransferStatusValues 回傳全部轉帳狀態，與 transfers_status_check 相同。
func TransferStatusValues() []TransferStatus {
	return []TransferStatus{TransferPending, TransferSucceeded, TransferFailed, TransferAnomaly}
}

// Transfer 是訂單應收轉給賣家的轉帳（transfers 的對外視圖）；ReversedAmount 是累計已撤回的額度。Amount 在 job 第一次送出（claim）之前只是暫定值（訂單應收金額減寫列當下這張訂單的非失敗退款），送出時才算定。
type Transfer struct {
	ID             uuid.UUID
	PaymentOrderID uuid.UUID
	StoreID        int64
	Amount         int64
	ReversedAmount int64
	Status         TransferStatus
	// FailureReason 只在 TransferFailed 與 TransferAnomaly 有值。
	FailureReason string
	CreatedAt     time.Time
}

// RefundStatus 是退款的狀態（refunds_status_check）。
type RefundStatus string

// 退款的狀態。
const (
	// RefundPending 已寫列，尚未呼叫 provider。
	RefundPending RefundStatus = "PENDING"
	// RefundRefunding 已呼叫退款，等 provider 的結果。
	RefundRefunding RefundStatus = "REFUNDING"
	// RefundSucceeded 退款成功。
	RefundSucceeded RefundStatus = "SUCCEEDED"
	// RefundFailed 退款失敗，不佔累計額度。
	RefundFailed RefundStatus = "FAILED"
	// RefundAnomaly 退款與 provider 對不起來或前置條件不成立：仍佔額度，補掃不再重試，由後台人工結案。
	RefundAnomaly RefundStatus = "ANOMALY"
)

// RefundStatusValues 回傳全部退款狀態，與 refunds_status_check 相同。
func RefundStatusValues() []RefundStatus {
	return []RefundStatus{RefundPending, RefundRefunding, RefundSucceeded, RefundFailed, RefundAnomaly}
}

// ReversalStatus 是退款列上向賣家收回（撤回轉帳）的彙總狀態（refunds_reversal_status_check），與退款狀態並行、各自收斂（D11、ADR D69）；REVERSING 起的四個值也是每一次 provider 撤回呼叫（refund_reversals.status，refund_reversals_status_check）的狀態，彙總由撤回列推導。
type ReversalStatus string

// 向賣家收回的狀態。
const (
	// ReversalNone 不需要收回（轉帳還沒承諾金額、沒有轉帳或沒有可撤額度）。
	ReversalNone ReversalStatus = "NONE"
	// ReversalWaiting 已預留額度、尚未開始：等 provider 接受退款且轉帳成立。
	ReversalWaiting ReversalStatus = "WAITING"
	// ReversalReversing 撤回進行中（至少一次 provider 撤回呼叫還沒終局，結果不明時依 D68 收斂）。
	ReversalReversing ReversalStatus = "REVERSING"
	// ReversalReversed 撤回成立（撤回列：這一次成立；彙總：全部撤回列成立）。
	ReversalReversed ReversalStatus = "REVERSED"
	// ReversalReleased 預留已釋出、沒有撤回（退款在撤回前失敗、轉帳少付或失敗、撤回永久失敗、人工結案）；彙總只在撤回列終局而不是全部成立時才是它，已成立的部分留在 reversal_recovered。
	ReversalReleased ReversalStatus = "RELEASED"
	// ReversalAnomaly 撤回與 provider 對不起來：額度持續佔住，由後台人工結案（撤回列逐列結案）。
	ReversalAnomaly ReversalStatus = "ANOMALY"
)

// ReversalStatusValues 回傳全部收回狀態，與 refunds_reversal_status_check 相同。
func ReversalStatusValues() []ReversalStatus {
	return []ReversalStatus{ReversalNone, ReversalWaiting, ReversalReversing, ReversalReversed, ReversalReleased, ReversalAnomaly}
}

// TopupStatus 是退款列上補差額轉帳給賣家的狀態（refunds_topup_status_check）：已算進轉帳金額的退款之後失敗，賣家少收的那筆錢補給賣家（D11 補充），與退款、撤回狀態並行、各自收斂。
type TopupStatus string

// 補差額的狀態。
const (
	// TopupNone 不需要補差額：退款沒有算進轉帳金額，或沒有失敗。
	TopupNone TopupStatus = "NONE"
	// TopupWaiting 退款已失敗、轉帳金額含它：等轉帳 SUCCEEDED。
	TopupWaiting TopupStatus = "WAITING"
	// TopupSending 補差額轉帳進行中（結果不明時依 D68 收斂）。
	TopupSending TopupStatus = "SENDING"
	// TopupDone 補差額轉帳已成立。
	TopupDone TopupStatus = "DONE"
	// TopupReleased 不補了（轉帳失敗、重新算定的金額已不含這筆退款，或補差額永久被拒絕）。
	TopupReleased TopupStatus = "RELEASED"
	// TopupAnomaly 補差額與 provider 對不起來，由後台人工結案。
	TopupAnomaly TopupStatus = "ANOMALY"
)

// TopupStatusValues 回傳全部補差額狀態，與 refunds_topup_status_check 相同。
func TopupStatusValues() []TopupStatus {
	return []TopupStatus{TopupNone, TopupWaiting, TopupSending, TopupDone, TopupReleased, TopupAnomaly}
}

// PaymentOrder 是付款底下一張訂單的應收（payment_orders 的對外視圖，D13）：Amount 是這張訂單的應收金額（整數、不帶幣別，D9），也是它的退款上限與轉帳金額的起點。
type PaymentOrder struct {
	ID      uuid.UUID
	OrderNo string
	StoreID int64
	Amount  int64
}

// Payment 是買家一次結帳的付款（一個 PaymentIntent 對 Orders 底下 N 張訂單應收，D13；過渡期 N 是 1）。Amount 是各訂單應收的總額，金額是整數、不帶幣別（D9）。
type Payment struct {
	ID        uuid.UUID
	BuyerID   int64
	Amount    int64
	Provider  string
	Status    PaymentStatus
	PaidAt    *time.Time
	LastError string
	CreatedAt time.Time
	// Orders 是付款底下的訂單應收，依 ID 升序，至少一張。
	Orders []PaymentOrder
	// ClientSecret 是買家端完成付款的憑證，只在狀態是 AWAITING_PAYMENT 時由 CreatePayment 與 GetPayment 向 provider 讀來回傳，不存。其餘狀態與讀取失敗時是空字串。
	ClientSecret string
}

// OrderPayment 是從一張訂單應收看到的付款（舊服務與訂單一方以訂單編號為鍵的讀取）：Payment 是群組，Order 是被問到的那張訂單應收。
type OrderPayment struct {
	Payment
	Order PaymentOrder
}

// PaymentChanged 在付款的狀態改變的 transaction 內發布（寫入快照時狀態與上次不同才發）：訂單一方（過渡期經 relay 給 Java order）據此得知付款成功或取消，
// 並判斷訂單已不可付時發起自動退款（D5 修訂：payment 不讀訂單狀態）。LastError 是最近一次嘗試失敗的 provider 原因碼（沒有時空字串）。
type PaymentChanged struct {
	OccurredAt time.Time
	PaymentID  uuid.UUID
	BuyerID    int64
	// Amount 是各訂單應收的總額；Orders 是付款底下的訂單應收（每張訂單一列，各自帶金額與店家），訂單一方依它逐張訂單處理。
	Amount   int64
	Orders   []PaymentOrder
	Provider string
	Status   PaymentStatus
	// Previous 是這次寫入之前的狀態。
	Previous  PaymentStatus
	PaidAt    *time.Time
	LastError string
}

// ExcessPaid 是一次重複入帳：同一張訂單的付款已經以另一個 PaymentIntent 入帳，這個舊 PaymentIntent 也扣到了款。payment 不自動退款（D5 修訂），給後台與訂單一方處理。
type ExcessPaid struct {
	AttemptID         uuid.UUID
	PaymentID         uuid.UUID
	Provider          string
	ExternalPaymentID string
	ExcessPaidAt      time.Time
}

// AnomalyKind 是進入 ANOMALY 的物件種類。
type AnomalyKind string

// 進入 ANOMALY 的物件種類。
const (
	AnomalyPayment  AnomalyKind = "payment"
	AnomalyTransfer AnomalyKind = "transfer"
	AnomalyRefund   AnomalyKind = "refund"
	// AnomalyReversal 是一次 provider 撤回呼叫（refund_reversals 的列，ObjectID 是撤回列 ID）。
	AnomalyReversal AnomalyKind = "reversal"
	// AnomalyTopup 是退款列上的補差額轉帳（ObjectID 是退款 ID）。
	AnomalyTopup AnomalyKind = "topup"
)

// MoneyAnomaly 是告警事件：搬錢的物件（付款、轉帳、退款）與 provider 對不起來而進入 ANOMALY，或 provider 已搬錢本地卻已不在可接收的狀態，需要人工核對。
// 在寫入狀態的同一個 transaction 內發布（沒有狀態可寫的孤兒結果也發布）；Reason 是原因碼與 ID 的簡短說明，不含個資。
type MoneyAnomaly struct {
	OccurredAt time.Time
	Kind       AnomalyKind
	ObjectID   uuid.UUID
	PaymentID  uuid.UUID
	// PaymentOrderID 是物件所屬的訂單應收；付款本身（AnomalyPayment）屬於整筆付款，沒有它，是零值。
	PaymentOrderID uuid.UUID
	Reason         string
}

// AnomalyOutcome 是人工結案的結果：AnomalySucceeded 是 provider 確實執行了（附 provider 物件 ID 為證據），AnomalyFailed 是確認 provider 沒有執行。
type AnomalyOutcome string

// 人工結案的結果。
const (
	AnomalySucceeded AnomalyOutcome = "SUCCEEDED"
	AnomalyFailed    AnomalyOutcome = "FAILED"
)

// AnomalyResolutionReason 是人工結案的原因代碼（寫進後台動作紀錄）：管理員必選，與備註一起記下為什麼這樣結案。
type AnomalyResolutionReason string

// 人工結案的原因代碼。
const (
	// ResolutionProviderExecutedNotRecorded provider 已執行、本地沒有記到結果（只搭配 AnomalySucceeded）。
	ResolutionProviderExecutedNotRecorded AnomalyResolutionReason = "PROVIDER_EXECUTED_NOT_RECORDED"
	// ResolutionProviderNotExecuted provider 沒有執行這次呼叫（只搭配 AnomalyFailed）。
	ResolutionProviderNotExecuted AnomalyResolutionReason = "PROVIDER_NOT_EXECUTED"
	// ResolutionDuplicate 重複：同一件事已由另一筆紀錄處理（搭配任一結果）。
	ResolutionDuplicate AnomalyResolutionReason = "DUPLICATE"
	// ResolutionOther 其他原因，細節寫在備註（搭配任一結果）。
	ResolutionOther AnomalyResolutionReason = "OTHER"
)

// AnomalyResolutionReasonValues 回傳全部人工結案的原因代碼，與 proto 的 PaymentAnomalyResolutionReason 相同（不含 UNSPECIFIED）。
func AnomalyResolutionReasonValues() []AnomalyResolutionReason {
	return []AnomalyResolutionReason{ResolutionProviderExecutedNotRecorded, ResolutionProviderNotExecuted, ResolutionDuplicate, ResolutionOther}
}

// ResolutionNoteMaxRunes 是人工結案備註的長度上限（去除首尾空白後，以 rune 計）：備註進 adminaudit 的 detail（上限 2048 位元組），255 個字元最多約 765 位元組。
const ResolutionNoteMaxRunes = 255

// RefundRequest 是建立退款的輸入：OrderNo 指向訂單應收；RequestKey 是呼叫端給的冪等鍵（同一張訂單應收內唯一，同一把鍵重送得到同一筆退款）；
// Auto 標示自動退款（D5 修訂：由訂單一方判斷與發起，payment 只標記）。
type RefundRequest struct {
	OrderNo    string
	Amount     int64
	RequestKey string
	Auto       bool
}

// Refund 是退款的對外視圖。ReversalAmount 是這筆退款在 provider 接受之後要向賣家收回的額度（轉帳還沒承諾金額、沒有轉帳或沒有可撤額度時是 0），ReversalStatus 是收回的狀態。
type Refund struct {
	ID             uuid.UUID
	PaymentOrderID uuid.UUID
	// OrderNo 是退款所屬訂單應收的訂單編號（建立與讀取時帶上，給只認訂單編號的呼叫方）。
	OrderNo        string
	RequestKey     string
	Amount         int64
	Auto           bool
	Status         RefundStatus
	ReversalAmount int64
	ReversalStatus ReversalStatus
	// TopupStatus 是已算進轉帳金額的退款失敗之後，補差額轉帳給賣家的狀態（沒有要補的是 TopupNone）。
	TopupStatus TopupStatus
	// FailureReason 只在 RefundFailed 與 RefundAnomaly 有值。
	FailureReason string
	CreatedAt     time.Time
}

// RefundChanged 在退款進入終局（SUCCEEDED、FAILED）的 transaction 內發布：訂單與售後一方據此得知退款結果（舊事件 refund.succeeded、refund.failed 的來源）。
// RequestKey 是呼叫端建立退款時給的冪等鍵（售後以案件 ID 對回）。
type RefundChanged struct {
	OccurredAt    time.Time
	RefundID      uuid.UUID
	PaymentID     uuid.UUID
	OrderNo       string
	StoreID       int64
	BuyerID       int64
	Amount        int64
	Auto          bool
	RequestKey    string
	Status        RefundStatus
	Previous      RefundStatus
	FailureReason string
}

// AutoRefundCompleted 在自動退款（Auto 為真）成功的 transaction 內發布（D5）：通知買家與後台可見；失敗的自動退款只發 RefundChanged。
type AutoRefundCompleted struct {
	OccurredAt time.Time
	RefundID   uuid.UUID
	PaymentID  uuid.UUID
	OrderNo    string
	StoreID    int64
	BuyerID    int64
	Amount     int64
}
