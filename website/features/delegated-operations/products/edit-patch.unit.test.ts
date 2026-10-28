import { describe, expect, it } from "vitest";
import type {
	ProductDescription,
	ProductDescriptionBlock,
} from "@/lib/products/seller-repo";
import {
	buildProductChanges,
	buildProductPatch,
	imageDraftsOf,
	type ProductEditForm,
	sameDescription,
} from "./edit-patch";
import type { DelegatedProductDetail } from "./repo";

/**
 * PATCH body 與送出前三欄表的算式（B20）。
 *
 * 這一組守的是兩個**不會轉紅**的失敗：
 * - 少送一個欄位 ⇒ Gateway 以 before 回填、回 `code: 0`，操作者以為改好了
 * - 多列一個唯讀觀察欄位 ⇒ 抽屜展示一筆不會發生的變更（`status`／`priceMin`／
 *   `priceMax`／商品層 `stockQty` 在 PATCH 一律被忽略且不報錯）
 *
 * 分類與認證是本輪新開放的兩欄（§6.1.12 規則表（2）皆標「可編輯 ✔」）。
 */

function detail(over: Partial<DelegatedProductDetail> = {}) {
	return {
		productId: "9100",
		version: "14",
		name: "梨山高麗菜",
		status: "4003",
		statusLabel: "未啟用",
		purchasable: false,
		publicationState: "unlisted",
		categoryId: "527",
		categoryName: "高麗菜",
		description: { version: "1.0" as const, blocks: [] },
		images: [{ assetId: "5001", imageKey: "product/a.jpg" }],
		origin: "台灣 梨山",
		hashtags: [],
		skus: [
			{
				skuId: "184",
				spec: "3 台斤裝",
				price: "300",
				salePrice: "",
				stockQty: "8",
				unit: "3302",
				weight: "1.8",
				lengthCm: "30",
				widthCm: "20",
				heightCm: "15",
				shippingMethods: [],
			},
		],
		priceMin: "300",
		priceMax: "300",
		stockQty: "8",
		salesCount: "0",
		...over,
	} satisfies DelegatedProductDetail;
}

/** 與 detail 完全一致的表單狀態＝「什麼都沒改」。 */
function formOf(
	base: DelegatedProductDetail,
	over: Partial<ProductEditForm> = {},
): ProductEditForm {
	return {
		name: base.name,
		categoryId: base.categoryId,
		origin: base.origin,
		description: base.description,
		images: imageDraftsOf(base),
		skus: Object.fromEntries(
			base.skus.map((sku) => [
				sku.skuId,
				{
					spec: sku.spec,
					price: sku.price,
					salePrice: sku.salePrice,
					stockQty: sku.stockQty,
					unit: sku.unit,
				},
			]),
		),
		...over,
	};
}

const categoryNameOf = (id: string) =>
	({ "527": "高麗菜", "531": "蘿蔔" })[id] ?? "";

describe("buildProductPatch：沒改就不送", () => {
	it("表單與 detail 一致時 patch 是空物件（送出鍵因此按不動）", () => {
		const base = detail();
		expect(buildProductPatch(base, formOf(base))).toEqual({});
	});

	it("只改分類：只送 categoryId", () => {
		const base = detail();
		const patch = buildProductPatch(base, formOf(base, { categoryId: "531" }));
		expect(patch).toEqual({ categoryId: "531" });
	});

	it("分類為空字串一律不送 —— 空值是 9260，而樹還沒載完時它就是空的", () => {
		const base = detail();
		expect(buildProductPatch(base, formOf(base, { categoryId: "" }))).toEqual(
			{},
		);
	});

	it("唯讀觀察欄位不出現在 patch 裡（status／priceMin／priceMax／商品層 stockQty）", () => {
		const base = detail();
		const patch = buildProductPatch(base, formOf(base, { name: "改名" }));
		expect(Object.keys(patch)).toEqual(["name"]);
	});

	it("動一個 SKU 就整份帶（少帶一筆＝那筆規格被移除）", () => {
		const base = detail({
			skus: [
				...detail().skus,
				{
					skuId: "185",
					spec: "5 台斤裝",
					price: "480",
					salePrice: "",
					stockQty: "3",
					unit: "3302",
					weight: "3",
					lengthCm: "35",
					widthCm: "25",
					heightCm: "20",
					shippingMethods: [],
				},
			],
		});
		const form = formOf(base);
		form.skus["184"].price = "280";
		const patch = buildProductPatch(base, form);
		expect(patch.skus).toEqual([
			{ skuId: "184", price: "280" },
			{ skuId: "185" },
		]);
	});
});

