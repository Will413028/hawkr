import { expect, test } from "@playwright/test";
import { ABORT_REQUEST } from "./helpers/mock-procurement-connect";
import {
	publicationApprovalJson,
	RFQ_CANDIDATE_SET_ID,
	RFQ_REQUEST_ID,
	RFQ_REVISION_ID,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ publication approval recovery", {
	tag: ["@ui"],
}, () => {
	test("a lost publication-approval response replays the exact same command without publishing", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		let executeCalls = 0;
		let responseLost = true;
		await setupRfqPriceReview(page, {
			publicationPreviewHandler: (request) => {
				bodies.push(request);
				if (responseLost) return ABORT_REQUEST;
				return publicationApprovalJson();
			},
			publicationExecuteHandler: () => {
				executeCalls += 1;
				throw new Error("取得核准階段不應該送出發布");
			},
		});

		await expect(page.getByText("你設定的希望詢價區間")).toBeVisible();
		await page
			.getByRole("checkbox", {
				name: "我已核對這份需求版本與價格範圍",
			})
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await page
			.getByRole("checkbox", {
				name: "我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變",
			})
			.check();
		await page.getByRole("button", { name: "確認並準備發布" }).click();
		await expect(page.getByText("無法取得發布核准")).toBeVisible();

		responseLost = false;
		const lostAttempts = bodies.length;
		await page.getByRole("button", { name: "確認並準備發布" }).click();
		await expect(
			page.getByRole("button", { name: "確認並送出邀請" }),
		).toBeVisible();
		expect(bodies).toHaveLength(lostAttempts + 1);
		for (const body of bodies) expect(body).toEqual(bodies[0]);
		expect(bodies[0]).toMatchObject({
			requestId: RFQ_REQUEST_ID,
			expectedRevisionId: RFQ_REVISION_ID,
			candidateSetId: RFQ_CANDIDATE_SET_ID,
			selectedCandidateIds: ["seller-green"],
			buyerRequestedRange: { lowerBoundAmount: "30", upperBoundAmount: "42" },
		});
		expect(bodies[0].clientRequestId).toEqual(expect.any(String));
		expect(executeCalls).toBe(0);
	});
});
