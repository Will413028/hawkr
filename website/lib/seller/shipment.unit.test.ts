import { describe, expect, it } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import {
	classifyShipmentError,
	friendlyShipmentMessage,
	shipmentErrorGuidance,
	TCAT_BRANCH_LOOKUP_URL,
} from "@/lib/seller/shipment";

/**
 * 回歸測試：賣家叫黑貓回 HTTP 409 卻顯示「可能已處理過」、看不到真因。
 *
 * 背景：Gateway 對所有物流業務碼統一回 HTTP 409 + envelope `[5xxx]` 前綴。
 * 舊版 classifyShipmentError 用 `httpStatus === 409` blanket 短路，把 5044
 * （黑貓叫件失敗，PayUni E072 真因）吞成籠統文案；又把 E072 硬編成「假日」；
 * 又在 friendlyShipmentMessage 整段刪括號把真因刪掉。本測試鎖定三者皆已修。
 */

/** 模擬 gateway 對物流業務錯誤的回應：HTTP 409 + `[code] message`。 */
function shipmentErr(code: number, message: string): ApiClientError {
	return new ApiClientError(`[${code}] ${message}`, 409, 409);
}

describe("friendlyShipmentMessage", () => {
	it("抽出 PayUni 巢狀真因（E072 地址不在範圍），而非整段刪括號", () => {
		const err = shipmentErr(
			5044,
			"呼叫黑貓失敗：處理失敗 (執行失敗 -> E072-寄件地址不在簽約可收貨範圍，請聯繫簽約站所。)",
		);
		expect(friendlyShipmentMessage(err)).toBe(
			"呼叫黑貓失敗：寄件地址不在簽約可收貨範圍，請聯繫簽約站所",
		);
	});

	it("以 PayUni 實際文字為準，不再把 E072 一律猜成假日", () => {
		const holiday = shipmentErr(
			5044,
			"呼叫黑貓失敗：處理失敗 (執行失敗 -> 假日不受理預約取件。)",
		);
		expect(friendlyShipmentMessage(holiday)).toBe(
			"呼叫黑貓失敗：假日不受理預約取件",
		);
	});

	it("本身已乾淨的訊息原樣透傳（無 -> pattern）", () => {
		const err = shipmentErr(5011, "尚未取號，請先產生 PDF 後再呼叫黑貓");
		expect(friendlyShipmentMessage(err)).toBe(
			"尚未取號，請先產生 PDF 後再呼叫黑貓",
		);
	});
});

describe("classifyShipmentError", () => {
	it("5044 黑貓叫件失敗：透傳 PayUni 真因、不 reload、不顯示『可能已處理過』", () => {
		const err = shipmentErr(
			5044,
			"呼叫黑貓失敗：處理失敗 (執行失敗 -> E072-寄件地址不在簽約可收貨範圍，請聯繫簽約站所。)",
		);
		const o = classifyShipmentError(err, "O593290575");
		expect(o.bizCode).toBe(5044);
		expect(o.handled).toBe(true);
		expect(o.tone).toBe("error");
		expect(o.shouldReload).toBe(false);
		expect(o.message).toBe(
			"呼叫黑貓失敗：寄件地址不在簽約可收貨範圍，請聯繫簽約站所",
		);
		// 舊 bug 的籠統文案絕不能再出現
		expect(o.message).not.toContain("可能已處理過");
		expect(o.message).not.toContain("假日");
	});

	it("5012 已叫過 / 5013 併發 / 5003 已取號：stale-state → shouldReload=true 換按鈕", () => {
		for (const [code, msg] of [
			[5012, "已呼叫過黑貓，不可重複呼叫"],
			[5013, "操作失敗（併發衝突），請重試"],
			[5003, "已取號，不可重複產生"],
		] as const) {
			const o = classifyShipmentError(shipmentErr(code, msg), "O1");
			expect(o.bizCode).toBe(code);
			expect(o.shouldReload).toBe(true);
			expect(o.handled).toBe(true);
			expect(o.message).toBe(msg);
			expect(o.message).not.toContain("可能已處理過");
		}
	});

	it("5011 未取號：交給 caller 補動作前綴（handled=false），但保留可行動訊息", () => {
		const o = classifyShipmentError(
			shipmentErr(5011, "尚未取號，請先產生 PDF 後再呼叫黑貓"),
			"O1",
		);
		expect(o.bizCode).toBe(5011);
		expect(o.handled).toBe(false);
		expect(o.shouldReload).toBe(false);
		expect(o.message).toBe("尚未取號，請先產生 PDF 後再呼叫黑貓");
	});

	it("5002 物流單尚未取得 PayUni 序號：提示後端建檔中，不顯示像操作順序錯的原始訊息", () => {
		const o = classifyShipmentError(
			shipmentErr(5002, "物流單尚未取得序號，無法操作"),
			"O1",
		);
		expect(o.bizCode).toBe(5002);
		expect(o.handled).toBe(true);
		expect(o.tone).toBe("warning");
		expect(o.shouldReload).toBe(false);
		expect(o.message).toContain("後端建檔中");
		expect(o.message).toContain("預約取號");
		expect(o.message).not.toContain("無法操作");
	});

	it("5046 建立未滿 30 秒：不 reload（只是太快）；5047 物流商處理中：reload", () => {
		const tooSoon = classifyShipmentError(
			shipmentErr(5046, "物流單建立未滿 30 秒，請稍候再列印"),
		);
		expect(tooSoon.shouldReload).toBe(false);

		const inProgress = classifyShipmentError(
			shipmentErr(5047, "物流商呼叫進行中，請稍候再試"),
		);
		expect(inProgress.shouldReload).toBe(true);
	});
});

describe("shipmentErrorGuidance", () => {
	it("黑貓寄件地址不在收件範圍：補上聯繫 PayUni / 找站所 / 自行預約引導 + 站所連結", () => {
		const o = classifyShipmentError(
			shipmentErr(
				5044,
				"呼叫黑貓失敗：處理失敗 (執行失敗 -> E072-寄件地址不在簽約可收貨範圍，請聯繫簽約站所。)",
			),
		);
		const g = shipmentErrorGuidance(o);
		expect(g).not.toBeNull();
		// 地址於開通鎖定，修改須走 PayUni 客服（與 ShippingClient 一致）
		expect(g?.steps.some((s) => s.includes("PayUni"))).toBe(true);
		expect(g?.links.some((l) => l.href === TCAT_BRANCH_LOOKUP_URL)).toBe(true);
	});

	it("其他成因（假日不收件）不顯示地址引導", () => {
		const o = classifyShipmentError(
			shipmentErr(
				5044,
				"呼叫黑貓失敗：處理失敗 (執行失敗 -> 假日不受理預約取件。)",
			),
		);
		expect(shipmentErrorGuidance(o)).toBeNull();
	});
});
