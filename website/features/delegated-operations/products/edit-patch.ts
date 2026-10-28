import { WEIGHT_UNIT_LABEL } from "@/lib/codemeta/enums";
import { ossImageUrl } from "@/lib/media/cdn";
import type {
	ProductDescription,
	ProductDescriptionBlock,
} from "@/lib/products/seller-repo";
import type { SellerMutationChange } from "@/lib/seller-data/scope";
import type { DelegatedProductFormDraft } from "./conflict";
import type {
	DelegatedProductDetail,
	DelegatedProductImageInput,
	DelegatedProductPatch,
	DelegatedProductSkuInput,
} from "./repo";

/**
 * 代行編輯頁「表單 → PATCH body」與「表單 → 送出前三欄表」的唯一算式（B20）。
 *
 * 抽成純函式而不是留在頁面的 `useMemo` 裡，是因為這兩件事的錯誤形狀都**不會轉紅**：
 * 少送一個欄位 ⇒ Gateway 以 before 回填，請求合法、回 `code: 0`、稽核的
 * `changedFields` 少一列；多列一個唯讀觀察欄位 ⇒ 抽屜展示一筆不會發生的變更。
 * 兩者都只有靠逐條斷言才看得見。
 */

/** 表單上一張圖。既有圖用 `assetId` 當 React key，新上傳的用 `stagedToken`。 */
export interface ImageDraft {
	key: string;
	assetId?: string;
	stagedToken?: string;
	previewUrl: string;
}

/** 編輯頁的完整表單狀態（`DelegatedProductFormDraft` ＋ 不進合流的圖片）。 */
export interface ProductEditForm extends DelegatedProductFormDraft {
	images: ImageDraft[];
}

export function imageDraftsOf(detail: DelegatedProductDetail): ImageDraft[] {
	return detail.images.map((image) => ({
		key: image.assetId,
		assetId: image.assetId,
		previewUrl: ossImageUrl(image.imageKey, "productDetail"),
	}));
}

/**
 * 比較描述時**不看**的區塊欄位 —— 編輯器對它們既無讀路徑也無寫路徑，所以兩邊的差異
 * 不可能是這次編輯造成的（#2481）。
 *
 * - `imageUrl` — Gateway 只回 `imageKey`，這個孿生欄是前端 `readDescriptionBlock`
 *   自己補上去給 `<img src>` 用的（`repo.ts`），送出前又被 `toDescriptionWireInput`
 *   剝掉。它從來不是文件的一部分。
 * - `width` / `height` — 後端對 null 一律回字串 `"0"`
 *   （`ProductProtoConverter.java:811-812`，`omitempty` 只擋得掉 `""`），於是**每一個**
 *   區塊都帶著 `width:"0"`／`height:"0"` 進來；而 Editor.js 的資料模型不帶尺寸，
 *   `toEditorData`／`fromEditorData` 兩頭都沒有這兩欄，round-trip 後必然消失。
 *
 * ⚠️ 這裡只排除「編輯器碰不到」的欄位，**不排除任何編輯器產得出來的東西**：`runs`
 * 的每個樣式旗標、`style` 的每個 key、`alt`／`imageKey`／`assetId`／`tableJson`／
 * `listJson`／`id`／區塊順序與數量全部照比。放寬到會吞掉真實編輯，比多送一次嚴重
 * 得多 —— 那是「改了卻沒存、而且沒有任何訊號」。
 */
const NON_EDITABLE_BLOCK_FIELDS: ReadonlySet<string> = new Set([
	"imageUrl",
	"width",
	"height",
]);

/**
 * 穩定序列化：key 依字母排序、空值（`undefined`／`null`／`""`）視同不存在。
 *
 * `JSON.stringify` 對 key 的**插入順序**敏感，而這兩份文件是兩個不同的生產者組出來的
 * （後端 DTO 的欄序 vs `fromEditorData` 的欄序），順序本身不帶語意。實例：一個同時是
 * 連結又是粗體的 run，Gateway 給 `{text,bold,href}`、編輯器給 `{text,href,bold}`。
 * 空值同理 —— `alt:""` 與沒有 `alt` 是同一件事（`readDescriptionBlock` 自己就是這樣讀的）。
 */
function canonicalValue(value: unknown, drop?: ReadonlySet<string>): unknown {
	if (Array.isArray(value)) return value.map((item) => canonicalValue(item));
	if (value && typeof value === "object") {
		const source = value as Record<string, unknown>;
		const out: Record<string, unknown> = {};
		for (const key of Object.keys(source).sort()) {
			if (drop?.has(key)) continue;
			const inner = source[key];
			if (inner === undefined || inner === null || inner === "") continue;
			out[key] = canonicalValue(inner);
		}
		return out;
	}
	return value;
}

function canonicalBlocks(blocks: readonly ProductDescriptionBlock[]): string {
	return JSON.stringify(
		blocks.map((block) => canonicalValue(block, NON_EDITABLE_BLOCK_FIELDS)),
	);
}

/**
 * 兩份描述是否等值。整份替換的欄位，只有「真的不同」才送。
 *
 * 比的是 canonical 形而不是原字串：後端回來的那份與編輯器 round-trip 出來的那份，在
 * 「編輯器碰不到的欄位」與「key 順序」上本來就長得不一樣，直接 `JSON.stringify` 對拍
 * 會讓**每一個商品**在完全沒動描述的情況下判為有異 —— PATCH 恆帶 description，
 * 送出前的三欄表恆多一列「商品描述 N 個區塊 → N 個區塊」（#2481）。
 */
export function sameDescription(
	a: ProductDescription,
	b: ProductDescription,
): boolean {
	return canonicalBlocks(a.blocks) === canonicalBlocks(b.blocks);
}

