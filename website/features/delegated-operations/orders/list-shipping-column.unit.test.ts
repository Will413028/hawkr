import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { ShippingStatus } from "@/lib/codemeta/enums";
import { shippingStatusText } from "./display";

/**
 * 設計書 §6.7 的保留清單逐字列了「物流狀態」。列表 DTO 一直帶著
 * `shippingStatus`／`shippingStatusLabel`（`gateway/internal/handler/admin_delegated_order.go`
 * 的 `DelegatedOrderListItem`），先前只有明細面板在讀 —— 代行者要回答的「賣家沒動，
 * 這筆卡在哪」，卡點在物流那一側，而列表是他唯一會逐筆掃過的畫面。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const PAGE = "features/delegated-operations/orders/DelegatedOrdersPage.tsx";

function read(file: string): string {
	return readFileSync(join(ROOT, file), "utf8");
}

describe("shippingStatusText", () => {
	it("優先用後端下發的 label", () => {
		expect(
			shippingStatusText(ShippingStatus.IN_TRANSIT, "配送中（後端）"),
		).toBe("配送中（後端）");
	});

	it("後端沒送 label 時查 CodeMeta", () => {
		expect(shippingStatusText(ShippingStatus.IN_TRANSIT, "")).toBe("配送中");
		expect(shippingStatusText(ShippingStatus.ABNORMAL, "")).toBe("異常訂單");
	});

	it("未知碼原樣顯示，不編字", () => {
		expect(shippingStatusText("99999", "")).toBe("99999");
	});

	// 還沒建立物流單的訂單 `shippingStatus` 就是空的。退到「待出貨」會與訂單狀態的
	// 「待出貨」同字，看起來像物流單已經建好了——而這一欄存在的理由正是分辨那件事。
	it("空碼回空字串，呼叫端據此顯示破折號", () => {
		expect(shippingStatusText("", "")).toBe("");
	});
});

describe("代行訂單列表：物流狀態欄", () => {
	const source = read(PAGE);

	it("表頭有「物流狀態」", () => {
		expect(source).toContain("<th>物流狀態</th>");
	});

	it("列內容讀的是列表 DTO 的物流欄位，不是訂單狀態", () => {
		expect(source).toContain("shippingStatusText(");
		expect(source).toContain("order.shippingStatus");
		expect(source).toContain("order.shippingStatusLabel");
	});

	/**
	 * 表頭欄數與空狀態列的 `colSpan` 是一份手寫的平行清單：加一欄而漏改 `colSpan`，
	 * 編譯過、測試綠，只有「載入中／載入失敗／零筆」那三列會少掉一欄的寬度而縮在
	 * 左邊。這一條把兩者綁起來。
	 */
	it("colSpan 與表頭欄數一致", () => {
		const thead = source.match(/<thead>([\s\S]*?)<\/thead>/);
		expect(thead).not.toBeNull();
		const columns = (thead?.[1].match(/<th>/g) ?? []).length;
		expect(columns).toBe(7);
		const spans = [...source.matchAll(/colSpan=\{(\d+)\}/g)].map((m) =>
			Number(m[1]),
		);
		expect(spans.length).toBeGreaterThanOrEqual(3);
		for (const span of spans) expect(span).toBe(columns);
	});
});
