/**
 * Product management — seller-side under `/app/products`.
 *
 *   GET  /app/products                  list my products (auth)
 *   POST /app/products                  create
 *   PUT  /app/products/{id}             update
 *   GET  /app/products/{id}             edit-form payload (full incl. SKU)
 *   GET  /app/products/{id}/skus        SKU list
 *
 * Status filter values mirror the buyer Flutter app: 0=draft, 1=active,
 * 2=offshelf, etc. Backends differ — we just pass through.
 */

import { fetchProducts } from "../api";
import { apiClient, authedFetch } from "../api/client";
import type { DelegatedMutationMeta, RepoFetcher } from "../api/delegated";
import type { ProductCertificationCode } from "./certifications";

export type { ProductCertificationCode } from "./certifications";

export interface SellerProductListItem {
	// Snowflake IDs exceed Number.MAX_SAFE_INTEGER, so api/json.ts hands them
	// back as strings — accept either form so consumers can pass through.
	id: number | string;
	/**
	 * 樂觀鎖版本 —— **只有代行路徑有值**（56 號 §6.1.8）。上架／下架／批次下架的
	 * `expectedVersion` 取自這裡；賣家自助端的列表不回它。
	 */
	version?: string;
	name: string;
	/** Snake-case alias kept for back-compat with existing renderers
	 *  (SellerProductGridCard, InventoryPage). Filled from wire `imageUrl`. */
	main_image?: string;
	/** Mirror of wire `imageUrl` (a fully-qualified URL). */
	image_url?: string;
	/** Single-price legacy field. Not in the new wire shape; consumers
	 *  should fall back to `price_min` / `price_max`. */
	price?: string;
	/** Effective sale price aggregated from SKUs (priceMin/Max in wire). */
	price_min?: string;
	price_max?: string;
	/** Original list price — used for strikethrough display when a sale is
	 *  active (listPriceMin/Max in wire). */
	list_price_min?: string;
	list_price_max?: string;
	/** Per-SKU sale-price aggregate (salePriceMin/Max in wire). Empty when
	 *  no SKU has a discount running. */
	sale_price_min?: string;
	sale_price_max?: string;
	/** Wire emits a numeric category id; legacy snake_case payloads sometimes
	 *  shipped it as a string, hence the union — `SellerProductDetail`
	 *  narrows back to `number` for the editor form. */
	category_id?: string | number;
	/** Display label like "蔬果" / "水果" — taken from wire `categoryName`. */
	category_name?: string;
	origin?: string;
	/** Aggregate rating, wire returns as string. */
	rating?: string;
	/** NOT in the seller list endpoint today — the backend `GET /app/products`
	 *  (handler.SellerProductItem) does not emit stock / sales / favourites. Kept
	 *  on the type (and mapped below) so the columns light up automatically once
	 *  the backend ticket adds them; until then the table renders "—".
	 *  See docs/backend-issues/B-192-seller-product-list-metrics.md. */
	stock?: number;
	/** Σ(sku.stock × price) for the whole product. NOT in the list endpoint
	 *  today (B-192); mapped forward-compat so 庫存管理 can drop its per-product
	 *  `/skus` N+1 the moment the backend ships it. */
	inventory_value?: number;
	sales_count?: number;
	/** 被收藏數 — how many buyers favourited this product (favoriteCount on the
	 *  wire). Pending backend support; see B-192. */
	favorite_count?: number;
	/**
	 * Product lifecycle. Wire emits a CodeMeta string ("4001"=草稿,
	 * "4002"=審核中, "4003"=未啟用, "4004"=啟用); legacy server paths may
	 * still return a numeric code, hence the union.
	 */
	status?: string | number;
	/**
	 * 後端下發的狀態顯示名（CodeMeta Label：`啟用`／`未啟用`／`草稿`／`審核中`）——
	 * **只有代行路徑有值**。前端自建一份對照表會在後端新增狀態時顯示成空白或代碼，
	 * 所以有 label 就用 label，沒有才退回本地表。
	 */
	status_label?: string;
	/** Free-form / preset tags (#當季、#禮盒…). The list endpoint emits these as
	 *  `hashtags`; values may be plain strings or `{name}` objects. */
	hashtags?: Array<string | { name?: string }>;
	/** Seller's account id (wire `ownerId`). All of a seller's own products
	 *  share the same value — used to backfill 銷量 from the public list. */
	owner_id?: string;
	created_at?: number | string;
}

/**
 * Legacy single-temp keyword kept readable for any old hydration paths that
 * still receive it. The canonical shape is now `delivery_temp: string[]`
 * (codes "3201"/"3202"/"3203"), matching the Flutter app's multi-select
 * "可配送溫層（可複選）" model. New code should not write this field.
 *
 * @deprecated use `delivery_temp` (string[] of codes) instead.
 */
export type ProductTempLayer = "ambient" | "chilled" | "frozen";

/** Canonical delivery-temp codes from `lib/codemeta/enums` ShippingTemperature. */
export const DELIVERY_TEMP_CODES = {
	AMBIENT: "3201",
	CHILLED: "3202",
	FROZEN: "3203",
} as const;
export type DeliveryTempCode =
	(typeof DELIVERY_TEMP_CODES)[keyof typeof DELIVERY_TEMP_CODES];

/**
 * Coerce the various wire shapes the backend has historically emitted for a
 * product's temp set into a canonical `string[]` of codes. Accepts:
 *   - array of codes:        ["3201","3202","3203"]
 *   - comma-delimited string: "3201,3202,3203"
 *   - single code:           "3203"
 *   - legacy keyword:        "frozen" / "chilled" / "ambient"
 * Returns an empty array if nothing recognisable is present.
 */
export function parseDeliveryTempCodes(value: unknown): string[] {
	if (Array.isArray(value)) {
		return value
			.map((v) => String(v).trim())
			.filter((v) => v === "3201" || v === "3202" || v === "3203");
	}
	if (typeof value === "string") {
		const trimmed = value.trim();
		if (!trimmed) return [];
		// Legacy single-keyword
		if (trimmed === "ambient") return ["3201"];
		if (trimmed === "chilled") return ["3202"];
		if (trimmed === "frozen") return ["3203"];
		// Comma-delimited codes
		return trimmed
			.split(",")
			.map((s) => s.trim())
			.filter((s) => s === "3201" || s === "3202" || s === "3203");
	}
	return [];
}