export function sameImages(
	before: readonly ImageDraft[],
	after: readonly ImageDraft[],
): boolean {
	if (before.length !== after.length) return false;
	return before.every((image, index) => image.key === after[index]?.key);
}

export function toImageInputs(
	images: readonly ImageDraft[],
): DelegatedProductImageInput[] {
	return images.map((image) =>
		image.stagedToken
			? { stagedToken: image.stagedToken }
			: { assetId: image.assetId ?? "" },
	);
}

export function unitText(unit: string): string {
	return WEIGHT_UNIT_LABEL[unit] ?? unit;
}

/**
 * 只送真的改了的欄位：未送的欄位由 Gateway 以 before 快照回填（見 `repo.ts`）。
 *
 * ⚠️ `categoryId` **空字串一律不送**：Gateway 的 `validateProductCategoryID` 只認數字
 * 字串，空字串是 9260；而分類樹還沒載完時表單上的 `categoryId` 本來就是空的。
 */
export function buildProductPatch(
	detail: DelegatedProductDetail,
	form: ProductEditForm,
): DelegatedProductPatch {
	const next: DelegatedProductPatch = {};
	if (form.name.trim() !== detail.name) next.name = form.name.trim();
	if (form.categoryId !== "" && form.categoryId !== detail.categoryId) {
		next.categoryId = form.categoryId;
	}
	if (form.origin.trim() !== detail.origin) next.origin = form.origin.trim();
	if (!sameDescription(detail.description, form.description)) {
		next.description = form.description;
	}
	if (!sameImages(imageDraftsOf(detail), form.images)) {
		next.images = toImageInputs(form.images);
	}
	// ⚠️ **有任何一個 SKU 改動，就必須把全部 SKU 一起送**：product-service 對
	// 一個非空的 `skus` 陣列做的是整份對齊 —— 沒出現在陣列裡的既有規格會被
	// **移除**（本機實打坐實：只送 SKU 184 之後，SKU 183「4兩裝」整列消失，而
	// 請求合法、回 `code: 0`）。未改的那幾筆只帶 `skuId`，其餘欄位由 Gateway 以
	// before 快照逐欄回填，等於原值。
	const skus: DelegatedProductSkuInput[] = [];
	let skusDirty = false;
	for (const sku of detail.skus) {
		const draft = form.skus[sku.skuId];
		const changed: DelegatedProductSkuInput = { skuId: sku.skuId };
		skus.push(changed);
		if (!draft) continue;
		let dirty = false;
		if (draft.spec !== sku.spec) {
			changed.spec = draft.spec;
			dirty = true;
		}
		if (draft.price !== sku.price) {
			changed.price = draft.price;
			dirty = true;
		}
		if (draft.salePrice !== sku.salePrice) {
			changed.salePrice = draft.salePrice;
			dirty = true;
		}
		if (draft.stockQty !== sku.stockQty) {
			changed.stockQty = draft.stockQty;
			dirty = true;
		}
		if (dirty) skusDirty = true;
	}
	if (skusDirty) next.skus = skus;
	return next;
}

/**
 * 送出前的三欄表（B20）。欄位名取表單自己的標籤；商品層的 `stockQty`／`priceMin`／
 * `priceMax`／`status` 是唯讀觀察欄位，**不列入**。
 *
 * `categoryName` 由呼叫端提供（分類樹是頁面自己抓的，這裡不做 I/O）：三欄表上要出現
 * 的是可讀的分類名，印 id 等於要操作者自己去查那個數字是什麼。查不到名字時退回 id
 * 而不是空白 —— 空白會讓「改了分類」看起來像「清掉分類」。
 */
export function buildProductChanges(
	detail: DelegatedProductDetail,
	form: ProductEditForm,
	patch: DelegatedProductPatch,
	categoryNameOf: (categoryId: string) => string,
): SellerMutationChange[] {
	const rows: SellerMutationChange[] = [];
	if (patch.name !== undefined) {
		rows.push({ field: "商品名稱", before: detail.name, after: patch.name });
	}
	if (patch.categoryId !== undefined) {
		rows.push({
			field: "商品分類",
			before: detail.categoryName || detail.categoryId,
			after: categoryNameOf(patch.categoryId) || patch.categoryId,
		});
	}
	if (patch.origin !== undefined) {
		rows.push({ field: "產地", before: detail.origin, after: patch.origin });
	}
	if (patch.description !== undefined) {
		rows.push({
			field: "商品描述",
			before: `${detail.description.blocks.length} 個區塊`,
			after: `${form.description.blocks.length} 個區塊`,
		});
	}
	if (patch.images !== undefined) {
		rows.push({
			field: "商品圖片",
			before: `${detail.images.length} 張`,
			after: `${form.images.length} 張`,
		});
	}
	for (const sku of detail.skus) {
		const draft = form.skus[sku.skuId];
		if (!draft) continue;
		const prefix = sku.spec ? `${sku.spec}・` : "";
		if (draft.spec !== sku.spec) {
			rows.push({
				field: `${prefix}規格名稱`,
				before: sku.spec,
				after: draft.spec,
			});
		}
		if (draft.price !== sku.price) {
			rows.push({
				field: `${prefix}售價`,
				before: sku.price,
				after: draft.price,
			});
		}
		if (draft.salePrice !== sku.salePrice) {
			rows.push({
				field: `${prefix}特價`,
				before: sku.salePrice,
				after: draft.salePrice,
			});
		}
		if (draft.stockQty !== sku.stockQty) {
			rows.push({
				field: `${prefix}庫存數量`,
				before: `${sku.stockQty} ${unitText(sku.unit)}`,
				after: `${draft.stockQty} ${unitText(sku.unit)}`,
			});
		}
	}
	return rows;
}
