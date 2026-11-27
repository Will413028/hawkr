package authz

// AdminCapability is the ADM 代行營運 authorization token (56 §6.1.1): one class of
// thing an administrator may do to a store while operating on its behalf inside a
// delegated operation session. The value is the lowercase dotted machine name the
// whole stack shares — this file, testdata/admin_capabilities.golden, the two
// delegation-module CHECK constraints (chk_aoal_capability_closed on the audit table,
// chk_sda_capabilities_closed on the authorization table) and the admin repo's
// policy.ts allow-list.
//
// AdminCapability and the seller-facing Capability are deliberately TWO types with
// TWO registries (56 §4.4.1). Merging them would mean a seller route that
// accidentally declared an admin token no longer panics at boot, which silently
// weakens the deny-by-default guarantee 46 號 built.
//
// The ten values are a closed enumeration. Adding one is a governance-level change
// (§8.1.0): Go constants, the golden file and both DB CHECKs move together, and the
// G2 gate (scripts/check-delegation-capability-closure.py) turns the PR red if only
// some of them do. None of the ten represents a fulfilment or money capability —
// that is the D2 leg of the four mechanical guarantees, and it is why the list is
// closed rather than open.
//
// Tokens stay OUT of CodeMeta, same call as the seller capabilities (capability.go):
// Gateway-internal authorization concept, does not cross service boundaries as a
// business code.
type AdminCapability string

// The ten capabilities, in §6.1.1 declaration order.
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
	// AdminCapOrdersRead — 查看訂單（high）。十值中唯一「唯讀但必落稽核」者
	// （ORDER_PII_REVEAL），例外由 order-service 自行斷言 audit_log_id 非空承擔。
	AdminCapOrdersRead AdminCapability = "orders.read"
	// AdminCapMediaUpload — 上傳機敏檔案（low）。由單體的 MediaDelegatedService 與代行段上傳路由（/api/files/delegated/secure_uploads/**）檢查；
	// 上傳只產生會過期的暫存，不讓檔案生效，掛上時仍由擁有者的寫入能力把關。這個 gateway 沒有任何路由宣告它，
	// 列在這裡是為了能被授權（授權建立與能力目錄都以本表為準）。
	AdminCapMediaUpload AdminCapability = "media.upload"
)

// AllAdminCapabilities lists every delegated capability. Order matches the §6.1.1
// table so the golden file reads the same way.
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
	AdminCapMediaUpload,
}

// IsKnownAdmin reports whether cap is one of the ten delegated capability tokens.
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
	AdminCapMediaUpload:     {},
}

// Risk levels for the授權 UI。三值封閉：low / medium / high。
const (
	AdminCapRiskLow    = "low"
	AdminCapRiskMedium = "medium"
	AdminCapRiskHigh   = "high"
)

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
	AdminCapMediaUpload:     AdminCapRiskLow,
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
// 展開的是**遞移**閉包：即使某個能力只宣告一層依賴，缺的那一層自己的依賴也會被
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
