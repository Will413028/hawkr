/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";
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
	RfqPublicationPreviewCommand,
	RfqPublicationResult,
} from "../rfq-publication-contract";

type PublicationExecute = (
	command: RfqPublicationExecuteCommand,
) => Promise<RfqCommandResult<RfqPublicationResult>>;

// ⛔ 用真正的 prop 簽章，不要用 ReturnType<typeof vi.fn>：後者會塌成
// Mock<Procedure>，tsc 對不上 RfqDraftFlow 的 prop（vitest 執行期不做型別檢查，
// 所以只有 `pnpm lint` 的 tsc 那一關抓得到）。
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

function preview(): RfqPricePreview {
	return {
		pricePreviewId: "1234567890123456799",
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
		marketPriceBasis: {
			regionName: "臺北市",
			supportingMarketCount: "3",
			sourceDataAsOf: "2026-08-20T12:36:00Z",
			freshnessStatus: "fresh",
		},
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
		dealTemperatureStatus: "UNKNOWN",
		audiencePolicyId: "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
		audiencePolicyVersion: "1",
		recipientCount: "0",
		...overrides,
	};
}

function publicationResult(): RfqPublicationResult {
	return {
		publicationId: "1234567890123456804",
		publicationVersion: "1",
		state: "PUBLISHED",
		audienceMode: "DIRECT_RECIPIENTS",
		audienceHash: "d".repeat(64),
		sellerFacingPayloadHash: "e".repeat(64),
		publishedAt: "2026-08-22T12:40:00Z",
		replayed: false,
	};
}

function candidate(id: string): ProcurementCandidate {
	return {
		candidateId: id,
		sourceId: `product-${id}`,
		lane: "PLATFORM" as const,
		displayName: `候選 ${id}`,
		score: 90,
		matchedFacts: [],
		unverifiableFacts: [],
		factsAsOf: "2026-08-14T12:00:00Z",
		selected: true,
	} as unknown as ProcurementCandidate;
}

const EXECUTE_CTA = "確認並送出邀請";
const APPROVE_CTA = "確認並準備發布";

function element(
	candidates: ProcurementCandidate[],
	handlers: {
		onExecuteRfqPublication: PublicationExecute;
		onPreviewRfqPublication: PublicationPreview;
	},
) {
	return React.createElement(RfqDraftFlow, {
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
		onPreviewRfqPublication: handlers.onPreviewRfqPublication,
		onExecuteRfqPublication: handlers.onExecuteRfqPublication,
		approvalEnabled: true,
	});
}

/**
 * 走到「重新確認」畫面並取得一份可執行的核准。回傳 rerender，讓測試能在**同一棵
 * React tree 上**換掉候選名單——那正是使用者返回候選頁改選時發生的事。用新的
 * render() 重來會把 state 一起清掉，就測不到「舊核准還活著」這個缺陷。
 */
async function renderApproved(
	candidates: ProcurementCandidate[],
	handlers: {
		onExecuteRfqPublication: PublicationExecute;
		onPreviewRfqPublication: PublicationPreview;
	},
) {
	const { rerender } = render(element(candidates, handlers));

	await driveToApproval();

	return (next: ProcurementCandidate[]) => rerender(element(next, handlers));
}

/** 從第一步點到「已取得一份可執行的核准」。兩種 harness 共用同一段操作。 */
async function driveToApproval() {
	fireEvent.click(screen.getByRole("button", { name: "查看預計名單" }));
	fireEvent.click(screen.getByRole("button", { name: "取得價格範圍" }));
	await waitFor(() => {
		expect(screen.getByText("參考行情與希望詢價")).toBeTruthy();
	});
	fireEvent.click(
		screen.getByRole("checkbox", { name: /我已核對這份需求版本與價格範圍/ }),
	);
	fireEvent.click(screen.getByRole("button", { name: "檢視不可變 RFQ 快照" }));
	await waitFor(() => {
		expect(screen.getByText("重新確認 RFQ 快照")).toBeTruthy();
	});
	fireEvent.click(
		screen.getByRole("checkbox", {
			name: /我已重新檢查需求、希望價格與發布受眾/,
		}),
	);
	fireEvent.click(screen.getByRole("button", { name: APPROVE_CTA }));
	await waitFor(() => {
		expect(screen.getByRole("button", { name: EXECUTE_CTA })).toBeTruthy();
	});
}

