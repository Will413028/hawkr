/**
 * 商品匯入精靈 — ImportItem ↔ ProductForm draft 雙向 adapter。
 *
 * 設計目標：把後端 `ImportItemDetail` 形狀（camelCase wire / brief wrapper /
 * shippingMethods 陣列）轉成既有 ProductForm 的 `SellerProductDetail` draft
 * 形狀（snake_case form / `shipping_methods: SkuShippingMethod[]`），讓
 * 既有 `ProductFormSkuEditor` / `DescriptionEditor` / `CategorySelector` /
 * `ProductImageUploader` 全部能 reuse。
 *
 * 純函式，無 React 依賴 — 與 `seller-center/products/adapters/productFormAdapter.ts`
 * 風格一致。
 *
 * Backend contract 對齊（v2，2026-06）：
 *  - `ImportItemDetailResponse.brief` 攜帶 name / category / cover / blocking / dup
 *  - `descriptionJson` 是 JSON 字串（內部結構為 ImportDescription）
 *  - `skus[].shippingMethods: [{ method, temperature }]`（取代舊的逐配送方式欄位）；送出一律帶 `AMBIENT_TEMPERATURE_CODE`
 *  - `galleryAssets[].sourceUrl / previewUrl` 取代舊 cdn_url
 */

import { coerceWeightUnitCode, WeightUnit } from "@/lib/codemeta/enums";
import type {
	ProductImageDraft,
	ProductSku,
	SellerProductDetail,
	SkuShippingMethod,
} from "@/lib/products/seller-repo";
import { AMBIENT_TEMPERATURE_CODE } from "@/lib/seller/ambient-temperature";
import {
	type ImportDescription,
	type ImportDescriptionBlock,
	type ImportItemAsset,
	type ImportItemDetail,
	type ImportItemImageInput,
	type ImportItemSku,
	type ShippingMethodInput,
	TARGET_STATUS,
	type TargetStatusValue,
	type UpdateImportItemInput,
	type UpdateImportSkuInput,
} from "../types";

/* ===== 小工具 ===== */

const toNum = (v: unknown): number | undefined => {
	if (v === null || v === undefined || v === "") return undefined;
	const n = Number(v);
	return Number.isFinite(n) ? n : undefined;
};

const trimOrEmpty = (v: unknown): string => {
	if (typeof v === "string") return v.trim();
	if (typeof v === "number") return String(v);
	return "";
};

/** Backend descriptionJson 解析（防呆：空字串 / 非法 JSON 一律回空 description）。 */
export function parseDescriptionJson(
	json: string | undefined,
): ImportDescription {
	const fallback: ImportDescription = { version: "1.0", blocks: [] };
	if (!json || typeof json !== "string") return fallback;
	try {
		const parsed = JSON.parse(json);
		if (!parsed || typeof parsed !== "object") return fallback;
		const blocks = Array.isArray(parsed.blocks)
			? (parsed.blocks as ImportDescriptionBlock[])
			: [];
		return {
			version: "1.0",
			sourceKind: parsed.sourceKind ?? parsed.source_kind,
			blocks,
		};
	} catch {
		return fallback;
	}
}

/* ===== Target status ↔ form status ===== */

/**
 * Form `status`（0=草稿 / 1=上架 / 2=下架）來自正式 PRODUCT_STATUS 模型。
 * Import 端多了 PENDING_REVIEW（送審），形式上不屬於 PRODUCT_STATUS — 我們仍把
 * 表單顯示用的 status 對應到 0/1（PENDING_REVIEW 視為「保持草稿，但目標送審」），
 * 真正 targetStatus 由 hook 額外 state 追蹤、`draftToImportUpdateInput` 寫回。
 */
export function targetStatusToFormStatus(ts: TargetStatusValue): 0 | 1 {
	if (ts === TARGET_STATUS.ACTIVE) return 1;
	return 0;
}

/** Form status → targetStatus：上架 → ACTIVE；草稿 → DRAFT。 */
export function formStatusToTargetStatus(
	status: string | number | undefined,
): TargetStatusValue {
	const n = typeof status === "string" ? Number(status) : status;
	return n === 1 ? TARGET_STATUS.ACTIVE : TARGET_STATUS.DRAFT;
}

