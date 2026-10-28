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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import RfqDraftFlow from "../RfqDraftFlow";
import type {
	RfqCommandResult,
	RfqConfirmation,
	RfqPricePreview,
} from "../rfq-price-contract";
import {
	publicationPreviewBody,
	type RfqPublicationApproval,
	type RfqPublicationPreviewCommand,
} from "../rfq-publication-contract";

type PublicationPreview = (
	command: RfqPublicationPreviewCommand,
) => Promise<RfqCommandResult<RfqPublicationApproval>>;

/**
 * 台北 2026-09-14 00:30，UTC 還停在 2026-09-13 16:30。
 *
 * 刻意選在跨日窗口裡：下限若是拿 UTC 曆日算的，這裡會變成 `2026-09-13`，
 * 於是台北的買家在午夜過後可以選一個「昨天」當回覆期限而畫面不吭聲。
 */
const NOW_MS = Date.parse("2026-09-13T16:30:00Z");
const TODAY_TAIPEI = "2026-09-14";
const YESTERDAY_TAIPEI = "2026-09-13";

beforeEach(() => {
	vi.spyOn(Date, "now").mockReturnValue(NOW_MS);
});

afterEach(() => {
	vi.restoreAllMocks();
	cleanup();
});

const baseSlots = {
	item: "蘋果",
	quantity: 100,
	unit: "KG",
	deliveryCity: "臺北市",
	deliveryDate: "2026-09-28",
};

function preview(): RfqPricePreview {
	return {
		pricePreviewId: "1234567890123456799",
		version: "1",
		qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
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
		expiresAt: "2026-09-16T12:36:00Z",
	};
}

function approval(): RfqPublicationApproval {
	return {
		approvalId: "1234567890123456803",
		approvalVersion: "1",
		audienceHash: "d".repeat(64),
		sellerFacingPayloadHash: "e".repeat(64),
		sellerFacingPayloadJson: '{"item":"蘋果"}',
		expiresAt: "2026-09-16T12:36:00Z",
		audiencePolicyId: "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
		audiencePolicyVersion: "1",
		recipientCount: "1",
	};
}

/**
 * 收集實際送出的 preview command。用自建陣列而不是讀 `mock.calls`：後者在
 * 無參數的 `vi.fn(async () => …)` 上被推成長度 0 的 tuple，取 `[0][0]` 只能靠
 * 斷言硬轉，等於把「真的收到一個 command」這件事讓掉。
 */
function publicationSpy() {
	const commands: RfqPublicationPreviewCommand[] = [];
	const fn = vi.fn(async (command: RfqPublicationPreviewCommand) => {
		commands.push(command);
		return {
			ok: true,
			data: approval(),
		} as RfqCommandResult<RfqPublicationApproval>;
	});
	return { fn, commands };
}