/**
 * A certification the seller attached to the product.
 *   - `code`     CertificationType value ("3701"…"3706"); sent to the backend
 *                as `certifications[].name`.
 *   - `certNo`   seller-entered certificate number.
 *
 * The legacy product wire carries no review status. Do not add a local
 * `verified`/`approved` flag here: only the planned manual-review API may supply
 * authoritative review state.
 */
export interface ProductCertificationEntry {
	code: ProductCertificationCode;
	certNo?: string;
}

export interface SellerProductDetail extends SellerProductListItem {
	/**
	 * Backend now returns the structured shape; legacy listings may still come
	 * back as a plain string. Editor code reads with `parseDescription()` and
	 * always writes the structured form.
	 */
	description?: string | ProductDescription;
	images?: ProductImageDraft[];
	category_id?: number;
	category_name?: string;
	/** Free-form / preset tags (#當季、#禮盒、#媽媽推薦…) used for search,
	 *  filter chips, and product-card badges. */
	tags?: string[];
	/** Canonical multi-value delivery temp set. Codes "3201"/"3202"/"3203". */
	delivery_temp?: string[];
	/** @deprecated single-temp legacy mirror; new code should write `delivery_temp`. */
	temp_layer?: ProductTempLayer;
	/** ISO 3166-2 region or city/county; surfaced as 「產地」 on PDP. */
	origin?: string;
	/** Quality marks shown as trust badges (有機 / 履歷 …). */
	certifications?: ProductCertificationEntry[];
	/** Pre-order: ship-after-order date in ISO yyyy-mm-dd. */
	preorder_ship_at?: string | null;
	/** Lower bound for "low stock" warning in seller dashboard. */
	low_stock_threshold?: number;
	/** Best-before days; combined with check-in time to compute "即期" badge. */
	expiry_days?: number;
	/** Per-order purchase cap. */
	max_per_order?: number;
	/** Whether seller wants the storefront to show countdown when sale active. */
	show_sale_countdown?: boolean;
	skus?: ProductSku[];
}

/**
 * A single deliverable shipping option carried on a SKU (mirrors the seller
 * app's `shippingMethods` entry and the backend `CreateShippingMethodInput` /
 * `handler.ShippingMethodResponse`). Stored snake_case in the form draft and
 * serialised to camelCase (`{ type, temperature, feePayer, freeThreshold }`)
 * at submit. All values are CodeMeta strings — see `lib/codemeta/enums.ts`.
 */
export interface SkuShippingMethod {
	/** ShippingMethod value: "3001" 黑貓宅急便 / "3002" 7-11店到店 */
	type: string;
	/** ShippingTemperature value: "3201" 常溫 / "3202" 冷藏 / "3203" 冷凍。 */
	temperature: string;
	/** ShippingFeePayer value: "3101" 買方 / "3102" 賣方。 */
	fee_payer: string;
	/** 免運門檻（新台幣元字串；僅賣家負擔時有效，"0" = 無條件免運）。 */
	free_threshold: string;
}

export interface ProductSku {
	/** Backend SKU TSID (string-on-the-wire)；新建未存的 SKU 為 `""`。
	 *  禁止改回 number — 18 位 TSID 超出 JS Number 安全範圍 (2^53)，
	 *  Number() 解析會精度遺失導致 PUT 後 backend selectById miss。 */
	id: string;
	sku_name?: string;
	spec_name?: string;
	price?: string;
	/** Original "list" price; when sale_price is set + active, store shows it
	 *  with strikethrough and computes discount %. */
	original_price?: string;
	/** Active sale price for the SKU. */
	sale_price?: string;
	stock?: number;
	/** 重量單位（WeightUnit code：3301 公斤 / 3302 台斤 / 3303 公克）。後端 product_sku.unit
	 *  嚴格驗證為 WeightUnit；重量（weight）以此單位計，結帳依此換算算運費。 */
	unit?: string;
	weight?: string;
	/** 包裝尺寸（公分，字串）→ 後端 lengthCm / widthCm / heightCm，用於運費試算。 */
	length_cm?: string;
	width_cm?: string;
	height_cm?: string;
	/** 此 SKU 可選的配送方案（黑貓 / 711 × 溫層），含運費負擔方與免運門檻。 */
	shipping_methods?: SkuShippingMethod[];
}

export interface ProductListResult {
	records: SellerProductListItem[];
	total: number;
	page: number;
	pageSize: number;
}

interface ServerListEnv {
	records?: SellerProductListItem[];
	list?: SellerProductListItem[];
	total?: number | string;
	page?: number | string;
	pageSize?: number | string;
	page_size?: number | string;
}

/**
 * `GET /app/products` (handler.SellerProductItem) emits camelCase fields
 * — `imageUrl`, `priceMin`, `categoryName`, `status` as a CodeMeta string
 * ("4001".."4004"), etc. Every page-side consumer of this repo was written
 * against an older snake_case shape, so we mirror each wire field onto its
 * snake_case alias here. Existing snake_case fields (legacy responses,
 * tests) win when present, so this stays compatible with anything still on
 * the old shape.
 */
function num(value: unknown): number | undefined {
	if (value == null || value === "") return undefined;
	const n = Number(value);
	return Number.isFinite(n) ? n : undefined;
}

function str(value: unknown): string | undefined {
	if (value == null) return undefined;
	const s = String(value).trim();
	return s ? s : undefined;
}

function mapWireItem(raw: unknown): SellerProductListItem {
	const r = (raw ?? {}) as Record<string, unknown>;
	// §231 flat-pair: the bare objectKey is the SSOT (`imageKeys[0]` for the
	// multi-image list). The `imageUrl`/`imageUrls` twins are
	// `mediaurl.BuildURL(base, imageKey…)` (gateway product.go:924-927) —
	// derived from these very keys, so they are not read.
	const imageUrl =
		str(r.imageKey) ?? str((r.imageKeys as unknown[] | undefined)?.[0]);
	return {
		id: (r.id ?? "") as string | number,
		name: (r.name as string) ?? "",
		main_image: imageUrl,
		image_url: imageUrl,
		price: str(r.price),
		price_min: str(r.price_min) ?? str(r.priceMin),
		price_max: str(r.price_max) ?? str(r.priceMax),
		list_price_min: str(r.list_price_min) ?? str(r.listPriceMin),
		list_price_max: str(r.list_price_max) ?? str(r.listPriceMax),
		sale_price_min: str(r.sale_price_min) ?? str(r.salePriceMin),
		sale_price_max: str(r.sale_price_max) ?? str(r.salePriceMax),
		category_id: str(r.category_id) ?? str(r.categoryId),
		category_name: str(r.category_name) ?? str(r.categoryName),
		origin: str(r.origin),
		rating: str(r.rating),
		stock: num(r.stock),
		inventory_value:
			num(r.inventory_value) ?? num(r.inventoryValue) ?? num(r.stockValue),
		sales_count: num(r.sales_count) ?? num(r.salesCount),
		favorite_count:
			num(r.favorite_count) ?? num(r.favoriteCount) ?? num(r.favorites_count),
		status: (r.status as string | number | undefined) ?? undefined,
		hashtags: Array.isArray(r.hashtags)
			? (r.hashtags as Array<string | { name?: string }>)
			: Array.isArray(r.tags)
				? (r.tags as Array<string | { name?: string }>)
				: undefined,
		owner_id: str(r.owner_id) ?? str(r.ownerId),
		created_at: (r.created_at ?? r.createdAt) as number | string | undefined,
	};
}