/* ===== Description ↔ form description ===== */

/**
 * Import 端 `ImportDescription.blocks` 用 `{ type:"image", url }`（原站 URL）；
 * `ProductDescription.blocks`（DescriptionEditor 消化的形狀）image block
 * 用 `{ type:"image", imageKey, stagedToken? }`。轉換時 import url 對應到
 * editor `imageKey`（display 用），新上傳產的 stagedToken 在反向序列化時
 * 變成 import url。
 */
function importDescriptionToForm(desc: ImportDescription | undefined) {
	const blocks = (desc?.blocks ?? []).map((b) => {
		if (b.type === "image") {
			return {
				type: "image" as const,
				imageKey: b.url ?? "",
				stagedToken: undefined,
			};
		}
		if (b.type === "table") {
			return { type: "table" as const, tableJson: b.tableJson };
		}
		if (b.type === "list") {
			return { type: "list" as const, listJson: b.listJson };
		}
		// text：runs 為主（inline-run 模型）、legacy content 相容；styleJson 反解
		// 回 editor 的 style 物件（非法 JSON 靜默忽略，fallback 純文字顯示）。
		let style: Record<string, unknown> | undefined;
		if (b.styleJson) {
			try {
				const parsed = JSON.parse(b.styleJson);
				if (parsed && typeof parsed === "object") style = parsed;
			} catch {
				/* 樣式壞了不擋內容 */
			}
		}
		return {
			type: "text" as const,
			...(b.runs?.length ? { runs: b.runs } : {}),
			content: b.content ?? "",
			...(style ? { style } : {}),
		};
	});
	return { version: "1.0" as const, blocks };
}

/* ===== Gallery（ImportItemAsset → ProductImageDraft） ===== */

/**
 * 既有圖（CSV 下載完的 asset）以 `sourceUrl` 為 identity；本地新上傳走
 * stagedToken。ProductImageUploader 內部讀 `img.url` 顯示、`img.stagedToken`
 * 判斷新舊 — 我們把 import 端的 `sourceUrl` 放進 `ProductImageDraft.url`，
 * 顯示優先用 `previewUrl`（CDN 加速版，沒下載完才會沒值）。
 */
function galleryAssetToDraftImage(asset: ImportItemAsset): ProductImageDraft {
	return {
		url: asset.previewUrl || asset.sourceUrl,
		stagedToken: undefined,
	};
}

/* ===== ImportItemSku ↔ ProductSku ===== */

/** Import SKU → form ProductSku。 */
export function importSkuToDraft(sku: ImportItemSku): ProductSku {
	const methods: SkuShippingMethod[] = [
		...new Set((sku.shippingMethods ?? []).map((m) => m.method)),
	].map((type) => ({
		type,
		fee_payer: "3101", // ShippingFeePayer.BUYER（import 無此欄位，預設買家）
		free_threshold: "0",
	}));

	return {
		// SKU id 一律 string pass-through：backend TSID 18 位超出 JS Number 安全範圍 (2^53)，
		// Number() 解析會精度遺失導致 PUT 後 backend selectById miss → reconciliation 拒絕。
		// 新 SKU（form 慣例）以 `""` 表示，draftSkuToImportSku 不會送 skuId 給後端。
		id: sku.skuId ?? "",
		sku_name: sku.skuName ?? "",
		price: trimOrEmpty(sku.price) || "0",
		original_price: "",
		sale_price: trimOrEmpty(sku.salePrice) || "",
		stock: toNum(sku.stockQty) ?? 0,
		unit: coerceWeightUnitCode(sku.unit),
		weight: trimOrEmpty(sku.weight) || "0",
		length_cm: trimOrEmpty(sku.lengthCm) || "0",
		width_cm: trimOrEmpty(sku.widthCm) || "0",
		height_cm: trimOrEmpty(sku.heightCm) || "0",
		shipping_methods: methods.length ? methods : undefined,
	};
}