/**
 * 同 renderApproved，但 React root 由測試自己持有。理由只有一個：要用 flushSync
 * 把新 props **只 commit 到畫面**、把作廢核准的 passive effect 留在 scheduler 裡。
 * RTL 的 render／rerender／fireEvent 全都包在 act 中，act 離場就把 effect 沖掉，
 * 用它們永遠複製不出「使用者改完名單立刻按下送出」的那個視窗。
 */
async function renderApprovedWithOwnRoot(
	candidates: ProcurementCandidate[],
	handlers: {
		onExecuteRfqPublication: PublicationExecute;
		onPreviewRfqPublication: PublicationPreview;
	},
) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	await act(async () => {
		root.render(element(candidates, handlers));
	});

	await driveToApproval();

	return {
		/** 只 commit，不 flush passive effect——這就是那個視窗。 */
		commitWithoutEffects(next: ProcurementCandidate[]) {
			flushSync(() => root.render(element(next, handlers)));
		},
		async flushEffects() {
			await act(async () => {});
		},
		async unmount() {
			await act(async () => {
				root.unmount();
			});
			container.remove();
		},
	};
}

function handlers(approvals: RfqPublicationApproval[] = [approval()]) {
	const queue = [...approvals];
	return {
		onExecuteRfqPublication: vi.fn(
			async () =>
				({
					ok: true,
					data: publicationResult(),
				}) as RfqCommandResult<RfqPublicationResult>,
		) as unknown as PublicationExecute,
		// 明確標註回傳型別：vi.fn 的推導會塌成 Mock<Procedure>，tsc 便對不上
		// RfqDraftFlow 的 prop 簽章（vitest 執行期不做型別檢查，所以只有 tsc 抓得到）。
		onPreviewRfqPublication: vi.fn(
			async (): Promise<RfqCommandResult<RfqPublicationApproval>> => ({
				ok: true,
				data:
					queue.length > 1
						? (queue.shift() as RfqPublicationApproval)
						: queue[0],
			}),
		),
	};
}

describe("RfqDraftFlow — 核准只對它核准過的那份受眾有效", () => {
	it("受眾換成另一組候選後，舊核准不可執行", async () => {
		const h = handlers();
		const rerenderWith = await renderApproved([candidate("seller-a")], h);

		rerenderWith([candidate("seller-b"), candidate("seller-c")]);

		await waitFor(() => {
			expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
		});
		expect(screen.getByRole("button", { name: APPROVE_CTA })).toBeTruthy();
		expect(h.onExecuteRfqPublication).not.toHaveBeenCalled();
	});

	it("只改順序也要作廢：受眾身分包含有序的收件人清單", async () => {
		const h = handlers();
		const a = candidate("seller-a");
		const b = candidate("seller-b");
		const rerenderWith = await renderApproved([a, b], h);

		rerenderWith([b, a]);

		await waitFor(() => {
			expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
		});
	});

	it("完全相同的 rerender 不得作廢核准", async () => {
		const h = handlers();
		const rerenderWith = await renderApproved([candidate("seller-a")], h);

		rerenderWith([candidate("seller-a")]);

		expect(screen.getByRole("button", { name: EXECUTE_CTA })).toBeTruthy();
	});

	it("重新核准後 execute 帶的是新核准，不是舊的那一份", async () => {
		const h = handlers([
			approval({ approvalId: "1111111111111111111" }),
			approval({ approvalId: "2222222222222222222", approvalVersion: "2" }),
		]);
		const rerenderWith = await renderApproved([candidate("seller-a")], h);

		rerenderWith([candidate("seller-b")]);
		await waitFor(() => {
			expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
		});

		fireEvent.click(
			screen.getByRole("checkbox", {
				name: /我已重新檢查需求、希望價格與發布受眾/,
			}),
		);
		fireEvent.click(screen.getByRole("button", { name: APPROVE_CTA }));
		await waitFor(() => {
			expect(screen.getByRole("button", { name: EXECUTE_CTA })).toBeTruthy();
		});
		fireEvent.click(screen.getByRole("button", { name: EXECUTE_CTA }));

		await waitFor(() => {
			expect(h.onExecuteRfqPublication).toHaveBeenCalledTimes(1);
		});
		const command = (
			h.onExecuteRfqPublication as unknown as ReturnType<typeof vi.fn>
		).mock.calls[0][0] as RfqPublicationExecuteCommand;
		expect(command.approval.approvalId).toBe("2222222222222222222");
	});

	it("名單被清空後 execute 不可用（fail closed 不只在 preview 階段）", async () => {
		const h = handlers();
		const rerenderWith = await renderApproved([candidate("seller-a")], h);

		rerenderWith([]);

		await waitFor(() => {
			expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
		});
		expect(h.onExecuteRfqPublication).not.toHaveBeenCalled();
	});
});