async function renderAtReconfirm(
	onPreviewRfqPublication: PublicationPreview = vi.fn(
		async () =>
			({
				ok: true,
				data: approval(),
			}) as RfqCommandResult<RfqPublicationApproval>,
	),
) {
	const data = preview();
	const confirmed = confirmation();
	render(
		React.createElement(RfqDraftFlow, {
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			candidateSetId: "1234567890123456790",
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
					factsAsOf: "2026-09-10T12:00:00Z",
					selected: false,
				},
			],
			onBackToCandidates: () => {},
			onReturnToConversation: () => {},
			onPreviewRfqPrice: vi.fn(
				async () => ({ ok: true, data }) as RfqCommandResult<RfqPricePreview>,
			),
			onConfirmRfqSnapshot: vi.fn(
				async () =>
					({ ok: true, data: confirmed }) as RfqCommandResult<RfqConfirmation>,
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

const deadlineInput = () =>
	screen.getByLabelText(/回覆期限/) as HTMLInputElement;

const reconfirmCheckbox = () =>
	screen.getByRole("checkbox", {
		name: /我已重新檢查需求、希望價格與發布受眾/,
	});

describe("RfqDraftFlow — 回覆期限", () => {
	// 站外供應商的落地頁一直有「回覆期限」那一列，但買家在網站上沒有地方填，
	// 所以它永遠是空的。鏈路每一層都已經支援，缺的就是這個輸入。
	it("R-01 上有一個回覆期限輸入，格式是曆日", async () => {
		await renderAtReconfirm();

		const input = deadlineInput();
		expect(input.type).toBe("date");
		expect(input.value).toBe("");
	});

	// 後端 DB 是 DATE、`domain.ResponseDeadlineLayout = "2006-01-02"`。
	it("送出台北曆日 YYYY-MM-DD，不是 RFC3339", async () => {
		const { fn: onPreviewRfqPublication, commands } = publicationSpy();
		await renderAtReconfirm(onPreviewRfqPublication);

		fireEvent.change(deadlineInput(), { target: { value: "2026-09-20" } });
		fireEvent.click(reconfirmCheckbox());
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(onPreviewRfqPublication).toHaveBeenCalledTimes(1);
		});
		const [command] = commands;
		expect(command.responseDeadline).toBe("2026-09-20");
		expect(publicationPreviewBody(command).responseDeadline).toBe("2026-09-20");
	});

	// 選填是既有行為，不得因為多了這一欄就變成必填。
	it("沒填也能取得發布核准", async () => {
		const { fn: onPreviewRfqPublication, commands } = publicationSpy();
		await renderAtReconfirm(onPreviewRfqPublication);

		fireEvent.click(reconfirmCheckbox());
		expect(
			screen
				.getByRole("button", { name: "確認並準備發布" })
				.hasAttribute("disabled"),
		).toBe(false);
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(onPreviewRfqPublication).toHaveBeenCalledTimes(1);
		});
	});

	// ⛔ gateway 那側是 `*string` + `omitempty`：送空字串會被讀成「有給截止日而且
	// 是空的」並寫進賣家看到的內容。沒填就整個 key 不出現。
	it("沒填時 body 裡整個沒有 responseDeadline 這個 key", async () => {
		const { fn: onPreviewRfqPublication, commands } = publicationSpy();
		await renderAtReconfirm(onPreviewRfqPublication);

		fireEvent.click(reconfirmCheckbox());
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));

		await waitFor(() => {
			expect(onPreviewRfqPublication).toHaveBeenCalledTimes(1);
		});
		const [command] = commands;
		expect("responseDeadline" in publicationPreviewBody(command)).toBe(false);
	});

	// 下限用的是台北曆日：這一刻 UTC 還是 09-13，台北已經是 09-14。
	it("下限是台北的今天，不是 UTC 的今天", async () => {
		await renderAtReconfirm();

		expect(deadlineInput().getAttribute("min")).toBe(TODAY_TAIPEI);
	});

	// 已經過期的回覆期限對供應商只是雜訊。後端目前不驗，前端擋掉。
	it("早於今天時擋下發布並說明原因", async () => {
		const { fn: onPreviewRfqPublication, commands } = publicationSpy();
		await renderAtReconfirm(onPreviewRfqPublication);

		fireEvent.change(deadlineInput(), {
			target: { value: YESTERDAY_TAIPEI },
		});
		fireEvent.click(reconfirmCheckbox());

		expect(screen.getByText("回覆期限不能早於今天")).toBeTruthy();
		const cta = screen.getByRole("button", { name: "確認並準備發布" });
		expect(cta.hasAttribute("disabled")).toBe(true);
		fireEvent.click(cta);
		expect(onPreviewRfqPublication).not.toHaveBeenCalled();
	});

	// 台北的今天本身是合法的（「不得早於今天」不是「必須晚於今天」）。
	it("選今天不算過期", async () => {
		await renderAtReconfirm();

		fireEvent.change(deadlineInput(), { target: { value: TODAY_TAIPEI } });
		fireEvent.click(reconfirmCheckbox());

		expect(screen.queryByText("回覆期限不能早於今天")).toBeNull();
		expect(
			screen
				.getByRole("button", { name: "確認並準備發布" })
				.hasAttribute("disabled"),
		).toBe(false);
	});

	// 回覆期限進得了 sellerFacingPayload，所以它被核准的 payload hash 涵蓋：
	// 改了之後既有核准描述的就不是畫面上這一份，沿用它等於讓買家以為自己核准過。
	it("改期限會作廢既有核准", async () => {
		const { fn: onPreviewRfqPublication, commands } = publicationSpy();
		await renderAtReconfirm(onPreviewRfqPublication);

		fireEvent.click(reconfirmCheckbox());
		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));
		await waitFor(() => {
			expect(screen.getByText("WISH_POOL_ALL_ACTIVE_SELLERS_V1")).toBeTruthy();
		});

		fireEvent.change(deadlineInput(), { target: { value: "2026-09-21" } });

		expect(screen.queryByText("WISH_POOL_ALL_ACTIVE_SELLERS_V1")).toBeNull();
		expect(
			screen
				.getByRole("button", { name: "確認並準備發布" })
				.hasAttribute("disabled"),
		).toBe(false);

		fireEvent.click(screen.getByRole("button", { name: "確認並準備發布" }));
		await waitFor(() => {
			expect(onPreviewRfqPublication).toHaveBeenCalledTimes(2);
		});
		expect(commands[1].responseDeadline).toBe("2026-09-21");
	});
});
