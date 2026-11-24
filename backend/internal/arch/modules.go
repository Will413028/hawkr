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
	"crm":               {"buyer", "seller"}, // 訂閱 buyer.BuyerDeleted、seller.StoreRetired。
	"finance":           {"aftersales", "ordering", "payment", "fulfillment"},
	"social":            {"buyer", "seller", "media"},
	"discovery":         {"ordering", "catalog", "buyer", "seller", "review"},
	"delegation":        {"identity", "seller", "ordering", "catalog", "social", "media"},            // 授權、session、稽核與編排全由 delegation 擁有。
	"accountdeletion":   {"identity", "buyer", "seller"},                                             // 刪除帳號的流程管理者。
	"accountsuspension": {"identity", "seller"},                                                      // 停權與解除的唯一入口：鎖帳號、確認沒有店家、再呼叫 identity。
	"moderation":        {"social", "seller", "buyer", "accountsuspension", "media", "review", "im"}, // 社群、評價與聊天訊息的檢舉與處置：案件、檢舉、內容快照；處分經各擁有者模組建立（評價、聊天訊息的隱藏與還原經 review、im）；快照引用的圖片在 media 登記引用（證據保全）。
	"reviewqueue":       {"seller", "payment", "moderation"},                                         // 後台待辦：讀各來源發布的 pub_*_review_items 排序分頁。
	"procurement":       {"ordering", "catalog", "buyer", "seller"},                                  // 採購（RFQ）；事件由 notification 訂閱，procurement 不依賴 notification（A9-D3）。
	"assistant":         {"ordering", "catalog", "buyer"},                                            // 買家助理（A9-D3）。
	"imageai":           {"seller", "media"},                                                         // AI 修圖與配額（A9-D3）。
	"notification": {
		"identity", "buyer", "seller", "review", "im", "social", "procurement", "delegation", "payment", "accountsuspension", "moderation",
	}, // 訂閱與讀取已在單體的來源模組；還沒搬的（catalog、ordering、fulfillment、invoice、coupon、aftersales、finance）隨各自搬遷時加（A8 D13）。
}

// InternalGraph 列出模組內每個 package 可直接 import 的 package。
type InternalGraph struct {
	Packages map[string][]string
}