describe("buildProductChanges：三欄表", () => {
	it("分類列印可讀名稱，不印 id", () => {
		const base = detail();
		const form = formOf(base, { categoryId: "531" });
		const rows = buildProductChanges(
			base,
			form,
			buildProductPatch(base, form),
			categoryNameOf,
		);
		expect(rows).toEqual([
			{ field: "商品分類", before: "高麗菜", after: "蘿蔔" },
		]);
	});

	it("樹裡查不到名字時退回 id，不留白（留白看起來像分類被清掉）", () => {
		const base = detail();
		const form = formOf(base, { categoryId: "999" });
		const rows = buildProductChanges(
			base,
			form,
			buildProductPatch(base, form),
			categoryNameOf,
		);
		expect(rows[0]?.after).toBe("999");
	});

	it("沒改到的欄位不列（三欄表只列這一次真的會送的東西）", () => {
		const base = detail();
		const form = formOf(base);
		expect(
			buildProductChanges(
				base,
				form,
				buildProductPatch(base, form),
				categoryNameOf,
			),
		).toEqual([]);
	});
});

/**
 * #2481：完全沒動描述，送出前的三欄表卻多一列「商品描述 N 個區塊 → N 個區塊」。
 *
 * 下面兩份 blocks 是 dev 實測抓下來的原形（`/adm/api/products/{store}/{product}` 的
 * 回應 vs 攔截到的 PATCH body），**不是杜撰的**：後端對 null 的 width／height 一律回
 * 字串 `"0"`，而 Editor.js round-trip 之後那兩欄必然消失；image 區塊還多一個
 * `imageUrl`（前端補的顯示值）與 `alt: ""`／無 `alt` 的空值表述差。
 */
const API_BLOCKS = [
	{
		type: "text" as const,
		id: "blk-0",
		runs: [{ text: "示範農場 TGAP 產銷履歷 無籽檸檬。" }],
		width: "0",
		height: "0",
	},
	{
		type: "image" as const,
		id: "1N0EovusW_",
		assetId: "866353748220725367",
		width: "0",
		height: "0",
		imageKey: "public/product_desc/2685914682/20260718/a.png",
		imageUrl:
			"https://cdn.example/public/product_desc/2685914682/20260718/a.png",
	},
];
const EDITOR_BLOCKS = [
	{
		type: "text" as const,
		id: "blk-0",
		runs: [{ text: "示範農場 TGAP 產銷履歷 無籽檸檬。" }],
		style: undefined,
	},
	{
		type: "image" as const,
		id: "1N0EovusW_",
		assetId: "866353748220725367",
		imageKey: "public/product_desc/2685914682/20260718/a.png",
		stagedToken: undefined,
		alt: "",
	},
];
const doc = (blocks: ProductDescriptionBlock[]): ProductDescription => ({
	version: "1.0",
	blocks,
});
/** 改一個字：任何放寬都必須讓這一組維持「不同」。 */
const edited = (
	blocks: ProductDescriptionBlock[],
	over: Partial<ProductDescriptionBlock>,
) => doc([{ ...blocks[0], ...over }, blocks[1]]);

