package sse

// Viewer 是一條連線的觀看者此刻的業務身分（沒有的是 0，訪客兩者皆 0）。
type Viewer struct {
	BuyerID int64
	StoreID int64
}

// PreparedEvent 是一個事件與觀看者無關的解析結果，Hub 對每個事件只準備一次。
type PreparedEvent interface {
	// Variant 回傳這個觀看者看到的版本的鍵：鍵相同的觀看者得到逐位元相同的 payload。
	// 一個事件的版本數有限（單筆事件只有「是我」與「不是我」兩種），不隨連線數增加。
	Variant(v Viewer) string
	// Render 產生這個觀看者看到的 payload。失敗時事件不得送出（fail closed）。
	Render(v Viewer) ([]byte, error)
}

// Personalizer 把一個內部形狀的事件解析成 PreparedEvent。解析失敗回 error：
// 內部身分欄位不得原樣送到公開的串流，所以呼叫端必須丟掉這個事件，不是改送原始 payload。
// 回傳的 PreparedEvent 只在同一個 goroutine 內依序使用。
type Personalizer func(eventName string, data []byte) (PreparedEvent, error)
