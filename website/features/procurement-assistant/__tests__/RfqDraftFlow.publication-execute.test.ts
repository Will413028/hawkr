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
import type { ProcurementCandidate } from "@/lib/api/openapi-types";
import RfqDraftFlow from "../RfqDraftFlow";
import type {
	RfqCommandResult,
	RfqConfirmation,
	RfqPricePreview,
} from "../rfq-price-contract";
import type {
	RfqPublicationApproval,
	RfqPublicationExecuteCommand,
	RfqPublicationResult,
} from "../rfq-publication-contract";

type PublicationExecute = (
	command: RfqPublicationExecuteCommand,
) => Promise<RfqCommandResult<RfqPublicationResult>>;

afterEach(cleanup);

const baseSlots = {
	item: "蘋果",
	quantity: 100,
	unit: "KG",
	deliveryCity: "臺北市",
	deliveryDate: "2026-08-28",
};

function preview(): RfqPricePreview {
	return {
		pricePreviewId: "1234567890123456799",
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
	} as RfqPricePreview;
}

function confirmation(): RfqConfirmation {
	return {
		confirmedRfqSnapshotId: "1234567890123456802",
		payloadHash: "c".repeat(64),
		expiresAt: "2026-08-22T12:36:00Z",
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
		sellerFacingPayloadJson: '{"item":"蘋果"}',
		expiresAt: "2026-08-22T12:36:00Z",
		audiencePolicyId: "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
		audiencePolicyVersion: "1",
		recipientCount: "0",
		...overrides,
	};
}

function result(
	overrides: Partial<RfqPublicationResult> = {},
): RfqPublicationResult {
	return {
		publicationId: "1234567890123456804",
		publicationVersion: "1",
		state: "PUBLISHED",
		audienceMode: "DIRECT_RECIPIENTS",
		audienceHash: "d".repeat(64),
		sellerFacingPayloadHash: "e".repeat(64),
		publishedAt: "2026-08-22T12:40:00Z",
		replayed: false,
		...overrides,
	};
}

function laneCandidate(
	id: string,
	lane: "PLATFORM" | "EXTERNAL",
	storeId?: string,
): ProcurementCandidate {
	return {
		candidateId: id,
		sourceId: `product-${id}`,
		lane,
		displayName: `候選 ${id}`,
		score: 90,
		matchedFacts: [],
		unverifiableFacts: [],
		factsAsOf: "2026-08-14T12:00:00Z",
		selected: false,
		...(storeId ? { storeId } : {}),
	};
}

function okExecute(data: RfqPublicationResult = result()): PublicationExecute {
	return vi.fn(
		async () => ({ ok: true, data }) as RfqCommandResult<RfqPublicationResult>,
	);
}

async function renderAtReconfirm(
	onExecuteRfqPublication: PublicationExecute,
	options: {
		approval?: RfqPublicationApproval;
		candidates?: ProcurementCandidate[];
	} = {},
) {
	render(
		React.createElement(RfqDraftFlow, {
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			candidateSetId: "1234567890123456790",
			slots: baseSlots,
			candidates: options.candidates ?? [
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
				async () =>
					({ ok: true, data: preview() }) as RfqCommandResult<RfqPricePreview>,
			),
			onConfirmRfqSnapshot: vi.fn(
				async () =>
					({
						ok: true,
						data: confirmation(),
					}) as RfqCommandResult<RfqConfirmation>,
			),
			onRefreshSnapshot: async () => true,
			onPreviewRfqPublication: vi.fn(
				async () =>
					({
						ok: true,
						data: options.approval ?? approval(),
					}) as RfqCommandResult<RfqPublicationApproval>,
			),
			onExecuteRfqPublication,
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

async function obtainApproval(cta = "確認並送出邀請") {
	fireEvent.click(
		screen.getByRole("checkbox", {
			name: /我已重新檢查需求、希望價格與發布受眾/,
		}),
	);
	fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));
	await waitFor(() => {
		expect(screen.getByRole("button", { name: cta })).toBeTruthy();
	});
}

describe("RfqDraftFlow — A13 execute CTA「確認並送出邀請」", () => {
	it("取得可執行核准前不顯示 execute CTA", async () => {
		await renderAtReconfirm(okExecute());

		expect(screen.queryByRole("button", { name: "確認並送出邀請" })).toBeNull();
	});

	it("取得核准後顯示 execute CTA", async () => {
		await renderAtReconfirm(okExecute());
		await obtainApproval();

		expect(screen.getByRole("button", { name: "確認並送出邀請" })).toBeTruthy();
	});

	it("固定說明改由 execute CTA 描述", async () => {
		await renderAtReconfirm(okExecute());
		await obtainApproval();

		const cta = screen.getByRole("button", { name: "確認並送出邀請" });
		const described = document.getElementById(
			cta.getAttribute("aria-describedby") ?? "",
		);
		expect(described?.textContent).toBe(
			"這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。",
		);
	});

	it("送出時帶 approval 的 CAS token 與 revision", async () => {
		const execute = okExecute();
		await renderAtReconfirm(execute);
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));

		await waitFor(() => {
			expect(execute).toHaveBeenCalledTimes(1);
		});
		const command = vi.mocked(execute).mock.calls[0][0];
		expect(command.approval.approvalId).toBe("1234567890123456803");
		expect(command.approval.approvalVersion).toBe("1");
		expect(command.expectedRevisionId).toBe("1234567890123456791");
		expect(command.clientRequestId.length).toBeGreaterThan(0);
	});

	it("失敗後重試沿用同一個冪等鍵", async () => {
		const execute = vi
			.fn()
			.mockResolvedValueOnce({
				ok: false,
				error: { code: "NETWORK", detail: "連線中斷。", retryable: true },
			})
			.mockResolvedValueOnce({ ok: true, data: result() });
		await renderAtReconfirm(execute as unknown as PublicationExecute);
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));
		await waitFor(() => {
			expect(execute).toHaveBeenCalledTimes(1);
		});
		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));
		await waitFor(() => {
			expect(execute).toHaveBeenCalledTimes(2);
		});

		expect(execute.mock.calls[1][0].clientRequestId).toBe(
			execute.mock.calls[0][0].clientRequestId,
		);
	});

	it("發布成功後顯示邀請已排入處理", async () => {
		await renderAtReconfirm(okExecute());
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));

		await waitFor(() => {
			expect(screen.getByText("已建立發布，邀請已排入處理")).toBeTruthy();
		});
	});

	it("不宣稱賣家已看到或已可供查看", async () => {
		await renderAtReconfirm(okExecute());
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));

		await waitFor(() => {
			expect(screen.getByText("已建立發布，邀請已排入處理")).toBeTruthy();
		});
		const text = document.body.textContent ?? "";
		expect(text).not.toContain("已可供符合資格的賣家查看");
		expect(text).not.toContain("正在提供給符合資格的賣家");
		expect(text.replaceAll("不代表賣家已看到", "")).not.toContain("賣家已看到");
	});

	it("發布成立後 execute CTA 不再可點", async () => {
		const execute = okExecute();
		await renderAtReconfirm(execute);
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));
		await waitFor(() => {
			expect(screen.getByText("已建立發布，邀請已排入處理")).toBeTruthy();
		});

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));
		expect(execute).toHaveBeenCalledTimes(1);
	});

	it("失敗時不顯示已發布", async () => {
		const execute = vi.fn(async () => ({
			ok: false as const,
			error: { code: "CONFLICT", detail: "核准已過期。", retryable: false },
		}));
		await renderAtReconfirm(execute as unknown as PublicationExecute);
		await obtainApproval();

		fireEvent.click(screen.getByRole("button", { name: "確認並送出邀請" }));

		await waitFor(() => {
			expect(screen.getByText("無法送出邀請")).toBeTruthy();
		});
		expect(screen.queryByText("已建立發布，邀請已排入處理")).toBeNull();
	});
});