function normalizeList(d: unknown, fallbackSize: number): ProductListResult {
	if (Array.isArray(d)) {
		return {
			records: d.map(mapWireItem),
			total: d.length,
			page: 1,
			pageSize: fallbackSize,
		};
	}
	const env = (d ?? {}) as ServerListEnv;
	const rawRecords = (env.records ?? env.list ?? []) as unknown[];
	return {
		records: rawRecords.map(mapWireItem),
		total: Number(env.total ?? rawRecords.length),
		page: Number(env.page ?? 1),
		pageSize: Number(env.pageSize ?? env.page_size ?? fallbackSize),
	};
}

export interface SellerProductListParams {
	page?: number;
	pageSize?: number;
	status?: number | string;
}

/**
 * 商品寫入的回執。
 *
 * `version` **只有代行路徑會回**（56 號 §6.1.8，Gateway `delegatedMutationData` 的
 * 註解寫得最直白：「the NEW version after the write — the front end feeds it straight
 * back as the next expectedVersion」）。它是**同一次載入內第二次寫入**的唯一版本
 * 來源：每成功一次就把版本推進一格，呼叫端手上那個 `expectedVersion` 在那一刻就
 * 過期了。丟掉它的失敗形狀是「第一次上架成功、第二次下架必 9266」，而畫面上寫的是
 * 「資料已被更新」—— 現場只有代行者一個人在改，那句話讀起來像系統壞了，還會在稽核
 * 表留下一列 FAILED。
 *
 * ⚠️ **不得改成「背景 refetch 回來就把版本抄進本地狀態」**：那等於把賣家在這段期間
 * 的修改當成自己的版本，樂觀鎖從此永遠不會擋 —— 用一個「會被擋下來的錯誤」換一次
 * 「無聲的覆寫」。只有「自己這一次寫入的回應」可以更新版本。
 * （店面側的同一條規則見 `lib/seller/repo.ts` 的 `StorefrontWriteResult`。）
 */
export interface ProductWriteResult {
	version?: string;
}

/** 建立商品的回執：新資源 id ＋（代行才有的）版本。 */
export interface ProductCreateResult extends ProductWriteResult {
	id?: number | string;
}

/**
 * 商品資料層的介面（56 號 §4.5 L1053）。
 *
 * 代行工作區沿用賣家中心的元件，而元件不得自己取得傳輸層 —— 一律經工廠注入。
 * 賣家路徑注入 `authedFetch`、代行路徑注入 `delegatedFetch`（`lib/api/delegated.ts`）。
 *
 * `meta` 是代行寫入類 mutation 的共通三欄（`expectedVersion`／`operationReason`／
 * `notifySeller`）：**賣家路徑一律不傳**，賣家實作也不讀它。
 */
export interface SellerProductRepo {
	fetchSellerProducts(
		params?: SellerProductListParams,
	): Promise<ProductListResult>;
	fetchSellerProductEdit(
		id: number | string,
	): Promise<SellerProductDetail | null>;
	fetchProductSkus(id: number | string): Promise<ProductSku[]>;
	/**
	 * 建立商品。
	 *
	 * ⚠️ `version` 只有代行路徑會回：代行的「建立並上架」是**兩支端點**（`POST
	 * .../products` → `POST .../products/{id}/publish`），而第二支的 `expectedVersion`
	 * 沒有第二個來源 —— 少了它就只能先建立、再重抓一次 detail，那一次重抓與建立
	 * 之間的任何寫入都會讓上架打在別人的版本上。賣家路徑沒有這個概念，恆為
	 * `undefined`。
	 */
	createProduct(
		body: ProductWriteBody,
		meta?: DelegatedMutationMeta,
	): Promise<ProductCreateResult>;
	updateProduct(
		id: number | string,
		body: Partial<ProductWriteBody>,
		meta?: DelegatedMutationMeta,
	): Promise<void>;
	/** 上／下架。代行實作把它對映到 `/publish`／`/unpublish` 兩支命令端點。 */
	updateProductStatus(
		id: number | string,
		status: "4003" | "4004",
		meta?: DelegatedMutationMeta,
	): Promise<ProductWriteResult>;
	deleteProduct(id: number | string): Promise<void>;
}

export interface SellerProductRepoOptions {
	/** 資源根。賣家 `/app/products`；代行由自己的工廠決定（BFF 路徑不同形）。 */
	root?: string;
}