describe("sameDescription：mount 後的 canonical 回填不算變更（#2481）", () => {
	it("後端原形 vs 編輯器 round-trip 形＝等值（width/height/imageUrl/空 alt 都不是內容）", () => {
		expect(sameDescription(doc(API_BLOCKS), doc(EDITOR_BLOCKS))).toBe(true);
	});

	it("整條路徑：沒動描述就不進 patch，三欄表也不多那一列", () => {
		const base = detail({ description: doc(API_BLOCKS) });
		const form = formOf(base, { description: doc(EDITOR_BLOCKS) });
		const patch = buildProductPatch(base, form);
		expect(patch.description).toBeUndefined();
		expect(
			buildProductChanges(base, form, patch, categoryNameOf).map(
				(r) => r.field,
			),
		).not.toContain("商品描述");
	});

	it("key 順序不帶語意：連結＋粗體的 run 兩邊欄序不同仍算同一份", () => {
		const gateway = doc([
			{
				type: "text",
				id: "a",
				runs: [{ text: "看這裡", bold: true, href: "https://x.test/" }],
			},
		]);
		const editor = doc([
			{
				type: "text",
				id: "a",
				runs: [{ text: "看這裡", href: "https://x.test/", bold: true }],
			},
		]);
		expect(sameDescription(gateway, editor)).toBe(true);
	});
});

/**
 * 這一組是本次放寬的**唯一危險面**：判成相同＝改了卻不送，而 Gateway 會以 before
 * 回填、回 `code: 0`，操作者不會收到任何訊號。逐項釘住「真的改了就必須是不同」。
 */
describe("sameDescription：真實的描述變更一律不得被判成相同", () => {
	const cases: [string, ProductDescription][] = [
		[
			"改文字",
			edited(EDITOR_BLOCKS, { runs: [{ text: "示範農場 有籽檸檬。" }] }),
		],
		[
			"加粗體",
			edited(EDITOR_BLOCKS, {
				runs: [{ text: "示範農場 TGAP 產銷履歷 無籽檸檬。", bold: true }],
			}),
		],
		[
			"改顏色",
			edited(EDITOR_BLOCKS, {
				runs: [{ text: "示範農場 TGAP 產銷履歷 無籽檸檬。", color: "#ff0000" }],
			}),
		],
		[
			"加連結",
			edited(EDITOR_BLOCKS, {
				runs: [
					{
						text: "示範農場 TGAP 產銷履歷 無籽檸檬。",
						href: "https://x.test/",
					},
				],
			}),
		],
		[
			"改對齊（style）",
			edited(EDITOR_BLOCKS, { style: { textAlign: "center" } }),
		],
		["改區塊 id", edited(EDITOR_BLOCKS, { id: "blk-9" })],
		[
			"改區塊型別",
			edited(EDITOR_BLOCKS, { type: "table", tableJson: '{"rows":[]}' }),
		],
		["刪一個區塊", doc([EDITOR_BLOCKS[0]])],
		[
			"加一個區塊",
			doc([
				...EDITOR_BLOCKS,
				{ type: "text", id: "blk-9", runs: [{ text: "新段落" }] },
			]),
		],
		["調換順序", doc([EDITOR_BLOCKS[1], EDITOR_BLOCKS[0]])],
		[
			"換圖（imageKey）",
			doc([
				EDITOR_BLOCKS[0],
				{
					...EDITOR_BLOCKS[1],
					imageKey: "public/product_desc/2685914682/20260718/b.png",
				},
			]),
		],
		[
			"填圖說（alt 由空變非空）",
			doc([EDITOR_BLOCKS[0], { ...EDITOR_BLOCKS[1], alt: "檸檬園實拍" }]),
		],
		[
			"上傳新圖（stagedToken）",
			doc([
				EDITOR_BLOCKS[0],
				{ type: "image", id: "1N0EovusW_", stagedToken: "stg-1" },
			]),
		],
	];
	for (const [name, after] of cases) {
		it(`${name} ⇒ 不同`, () => {
			expect(sameDescription(doc(API_BLOCKS), after)).toBe(false);
		});
	}

	it("清掉圖說（alt 由非空變空）也是變更", () => {
		const before = doc([
			API_BLOCKS[0],
			{ ...API_BLOCKS[1], alt: "檸檬園實拍" },
		]);
		expect(sameDescription(before, doc(EDITOR_BLOCKS))).toBe(false);
	});

	it("legacy style 的 key 被 round-trip 掉了也算變更（不是靜默丟棄）", () => {
		const before = doc([
			{ ...API_BLOCKS[0], style: { color: "#336633" } },
			API_BLOCKS[1],
		]);
		expect(sameDescription(before, doc(EDITOR_BLOCKS))).toBe(false);
	});
});
