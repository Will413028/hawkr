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
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "58",
			upperBoundAmount: "70",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		payloadHash: "a".repeat(64),
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
	fireEvent.change(screen.getByLabelText("希望下限", { exact: false }), {
		target: { value: "58" },
	});
	fireEvent.change(screen.getByLabelText("希望上限", { exact: false }), {
		target: { value: "70" },
	});
	fireEvent.click(screen.getByRole("button", { name: "取得無參考價試算" }));
}

describe("RfqDraftFlow — P-01 頂部需求摘要", () => {
	it("顯示品項、數量、配送城市，不顯示品相", async () => {
		await renderAtStep2(preview());

		await waitFor(() => {
			expect(screen.getByText("希望詢價區間")).toBeTruthy();
		});
		expect(screen.getByText("100 KG")).toBeTruthy();
		expect(screen.queryByText("品相")).toBeNull();
		expect(screen.queryByText("買家尚未指定")).toBeNull();
	});
});
