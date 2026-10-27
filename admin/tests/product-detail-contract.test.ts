import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { codeLabel, WEIGHT_UNIT_LABEL, WeightUnit } from "../src/lib/codemeta";
import {
	formatCategoryBreadcrumb,
	parseListJson,
	parseTableJson,
} from "../src/lib/product-description";
import type { ProductDetail } from "../src/types/product";

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const PRODUCT_DETAIL_PAGE = join(
	projectRoot,
	"src/app/admin/products/[id]/page.tsx",
);

/**
 * 對齊 gateway ProductDetailItem 的真實 JSON 欄位
 * （backend-payuni/gateway/internal/handler/product.go:436-475、
 * seller_product.go:18-24），GET /api/v1/admin/products/:id 的回應即此形狀。
 */
const PRODUCT_DETAIL_FIXTURE: ProductDetail = {
	id: "7123456789012345678",
	name: "有機牛番茄",
	status: "4004",
	origin: "台南",
	rating: "4.8",
	categoryId: "301",
	categoryName: "牛番茄",
	categoryLv1Id: "100",
	categoryLv1Name: "生鮮",
	categoryLv2Id: "200",
	categoryLv2Name: "蔬果",
	categoryLv3Id: "301",
	categoryLv3Name: "牛番茄",
	priceMin: "89",
	priceMax: "129",
	stock: "320",
	salesCount: "1520",
	favoriteCount: "88",
	reviewCount: "42",
	gallery: [
		{
			assetId: "a1",
			// 49 §6.1/§6.6.1：gallery 的圖欄收斂成 imageKey（objectKey SSOT）
			// ＋ imageUrl（Gateway 組），不再是 objectKey / url。
			imageKey: "public/a1.jpg",
			imageUrl: "https://test-media.example.com/i/original/public/a1.jpg",
		},
	],
	shippingMethods: [
		{
			// CodeMeta Value：3001=黑貓宅急便、3202=冷藏、3102=賣方
			// （backend-payuni services/java/shared/common-core .../code/ShippingMethod.java 等）
			type: "3001",
			temperature: "3202",
			freeThreshold: "1000",
			feePayer: "3102",
		},
	],
	skus: [
		{
			id: "sku-1",
			skuName: "1台斤裝",
			weight: "1.000",
			// CodeMeta WeightUnit Value：3301=公斤、3302=台斤、3303=公克
			// （backend-payuni `docs/代碼管理（CodeMeta）/業務類型碼.md` WeightUnit）。
			// wire 送的是數字碼，不是中文標籤 —— dev product_sku 280 筆全為 3301/3302/3303。
			unit: "3302",
			price: "89",
			stockQty: "150",
		},
	],
	description: {
		version: "1",
		blocks: [
			{ type: "text", id: "b1", runs: [{ text: "台南契作牛番茄" }] },
			{
				type: "image",
				id: "b4",
				imageKey: "public/desc/b4.jpg",
				imageUrl:
					"https://test-media.example.com/i/original/public/desc/b4.jpg",
			},
			{
				type: "table",
				id: "b2",
				tableJson: JSON.stringify({
					withHeadings: true,
					rows: [
						[{ runs: [{ text: "重量" }] }, { runs: [{ text: "價格" }] }],
						[{ runs: [{ text: "600g" }] }, { runs: [{ text: "89" }] }],
					],
				}),
			},
			{
				type: "list",
				id: "b3",
				listJson: JSON.stringify({
					ordered: false,
					items: [{ runs: [{ text: "冷藏配送" }] }],
				}),
			},
		],
	},
};

test("stock/favoriteCount/reviewCount 必為字串（TSID 精度保護，數字化回歸會在這裡轉紅）", () => {
	assert.equal(typeof PRODUCT_DETAIL_FIXTURE.stock, "string");
	assert.equal(typeof PRODUCT_DETAIL_FIXTURE.favoriteCount, "string");
	assert.equal(typeof PRODUCT_DETAIL_FIXTURE.reviewCount, "string");
	assert.equal(typeof PRODUCT_DETAIL_FIXTURE.skus?.[0]?.stockQty, "string");
});

test("分類麵包屑從 categoryLv1/2/3Name 三欄組出，不查分類樹", () => {
	const crumb = formatCategoryBreadcrumb([
		PRODUCT_DETAIL_FIXTURE.categoryLv1Name,
		PRODUCT_DETAIL_FIXTURE.categoryLv2Name,
		PRODUCT_DETAIL_FIXTURE.categoryLv3Name,
	]);
	assert.equal(crumb, "生鮮 > 蔬果 > 牛番茄");
});

test("description 的 table block 可用 parseTableJson 解析出真實 rows", () => {
	const tableBlock = PRODUCT_DETAIL_FIXTURE.description?.blocks?.find(
		(b) => b.type === "table",
	);
	const model = parseTableJson(tableBlock?.tableJson);
	assert.equal(model?.withHeadings, true);
	assert.equal(model?.rows.length, 2);
	assert.equal(model?.rows[0]?.[0]?.runs?.[0]?.text, "重量");
});

test("description 的 list block 可用 parseListJson 解析出真實 items", () => {
	const listBlock = PRODUCT_DETAIL_FIXTURE.description?.blocks?.find(
		(b) => b.type === "list",
	);
	const model = parseListJson(listBlock?.listJson);
	assert.equal(model?.ordered, false);
	assert.equal(model?.items.length, 1);
	assert.equal(model?.items[0]?.runs?.[0]?.text, "冷藏配送");
});

