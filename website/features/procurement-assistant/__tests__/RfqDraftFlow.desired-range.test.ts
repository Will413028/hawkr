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

function preview(
	overrides: Partial<RfqPricePreview["buyerRequestedRange"]> = {},
): RfqPricePreview {
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
			...overrides,
		},
		payloadHash: "a".repeat(64),
		quoteTotalRange: null,
	};
}

function echoPreview(command: RfqPricePreviewCommand): RfqPricePreview {
	return preview({
		lowerBoundAmount: command.buyerRequestedRange.lowerBoundAmount,
		upperBoundAmount: command.buyerRequestedRange.upperBoundAmount,
	});
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

function fillRange(lower: string, upper: string) {
	fireEvent.change(screen.getByLabelText("希望下限", { exact: false }), {
		target: { value: lower },
	});
	fireEvent.change(screen.getByLabelText("希望上限", { exact: false }), {
		target: { value: upper },
	});
}

describe("RfqDraftFlow — 希望詢價區間", () => {
	it("進入價格步驟時不自動試算，欄位空白", async () => {
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			return {
				ok: true,
				data: echoPreview(command),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);

		await waitFor(() =>
			expect(screen.getByLabelText("希望下限", { exact: false })).toBeTruthy(),
		);
		expect(
			(screen.getByLabelText("希望下限", { exact: false }) as HTMLInputElement)
				.value,
		).toBe("");
		expect(
			(screen.getByLabelText("希望上限", { exact: false }) as HTMLInputElement)
				.value,
		).toBe("");
		expect(onPreviewRfqPrice).not.toHaveBeenCalled();
	});

	it("買家填入區間後取得試算，command 帶上 buyerRequestedRange 且沒有 pricingMode", async () => {
		const calls: RfqPricePreviewCommand[] = [];
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			calls.push(command);
			return {
				ok: true,
				data: echoPreview(command),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() =>
			expect(screen.getByLabelText("希望下限", { exact: false })).toBeTruthy(),
		);

		fillRange("60", "75");
		fireEvent.click(screen.getByRole("button", { name: "取得無參考價試算" }));

		await waitFor(() => expect(calls.length).toBe(1));
		expect(calls[0].buyerRequestedRange).toEqual({
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "60",
			upperBoundAmount: "75",
		});
		expect(calls[0]).not.toHaveProperty("pricingMode");
	});

	it("欄位留白（partial 或全空）時，按鈕停用且不會呼叫 preview", async () => {
		const onPreviewRfqPrice = vi.fn(async (command: RfqPricePreviewCommand) => {
			return {
				ok: true,
				data: echoPreview(command),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() =>
			expect(screen.getByLabelText("希望下限", { exact: false })).toBeTruthy(),
		);

		fillRange("60", "");

		expect(screen.getByText("請完整填寫希望下限與上限。")).toBeTruthy();
		const button = screen.getByRole("button", {
			name: "取得無參考價試算",
		}) as HTMLButtonElement;
		expect(button.disabled).toBe(true);

		fireEvent.click(button);

		expect(onPreviewRfqPrice).not.toHaveBeenCalled();
	});

	it("回應的區間與送出的不一致時顯示錯誤，不採用該回應", async () => {
		const onPreviewRfqPrice = vi.fn(async () => {
			return {
				ok: true,
				data: preview(),
			} as RfqCommandResult<RfqPricePreview>;
		});

		await renderAtStep2(onPreviewRfqPrice);
		await waitFor(() =>
			expect(screen.getByLabelText("希望下限", { exact: false })).toBeTruthy(),
		);

		fillRange("60", "75");
		fireEvent.click(screen.getByRole("button", { name: "取得無參考價試算" }));

		await waitFor(() =>
			expect(
				screen.getByText("取得的區間與本次設定不一致，請重試。"),
			).toBeTruthy(),
		);
	});
});
