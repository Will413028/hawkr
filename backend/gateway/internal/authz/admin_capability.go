package authz

// AdminCapability is the ADM 代行營運 authorization token (56 §6.1.1): one class of
// thing an administrator may do to a store while operating on its behalf inside a
// delegated operation session. The value is the lowercase dotted machine name the
// whole stack shares — this file, testdata/admin_capabilities.golden, the two
// account-service CHECK constraints (chk_aoal_capability_closed on the audit table,
// chk_sda_capabilities_closed on the authorization table) and the admin repo's
// policy.ts allow-list.
//
// AdminCapability and the seller-facing Capability are deliberately TWO types with
// TWO registries (56 §4.4.1). Merging them would mean a seller route that
// accidentally declared an admin token no longer panics at boot, which silently
// weakens the deny-by-default guarantee 46 號 built.
//
// The nine values are a closed enumeration. Adding one is a governance-level change
// (§8.1.0): Go constants, the golden file and both DB CHECKs move together, and the
// G2 gate (scripts/check-delegation-capability-closure.py) turns the PR red if only
// some of them do. None of the nine represents a fulfilment or money capability —
// that is the D2 leg of the four mechanical guarantees, and it is why the list is
// closed rather than open.
//
// Tokens stay OUT of CodeMeta, same call as the seller capabilities (capability.go):
// Gateway-internal authorization concept, does not cross service boundaries as a
// business code.
type AdminCapability string

// The nine capabilities, in §6.1.1 declaration order.
const (
	// AdminCapStorefrontRead — 查看店面內容（low）。唯讀，不落稽核 action。
	AdminCapStorefrontRead AdminCapability = "storefront.read"
	// AdminCapStorefrontWrite — 編輯店面內容（medium）。STOREFRONT_UPDATE / _LOGO_UPDATE / _COVER_UPDATE。
	AdminCapStorefrontWrite AdminCapability = "storefront.write"
	// AdminCapProductsRead — 查看商品（low）。
	AdminCapProductsRead AdminCapability = "products.read"
	// AdminCapProductsWrite — 編輯商品（medium）。PRODUCT_CREATE / PRODUCT_UPDATE。
	AdminCapProductsWrite AdminCapability = "products.write"
	// AdminCapProductsPublish — 上架與下架商品（high）。含 batch-unpublish。
	AdminCapProductsPublish AdminCapability = "products.publish"
	// AdminCapPostsRead — 查看貼文（low）。
	AdminCapPostsRead AdminCapability = "posts.read"
	// AdminCapPostsWrite — 編輯貼文（medium）。POST_CREATE / POST_UPDATE。
	AdminCapPostsWrite AdminCapability = "posts.write"
	// AdminCapPostsPublish — 發布與下架貼文（high）。含 POST_SCHEDULE。
	AdminCapPostsPublish AdminCapability = "posts.publish"
	// AdminCapOrdersRead — 查看訂單（high）。九值中唯一「唯讀但必落稽核」者
	// （ORDER_PII_REVEAL），例外由 order-service 自行斷言 audit_log_id 非空承擔。
	AdminCapOrdersRead AdminCapability = "orders.read"
)

// AllAdminCapabilities lists every delegated capability. Order matches the §6.1.1
// table so the golden file reads the same way.
//
// 媒體上傳刻意不在其中，也不新增第十個（§6.1.13）：上傳只在 media 自己的表插一列、
// 不碰 target store 的業務狀態；真正需要授權的是「把圖掛上去」，已由
// storefront.write / products.write / posts.write 各自守住。
var AllAdminCapabilities = []AdminCapability{
	AdminCapStorefrontRead,
	AdminCapStorefrontWrite,
	AdminCapProductsRead,
	AdminCapProductsWrite,
	AdminCapProductsPublish,
	AdminCapPostsRead,
	AdminCapPostsWrite,
	AdminCapPostsPublish,
	AdminCapOrdersRead,
}