export function createSellerProductRepo(
	fetcher: RepoFetcher,
	options: SellerProductRepoOptions = {},
): SellerProductRepo {
	const root = options.root ?? "/app/products";
	const at = (id: number | string) =>
		`${root}/${encodeURIComponent(String(id))}`;

	return {
		async fetchSellerProducts({
			page = 1,
			pageSize = 20,
			status,
		}: SellerProductListParams = {}): Promise<ProductListResult> {
			const qs = new URLSearchParams({
				page: String(page),
				page_size: String(pageSize),
			});
			if (status != null) qs.set("status", String(status));
			const env = await fetcher<unknown>(`${root}?${qs}`);
			return normalizeList(env.data, pageSize);
		},

		async fetchSellerProductEdit(
			id: number | string,
		): Promise<SellerProductDetail | null> {
			const env = await fetcher<SellerProductDetail>(at(id), {
				throwOnHttpError: false,
			});
			return env.data ?? null;
		},

		async fetchProductSkus(id: number | string): Promise<ProductSku[]> {
			const env = await fetcher<
				{ skus?: unknown[]; records?: unknown[] } | unknown[]
			>(`${at(id)}/skus`, {
				throwOnHttpError: false,
			});
			// Backend wraps SKUs under `data.skus` (handler.SellerSkuListResponse);
			// keep bare-array / `records` fallbacks for shape drift.
			const raw = Array.isArray(env.data)
				? env.data
				: (env.data?.skus ?? env.data?.records ?? []);
			return raw.map(mapWireSku);
		},

		async createProduct(body: ProductWriteBody): Promise<ProductCreateResult> {
			const env = await fetcher<ProductCreateResult>(root, {
				method: "POST",
				json: {
					idempotencyKey: body.idempotencyKey ?? newIdempotencyKey(),
					...body,
				},
			});
			return env.data ?? {};
		},

		async updateProduct(
			id: number | string,
			body: Partial<ProductWriteBody>,
		): Promise<void> {
			await fetcher(at(id), {
				method: "PUT",
				json: {
					idempotencyKey: body.idempotencyKey ?? newIdempotencyKey(),
					...body,
				},
			});
		},

		async updateProductStatus(
			id: number | string,
			status: "4003" | "4004",
		): Promise<ProductWriteResult> {
			await fetcher(`${at(id)}/status`, {
				method: "PUT",
				json: { status },
			});
			// 賣家自助端沒有樂觀鎖版本（`PUT /app/products/{id}/status` 不回它），
			// `version` 留空 —— 清單那一側的 `patchStatusLocal` 對 undefined 不加欄位，
			// 這條路徑的行為一格未動。
			return {};
		},

		async deleteProduct(id: number | string): Promise<void> {
			await fetcher(at(id), { method: "DELETE" });
		},
	};
}

/** 賣家路徑的預設實例。`/seller-center` 既有呼叫端全部經由下面的具名匯出用它。 */
export const sellerProductRepo = createSellerProductRepo(authedFetch);

export const fetchSellerProducts: SellerProductRepo["fetchSellerProducts"] = (
	params,
) => sellerProductRepo.fetchSellerProducts(params);

/**
 * Backfill 銷量 for the seller's own products from the PUBLIC list, which DOES
 * return `salesCount` (`handler.ProductItem`) — unlike the seller list endpoint
 * (`handler.SellerProductItem`, see B-192). One extra request, no per-product
 * N+1. Caveat: `/public/products` only returns 上架中 (status 4004) products, so
 * drafts / 未啟用 stay unmapped (they have ~0 sales anyway). Returns a
 * `productId → salesCount` map. Delete this once B-192 ships `salesCount` on the
 * seller list (the list mapping already reads `sales_count` forward-compat).
 */
export async function fetchSellerSalesCounts(
	sellerStoreId: string | number,
	pageSize = 100,
): Promise<Map<string, number>> {
	const map = new Map<string, number>();
	if (sellerStoreId == null || sellerStoreId === "") return map;
	try {
		const env = await fetchProducts("", 1, pageSize, undefined, undefined, {
			sellerStoreIds: [sellerStoreId],
		});
		const records = ((env as { data?: { records?: unknown[] } })?.data
			?.records ?? []) as Array<Record<string, unknown>>;
		for (const r of records) {
			const id = r.id != null ? String(r.id) : "";
			const sales = num(r.salesCount) ?? num(r.sales_count);
			if (id && sales != null) map.set(id, sales);
		}
	} catch {
		/* best-effort backfill — leave 銷量 as "—" on failure */
	}
	return map;
}

export const fetchSellerProductEdit: SellerProductRepo["fetchSellerProductEdit"] =
	(id) => sellerProductRepo.fetchSellerProductEdit(id);

/** Map a raw `handler.SellerSkuResponse` (camelCase, all-string fields) onto
 *  our snake_case `ProductSku`. The list endpoint only carries `price` +
 *  `stockQty` (no sale window), so the sale fields stay undefined here. */
function mapWireSku(raw: unknown): ProductSku {
	const r = (raw ?? {}) as Record<string, unknown>;
	return {
		// SKU id 一律 string pass-through（backend TSID 18 位超出 JS Number 安全範圍）
		id: typeof r.id === "string" ? r.id : r.id != null ? String(r.id) : "",
		sku_name: str(r.skuName) ?? str(r.sku_name),
		spec_name: str(r.specName) ?? str(r.spec_name),
		price: str(r.price),
		original_price: str(r.originalPrice) ?? str(r.original_price),
		sale_price: str(r.salePrice) ?? str(r.sale_price),
		stock: num(r.stockQty) ?? num(r.stock),
		unit: str(r.unit),
		weight: str(r.weight),
	};
}

export const fetchProductSkus: SellerProductRepo["fetchProductSkus"] = (id) =>
	sellerProductRepo.fetchProductSkus(id);

/**
 * Structured product description matching mobile's contract.
 * `version: "1.0"` is the only schema in production today.
 */
/**
 * Inline run（02 §3.6 inline-run 模型）。text block 的 `runs` 與 table cell 的
 * `runs` 皆用此型別；句中任意片段可獨立帶粗體 / 斜體 / 底線 / 顏色 / 連結。
 */
export interface InlineRun {
	text: string;
	bold?: boolean;
	italic?: boolean;
	underline?: boolean;
	color?: string; // #RRGGBB
	href?: string; // http(s):// 絕對 URL
}

export interface ProductDescriptionBlock {
	type: "text" | "table" | "list" | "image";
	id?: string;
	/** text blocks：行內 runs（取代 legacy content） */
	runs?: InlineRun[];
	content?: string; // text blocks（legacy 唯讀相容）
	/** table blocks：`{ withHeadings, rows:[[{runs,textAlign?}]] }` 的 JSON 字串 */
	tableJson?: string;
	/** list blocks：`{ ordered, items:[{runs}] }` 的 JSON 字串 */
	listJson?: string;
	assetId?: string; // existing image reference
	stagedToken?: string; // newly uploaded image
	/** 圖片 objectKey（wire `imageKey`；讀寫兩面同名，backend#2235 起與 gallery
	 *  條目、商品卡、購物車項、貼文商品共用同一組欄名，49 §6.1）。 */
	imageKey?: string;
	/** Full CDN URL the backend builds from `imageKey`; the only renderable src
	 *  for a saved image — the bare key resolves to a 404 relative URL in the
	 *  browser. Read-only：送出時只送 `imageKey`。 */
	imageUrl?: string;
	alt?: string;
	width?: string;
	height?: string;
	style?: Record<string, unknown>;
}

/**
 * 送後端的 gallery 圖片（對齊 `handler.CreateProductImageInput`）。兩種身分擇一：
 *   - `stagedToken` — 這次新上傳的圖，後端 finalize 後入庫
 *   - `assetId`     — 商品既有的圖，後端以此在既有 gallery 內比對保留
 * 後端 `ProductService#processImageUpdate` 依送出陣列的順序逐項組裝，兩種可交錯混用；
 * **沒出現在陣列裡的既有圖會被移除**，所以送出時務必包含全部要保留的圖。
 */