describe("RfqDraftFlow — A13 發布受眾預設", () => {
	it("候選在 R-01 上消失時 fail closed，不送出必然被拒的請求", async () => {
		const onPreview = vi.fn();
		const candidate = {
			candidateId: "seller-green",
			sourceId: "product-green",
			lane: "PLATFORM" as const,
			displayName: "範例鮮果合作社",
			score: 94,
			matchedFacts: [],
			unverifiableFacts: [],
			factsAsOf: "2026-08-14T12:00:00Z",
			selected: false,
		};
		const props = (candidates: unknown[]) => ({
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			candidateSetId: "1234567890123456790",
			slots: baseSlots,
			candidates,
			onBackToCandidates: () => {},
			onReturnToConversation: () => {},
			onPreviewRfqPrice: vi.fn(
				async () =>
					({ ok: true, data: preview() }) as RfqCommandResult<RfqPricePreview>,
			),
			onConfirmRfqSnapshot: vi.fn(
				async () =>
					({
						ok: true,
						data: confirmation(),
					}) as RfqCommandResult<RfqConfirmation>,
			),
			onRefreshSnapshot: async () => true,
			onPreviewRfqPublication: onPreview,
			onExecuteRfqPublication: okExecute(),
			approvalEnabled: true,
		});
		const { rerender } = render(
			React.createElement(RfqDraftFlow, props([candidate]) as never),
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
		fireEvent.click(
			screen.getByRole("button", { name: "檢視不可變 RFQ 快照" }),
		);
		await waitFor(() => {
			expect(screen.getByText("重新確認 RFQ 快照")).toBeTruthy();
		});

		rerender(React.createElement(RfqDraftFlow, props([]) as never));

		expect(
			screen.getByText("這筆詢價沒有可指定的對象，無法發布。"),
		).toBeTruthy();
		expect(
			screen
				.getByRole("button", { name: "確認並準備發布" })
				.hasAttribute("disabled"),
		).toBe(true);
		expect(onPreview).not.toHaveBeenCalled();
	});

	it("不再提供許願池這個選項", async () => {
		await renderAtReconfirm(okExecute());

		expect(screen.queryByRole("radio", { name: "投入許願池" })).toBeNull();
		expect(screen.queryByText(/投入許願池/)).toBeNull();
	});

	it("發布對象的家數與候選頁一致：同商家多筆只算一家", async () => {
		await renderAtReconfirm(okExecute(), {
			candidates: [
				laneCandidate("a", "PLATFORM", "77"),
				laneCandidate("b", "PLATFORM", "77"),
				laneCandidate("c", "PLATFORM", "88"),
			],
		});

		expect(screen.getByText("平台賣家 2 家")).toBeTruthy();
	});

	it("混選兩條 lane 時分別列出平台與站外家數", async () => {
		await renderAtReconfirm(okExecute(), {
			candidates: [
				laneCandidate("a", "PLATFORM", "77"),
				laneCandidate("b", "PLATFORM", "77"),
				laneCandidate("c", "EXTERNAL"),
				laneCandidate("d", "EXTERNAL"),
			],
		});

		expect(screen.getByText("平台賣家 1 家 · 站外業者 2 家")).toBeTruthy();
	});

	it("只有一條 lane 時不列出 0 家的那一項", async () => {
		await renderAtReconfirm(okExecute(), {
			candidates: [laneCandidate("c", "EXTERNAL")],
		});

		expect(screen.getByText("站外業者 1 家")).toBeTruthy();
		expect(screen.queryByText(/平台賣家/)).toBeNull();
	});
});
