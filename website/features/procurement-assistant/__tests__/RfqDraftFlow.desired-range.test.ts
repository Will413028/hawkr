/** @vitest-environment jsdom */
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

function guidedPreview(
	overrides: Partial<RfqPricePreview["buyerRequestedRange"]> = {},
): RfqPricePreview {
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
			basisQuantity: "1",
			basisUnit: "KG",
		},
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "58",
			upperBoundAmount: "70",
			basisQuantity: "1",
			basisUnit: "KG",
			...overrides,
		},
		dealTemperature: { status: "UNKNOWN", reasonCode: "BUYER_RANGE_NOT_SET" },
		payloadHash: "a".repeat(64),
		marketPriceBasis: null,
		quoteTotalRange: null,
	};
}

async function renderAtStep2(
	onPreviewRfqPrice: (
		command: RfqPricePreviewCommand,
	) => Promise<RfqCommandResult<RfqPricePreview>>,
) {
	render(
		React.createElement(RfqDraftFlow, {
			requestId: REQUEST_ID,
			revisionId: REVISION_ID,
			candidateSetId: "candidate-set-unit",
			slots: baseSlots,
			candidates,
			onBackToCandidates: () => {},
			onReturnToConversation: () => {},
			onPreviewRfqPrice,
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

describe("RfqDraftFlow — guided 模式的希望詢價區間", () => {
	it("第一次 preview（欄位空白）省略 buyerRequestedRange，交由 server 用參考區間預填", async () => {
		const calls: RfqPricePreviewCommand[] = [];
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			calls.push(command);
			return {
				ok: true,
				data: guidedPreview(),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);

		await waitFor(() => expect(calls.length).toBe(1));
		expect(calls[0].pricingMode).toBe("PRICEBOOK_GUIDED");
		expect(calls[0]).not.toHaveProperty("buyerRequestedRange");

		await waitFor(() => {
			expect(
				(
					screen.getByLabelText("希望下限", {
						exact: false,
					}) as HTMLInputElement
				).value,
			).toBe("58");
			expect(
				(
					screen.getByLabelText("希望上限", {
						exact: false,
					}) as HTMLInputElement
				).value,
			).toBe("70");
		});
	});

	it("買家調整區間後按「重新試算」，command 帶上編輯後的 buyerRequestedRange", async () => {
		const calls: RfqPricePreviewCommand[] = [];
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			calls.push(command);
			if (command.buyerRequestedRange) {
				return {
					ok: true,
					data: guidedPreview({
						lowerBoundAmount: command.buyerRequestedRange.lowerBoundAmount,
						upperBoundAmount: command.buyerRequestedRange.upperBoundAmount,
					}),
				} as RfqCommandResult<RfqPricePreview>;
			}
			return {
				ok: true,
				data: guidedPreview(),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() => expect(calls.length).toBe(1));
		await waitFor(() =>
			expect(
				(
					screen.getByLabelText("希望下限", {
						exact: false,
					}) as HTMLInputElement
				).value,
			).toBe("58"),
		);

		fireEvent.change(screen.getByLabelText("希望下限", { exact: false }), {
			target: { value: "60" },
		});
		fireEvent.change(screen.getByLabelText("希望上限", { exact: false }), {
			target: { value: "75" },
		});
		fireEvent.click(screen.getByRole("button", { name: "重新試算" }));

		await waitFor(() => expect(calls.length).toBe(2));
		expect(calls[1].buyerRequestedRange).toEqual({
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "60",
			upperBoundAmount: "75",
		});
	});

	it("欄位留白（partial 或全空）時，重新試算不會送出 buyerRequestedRange 也不會呼叫 preview", async () => {
		const calls: RfqPricePreviewCommand[] = [];
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			calls.push(command);
			return {
				ok: true,
				data: guidedPreview(),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() => expect(calls.length).toBe(1));
		await waitFor(() =>
			expect(
				(
					screen.getByLabelText("希望下限", {
						exact: false,
					}) as HTMLInputElement
				).value,
			).toBe("58"),
		);

		fireEvent.change(screen.getByLabelText("希望下限", { exact: false }), {
			target: { value: "" },
		});

		expect(screen.getByText("請完整填寫希望下限與上限。")).toBeTruthy();
		expect(
			(screen.getByRole("button", { name: "重新試算" }) as HTMLButtonElement)
				.disabled,
		).toBe(true);

		fireEvent.click(screen.getByRole("button", { name: "重新試算" }));

		expect(calls.length).toBe(1);
	});

	it("guided preview 成功後，重新算一次只有唯一一個觸發點（沒有重複按鈕）", async () => {
		const onPreviewRfqPrice = vi.fn(async () => {
			return {
				ok: true,
				data: guidedPreview(),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() => expect(onPreviewRfqPrice).toHaveBeenCalledTimes(1));
		await waitFor(() =>
			expect(
				(
					screen.getByLabelText("希望下限", {
						exact: false,
					}) as HTMLInputElement
				).value,
			).toBe("58"),
		);

		expect(
			screen.queryAllByRole("button", { name: "重新查詢行情" }),
		).toHaveLength(0);
		expect(screen.getAllByRole("button", { name: "重新試算" })).toHaveLength(1);
	});
});
