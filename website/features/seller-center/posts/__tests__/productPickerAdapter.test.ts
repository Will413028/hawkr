import { describe, expect, it } from "vitest";
import { mapProductRecord, mapProductRecords } from "../productPickerAdapter";
import onSaleFixture from "./fixtures/public-products-on-sale.json";
import wireFixture from "./fixtures/public-seller-products.json";

/**
 * 契約測試。fixture 是 2026-08-06 從 dev 實抓的
 * `GET /public/sellers/{id}/products` 與 `GET /public/products` 回應。
 *
 * 這支 adapter 以前讀 `skuId` / `skuName` / `productName` / `image` / `pic` /
 * `images[0]` / `unit` / `price` / `salePrice` —— 九個候選欄位在 wire 上一個都不
 * 存在。所以本檔的重點不只是「新形狀讀得到」，更是**只餵舊 key 時必須讀不到**：
 * 舊 key 還讀得到，就代表猜欄位的 `??` 鏈又長回來了。
 */

const records = wireFixture as Array<Record<string, unknown>>;
const onSale = onSaleFixture as Array<Record<string, unknown>>;

describe("mapProductRecord — 實錄 wire", () => {
	it("fixture 本身就是真的 wire（有 id/name/imageUrl/priceMin，沒有那九個舊 key）", () => {
		expect(records.length).toBeGreaterThan(0);
		for (const r of records) {
			expect(typeof r.id).toBe("string");
			expect(typeof r.name).toBe("string");
			expect(r.priceMin).toBeDefined();
			for (const legacy of [
				"skuId",
				"skuName",
				"productName",
				"image",
				"pic",
				"images",
				"unit",
				"price",
				"salePrice",
			]) {
				expect(Object.hasOwn(r, legacy)).toBe(false);
			}
		}
	});

	it("每一筆都映射得出 id / 名稱 / 圖 / 售價文字", () => {
		const rows = mapProductRecords(records);
		expect(rows).toHaveLength(records.length);
		for (const row of rows) {
			expect(row.id).toMatch(/^\d+$/);
			expect(row.name).not.toBe("未命名商品");
			expect(row.imageUrl).toMatch(/^https?:\/\//);
			// 價格必須是實際數字，不能是空字串（空＝讀錯欄位的症狀）。
			expect(row.price.effectiveText).toMatch(/^NT\$ \d/);
		}
	});

	it("有特價的商品會顯示刪除線原價（舊 code 讀 salePrice，永遠顯示不出來）", () => {
		const rows = mapProductRecords(onSale);
		const discounted = rows.filter((r) => r.price.isOnSale);
		expect(discounted.length).toBeGreaterThan(0);
		for (const row of discounted) {
			expect(row.price.originalText).toMatch(/^NT\$ \d/);
			expect(row.price.originalText).not.toBe(row.price.effectiveText);
		}
	});
});

describe("mapProductRecord — 舊 key 一律讀不到（禁止 ?? 猜欄位鏈）", () => {
	it("只給 skuId 沒有 id → 整筆被丟棄", () => {
		expect(mapProductRecord({ skuId: "123", skuName: "舊形狀" })).toBeNull();
	});

	it("只給 skuName / productName → 名稱落到未命名，不是讀到舊值", () => {
		const row = mapProductRecord({
			id: "1",
			skuName: "舊名 A",
			productName: "舊名 B",
		});
		expect(row?.name).toBe("未命名商品");
	});

	it("只給 image / pic / images → 沒有圖，不是讀到舊值", () => {
		const row = mapProductRecord({
			id: "1",
			image: "https://old/a.png",
			pic: "https://old/b.png",
			images: ["https://old/c.png"],
		});
		expect(row?.imageUrl).toBeUndefined();
	});

	it("只給 price / salePrice → 價格是空的，不是讀到舊值", () => {
		const row = mapProductRecord({ id: "1", price: 320, salePrice: 280 });
		expect(row?.price.effectiveText).toBe("");
		expect(row?.price.isOnSale).toBe(false);
	});

	it("unit 已不在 view model 上（那個「（組）」是憑空捏的）", () => {
		const row = mapProductRecord({ id: "1", name: "A", unit: "斤" });
		expect(Object.hasOwn(row as object, "unit")).toBe(false);
	});
});
