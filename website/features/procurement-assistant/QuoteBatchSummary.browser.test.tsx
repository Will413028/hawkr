import { describe, expect, it, vi } from "vitest";
import { page } from "vitest/browser";
import type { PublishedRfqPublication } from "@/features/procurement-assistant/types";
import { renderWithIntl } from "@/tests/browser/render";
import type { BuyerQuoteAcceptanceResponse } from "./buyer-quote-contract";
import QuoteBatchSummary from "./QuoteBatchSummary";

function publicationWith(
	counts: Partial<{
		recipientCount: number;
		queuedCount: number;
	}> = {},
): PublishedRfqPublication {
	return {
		publicationId: "1234567890123456788",
		version: "3",
		publishedAt: "2026-08-20T12:36:00Z",
		recipientCount: 0,
		queuedCount: 0,
		sellerFacingHash: "b".repeat(64),
		...counts,
	};
}

const publication = publicationWith({
	recipientCount: 5,
	queuedCount: 4,
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
	return renderWithIntl(
		<div style={{ width: "100%", maxWidth: 1100, padding: 16 }}>
			<QuoteBatchSummary {...props} />
		</div>,
	);
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
		expect(container.textContent).not.toMatch(/已成交|已下單/);
	});

	it("shows the six-step progress", async () => {
		setup();
		await expect.element(text("已確認 RFQ 快照")).toBeVisible();
		await expect.element(text("已發布詢價")).toBeVisible();
		await expect.element(text("2026/08/20 20:36")).toBeVisible();
		await expect.element(text("等待供應商正式回價")).toBeVisible();
		await expect.element(text("付款成功後正式成立")).toBeVisible();
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
		await expect.element(text("尚未成立")).toBeVisible();
	});

	it("shows the accepted price while the order is still being created", async () => {
		const { container } = setup({
			counts: { responseCount: 3, shortlistCount: 1, error: null },
			acceptance: acceptedCreatingOrder,
		});
		await expect.element(text("NT$18,900")).toBeVisible();
		await expect.element(text("建立中")).toBeVisible();
		expect(container.textContent).not.toMatch(/已成交|已下單/);
	});

	it("describes recipients as intent, not delivery", async () => {
		const { container } = setup();
		await expect.element(text("這次發布的收件人")).toBeVisible();
		await expect
			.element(text("這三個數字是發布當下的收件人分布，不是投遞結果。"))
			.toBeVisible();
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
