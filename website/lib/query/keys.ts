// 型別匯入（執行期零依賴）。工作階段身分的定義權在解析 `GET current` 的那一支，
// 這裡只是用它當快取鍵；再宣告一份同形別名就是兩份會漂移的定義。
import type { DelegatedIdentity } from "@/features/delegated-operations/state/session";
import type { RoleSlot } from "@/lib/auth/active-role";

export type QueryIdentity = {
	accountId: string | null;
	activeRole: RoleSlot | null;
};

function idPrefix(id: QueryIdentity | null | undefined) {
	return [id?.accountId ?? "anon", id?.activeRole ?? "none"] as const;
}

export type { DelegatedIdentity };

/**
 * 代行工作區的快取前綴（56 號 §4.5 定案第 4 條）。
 *
 * 不能沿用 `QueryIdentity`：代行的瀏覽器在 www 上**沒有登入態**，`accountId` 與
 * `activeRole` 恆為 `null` ⇒ 先後代行 A 店與 B 店會共用同一個快取槽，第二家直接
 * 看到第一家的資料；管理員本人剛好也是賣家時，還會與他自己的商品列表撞在一起。
 * 兩種都是**靜默**的。
 *
 * `issuedAt` 讓「同一個管理員對同一家店的前後兩次工作階段」也彼此隔離。根前綴恆為
 * `"delegated"`：結束代行時清空是一次 `removeQueries({queryKey:["delegated"]})`，
 * 且絕不誤刪買家／賣家快取。
 */
function delegatedPrefix(id: DelegatedIdentity) {
	return [
		"delegated",
		id.adminId,
		id.storeId,
		id.issuedAt,
		id.revision,
	] as const;
}