export type ProductImageInput = { stagedToken: string } | { assetId: string };

/**
 * 表單 draft 的 gallery 圖片。
 *   - `url`         顯示用（`buildImageUrl` 可再套 preset）
 *   - `assetId`     既有圖的媒體資產 TSID，來自 detail 回應的 `gallery[].assetId`；
 *                   存檔時原樣送回即可保留該圖，不需重新下載+上傳
 *   - `stagedToken` 這次新上傳（或既有圖重傳）後拿到的 staged token
 * 兩者皆無 = 只有一個外部 url 的來源圖（商品匯入），存檔前必須重傳取得 stagedToken。
 */
export interface ProductImageDraft {
	url: string;
	assetId?: string;
	stagedToken?: string;
}

/**
 * An update to the product image gallery — either a replacement array or a
 * functional updater. The functional form is the escape hatch concurrent
 * uploads need: each in-flight upload appends onto the *latest* gallery state
 * instead of a stale closure snapshot, so multi-select / overlapping uploads
 * accumulate every image instead of last-write-wins clobbering down to one.
 */
export type ImageGalleryUpdate =
	| ProductImageDraft[]
	| ((prev: ProductImageDraft[]) => ProductImageDraft[]);

/**
 * Paragraph-level visual style stored on a text block. Each property is
 * persisted as a flat key on `block.style` and reapplied verbatim on the
 * public PDP and the seller-side editor preview. Anything not on this
 * whitelist is dropped on read so seller-supplied JSON can never inject
 * arbitrary CSS (e.g. `background-image: url(javascript:...)`).
 */
export type BlockTextStyle = {
	fontSize?: string;
	fontWeight?: 400 | 700;
	fontStyle?: "italic";
	textDecoration?: "underline";
	textAlign?: "left" | "center" | "right";
	color?: string;
};

export const BLOCK_FONT_SIZES: ReadonlyArray<{
	value: string;
	label: string;
}> = [
	{ value: "14px", label: "小" },
	{ value: "16px", label: "內文" },
	{ value: "20px", label: "大" },
	{ value: "26px", label: "標題" },
];

export const BLOCK_COLOR_PRESETS: ReadonlyArray<{
	value: string;
	label: string;
}> = [
	{ value: "#222222", label: "深灰" },
	{ value: "#555555", label: "灰" },
	{ value: "#228b57", label: "綠" },
	{ value: "#c0392b", label: "紅" },
	{ value: "#d68910", label: "橙" },
	{ value: "#2874a6", label: "藍" },
];

const FONT_SIZE_VALUES = new Set(BLOCK_FONT_SIZES.map((o) => o.value));
const TEXT_ALIGN_VALUES = new Set(["left", "center", "right"]);
const HEX_COLOR = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

/**
 * Read `block.style` from whatever the backend handed back and produce a
 * React-safe CSS object. Drops unknown keys, rejects non-hex colors, and
 * snaps fontSize to one of the allowed steps so styling stays predictable
 * across editor → preview → PDP.
 */
export function sanitizeBlockStyle(input: unknown): BlockTextStyle {
	if (!input || typeof input !== "object") return {};
	const raw = input as Record<string, unknown>;
	const out: BlockTextStyle = {};
	if (typeof raw.fontSize === "string" && FONT_SIZE_VALUES.has(raw.fontSize)) {
		out.fontSize = raw.fontSize;
	}
	const weight = Number(raw.fontWeight);
	if (weight === 700) out.fontWeight = 700;
	if (raw.fontStyle === "italic") out.fontStyle = "italic";
	if (raw.textDecoration === "underline") out.textDecoration = "underline";
	if (
		typeof raw.textAlign === "string" &&
		TEXT_ALIGN_VALUES.has(raw.textAlign)
	) {
		out.textAlign = raw.textAlign as "left" | "center" | "right";
	}
	if (typeof raw.color === "string" && HEX_COLOR.test(raw.color)) {
		out.color = raw.color;
	}
	return out;
}

export interface ProductDescription {
	version: "1.0";
	blocks: ProductDescriptionBlock[];
}

/**
 * 送出用的描述文件：`imageUrl` 是後端由 `imageKey` 組出來的顯示值，回送它就是把一個
 * 衍生欄當成輸入。
 *
 * 放在型別的家而不是各呼叫端自己剝：代行的 body 走
 * `json.Decoder.DisallowUnknownFields()`（`admin_delegated_common.go:121`），多一個
 * `imageUrl` 就是 **9260／400，整張表單完全送不出去**，而漏剝的那一處不會有任何東西
 * 提醒你 —— 直到有人在該頁編輯了一張圖片。
 */
export function toDescriptionWireInput(description: ProductDescription): {
	version: string;
	blocks: ProductDescriptionBlock[];
} {
	return {
		version: description.version,
		blocks: description.blocks.map((block) => {
			const { imageUrl: _imageUrl, ...rest } = block;
			return rest;
		}),
	};
}

function newIdempotencyKey(): string {
	if (
		typeof crypto !== "undefined" &&
		typeof crypto.randomUUID === "function"
	) {
		return crypto.randomUUID();
	}
	// Fallback for ancient browsers — not cryptographically perfect, fine for
	// a request key that the server only uses for dedupe.
	return `key-${Date.now()}-${Math.random().toString(36).slice(2, 12)}`;
}

/**
 * Coerce whatever the backend returns into a ProductDescription. Older
 * listings may come back as a plain string; new ones are already structured.
 * Empty input returns an empty document so the editor can render.
 */
export function parseDescription(
	input: string | ProductDescription | null | undefined,
): ProductDescription {
	if (!input) return { version: "1.0", blocks: [] };
	if (typeof input === "string") {
		const text = input.trim();
		if (!text) return { version: "1.0", blocks: [] };
		// Backend used to send JSON-encoded descriptions before the type was
		// promoted on the wire; try to parse so we don't render literal JSON.
		if (text.startsWith("{") && text.includes("blocks")) {
			try {
				const parsed = JSON.parse(text) as ProductDescription;
				if (parsed?.version === "1.0" && Array.isArray(parsed.blocks)) {
					return parsed;
				}
			} catch {
				/* fall through to plain-text wrap */
			}
		}
		return {
			version: "1.0",
			blocks: [{ type: "text", content: text }],
		};
	}
	return {
		version: "1.0",
		blocks: Array.isArray(input.blocks) ? input.blocks : [],
	};
}

