/** @vitest-environment jsdom */

// 同目錄 DealTemperature.test.ts 已經解釋過這個慣例：vitest.config.ts 的
// include 只吃 `**/__tests__/**/*.test.ts`（不吃 `.tsx`），所以這裡用
// `React.createElement` 而不是 JSX，檔名維持 `.ts`。
//
// 本檔釘住 v6.0 §V.11.3 圖 P-01 的版面契約：參考區間是一條數線（不是三格
// 堆疊卡）、meta 行揭露資料截至與映射、三張卡的順序是 參考區間 → 希望詢價
// 區間 → 成交溫度計。**「信心」不在其中**——wire 上沒有這個欄位（見
// ADR 2026-09-09-deal-temperature-v1-coverage-not-outcome 的 Followup），
// 前端不得自行捏一個等級出來。

import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RfqDraftFlow from "../RfqDraftFlow";
import type {
	RfqCommandResult,
	RfqPricePreview,
	RfqPricePreviewCommand,
} from "../rfq-price-contract";

afterEach(cleanup);

const REQUEST_ID = "1234567890123456788";
const REVISION_ID = "1234567890123456791";

const baseSlots = {
	item: "珍珠芭樂",
	quantity: 50,
	unit: "KG",
	deliveryCity: "桃園市",
	deliveryDate: "2026-09-01",
};

const candidates = [
	{
		candidateId: "seller-green",
		sourceId: "product-green",
		lane: "PLATFORM" as const,
		displayName: "範例鮮果合作社",
		score: 94,
		matchedFacts: [],
		unverifiableFacts: [],
		factsAsOf: "2026-08-14T12:00:00Z",
		selected: false,
	},
];

// 52／61／74 對齊稿 P-01 上的三個數字。
function guidedPreview(
	overrides: Partial<RfqPricePreview> = {},
): RfqPricePreview {
	return {
		pricePreviewId: "1",
		version: "1",
		pricingMode: "PRICEBOOK_GUIDED",
		referenceAvailability: "AVAILABLE",
		referencePriceRange: {
			lowerBoundAmount: "52",
			midpointAmount: "61",
			upperBoundAmount: "74",
		},
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "58",
			upperBoundAmount: "70",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		dealTemperature: { status: "UNKNOWN", reasonCode: "BUYER_RANGE_NOT_SET" },
		payloadHash: "a".repeat(64),
		marketPriceBasis: {
			regionName: "桃園市",
			supportingMarketCount: "3",
			sourceDataAsOf: "2026-08-14T00:00:00Z",
			freshnessStatus: "fresh",
		},
		quoteTotalRange: null,
		...overrides,
	} as RfqPricePreview;
}

// 元件會擋下 pricingMode 與請求不符的回應，而價格方式的 radio 只在 step 2 才
// 存在，所以要拿到 no-reference 的 preview，得走真實流程：先取得一次 guided
// 試算進到 step 2，再切模式、重新試算——不能只換 mock 的回傳值。
async function renderAtStep2(
	respond: (command: RfqPricePreviewCommand) => RfqPricePreview = () =>
		guidedPreview(),
	mode: "PRICEBOOK_GUIDED" | "BUYER_ENTERED_NO_REFERENCE" = "PRICEBOOK_GUIDED",
) {
	const onPreviewRfqPrice = vi.fn(
		async (command: RfqPricePreviewCommand) =>
			({
				ok: true,
				data: respond(command),
			}) as RfqCommandResult<RfqPricePreview>,
	);
	const result = render(
		React.createElement(RfqDraftFlow, {
			requestId: REQUEST_ID,
			revisionId: REVISION_ID,
			candidateSetId: "candidate-set-unit",
			slots: baseSlots,
			candidates,
			onBackToCandidates: () => {},
			onReturnToConversation: () => {},
			onPreviewRfqPrice: onPreviewRfqPrice as unknown as (
				command: RfqPricePreviewCommand,
			) => Promise<RfqCommandResult<RfqPricePreview>>,
			onConfirmRfqSnapshot: vi.fn(),
			onPreviewRfqPublication: vi.fn(),
			onExecuteRfqPublication: vi.fn(async () => {
				throw new Error("這個測試不應該送出發布");
			}),
			onRefreshSnapshot: async () => true,
			approvalEnabled: true,
		}),
	);
	screen.getByRole("button", { name: "查看預計名單" }).click();
	await waitFor(() =>
		expect(screen.getByRole("button", { name: "取得價格範圍" })).toBeTruthy(),
	);
	screen.getByRole("button", { name: "取得價格範圍" }).click();
	await waitFor(() => expect(onPreviewRfqPrice).toHaveBeenCalledTimes(1));
	if (mode === "BUYER_ENTERED_NO_REFERENCE") {
		await waitFor(() =>
			expect(screen.getByRole("radio", { name: /自行設定價格/ })).toBeTruthy(),
		);
		fireEvent.click(screen.getByRole("radio", { name: /自行設定價格/ }));
		fireEvent.click(screen.getByRole("button", { name: "取得無參考價試算" }));
		await waitFor(() => expect(onPreviewRfqPrice).toHaveBeenCalledTimes(2));
	}
	return result;
}