export const queryKeys = {
	home: {
		snapshot: () => ["home", "snapshot"] as const,
	},
	navbar: {
		megaMenu: () => ["navbar", "mega-menu"] as const,
	},
	buyer: {
		profile: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "profile"] as const,
		addresses: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "addresses"] as const,
		coupons: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "coupons", params ?? {}] as const,
		availableCoupons: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "available-coupons", params ?? {}] as const,
		afterSales: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "after-sales", params ?? {}] as const,
		// One after-sale case detail (+ arbitration), keyed by caseNo. A string in
		// the slot keeps it disjoint from the `afterSales` list family above (which
		// uses an object slot); invalidate just this key after a buyer action.
		afterSale: (id: QueryIdentity, caseNo: string) =>
			["buyer", ...idPrefix(id), "after-sales", String(caseNo)] as const,
		// Resolve an order's refund landing page (PayUni ATM 302) → its after-sale
		// case + refund progress. Keyed by orderNo; one-shot read, no mutation.
		refundResolve: (id: QueryIdentity, orderNo: string) =>
			["buyer", ...idPrefix(id), "refund-resolve", String(orderNo)] as const,
		favorites: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "favorites", params ?? {}] as const,
		following: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "following", params ?? {}] as const,
		// One seller's subscribed boolean (`GET /subscriptions/{sellerId}`),
		// keyed by sellerId. A distinct segment keeps it disjoint from the
		// `following` list family above.
		subscription: (id: QueryIdentity, sellerId: string | number) =>
			["buyer", ...idPrefix(id), "subscription", String(sellerId)] as const,
		history: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "history", params ?? {}] as const,
		memberCards: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "member-cards", params ?? {}] as const,
		// Spending records for one member card (`/member-cards/{sellerId}/records`).
		memberCardRecords: (
			id: QueryIdentity,
			sellerId: string,
			params?: Record<string, unknown>,
		) =>
			[
				"buyer",
				...idPrefix(id),
				"member-cards",
				String(sellerId),
				"records",
				params ?? {},
			] as const,
		orders: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "orders", params ?? {}] as const,
		order: (
			id: QueryIdentity,
			orderNo: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"buyer",
				...idPrefix(id),
				"orders",
				String(orderNo),
				params ?? {},
			] as const,
		myReviews: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "my-reviews", params ?? {}] as const,
		// 常用發票資料（58 §6.1.1 端點 1）。整份清單一把鍵——四支 mutation 之後
		// 統一 invalidate 這一把，不做 per-profile 的細鍵。
		invoiceProfiles: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "invoice-profiles"] as const,
		// 一張訂單的發票總覽（端點 8）。訂單發票與訂單本身刷新節奏不同（發票以
		// 分鐘計、訂單以天計），所以是獨立的鍵而不是 `order` 家族的參數。
		orderInvoice: (id: QueryIdentity, orderNo: string) =>
			["buyer", ...idPrefix(id), "order-invoice", String(orderNo)] as const,
		// 一件發票申訴案件（端點 15，B07）。鍵上帶 orderNo 是因為端點路徑本身是
		// order-scoped；三支動作端點（63／64／65）成功後直接 setQueryData 覆蓋
		// 這一格，不再多打一次查詢。
		orderInvoiceComplaint: (
			id: QueryIdentity,
			orderNo: string,
			complaintId: string,
		) =>
			[
				"buyer",
				...idPrefix(id),
				"order-invoice-complaint",
				String(orderNo),
				String(complaintId),
			] as const,
		// 單一發票／收據文件詳情（端點 69）。鍵上**沒有 orderNo**：B05 的路由是
		// document-scoped（`/account/invoices/[documentId]`，發票通知也深連結到
		// 這裡），同一份文件從訂單詳情點進來與從通知點進來必須命中同一格快取。
		invoiceDocument: (id: QueryIdentity, documentId: string) =>
			[
				"buyer",
				...idPrefix(id),
				"invoice-document",
				String(documentId),
			] as const,
		// 買家發票中心的跨訂單列表（端點 68，B09）。篩選條件進鍵：換一組條件就是
		// 另一份結果，共用同一格會讓切換篩選時先閃出前一組的資料再被覆蓋。
		invoiceDocuments: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "invoice-documents", params ?? {}] as const,
	},
	checkout: {
		cart: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["checkout", ...idPrefix(id), "cart", params ?? {}] as const,
		shippingOptions: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["checkout", ...idPrefix(id), "shipping-options", params ?? {}] as const,
		storePickups: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["checkout", ...idPrefix(id), "store-pickups", params ?? {}] as const,
	},
	notifications: {
		// Identity-scoped prefixes let mutation / realtime paths invalidate every
		// cached list variant without touching preferences or another identity.
		lists: (id: QueryIdentity) =>
			["notifications", ...idPrefix(id), "list"] as const,
		list: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["notifications", ...idPrefix(id), "list", params ?? {}] as const,
		// Per-role notification toggles (buyer/seller workspace share the read).
		preferences: (id: QueryIdentity, role: string) =>
			["notifications", ...idPrefix(id), "preferences", role] as const,
	},
	assistant: {
		// Hawkr Agent 對話列表（B-244）。identity-scoped：角色／帳號切換不得
		// 沿用他人的對話列表快取。
		conversations: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["assistant", ...idPrefix(id), "conversations", params ?? {}] as const,
		/** 單一對話的完整紀錄。快取住才有「側欄點回上一個對話瞬間就有內容」
		 * ——切換會讓右側 workspace remount，掛載時能同步讀到這份就不閃骨架。 */
		transcript: (id: QueryIdentity, conversationId: string) =>
			["assistant", ...idPrefix(id), "transcript", conversationId] as const,
		/** 需求快照（slots／stage 的權威來源），依 requestId。 */
		snapshot: (id: QueryIdentity, requestId: string) =>
			["assistant", ...idPrefix(id), "snapshot", requestId] as const,
	},
	chat: {
		conversations: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["chat", ...idPrefix(id), "conversations", params ?? {}] as const,
		thread: (
			id: QueryIdentity,
			targetId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"chat",
				...idPrefix(id),
				"thread",
				String(targetId),
				params ?? {},
			] as const,
	},
	feed: {
		recommendedSellers: () => ["feed", "recommended-sellers"] as const,
		// posts is intentionally NOT identity-scoped: it's an SSR-seeded paged
		// network list (initialData rendered server-side with the viewer's cookie).
		// Scoping would force a refetch right after hydration and strand the correct
		// SSR seed under the `anon` slot. Cross-identity bleed on a live tab is
		// instead handled by teardownPrivateCache() on identity change.
		posts: (params?: Record<string, unknown>) =>
			["feed", "posts", params ?? {}] as const,
		// The on-demand interaction/comment keys below ARE identity-scoped: they
		// hold per-viewer like/save state and carry no network/SSR cost to scope
		// (postInteractions resolves locally; comments fetch on modal open).
		postInteractions: (id: QueryIdentity, postId: string | number) =>
			[
				"feed",
				...idPrefix(id),
				"posts",
				String(postId),
				"interactions",
			] as const,
		comments: (
			id: QueryIdentity,
			postId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"feed",
				...idPrefix(id),
				"posts",
				String(postId),
				"comments",
				params ?? {},
			] as const,
		commentReplies: (
			id: QueryIdentity,
			commentId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"feed",
				...idPrefix(id),
				"comments",
				String(commentId),
				"replies",
				params ?? {},
			] as const,
	},
	seller: {
		self: (id: QueryIdentity) => ["seller", ...idPrefix(id), "self"] as const,
		// `GetMyKYC` 的申請（沒有申請是 null）；流程狀態由它推導（`useMyKycFlow`）。
		kyc: (id: QueryIdentity) => ["seller", ...idPrefix(id), "kyc"] as const,
		dashboard: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "dashboard"] as const,
		orders: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "orders", params ?? {}] as const,
		order: (
			id: QueryIdentity,
			orderNo: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"seller",
				...idPrefix(id),
				"orders",
				String(orderNo),
				params ?? {},
			] as const,
		orderCounts: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "order-counts", params ?? {}] as const,
		afterSales: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "after-sales", params ?? {}] as const,
		// One after-sale case detail (+ arbitration), keyed by caseNo — opened from
		// the seller decision drawer. String slot keeps it disjoint from the
		// `afterSales` list family (object slot); invalidate after confirm-receipt.
		afterSale: (id: QueryIdentity, caseNo: string) =>
			["seller", ...idPrefix(id), "after-sales", String(caseNo)] as const,
		forum: (
			id: QueryIdentity,
			sellerId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"seller",
				...idPrefix(id),
				"forum",
				String(sellerId),
				params ?? {},
			] as const,
		shipmentAnomalies: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "shipment-anomalies", params ?? {}] as const,
		shipmentAnomalyStats: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "shipment-anomaly-stats"] as const,
		shippingOnboardings: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "shipping-onboardings"] as const,
		// Own-products list (`/app/products`). Each consumer passes its own
		// page/pageSize/status, so the `params` slot keeps the variants in
		// separate cache entries; invalidating with no params (`{}`) partial-
		// matches and refetches the whole family after a mutation.
		products: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "products", params ?? {}] as const,
		// CRM customer list (`/app/sellers/me/customers`). Params-driven
		// (page/keyword/customerType/tagId/sort), so — like `products` — each
		// variant keeps its own cache entry; invalidating with no params (`{}`)
		// partial-matches and refetches the whole family after a mutation.
		customers: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "customers", params ?? {}] as const,
		// One CRM customer's detail + purchase history, keyed by buyerId. Shared
		// by the full-page route and the customer-list drawer; a string in the
		// params slot keeps it disjoint from the `customers` list family above.
		customer: (id: QueryIdentity, buyerId: string | number) =>
			["seller", ...idPrefix(id), "customers", String(buyerId)] as const,
		// Lightweight current-profile lookup used when an order snapshot omitted
		// the buyer nickname. Kept separate from `customer`, whose cache value also
		// contains the customer's order history.
		customerLookup: (id: QueryIdentity, buyerId: string | number) =>
			["seller", ...idPrefix(id), "customer-lookups", String(buyerId)] as const,
		// The seller's tag pool (`/app/sellers/me/tags`) — filter chips + tag-
		// editor suggestions share one entry (both mount with the drawer open).
		tags: (id: QueryIdentity) => ["seller", ...idPrefix(id), "tags"] as const,
		// The seller's own coupons / posts lists — single consumer each, optimistic
		// delete updates the cache. Params slot keeps page/pageSize variants apart.
		coupons: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "coupons", params ?? {}] as const,
		posts: (
			id: QueryIdentity,
			sellerId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"seller",
				...idPrefix(id),
				"posts",
				String(sellerId),
				params ?? {},
			] as const,
		// My measures (Connect `SellerAppService.ListMyMeasures`／`GetMyMeasure`, 見 lib/seller/measures.ts)：措施與申覆頁、
		// 停權公告。列表用 object slot（`state` 篩選）、單筆用 string slot，讓
		// `invalidateQueries({queryKey: measures(id)})` 一次命中全部列表與單筆（申覆成功後都要重讀）。
		measures: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "measures"] as const,
		measureList: (id: QueryIdentity, params: { state: number }) =>
			["seller", ...idPrefix(id), "measures", params] as const,
		measure: (id: QueryIdentity, measureId: string) =>
			["seller", ...idPrefix(id), "measures", measureId] as const,
		// 開店頁問「有沒有措施」的一筆查詢（`ListMyMeasures` pageSize 1）；object slot 與單筆的 string slot 不會撞。
		measureExists: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "measures", { exists: true }] as const,
		// ── 58 號電子發票賣家面（58A §B-2.2 (4)(b)）─────────────────────────
		// 全部帶 `idPrefix(id)`：漏掉就是跨帳號快取污染，而且 `invalidateQueries`
		// 找不到 family ⇒ mutation 之後畫面不更新（§B-2.0 T18）。
		// 列表用 object slot、單筆用 string slot，讓
		// `invalidateQueries({queryKey: [...].slice(0, -1)})` 一次命中 list ＋ detail。
		invoiceOverview: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-overview"] as const,
		invoiceTaxProfile: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-tax-profile"] as const,
		// 端點 28（S05）。與 S04 分開：兩者是不同的資格軸（自行開票 vs 免用發票），
		// 同一個賣家只會走其中一條。
		invoiceReceiptAuth: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-receipt-auth"] as const,
		// 端點 21（S04）。與 `invoice-tax-profile` 分開：S04 送出後兩者都要失效，
		// 但 S02 的送審只動 tax profile ⇒ 合併會讓每次送審都連帶重打端點 21。
		invoiceAttestation: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-attestation"] as const,
		// 端點 23（S03）。自成 family：AMEGO 帳號的 mutation 會改 `version`，
		// 但不會動稅務身分——掛在 `invoice-tax-profile` 底下會讓每次測試連線
		// 都連帶重打端點 17。
		invoiceProviderAccount: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-provider-account"] as const,
		invoiceDocuments: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "invoice-documents", params ?? {}] as const,
		invoiceDocument: (id: QueryIdentity, documentId: string) =>
			[
				"seller",
				...idPrefix(id),
				"invoice-documents",
				String(documentId),
			] as const,
		// 🔴 端點 62R 自成一個 family，**不掛進 `invoice-documents` 也不掛進
		// `invoice-jobs`**：它的 capability 是 `CapInvoiceTaxRemedy`（停權放行），
		// 而那兩族是 `CapInvoiceManage`（停權 BLOCK）。掛在一起的話，一次
		// invalidate 就會在停權狀態下連帶重打一支必回 9240 的查詢（§B-2.0 T18）。
		orderExternalIssuances: (id: QueryIdentity, orderNo: string) =>
			[
				"seller",
				...idPrefix(id),
				"invoice-external-issuances",
				String(orderNo),
			] as const,
		// 端點 72（S12 進項分頁）自成 family：它是 recipient-scoped，與
		// `invoice-documents`（issuer-scoped）是兩個資料集。掛在一起的話，銷項的
		// mutation 一次 invalidate 會連帶重打進項——兩者沒有任何因果關係。
		invoiceReceivedDocuments: (
			id: QueryIdentity,
			params?: Record<string, unknown>,
		) =>
			[
				"seller",
				...idPrefix(id),
				"invoice-received-documents",
				params ?? {},
			] as const,
		invoiceJob: (id: QueryIdentity, jobId: string) =>
			["seller", ...idPrefix(id), "invoice-jobs", String(jobId)] as const,
		invoiceComplaints: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "invoice-complaints", params ?? {}] as const,
		invoiceComplaint: (id: QueryIdentity, complaintId: string) =>
			[
				"seller",
				...idPrefix(id),
				"invoice-complaints",
				String(complaintId),
			] as const,
		// 尚未新增：`invoiceAttestation` / `invoiceProviderAccount` /
		// `invoiceReceiptAuth`。58A §B-2.1 #6 列了十一支，這裡只放真正有 consumer
		// 的那些——沒有讀取端的 key 工廠是死碼，會讓下一個人以為對應畫面已經接上了。
		// S03–S05 落地時各自帶上自己那一支。
	},
	products: {
		list: (params?: Record<string, unknown>) =>
			["products", "list", params ?? {}] as const,
		detail: (id: string | number) =>
			["products", "detail", String(id)] as const,
		// Public per-product buyer reviews (`GET /public/products/{id}/reviews`).
		// Not identity-scoped — the seller 商品管理 reviews drawer reads the same
		// list a buyer would see; invalidate after a seller reply to show it.
		reviews: (id: string | number, params?: Record<string, unknown>) =>
			["products", "reviews", String(id), params ?? {}] as const,
	},
	// Global category tree (`/app/categories/tree`) — the same content for
	// every identity, so not identity-scoped; long staleTime set in the hook.
	categories: {
		tree: () => ["categories", "tree"] as const,
		publicTree: () => ["categories", "tree", "public"] as const,
	},
	// Public, non-identity-scoped content reads.
	postTags: {
		trending: (params?: Record<string, unknown>) =>
			["post-tags", "trending", params ?? {}] as const,
	},
	productHashtags: {
		popular: (params?: Record<string, unknown>) =>
			["product-hashtags", "popular", params ?? {}] as const,
	},
	banners: {
		list: () => ["banners", "list"] as const,
	},
	/**
	 * 代行工作區（`/adm`）。與 `seller.*` 是**兩個鍵空間**：同一支端點在兩條路徑下
	 * 回的是不同店家的資料，共用鍵就是跨店外洩。
	 *
	 * `["delegated","session","current"]`（工作階段本身）刻意不在這裡：它是 identity
	 * 的來源、不吃 identity 前綴，定義在
	 * `features/delegated-operations/state/session-client.ts` 的
	 * `DELEGATED_SESSION_QUERY_KEY`，本檔不另立第二份。
	 */
	delegated: {
		storefront: (id: DelegatedIdentity) =>
			[...delegatedPrefix(id), "storefront"] as const,
		products: (id: DelegatedIdentity, params?: Record<string, unknown>) =>
			[...delegatedPrefix(id), "products", params ?? {}] as const,
		product: (id: DelegatedIdentity, productId: string | number) =>
			[...delegatedPrefix(id), "products", String(productId)] as const,
		posts: (id: DelegatedIdentity, params?: Record<string, unknown>) =>
			[...delegatedPrefix(id), "posts", params ?? {}] as const,
		post: (id: DelegatedIdentity, postId: string | number) =>
			[...delegatedPrefix(id), "posts", String(postId)] as const,
		orders: (id: DelegatedIdentity, params?: Record<string, unknown>) =>
			[...delegatedPrefix(id), "orders", params ?? {}] as const,
		order: (
			id: DelegatedIdentity,
			orderNo: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				...delegatedPrefix(id),
				"orders",
				String(orderNo),
				params ?? {},
			] as const,
		auditLogs: (id: DelegatedIdentity, params?: Record<string, unknown>) =>
			[...delegatedPrefix(id), "audit-logs", params ?? {}] as const,
	},
	sellers: {
		detail: (id: string | number) => ["sellers", "detail", String(id)] as const,
		/** 公開商品列表（/sellers/{id}/products）——非 identity-scoped，與認證版
		 * queryKeys.seller.products（/app/products）是不同鍵空間。 */
		products: (id: string | number, params?: Record<string, unknown>) =>
			["sellers", "products", String(id), params ?? {}] as const,
		productsRoot: () => ["sellers", "products"] as const,
	},
};
