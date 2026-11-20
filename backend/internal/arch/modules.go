// Package arch 以測試守住單體的依賴方向。
//
// 規則的來源是 docs/adr/2026-09-24-go-modular-monolith.md D2；這裡是它的機械版本。
// 改 Dependencies 等於改架構：先補 ADR 或計畫說明理由，再改這張表。
package arch

// ModulePath 是單體的 Go module path。
const ModulePath = "github.com/Will413028/hawkr/backend"

// Dependencies 列出每個模組可以依賴的其他模組。
//
// 「依賴」只能是 import 對方的根套件（internal/modules/<名稱>），也就是它的公開 API；
// 對方的子套件一律不得 import。notification 依賴的是它訂閱事件的發布方。
var Dependencies = map[string][]string{
	"identity":          {},
	"media":             {},
	"payment":           {},
	"buyer":             {"identity", "media"},
	"seller":            {"identity", "media", "payment"},
	"content":           {"media", "social"}, // banner 的貼文目標：建立時經 social 確認公開可讀，讀取時隱藏失效的（A7-D28、A7-D23 O5）；social 不得依賴 content。
	"fulfillment":       {"buyer", "seller", "media"},
	"catalog":           {"seller", "buyer", "media", "fulfillment"},
	"invoice":           {"payment", "fulfillment", "buyer", "seller", "media"},
	"cart":              {"catalog", "buyer", "seller"},
	"coupon":            {"identity", "buyer", "seller"},
	"ordering":          {"cart", "coupon", "catalog", "fulfillment", "payment", "invoice", "buyer", "seller", "identity", "media"},
	"aftersales":        {"ordering", "coupon", "payment", "fulfillment", "buyer", "seller", "media"},
	"review":            {"ordering", "catalog", "buyer", "seller", "media"},
	"im":                {"ordering", "buyer", "seller", "media"},
	"crm":               {"ordering", "coupon", "buyer", "identity"},
	"finance":           {"aftersales", "ordering", "payment", "fulfillment"},
	"social":            {"buyer", "seller", "media"},
	"discovery":         {"ordering", "catalog", "buyer", "seller", "review"},
	"delegation":        {"identity", "seller", "ordering", "catalog", "social", "media"}, // 授權、session、稽核與編排全由 delegation 擁有。
	"accountdeletion":   {"identity", "buyer", "seller"},                                  // 刪除帳號的流程管理者。
	"accountsuspension": {"identity", "seller"},                                           // 停權與解除的唯一入口：鎖帳號、確認沒有店家、再呼叫 identity。
	"moderation":        {"social", "seller", "buyer", "accountsuspension", "media"},      // 社群檢舉與處置：案件、檢舉、內容快照；處分經各擁有者模組建立；快照引用的圖片在 media 登記引用（證據保全）。
	"reviewqueue":       {"seller", "payment", "moderation"},                              // 後台待辦：讀各來源發布的 pub_*_review_items 排序分頁。
	"procurement":       {"ordering", "catalog", "buyer", "seller"},                       // 採購（RFQ）；事件由 notification 訂閱，procurement 不依賴 notification（A9-D3）。
	"assistant":         {"ordering", "catalog", "buyer"},                                 // 買家助理（A9-D3）。
	"imageai":           {"seller", "media"},                                              // AI 修圖與配額（A9-D3）。
	"notification": {
		"identity", "buyer", "seller", "catalog", "ordering", "fulfillment", "payment", "invoice",
		"coupon", "aftersales", "review", "im", "social", "procurement", "finance", "delegation",
	},
}

// InternalGraph 列出模組內每個 package 可直接 import 的 package。
type InternalGraph struct {
	Packages map[string][]string
}

