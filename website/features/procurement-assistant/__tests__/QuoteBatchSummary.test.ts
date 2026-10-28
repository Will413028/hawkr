/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render,
	screen,
	within,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ProcurementSlots } from "@/lib/api/openapi-types";
import type { BuyerQuoteAcceptanceResponse } from "../buyer-quote-contract";
import QuoteBatchSummary from "../QuoteBatchSummary";
import {
	type PublicationResultView,
	publicationResultDataSchema,
} from "../runtime-snapshot-contract";
import type { RfqDealTruthCounts } from "../useRfqRuntimeSnapshot";

/**
 * 稿 R-02 三區版面（issue #2538 A8 後半）。
 *
 * 這支測試的重心是**空狀態不得被藏起來或美化**：第三張卡是 v6.0 §V.11.4
 * 「Delivery truth」的視覺落地，沒有回價就得誠實顯示 0 筆、成交價格尚未成立、
 * 訂單尚未建立。順帶鎖住 D3：`CREATING_ORDER` 不得被寫成已成交或已下單。
 */

afterEach(cleanup);

/** 稿 R-02 ①的需求來源。legacy `QuoteBatch` 已從這個面板下架。 */
const slots: ProcurementSlots = {
	item: "蘋果",
	quantity: 100,
	unit: "公斤",
	deliveryCity: "臺北市",
	deliveryDate: "2026-08-25",
};

function publicationWith(
	counts: Partial<{
		recipientCount: number;
		queuedCount: number;
		notSentCount: number;
	}> = {},
): PublicationResultView {
	return {
		artifactVersion: "3",
		publishedAt: "2026-08-20T12:36:00Z",
		data: publicationResultDataSchema.parse({
			publicationId: "1234567890123456788",
			version: "1",
			sellerFacingPayloadHash: "b".repeat(64),
			recipientCount: 0,
			queuedCount: 0,
			notSentCount: 0,
			...counts,
		}),
	};
}

const publication = publicationWith();

const zeroCounts: RfqDealTruthCounts = {
	responseCount: 0,
	shortlistCount: 0,
	error: null,
};

const unknownCounts: RfqDealTruthCounts = {
	responseCount: null,
	shortlistCount: null,
	error: null,
};

function ack(
	overrides: Partial<BuyerQuoteAcceptanceResponse> = {},
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: "1234567890123456805",
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		acceptedDealPrice: "18900",
		currency: "TWD",
		acceptedAt: "2026-08-22T06:00:00Z",
		orderProvisioningState: "CREATING_ORDER",
		orderNo: "",
		paymentDeadline: "",
		replayed: false,
		...overrides,
	};
}

type Props = React.ComponentProps<typeof QuoteBatchSummary>;

function mount(overrides: Partial<Props> = {}) {
	const props: Props = {
		slots,
		qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		publication,
		counts: zeroCounts,
		acceptance: null,
		onViewSnapshot: vi.fn(),
		onCompareQuotes: vi.fn(),
		...overrides,
	};
	return {
		props,
		...render(React.createElement(QuoteBatchSummary, props)),
	};
}

/** 讀「目前交易真相」的某一列（dt 與 dd 同在一個 row 容器裡）。 */
function truthValue(label: string): string {
	const term = screen.getByText(label);
	const row = term.parentElement as HTMLElement;
	return within(row).getByRole("definition").textContent ?? "";
}

