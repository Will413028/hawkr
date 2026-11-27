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
		linkedProviders: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "linked-providers"] as const,
		profile: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "profile"] as const,
		addresses: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "addresses"] as const,
		myReports: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "my-reports"] as const,
		myReport: (id: QueryIdentity, reportId: string) =>
			["buyer", ...idPrefix(id), "my-reports", reportId] as const,
		coupons: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "coupons", params ?? {}] as const,
		availableCoupons: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "available-coupons", params ?? {}] as const,
		afterSales: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "after-sales", params ?? {}] as const,
		afterSale: (id: QueryIdentity, caseNo: string) =>
			["buyer", ...idPrefix(id), "after-sales", String(caseNo)] as const,
		favorites: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "favorites", params ?? {}] as const,
		following: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "following", params ?? {}] as const,
		blockedStores: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "blocked-stores"] as const,
		blockedBuyers: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "blocked-buyers"] as const,
		subscription: (id: QueryIdentity, sellerId: string | number) =>
			["buyer", ...idPrefix(id), "subscription", String(sellerId)] as const,
		history: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "history", params ?? {}] as const,
		memberCards: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["buyer", ...idPrefix(id), "member-cards", params ?? {}] as const,
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
		invoiceProfiles: (id: QueryIdentity) =>
			["buyer", ...idPrefix(id), "invoice-profiles"] as const,
		orderInvoice: (id: QueryIdentity, orderNo: string) =>
			["buyer", ...idPrefix(id), "order-invoice", String(orderNo)] as const,
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
		invoiceDocument: (id: QueryIdentity, documentId: string) =>
			[
				"buyer",
				...idPrefix(id),
				"invoice-document",
				String(documentId),
			] as const,
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
		lists: (id: QueryIdentity) =>
			["notifications", ...idPrefix(id), "list"] as const,
		list: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["notifications", ...idPrefix(id), "list", params ?? {}] as const,
		preferences: (id: QueryIdentity, role: string) =>
			["notifications", ...idPrefix(id), "preferences", role] as const,
	},
	assistant: {
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
		posts: (params?: Record<string, unknown>) =>
			["feed", "posts", params ?? {}] as const,
		postInteractions: (id: QueryIdentity, postId: string | number) =>
			[
				"feed",
				...idPrefix(id),
				"posts",
				String(postId),
				"interactions",
			] as const,
		postViewer: (id: QueryIdentity, postId: string | number) =>
			["feed", ...idPrefix(id), "posts", String(postId), "viewer"] as const,
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
		commentCreate: (id: QueryIdentity, postId: string | number) =>
			[
				"feed",
				...idPrefix(id),
				"posts",
				String(postId),
				"comment-create",
			] as const,
		replyCreate: (id: QueryIdentity, commentId: string | number) =>
			[
				"feed",
				...idPrefix(id),
				"comments",
				String(commentId),
				"reply-create",
			] as const,
	},
	seller: {
		self: (id: QueryIdentity) => ["seller", ...idPrefix(id), "self"] as const,
		kyc: (id: QueryIdentity) => ["seller", ...idPrefix(id), "kyc"] as const,
		onboarding: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "onboarding"] as const,
		paymentAccount: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "payment-account"] as const,
		contactEmail: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "contact-email"] as const,
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
		/** 一家店所有討論串頁的快取（`forumThread` 的共同前綴）。 */
		forumThreads: (id: QueryIdentity, sellerId: string | number) =>
			["seller", ...idPrefix(id), "forum-thread", String(sellerId)] as const,
		forumThread: (
			id: QueryIdentity,
			sellerId: string | number,
			threadId: string | number,
		) =>
			[
				"seller",
				...idPrefix(id),
				"forum-thread",
				String(sellerId),
				String(threadId),
			] as const,
		forumReplies: (
			id: QueryIdentity,
			sellerId: string | number,
			threadId: string | number,
			params?: Record<string, unknown>,
		) =>
			[
				"seller",
				...idPrefix(id),
				"forum-replies",
				String(sellerId),
				String(threadId),
				params ?? {},
			] as const,
		forumPostCreate: (id: QueryIdentity, sellerId: string | number) =>
			[
				"seller",
				...idPrefix(id),
				"forum-post-create",
				String(sellerId),
			] as const,
		forumReplyCreate: (id: QueryIdentity, sellerId: string | number) =>
			[
				"seller",
				...idPrefix(id),
				"forum-reply-create",
				String(sellerId),
			] as const,
		shipmentAnomalies: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "shipment-anomalies", params ?? {}] as const,
		shipmentAnomalyStats: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "shipment-anomaly-stats"] as const,
		shippingOnboardings: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "shipping-onboardings"] as const,
		products: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "products", params ?? {}] as const,
		customers: (id: QueryIdentity, params?: Record<string, unknown>) =>
			["seller", ...idPrefix(id), "customers", params ?? {}] as const,
		customer: (id: QueryIdentity, buyerId: string | number) =>
			["seller", ...idPrefix(id), "customers", String(buyerId)] as const,
		customerLookup: (id: QueryIdentity, buyerId: string | number) =>
			["seller", ...idPrefix(id), "customer-lookups", String(buyerId)] as const,
		tags: (id: QueryIdentity) => ["seller", ...idPrefix(id), "tags"] as const,
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
		blockedBuyers: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "blocked-buyers"] as const,
		measures: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "measures"] as const,
		measureList: (id: QueryIdentity, params: { state: number }) =>
			["seller", ...idPrefix(id), "measures", params] as const,
		measure: (id: QueryIdentity, measureId: string) =>
			["seller", ...idPrefix(id), "measures", measureId] as const,
		measureExists: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "measures", { exists: true }] as const,
		inquiries: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "inquiries"] as const,
		inquiryList: (id: QueryIdentity, params: { state: number }) =>
			["seller", ...idPrefix(id), "inquiries", params] as const,
		inquiry: (id: QueryIdentity, inquiryId: string) =>
			["seller", ...idPrefix(id), "inquiries", inquiryId] as const,
		inquiryPending: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "inquiries", { pending: true }] as const,
		invoiceOverview: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-overview"] as const,
		invoiceTaxProfile: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-tax-profile"] as const,
		invoiceReceiptAuth: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-receipt-auth"] as const,
		invoiceAttestation: (id: QueryIdentity) =>
			["seller", ...idPrefix(id), "invoice-attestation"] as const,
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
		orderExternalIssuances: (id: QueryIdentity, orderNo: string) =>
			[
				"seller",
				...idPrefix(id),
				"invoice-external-issuances",
				String(orderNo),
			] as const,
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
	},
	products: {
		list: (params?: Record<string, unknown>) =>
			["products", "list", params ?? {}] as const,
		detail: (id: string | number) =>
			["products", "detail", String(id)] as const,
		reviews: (id: string | number, params?: Record<string, unknown>) =>
			["products", "reviews", String(id), params ?? {}] as const,
	},
	// Global category tree (`/app/categories/tree`) — the same content for
	categories: {
		tree: () => ["categories", "tree"] as const,
		publicTree: () => ["categories", "tree", "public"] as const,
	},
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
		/** 公開商品列表（/sellers/{id}/products）——非 identity-scoped，與認證版
		 * queryKeys.seller.products（/app/products）是不同鍵空間。 */
		products: (id: string | number, params?: Record<string, unknown>) =>
			["sellers", "products", String(id), params ?? {}] as const,
		productsRoot: () => ["sellers", "products"] as const,
		/** 參照入口的店家卡片（BatchGetStores）；ids 排序去重後當鍵。 */
		cards: (ids: readonly string[]) => ["sellers", "cards", ...ids] as const,
	},
};