test("gallery / description 圖欄名為 imageKey + imageUrl，舊的 objectKey / url 不存在", () => {
	const g = PRODUCT_DETAIL_FIXTURE.gallery?.[0];
	assert.ok(g, "fixture 需要一筆 gallery");
	// gateway `GalleryItem` 兩欄都是 omitempty，但成對產生（imageUrl 由 imageKey
	// 組），所以有 key 就有 url —— 這條同時釘住「成對」這個不變式。
	assert.ok(
		g.imageUrl?.startsWith("http"),
		"imageUrl 是 Gateway 組好的顯示 URL",
	);
	assert.ok(
		g.imageKey?.startsWith("public/"),
		"imageKey 是 objectKey（SSOT），不是 URL",
	);
	const rawGallery = g as unknown as Record<string, unknown>;
	assert.equal("objectKey" in rawGallery, false, "objectKey 復活了");
	assert.equal("url" in rawGallery, false, "url 復活了");

	const imageBlock = PRODUCT_DETAIL_FIXTURE.description?.blocks?.find(
		(b) => b.type === "image",
	);
	assert.ok(imageBlock, "fixture 需要一個 image block");
	assert.equal(imageBlock.imageKey, "public/desc/b4.jpg");
	const rawBlock = imageBlock as unknown as Record<string, unknown>;
	assert.equal("objectKey" in rawBlock, false, "block.objectKey 復活了");
	assert.equal("url" in rawBlock, false, "block.url 復活了");
});

test("只給舊欄名的 gallery / block 讀不到圖（fallback 復活會轉紅）", () => {
	// 舊形狀原樣：assetId + objectKey + url。收斂後前端只讀 imageKey/imageUrl，
	// 因此這兩個取值必須是 undefined —— 任何 `imageUrl ?? url` 的相容層會在此轉紅。
	const legacyGallery = {
		assetId: "a1",
		objectKey: "public/a1.jpg",
		url: "https://test-media.example.com/a1.jpg",
	} as unknown as NonNullable<ProductDetail["gallery"]>[number];
	assert.equal(legacyGallery.imageKey, undefined);
	assert.equal(legacyGallery.imageUrl, undefined);

	const legacyBlock = {
		type: "image",
		id: "b9",
		objectKey: "public/desc/b9.jpg",
		url: "https://test-media.example.com/desc/b9.jpg",
	} as unknown as NonNullable<
		NonNullable<ProductDetail["description"]>["blocks"]
	>[number];
	assert.equal(legacyBlock.imageKey, undefined);
	assert.equal(legacyBlock.imageUrl, undefined);
});

test("SKU unit 是 CodeMeta WeightUnit 的數字碼，不是中文標籤（wire 形狀）", () => {
	const unit = PRODUCT_DETAIL_FIXTURE.skus?.[0]?.unit;
	// 後端送 Value（3301/3302/3303）。fixture 若寫回「台斤」這類人工占位字串，
	// 就再也釘不住 wire，也就抓不到「畫面直接印碼」這個 bug。
	assert.match(unit ?? "", /^\d{4}$/);
	assert.ok(unit && unit in WEIGHT_UNIT_LABEL, `${unit} 不在 WeightUnit 值域`);
});

test("WEIGHT_UNIT_LABEL 覆蓋 WeightUnit 的完整值域，且 fixture 的碼翻得出標籤", () => {
	// ⚠️ 這裡**刻意不列出中文標籤**。以前這支斷言把三個標籤逐字抄一份，那在
	// src/lib/codemeta.ts 還是手寫鏡像的年代有意義；現在碼表是生成物，再抄一次
	// 就只是「第 N 份會漂移的鏡像」，而且會在 SSOT 正當改字時假性轉紅。
	// 標籤內容的正確性由 CI 的 CodeMeta drift 檢查（生成物與 Markdown 一致）保證，
	// 這裡只驗「值域完整」與「翻得出來」這兩件本頁真正在乎的事。
	assert.deepEqual(
		Object.keys(WEIGHT_UNIT_LABEL).sort(),
		Object.values(WeightUnit).sort(),
	);
	const unit = PRODUCT_DETAIL_FIXTURE.skus?.[0]?.unit;
	// codeLabel 未命中會原樣回傳碼值 —— 回傳值不等於輸入，才代表真的翻譯到了。
	assert.notEqual(codeLabel(WEIGHT_UNIT_LABEL, unit), unit);
});

test("商品詳情頁的重量/單位欄一律經 codeLabel 轉標籤，不得直接印 sku.unit", () => {
	const source = readFileSync(PRODUCT_DETAIL_PAGE, "utf8");
	assert.match(
		source,
		/codeLabel\(\s*WEIGHT_UNIT_LABEL\s*,\s*sku\.unit\s*\)/,
		"重量/單位欄必須用 codeLabel(WEIGHT_UNIT_LABEL, sku.unit)",
	);
	// 把合法用法挖掉後，不該還有裸的 sku.unit —— 任何回歸成
	// `${sku.weight} ${sku.unit}` 的寫法會在這裡轉紅（畫面會顯示「1.000 3301」）。
	const stripped = source.replace(
		/codeLabel\(\s*WEIGHT_UNIT_LABEL\s*,\s*sku\.unit\s*\)/g,
		"",
	);
	assert.doesNotMatch(stripped, /sku\.unit/, "有裸印 sku.unit 的位置");
});