// InternalDependencies 是已建立模組的內部 package 依賴圖。
var InternalDependencies = map[string]InternalGraph{
	"im": {Packages: map[string][]string{
		"live":     {"model", "store", "text"},
		"mediaref": {"store"},
		"model":    {},
		"purge":    {"mediaref", "model", "store"},
		"read":     {"model", "rows", "store", "text"},
		"rows":     {"model", "store"},
		"store":    {},
		"text":     {},
		"write":    {"live", "mediaref", "model", "rows", "store", "text"},
	}},
	"review": {Packages: map[string][]string{
		"mediaref": {"store"},
		"model":    {},
		"purge":    {"mediaref", "model", "rows", "store"},
		"read":     {"model", "rows", "store", "text"},
		"rows":     {"model", "store"},
		"store":    {},
		"text":     {},
		"write":    {"mediaref", "model", "rows", "store", "text"},
	}},
	"imageai": {Packages: map[string][]string{
		"feature": {},
		"model":   {},
		"store":   {},
	}},
	"assistant": {Packages: map[string][]string{
		"feature": {},
		"model":   {},
		"store":   {},
	}},
	"procurement": {Packages: map[string][]string{
		"feature": {},
		"model":   {},
		"store":   {},
	}},
	"moderation": {Packages: map[string][]string{
		"handle":    {"model", "snapshot", "store", "text", "view"},
		"mediaref":  {"store"},
		"model":     {},
		"report":    {"model", "snapshot", "store", "text", "view"}, // text：送出被擋下時的伺服器句子（ADR 2026-10-17 多語系）
		"retention": {"mediaref", "model", "store"},
		"snapshot":  {"model"},
		"store":     {},
		"text":      {},
		"view":      {"model", "snapshot", "store", "text"}, // snapshot：案件的店家取自快照
	}},
	"payment": {Packages: map[string][]string{"account": {"model", "store"}, "model": {}, "store": {}}},
	"social": {Packages: map[string][]string{
		"actor":      {"model", "text", "visibility"},
		"comment":    {"actor", "model", "reads", "store", "text", "textcheck", "visibility"},
		"engage":     {"actor", "model", "reads", "store", "text", "visibility"},
		"forum":      {"actor", "model", "reads", "store", "text", "textcheck"},
		"hashtag":    {"text"},
		"jet/table":  {},
		"mediaref":   {"store"},
		"model":      {},
		"post":       {"hashtag", "mediaref", "model", "reads", "readcache", "schedule", "store", "text", "textcheck", "visibility"},
		"pubsql":     {},
		"purge":      {"mediaref", "model", "readcache", "schedule", "stats", "store"},
		"reads":      {"jet/table", "store", "visibility"},
		"readcache":  {},
		"sanction":   {"model", "reads", "readcache", "store", "text"},
		"schedule":   {"model", "readcache", "store"},
		"stats":      {"model", "readcache", "reads", "schedule", "store"},
		"store":      {},
		"text":       {},
		"textcheck":  {"text"},
		"visibility": {"jet/table", "pubsql"},
	}},
	"reviewqueue": {Packages: map[string][]string{
		"items": {"model", "store"},
		"model": {},
		"store": {},
	}},
	"discovery": {Packages: map[string][]string{
		"ranking": {"model", "store"},
		"model":   {},
		"store":   {},
	}},
	"accountdeletion": {Packages: map[string][]string{
		"request": {"model", "store"},
		"model":   {},
		"store":   {},
	}},
	"buyer": {Packages: map[string][]string{
		"profile":    {"mediaref", "model", "store", "stint"},
		"address":    {"model", "store", "stint"},
		"favorite":   {"model", "store", "stint"},
		"follow":     {"model", "store", "stint"},
		"block":      {"model", "store", "stint"},
		"visit":      {"model", "store", "stint"},
		"restock":    {"model", "store", "stint"},
		"membership": {"model", "store", "stint"},
		"deletion":   {"mediaref", "model", "store"},
		"mediaref":   {"model", "store"},
		"stint":      {"model", "store"},
		"model":      {}, "store": {},
	}},
	"seller": {Packages: map[string][]string{
		"onboarding": {"agreement", "model", "store", "stint"},
		"agreement":  {"model", "store"},
		"profile":    {"exposure", "mediaref", "model", "store", "stint"},
		"suspension": {"exposure", "model", "store", "stint"},
		"governance": {"mediaref", "model", "store", "stint", "text"}, // text：指令被擋下時的伺服器句子（ADR 2026-10-17 多語系）
		"measure":    {"closing", "exposure", "mediaref", "model", "store", "stint", "suspension", "text"},
		"closing":    {"exposure", "model", "store", "stint", "suspension"},
		"mediaref":   {"model", "store"},
		"kyc":        {"agreement", "exposure", "mediaref", "model", "store", "stint"},
		"payout":     {"exposure", "store", "stint"},
		"exposure":   {"model", "store", "stint"},
		"purge":      {"mediaref", "model", "store"},
		"prospect":   {"mediaref", "model", "store"},
		"views":      {"model", "store"},
		"block":      {"model", "store"},
		"devseed":    {"agreement", "kyc", "mediaref", "model", "stint", "store"},
		"stint":      {"model", "store"},
		"model":      {"text"}, "store": {}, "text": {},
	}},
	"delegation": {Packages: map[string][]string{
		"authorization": {"model", "store"},
		"audit":         {"model", "store"},
		"model":         {},
		"session":       {"model", "store"},
		"store":         {},
	}},
	"content": {
		Packages: map[string][]string{
			"appversion": {"model", "store", "validation"},
			"banner":     {"images", "mediaref", "model", "store", "validation"},
			"images":     {},
			"mediaref":   {"store"},
			"model":      {},
			"store":      {},
			"validation": {},
		},
	},
	"identity": {
		Packages: map[string][]string{
			"account":  {"model", "store"},
			"admin":    {"model", "store"},
			"auth":     {"model", "session", "store"},
			"login":    {"model", "session", "store"},
			"provider": {"model", "oidc"},
			"session":  {"model", "store"},
			"model":    {},
			"oidc":     {},
			"store":    {},
		},
	},
	"media": {
		Packages: map[string][]string{"model": {}, "refsync": {"model"}},
	},
	"accountsuspension": {
		Packages: map[string][]string{}, // 只有根套件：編排，沒有內部套件。
	},
}
