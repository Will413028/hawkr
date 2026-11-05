// Package lockorder 是單體規則 4「跨模組 transaction 由下往上鎖列」的機械版本：全域順序表，以及在測試裡
// 檢查每個 transaction 實際鎖列順序的 pgx tracer。
//
// 全域順序只在 Modules 一份（backend/AGENTS.md 規則 4 指向它）。順序對每一次鎖列都生效，沒有豁免清單：
// 檢查器不知道哪些表只有單一模組會鎖，所以不分「兩個以上模組會鎖的列」與其他列；已知的倒退改程式，不登記豁免。
//
// 檢查的單位是模組（schema），不是表：模組內的順序（seller §18、social §6）不在這裡。一句 SQL 只碰自己模組的
// schema（規則 2），所以它鎖到的列屬於哪個模組，看它的表寫在哪個 schema 就知道。
//
// # 不在檢查範圍
//
// 這是測試期的啟發式檢查，不是死鎖的證明。下面這些不看，通過不代表沒有這類順序問題：
//   - 外鍵檢查隱含的鎖（被參照列上的 FOR KEY SHARE）：不出現在 SQL 文字裡；規則 3 不建跨 schema FK，
//     所以只可能在同一模組內。
//   - trigger 與函式（含 PL/pgSQL）內部的 SQL：tracer 只看到呼叫它們的那一句。
//   - advisory lock：沒有表，看不出屬於哪個模組。
//   - 單純 INSERT（含 COPY FROM）撞到另一個未 commit 的 INSERT 的唯一鍵而等待：不鎖既有列，不分類。
//   - 同一模組裡不同根的列（例如兩個帳號）：判定單位是模組，模組內的順序靠各模組自己的規定與個案死鎖測試。
//   - 測試檔（_test.go）自己開的 transaction：以開 transaction 的呼叫處是否在 _test.go 判斷，整個 transaction
//     跳過。這個判斷是看呼叫堆疊的啟發式，helper 或 closure 讓呼叫處落在非測試檔時會被當成單體程式檢查，反之亦然。
//
// 有涵蓋的送出路徑：Query、Exec、QueryRow、SendBatch（batch 內每句）；CopyFrom 視同單純 INSERT，不加鎖。
package lockorder

// Modules 是跨模組 transaction 鎖列的全域順序，由下往上：一個 transaction 鎖過某個模組的列之後，不再等待
// 排在它前面的模組的列。
//
// 每個模組都排在它依賴的模組之後（internal/arch 的 TestLockOrderIsBottomUpOverTheDependencyTable 檢查），
// 所以上層呼叫下層的 API 時，要在鎖自己的列之前呼叫；下層通知上層（同 transaction 的事件）天然符合。
// 同一層的模組之間的先後是這張表決定的：identity → buyer → seller；編排者
// （accountsuspension、accountdeletion）自己的列排在它們會碰到的模組之後；最後是 notification：它只訂閱別人的
// 事件、寫通知列，不被任何模組依賴，通知列是同一個 transaction 裡最後才寫的副作用，所以排在編排者之後。
//
// 不在表裡的 schema（river、pg_catalog 等）不檢查。新模組加進 internal/arch 的依賴表時，同一次變更加進這裡。
var Modules = []string{
	"identity",
	"media",
	"payment",
	"buyer",
	"seller",
	"content",
	"fulfillment",
	"catalog",
	"productimport",
	"invoice",
	"cart",
	"coupon",
	"ordering",
	"aftersales",
	"review",
	"im",
	"crm",
	"finance",
	"social",
	"discovery",
	"agent",
	"reviewqueue",
	"delegation",
	"accountsuspension",
	"accountdeletion",
	"notification",
}

var ranks = func() map[string]int {
	m := make(map[string]int, len(Modules))
	for i, name := range Modules {
		m[name] = i
	}
	return m
}()

// Rank 回傳模組在全域順序裡的位置；不是模組的 schema 回傳 false。
func Rank(schema string) (int, bool) {
	r, ok := ranks[schema]
	return r, ok
}
