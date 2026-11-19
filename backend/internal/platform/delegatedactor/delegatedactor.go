// Package delegatedactor 是代行寫入帶給資源模組的「代行者」：資源模組的寫入方法明確收一個 *Actor（店主寫入傳 nil，A7-D27），在自己的 transaction 內為寫入發
// 領域事件並附上它，稽核的訂閱者同 transaction 寫 SUCCEEDED。資源模組只傳遞它、不解讀它、不 import delegation，所以放在 platform。
package delegatedactor

// AuditHandler 是代行寫入的稽核 handler 在 events.Bus 上的註冊名稱：delegation 以它訂閱 social 的貼文寫入事件，social 帶代行者發布事件時用
// events.PublishRequired 指名要求它存在，沒有就失敗關閉（A7-D27）。
const AuditHandler = "delegation.audit"

// Actor 是一次代行寫入的代行者與請求資料。欄位全是事實，不含判斷：授權與稽核由訂閱者在同一個 transaction 內檢查與寫入。
type Actor struct {
	SessionID int64
	AdminID   int64
	// StoreID 是代行 session 的目標店家；資源模組的事件帶它，訂閱者核對它與被寫的店家相同。
	StoreID int64
	// RequestID 是 request_id（AIP-155），同一個 session 內去重。
	RequestID    string
	Reason       string
	NotifySeller bool
	// PayloadHash 綁定 RequestID 與請求內容。
	PayloadHash string
	// ResourceVersionBefore 是請求帶的資源版本（寫入前），沒有則為 nil。
	ResourceVersionBefore *int64
	CorrelationID         string
	UserAgent             string
}
