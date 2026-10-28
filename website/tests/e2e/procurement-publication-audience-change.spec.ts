import { expect, test } from "@playwright/test";
import {
	pricePreviewData,
	RFQ_APPROVAL_ID,
	RFQ_CONFIRMED_HASH,
	RFQ_CONFIRMED_ID,
	RFQ_PREVIEW_HASH,
	RFQ_REQUEST_ID,
	RFQ_REVISION_ID,
	RFQ_SELLER_PAYLOAD_HASH,
	setupRfqPriceReview,
} from "./helpers/mock-procurement-rfq";

/**
 * A7／A13：核准只對它核准過的那一份受眾有效。
 *
 * 畫面上的警示條承諾「任一需求、價格、**候選或受眾**變更，都會使此確認失效並
 * 要求重新試算」。在修正之前只有需求與價格那一半有實作：改候選會把使用者切回
 * 候選頁，但 RfqDraftFlow 仍然掛著（`hidden` 而非卸載），`publicationState` 原封
 * 不動，點回去就還是能送出。
 *
 * ⛔ 一定要 browser 層：unit test 是直接用新的 candidates 重繪 RfqDraftFlow，
 * 證不到「在候選頁真的點一下核取方塊」會不會傳導到草稿流程。
 *
 * ⛔ 後果方向：execute 的 body **不帶受眾**（server 在 preview 階段就把
 * audienceHash 編進 approval），所以舊核准被執行時寄出去的是**舊名單**，
 * 不是畫面上那組新的。斷言要釘在「不可執行」與「重新核准後帶的是新 approvalId」，
 * 不要寫成「會寄給新名單」——那是假的。
 */
const SECOND_APPROVAL_ID = "1234567890123456903";

test.describe("procurement assistant — 受眾變更使核准失效", {
	tag: ["@ui"],
}, () => {
	test("改選受眾後舊核准不可執行；重新核准後 execute 帶的是新核准", async ({
		page,
	}) => {
		test.setTimeout(60_000);
		const executeBodies: Array<Record<string, unknown>> = [];
		const previewBodies: Array<Record<string, unknown>> = [];
		let approvalCalls = 0;

		await setupRfqPriceReview(page, {
			candidateSearchPollHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							requestId: RFQ_REQUEST_ID,
							runId: "1234567890123456800",
							candidateSetId: "candidate-set-e2e",
							status: "SUCCEEDED",
							platformLaneStatus: "SUCCEEDED",
							externalLaneStatus: "SUCCEEDED",
							failureReason: "",
							candidates: {
								// 第一筆刻意沿用 helper 預設的 id 與店名：setupRfqPriceReview
								// 的收尾會點那一張把它選起來。另外兩筆才是這條測試加的。
								platform: [
									{
										candidateId: "seller-green",
										displayName: "範例鮮果合作社",
									},
									{ candidateId: "seller-south", displayName: "南方果園" },
									{ candidateId: "seller-east", displayName: "東岸鮮果" },
								].map((c, i) => ({
									candidateId: c.candidateId,
									sourceId: `product-${i}`,
									lane: "PLATFORM",
									displayName: c.displayName,
									score: 94 - i,
									matchedFacts: [{ key: "item", label: "珍珠芭樂" }],
									unverifiableFacts: [],
									factsAsOf: "2026-08-14T12:00:00Z",
									selected: false,
								})),
								external: [],
							},
						},
					}),
				}),
			previewHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({ code: 0, data: pricePreviewData() }),
				}),
			confirmationHandler: (route) =>
				route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							confirmedRfqSnapshotId: RFQ_CONFIRMED_ID,
							payloadHash: RFQ_CONFIRMED_HASH,
							expiresAt: "2026-08-22T12:36:00Z",
						},
					}),
				}),
			publicationPreviewHandler: (route) => {
				approvalCalls += 1;
				previewBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				// 第二次核准是針對新受眾的另一份 approval。
				const approvalId =
					approvalCalls === 1 ? RFQ_APPROVAL_ID : SECOND_APPROVAL_ID;
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							approvalId,
							approvalVersion: "1",
							audienceHash: "c".repeat(64),
							sellerFacingPayloadHash: RFQ_SELLER_PAYLOAD_HASH,
							sellerFacingPayloadJson: '{"item":"珍珠芭樂"}',
							expiresAt: "2026-08-22T12:36:00Z",
							audiencePolicyId: "",
							audiencePolicyVersion: "0",
							recipientCount: approvalCalls === 1 ? "1" : "2",
						},
					}),
				});
			},
			publicationExecuteHandler: (route) => {
				executeBodies.push(
					route.request().postDataJSON() as Record<string, unknown>,
				);
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						data: {
							publicationId: "1234567890123456804",
							version: "1",
							publishedAt: "2026-08-22T12:40:00Z",
						},
					}),
				});
			},
		});

		// —— 走到取得核准（受眾＝範例鮮果合作社）——
		// setupRfqPriceReview 已經把畫面帶到價格確認那一步，序列照既有的
		// procurement-assistant-rfq-draft.spec.ts。
		await page
			.getByRole("checkbox", { name: "我已核對這份需求版本與價格範圍" })
			.check();
		await page.getByRole("button", { name: "檢視不可變 RFQ 快照" }).click();
		await expect(
			page.getByRole("heading", { name: "重新確認 RFQ 快照" }),
		).toBeVisible();
		await page
			.getByRole("checkbox", {
				name: "我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變",
			})
			.check();
		await page.getByRole("button", { name: "確認並準備發布" }).click();

		const executeCta = page.getByRole("button", { name: "確認並送出邀請" });
		await expect(executeCta).toBeVisible();
		expect(approvalCalls).toBe(1);

		// —— 回到候選頁換掉受眾 ——
		await page.getByRole("button", { name: "返回候選" }).click();
		// 點 label 而不是 input：候選列在視窗底部時，頁尾會攔截對 input 的點擊。
		// 這也是 setupRfqPriceReview 自己選第一張候選時用的方式。
		for (const id of ["seller-green", "seller-south", "seller-east"]) {
			await page.locator(`label[for="procurement-candidate-${id}"]`).click();
		}
		await expect(
			page.getByRole("checkbox", { name: "選取 範例鮮果合作社" }),
		).not.toBeChecked();

		// —— 舊核准必須失效 ——
		// 改受眾會把畫面切回候選頁（CandidateCanvas 既有行為），所以先確認
		// execute CTA 不在，再走一次「建立詢價草稿」回到草稿流程。
		await expect(executeCta).toBeHidden();
		expect(executeBodies).toHaveLength(0);

		await page.getByRole("button", { name: "建立詢價草稿" }).click();
		await expect(executeCta).toBeHidden();
		await expect(
			page.getByRole("button", { name: "確認並準備發布" }),
		).toBeVisible();

		// —— 重新核准之後才能送出，而且帶的是新核准 ——
		await page
			.getByRole("checkbox", {
				name: "我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變",
			})
			.check();
		await page.getByRole("button", { name: "確認並準備發布" }).click();
		await expect(executeCta).toBeVisible();
		await executeCta.click();

		expect(approvalCalls).toBe(2);
		expect(executeBodies).toHaveLength(1);
		expect(executeBodies[0]).toMatchObject({
			approvalId: SECOND_APPROVAL_ID,
			expectedRevisionId: RFQ_REVISION_ID,
			payloadHash: RFQ_SELLER_PAYLOAD_HASH,
		});
		// 第二次核准送出的受眾就是畫面上那兩家。
		expect(previewBodies[1]).toMatchObject({
			selectedCandidateIds: ["seller-south", "seller-east"],
		});
	});
});
