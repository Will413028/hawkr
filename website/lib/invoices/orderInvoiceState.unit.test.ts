import { describe, expect, it } from "vitest";
import {
	foldOrderInvoiceState,
	invoiceFileUnavailableHint,
	ORDER_INVOICE_STATE,
	orderInvoiceEmptyHint,
} from "./orderInvoiceState";

/**
 * 這張 display map 守兩件事：**後端加了前端還不知道的 state 時畫面不能白掉**
 * （58A §0.6 C11），以及**開票責任歸屬不可被折疊掉**（§B04）。後者不是美觀問題：
 * 「賣家自行開立」被顯示成「平台處理中」會讓買家對著平台等一張根本不在平台手上
 * 的發票。
 */
describe("訂單發票 state 的橫幅折疊表", () => {
	it("已成立的三個狀態不出橫幅，交給文件卡自己的 Chip 表達", () => {
		for (const state of [
			ORDER_INVOICE_STATE.ISSUED,
			ORDER_INVOICE_STATE.VOIDED,
			ORDER_INVOICE_STATE.ALLOWANCE_APPLIED,
		]) {
			expect(foldOrderInvoiceState(state)).toBeNull();
		}
	});

	it("賣家自行開立的兩個狀態彼此可分辨，且都不說成平台已開立", () => {
		const willIssue = foldOrderInvoiceState(
			ORDER_INVOICE_STATE.SELLER_WILL_ISSUE,
		);
		const selfIssued = foldOrderInvoiceState(
			ORDER_INVOICE_STATE.SELLER_SELF_ISSUED,
		);
		expect(willIssue?.title).not.toBe(selfIssued?.title);
		for (const banner of [willIssue, selfIssued]) {
			expect(banner?.title).toContain("賣家");
			expect(banner?.title).not.toContain("平台");
		}
	});

	it("開立異常是 alert 而不是 status，其餘反之", () => {
		expect(foldOrderInvoiceState(ORDER_INVOICE_STATE.FAILED)?.role).toBe(
			"alert",
		);
		expect(foldOrderInvoiceState(ORDER_INVOICE_STATE.PROCESSING)?.role).toBe(
			"status",
		);
	});

	it("只有處理中給重新整理鍵——其餘狀態按了也不會變", () => {
		expect(
			foldOrderInvoiceState(ORDER_INVOICE_STATE.PROCESSING)?.showRefresh,
		).toBe(true);
		expect(
			foldOrderInvoiceState(ORDER_INVOICE_STATE.AWAITING_PAYMENT)?.showRefresh,
		).toBe(false);
	});

	it("後端加了新 state 時落 default，橫幅仍然出得來", () => {
		const unseen = foldOrderInvoiceState("PENDING_TAX_AUTHORITY_REVIEW");
		expect(unseen?.unknown).toBe(true);
		expect(unseen?.title.length).toBeGreaterThan(0);
		// 未知值不得被當成失敗——那會讓買家去申訴一件其實正在正常進行的事。
		expect(unseen?.role).toBe("status");
	});

	/**
	 * backend #2904：已取消的訂單原本會拿到 `AWAITING_PAYMENT` ＋「付款完成後會立即
	 * 開立發票」。後端補了第 9 個 state，前端**必須同批補分支**——落 `default` 的話
	 * 標題是「處理中」、空狀態句是「系統正在處理，完成後會寄送 Email」，
	 * 對一張已取消的訂單那是兩句假話，後者還多承諾了一封不會寄出的信。
	 */
	it("已取消的訂單有自己的分支，不落 default", () => {
		const banner = foldOrderInvoiceState(ORDER_INVOICE_STATE.ORDER_CANCELLED);
		expect(banner?.unknown).toBe(false);
		expect(banner?.title).toBe("訂單已取消");
		// 取消不是異常：紅色會讓買家以為發票出了問題而跑去申訴。
		expect(banner?.role).toBe("status");
		expect(banner?.tone).not.toBe("danger");
		// 這個狀態不會再變，給重新整理鍵是騙人的。
		expect(banner?.showRefresh).toBe(false);
	});

	it("已取消的訂單不得被告知「完成後會寄送 Email」", () => {
		const hint = orderInvoiceEmptyHint(ORDER_INVOICE_STATE.ORDER_CANCELLED);
		// 🔴 陰性斷言：這正是落 default 時會印出來的那一句。
		expect(hint).not.toContain("寄送 Email");
		expect(hint).not.toContain("處理");
		expect(hint).toContain("不會開立發票");
	});

	/**
	 * 🔴 鑑別力：上面兩支若只比對字面值，把 `ORDER_CANCELLED` 的 case 刪掉之後
	 * 它們確實會紅——但「刪掉 case」不是唯一的退步方式。這一支釘住的是
	 * **每一個宣告出來的 state 都真的有自己的分支**，所以未來再加第 10 個值
	 * 而忘了補 case 時，一樣會在這裡紅。
	 */
	it("宣告出來的每一個 state 都有自己的分支，沒有一個落 default", () => {
		const states = Object.values(ORDER_INVOICE_STATE);
		expect(states.length).toBe(9);
		for (const state of states) {
			const banner = foldOrderInvoiceState(state);
			// null（ISSUED／VOIDED／ALLOWANCE_APPLIED）是刻意不出橫幅，不是落 default。
			if (banner === null) continue;
			expect(
				banner.unknown,
				`${state} 落到 default 分支了 —— 它會顯示「處理中」`,
			).toBe(false);
		}
		// 控制組：真正未知的值仍然要落 default（不能為了讓上面全綠而把 default 拿掉）。
		expect(foldOrderInvoiceState("SOMETHING_NEW")?.unknown).toBe(true);
	});

	it("未知 state 的空狀態說明句也有東西可印", () => {
		expect(
			orderInvoiceEmptyHint("PENDING_TAX_AUTHORITY_REVIEW").length,
		).toBeGreaterThan(0);
	});
});

describe("PDF 不可用的三個理由", () => {
	it("三句話彼此不同——它們對買家的下一步不一樣", () => {
		const hints = ["CARRIER_NOT_WON", "EXPIRED", "NOT_APPLICABLE"].map(
			invoiceFileUnavailableHint,
		);
		expect(new Set(hints).size).toBe(3);
	});

	it("後端加了第四個理由時仍有一句可印", () => {
		expect(
			invoiceFileUnavailableHint("PROVIDER_OUTAGE").length,
		).toBeGreaterThan(0);
	});
});