// IsKnownAdmin reports whether cap is one of the nine delegated capability tokens.
// The empty string is not one of them — a route that forgot to declare a capability
// therefore fails at RegisterAdmin rather than mounting under no capability at all.
func IsKnownAdmin(cap AdminCapability) bool {
	for _, c := range AllAdminCapabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// ==================== 依賴閉包與風險等級 ====================

// adminCapabilityDependencies 是「授予 X 就必須同時授予哪些」的直接依賴表。
//
// 它與封閉九值同檔，因為兩者是同一個 SSOT 的兩半：值域說「有哪些能力」，本表說
// 「哪些組合是自洽的」。分兩個檔就會出現「新增了第十個 capability，但沒有人被迫回答
// 它依賴什麼」——而那個漏答不會讓任何東西轉紅。
//
// ⚠️ 這條規則在 DB 層**不存在**：chk_sda_capabilities_closed 的九個 CASE 是去重加總
// （PG 的 CHECK 不能有子查詢，做不出 DISTINCT），不是依賴檢查。授權建立路徑的
// fail-closed 保證因此完全落在 Gateway：MissingAdminCapabilityDependencies 不齊即
// 9287，在送出 gRPC 之前。前端 hawkr-admin 的 policy.ts 另有一份同內容的表，
// 那一份只用來收窄 UI 顯示（方向 fail-safe），不是保證。
//
// 依賴的語意是「沒有它，這個能力做出來的事在畫面上看不見也對不了帳」：編輯必須看得到
// 現況才知道自己改了什麼，發布必須能編輯才有東西可發。orders.read 沒有依賴——它是唯讀
// 的終點；風險高是因為 PII 揭露，與依賴無關。
//
// 表是**直接**依賴，不是遞移閉包；閉包由 MissingAdminCapabilityDependencies 展開。
// 現況兩者恰好相同（products.publish 直接列了 read 與 write），刻意如此：讀表的人不必
// 在腦中算一次遞移就知道完整答案。
var adminCapabilityDependencies = map[AdminCapability][]AdminCapability{
	AdminCapStorefrontRead:  {},
	AdminCapStorefrontWrite: {AdminCapStorefrontRead},
	AdminCapProductsRead:    {},
	AdminCapProductsWrite:   {AdminCapProductsRead},
	AdminCapProductsPublish: {AdminCapProductsRead, AdminCapProductsWrite},
	AdminCapPostsRead:       {},
	AdminCapPostsWrite:      {AdminCapPostsRead},
	AdminCapPostsPublish:    {AdminCapPostsRead, AdminCapPostsWrite},
	AdminCapOrdersRead:      {},
}

// Risk levels for the授權 UI。三值封閉：low / medium / high。
const (
	AdminCapRiskLow    = "low"
	AdminCapRiskMedium = "medium"
	AdminCapRiskHigh   = "high"
)

// adminCapabilityRisks 是勾選介面用來標示危險度的等級，取自 §6.1.1 的能力表。
//
// orders.read 是九值中唯一「唯讀卻是 high」的一項：它每一次揭露都會落一列
// ORDER_PII_REVEAL 稽核，風險來自個資而不是寫入。把它按「唯讀＝low」歸類，勾選畫面
// 就不會提醒任何人。
var adminCapabilityRisks = map[AdminCapability]string{
	AdminCapStorefrontRead:  AdminCapRiskLow,
	AdminCapStorefrontWrite: AdminCapRiskMedium,
	AdminCapProductsRead:    AdminCapRiskLow,
	AdminCapProductsWrite:   AdminCapRiskMedium,
	AdminCapProductsPublish: AdminCapRiskHigh,
	AdminCapPostsRead:       AdminCapRiskLow,
	AdminCapPostsWrite:      AdminCapRiskMedium,
	AdminCapPostsPublish:    AdminCapRiskHigh,
	AdminCapOrdersRead:      AdminCapRiskHigh,
}

// AdminCapabilityDependsOn 回傳 cap 的直接依賴（未知 token 回空 slice，不回 nil：
// 呼叫端會把它序列化成 JSON 陣列，nil 會變成 null 而前端對 dependsOn 做 .length）。
func AdminCapabilityDependsOn(cap AdminCapability) []AdminCapability {
	deps := adminCapabilityDependencies[cap]
	out := make([]AdminCapability, len(deps))
	copy(out, deps)
	return out
}

// AdminCapabilityRisk 回傳 cap 的風險等級；未知 token 回空字串。
func AdminCapabilityRisk(cap AdminCapability) string {
	return adminCapabilityRisks[cap]
}

// MissingAdminCapabilityDependencies 回傳「要讓 caps 這組能力自洽，還缺哪些」，
// 依 AllAdminCapabilities 的宣告順序排列且去重。回空 slice ＝ 閉包完整。
//
// 展開的是**遞移**閉包：即使日後某個能力只宣告一層依賴，缺的那一層自己的依賴也會被
// 一併算進來。少了這一步，「A 依賴 B、B 依賴 C」的表在只勾 A 時會回報「缺 B」，補了 B
// 之後才發現還缺 C —— 使用者要送出兩次才知道完整答案。
//
// 未知 token 一律忽略：值域檢查是呼叫端在此之前的責任（9260），在這裡再判一次會讓
// 同一個錯誤有兩個碼。
func MissingAdminCapabilityDependencies(caps []AdminCapability) []AdminCapability {
	have := make(map[AdminCapability]struct{}, len(caps))
	for _, c := range caps {
		have[c] = struct{}{}
	}

	missing := make(map[AdminCapability]struct{})
	// 佇列展開遞移閉包。封閉九值且表無環，迴圈必然停止；即便日後有人寫出環，
	// visited 也讓它停在有限步而不是掛住一個 request。
	queue := append([]AdminCapability(nil), caps...)
	visited := make(map[AdminCapability]struct{}, len(caps))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := visited[cur]; seen {
			continue
		}
		visited[cur] = struct{}{}
		for _, dep := range adminCapabilityDependencies[cur] {
			if _, ok := have[dep]; !ok {
				missing[dep] = struct{}{}
			}
			queue = append(queue, dep)
		}
	}

	out := make([]AdminCapability, 0, len(missing))
	for _, c := range AllAdminCapabilities {
		if _, ok := missing[c]; ok {
			out = append(out, c)
		}
	}
	return out
}
