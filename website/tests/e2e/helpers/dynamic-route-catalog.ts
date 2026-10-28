export type DynamicRouteAccess =
	| "public"
	| "buyer"
	| "seller"
	| "account"
	| "delegated";

export type DynamicRouteFixture =
	| "protected-placeholder"
	| "delegated-workspace"
	| "order-callback"
	| "product"
	| "seller"
	| "seller-product";

export interface DynamicRouteCatalogEntry {
	pattern: string;
	access: DynamicRouteAccess;
	fixture: DynamicRouteFixture;
}

export interface PublicDynamicFixtureIds {
	productId: string;
	/**
	 * CANONICAL storefront path segment (the seller's `store_code`). Routes are
	 * materialized with this so a document assertion lands on the URL it asked
	 * for instead of following the `308` in front of a `store_id` URL.
	 */
	sellerId: string;
	/**
	 * The key the public product feed actually exposes (`store_id`), kept so the
	 * consolidation itself can be asserted. Equal to `sellerId` when the seller
	 * has no distinct store_code to be sent to.
	 */
	sellerStoreId?: string;
}

/**
 * Every independently addressable dynamic App Router page.
 *
 * Keep this list explicit: the set-equality contract in site-links.spec.ts
 * intentionally fails whenever a dynamic page is added or removed without a
 * deliberate access/runtime test decision here.
 *
 * The intercepted product modal under `app/[locale]/(public)/@modal` is not an
 * independently addressable document, so it belongs in an interaction test
 * rather than this route catalog.
 */