describe("① 詢價快照", () => {
	it("顯示結果版本並標明不可變", () => {
		mount();
		const version = screen.getByText("結果版本").parentElement as HTMLElement;
		expect(version.textContent).toContain("v3");
		expect(version.textContent).toContain("不可變");
	});

	it("逐字帶出回價須知那一行", () => {
		mount();
		expect(
			screen.getByText(
				"供應商回價時須提交實際品相、驗收條件、計價單位、商品／運費／稅額／總額與有效期限。",
			),
		).toBeTruthy();
	});

	it("品相顯示「買家尚未指定」，不補預設等級", () => {
		mount();
		const quality = screen.getByText("品相").parentElement as HTMLElement;
		expect(quality.textContent).toContain("買家尚未指定");
	});

	// budgetPerUnit 是買家私有預算（v6.0 §V.7 不得等同 buyerRequestedRange），
	// 沒有伺服器給的希望詢價區間時整列不顯示，不得拿任何東西頂替。
	it("沒有希望詢價區間就不顯示那一列，也不拿買家預算頂替", () => {
		mount({ slots: { ...slots, budgetPerUnit: 70 } });
		expect(screen.queryByText("希望詢價")).toBeNull();
		expect(screen.queryByText(/NT\$70/)).toBeNull();
	});

	it("伺服器給了區間才顯示", () => {
		mount({
			buyerRequestedRange: {
				currency: "TWD",
				priceComponent: "GOODS_ONLY",
				lowerBoundAmount: "58",
				upperBoundAmount: "70",
				basisQuantity: "1",
				basisUnit: "公斤",
			},
		});
		const row = screen.getByText("希望詢價").parentElement as HTMLElement;
		expect(row.textContent).toContain("NT$58–NT$70");
	});

	it("沒有發布事實時標題退回「詢價建立摘要」並保留未核准揭露", () => {
		mount({ publication: null });
		expect(screen.getByRole("heading", { name: "詢價建立摘要" })).toBeTruthy();
		expect(
			screen.getByText("後端未提供不可變核准證據；此畫面不代表核准已完成。"),
		).toBeTruthy();
		expect(screen.queryByText("結果版本")).toBeNull();
	});

	it("有發布事實時標題是「詢價已發布」，並撤下未核准揭露", () => {
		mount();
		expect(screen.getByRole("heading", { name: "詢價已發布" })).toBeTruthy();
		expect(
			screen.queryByText("後端未提供不可變核准證據；此畫面不代表核准已完成。"),
		).toBeNull();
	});

	it("保留既有的送達真相揭露", () => {
		mount();
		expect(
			screen.getByText("通知已排入處理；不代表通知或聯絡已送達。"),
		).toBeTruthy();
	});

	it("標題那句的邀請筆數來自 recipientCount，數量來自 slots", () => {
		mount({ publication: publicationWith({ recipientCount: 5 }) });
		expect(
			screen.getByText("已建立 1 組詢價・5 筆邀請（100 公斤）"),
		).toBeTruthy();
	});

	it("數量與配送都來自 slots；缺席就整列不顯示", () => {
		mount({ slots: { item: "蘋果" } });
		expect(screen.queryByText("數量")).toBeNull();
		expect(screen.queryByText("配送")).toBeNull();
		const item = screen.getByText("品項").parentElement as HTMLElement;
		expect(item.textContent).toContain("蘋果");
	});

	// 回覆期限的買家側來源隨 legacy quoteDeadline 一起下架；deliveryDate 是
	// 交期不是回覆期限，不得拿它頂替。
	it("沒有「截止」那一列，也不拿交期頂替", () => {
		mount();
		expect(screen.queryByText("截止")).toBeNull();
		const delivery = screen.getByText("配送").parentElement as HTMLElement;
		expect(delivery.textContent).toContain("2026/08/25");
	});
});

describe("② 流程進度", () => {
	function stepStates(): Array<string | null> {
		return screen
			.getAllByRole("listitem")
			.filter((node) => node.dataset.step)
			.map((node) => node.dataset.state ?? null);
	}

	it("六步驟齊備，已發布時停在等待回價", () => {
		mount();
		expect(stepStates()).toEqual([
			"done",
			"done",
			"current",
			"pending",
			"pending",
			"pending",
		]);
		expect(screen.getByText("等待供應商正式回價")).toBeTruthy();
	});

	it("已發布那一步帶伺服器時間戳，已確認快照那一步沒有時間可寫", () => {
		mount();
		const steps = screen
			.getAllByRole("listitem")
			.filter((node) => node.dataset.step);
		expect(steps[0].textContent).toContain("已完成");
		expect(steps[1].textContent).toContain("2026/08/20 20:36");
	});

	it("進行中那一步標了 aria-current", () => {
		mount();
		const current = screen
			.getAllByRole("listitem")
			.filter((node) => node.dataset.state === "current");
		expect(current).toHaveLength(1);
		expect(current[0].getAttribute("aria-current")).toBe("step");
	});

	it("付款那一步永遠不會標成已完成", () => {
		mount({
			counts: { responseCount: 2, shortlistCount: 1, error: null },
			acceptance: ack({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260822001",
				paymentDeadline: "2026-08-23T06:00:00Z",
			}),
		});
		expect(stepStates()[5]).toBe("pending");
	});
});

describe("③ 目前交易真相", () => {
	it("沒有回價就誠實顯示 0 筆、尚未成立、尚未建立", () => {
		mount();
		expect(truthValue("已收到正式回價")).toBe("0 筆");
		expect(truthValue("shortlist")).toBe("0");
		expect(truthValue("成交價格")).toBe("尚未成立");
		expect(truthValue("訂單")).toBe("尚未建立");
	});

	it("空狀態不得被藏起來或換成樂觀文案", () => {
		mount();
		const panel = screen.getByTestId("quote-batch-summary");
		expect(panel.textContent).not.toMatch(/已成交|已下單|訂單成立|即將收到/);
	});

	it("筆數還沒讀到時顯示「尚未取得」，不得當成 0", () => {
		mount({ counts: unknownCounts });
		expect(truthValue("已收到正式回價")).toBe("尚未取得");
		expect(truthValue("shortlist")).toBe("尚未取得");
	});

	it("清單讀失敗時把原因講出來，不留一個沒來由的空格", () => {
		mount({
			counts: {
				responseCount: null,
				shortlistCount: null,
				error: {
					code: "SERVER_ERROR",
					detail: "伺服器暫時無法處理，請稍後再試。",
					retryable: true,
				},
			},
		});
		expect(screen.getByText("伺服器暫時無法處理，請稍後再試。")).toBeTruthy();
	});

	it("CREATING_ORDER 只說「建立中」，不得寫成已成交或已下單", () => {
		mount({
			counts: { responseCount: 1, shortlistCount: 1, error: null },
			acceptance: ack(),
		});
		expect(truthValue("成交價格")).toBe("NT$18,900");
		expect(truthValue("訂單")).toBe("建立中");
		const panel = screen.getByTestId("quote-batch-summary");
		expect(panel.textContent).not.toMatch(/已成交|已下單/);
	});

	it("ORDER_CREATED 才出現訂單編號", () => {
		mount({
			counts: { responseCount: 1, shortlistCount: 1, error: null },
			acceptance: ack({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260822001",
				paymentDeadline: "2026-08-23T06:00:00Z",
			}),
		});
		expect(truthValue("訂單")).toBe("HK20260822001");
	});

	it("ORDER_FAILED 說「未建立成功」，不裝作還在建立", () => {
		mount({
			counts: { responseCount: 1, shortlistCount: 1, error: null },
			acceptance: ack({ orderProvisioningState: "ORDER_FAILED" }),
		});
		expect(truthValue("訂單")).toBe("未建立成功");
	});

	it("固定講明參考區間與希望價格都不是成交價格", () => {
		mount();
		const panel = screen.getByTestId("quote-batch-summary");
		expect(panel.textContent).toContain("買家希望價格不是成交價格");
	});
});

