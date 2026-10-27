import {
	type DelegatedMutationMeta,
	delegatedFetch,
	type RepoFetcher,
} from "@/lib/api/delegated";
import type { MyPostDetail, PostListItem } from "@/lib/api/openapi-types";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import { createMediaUploadRepo } from "@/lib/media/upload";
import { createOrderRepo } from "@/lib/orders/repo";
import {
	type ProductListResult,
	type ProductWriteResult,
	type SellerProductListItem,
	type SellerProductRepo,
	type SkuWriteInput,
	toDescriptionWireInput,
} from "@/lib/products/seller-repo";
import type { SellerPostRepo, SellerStorefrontRepo } from "@/lib/seller/repo";
import type { SellerDataRepos } from "./scope";

/**
 * 代行工作區的資料層（56 號 §6.1.7／§6.1.8／§6.1.9）。
 *
 * 這裡**不是**「賣家工廠換一個 root」：三個面的 wire 形狀與賣家端不同，而
 * `lib/{seller,products}` 的 normalizer 讀不到的欄位一律變成 `undefined` 而不報錯
 * （商店設定頁靜默少欄、商品列表整欄空白）。所以每一支都帶自己的 mapper。
 *
 * 三個共通欄（§6.1.0）由 `DelegatedMutationMeta` 帶進來，**一律必填**：
 * `operationReason` 缺席就是 9260／400，那是一個「表單完全不能用」的失敗，讓它在
 * 送出之前就以呼叫端錯誤現形，比讓操作者打完字才收到一個沒有 message 的 400 好。
 */

/** BFF 的資源根（`delegatedFetch` 會補 `/adm/api` 前綴）。 */
const STOREFRONT_ROOT = "/storefront";
const PRODUCTS_ROOT = "/products";
const POSTS_ROOT = "/posts";
const MEDIA_ROOT = "/media/upload_sessions";
const ORDERS_ROOT = "/orders";

/** 代行沒有對應端點的資料層方法（§6.1.1 端點總表零命中）。 */
export class DelegatedEndpointMissingError extends Error {
	constructor(operation: string) {
		super(
			`代行工作階段沒有「${operation}」這個能力，請回到 ADM 以治理流程處理。`,
		);
		this.name = "DelegatedEndpointMissingError";
	}
}

/** 本期尚未接線的代行資料層方法（端點存在、前端映射未完成）。 */
export class DelegatedRepoNotWiredError extends Error {
	constructor(operation: string) {
		super(`代行工作區的「${operation}」本期尚未接線。`);
		this.name = "DelegatedRepoNotWiredError";
	}
}

/**
 * `Idempotency-Key`（§6.1.0）：8–128 字、`^[A-Za-z0-9._:-]+$`。
 *
 * ⚠️ 每次呼叫**產生新的一把**。重用同一把打不同 payload 是 9267；而重試同一次送出
 * 要沿用同一把，那是呼叫端在 catch 之後的事，不是這裡的預設。
 */