/**
 * Trim empty trailing blocks; keeps the doc tidy before submit so we don't
 * persist `{ blocks: [{ type: "text", content: "" }] }` rows.
 */
export function compactDescription(
	doc: ProductDescription,
): ProductDescription {
	return {
		version: "1.0",
		blocks: doc.blocks.filter((b) => {
			if (b.type === "text") {
				// inline-run 模型：runs 為主、legacy content 相容——兩者皆空才視為
				// 空 block。只看 content 會把所有 runs 段落當空丟掉（#1246 存檔掉字）。
				return Boolean(
					b.runs?.some((r) => r.text?.trim()) || b.content?.trim(),
				);
			}
			if (b.type === "image") {
				return Boolean(b.assetId || b.stagedToken || b.imageKey);
			}
			return true;
		}),
	};
}

/* ===== Categories ===== */

export interface CategoryNode {
	id: string | number;
	name: string;
	parentId?: string | number | null;
	parentName?: string;
	status?: string;
	children?: CategoryNode[];
}

/**
 * GET /app/categories/tree — full L1→L2→L3 tree in one request. Mobile uses
 * the same endpoint; safe to cache for the session.
 */
export async function fetchCategoryTree(): Promise<CategoryNode[]> {
	const env = await authedFetch<CategoryNode[]>("/app/categories/tree");
	return Array.isArray(env.data) ? env.data : [];
}

/**
 * GET /public/categories/tree — 同一棵樹的匿名版本。
 *
 * 代行工作區用這一支：那個瀏覽器在 www 上**沒有登入態**，`/app/categories/tree` 會
 * 401，而失敗形狀是「分類下拉恆為空 ⇒ 商品建不出來」而不是任何錯誤訊息。
 *
 * ⚠️ 它與 `/app` 那支不是同一份內容：`/app` 會帶上賣家自建的候選分類。所以快取鍵
 * 必須分開，兩者混在同一格會讓賣家中心的分類選單少掉自建分類。
 */
export async function fetchPublicCategoryTree(): Promise<CategoryNode[]> {
	const { data } =
		await apiClient.public.get<CategoryNode[]>("/categories/tree");
	return Array.isArray(data) ? data : [];
}

export interface CreateCategoryInput {
	parentId: string | number;
	name: string;
}

/**
 * Create an immediately-active seller category under an existing L1/L2 node.
 * The backend transparently returns an existing ACTIVE category when the same
 * parent already carries the same name, so callers can treat both outcomes as
 * a successful selection.
 */
export async function createCategory(
	input: CreateCategoryInput,
): Promise<CategoryNode> {
	const env = await authedFetch<CategoryNode>("/app/categories", {
		method: "POST",
		json: {
			parentId: String(input.parentId),
			name: input.name.trim(),
		},
	});
	const category = env.data;
	if (!category || category.id == null || !category.name) {
		throw new Error("分類服務回傳資料不完整");
	}
	return category;
}

/** Category nodes → `Dropdown` items. Ids are numbers on the wire, values are strings. */
export function categoryDropdownItems(
	nodes: readonly CategoryNode[],
): { value: string; label: string }[] {
	return nodes.map((node) => ({ value: String(node.id), label: node.name }));
}

/**
 * Walk the tree to find the L1/L2/L3 path that contains `targetId`.
 * Returns ["","",""] when nothing matches.
 *
 * Lives here rather than next to its first caller (`CategorySelector`) because
 * the delegated product pages need the same seeding without pulling that
 * component's authed `useCategoryTree` / `createCategory` graph into their
 * bundle — the delegated browser has no login session on www, so those calls
 * are 401s there.
 */
export function findCategoryPath(
	tree: CategoryNode[],
	targetId: number | string | undefined,
): [string, string, string] {
	if (targetId == null || targetId === "") return ["", "", ""];
	const target = String(targetId);
	for (const l1 of tree) {
		if (String(l1.id) === target) return [String(l1.id), "", ""];
		for (const l2 of l1.children ?? []) {
			if (String(l2.id) === target) {
				return [String(l1.id), String(l2.id), ""];
			}
			for (const l3 of l2.children ?? []) {
				if (String(l3.id) === target) {
					return [String(l1.id), String(l2.id), String(l3.id)];
				}
			}
		}
	}
	return ["", "", ""];
}

/**
 * Insert or replace a category in the cached tree without mutating the
 * previous query value. This lets the newly-created option become selectable
 * immediately while the server tree is marked stale for the next refresh.
 */
export function upsertCategoryNode(
	tree: CategoryNode[],
	parentId: string | number,
	category: CategoryNode,
): CategoryNode[] {
	const parentKey = String(parentId);
	let foundParent = false;

	const visit = (nodes: CategoryNode[]): CategoryNode[] =>
		nodes.map((node) => {
			if (String(node.id) === parentKey) {
				foundParent = true;
				const children = [...(node.children ?? [])];
				const existingIndex = children.findIndex(
					(child) => String(child.id) === String(category.id),
				);
				const normalized = {
					...category,
					parentId: category.parentId ?? parentId,
					children: category.children ?? [],
				};
				if (existingIndex >= 0) children[existingIndex] = normalized;
				else children.push(normalized);
				children.sort((a, b) =>
					String(a.name).localeCompare(String(b.name), "zh-Hant"),
				);
				return { ...node, children };
			}

			if (!node.children?.length) return node;
			const children = visit(node.children);
			return children === node.children ? node : { ...node, children };
		});

	const next = visit(tree);
	return foundParent ? next : tree;
}

/** Does `node` (or any descendant) carry `target` as its id? */
function categoryBranchHasId(node: CategoryNode, target: number): boolean {
	if (Number(node.id) === target) return true;
	for (const child of node.children ?? []) {
		if (categoryBranchHasId(child, target)) return true;
	}
	return false;
}

/**
 * True when `categoryId` sits inside a 加工食品-style L1 branch — the gate for
 * 「販售加工食品需先補食品業者登錄字號才能上架」. Matches by Chinese keyword on the
 * L1 node so renames lower in the tree don't break the gate. Pure (takes the
 * already-fetched tree) so it works both in the form hook and per-row on the
 * products list. Returns `false` while the tree is empty / id is unset.
 */
export function isProcessedFoodCategory(
	tree: CategoryNode[],
	categoryId: number | string | undefined,
): boolean {
	const target = Number(categoryId);
	if (!Number.isFinite(target) || tree.length === 0) return false;
	for (const l1 of tree) {
		if (categoryBranchHasId(l1, target)) {
			const name = (l1.name ?? "").toString();
			return name.includes("加工") || name.includes("食品");
		}
	}
	return false;
}

