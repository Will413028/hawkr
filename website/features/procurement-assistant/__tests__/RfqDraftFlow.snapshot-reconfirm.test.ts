/** @vitest-environment jsdom */


import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import RfqDraftFlow from "../RfqDraftFlow";
import type { RfqCommandResult, RfqPricePreview } from "../rfq-price-contract";
import type {
	RfqPublicationApproval,
	RfqPublicationPreviewCommand,
} from "../rfq-publication-contract";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

type PublicationPreview = (
	command: RfqPublicationPreviewCommand,
) => Promise<RfqCommandResult<RfqPublicationApproval>>;

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
		buyerRequestedRange: {
			lowerBoundAmount: "58",
			upperBoundAmount: "70",
			basisQuantity: "1",
			basisUnit: "KG",
		},
		quoteTotalRange: null,
		...overrides,
	};
}

function approval(
	overrides: Partial<RfqPublicationApproval> = {},
): RfqPublicationApproval {
	return {
		approvalId: "1234567890123456803",
		approvalVersion: "1",
		audienceHash: "d".repeat(64),
		sellerFacingPayloadHash: "e".repeat(64),
		expiresAt: "2026-08-22T12:36:00Z",
		recipientCount: 1,
		...overrides,
	};
}

async function renderAtReconfirm(
	data: RfqPricePreview,
	onPreviewRfqPublication: PublicationPreview = vi.fn(
		async () =>
			({
				ok: true,
				data: approval(),
			}) as RfqCommandResult<RfqPublicationApproval>,
	),
) {
	render(
		React.createElement(RfqDraftFlow, {
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			candidateSetId: "1234567890123456790",
			slots: baseSlots,
			candidates: [
				{
					candidateId: "seller-green",
					displayName: "範例鮮果合作社",
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
			onRefreshSnapshot: async () => true,
			onPreviewRfqPublication,
			onExecuteRfqPublication: vi.fn(async () => {
				throw new Error("這個測試不應該送出發布");
			}),
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
	await waitFor(() => {
		expect(screen.getByText("希望詢價區間")).toBeTruthy();
	});
	fireEvent.click(
		screen.getByRole("checkbox", { name: /我已核對這份需求版本與價格範圍/ }),
	);
	fireEvent.click(screen.getByRole("button", { name: "檢視不可變 RFQ 快照" }));
	await waitFor(() => {
		expect(screen.getByText("重新確認 RFQ 快照")).toBeTruthy();
	});
}

describe("RfqDraftFlow — R-01 重新確認 RFQ 快照", () => {
	it("取得發布核准後快照追蹤顯示 snapshotHash 與有效至", async () => {
		await renderAtReconfirm(preview());
		expect(screen.queryByText("snapshotHash")).toBeNull();

		fireEvent.click(
			screen.getByRole("checkbox", {
				name: /我已重新檢查需求、希望價格與發布受眾/,
			}),
		);
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(screen.getByText("snapshotHash")).toBeTruthy();
		});
		expect(screen.getByText("有效至")).toBeTruthy();
		expect(screen.queryByText("pricePreviewId")).toBeNull();
	});

	it("不顯示稿上的快照版本 badge", async () => {
		await renderAtReconfirm(preview());

		expect(screen.queryByText(/快照版本/)).toBeNull();
	});

	it("未勾選重新檢查前無法準備發布", async () => {
		await renderAtReconfirm(preview());

		const cta = screen.getByRole("button", { name: "確認並準備發布" });
		expect(cta.hasAttribute("disabled")).toBe(true);

		fireEvent.click(
			screen.getByRole("checkbox", {
				name: /我已重新檢查需求、希望價格與發布受眾/,
			}),
		);
		expect(
			screen
				.getByRole("button", { name: "確認並準備發布" })
				.hasAttribute("disabled"),
		).toBe(false);
	});

	it("CTA 下方帶固定的發布語意說明", async () => {
		await renderAtReconfirm(preview());

		expect(
			screen.getByText(
				"這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。",
			),
		).toBeTruthy();
	});

	it("發布受眾是唯讀事實，只說明這份名單會收到", async () => {
		await renderAtReconfirm(preview());

		expect(screen.getByText("賣家 1 家")).toBeTruthy();
		expect(screen.getByText("只有這份名單上的候選會收到這筆詢價")).toBeTruthy();
		expect(screen.queryByRole("radiogroup")).toBeNull();
	});



	it("送出候選集與候選 id", async () => {
		const onPreviewRfqPublication = vi.fn(
			async () =>
				({
					ok: true,
					data: approval(),
				}) as RfqCommandResult<RfqPublicationApproval>,
		);
		await renderAtReconfirm(preview(), onPreviewRfqPublication);

		fireEvent.click(
			screen.getByRole("checkbox", {
				name: /我已重新檢查需求、希望價格與發布受眾/,
			}),
		);
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(onPreviewRfqPublication).toHaveBeenCalledTimes(1);
		});
		expect(onPreviewRfqPublication).toHaveBeenCalledWith(
			expect.objectContaining({
				candidateSetId: "1234567890123456790",
				selectedCandidateIds: ["seller-green"],
			}),
		);
	});

	it("取得核准後呈現伺服器的受眾事實，且不宣稱已發布", async () => {
		const onPreviewRfqPublication = vi.fn(
			async () =>
				({
					ok: true,
					data: approval(),
				}) as RfqCommandResult<RfqPublicationApproval>,
		);
		await renderAtReconfirm(preview(), onPreviewRfqPublication);

		fireEvent.click(
			screen.getByRole("checkbox", {
				name: /我已重新檢查需求、希望價格與發布受眾/,
			}),
		);
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(screen.getByText("1 位候選")).toBeTruthy();
		});
		expect(screen.queryByText(/已發布|已送出|已通知/)).toBeNull();
	});


	it("顯示事實變更會使確認失效的警示", async () => {
		await renderAtReconfirm(preview());

		expect(
			screen.getByText(
				"任一需求、價格、候選或受眾變更，都會使此確認失效並要求重新試算",
			),
		).toBeTruthy();
	});
});
