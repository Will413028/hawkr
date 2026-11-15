import { describe, expect, it, vi } from "vitest";
import { page } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import type { BuyerQuoteAcceptanceResponse } from "./buyer-quote-contract";
import QuoteBatchSummary from "./QuoteBatchSummary";
import {
	type PublicationResultView,
	publicationResultDataSchema,
} from "./runtime-snapshot-contract";

function publicationWith(
	counts: Partial<{
		recipientCount: number;
		queuedCount: number;
		notSentCount: number;
	}> = {},
): PublicationResultView {
	return {
		artifactVersion: "3",
		publishedAt: "2026-08-20T12:36:00Z",
		data: publicationResultDataSchema.parse({
			publicationId: "1234567890123456788",
			version: "1",
			sellerFacingPayloadHash: "b".repeat(64),
			recipientCount: 0,
			queuedCount: 0,
			notSentCount: 0,
			...counts,
		}),
	};
}

const publication = publicationWith({
	recipientCount: 5,
	queuedCount: 4,
	notSentCount: 1,
});

const acceptedCreatingOrder: BuyerQuoteAcceptanceResponse = {
	buyerAcceptanceId: "1234567890123456805",
	sellerResponseId: "1234567890123456788",
	sellerResponseVersion: "1234567890123456791",
	acceptedDealPrice: "18900",
	currency: "TWD",
	acceptedAt: "2026-08-22T06:00:00Z",
	orderProvisioningState: "CREATING_ORDER",
	orderNo: "",
	paymentDeadline: "",
	replayed: false,
};

type Props = React.ComponentProps<typeof QuoteBatchSummary>;

function setup(overrides: Partial<Props> = {}) {
	const props: Props = {
		slots: {
			item: "蘋果",
			quantity: 100,
			unit: "公斤",
			deliveryCity: "臺北市",
			deliveryDate: "2026-08-25",
		},
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			lowerBoundAmount: "58",
			upperBoundAmount: "70",
			basisQuantity: "1",
			basisUnit: "公斤",
		},
		publication,
		counts: { responseCount: 0, shortlistCount: 0, error: null },
		acceptance: null,
		onViewSnapshot: vi.fn(),
		onCompareQuotes: vi.fn(),
		...overrides,
	};
	return renderWithIntl(<QuoteBatchSummary {...props} />, {
		frameMaxWidth: 1100,
	});
}

const text = (value: string) => page.getByText(value, { exact: true });

describe("QuoteBatchSummary", () => {
	it("is honest when published without quotes", async () => {
		const { container } = setup();
		await expect
			.element(page.getByRole("heading", { name: "詢價已發布", exact: true }))
			.toBeVisible();
		await expect
			.element(text("已建立 1 組詢價・5 筆邀請（100 公斤）"))
			.toBeVisible();
		await expect.element(text("0 筆")).toBeVisible();
		await expect.element(text("尚未成立")).toBeVisible();
		await expect.element(text("尚未建立")).toBeVisible();
		await expect
			.element(
				page.getByRole("button", { name: "比較供應商回價", exact: true }),
			)
			.toBeDisabled();
		// 沒有回價就不得出現任何成交語氣。
		expect(container.textContent).not.toMatch(/已成交|已下單/);
	});

	it("shows the six-step progress", async () => {
		setup();
		await expect.element(text("已確認 RFQ 快照")).toBeVisible();
		await expect.element(text("已發布詢價")).toBeVisible();
		await expect.element(text("2026/08/20 20:36")).toBeVisible();
		await expect.element(text("等待供應商正式回價")).toBeVisible();
		await expect.element(text("付款成功後正式成立")).toBeVisible();
		// 第一步沒有伺服器時間戳可寫，所以是「已完成」而不是補出來的時間。
		await expect.poll(() => text("已完成").elements().length).toBe(1);
	});

	it("enables the comparison once quotes arrive, price still unestablished", async () => {
		setup({ counts: { responseCount: 3, shortlistCount: 1, error: null } });
		await expect.element(text("3 筆")).toBeVisible();
		await expect
			.element(
				page.getByRole("button", { name: "比較供應商回價", exact: true }),
			)
			.toBeEnabled();
		// 成交價格仍未成立：接受回執才是唯一來源。
		await expect.element(text("尚未成立")).toBeVisible();
	});

	it("shows the accepted price while the order is still being created", async () => {
		const { container } = setup({
			counts: { responseCount: 3, shortlistCount: 1, error: null },
			acceptance: acceptedCreatingOrder,
		});
		await expect.element(text("NT$18,900")).toBeVisible();
		await expect.element(text("建立中")).toBeVisible();
		// CREATING_ORDER 不得渲染成已成交或已下單。
		expect(container.textContent).not.toMatch(/已成交|已下單/);
	});

	it("describes recipients as intent, not delivery", async () => {
		const { container } = setup();
		await expect.element(text("這次發布的收件人")).toBeVisible();
		await expect
			.element(text("這三個數字是發布當下的收件人分布，不是投遞結果。"))
			.toBeVisible();
		// queuedCount 不得被講成「已送出」；面板上也沒有任何撤回動作。
		expect(container.textContent).not.toMatch(/已送出|已寄送/);
		expect(container.textContent).not.toMatch(/撤回|撤銷/);
	});

	it("still shows the three recipient columns when all are 0", async () => {
		setup({ publication: publicationWith() });
		const recipients = page.getByRole("region", {
			name: "這次發布的收件人",
			exact: true,
		});
		await expect.element(recipients).toBeVisible();
		// 範圍鎖在這一區：第三張卡的 shortlist 也是 0，不能混進來當證據。
		await expect
			.poll(() => recipients.getByText("0", { exact: true }).elements().length)
			.toBe(3);
	});

	it("says 尚未取得 when counts cannot be read", async () => {
		setup({
			counts: {
				responseCount: null,
				shortlistCount: null,
				error: {
					code: "SERVER_ERROR",
					detail: "伺服器暫時無法處理，請稍後再試。",
					retryable: true,
				},
			},
		});
		await expect.poll(() => text("尚未取得").elements().length).toBe(2);
		await expect
			.element(text("伺服器暫時無法處理，請稍後再試。"))
			.toBeVisible();
	});

	it("shows the not-published state without a recipients card", async () => {
		setup({
			publication: null,
			counts: { responseCount: null, shortlistCount: null, error: null },
			buyerRequestedRange: null,
		});
		await expect
			.element(page.getByRole("heading", { name: "詢價建立摘要", exact: true }))
			.toBeVisible();
		await expect.element(text("這次發布的收件人")).not.toBeInTheDocument();
		await expect
			.element(text("後端未提供不可變核准證據；此畫面不代表核准已完成。"))
			.toBeVisible();
		await expect
			.element(page.getByRole("button", { name: /檢視不可變 RFQ 快照/ }))
			.toBeDisabled();
	});
});