function newIdempotencyKey(): string {
	const random =
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID().replace(/-/g, "")
			: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`;
	return `adm-${random}`;
}

function requireMeta(
	meta: DelegatedMutationMeta | undefined,
	operation: string,
): DelegatedMutationMeta {
	if (!meta || meta.operationReason.trim() === "") {
		throw new Error(
			`代行寫入「${operation}」缺少修改原因（operationReason）：這一欄是必填的，缺了後端一律回 9260。`,
		);
	}
	return meta;
}

/** 共通三欄 ＋ 該端點自己的欄位。`notifySeller` 不靠 DB DEFAULT 兜底，缺席時顯式填 true。 */
function mutationBody(
	meta: DelegatedMutationMeta,
	extra: Record<string, unknown> = {},
): Record<string, unknown> {
	return {
		...(meta.expectedVersion === undefined
			? {}
			: { expectedVersion: meta.expectedVersion }),
		operationReason: meta.operationReason.trim(),
		notifySeller: meta.notifySeller ?? true,
		...extra,
	};
}

function str(value: unknown): string | undefined {
	return typeof value === "string" && value !== "" ? value : undefined;
}

function numOrUndefined(value: unknown): number | undefined {
	if (value == null || value === "") return undefined;
	const n = Number(value);
	return Number.isFinite(n) ? n : undefined;
}

/** RFC3339 → unix 秒（`PostListItem.createdAt` 是 number）。 */
function epochSeconds(value: unknown): number {
	const parsed = Date.parse(String(value ?? ""));
	return Number.isFinite(parsed) ? Math.floor(parsed / 1000) : 0;
}

type Wire = Record<string, unknown>;

function asRecord(value: unknown): Wire {
	return value && typeof value === "object" && !Array.isArray(value)
		? (value as Wire)
		: {};
}

/**
 * 代行寫入的統一回應形狀是 `{auditLogId, version, resource}` —— gateway 的
 * `respondDelegatedMutation`（`admin_delegated_common.go:214`）是**每一支**代行寫入
 * 端點的唯一出口，寫入後的資源一律在 `resource` 之下。
 *
 * ⚠️ 直接讀頂層（`data.postId`）的失敗形狀是**靜默的**：拿到 `undefined`、沒有例外、
 * HTTP 200、稽核列也照樣寫成 SUCCEEDED，只有接續的第二支呼叫被跳過。實測坐實：
 * 「儲存並發布」因此只建了未發布的貼文，從未打過 `/publish`。
 *
 * ⚠️ 批次端點**不走**這個 envelope（`response.OK` 直接吐 `{requested,…,results}`），
 * 它的解析不要套這一支。
 */
function mutationResource(data: unknown): Wire {
	return asRecord(asRecord(data).resource);
}

/**
 * 代行店面（`/admin/sellers/:store_id/storefront*`）：店面資料已在單體的 seller 模組，四條路由從步驟 9
 * 起固定回 503，到 A10 以 Connect 的代行契約恢復（計畫 `2026-09-29-a3-buyer-seller.md` 步驟 8）。這一支不轉換
 * 任何資料：請求照樣送出，授權鏈與稽核的結果、503 都原樣交給呼叫端；萬一回應成功也不讀它，丟
 * `DelegatedRepoNotWiredError`，不憑一個還沒有契約的形狀猜欄位。
 */
export function createDelegatedStorefrontRepo(
	fetcher: RepoFetcher,
	storeId: string,
): SellerStorefrontRepo {
	const root = `${STOREFRONT_ROOT}/${encodeURIComponent(storeId)}`;

	async function unavailable(
		operation: string,
		path: string,
		init?: Parameters<RepoFetcher>[1],
	): Promise<never> {
		await fetcher<unknown>(path, init);
		throw new DelegatedRepoNotWiredError(operation);
	}

	return {
		fetchMyStore: async () => unavailable("讀取店面", root),
		updateMyStore: async (_update, meta) =>
			unavailable("儲存商店資訊", root, {
				method: "PATCH",
				json: mutationBody(requireMeta(meta, "儲存商店資訊")),
				idempotencyKey: newIdempotencyKey(),
			}),
		updateStoreLogo: async (_etag, stagedToken, meta) =>
			unavailable("更換商店頭像", `${root}/logo`, {
				method: "POST",
				json: mutationBody(requireMeta(meta, "更換商店頭像"), { stagedToken }),
				idempotencyKey: newIdempotencyKey(),
			}),
		updateStoreCover: async (_etag, stagedToken, meta) =>
			unavailable("更換商店封面", `${root}/cover`, {
				method: "POST",
				json: mutationBody(requireMeta(meta, "更換商店封面"), { stagedToken }),
				idempotencyKey: newIdempotencyKey(),
			}),
	};
}

/** 代行商品列表 DTO → 賣家中心列表元件吃的形狀。 */
export function mapDelegatedProductItem(raw: unknown): SellerProductListItem {
	const r = asRecord(raw);
	const imageKey = str(r.imageKey);
	return {
		id: str(r.productId) ?? "",
		version: str(r.version),
		name: str(r.name) ?? "",
		main_image: imageKey,
		image_url: imageKey,
		price_min: str(r.priceMin),
		price_max: str(r.priceMax),
		category_name: str(r.categoryName),
		stock: numOrUndefined(r.stockQty),
		sales_count: numOrUndefined(r.salesCount),
		status: str(r.status),
		// D5：狀態文字一律是 CodeMeta 的 Label（4004＝`啟用`，不是意譯的「上架中」），
		// 而後端已經在 DTO 裡送了它。
		status_label: str(r.statusLabel),
	};
}

/**
 * 賣家端的 SKU 輸入 → 代行 SKU 輸入。
 *
 * ⚠️ **`skuName` ↔ `spec`**：整條鏈上最容易接錯的一欄（Gateway DTO 註解明列）。接錯
 * 的失敗形狀是 9260「商品規格缺少必填欄位」——訊息指向規格，但真正缺的是欄名。
 *
 * ⚠️ 建立時**不送 `skuId`**：新規格沒有 id，送一個空字串等於宣稱「更新某個規格」。
 * 重量與長寬高一律送（就算是空字串）：product-service 對它們有「必須大於 0」的
 * 驗證，少送會在建立那一刻才以 400 現形，而前端這裡看不出是哪一欄。
 */
function toDelegatedSkuInput(sku: SkuWriteInput): Record<string, unknown> {
	return {
		spec: sku.skuName,
		price: sku.price,
		salePrice: sku.salePrice ?? "",
		stockQty: sku.stockQty ?? "0",
		unit: sku.unit,
		weight: sku.weight ?? "",
		lengthCm: sku.lengthCm ?? "",
		widthCm: sku.widthCm ?? "",
		heightCm: sku.heightCm ?? "",
		...(sku.shippingMethods === undefined
			? {}
			: { shippingMethods: sku.shippingMethods }),
	};
}

/** 逐項結果（L3168）：整批被受理時頂層 `code` 恆為 0，成敗看 `failed`。 */
export interface DelegatedBatchResultItem {
	productId: string;
	ok: boolean;
	code: number;
	version: string;
	auditLogId: string;
}

export interface DelegatedBatchResult {
	requested: number;
	succeeded: number;
	failed: number;
	results: DelegatedBatchResultItem[];
}

export interface DelegatedBatchUnpublishItem {
	productId: string;
	expectedVersion: string;
}

/** 批次下架 `items` 上限（L3144）：必須 ≥ 列表 `page_size` 上限，否則「全選本頁」勾完才失敗。 */
export const DELEGATED_BATCH_UNPUBLISH_MAX = 50;

export interface DelegatedProductCommands {
	batchUnpublish(
		items: readonly DelegatedBatchUnpublishItem[],
		meta: DelegatedMutationMeta,
	): Promise<DelegatedBatchResult>;
}

export interface DelegatedPostCommands {
	publish(
		postId: string | number,
		meta: DelegatedMutationMeta,
		schedule?: { publishMode: string; scheduledPublishAt?: string },
	): Promise<void>;
	unpublish(
		postId: string | number,
		meta: DelegatedMutationMeta,
	): Promise<void>;
	/**
	 * 貼文總數（總覽 KPI）：`page_size=1` 的列表，只讀它的 `total`。
	 *
	 * `fetchSellerPosts` 回的是 `PostListItem[]`，`total` 在映射時就被丟掉了 ——
	 * 拿陣列長度當總數會在超過 `page_size` 的店家靜默少報，而畫面上沒有任何東西
	 * 會告訴你那個數字是截斷的。
	 */
	count(): Promise<number>;
}

export function createDelegatedProductRepo(
	fetcher: RepoFetcher,
	storeId: string,
): SellerProductRepo & DelegatedProductCommands {
	const root = `${PRODUCTS_ROOT}/${encodeURIComponent(storeId)}`;
	const at = (id: number | string) =>
		`${root}/${encodeURIComponent(String(id))}`;

	return {
		async fetchSellerProducts({
			page = 1,
			pageSize = 20,
			status,
		} = {}): Promise<ProductListResult> {
			const query = new URLSearchParams({
				page: String(page),
				page_size: String(pageSize),
			});
			if (status != null) query.set("status", String(status));
			const env = await fetcher<unknown>(`${root}?${query}`);
			const data = asRecord(env.data);
			const records = Array.isArray(data.records) ? data.records : [];
			return {
				records: records.map(mapDelegatedProductItem),
				// 數字對外一律 string（`"total": "12"`），Number() 之後才是這裡的契約。
				total: Number(data.total ?? records.length),
				page: Number(data.page ?? page),
				pageSize: Number(data.pageSize ?? pageSize),
			};
		},

		/**
		 * ⚠️ 讀取／PATCH 既有商品**不走這裡**，走
		 * `features/delegated-operations/products/repo.ts`：代行的 detail 與 PATCH body
		 * 和賣家端的 draft 形狀不同（`images` 是整份替換且要以 `{assetId}` 回送原圖、
		 * `skus` 的刪除只認 `removedSkuIds`），半套映射的失敗形狀是**請求完全合法、
		 * 回 `code: 0`，而賣家的商品圖片或規格被清空**（L3774／L3079）。這兩支留著
		 * throw，是為了讓「有人不小心從賣家端的抽屜路徑進來」當場現形而不是靜默半套。
		 */
		fetchSellerProductEdit() {
			return Promise.reject(new DelegatedRepoNotWiredError("商品編輯"));
		},
		fetchProductSkus() {
			return Promise.reject(new DelegatedRepoNotWiredError("商品規格"));
		},

		/**
		 * 建立商品（`POST .../products`，capability `products.write`）。
		 *
		 * 建立**一律落 4003 未上架**：body 沒有 `status` 欄位，上架只能另外打 `/publish`
		 * （兩支端點、兩筆稽核列）。回應的頂層 `version` 是接著上架時
		 * `expectedVersion` 的唯一來源。
		 *
		 * ⚠️ 三個「送出去就是 9260」的陷阱，每一個都不會在本地轉紅：
		 * - **不得帶 `expectedVersion`**：新資源沒有版本，而 create 的 DTO 根本沒有
		 *   這個欄位 ＋ `DisallowUnknownFields`（`admin_delegated_common.go:121`）
		 * - **不得帶 `idempotencyKey`／`status`／`updateHashtags`**：前者是 header
		 *   不是 body 欄位，後兩者是賣家端 `ProductWriteBody` 專有
		 * - **描述要剝掉 `imageUrl`**（`toDescriptionWireInput`）
		 */
		async createProduct(
			body,
			meta,
		): Promise<{ id?: number | string; version?: string }> {
			const checked = requireMeta(meta, "建立商品");
			const env = await fetcher<unknown>(root, {
				method: "POST",
				json: mutationBody(
					// 只取兩欄：整個 meta 進來會把 expectedVersion 一起帶進 body。
					{
						operationReason: checked.operationReason,
						notifySeller: checked.notifySeller,
					},
					{
						name: body.name,
						categoryId: String(body.categoryId ?? ""),
						...(body.description === undefined
							? {}
							: { description: toDescriptionWireInput(body.description) }),
						...(body.images === undefined ? {} : { images: body.images }),
						skus: (body.skus ?? []).map(toDelegatedSkuInput),
						...(body.origin === undefined ? {} : { origin: body.origin }),
						...(body.certifications === undefined
							? {}
							: { certifications: body.certifications }),
						...(body.hashtags === undefined ? {} : { hashtags: body.hashtags }),
					},
				),
				idempotencyKey: newIdempotencyKey(),
			});
			// ⚠️ 建立後的資源在 `resource` 之下（統一 mutation envelope）。直接讀頂層
			// `data.productId` 拿到的是 `undefined`：沒有例外、HTTP 200、稽核照樣
			// SUCCEEDED，只有接續的 `/publish` 被靜默跳過 —— 貼文那一面真的發生過。
			const data = asRecord(env.data);
			return {
				id: str(mutationResource(data).productId),
				version: str(data.version),
			};
		},

		updateProduct() {
			return Promise.reject(new DelegatedRepoNotWiredError("商品編輯"));
		},

		/** 賣家端是「傳 status 值」，代行是兩支命令端點（body 只收共通三欄）。 */
		async updateProductStatus(id, status, meta): Promise<ProductWriteResult> {
			const publish = status === "4004";
			const checked = requireMeta(meta, publish ? "上架商品" : "下架商品");
			const env = await fetcher<unknown>(
				`${at(id)}/${publish ? "publish" : "unpublish"}`,
				{
					method: "POST",
					json: mutationBody(checked),
					idempotencyKey: newIdempotencyKey(),
				},
			);
			// 上架也好下架也好，每一次都把版本推進一格。不帶它回去，同一次載入內
			// 接著做的第二次上下架就拿著過期的 `expectedVersion` 送出 ⇒ 必 9266
			// （`ProductWriteResult`）。
			return { version: str(asRecord(env.data).version) };
		},

		deleteProduct() {
			return Promise.reject(new DelegatedEndpointMissingError("刪除商品"));
		},

		async batchUnpublish(items, meta): Promise<DelegatedBatchResult> {
			const checked = requireMeta(meta, "批次下架");
			if (items.length === 0 || items.length > DELEGATED_BATCH_UNPUBLISH_MAX) {
				throw new Error(
					`批次下架一次只能處理 1–${DELEGATED_BATCH_UNPUBLISH_MAX} 筆商品。`,
				);
			}
			const env = await fetcher<unknown>(`${root}/batch-unpublish`, {
				method: "POST",
				// notifySeller 是整批一個旗標、不逐項（L3126）。
				json: mutationBody(
					{
						operationReason: checked.operationReason,
						notifySeller: checked.notifySeller,
					},
					{ items: items.map((item) => ({ ...item })) },
				),
				idempotencyKey: newIdempotencyKey(),
			});
			const data = asRecord(env.data);
			const results = Array.isArray(data.results) ? data.results : [];
			return {
				requested: Number(data.requested ?? items.length),
				succeeded: Number(data.succeeded ?? 0),
				failed: Number(data.failed ?? 0),
				results: results.map((entry) => {
					const item = asRecord(entry);
					return {
						productId: String(item.productId ?? ""),
						ok: item.ok === true,
						code: Number(item.code ?? 0),
						version: String(item.version ?? ""),
						auditLogId: String(item.auditLogId ?? ""),
					};
				}),
			};
		},
	};
}

/** 代行貼文列表 DTO → 賣家中心動態列表吃的 `PostListItem`。 */
export function mapDelegatedPost(raw: unknown, storeId: string): PostListItem {
	const r = asRecord(raw);
	const hiddenAt = r.hiddenAt == null ? undefined : epochSeconds(r.hiddenAt);
	return {
		id: str(r.postId) ?? "",
		sellerId: storeId,
		// 代行 DTO 沒有 storeCode／店名／頭像：那是買家面的展示欄，代行的貼文列表
		// 是店家自己的內容管理視圖，不渲染店面連結。空字串是既有契約的「查不到」值。
		storeCode: "",
		sellerName: "",
		logoKey: "",
		content: str(r.content) ?? "",
		imageKeys: Array.isArray(r.imageKeys) ? (r.imageKeys as string[]) : [],
		tags: Array.isArray(r.tags) ? (r.tags as string[]) : [],
		product: null,
		// 互動計數不在代行 DTO 內。0 在這裡是「沒有這個數字」的佔位，而列表本來就
		// 不以它排序或篩選；渲染端在代行模式下不畫互動列（§13 定案）。
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: epochSeconds(r.createdAt),
		...(hiddenAt ? { hiddenAt } : {}),
		...(str(r.hiddenReason) ? { hiddenReason: String(r.hiddenReason) } : {}),
		// 草稿與已發布在列表上原本長得一樣（草稿沒有任何標記），而代行者正是靠這個
		// 列表判斷這家店的經營狀態。上游 backend#2470 一系列補了這一欄。
		...(str(r.publishStatus) ? { publishStatus: String(r.publishStatus) } : {}),
	};
}

/** 代行貼文詳情 DTO → 店主詳情的形狀（代行 DTO 沒有店名、頭像與互動計數；下架資訊與列表同口徑）。 */
export function mapDelegatedPostDetail(
	raw: unknown,
	storeId: string,
): MyPostDetail {
	const r = asRecord(raw);
	const products = Array.isArray(r.products) ? r.products : [];
	const hiddenAt = r.hiddenAt == null ? undefined : epochSeconds(r.hiddenAt);
	return {
		id: str(r.postId) ?? "",
		sellerId: storeId,
		sellerName: "",
		logoKey: "",
		content: str(r.content) ?? "",
		imageKeys: Array.isArray(r.imageKeys) ? (r.imageKeys as string[]) : [],
		tags: Array.isArray(r.tags) ? (r.tags as string[]) : [],
		products: products.map((entry) => {
			const p = asRecord(entry);
			return {
				id: str(p.productId) ?? "",
				imageKey: str(p.imageKey) ?? "",
				name: str(p.name) ?? "",
				price: str(p.price) ?? "",
			};
		}),
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: epochSeconds(r.createdAt),
		...(hiddenAt ? { hiddenAt } : {}),
		...(str(r.hiddenReason) ? { hiddenReason: String(r.hiddenReason) } : {}),
		...(str(r.publishStatus) ? { publishStatus: String(r.publishStatus) } : {}),
	};
}

export function createDelegatedPostRepo(
	fetcher: RepoFetcher,
	storeId: string,
): SellerPostRepo & DelegatedPostCommands {
	const root = `${POSTS_ROOT}/${encodeURIComponent(storeId)}`;
	const at = (id: number | string) =>
		`${root}/${encodeURIComponent(String(id))}`;

	async function command(
		path: string,
		meta: DelegatedMutationMeta,
		extra: Record<string, unknown> = {},
	): Promise<void> {
		await fetcher(path, {
			method: "POST",
			json: mutationBody(meta, extra),
			idempotencyKey: newIdempotencyKey(),
		});
	}

	return {
		async fetchSellerPostDetail(id): Promise<MyPostDetail> {
			const env = await fetcher<unknown>(at(id));
			return mapDelegatedPostDetail(env.data, storeId);
		},

		async fetchSellerPosts(_sellerId, params): Promise<PostListItem[]> {
			const env = await fetcher<unknown>(
				`${root}?page=1&page_size=${params?.pageSize ?? 50}`,
			);
			const data = asRecord(env.data);
			const records = Array.isArray(data.records) ? data.records : [];
			return records.map((record) => mapDelegatedPost(record, storeId));
		},

		/** 一律建為未發布：`publishMode`／`scheduledPublishAt` 出現即 9260（L3205）。 */
		async createSellerPost(
			body,
			meta,
		): Promise<{ id?: number; version?: string }> {
			const checked = requireMeta(meta, "建立貼文");
			const env = await fetcher<unknown>(root, {
				method: "POST",
				json: mutationBody(checked, {
					content: body.content,
					stagedTokens: body.stagedToken ? [body.stagedToken] : [],
					tags: body.tags ?? [],
					productIds: body.productId ? [String(body.productId)] : [],
				}),
				idempotencyKey: newIdempotencyKey(),
			});
			// 代行回 `resource.postId`（string TSID）＋頂層 `version`；兩者都是接著
			// 打 `/publish` 的必要輸入 —— 那一支的 `expectedVersion` 沒有第二個來源。
			const data = asRecord(env.data);
			const postId = mutationResource(data).postId;
			return {
				id: postId as unknown as number,
				version: str(data.version),
			};
		},

		async updateSellerPost(id, body, meta): Promise<void> {
			const checked = requireMeta(meta, "編輯貼文");
			await fetcher(at(id), {
				method: "PATCH",
				json: mutationBody(checked, {
					...(body.content === undefined ? {} : { content: body.content }),
					...(body.newStagedToken
						? { stagedTokens: [body.newStagedToken] }
						: {}),
					...(body.tags ? { tags: body.tags } : {}),
					...(body.productId ? { productIds: [String(body.productId)] } : {}),
				}),
				idempotencyKey: newIdempotencyKey(),
			});
		},

		/**
		 * 賣家中心的發布／下架入口在代行工作區不渲染（動作欄整段 `!delegated`），
		 * 但介面上這一支必須有實作 —— 走的是代行自己的兩支命令端點，共通三欄由
		 * `requireMeta` 強制。用 reject 佔位會在有人接線的那天變成一個「端點不存在」
		 * 的謊：`/admin/sellers/{id}/posts/{id}/publish` 一直都在。
		 *
		 * ⚠️ 代行的 `/publish` 必填 `expectedVersion`，而它只在 `meta` 裡 —— 列表的
		 * DTO 不帶版本，所以呼叫端得自己先取得版本再進來，不能拿列表那一列直接送。
		 */
		setSellerPostPublishStatus(id, target, meta): Promise<void> {
			const publish = target === PostPublishStatus.PUBLISHED;
			return command(
				`${at(id)}/${publish ? "publish" : "unpublish"}`,
				requireMeta(meta, publish ? "發布貼文" : "下架貼文"),
			);
		},

		deleteSellerPost() {
			return Promise.reject(new DelegatedEndpointMissingError("刪除貼文"));
		},

		async count(): Promise<number> {
			const env = await fetcher<unknown>(`${root}?page=1&page_size=1`);
			return Number(asRecord(env.data).total ?? 0);
		},

		publish: (postId, meta, schedule) =>
			command(
				`${at(postId)}/publish`,
				requireMeta(meta, "發布貼文"),
				schedule
					? {
							publishMode: schedule.publishMode,
							...(schedule.scheduledPublishAt
								? { scheduledPublishAt: schedule.scheduledPublishAt }
								: {}),
						}
					: {},
			),
		unpublish: (postId, meta) =>
			command(`${at(postId)}/unpublish`, requireMeta(meta, "下架貼文")),
	};
}

export interface DelegatedRepoBundle {
	repos: SellerDataRepos;
	products: DelegatedProductCommands;
	posts: DelegatedPostCommands;
}

/**
 * 一個工作階段一份（目標店家綁在每一條路徑上）。
 *
 * `fetcher` 可注入是為了測試；生產路徑一律是 `delegatedFetch` —— 它打的是同源的
 * `/adm/api/*`，憑證是 `Path=/adm` 的 httpOnly cookie，這一層看不到也碰不到。
 */
export function createDelegatedRepos(
	storeId: string,
	fetcher: RepoFetcher = delegatedFetch,
): DelegatedRepoBundle {
	const products = createDelegatedProductRepo(fetcher, storeId);
	const posts = createDelegatedPostRepo(fetcher, storeId);
	return {
		repos: {
			storefront: createDelegatedStorefrontRepo(fetcher, storeId),
			products,
			posts,
			orders: createOrderRepo(fetcher, {
				root: `${ORDERS_ROOT}/${encodeURIComponent(storeId)}`,
			}),
			media: createMediaUploadRepo(fetcher, { root: MEDIA_ROOT }),
		},
		products,
		posts,
	};
}
