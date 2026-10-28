/**
 * Adapters between the seller-product API shape and the form draft.
 *
 *   - emptyDraft()           initial draft for `mode === "create"`
 *   - applyAiPatch()         merge an AI assistant draft_patch into a draft
 *   - draftToCreateBody()    strip empty trailing description blocks before
 *                            POST /app/products
 *
 * Pure functions only — no React, no hooks. Keeps `useProductForm` lean.
 */

import {
	resolveShelfState,
	resolveStatusCode,
} from "@/features/seller-center/components/ShelfStatusControl";
import {
	coerceWeightUnitCode,
	ShippingFeePayer,
	WeightUnit,
} from "@/lib/codemeta/enums";
import { resolveCdnUrl } from "@/lib/media/cdn";
import {
	compactDescription,
	type ImageGalleryUpdate,
	type ProductDescription,
	type ProductImageDraft,
	type ProductImageInput,
	type ProductSku,
	type ProductWriteBody,
	parseDescription,
	type SellerProductDetail,
	type SkuWriteInput,
} from "@/lib/products/seller-repo";
import type { AiDraftPatch } from "@/lib/seller/ai-listing-chat";

const toNum = (v: unknown): number | undefined => {
	if (v === null || v === undefined || v === "") return undefined;
	const n = Number(v);
	return Number.isFinite(n) ? n : undefined;
};
const toStr = (v: unknown): string | undefined =>
	typeof v === "string" ? v : typeof v === "number" ? String(v) : undefined;

/**
 * Detail/edit SKUs wire camelCase (`skuName`/`stockQty`/`salePrice`) while the
 * form draft is snake_case — bridge the fields the SKU editor reads so 規格名稱 /
 * 庫存 / 售價 / 單位 hydrate on edit. The detail endpoint omits the sale window
 * and original price, so those stay undefined.
 */
function normaliseSkuFromApi(raw: unknown): ProductSku {
	const r = (raw ?? {}) as Record<string, unknown>;
	// Shipping methods (handler.ShippingMethodResponse) wire camelCase
	// `{ type, feePayer, freeThreshold }`; map onto the draft's snake_case
	// shape so the 可配送方案 picker re-hydrates on edit.
	const rawShipping = Array.isArray(r.shippingMethods) ? r.shippingMethods : [];
	const shipping_methods = rawShipping
		.map((m) => {
			const e = (m ?? {}) as Record<string, unknown>;
			return {
				type: toStr(e.type) ?? "",
				fee_payer: toStr(e.feePayer) ?? "",
				free_threshold: toStr(e.freeThreshold) ?? "0",
			};
		})
		.filter(
			(m, i, all) => m.type && all.findIndex((o) => o.type === m.type) === i,
		);
	// wire 端一律 camelCase（`handler.SellerSkuResponse`，seller_sku.go:22-33，
	// dev 實測 `GET /app/products/{id}` 的 skus[] key 集合完全吻合）。左手邊的
	// snake 是本地草稿型別 `ProductSku` 自己的形狀，與 wire 無關。
	return {
		// SKU id 一律 string pass-through（backend TSID 18 位超出 JS Number 安全範圍）
		id: toStr(r.id) ?? "",
		sku_name: toStr(r.skuName),
		spec_name: toStr(r.specName),
		price: toStr(r.price),
		original_price: toStr(r.originalPrice),
		sale_price: toStr(r.salePrice),
		stock: toNum(r.stockQty),
		unit: toStr(r.unit),
		weight: toStr(r.weight),
		length_cm: toStr(r.lengthCm),
		width_cm: toStr(r.widthCm),
		height_cm: toStr(r.heightCm),
		shipping_methods: shipping_methods.length ? shipping_methods : undefined,
	};
}

