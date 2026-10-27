import {
	type DelegatedMutationMeta,
	delegatedFetch,
	type RepoFetcher,
} from "@/lib/api/delegated";
import { ossImageUrl } from "@/lib/media/cdn";
import {
	type ProductDescription,
	type ProductDescriptionBlock,
	toDescriptionWireInput,
} from "@/lib/products/seller-repo";

/**
 * 代行商品編輯的資料層（56 號 §6.1.8：`GET`／`PATCH .../products/{productId}`）。
 *
 * ⚠️ **不沿用 `lib/products/seller-repo.ts` 的 detail／update**（定案第 1 條 ② 的同一
 * 個判準）：兩份 DTO 不同形，而那支 normalizer 對讀不到的欄位一律回 `undefined`、
 * 不報錯。代行這一面上，半套映射的失敗形狀不是「畫面少一欄」，是
 * **請求完全合法、`expectedVersion` 相符、回 `code: 0`，而賣家的商品圖片或規格被清空**
 * （L3774／L3079）。所以這裡每一個欄位都自己讀、自己送。
 *
 * PATCH 的三條整份替換規則（照抄 Gateway `UpdateDelegatedProduct` 的實作，
 * `admin_delegated_product.go:455-520`）：
 * - **未送的欄位由 Gateway 以 before 快照回填**（`name`／`categoryId`／`origin`／
 *   `images`／`certifications`／每個 SKU 的未送欄位）⇒ 前端只送「真的改了的」欄位
 *   是安全的，而且是唯一能讓稽核 `changedFields` 誠實的送法。
 * - **`images` 有送就是整份替換**：要保留的原圖必須以 `{assetId}` 回送，而 `assetId`
 *   必須來自這一次 GET（Gateway 拿 intent 之前讀到的原始資源比對，不在其中即 9275）。
 * - **`skus` 只動有送的那幾筆**，刪除規格的唯一表達是 `removedSkuIds`（本期 UI 不做
 *   刪除，所以這裡連這個欄位都不提供 —— 送不出去的東西不該有型別）。
 */

/** 商品既有圖。`assetId` 是回送保留的唯一憑據，`imageKey` 只用來顯示。 */
export interface DelegatedProductImage {
	assetId: string;
	imageKey: string;
}

/** SKU 的配送設定。本期唯讀（代行 UI 不編輯它，未送即由 Gateway 回填原值）。 */
export interface DelegatedProductShippingMethod {
	type: string;
	temperature: string;
	freeThreshold: string;
	feePayer: string;
}

/**
 * 一個規格。
 *
 * ⚠️ 欄名是 `spec` 而下游 `ProductSku` 叫 `sku_name` —— 這一欄是整條鏈上最容易接錯
 * 的地方（Gateway DTO 註解明列）。前端一律只認 `spec`。
 */
export interface DelegatedProductSku {
	skuId: string;
	spec: string;
	price: string;
	salePrice: string;
	stockQty: string;
	unit: string;
	weight: string;
	lengthCm: string;
	widthCm: string;
	heightCm: string;
	shippingMethods: DelegatedProductShippingMethod[];
}

export interface DelegatedProductCertification {
	name: string;
	certNo: string;
}

export interface DelegatedProductDetail {
	productId: string;
	/** `expectedVersion` 的唯一來源。缺它每一次 PATCH 都是 9260。 */
	version: string;
	name: string;
	status: string;
	statusLabel: string;
	// 買家視角的兩個判定旗標（gateway 於 #2479 補上，上游 ProductPublicationPolicy 一律填）。
	// ⚠️ purchasable 用非可選的 boolean：預覽的 deriveDetailLayoutProps 判定是
	// `purchasable !== false`，欄位缺席會恆為 true ⇒ 草稿預覽顯示成「可以購買」。
	purchasable: boolean;
	publicationState: string;
	categoryId: string;
	categoryName: string;
	description: ProductDescription;
	images: DelegatedProductImage[];
	origin: string;
	certifications: DelegatedProductCertification[];
	hashtags: string[];
	skus: DelegatedProductSku[];
	/**
	 * 商品層的聚合值，**唯讀觀察欄位**：PATCH 一律忽略它們且不報錯，所以它們既不進
	 * 表單、也不得列進送出前的變更摘要（列進去等於展示一筆不會發生的變更，L3700）。
	 */
	priceMin: string;
	priceMax: string;
	stockQty: string;
	salesCount: string;
}

