/**
 * 帳號私有瀏覽器 storage key 的單一真相來源（hawkr-website#382 續）。
 *
 * 這支模組刻意零 import：任何 feature 引用它都不帶 bundle 成本，也不可能
 * 產生循環依賴。所有 key 一律在此定義，各功能模組反向 import——方向反過來
 * 才能讓 registry 具權威性，而不是「另一份會 drift 的複本」。
 *
 * #382 的修復建立了 syncLocalPrivateState 這個統一清除入口，但入口本身只登記
 * 了當下知道的 2 把 key，於是 cartUiStore 與賣家中心 5 個 store 全數漏網。
 * 沒有「所有本地 key 都必須在此登記」的強制檢查，統一入口自己也會漏——
 * tests/local-private-registry.test.ts 就是那個檢查點。
 */

export type StorageArea = "local" | "session";

/**
 * 何時清除這把 key。
 *
 * - `any-transition`（預設）：擁有者一變就清，包含「訪客 → 登入」。絕大多數
 *   帳號私有狀態都屬於這類：它們是登入後才產生的，沒有理由跨越任何身份轉換。
 * - `leaving-a-signed-in-owner`：**只有在前一個擁有者是「已登入帳號」時才清**
 *   （登出、或直接換另一個帳號）；「訪客 → 登入」不清。給那種刻意要把 pre-auth
 *   狀態帶過登入的資料用——它們的存在意義就是跨越那一道轉換。
 *
 * 這個維度是 2026-08-07 補上的：當時把 hawkr-agent 搜尋 handoff 當成一般帳號
 * 私有狀態登記，結果在它唯一該存活的時刻（訪客打完字 → 登入 → 進 Agent）被清掉，
 * 打掉 hawkr-agent-live e2e。第一版的處置是把那把 key 移出 registry 丟進
 * 「裝置層級」白名單，但那是止血：它裝著使用者輸入的字，根本不是裝置層級資料，
 * 而且「A 登出 → B 登入」這條洩漏就這樣被放掉了。正解是讓 registry 有能力表達
 * 「跨得過登入、但跨不過換人」。
 */
export type ClearPolicy = "any-transition" | "leaving-a-signed-in-owner";

export interface LocalPrivateKeySpec {
	readonly key: string;
	/** 這把 key 實際寫進哪些 storage（結帳草稿橫跨兩者）。 */
	readonly in: readonly StorageArea[];
	/** 省略 = `any-transition`。 */
	readonly clearOn?: ClearPolicy;
}

export const LOCAL_PRIVATE_KEYS = {
	savedItems: { key: "hawkr:cart:saved", in: ["local"] },
	checkoutDraft: { key: "hawkr:checkout:draft", in: ["local"] },
	checkoutDraftPii: { key: "hawkr:checkout:draft-pii", in: ["session"] },
	cartUi: { key: "hawkr:cart:ui", in: ["local"] },
	sellerCustomersUi: { key: "hawkr:seller-customers:ui", in: ["local"] },
	sellerProductsUi: { key: "hawkr:seller-products:ui", in: ["local"] },
	sellerDashboardUi: { key: "hawkr:seller-dashboard:ui", in: ["local"] },
	sellerInventoryUi: { key: "hawkr:seller-inventory:ui", in: ["local"] },
	productsImportUi: { key: "hawkr:products-import:ui", in: ["local"] },
	marketListSnapshot: { key: "hk-market-list-snapshot", in: ["session"] },
	feedListSnapshot: { key: "hk-feed-list-snapshot", in: ["session"] },
	// 這兩把是補掃時才發現的帳號私有 key，正是
	// tests/local-private-registry.test.ts 要防的那種「清單看起來完整」的漏。
	sellerShipmentOperations: {
		key: "hawkr:seller-shipment-operations",
		in: ["session"],
	},
	// 搜尋 handoff：訪客在首頁打的字要能帶進登入後的 Agent 工作區，所以「訪客 →
	// 登入」不能清；但登出或換帳號時必須清，否則同裝置的下一個人會拿到上一位的
	// 搜尋字串。
	hawkrAgentHandoff: {
		key: "hawkr:hawkr-agent:search-handoff:v1",
		in: ["session"],
		clearOn: "leaving-a-signed-in-owner",
	},
} as const satisfies Record<string, LocalPrivateKeySpec>;

export type LocalPrivateKeyName = keyof typeof LOCAL_PRIVATE_KEYS;

export const LOCAL_PRIVATE_KEY_SPECS: readonly LocalPrivateKeySpec[] =
	Object.values(LOCAL_PRIVATE_KEYS);
