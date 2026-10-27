import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ORDER_INVOICE_STATE } from "@/lib/invoices/orderInvoiceState";
import { DocumentTimeline } from "./DocumentTimeline";
import { InvoiceStateBanner } from "./InvoiceStateBanner";

/**
 * 折疊表本身已在 `lib/invoices/orderInvoiceState.unit.test.ts` 測過；這裡測的是
 * **元件真的把折疊結果接上去了**——兩者是不同的失敗：折疊表對、元件忘了讀
 * `role`，症狀是讀屏使用者永遠聽不到「發票開立異常」，而畫面看起來完全正常。
 */
describe("InvoiceStateBanner", () => {
	it("開立異常是 role=alert；處理中是 role=status", () => {
		const failed = renderToStaticMarkup(
			createElement(InvoiceStateBanner, {
				state: ORDER_INVOICE_STATE.FAILED,
				notice: "後端給的那句話",
			}),
		);
		expect(failed).toContain('role="alert"');
		expect(failed).toContain("後端給的那句話");

		const processing = renderToStaticMarkup(
			createElement(InvoiceStateBanner, {
				state: ORDER_INVOICE_STATE.PROCESSING,
				notice: "",
			}),
		);
		expect(processing).toContain('role="status"');
	});

	it("已開立不出橫幅——狀態由文件卡的 Chip 表達，不重複講一次", () => {
		expect(
			renderToStaticMarkup(
				createElement(InvoiceStateBanner, {
					state: ORDER_INVOICE_STATE.ISSUED,
					notice: "發票已開立",
				}),
			),
		).toBe("");
	});

	it("內文逐字用後端的 stateNotice，前端不自組", () => {
		const markup = renderToStaticMarkup(
			createElement(InvoiceStateBanner, {
				state: ORDER_INVOICE_STATE.SELLER_SELF_ISSUED,
				notice: "發票號碼 AB12345678，由賣家開立與寄送。",
			}),
		);
		expect(markup).toContain("發票號碼 AB12345678，由賣家開立與寄送。");
	});
});

describe("DocumentTimeline", () => {
	it("事件名走 CodeMeta，且時間掛在 <time dateTime>", () => {
		const markup = renderToStaticMarkup(
			createElement(DocumentTimeline, {
				items: [
					{ occurredAt: "2026-08-30T02:00:00Z", event: "ISSUED", note: "" },
					{
						occurredAt: "2026-08-30T03:00:00Z",
						event: "VOIDED",
						note: "買家申請更正",
					},
				],
			}),
		);
		expect(markup).toContain("已開立");
		expect(markup).toContain("已作廢");
		// React 的 SSR 輸出保留 `dateTime` 的大小寫（HTML 屬性不分大小寫，瀏覽器
		// 一樣認得），所以這裡不分大小寫地比對，不是把斷言放寬。
		expect(markup.toLowerCase()).toContain('datetime="2026-08-30t02:00:00z"');
		expect(markup).toContain("買家申請更正");
	});

	it("後端加了第六種事件時退回原字串，不是整欄空白", () => {
		const markup = renderToStaticMarkup(
			createElement(DocumentTimeline, {
				items: [
					{ occurredAt: "2026-08-30T02:00:00Z", event: "REVOKED", note: "" },
				],
			}),
		);
		expect(markup).toContain("REVOKED");
	});
});