/** Form ProductSku → import UpdateImportSkuInput（PUT 用，set-pattern）。 */
export function draftSkuToImportSku(sku: ProductSku): UpdateImportSkuInput {
	const methods = sku.shipping_methods ?? [];
	const shippingMethods: ShippingMethodInput[] = methods.map((m) => ({
		method: m.type,
		temperature: AMBIENT_TEMPERATURE_CODE,
	}));

	const out: UpdateImportSkuInput = {
		skuName: sku.sku_name ?? "",
		price: trimOrEmpty(sku.price) || "0",
		salePrice: trimOrEmpty(sku.sale_price) || "",
		stockQty: sku.stock != null ? String(sku.stock) : "0",
		weight: trimOrEmpty(sku.weight) || "0",
		unit: coerceWeightUnitCode(sku.unit),
		lengthCm: trimOrEmpty(sku.length_cm) || "0",
		widthCm: trimOrEmpty(sku.width_cm) || "0",
		heightCm: trimOrEmpty(sku.height_cm) || "0",
		shippingMethods,
	};
	// skuId 只有在編輯既有 SKU 才有意義；新 SKU 為 `""` 不送回後端。
	if (sku.id) out.skuId = sku.id;
	return out;
}

/* ===== Main: ImportItemDetail → ProductForm draft ===== */

/**
 * 把 ImportItemDetail 轉成 ProductForm 用的 SellerProductDetail draft。
 *
 * 注意：detail 的多數欄位實際在 `brief` 子物件內（backend response 結構）。
 */
export function importItemToDraft(item: ImportItemDetail): SellerProductDetail {
	const brief = item.brief;
	const skus = (item.skus ?? []).map(importSkuToDraft);
	const images: ProductImageDraft[] = (item.galleryAssets ?? []).map(
		galleryAssetToDraftImage,
	);
	const desc = parseDescriptionJson(item.descriptionJson);
	return {
		id: 0,
		name: brief?.name ?? "",
		description: importDescriptionToForm(desc),
		main_image: images[0]?.url ?? "",
		images,
		// brief.categoryId 為編輯頁 form load 時的 SSOT（backend issue #1043）；
		// 賣家不主動觸碰分類選擇器時 draftToImportUpdateInput 也能拿到原值，
		// 送 categoryIdSet=true + categoryId="<原 id>" 即可走 validateLeafCategory。
		category_id:
			brief?.categoryId != null && brief.categoryId !== ""
				? Number(brief.categoryId)
				: undefined,
		category_name: brief?.catL3Name,
		tags: Array.isArray(item.hashtags) ? [...item.hashtags] : [],
		origin: item.origin ?? "",
		preorder_ship_at: null,
		low_stock_threshold: 5,
		expiry_days: 7,
		max_per_order: 0,
		show_sale_countdown: false,
		status: targetStatusToFormStatus(item.targetStatus),
		skus: skus.length
			? skus
			: [
					{
						id: "",
						sku_name: "",
						price: "0",
						original_price: "",
						sale_price: "",
						stock: 0,
						unit: WeightUnit.KG,
					},
				],
	};
}

/* ===== Main: draft → ImportUpdateInput ===== */

export interface DraftToImportInputOptions {
	/** 對應 hasDuplicateExisting 場景的 checkbox 狀態。 */
	allowDuplicateName?: boolean;
	/** form status 沒有 PENDING_REVIEW；hook 端獨立追蹤、傳進來覆寫。 */
	overrideTargetStatus?: TargetStatusValue;
	/** ItemEditForm 自掌的 category id（brief 不攜帶 categoryId）。 */
	overrideCategoryId?: string;
}

/**
 * 把 form draft 轉成 PUT /items/{id} 的 set-pattern 輸入。
 *
 * **per-field dirty-tracking**（與 backend 設計書 §4.2.8 set-pattern 語意對齊）：
 * - 只對使用者實際改過的欄位送 `*Set: true`
 * - 未改的欄位 `*Set: false` + 值省略，避免後端做不必要的 RPC 重驗證 / asset diff /
 *   SKU reconcile（即使後端有 content-diff guard，不送也是更乾淨）
 * - dirty 判定：把 original / draft 兩側 normalize 成同一 canonical form 後 JSON 比對
 * - duplicate 場景才帶 `allowDuplicateNameSet` + 值
 * - 必帶 `expectedVersion`（樂觀鎖，從 `original.brief.version` 取得；禁止用 commitRevision，那是 commit 嘗試編號，語意不同會撞 409。詳 backend issue #1037）
 */
