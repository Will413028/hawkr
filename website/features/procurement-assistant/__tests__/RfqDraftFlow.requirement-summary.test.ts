/** @vitest-environment jsdom */

// 同目錄其他測試已說明這個慣例：vitest 的 include 只吃 `.ts`，所以用
// React.createElement 而不是 JSX。

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
import type { RfqCommandResult, RfqPricePreview } from "../rfq-price-contract";

afterEach(cleanup);

const baseSlots = {
	item: "蘋果",
	quantity: 100,
	unit: "KG",
	deliveryCity: "臺北市",
	deliveryDate: "2026-08-28",
};

function preview(overrides: Partial<RfqPricePreview> = {}): RfqPricePreview {
	return {
		pricePreviewId: "1",
		version: "1",
		pricingMode: "PRICEBOOK_GUIDED",
		qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
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
		marketPriceBasis: null,
		quoteTotalRange: null,
		...overrides,
	} as RfqPricePreview;
}

async function renderAtStep2(data: RfqPricePreview) {
	render(
		React.createElement(RfqDraftFlow, {
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			candidateSetId: "candidate-set-unit",
			slots: baseSlots,
			candidates: [
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
			],
			onBackToCandidates: () => {},
			onReturnToConversation: () => {},
			onPreviewRfqPrice: vi.fn(
				async () => ({ ok: true, data }) as RfqCommandResult<RfqPricePreview>,
			),
			onConfirmRfqSnapshot: vi.fn(),
			onPreviewRfqPublication: vi.fn(),
			onExecuteRfqPublication: vi.fn(async () => {
				throw new Error("這個測試不應該送出發布");
			}),
			onRefreshSnapshot: async () => true,
			approvalEnabled: true,
		}),
	);
	fireEvent.click(screen.getByRole("button", { name: "查看預計名單" }));
	fireEvent.click(screen.getByRole("button", { name: "取得價格範圍" }));
}

// 稿 P-01 的頂部四格：品項／數量／配送城市／品相。前三格買家自己填過，第四格
// 是買家唯一看不到自己填過什麼的事實——系統把它記成「未指定」而不是默默補一個
// 等級，買家必須看得到這件事（docx Part V 總則）。
describe("RfqDraftFlow — P-01 頂部需求摘要", () => {
	it("顯示品項、數量、配送城市，以及「買家尚未指定」的品相", async () => {
		await renderAtStep2(preview());

		await waitFor(() => {
			expect(screen.getByText("買家尚未指定")).toBeTruthy();
		});
		expect(screen.getByText("品相")).toBeTruthy();
		expect(screen.getByText("100 KG")).toBeTruthy();
	});

	// 後端加這個欄位之前建立的 preview 回空字串。補成「買家尚未指定」等於替一份
	// 沒有記錄這件事的舊資料發明事實。
	it("品相為空字串時整格不顯示", async () => {
		await renderAtStep2(preview({ qualityKnowledgeState: "" }));

		// 先確定真的走到了價格確認這一步，否則「找不到品相」會是假綠。
		await waitFor(() => {
			expect(screen.getByText("參考行情與希望詢價")).toBeTruthy();
		});
		expect(screen.queryByText("買家尚未指定")).toBeNull();
		expect(screen.queryByText("品相")).toBeNull();
	});
});