/** 圖片項：`stagedToken`（本次新上傳）與 `assetId`（保留既有）**二選一**。 */
export type DelegatedProductImageInput =
	| { stagedToken: string }
	| { assetId: string };

/** 只送改過的欄位；`skuId` 必填，其餘未送即保留原值。 */
export interface DelegatedProductSkuInput {
	skuId: string;
	spec?: string;
	price?: string;
	salePrice?: string;
	stockQty?: string;
	unit?: string;
	weight?: string;
	lengthCm?: string;
	widthCm?: string;
	heightCm?: string;
}

export interface DelegatedProductPatch {
	name?: string;
	categoryId?: string;
	origin?: string;
	hashtags?: string[];
	description?: ProductDescription;
	images?: DelegatedProductImageInput[];
	skus?: DelegatedProductSkuInput[];
	certifications?: DelegatedProductCertification[];
}

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function str(value: unknown): string {
	return typeof value === "string" ? value : "";
}

function strings(value: unknown): string[] {
	return Array.isArray(value) ? value.filter((v) => typeof v === "string") : [];
}

function readImage(value: unknown): DelegatedProductImage {
	const raw = isRecord(value) ? value : {};
	return { assetId: str(raw.assetId), imageKey: str(raw.imageKey) };
}

function readShippingMethod(value: unknown): DelegatedProductShippingMethod {
	const raw = isRecord(value) ? value : {};
	return {
		type: str(raw.type),
		temperature: str(raw.temperature),
		freeThreshold: str(raw.freeThreshold),
		feePayer: str(raw.feePayer),
	};
}

function readSku(value: unknown): DelegatedProductSku {
	const raw = isRecord(value) ? value : {};
	return {
		skuId: str(raw.skuId),
		spec: str(raw.spec),
		price: str(raw.price),
		salePrice: str(raw.salePrice),
		stockQty: str(raw.stockQty),
		unit: str(raw.unit),
		weight: str(raw.weight),
		lengthCm: str(raw.lengthCm),
		widthCm: str(raw.widthCm),
		heightCm: str(raw.heightCm),
		shippingMethods: Array.isArray(raw.shippingMethods)
			? raw.shippingMethods.map(readShippingMethod)
			: [],
	};
}

function readCertification(value: unknown): DelegatedProductCertification {
	const raw = isRecord(value) ? value : {};
	return { name: str(raw.name), certNo: str(raw.certNo) };
}

/**
 * 描述區塊。
 *
 * ⚠️ **`imageUrl` 必須在這裡補上**：代行 DTO 只回 `imageKey`（媒體欄位誠實命名，
 * §25），而編輯器的 `toEditorData` 對沒有可渲染 URL 的 image block **直接丟棄**
 * （`descriptionEditorJs.ts:355-357`）。少了這一步，含圖的商品描述進編輯器就少掉
 * 那幾塊，而 PATCH 的 description 是整份替換 ⇒ **一次儲存就把賣家描述裡的圖片刪掉**，
 * 請求合法、回 `code: 0`、沒有任何東西轉紅。URL 由前端唯一入口 `ossImageUrl` 組。
 */
function readDescriptionBlock(value: unknown): ProductDescriptionBlock {
	const raw = isRecord(value) ? value : {};
	const imageKey = str(raw.imageKey);
	const block: ProductDescriptionBlock = {
		type: (str(raw.type) || "text") as ProductDescriptionBlock["type"],
	};
	if (str(raw.id)) block.id = str(raw.id);
	if (Array.isArray(raw.runs))
		block.runs = raw.runs as ProductDescriptionBlock["runs"];
	if (str(raw.content)) block.content = str(raw.content);
	if (isRecord(raw.style)) block.style = raw.style;
	if (str(raw.tableJson)) block.tableJson = str(raw.tableJson);
	if (str(raw.listJson)) block.listJson = str(raw.listJson);
	if (str(raw.assetId)) block.assetId = str(raw.assetId);
	if (str(raw.alt)) block.alt = str(raw.alt);
	if (str(raw.width)) block.width = str(raw.width);
	if (str(raw.height)) block.height = str(raw.height);
	if (imageKey) {
		block.imageKey = imageKey;
		block.imageUrl = ossImageUrl(imageKey, "productDetail");
	}
	return block;
}

