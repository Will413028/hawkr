package agent

// ADKAppName 是 ADK session 在 Postgres 上的分區鍵。用具名型別而不是字串常數，
// 是為了讓「把 assistant 的名字傳給 procurement 的 runner」這種錯誤在編譯期就擋下來
// ——它造成的症狀是讀與寫落在不同分區、對話靜默分裂，沒有任何錯誤訊息。
// ⚠️ procurement 刻意使用不同名稱（ADR D12），那不是 bug。
type ADKAppName string

const (
	AppNameAssistant   ADKAppName = "hawkr_ai"
	AppNameProcurement ADKAppName = "hawkr_procurement"
)