export const DYNAMIC_ROUTE_CATALOG: readonly DynamicRouteCatalogEntry[] = [
	// ADM 代行工作區（56 號）。access 是 `delegated` 而非 `seller`：這個子樹不看
	// JWT 角色，`requiredAccessForPath` 對它一律回 null，守門由工作階段本身承擔
	// （client 端的 `GET /adm/api/session/current`，見 app/(delegated)/adm/layout.tsx）。
	// 標成 buyer/seller 會讓 site-links 的「訪客導 /login 且帶 role 參數」斷言跑錯
	// 路徑——它們根本不發 307。專屬斷言見 site-links.spec.ts 的 delegated 區塊。
	//
	// `/adm/[storeId]` 不在本清冊：它已無 page.tsx，導向由 next.config.ts 的
	// redirects() 在 HTTP 層完成（307 → /products）。清冊與 App Router 掃描結果
	// 是 set-equality（site-links.spec.ts「dynamic route catalog exactly matches」），
	// 留著會直接轉紅；它的契約改由同檔的 307 斷言承接。
	{
		pattern: "/adm/[storeId]/[...area]",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/dashboard",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/orders",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/orders/[orderNo]",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/posts",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/posts/new",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/[productId]/edit",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/[productId]/preview",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/products/new",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/adm/[storeId]/storefront",
		access: "delegated",
		fixture: "delegated-workspace",
	},
	{
		pattern: "/account/addresses/[id]/edit",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/after-sales/[caseNo]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/invoices/[documentId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/messages/[targetId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/messages/assistant/[conversationId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/return",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/review",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/orders/[orderNo]/tracking",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/account/wallet/[sellerId]",
		access: "buyer",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/orders/[orderNo]",
		access: "public",
		fixture: "order-callback",
	},
	{
		pattern: "/orders/[orderNo]/refund",
		access: "public",
		fixture: "order-callback",
	},
	{
		pattern: "/products/[id]",
		access: "public",
		fixture: "product",
	},
	{
		pattern: "/seller-center/coupons/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/customers/[buyerId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	// 帳號層級的頁面：登入即可、不分工作區（`requiredAccessForPath` 回 `account`）。
	// 訪客導 /login 但不帶 role。
	{
		pattern: "/seller/measures/[measureId]",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/measures/[measureId]/appeal",
		access: "account",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller/measures/[measureId]/remediation",
		access: "account",
		fixture: "protected-placeholder",
	},
	// 58 號電子發票賣家面的三條動態路由（58A S08／S10／S11）。
	//
	// ⚠️ `/seller-center/invoices/[documentId]` 與同層的四個靜態段
	// （`documents`／`tax-status`／`appeals`／`jobs`）並存：App Router 的靜態段優先，
	// 所以那四條仍由各自的 page.tsx 承接、只會出現在 PROTECTED_STATIC_ROUTES。
	{
		pattern: "/seller-center/invoices/[documentId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/invoices/appeals/[complaintId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		// 目錄名一律 `[jobId]`：`dynamic-route-catalog` 的斷言與 App Router 目錄名
		// 逐字比對，改名就是 CI 紅（§B-2.0 T19 的同一條規則）。
		pattern: "/seller-center/invoices/jobs/[jobId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/messages/[targetId]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/orders/[orderNo]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/posts/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/preview/[id]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/preview/[id]/messages",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/preview/[id]/posts",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/preview/[id]/reviews",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/products/[id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/products/import/[job_id]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-center/products/import/[job_id]/items/[item_id]/edit",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-product-import-preview/[job_id]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/seller-product-import-preview/[job_id]/items/[item_id]",
		access: "seller",
		fixture: "protected-placeholder",
	},
	{
		pattern: "/sellers/[id]",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/card",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/messages",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/posts",
		access: "public",
		fixture: "seller",
	},
	{
		pattern: "/sellers/[id]/products/[pid]",
		access: "public",
		fixture: "seller-product",
	},
	{
		pattern: "/sellers/[id]/reviews",
		access: "public",
		fixture: "seller",
	},
];

function requireFixtureId(
	fixtures: PublicDynamicFixtureIds | undefined,
	key: keyof PublicDynamicFixtureIds,
	pattern: string,
): string {
	const value = fixtures?.[key];
	if (!value) {
		throw new Error(`${pattern} requires a non-empty ${key} fixture`);
	}
	return encodeURIComponent(value);
}

export function materializeDynamicRoute(
	entry: DynamicRouteCatalogEntry,
	fixtures?: PublicDynamicFixtureIds,
): string {
	let route = entry.pattern;

	switch (entry.fixture) {
		case "protected-placeholder":
			route = route.replace(/\[[^\]]+\]/g, "route-contract-id");
			break;
		// 代行工作區的 `[storeId]` **必須是數字**，其餘 param 才用字串佔位。
		//
		// ⚠️ 這不是美化：`app/(delegated)/adm/[storeId]/layout.tsx` 對非數字
		// storeId 一律 notFound()，而在它存在之前，client 的
		// `DelegatedRuntimeBoundary`（:28 同一條 `/^\d{1,19}$/`）會走
		// `return <>{children}</>` 不掛 Provider ⇒ 各頁的嚴格
		// `useDelegatedSession()` throw ⇒ **SSR 500**。
		//
		// 用 `protected-placeholder`（把 `[storeId]` 也換成 `route-contract-id`）的
		// 後果是：本檔案的消費者「delegated workspace routes stay guest-safe and
		// unindexed」**每一條都在 500**，於是它另外兩條斷言（缺 noindex、骨架夾帶
		// 賣家資料）**從來沒有機會執行過** —— 那支測試存在的理由正是那兩條。
		// 換成數字佔位，它才第一次真的走到 DelegatedWorkspaceFrame。
		case "delegated-workspace":
			route = route
				.replace("[storeId]", "9000000000000000000")
				.replace(/\[[^\]]+\]/g, "route-contract-id");
			break;
		case "order-callback":
			route = route.replace("[orderNo]", "route-contract-order");
			break;
		case "product":
			route = route.replace(
				"[id]",
				requireFixtureId(fixtures, "productId", entry.pattern),
			);
			break;
		case "seller":
			route = route.replace(
				"[id]",
				requireFixtureId(fixtures, "sellerId", entry.pattern),
			);
			break;
		case "seller-product":
			route = route
				.replace("[id]", requireFixtureId(fixtures, "sellerId", entry.pattern))
				.replace(
					"[pid]",
					requireFixtureId(fixtures, "productId", entry.pattern),
				);
			break;
	}

	if (route.includes("[")) {
		throw new Error(`unresolved dynamic route parameters: ${route}`);
	}
	return route;
}