function readDescription(value: unknown): ProductDescription {
	const raw = isRecord(value) ? value : {};
	return {
		version: "1.0",
		blocks: Array.isArray(raw.blocks)
			? raw.blocks.map(readDescriptionBlock)
			: [],
	};
}

export function readDelegatedProductDetail(
	value: unknown,
): DelegatedProductDetail {
	const raw = isRecord(value) ? value : {};
	return {
		productId: str(raw.productId),
		version: str(raw.version),
		name: str(raw.name),
		status: str(raw.status),
		statusLabel: str(raw.statusLabel),
		// 預設 false 而不是 true：讀不到就當「不可購買」才是 fail-safe 的方向——
		// 預覽把不可買的說成可買，比反過來嚴重。
		purchasable: raw.purchasable === true,
		publicationState: str(raw.publicationState),
		categoryId: str(raw.categoryId),
		categoryName: str(raw.categoryName),
		description: readDescription(raw.description),
		images: Array.isArray(raw.images) ? raw.images.map(readImage) : [],
		origin: str(raw.origin),
		certifications: Array.isArray(raw.certifications)
			? raw.certifications.map(readCertification)
			: [],
		hashtags: strings(raw.hashtags),
		skus: Array.isArray(raw.skus) ? raw.skus.map(readSku) : [],
		priceMin: str(raw.priceMin),
		priceMax: str(raw.priceMax),
		stockQty: str(raw.stockQty),
		salesCount: str(raw.salesCount),
	};
}

/**
 * `Idempotency-Key`（§6.1.0）：8–128 字、`^[A-Za-z0-9._:-]+$`。每次送出產生新的一把；
 * 重送同一次送出要沿用同一把，那是呼叫端 catch 之後的事。
 */
function newIdempotencyKey(): string {
	const random =
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID().replace(/-/g, "")
			: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`;
	return `adm-${random}`;
}

export interface DelegatedProductEditRepo {
	detail(
		productId: string,
		opts?: { signal?: AbortSignal },
	): Promise<DelegatedProductDetail>;
	update(
		productId: string,
		patch: DelegatedProductPatch,
		meta: DelegatedMutationMeta,
	): Promise<void>;
}

export function createDelegatedProductEditRepo(
	storeId: string,
	fetcher: RepoFetcher = delegatedFetch,
): DelegatedProductEditRepo {
	const root = `/products/${encodeURIComponent(storeId)}`;
	return {
		async detail(productId, opts = {}) {
			const env = await fetcher<unknown>(
				`${root}/${encodeURIComponent(productId)}`,
				{ signal: opts.signal },
			);
			return readDelegatedProductDetail(env.data);
		},

		async update(productId, patch, meta) {
			const body: Record<string, unknown> = {
				expectedVersion: meta.expectedVersion,
				operationReason: meta.operationReason.trim(),
				// 不靠 DB DEFAULT 兜底：proto3 分不出「未設定」與 false。
				notifySeller: meta.notifySeller ?? true,
			};
			if (patch.name !== undefined) body.name = patch.name;
			if (patch.categoryId !== undefined) body.categoryId = patch.categoryId;
			if (patch.origin !== undefined) body.origin = patch.origin;
			if (patch.hashtags !== undefined) body.hashtags = patch.hashtags;
			if (patch.description !== undefined) {
				body.description = toDescriptionWireInput(patch.description);
			}
			if (patch.images !== undefined) body.images = patch.images;
			if (patch.skus !== undefined) body.skus = patch.skus;
			if (patch.certifications !== undefined) {
				body.certifications = patch.certifications;
			}
			await fetcher(`${root}/${encodeURIComponent(productId)}`, {
				method: "PATCH",
				json: body,
				idempotencyKey: newIdempotencyKey(),
			});
		},
	};
}