describe("RfqDraftFlow — 參考區間數線（稿 P-01）", () => {
	it("三個刻度用稿上的標籤，中位刻度依三個金額定位", async () => {
		const { container } = await renderAtStep2();

		await waitFor(() => expect(screen.getByText("區間下限")).toBeTruthy());
		expect(screen.getByText("中位數")).toBeTruthy();
		expect(screen.getByText("區間上限")).toBeTruthy();
		expect(screen.getByText("NT$52")).toBeTruthy();
		expect(screen.getByText("NT$61")).toBeTruthy();
		expect(screen.getByText("NT$74")).toBeTruthy();

		// (61 - 52) / (74 - 52) = 40.9%
		const mark = container.querySelector(
			".assistant-rfq-guidance__reference-mark",
		) as HTMLElement;
		expect(mark.style.left).toBe("40.9%");
	});

	it("沒有參考區間時不畫數線，也不編造刻度", async () => {
		const { container } = await renderAtStep2(
			(command) =>
				command.pricingMode === "BUYER_ENTERED_NO_REFERENCE"
					? guidedPreview({
							pricingMode: "BUYER_ENTERED_NO_REFERENCE",
							referenceAvailability: "UNAVAILABLE",
							referencePriceRange: null,
							marketPriceBasis: null,
						})
					: guidedPreview(),
			"BUYER_ENTERED_NO_REFERENCE",
		);

		await waitFor(() => expect(screen.getAllByText("無參考")).toHaveLength(2));
		expect(
			container.querySelector(".assistant-rfq-guidance__reference-mark"),
		).toBeNull();
	});
});

describe("RfqDraftFlow — 行情出處 meta 行（稿 P-01）", () => {
	it("揭露資料截至與映射（品項／區域），並保留重量計價基準", async () => {
		await renderAtStep2();

		await waitFor(() => expect(screen.getByText("資料截至")).toBeTruthy());
		expect(screen.getByText("2026/08/14")).toBeTruthy();
		expect(screen.getByText("映射")).toBeTruthy();
		expect(screen.getByText("珍珠芭樂／桃園市")).toBeTruthy();
		expect(screen.getByText("3 個批發市場・每 1 KG")).toBeTruthy();
	});

	// wire 上沒有 confidence 欄位（`api/proto/pricebook/` 零命中）。稿上的
	// 「信心：中」在補上真欄位之前不得出現——捏一個等級比不顯示更糟。
	it("不顯示稿上的「信心」——wire 上沒有這個欄位", async () => {
		const { container } = await renderAtStep2();

		await waitFor(() => expect(screen.getByText("資料截至")).toBeTruthy());
		expect(container.textContent).not.toContain("信心");
	});
});

describe("RfqDraftFlow — 版面順序（稿 P-01）", () => {
	it("參考區間 → 希望詢價區間 → 成交溫度計", async () => {
		const { container } = await renderAtStep2();

		await waitFor(() => expect(screen.getByText("區間下限")).toBeTruthy());
		const reference = container.querySelector(
			".assistant-rfq-guidance__price",
		) as HTMLElement;
		const desired = container.querySelector(
			".assistant-rfq-guidance__desired",
		) as HTMLElement;
		const temperature = container.querySelector(
			".assistant-rfq-guidance__temperature",
		) as HTMLElement;

		expect(
			reference.compareDocumentPosition(desired) &
				Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
		expect(
			desired.compareDocumentPosition(temperature) &
				Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
	});
});
