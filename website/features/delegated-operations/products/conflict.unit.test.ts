import { describe, expect, it } from "vitest";
import { draftFromDetail, keepMyEdits } from "./conflict";
import type { DelegatedProductDetail } from "./repo";

/**
 * 版本衝突（9266）的兩個出口在資料上的差別（契約 L5346）。
 *
 * 「複製我的內容」最容易寫錯的方向是把它做成「用我的整份狀態覆蓋」：那正是這條契約
 * 要防的事（覆寫賣家較新的修改），而且它的失敗是靜默的 —— 送出去請求完全合法、
 * `expectedVersion` 相符（因為是剛抓的）、回 `code: 0`。
 */

function detail(over: Partial<DelegatedProductDetail> = {}) {
	return {
		productId: "9100",
		version: "14",
		name: "梨山高麗菜（賣家已改）",
		status: "4003",
		statusLabel: "未啟用",
		purchasable: false,
		publicationState: "unlisted",
		categoryId: "527",
		categoryName: "蔬菜",
		description: { version: "1.0" as const, blocks: [] },
		images: [{ assetId: "5001", imageKey: "product/new.jpg" }],
		origin: "台灣 梨山（賣家已改）",
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

const mine = {
	name: "梨山高麗菜（我打的）",
	categoryId: "531",
	origin: "台灣 和平區（我打的）",
	description: {
		version: "1.0" as const,
		blocks: [{ type: "text" as const, content: "我寫的說明" }],
	},
	skus: {
		"184": {
			spec: "3 台斤裝",
			price: "280",
			salePrice: "",
			stockQty: "20",
			unit: "3302",
		},
	},
};

describe("版本衝突的合流", () => {
	it("重新載入最新版本：整份改用賣家的內容", () => {
		const draft = draftFromDetail(detail());
		expect(draft.name).toBe("梨山高麗菜（賣家已改）");
		expect(draft.categoryId).toBe("527");
		expect(draft.skus["184"].price).toBe("300");
	});

	it("複製我的內容：我填過的欄位留我的，版本基底換成最新的", () => {
		const latest = detail();
		const merged = keepMyEdits(latest, mine);

		expect(merged.name).toBe("梨山高麗菜（我打的）");
		expect(merged.categoryId).toBe("531");
		expect(merged.origin).toBe("台灣 和平區（我打的）");
		expect(merged.description.blocks).toHaveLength(1);
		expect(merged.skus["184"].price).toBe("280");
	});

	it("分類還沒選出來（樹未載完）時退回最新版本的分類，不清成空", () => {
		const latest = detail();
		const merged = keepMyEdits(latest, { ...mine, categoryId: "" });
		// 空的 categoryId 不會被送出，畫面卻空著 —— 兩邊對不上就是靜默的不一致。
		expect(merged.categoryId).toBe("527");
	});

	it("賣家新增的規格會出現在合流結果裡（少一筆＝那筆規格會被移除）", () => {
		const latest = detail({
			skus: [
				...detail().skus,
				{
					skuId: "185",
					spec: "5 台斤裝（賣家新增）",
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

		const merged = keepMyEdits(latest, mine);

		expect(Object.keys(merged.skus).sort()).toEqual(["184", "185"]);
		// 我沒碰過的那一筆，值取自最新版本而不是空白。
		expect(merged.skus["185"].price).toBe("480");
	});

	it("賣家刪掉的規格不會被我帶回來", () => {
		const latest = detail({ skus: [] });
		const merged = keepMyEdits(latest, mine);
		expect(Object.keys(merged.skus)).toEqual([]);
	});

	it("圖片不在合流結果內：assetId 憑據必須來自這一次 GET（否則 9275）", () => {
		const merged = keepMyEdits(detail(), mine);
		expect(merged).not.toHaveProperty("images");
	});
});
