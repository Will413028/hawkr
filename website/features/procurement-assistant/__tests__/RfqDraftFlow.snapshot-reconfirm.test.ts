/** @vitest-environment jsdom */

// 同目錄其他測試已說明這個慣例：vitest 的 include 只吃 `.ts`，所以用
// React.createElement 而不是 JSX。

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
	// P-01 的 CTA 逐字是「檢視不可變 RFQ 快照」（稿 P-01），前往 R-01。
	fireEvent.click(screen.getByRole("button", { name: "檢視不可變 RFQ 快照" }));
	await waitFor(() => {
		expect(screen.getByText("重新確認 RFQ 快照")).toBeTruthy();
	});
}

// docx §V 圖說寫死：「snapshot reconfirm heading 固定為『重新確認 RFQ 快照』」。
describe("RfqDraftFlow — R-01 重新確認 RFQ 快照", () => {
	// 快照追蹤只在取得發布核准之後出現：snapshotHash 與有效至都取自核准。
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

	// ADR 2026-09-12「confirmed snapshot 不加 version 欄」D2：稿上的「快照版本 v3」
	// badge 不實作，也不得改用 snapshot id 短碼或前端自行編號替代。
	it("不顯示稿上的快照版本 badge", async () => {
		await renderAtReconfirm(preview());

		expect(screen.queryByText(/快照版本/)).toBeNull();
	});

	// 稿 R-01：未勾選時「確認並準備發布」disabled。
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

	// docx §V 圖說：CTA 下固定說明，逐字不可改寫。
	it("CTA 下方帶固定的發布語意說明", async () => {
		await renderAtReconfirm(preview());

		expect(
			screen.getByText(
				"這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。",
			),
		).toBeTruthy();
	});

	// 稿 R-01 ④。docx V.7.5：publicationAudience 是互斥的 discriminator，
	// 受眾只有直接邀請，沒有可切換的模式。
	it("發布受眾是唯讀事實，只說明這份名單會收到", async () => {
		await renderAtReconfirm(preview());

		expect(screen.getByText("賣家 1 家")).toBeTruthy();
		expect(screen.getByText("只有這份名單上的候選會收到這筆詢價")).toBeTruthy();
		expect(screen.queryByRole("radiogroup")).toBeNull();
	});

	// 候選家數取自這張詢價單實際帶進來的候選，前端不自行編造數字。

	// docx §V 圖說：「勾選確認後先以『確認並準備發布』取得 immutable RFQ
	// Publication Approval」。這一步不建立發布、不通知任何賣家。

	// 順序有意義：audienceHash 涵蓋它，換順序等於換受眾。
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

	// ⛔ 取得核准 ≠ 已發布。docx §V 圖說：只有 server 回傳可執行 approval 之後，
	// 最終 execute CTA 才出現——這裡只呈現 server 回的受眾事實，不得出現任何宣稱已送出的字樣。
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

	// 警示條承諾「任一…受眾變更，都會使此確認失效」。既有核准描述的是舊受眾，
	// 沿用它等於讓買家以為自己核准過現在畫面上這一份。

	// 稿 R-01 的警示條：任一事實變更都會使確認失效。
	it("顯示事實變更會使確認失效的警示", async () => {
		await renderAtReconfirm(preview());

		expect(
			screen.getByText(
				"任一需求、價格、候選或受眾變更，都會使此確認失效並要求重新試算",
			),
		).toBeTruthy();
	});
});