describe("RfqDraftFlow — execute 送出前再比對一次受眾身分", () => {
	it("effect 還沒跑的視窗裡按下 execute：不得用上一份名單的核准送出", async () => {
		const h = handlers();
		const view = await renderApprovedWithOwnRoot([candidate("seller-a")], h);

		try {
			view.commitWithoutEffects([candidate("seller-b"), candidate("seller-c")]);

			// 這兩條是「視窗確實存在」的證據，不是順手多驗的東西：作廢核准的 effect
			// 若已經跑過，CTA 會消失、核准卡片會收起來，底下那一下 click 就什麼都沒測到。
			const executeButton = screen.getByRole("button", { name: EXECUTE_CTA });
			expect(screen.getByText("發布核准已建立，尚未送出邀請。")).toBeTruthy();

			// ⛔ 原生 dispatchEvent：RTL 的 fireEvent 包了 act，會先把 effect 沖乾淨，
			// 視窗就不見了。這裡要的正是「effect 還沒跑，使用者已經按下去」。
			executeButton.dispatchEvent(
				new MouseEvent("click", { bubbles: true, cancelable: true }),
			);

			expect(h.onExecuteRfqPublication).not.toHaveBeenCalled();

			await view.flushEffects();
			await waitFor(() => {
				expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
			});
			expect(screen.getByRole("button", { name: APPROVE_CTA })).toBeTruthy();
			expect(h.onExecuteRfqPublication).not.toHaveBeenCalled();
		} finally {
			await view.unmount();
		}
	});

	it("同一份名單在同視窗內按 execute 仍然送得出去（守門不是把 execute 一律擋掉）", async () => {
		const h = handlers();
		const view = await renderApprovedWithOwnRoot([candidate("seller-a")], h);

		try {
			// 名單沒變，只是重新 commit 一次；核准仍然是針對這份名單發的。
			view.commitWithoutEffects([candidate("seller-a")]);

			screen
				.getByRole("button", { name: EXECUTE_CTA })
				.dispatchEvent(new MouseEvent("click", { bubbles: true }));

			await waitFor(() => {
				expect(h.onExecuteRfqPublication).toHaveBeenCalledTimes(1);
			});
		} finally {
			await view.unmount();
		}
	});

	it("已經在飛的 execute 不因畫面改動被作廢或重送：side effect 只有一次", async () => {
		const executed: RfqPublicationExecuteCommand[] = [];
		let release: (() => void) | undefined;
		const pending = new Promise<void>((resolve) => {
			release = resolve;
		});
		const onExecuteRfqPublication = vi.fn(
			async (
				command: RfqPublicationExecuteCommand,
			): Promise<RfqCommandResult<RfqPublicationResult>> => {
				executed.push(command);
				await pending;
				return { ok: true, data: publicationResult() };
			},
		);
		const h = {
			...handlers(),
			onExecuteRfqPublication:
				onExecuteRfqPublication as unknown as PublicationExecute,
		};
		const rerenderWith = await renderApproved([candidate("seller-a")], h);

		fireEvent.click(screen.getByRole("button", { name: EXECUTE_CTA }));
		await waitFor(() => {
			expect(executed).toHaveLength(1);
		});

		// 送出後才改名單：畫面回到「要重新核准」，但那筆已經在飛的 command 不受影響。
		rerenderWith([candidate("seller-b")]);
		await waitFor(() => {
			expect(screen.queryByRole("button", { name: EXECUTE_CTA })).toBeNull();
		});

		await act(async () => {
			release?.();
			await pending;
		});

		await waitFor(() => {
			expect(screen.getByText("已建立發布，邀請已排入處理")).toBeTruthy();
		});
		expect(executed).toHaveLength(1);
		expect(executed[0].approval.approvalId).toBe("1234567890123456803");
	});
});
