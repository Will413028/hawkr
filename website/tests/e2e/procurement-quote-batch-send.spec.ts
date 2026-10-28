import { expect, test } from "@playwright/test";
import {
	pricePreviewData,
	RFQ_CONFIRMED_HASH,
	RFQ_CONFIRMED_ID,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

test.describe("procurement assistant — RFQ confirmation recovery", {
	tag: ["@ui"],
}, () => {
	test("a lost confirmation response replays the exact same command without publishing", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		const bodies: Array<Record<string, unknown>> = [];
		const { legacy } = await setupRfqPriceReview(page, {
			confirmationHandler: (route) => {
				bodies.push(route.request().postDataJSON() as Record<string, unknown>);
				if (bodies.length === 1) return route.abort("failed");
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							confirmedRfqSnapshotId: RFQ_CONFIRMED_ID,
							payloadHash: RFQ_CONFIRMED_HASH,
						},
					}),
				});
			},
		});

		await expect(page.getByText("你設定的希望詢價區間")).toBeVisible();
		await page
			.getByRole("checkbox", {
				name: "我已核對這份需求版本與價格範圍",
			})
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await expect(
			page.getByText(/連線中斷，無法確認這次操作的結果/),
		).toBeVisible();
		await page.getByRole("button", { name: "以原操作重試" }).click();

		// 重試成功後與首次成功走同一條路：直接進到稿 R-01。
		await expect(
			page.getByRole("heading", { name: "重新確認 RFQ 快照" }),
		).toBeVisible();
		expect(bodies).toHaveLength(2);
		expect(bodies[1]).toEqual(bodies[0]);
		expect(bodies[0]).toMatchObject({
			expectedRevisionId: "1234567890123456791",
			pricePreviewId: "1234567890123456801",
			pricePreviewVersion: "1",
			payloadHash: "a".repeat(64),
		});
		expect(legacy.calls).toBe(0);
	});

	test("a missing preview is recovered as a price re-query, not a missing conversation", async ({
		page,
	}) => {
		test.setTimeout(45_000);
		let confirmationCalls = 0;
		const previewBodies: Array<Record<string, unknown>> = [];
		await setupRfqPriceReview(page, {
			previewHandler: (route) => {
				previewBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: pricePreviewData() }),
				});
			},
			confirmationHandler: (route) => {
				confirmationCalls += 1;
				return route.fulfill({
					status: 404,
					contentType: "application/json",
					body: JSON.stringify({
						code: 404,
						reason: "RESOURCE_NOT_FOUND",
						message: "preview not found",
					}),
				});
			},
		});

		await page
			.getByRole("checkbox", {
				name: "我已核對這份需求版本與價格範圍",
			})
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await expect(
			page.getByText("找不到這份價格試算，請重新試算後再確認。"),
		).toBeVisible();
		await expect(
			page.getByText("無法讀取先前的需求，請重新開始。"),
		).toHaveCount(0);
		await page.getByRole("button", { name: "重新試算" }).click();
		await expect.poll(() => previewBodies.length).toBe(2);
		expect(previewBodies[1]?.clientRequestId).not.toBe(
			previewBodies[0]?.clientRequestId,
		);
		expect(confirmationCalls).toBe(1);
	});
});
