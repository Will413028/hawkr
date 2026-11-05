// Package lockorder 是單體規則 4「跨模組 transaction 由下往上鎖列」的機械版本：全域順序表，以及在測試裡
// 檢查每個 transaction 實際鎖列順序的 pgx tracer。
//
// 原型（2026-11-05，架構 review 第 4 點）：全域順序的文字版仍在 backend/AGENTS.md 規則 4；採用時規則 4
// 改成指向 Modules，只留一份。
//
// 檢查的單位是模組（schema），不是表：模組內的順序（seller §18、social §6）不在這裡。一句 SQL 只碰自己模組的
// schema（規則 2），所以它鎖到的列屬於哪個模組，看它的表寫在哪個 schema 就知道。
package lockorder

import "slices"

// Modules 是跨模組 transaction 鎖列的全域順序，由下往上：一個 transaction 鎖過某個模組的列之後，不再等待
// 排在它前面的模組的列。
//
// 每個模組都排在它依賴的模組之後（internal/arch 的 TestLockOrderIsBottomUpOverTheDependencyTable 檢查），
// 所以上層呼叫下層的 API 時，要在鎖自己的列之前呼叫；下層通知上層（同 transaction 的事件）天然符合。
// 同一層的模組之間的先後是這張表決定的：identity → buyer → seller 照規則 4 的文字版；編排者
// （accountsuspension、accountdeletion）自己的列排在它們會碰到的模組之後。
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

// Exemption 是已知的倒退：先鎖的語句是 Held（<schema>.<sqlc 名稱>），之後第一次鎖 Then 裡的模組。每一筆都寫明為什麼
// 不會形成等待的環，以及還沒決定的事；決定之後刪掉這一筆。
type Exemption struct {
	Held string
	Then []string
	Why  string
}

// Exemptions 是目前的豁免。第一次以這個檢查跑全部 integration 測試（2026-11-05，dev fe7641a）時，正式程式的倒退只有這一處。
var Exemptions = []Exemption{
	{
		Held: "accountdeletion.PendingRequest",
		Then: []string{"buyer", "seller"},
		Why: "accountdeletion 在 transaction 一開始先鎖 identity 帳號列（request.go 的 lockAccount），才鎖自己的申請列，再鎖買家與店家；" +
			"鎖申請列的 transaction 都先鎖同一個帳號列，所以不會形成環。規則 4 的文字要求編排者自己的列最後鎖：" +
			"改程式（申請列移到買家、店家之後）或改規則（帳號列之後可以先鎖只由持有帳號列者鎖的列），待決定。",
	},
}

func exempt(held, then Acquisition) bool {
	for _, e := range Exemptions {
		if e.Held == held.Lock.Schema+"."+held.Statement && slices.Contains(e.Then, then.Lock.Schema) {
			return true
		}
	}
	return false
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
