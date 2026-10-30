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
	"seller":            {"identity", "media"},
	"content":           {"media"},
	"fulfillment":       {"buyer", "seller", "media"},
	"catalog":           {"seller", "buyer", "media", "fulfillment"},
	"productimport":     {"catalog", "media", "fulfillment"},
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
	"reviewqueue":       {"seller"},                                                       // 後台待辦：讀各來源發布的 pub_*_review_items 排序分頁。
	"agent":             {"ordering", "catalog", "buyer", "seller", "media"},
	"notification": {
		"identity", "buyer", "seller", "catalog", "ordering", "fulfillment", "payment", "invoice",
		"coupon", "aftersales", "review", "im", "social", "agent", "finance", "delegation",
	},
}

// InternalGraph 列出模組內每個 package 可直接 import 的 package。
type InternalGraph struct {
	Packages map[string][]string
}

// InternalDependencies 是已建立模組的內部 package 依賴圖。
var InternalDependencies = map[string]InternalGraph{
	"social": {Packages: map[string][]string{
		"actor":      {"model", "text", "visibility"},
		"comment":    {"actor", "model", "reads", "store", "text", "textcheck", "visibility"},
		"engage":     {"actor", "model", "reads", "store", "text", "visibility"},
		"forum":      {"actor", "model", "reads", "store", "text", "textcheck"},
		"hashtag":    {"text"},
		"jet/table":  {},
		"mediaref":   {"store"},
		"model":      {},
		"post":       {"hashtag", "mediaref", "model", "reads", "readcache", "store", "text", "textcheck", "visibility"},
		"pubsql":     {},
		"purge":      {"mediaref", "model", "readcache", "stats", "store"},
		"reads":      {"jet/table", "store", "visibility"},
		"readcache":  {},
		"sanction":   {"model", "reads", "readcache", "store", "text"},
		"stats":      {"readcache", "reads", "store"},
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
	"agent": {Packages: map[string][]string{"vendorportal": {"model", "store"}, "model": {}, "store": {}}},
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
		"profile":    {"exposure", "mediaref", "model", "payuni", "store", "stint"},
		"suspension": {"exposure", "model", "store", "stint"},
		"governance": {"mediaref", "model", "store", "stint", "text"}, // text：指令被擋下時的伺服器句子（ADR 2026-10-17 多語系）
		"measure":    {"closing", "exposure", "mediaref", "model", "store", "stint", "suspension", "text"},
		"closing":    {"exposure", "model", "payuni", "store", "stint", "suspension"},
		"mediaref":   {"model", "store"},
		"kyc":        {"agreement", "exposure", "mediaref", "model", "payuni", "store", "stint"},
		"payuni":     {"exposure", "model", "store", "stint"},
		"exposure":   {"model", "store", "stint"},
		"purge":      {"mediaref", "model", "store"},
		"prospect":   {"mediaref", "model", "store"},
		"views":      {"model", "store"},
		"block":      {"model", "store"},
		"devseed":    {"agreement", "kyc", "mediaref", "model", "payuni", "stint", "store"},
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
			"account":     {"model", "store"},
			"admin":       {"model", "store"},
			"applerevoke": {"model", "store"},
			"auth":        {"model", "session", "store"},
			"login":       {"model", "session", "store"},
			"provider":    {"apple", "clerk", "model", "oidc"},
			"session":     {"model", "store"},
			"apple":       {},
			"clerk":       {"oidc"},
			"model":       {},
			"oidc":        {},
			"store":       {},
		},
	},
	"media": {
		Packages: map[string][]string{"model": {}, "refsync": {"model"}},
	},
	"accountsuspension": {
		Packages: map[string][]string{}, // 只有根套件：編排，沒有內部套件。
	},
}
