import { describe, expect, it } from "vitest";
import { conversationRangeLabel } from "../list-range";

describe("conversationRangeLabel", () => {
	it("全部載入且沒有搜尋時，兩個數字相同（設計稿 01／03／04 的 1 - 6 / 6）", () => {
		expect(
			conversationRangeLabel({
				visible: 6,
				loaded: 6,
				maybeTruncated: false,
			}),
		).toBe("顯示 1 - 6 / 6");
	});

	it("搜尋縮小清單時，分子跟著看得到的列走、分母維持這個分頁載入了幾列", () => {
		expect(
			conversationRangeLabel({
				visible: 3,
				loaded: 6,
				maybeTruncated: false,
			}),
		).toBe("顯示 1 - 3 / 6");
	});

	it("清單可能被截斷時分母帶 +，不把「載入了幾筆」講成「總共幾筆」", () => {
		expect(
			conversationRangeLabel({
				visible: 50,
				loaded: 50,
				maybeTruncated: true,
			}),
		).toBe("顯示 1 - 50 / 50+");
	});

	it("截斷與搜尋同時發生時，兩個規則各管各的軸", () => {
		expect(
			conversationRangeLabel({
				visible: 4,
				loaded: 50,
				maybeTruncated: true,
			}),
		).toBe("顯示 1 - 4 / 50+");
	});

	it("一列都看不到時回 null——空狀態已經在講話，「顯示 1 - 0」不是句子", () => {
		expect(
			conversationRangeLabel({
				visible: 0,
				loaded: 6,
				maybeTruncated: false,
			}),
		).toBeNull();
		expect(
			conversationRangeLabel({
				visible: 0,
				loaded: 0,
				maybeTruncated: false,
			}),
		).toBeNull();
	});

	it("只有一列時照樣寫成範圍，不為了省字改成另一種句型", () => {
		expect(
			conversationRangeLabel({
				visible: 1,
				loaded: 1,
				maybeTruncated: false,
			}),
		).toBe("顯示 1 - 1 / 1");
	});
});