export function emptyDraft(): SellerProductDetail {
	return {
		id: 0,
		name: "",
		description: { version: "1.0", blocks: [] } satisfies ProductDescription,
		main_image: "",
		images: [],
		tags: [],
		origin: "",
		preorder_ship_at: null,
		low_stock_threshold: 5,
		expiry_days: 7,
		max_per_order: 0,
		show_sale_countdown: false,
		status: 0,
		skus: [
			{
				id: "",
				// Start empty so the input shows its placeholder and sellers can
				// type without clearing seeded text. draftToSubmitBody backfills
				// "預設規格" on save when a lone SKU is left unnamed.
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

/**
 * Resolve an image-gallery update (replacement array or functional updater)
 * against the current draft and recompute the derived `main_image`. The
 * functional form is what lets concurrent uploads accumulate: each resolves
 * against the freshest gallery rather than a stale closure snapshot, so a
 * multi-select keeps every image instead of last-write-wins collapsing to one.
 */
export function applyImagesUpdate(
	prev: SellerProductDetail,
	next: ImageGalleryUpdate,
): SellerProductDetail {
	const images = typeof next === "function" ? next(prev.images ?? []) : next;
	return {
		...prev,
		images,
		// `||` 不是 `??`：只帶 assetId 而組不出 URL 的項目 url 是空字串，
		// `??` 會讓它通過並把主圖清成 ""。同檔 normaliseDraftFromApi 的主圖 fallback 同此。
		main_image: images[0]?.url || prev.main_image,
	};
}

/**
 * Apply an AI draft_patch event onto an existing draft.
 *   - scalar fields (name / origin / categoryId / description) overwrite
 *   - array fields (tags / skus) replace entirely
 * Mirrors mobile's `mergePatch()` semantics.
 */
export function applyAiPatch(
	prev: SellerProductDetail,
	patch: AiDraftPatch,
): SellerProductDetail {
	const next: SellerProductDetail = { ...prev };
	if (patch.name) next.name = patch.name;
	if (patch.origin) next.origin = patch.origin;
	if (patch.categoryId != null) {
		const num = Number(patch.categoryId);
		if (Number.isFinite(num)) next.category_id = num;
	}
	if (patch.description) {
		next.description = parseDescription(patch.description);
	}
	if (Array.isArray(patch.tags)) next.tags = [...patch.tags];
	if (Array.isArray(patch.skus)) {
		next.skus = patch.skus.map((s, i) => ({
			id: "",
			sku_name: s.skuName ?? `規格 ${i + 1}`,
			price: s.price != null ? String(s.price) : "0",
			original_price: "",
			sale_price: "",
			stock: s.stockQty != null ? Number(s.stockQty) : 0,
			unit: s.unit ?? WeightUnit.KG,
			weight: s.weight != null ? String(s.weight) : undefined,
		}));
	}
	return next;
}

/**
 * Build the POST/PUT body. The structured description compaction strips
 * empty trailing blocks so we don't persist `[{type:"text",content:""}]`
 * which the renderer would treat as "I had a description, it was empty".
 */
export function draftToSubmitBody(
	draft: SellerProductDetail,
	statusOverride?: number,
	{
		includeStock = true,
		images = [],
	}: { includeStock?: boolean; images?: ProductImageInput[] } = {},
) {
	const status = resolveStatusCode(statusOverride ?? draft.status ?? 0);
	const description = compactDescription(parseDescription(draft.description));
	const multiSku = (draft.skus?.length ?? 0) > 1;
	// Serialise each SKU to the gateway's camelCase `handler.CreateSkuInput`.
	// The backend binds `skuName` / `unit` / `price` / `stockQty` with
	// binding:"required" on camelCase keys, so the form's snake_case draft must
	// be translated here — sending snake_case fails binding and the create 400s.
	const skus = (draft.skus ?? []).map((sku, i) => {
		// The 規格名稱 field starts empty (placeholder-only) so sellers can type
		// directly; backfill a sensible name here rather than persisting "".
		const skuName =
			sku.sku_name?.trim() || (multiSku ? `規格 ${i + 1}` : "預設規格");
		const out: SkuWriteInput = {
			skuName,
			price: String(Number(sku.price ?? 0) || 0),
			unit: coerceWeightUnitCode(sku.unit),
		};
		if (sku.id) out.id = sku.id;
		// PayUni #758: only send salePrice when it's a valid discount (0/empty
		// = clear); sending it otherwise would pin the price as a "sale".
		const sale = Number(sku.sale_price ?? 0);
		if (sale > 0 && sale < Number(sku.price ?? 0)) out.salePrice = String(sale);
		if (includeStock) out.stockQty = String(Number(sku.stock ?? 0) || 0);
		const weight = (sku.weight ?? "").trim();
		if (weight) out.weight = weight;
		// 包裝尺寸 — only send positive dimensions (mirrors the seller app).
		const dim = (v?: string) => {
			const n = Number(v);
			return Number.isFinite(n) && n > 0 ? String(n) : undefined;
		};
		const lengthCm = dim(sku.length_cm);
		const widthCm = dim(sku.width_cm);
		const heightCm = dim(sku.height_cm);
		if (lengthCm) out.lengthCm = lengthCm;
		if (widthCm) out.widthCm = widthCm;
		if (heightCm) out.heightCm = heightCm;
		// 可配送方案 → camelCase `handler.CreateShippingMethodInput`.
		const shipping = (sku.shipping_methods ?? []).filter((m) => m.type);
		if (shipping.length) {
			out.shippingMethods = shipping.map((m) => {
				const feePayer =
					m.fee_payer === ShippingFeePayer.SELLER
						? ShippingFeePayer.SELLER
						: ShippingFeePayer.BUYER;
				return {
					type: m.type,
					feePayer,
					// A threshold is a seller's subsidy condition, never a
					// buyer-paid shipping rule. Normalise legacy contradictory
					// drafts at the boundary before they reach the API.
					freeThreshold:
						feePayer === ShippingFeePayer.SELLER
							? m.free_threshold || "0"
							: "0",
				};
			});
		}
		return out;
	});
	const body: ProductWriteBody = {
		name: draft.name,
		description,
		...(images.length ? { images } : {}),
		categoryId:
			draft.category_id != null ? String(draft.category_id) : undefined,
		// Canonical hashtag field. The backend's product endpoint (both public
		// read and seller write) keys on `hashtags` (matches the Flutter seller's
		// `ProductUploadService` body and the public list response).
		hashtags: draft.tags ?? [],
		origin: draft.origin,
		skus,
		status,
	};
	return body;
}

/** Reconcile an incoming product response into the form's canonical shape. */
export function normaliseDraftFromApi(
	raw: SellerProductDetail & Record<string, unknown>,
): SellerProductDetail {
	// Hashtags: backend canonical key is `hashtags` (matches Flutter
	// `product_upload_service.dart` write and `/public/products` read);
	// legacy `tags` is the form's old local field name. Accept either
	// and store under the form draft's `tags` slot.
	const rawHashtags =
		(raw as Record<string, unknown>).hashtags ??
		(raw as Record<string, unknown>).tags;
	const hashtags: string[] = Array.isArray(rawHashtags)
		? rawHashtags
				.map((t) => {
					if (typeof t === "string") return t;
					if (t && typeof t === "object") {
						const rec = t as Record<string, unknown>;
						const name = rec.name ?? rec.label ?? rec.tag ?? rec.value;
						return typeof name === "string" ? name : "";
					}
					return "";
				})
				.map((s) => s.trim())
				.filter(Boolean)
		: [];

	// 商品分類：detail wire is camelCase (`categoryId` + `categoryLv1/2/3Id`) but
	// the form draft + CategorySelector key on snake_case numeric `category_id`.
	// Without this bridge the cascade never pre-fills on edit. Prefer the most
	// specific (leaf) id available so `findPath` resolves the full L1→L2→L3 path.
	const r = raw as Record<string, unknown>;
	const category_id =
		toNum(raw.category_id) ??
		toNum(r.categoryId) ??
		toNum(r.categoryLv3Id) ??
		toNum(r.categoryLv2Id) ??
		toNum(r.categoryLv1Id);

	// SKUs also wire camelCase on the detail endpoint — normalise each so the
	// SKU editor hydrates (規格名稱 / 庫存 / 售價 / 單位) instead of showing
	// blanks + 庫存 0.
	const skus = Array.isArray(raw.skus)
		? raw.skus.map(normaliseSkuFromApi)
		: raw.skus;

	// Gallery images live under `gallery` — `handler.GalleryItem[]`, i.e.
	// `{ assetId, imageKey, imageUrl }` (gateway `seller_product.go:22-26`;
	// 圖片欄自 backend#2235 起統一叫 imageKey/imageUrl，49 §6.1). `assetId`
	// is the load-bearing field: submit sends it back verbatim and the backend
	// keeps that image (`ProductService#processImageUpdate`), so an entry without
	// one cannot round-trip. Keep an entry whenever EITHER identity survives —
	// dropping a displayable-but-unkeyed image would delete it on the next save,
	// which is exactly the failure this mapping exists to prevent.
	const rawGallery = Array.isArray(r.gallery) ? (r.gallery as unknown[]) : [];
	const images: ProductImageDraft[] = rawGallery
		.map((g): ProductImageDraft | null => {
			if (!g || typeof g !== "object") return null;
			const rec = g as Record<string, unknown>;
			// imageKey is the objectKey SSOT (49 §6.1); build the display URL from
			// it rather than trusting the response's pre-built `/i/original/` URL,
			// so the production media boundary in `buildImageUrl` still applies.
			const url = resolveCdnUrl(toStr(rec.imageKey));
			const assetId = toStr(rec.assetId);
			if (!url && !assetId) return null;
			return assetId ? { url, assetId } : { url };
		})
		.filter((x): x is ProductImageDraft => x !== null);
	if (rawGallery.length > 0 && images.length === 0) {
		// Source had entries but none mapped — an unexpected wire shape. Surface
		// it instead of silently dropping the seller's whole gallery on edit.
		console.warn(
			"[normaliseDraftFromApi] dropped all gallery images; unexpected shape:",
			rawGallery,
		);
	}

	// 後端 status 是 CodeMeta 字串（4001…），但表單 radio 用 0/1/2（草稿/上架/
	// 下架）做數字比對。收斂成數字，否則 radio 預選對不上、開編輯頁全落到草稿。
	const shelf = resolveShelfState(raw.status);
	const status = shelf === "active" ? 1 : shelf === "off" ? 2 : 0;

	return {
		...raw,
		status,
		category_id,
		skus,
		tags: hashtags,
		images,
		// 主圖 = gallery 第一張；後端另有權威的 `imageKey`（主圖 objectKey，49 §6.1）
		// 當 gallery 缺圖或該筆組不出 URL 時的來源。
		main_image: images[0]?.url || resolveCdnUrl(toStr(r.imageKey)),
	};
}

/** `> 0` for a form field that arrives as a (possibly empty / non-numeric) string. */
function isPositive(value: unknown): boolean {
	const n = Number(value);
	return Number.isFinite(n) && n > 0;
}

/**
 * Everything wrong with this draft, in the order the seller reads the form.
 * An empty array means the draft will be accepted by the backend.
 *
 * Returning ALL of them (rather than the first) is the point: the backend
 * validates one rule at a time and returns on the first failure, so a seller
 * who left the 包裝尺寸 empty used to submit, get 「商品重量 必須大於 0」, fill the
 * weight, submit, get 「包裝長度 必須大於 0」, and so on — four round trips for one
 * missing section (#435). The form knows all four up front.
 *
 * Each rule below mirrors a real backend rejection; the citation is where that
 * rejection lives, so a divergence is traceable rather than folklore.
 * `tests/seller-product-required-fields.test.ts` pins the list.
 *
 *   name 非空             gateway seller_product.go:180 (binding:"required")
 *   categoryId 非空       gateway seller_product.go:181 (binding:"required")
 *   categoryId 為葉節點    ProductService.java:225 (L3 才解得出 lv2/lv1)
 *   unit ∈ WeightUnit     ProductService.java:253
 *   price > 0             ProductService.java:255
 *   salePrice > 0         ProductService.java:260
 *   salePrice < price     ProductService.java:263
 *   stockQty >= 0         ProductService.java:267
 *   weight > 0            ProductService.java:270
 *   lengthCm > 0          ProductService.java:273
 *   widthCm > 0           ProductService.java:276
 *   heightCm > 0          ProductService.java:279
 *
 * The same twelve run again on update (ProductService.java:444-504), so the
 * rules are mode-independent and this function needs no `mode` argument.
 *
 * `unit` has no rule here on purpose: `draftToSubmitBody` sends
 * `coerceWeightUnitCode(sku.unit)`, which cannot produce anything outside
 * WeightUnit, so the backend rule is unreachable from this form.
 * `idempotencyKey` likewise — `createProduct`/`updateProduct` mint one.
 */
export function validateDraft(draft: SellerProductDetail): string[] {
	const problems: string[] = [];

	if (!draft.name.trim()) problems.push("商品名稱必填");
	// CategorySelector emits `undefined` until the seller reaches a leaf (L3),
	// so "has a value" and "is a leaf" are the same check on this form.
	if (draft.category_id == null) {
		problems.push("請選擇商品分類（需選到最後一層）");
	}

	const skus = draft.skus ?? [];
	if (skus.length === 0) {
		problems.push("至少要一個規格 (SKU)");
		return problems;
	}

	// A lone SKU may stay nameless — draftToSubmitBody backfills "預設規格".
	// With multiple SKUs the name disambiguates them, so it's required, and
	// every message needs to say which SKU it is about.
	const multiSku = skus.length > 1;
	skus.forEach((sku, i) => {
		const named = sku.sku_name?.trim();
		const where = multiSku ? `規格「${named || `第 ${i + 1} 個`}」的` : "";

		if (multiSku && !named) problems.push(`第 ${i + 1} 個規格需要名稱`);
		if (!isPositive(sku.price)) problems.push(`${where}原價需大於 0`);
		if (sku.sale_price) {
			if (!isPositive(sku.sale_price)) {
				problems.push(`${where}售價需大於 0`);
			} else if (Number(sku.sale_price) >= Number(sku.price ?? 0)) {
				problems.push(`${where}售價需小於原價`);
			}
		}
		if (Number(sku.stock ?? 0) < 0) problems.push(`${where}庫存不可為負`);
		// 包裝尺寸 + 重量 live behind the 可配送方案 disclosure, which is why they
		// are the ones sellers miss: the section is collapsed by default on a new
		// product, so nothing on screen says they are mandatory.
		if (!isPositive(sku.weight)) problems.push(`${where}重量需大於 0`);
		if (!isPositive(sku.length_cm)) problems.push(`${where}包裝長度需大於 0`);
		if (!isPositive(sku.width_cm)) problems.push(`${where}包裝寬度需大於 0`);
		if (!isPositive(sku.height_cm)) problems.push(`${where}包裝高度需大於 0`);
	});

	return problems;
}

export const NEW_SKU_TEMPLATE: ProductSku = {
	id: "",
	// Empty by default (placeholder-only); a name becomes required once there
	// is more than one SKU — see validateDraft.
	sku_name: "",
	price: "0",
	original_price: "",
	sale_price: "",
	stock: 0,
	unit: WeightUnit.KG,
};

export type ShelfSubmitAction = "save" | "publish";
export interface ShelfButton {
	label: string;
	action: ShelfSubmitAction;
}
export interface ShelfButtons {
	primary: ShelfButton;
	secondary: ShelfButton | null;
}

/**
 * 依當前狀態決定編輯頁底部按鈕（無 radio，狀態靠按鈕表達）。
 * - create：後端一律建成上架，故只有一顆「建立並上架」。
 * - 草稿/下架：可更新內容 → primary 存欄位不變狀態、secondary 上架。
 * - 上架(1)：後端拒絕內容更新，由頁面閘記處理，不會走到這裡。
 */
export function resolveStatusButtons(
	mode: "create" | "edit",
	status: number,
): ShelfButtons {
	if (mode === "create") {
		return {
			primary: { label: "建立並上架", action: "publish" },
			secondary: null,
		};
	}
	if (status === 2) {
		return {
			primary: { label: "儲存變更", action: "save" },
			secondary: { label: "儲存並上架", action: "publish" },
		};
	}
	return {
		primary: { label: "儲存草稿", action: "save" },
		secondary: { label: "儲存並上架", action: "publish" },
	};
}