/**
 * Verify a seller-entered certification number against the 農業部 open-data
 * platform via `GET /app/products/certifications/verify`. Only the verifiable
 * types (3701 有機 / 3702 產銷履歷 / 3703 CAS) are supported — 3704–3706 return
 * 501. Returns `false` on any non-OK / not-found response so the form can show
 * 「查無此編號」without throwing.
 */
export async function verifyProductCertification(
	certNo: string,
	type: ProductCertificationCode,
): Promise<boolean> {
	const qs = new URLSearchParams({ cert_no: certNo, type });
	const env = await authedFetch<{ exists?: boolean }>(
		`/app/products/certifications/verify?${qs}`,
		{ throwOnHttpError: false },
	);
	return Boolean(env.data?.exists);
}

/* ===== 商品認證申請（19 號設計書 §9.1） ===== */

/**
 * 一筆認證申請 —— `GET /app/products/{id}/certification_applications` 的
 * `data.records[]` 元素（19 §9.1）。
 *
 * ⚠️ `id` / `productId` / `version` **一律是字串**：TSID 為 18 位十進位，超出
 * `Number.MAX_SAFE_INTEGER`；宣告成 number 會在解析時靜默掉精度，而撤回帶著錯的
 * `expectedVersion` 只會回 409，看起來像後端有問題。
 *
 * 🔴 賣家 DTO **不含** `internalNote` 與 `govVerifyPayload` 原文（19 §9.1）：賣家這一面
 * 只需要「狀態徽章 + 駁回原因」，多要一個欄位就是把 ADM 內部註記與政府原始資料外流。
 */
export interface ProductCertificationApplication {
	id: string;
	productId: string;
	/** `CertificationType` 值（"3701"…"3706"）。 */
	certificationType: string;
	certificateNo: string;
	/** `ProductCertificationStatus`（SUBMITTED / APPROVED / …）。 */
	status: string;
	/** `ProductCertificationGovVerifyState`。與 `status` 是兩件事，不可互相推導。 */
	govVerifyState?: string | null;
	govVerifiedAt?: string | null;
	submittedAt?: string | null;
	reviewedAt?: string | null;
	/** `ProductCertificationRejectReason`（駁回／撤銷才有）。 */
	reasonCode?: string | null;
	/** ADM 自由填寫、直接給賣家看的補充說明。 */
	publicReason?: string | null;
	/** 樂觀鎖版本；撤回必帶。 */
	version: string;
}

function mapWireCertificationApplication(
	raw: unknown,
): ProductCertificationApplication {
	const r = (raw ?? {}) as Record<string, unknown>;
	const str = (v: unknown): string =>
		typeof v === "string" ? v : typeof v === "number" ? String(v) : "";
	const nullable = (v: unknown): string | null => {
		const s = str(v);
		return s === "" ? null : s;
	};
	return {
		id: str(r.id),
		productId: str(r.productId),
		certificationType: str(r.certificationType).trim(),
		certificateNo: str(r.certificateNo),
		status: str(r.status).trim(),
		govVerifyState: nullable(r.govVerifyState),
		govVerifiedAt: nullable(r.govVerifiedAt),
		submittedAt: nullable(r.submittedAt),
		reviewedAt: nullable(r.reviewedAt),
		reasonCode: nullable(r.reasonCode),
		publicReason: nullable(r.publicReason),
		version: str(r.version),
	};
}

/**
 * 這個商品的全部認證申請。
 *
 * ⚠️ 這一支是**唯一**能看到「待審」與「被駁回」那些筆的地方：`product.certifications`
 * 自 19 號起只是 approved-only 公開投影，賣家剛送出的申請不在裡面。只讀商品而不讀這
 * 一支，賣家送完會以為沒存到（19 §14.2）。
 *
 * ⚠️ 分頁欄位（`total` / `page` / `pageSize`）在 wire 上是**字串**（`pkg/envelope.Page`
 * 的簽章就是 string），不要照數字解讀。這裡只取 `records`，所以不受影響。
 *
 * 端點不存在或無權時回空陣列而非丟例外：表單少了徽章仍可編輯，硬擋住整頁比較糟。
 */
export async function fetchProductCertificationApplications(
	productId: number | string,
): Promise<ProductCertificationApplication[]> {
	const env = await authedFetch<{ records?: unknown[] } | unknown[]>(
		`/app/products/${encodeURIComponent(String(productId))}/certification_applications`,
		// 兩個開關都要關：HTTP 狀態與 envelope 業務碼是**兩道獨立的丟出點**，只關前者
		// 的話，一個 `{"code":404}` 的 200 回應（Gateway 對未登錄業務碼會塌縮成這種形狀）
		// 仍然會丟例外，整張表單就被一個「只是少了徽章」的情況擋住。
		{ throwOnHttpError: false, throwOnEnvelopeError: false },
	);
	const raw = Array.isArray(env.data) ? env.data : (env.data?.records ?? []);
	return Array.isArray(raw) ? raw.map(mapWireCertificationApplication) : [];
}

/**
 * 送出一筆認證申請（19 §9.1 的 POST）。
 *
 * ⚠️ 單筆端點**不走剔除語意**：非法輸入回 typed 業務碼而不是靜默跳過（19 §6.2.10），
 * 所以這裡刻意讓錯誤往上丟，由呼叫端顯示訊息。吞掉它就回到「送出成功但清單沒有這筆」
 * ——那正是本設計書全篇在防的失效形狀。
 */
export async function submitProductCertificationApplication(
	productId: number | string,
	input: {
		certificationType: string;
		certificateNo: string;
		/** Reuse for a retry of the same application after an uncertain response. */
		idempotencyKey?: string;
	},
): Promise<ProductCertificationApplication | null> {
	const env = await authedFetch<unknown>(
		`/app/products/${encodeURIComponent(String(productId))}/certification_applications`,
		{
			method: "POST",
			json: {
				certificationType: input.certificationType,
				certificateNo: input.certificateNo,
				idempotencyKey: input.idempotencyKey ?? `cert-${newIdempotencyKey()}`,
			},
		},
	);
	return env.data ? mapWireCertificationApplication(env.data) : null;
}

/**
 * 撤回一筆認證申請（19 §9.1 的 withdraw）。
 *
 * 🔴 取消勾選必須走這一支，**不能靠「全量取代時不送」來移除**：全量取代的「沒送」是
 * 以 `certification_type` 為粒度（19 §6.4），而待審中的申請在那條路徑上是**保留**的
 * ——只把它從商品 payload 拿掉，賣家看到勾選消失、後端的申請卻還在佇列裡。
 */