describe("CTA", () => {
	it("沒有回價時比較按鈕鎖住，並說明為什麼", () => {
		mount();
		expect(
			screen
				.getByRole("button", { name: "比較供應商回價" })
				.hasAttribute("disabled"),
		).toBe(true);
		expect(
			screen.getByText("目前沒有可比較的正式回價，收到之後這顆按鈕才會開啟。"),
		).toBeTruthy();
	});

	it("有回價才開啟比較", () => {
		const onCompareQuotes = vi.fn();
		mount({
			counts: { responseCount: 2, shortlistCount: 0, error: null },
			onCompareQuotes,
		});
		const button = screen.getByRole("button", { name: "比較供應商回價" });
		expect(button.hasAttribute("disabled")).toBe(false);
		fireEvent.click(button);
		expect(onCompareQuotes).toHaveBeenCalledTimes(1);
	});

	it("沒有發布事實時快照 CTA 鎖住，不連到一個不存在的快照", () => {
		mount({ publication: null });
		expect(
			screen
				.getByRole("button", { name: /檢視不可變 RFQ 快照/ })
				.hasAttribute("disabled"),
		).toBe(true);
	});
});

/**
 * legacy `QuoteBatch` 下架後這一區塊改吃 publication 的三個 count。
 *
 * ⛔ 三個 count 是發布當下的意圖分布，不是投遞結果（後端刻意不放
 * `sentCount`），所以 `queuedCount` 不得被講成「已送出」。
 */
describe("這次發布的收件人", () => {
	function factValue(label: string): string {
		const term = screen.getByText(label);
		const row = term.parentElement as HTMLElement;
		return within(row).getByRole("definition").textContent ?? "";
	}

	it("三個 count 逐項顯示，WISH_POOL 的 0 照樣顯示而不是整區藏起來", () => {
		mount();
		expect(factValue("收件人")).toBe("0");
		expect(factValue("已排入處理")).toBe("0");
		expect(factValue("未排入處理")).toBe("0");
	});

	it("DIRECT_RECIPIENTS 的分布逐項對上", () => {
		mount({
			publication: publicationWith({
				recipientCount: 5,
				queuedCount: 4,
				notSentCount: 1,
			}),
		});
		expect(factValue("收件人")).toBe("5");
		expect(factValue("已排入處理")).toBe("4");
		expect(factValue("未排入處理")).toBe("1");
	});

	it("queuedCount 不得被呈現成投遞結果", () => {
		mount({
			publication: publicationWith({ recipientCount: 5, queuedCount: 4 }),
		});
		const panel = screen.getByTestId("quote-batch-summary");
		// 「已送達」只准出現在既有那句否定式界線句（「不代表通知或聯絡已送達」）
		// 裡，所以這裡擋的是肯定式的說法。
		expect(panel.textContent).not.toMatch(/已送出|已寄送|已通知/);
		expect(
			screen.getByText("這三個數字是發布當下的收件人分布，不是投遞結果。"),
		).toBeTruthy();
	});

	it("沒有發布事實時整個區塊不顯示，不顯示 0", () => {
		mount({ publication: null });
		expect(screen.queryByText("這次發布的收件人")).toBeNull();
		expect(screen.queryByText("收件人")).toBeNull();
		expect(screen.queryByText(/已建立 1 組詢價/)).toBeNull();
	});
});

/**
 * ⛔ 撤回入口已從這個面板移除：背後的 `quote-batches/{id}/withdraw` 是 doc 64
 * §7 明列退場的 legacy 端點，publication 側沒有對應的撤回。這一則擋的是「順手
 * 補一顆停用的按鈕」——那等於宣稱功能還在。
 */
describe("撤回入口", () => {
	it("面板上沒有任何撤回動作", () => {
		mount();
		const panel = screen.getByTestId("quote-batch-summary");
		expect(panel.textContent).not.toMatch(/撤回|撤銷|取消發布/);
		expect(screen.queryByRole("button", { name: /撤回/ })).toBeNull();
	});
});