// InternalDependencies 是已建立模組的內部 package 依賴圖。
var InternalDependencies = map[string]InternalGraph{
	"coupon": {Packages: map[string][]string{
		"admin":      {"campaign", "model", "voucher"},
		"campaign":   {"model", "rows", "store"},
		"expiry":     {"model", "store"},
		"membership": {"model", "store"},
		"model":      {"text"},
		"purge":      {"model", "store"},
		"qualify":    {"model", "store"},
		"read":       {"model", "qualify", "rows", "store"},
		"rows":       {"model", "store"},
		"store":      {},
		"text":       {},
		"voucher":    {"model", "qualify", "rows", "store"},
	}},
	"notification": {Packages: map[string][]string{
		"announce": {"intake", "model", "store"},
		"app":      {"model", "registry", "store", "text"},
		"deliver":  {"intake", "model", "registry", "store"},
		"intake":   {"model", "registry", "store"},
		"model":    {},
		"purge":    {"model", "store"},
		"registry": {"model", "text"},
		"store":    {},
		"stream":   {"model", "store"},
		"suppress": {"store"},
		"text":     {},
	}},
	"crm": {Packages: map[string][]string{
		"model":   {},
		"project": {"model", "store"},
		"purge":   {"store"},
		"read":    {"model", "store", "text"},
		"store":   {},
		"text":    {},
		"write":   {"model", "store", "text"},
	}},
	"im": {Packages: map[string][]string{
		"live":  {"model", "store", "text"},
		"model": {},
		"purge": {"model", "store"},
		"read":  {"model", "rows", "store", "text"},
		"rows":  {"model", "store"},
		"store": {},
		"text":  {},
		"write": {"live", "model", "rows", "store", "text"},
	}},
	"review": {Packages: map[string][]string{
		"model": {},
		"purge": {"model", "rows", "store"},
		"read":  {"model", "rows", "store", "text"},
		"rows":  {"model", "store"},
		"store": {},
		"text":  {},
		"write": {"model", "rows", "store", "text"},
	}},
	"imageai": {Packages: map[string][]string{
		"admin":     {"model", "store", "text"}, // 後台的風格與額度維護（7 支）；縮圖與 seller 經介面，由根套件組裝
		"limit":     {},
		"model":     {},
		"quota":     {"model", "store"},
		"store":     {},
		"style":     {"model", "store", "text"},
		"text":      {},
		"transform": {"model", "text"},
	}},
	"assistant": {Packages: map[string][]string{
		"adksession": {},                // ADK session.Service 的 pgx 實作，schema assistant（A9-D27）
		"buyeragent": {"model", "text"}, // 買家 agent 與 3 支 tool；tool 身分用 model.Tool，coordinator 的退貨流程取共用文案
		"feature":    {},
		"history":    {"buyeragent", "model"}, // 買家最近一次對話的歷史（GetLatestAssistantSession）
		"model":      {},
		"text":       {},
		"turn":       {"buyeragent", "model", "text"}, // 輪次執行器、每輪事件緩衝與同對話單一輪次
	}},
	"procurement": {Packages: map[string][]string{
		"candidate":    {},                        // 候選搜尋：翻頁收集、只收 OPEN 且沒封鎖這位買家的店（S9、A9-D29）
		"conversation": {"model", "run", "store"}, // 買家的對話管理：keyset 列表、改名、刪除並取消進行中的 run（A9-3b-3a、A9-D31 Q7–Q9）
		"extract":      {"slots"},                 // 一次模型呼叫把買家訊息抽成欄位 patch，呼叫內重試一次（A9-3b-2c、A9-D28(2)）
		"model":        {},
		"planner":      {"model", "slots"},                                                   // 一輪釐清要問什麼、說哪一類話：問題挑選、act 優先序（A9-3b-2a）
		"reply":        {"model", "planner", "slots", "text"},                                // act 模板與問題卡、被拒原因的語系文字，沒有 LLM 敘述（A9-D24）
		"resolver":     {},                                                                   // 品項用詞對分類樹，人工同義詞（A9-D7）
		"rfq":          {"candidate", "model", "rfqhash", "search", "store", "text"},         // 詢價發布鏈：價格預覽、確認快照、發布核准、執行發布，hash 由不可變的列重算（A9-3b-3c、A9-D31）
		"provisioning": {"quote", "run", "store"},                                            // 接受報價後的建單 worker 與 reconciler：經 port 呼叫舊 ordering、用盡或拒絕標建單失敗並解除 live 鎖（A9-3c-4、A9-D45(1)）
		"quote":        {"model", "rfq", "rfqhash", "run", "store"},                          // 買家對賣家回價的使用：列出目前版本、入圍清單、接受報價並同 transaction 排入建單 job（A9-3b-3d）
		"rfqhash":      {},                                                                   // 價格預覽、確認快照、受眾與賣家可見內容的 hash（SHA-256 對固定欄位順序的 JSON，A9-D31 Q5）
		"retention":    {"store"},                                                            // 買家關閉後刪除與匿名（S10）
		"run":          {"store"},                                                            // run 帳本：送出與配額、受守衛的狀態轉換與事件續傳、取消、reconciler、事件保留期、WatchRun 的共用 listener（A9-D6、A9-D20）
		"sellerside":   {"model", "quote", "rfq", "store"},                                   // 賣家的詢價清單與詳情、回價草稿、送出、撤回與婉拒：邀請列定可見性、冪等重送先於版本檢查、hash 重用 quote.PayloadHash
		"search":       {"candidate", "model", "reply", "resolver", "run", "slots", "store"}, // CANDIDATE_SEARCH 的 executor 與開始搜尋的 use case：品項解析定輸入、單一 transaction 提交（A9-3b-2d、A9-D29）
		"slots":        {"model", "store"},                                                   // 需求欄位的驗證與套用、本輪記下的欄位（A9-3b-2a）
		"snapshot":     {"model", "rfq", "run", "slots", "store", "turn"},                    // GetRuntimeSnapshot 的讀取模型：對話、訊息、run、待回答的中斷、發布結果在同一份快照內讀齊（A9-D30(4)）
		"store":        {},
		"text":         {},                                                                            // 模組句子（ADR 2026-10-17 多語系）
		"turn":         {"extract", "model", "planner", "reply", "resolver", "run", "slots", "store"}, // TURN 與 CLARIFICATION_RESUME 的 executor 與 ResumeRun 的 use case：抽取、套用、解析品項、規劃、單一 transaction 提交（A9-3b-2c、A9-3b-2e）
	}},
	"moderation": {Packages: map[string][]string{
		"handle":    {"model", "snapshot", "store", "text", "view"},
		"model":     {},
		"report":    {"model", "snapshot", "store", "text", "view"}, // text：送出被擋下時的伺服器句子（ADR 2026-10-17 多語系）
		"retention": {"model", "store"},
		"snapshot":  {"model"},
		"store":     {},
		"text":      {},
		"view":      {"model", "snapshot", "store", "text"}, // snapshot：案件的店家取自快照
	}},
	"payment": {Packages: map[string][]string{"account": {"model", "store"}, "charge": {"model", "store"}, "model": {}, "store": {}}},
	"social": {Packages: map[string][]string{
		"actor":      {"model", "text", "visibility"},
		"comment":    {"actor", "model", "reads", "store", "text", "textcheck", "visibility"},
		"engage":     {"actor", "model", "reads", "store", "text", "visibility"},
		"forum":      {"actor", "model", "reads", "store", "text", "textcheck"},
		"hashtag":    {"text"},
		"jet/table":  {},
		"model":      {},
		"post":       {"hashtag", "model", "reads", "readcache", "schedule", "store", "text", "textcheck", "visibility"},
		"pubsql":     {},
		"purge":      {"model", "readcache", "schedule", "stats", "store"},
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
		"profile":    {"model", "store", "stint"},
		"address":    {"model", "store", "stint"},
		"favorite":   {"model", "store", "stint"},
		"follow":     {"model", "store", "stint"},
		"block":      {"model", "store", "stint"},
		"visit":      {"model", "store", "stint"},
		"restock":    {"model", "store", "stint"},
		"membership": {"model", "store", "stint"},
		"deletion":   {"model", "store"},
		"stint":      {"model", "store"},
		"model":      {}, "store": {},
	}},
	"seller": {Packages: map[string][]string{
		"onboarding": {"agreement", "model", "store", "stint"},
		"agreement":  {"model", "store"},
		"profile":    {"exposure", "model", "store", "stint"},
		"suspension": {"exposure", "model", "store", "stint"},
		"governance": {"model", "store", "stint", "text"}, // text：指令被擋下時的伺服器句子（ADR 2026-10-17 多語系）
		"measure":    {"closing", "exposure", "model", "store", "stint", "suspension", "text"},
		"closing":    {"exposure", "model", "store", "stint", "suspension"},
		"kyc":        {"agreement", "exposure", "model", "store", "stint", "text"},
		"payout":     {"exposure", "store", "stint"},
		"exposure":   {"model", "store", "stint"},
		"purge":      {"model", "store"},
		"prospect":   {"model", "store"},
		"views":      {"model", "store"},
		"block":      {"model", "store"},
		"devseed":    {"agreement", "kyc", "model", "stint", "store"},
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
			"banner":     {"images", "model", "store", "validation"},
			"images":     {"model"},
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
		Packages: map[string][]string{
			"image":   {},
			"model":   {},
			"reclaim": {"model", "storage", "store"},
			"secure":  {},
			"storage": {},
			"store":   {},
			"text":    {},
			"upload":  {"image", "model", "secure", "storage", "store", "text"},
		},
	},
	"accountsuspension": {
		Packages: map[string][]string{}, // 只有根套件：編排，沒有內部套件。
	},
}
