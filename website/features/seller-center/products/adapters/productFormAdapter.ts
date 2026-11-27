/**
 * Adapters between the seller-product API shape and the form draft.
 *
 *   - emptyDraft()           initial draft for `mode === "create"`
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
	return {
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
		main_image: images[0]?.url || prev.main_image,
	};
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
	const skus = (draft.skus ?? []).map((sku, i) => {
		const skuName =
			sku.sku_name?.trim() || (multiSku ? `規格 ${i + 1}` : "預設規格");
		const out: SkuWriteInput = {
			skuName,
			price: String(Number(sku.price ?? 0) || 0),
			unit: coerceWeightUnitCode(sku.unit),
		};
		if (sku.id) out.id = sku.id;
		const sale = Number(sku.sale_price ?? 0);
		if (sale > 0 && sale < Number(sku.price ?? 0)) out.salePrice = String(sale);
		if (includeStock) out.stockQty = String(Number(sku.stock ?? 0) || 0);
		const weight = (sku.weight ?? "").trim();
		if (weight) out.weight = weight;
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

	const r = raw as Record<string, unknown>;
	const category_id =
		toNum(raw.category_id) ??
		toNum(r.categoryId) ??
		toNum(r.categoryLv3Id) ??
		toNum(r.categoryLv2Id) ??
		toNum(r.categoryLv1Id);

	const skus = Array.isArray(raw.skus)
		? raw.skus.map(normaliseSkuFromApi)
		: raw.skus;

	const rawGallery = Array.isArray(r.gallery) ? (r.gallery as unknown[]) : [];
	const images: ProductImageDraft[] = rawGallery
		.map((g): ProductImageDraft | null => {
			if (!g || typeof g !== "object") return null;
			const rec = g as Record<string, unknown>;
			const url = resolveCdnUrl(toStr(rec.imageKey));
			const assetId = toStr(rec.assetId);
			if (!url && !assetId) return null;
			return assetId ? { url, assetId } : { url };
		})
		.filter((x): x is ProductImageDraft => x !== null);
	if (rawGallery.length > 0 && images.length === 0) {
		console.warn(
			"[normaliseDraftFromApi] dropped all gallery images; unexpected shape:",
			rawGallery,
		);
	}

	const shelf = resolveShelfState(raw.status);
	const status = shelf === "active" ? 1 : shelf === "off" ? 2 : 0;

	return {
		...raw,
		status,
		category_id,
		skus,
		tags: hashtags,
		images,
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
	if (draft.category_id == null) {
		problems.push("請選擇商品分類（需選到最後一層）");
	}

	const skus = draft.skus ?? [];
	if (skus.length === 0) {
		problems.push("至少要一個規格 (SKU)");
		return problems;
	}

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
		if (!isPositive(sku.weight)) problems.push(`${where}重量需大於 0`);
		if (!isPositive(sku.length_cm)) problems.push(`${where}包裝長度需大於 0`);
		if (!isPositive(sku.width_cm)) problems.push(`${where}包裝寬度需大於 0`);
		if (!isPositive(sku.height_cm)) problems.push(`${where}包裝高度需大於 0`);
	});

	return problems;
}

export const NEW_SKU_TEMPLATE: ProductSku = {
	id: "",
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