export async function withdrawProductCertificationApplication(
	productId: number | string,
	applicationId: string,
	expectedVersion: string,
): Promise<void> {
	await authedFetch(
		`/app/products/${encodeURIComponent(String(productId))}/certification_applications/${encodeURIComponent(applicationId)}/withdraw`,
		{
			method: "POST",
			json: {
				expectedVersion,
				idempotencyKey: `cert-wd-${newIdempotencyKey()}`,
			},
		},
	);
}

/* ===== Shipping quotes ===== */

/** One carrier × 溫層 option the backend says fits a given package. Mirrors
 *  `handler.ShippingOptionItem` (camelCase, all-string). */
export interface ShippingQuoteOption {
	/** ShippingMethod value ("3001" 黑貓 / "3002" 7-11)。 */
	shippingMethod: string;
	/** ShippingTemperature value ("3201"/"3202"/"3203")。 */
	temperature: string;
	/** Carrier display name from the backend ("黑貓宅急便"…)。 */
	displayName: string;
	/** 運費（新台幣元字串）。 */
	fee: string;
	/** 預計到貨天數（字串）。 */
	estimatedDays: string;
}

export interface ShippingQuoteResult {
	availableTemperatures: Array<{ temperature: string; displayName?: string }>;
	shippingOptions: ShippingQuoteOption[];
}

export interface ShippingQuoteInput {
	length: string;
	width: string;
	height: string;
	weight: string;
	/** WeightUnit value ("3301" 公斤 / "3302" 台斤 / "3303" 公克) — matches the
	 *  seller app, which passes `spec.unitCode` (the value, not "KG"). */
	weightUnit: string;
}

/**
 * `POST /app/shipping_quotes` — given package dimensions + weight, the backend
 * returns the carrier × 溫層 combinations that fit (per-carrier size / weight
 * limits) plus their fees. Drives the product form's 可配送方案 picker exactly
 * like the seller app's spec screen. Throws with the backend message on a 4xx
 * (e.g. 4301「所有配送方式都超過尺寸或重量限制」) so the UI can surface it.
 */
export async function fetchShippingQuotes(
	input: ShippingQuoteInput,
): Promise<ShippingQuoteResult> {
	const env = await authedFetch<{
		availableTemperatures?: unknown[];
		shippingOptions?: unknown[];
	}>("/app/shipping_quotes", {
		method: "POST",
		json: input,
		throwOnHttpError: false,
	});
	const data = env.data as
		| { availableTemperatures?: unknown[]; shippingOptions?: unknown[] }
		| null
		| undefined;
	const temps = Array.isArray(data?.availableTemperatures)
		? data.availableTemperatures
		: [];
	const opts = Array.isArray(data?.shippingOptions) ? data.shippingOptions : [];
	if (temps.length === 0 && opts.length === 0) {
		// No options → either a real backend error (surface its message) or an
		// empty result for an out-of-range package.
		const msg = env.message ?? env.msg;
		if (msg) throw new Error(msg);
		return { availableTemperatures: [], shippingOptions: [] };
	}
	return {
		availableTemperatures: temps
			.map((t) => {
				const r = (t ?? {}) as Record<string, unknown>;
				return {
					temperature: String(r.temperature ?? ""),
					displayName: str(r.displayName) ?? str(r.label),
				};
			})
			.filter((t) => t.temperature),
		shippingOptions: opts
			.map((o) => {
				const r = (o ?? {}) as Record<string, unknown>;
				return {
					shippingMethod: String(r.shippingMethod ?? r.type ?? ""),
					temperature: String(r.temperature ?? ""),
					displayName: String(r.displayName ?? r.label ?? ""),
					fee: String(r.fee ?? ""),
					estimatedDays: String(r.estimatedDays ?? ""),
				};
			})
			.filter((o) => o.shippingMethod && o.temperature),
	};
}

/* ===== Product create / update ===== */

/** camelCase SKU exactly as `handler.CreateSkuInput` wants it on the wire. */
export interface SkuWriteInput {
	id?: string;
	skuName: string;
	price: string;
	salePrice?: string;
	/** Omitted only on the `includeStock:false` path (defer to 庫存管理);
	 *  the backend binds it `required` whenever a SKU is sent. */
	stockQty?: string;
	unit: string;
	weight?: string;
	lengthCm?: string;
	widthCm?: string;
	heightCm?: string;
	shippingMethods?: Array<{
		type: string;
		temperature: string;
		feePayer: string;
		freeThreshold: string;
	}>;
}

/**
 * camelCase product body matching the gateway's `handler.CreateProductRequest`
 * / `UpdateProductRequest`. The backend binds these with `binding:"required"`
 * on camelCase keys (`categoryId`, `skuName`, `unit`, `price`, `stockQty`), so
 * the form's snake_case draft must be serialised to this shape — see
 * `draftToSubmitBody`.
 */
export interface ProductWriteBody {
	idempotencyKey?: string;
	name: string;
	description?: ProductDescription;
	categoryId?: string;
	origin?: string;
	images?: ProductImageInput[];
	hashtags?: string[];
	certifications?: Array<{ certNo: string; name: string }>;
	skus?: SkuWriteInput[];
	status?: string | number;
	/** UpdateProductRequest only: replace hashtags wholesale when true. */
	updateHashtags?: boolean;
}

export const createProduct: SellerProductRepo["createProduct"] = (body, meta) =>
	sellerProductRepo.createProduct(body, meta);

export const updateProduct: SellerProductRepo["updateProduct"] = (
	id,
	body,
	meta,
) => sellerProductRepo.updateProduct(id, body, meta);

/** Target status codes accepted by `PUT /app/products/{id}/status`.
 * 4004 = 上架, 4003 = 下架. Backend allows 草稿→上架, 上架→下架, 下架→上架;
 * 審核中 (4002) cannot be toggled. */
export const PRODUCT_STATUS = { ON: "4004", OFF: "4003" } as const;

export const updateProductStatus: SellerProductRepo["updateProductStatus"] = (
	id,
	status,
	meta,
) => sellerProductRepo.updateProductStatus(id, status, meta);

/** Soft-delete a product. Backend only permits this for 草稿 (4001) or
 * 已下架 (4003) products. */
export const deleteProduct: SellerProductRepo["deleteProduct"] = (id) =>
	sellerProductRepo.deleteProduct(id);
