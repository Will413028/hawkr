// Package model 定義刪帳流程的公開狀態與輸入。
package model

// Requester 是申請來源。
type Requester string

const (
	AccountOwner Requester = "ACCOUNT_OWNER"
	Admin        Requester = "ADMIN"
)

// State 是刪帳流程的結果。
type State string

const (
	PendingOrders State = "PENDING_ORDERS"
	Completed     State = "COMPLETED"
)

// RequestInput 包含帳號與申請稽核資料。
type RequestInput struct {
	AccountID       int64
	Requester       Requester
	OperatorAdminID *int64
	Reason          string
}