export function draftToImportUpdateInput(
	draft: SellerProductDetail,
	original: ImportItemDetail,
	options: DraftToImportInputOptions = {},
): UpdateImportItemInput {
	const targetStatus =
		options.overrideTargetStatus ?? formStatusToTargetStatus(draft.status);

	// description：DescriptionEditor 寫回 `{ version, blocks }`；image block
	// 在 form 用 `{ type:"image", imageKey, stagedToken? }`。reverse 回 import 的
	// `{ type:"image", url }`，url 取 imageKey（既有圖的 CDN URL）。
	// text 帶 runs（inline-run 模型，backend descriptionJson 原樣存入）、table/list
	// 透傳 JSON 字串——與正式商品表單同保真度（#1246：早期只送 content 會把
	// runs 格式與表格/清單整批丟掉）。
	const descriptionBlocks =
		draft.description &&
		typeof draft.description === "object" &&
		Array.isArray(draft.description.blocks)
			? draft.description.blocks
			: [];
	const description: ImportDescription = {
		version: "1.0",
		sourceKind: "text",
		blocks: descriptionBlocks
			.map((b): ImportDescriptionBlock | null => {
				if (b.type === "text") {
					const hasRuns = b.runs?.some((r) => r.text?.trim());
					if (!hasRuns && !b.content?.trim()) return null;
					return {
						type: "text",
						...(b.runs?.length ? { runs: b.runs } : {}),
						...(b.content ? { content: b.content } : {}),
						...(b.style ? { styleJson: JSON.stringify(b.style) } : {}),
					};
				}
				if (b.type === "table") {
					return b.tableJson ? { type: "table", tableJson: b.tableJson } : null;
				}
				if (b.type === "list") {
					return b.listJson ? { type: "list", listJson: b.listJson } : null;
				}
				if (b.type === "image") {
					const url = b.imageKey ?? "";
					if (!url) return null;
					return { type: "image", url };
				}
				return null;
			})
			.filter((b): b is ImportDescriptionBlock => b !== null),
	};

	// gallery：依 stagedToken 有無分流。
	// 既有圖（無 stagedToken）的 sourceUrl 必須回送 backend 原始 sourceUrl 作 identity
	// （40 §5.5 前端 gallery payload identity 契約，issue #1053）；
	// draft.images[i].url 在 galleryAssetToDraftImage 時被設為 previewUrl||sourceUrl 以加速顯示，
	// 直接送 url 會讓 backend content-key sha256 不命中既有 asset → 誤判全新圖 → status 退回 ASSET_DOWNLOADING。
	// 反查策略：先用 url 比對 original.galleryAssets 的 previewUrl 或 sourceUrl，找到對應 asset 取其 sourceUrl；
	// 找不到（極少數，例如 image 順序錯亂）fallback 用 url 本身，由 backend noop 深度防禦兜底。
	const originalAssets = original.galleryAssets ?? [];
	const resolveSourceUrl = (url: string): string => {
		const hit = originalAssets.find(
			(a) => a.previewUrl === url || a.sourceUrl === url,
		);
		return hit?.sourceUrl ?? url;
	};
	const galleryImages: ImportItemImageInput[] = (draft.images ?? []).map(
		(img) => {
			if (img.stagedToken) {
				return { stagedToken: img.stagedToken };
			}
			return { sourceUrl: resolveSourceUrl(img.url) };
		},
	);

	const categoryId =
		options.overrideCategoryId ??
		(draft.category_id != null ? String(draft.category_id) : undefined);

	const newName = (draft.name ?? "").trim();
	const newOrigin = draft.origin ?? "";
	const newHashtags = draft.tags ?? [];
	const newDescStr = JSON.stringify(description);
	const newSkus = (draft.skus ?? []).map(draftSkuToImportSku);

	// dirty 判定
	const nameDirty = newName !== (original.brief?.name ?? "").trim();
	const categoryIdDirty =
		(categoryId ?? "") !== (original.brief?.categoryId ?? "");
	const originDirty = newOrigin !== (original.origin ?? "");
	const targetStatusDirty = targetStatus !== original.targetStatus;
	const hashtagsDirty = !arrayEquals(newHashtags, original.hashtags ?? []);
	const descriptionDirty =
		newDescStr !== normalizeDescJson(original.descriptionJson);
	const galleryDirty = !galleryEqual(galleryImages, original.galleryAssets);
	const skusDirty = !skusEqual(newSkus, original.skus);

	const input: UpdateImportItemInput = {
		expectedVersion: original.brief.version ?? "0",
		nameSet: nameDirty,
		categoryIdSet: categoryIdDirty,
		originSet: originDirty,
		targetStatusSet: targetStatusDirty,
		hashtagsSet: hashtagsDirty,
		descriptionSet: descriptionDirty,
		galleryImagesSet: galleryDirty,
		skusSet: skusDirty,
	};

	if (nameDirty) input.name = newName;
	if (categoryIdDirty) input.categoryId = categoryId;
	if (originDirty) input.origin = newOrigin;
	if (targetStatusDirty) input.targetStatus = targetStatus;
	if (hashtagsDirty) input.hashtags = newHashtags;
	if (descriptionDirty) input.description = newDescStr;
	if (galleryDirty) input.galleryImages = galleryImages;
	if (skusDirty) input.skus = newSkus;

	if (original.brief?.hasDuplicateExisting) {
		input.allowDuplicateNameSet = true;
		input.allowDuplicateName = options.allowDuplicateName ?? false;
	}

	return input;
}

/* ===== dirty-tracking helpers ===== */

function arrayEquals(a: readonly string[], b: readonly string[]): boolean {
	if (a.length !== b.length) return false;
	for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
	return true;
}

function normalizeDescJson(json: string | undefined): string {
	// 把 original.descriptionJson 重序列化成與 draftToImport 出來相同的 canonical form，避免空白 / key 順序差異誤判 dirty。
	const parsed = parseDescriptionJson(json);
	return JSON.stringify({
		version: "1.0",
		sourceKind: "text",
		blocks: parsed.blocks,
	});
}

function galleryEqual(
	input: readonly ImportItemImageInput[],
	existing: readonly ImportItemAsset[] | undefined,
): boolean {
	const ex = existing ?? [];
	if (input.length !== ex.length) return false;
	// 對齊 backend content-key（§5.5 階段 3.6）：STAGED 用 stagedToken / URL 用 sourceUrl
	// 既有 asset 沒有 stagedToken（已落地為 URL），所以這邊只能用 sourceUrl 比對；
	// 若 input 中有 stagedToken 表示新上傳 → 一定 dirty。
	for (let i = 0; i < input.length; i++) {
		const inp = input[i];
		const e = ex[i];
		if (inp.stagedToken) return false; // 新上傳必定 dirty
		if ((inp.sourceUrl ?? "") !== (e.sourceUrl ?? "")) return false;
	}
	return true;
}

function skusEqual(
	input: readonly UpdateImportSkuInput[],
	existing: readonly ImportItemSku[] | undefined,
): boolean {
	const ex = existing ?? [];
	if (input.length !== ex.length) return false;
	const canon = (s: UpdateImportSkuInput) =>
		JSON.stringify({
			id: s.skuId ?? "",
			n: s.skuName ?? "",
			p: s.price ?? "",
			sp: s.salePrice ?? "",
			q: s.stockQty ?? "",
			w: s.weight ?? "",
			u: s.unit ?? "",
			l: s.lengthCm ?? "",
			wi: s.widthCm ?? "",
			h: s.heightCm ?? "",
			sm: [...new Set((s.shippingMethods ?? []).map((m) => m.method))].sort(),
		});
	const canonEx = (s: ImportItemSku) =>
		JSON.stringify({
			id: s.skuId ?? "",
			n: s.skuName ?? "",
			p: s.price ?? "",
			sp: s.salePrice ?? "",
			q: String(s.stockQty ?? ""),
			w: s.weight ?? "",
			u: s.unit ?? "",
			l: s.lengthCm ?? "",
			wi: s.widthCm ?? "",
			h: s.heightCm ?? "",
			sm: [...new Set((s.shippingMethods ?? []).map((m) => m.method))].sort(),
		});
	for (let i = 0; i < input.length; i++) {
		if (canon(input[i]) !== canonEx(ex[i])) return false;
	}
	return true;
}
