import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, within } from "storybook/test";
import type { BuyerQuoteAcceptanceResponse } from "@/features/procurement-assistant/buyer-quote-contract";
import QuoteBatchSummary from "@/features/procurement-assistant/QuoteBatchSummary";
import {
	type PublicationResultView,
	publicationResultDataSchema,
} from "@/features/procurement-assistant/runtime-snapshot-contract";

/**
 * 買家端「詢價已發布」三區版面（稿 R-02，issue #2538 A8 後半）。
 *
 * 這個版面在 `CandidateCanvas` 的 quote-invites 檢視底下，入口是「發布 execute
 * 成功」。全部固定 fixture：不建立詢價、不發布、也不代表現行行情。
 *
 * ⛔ 第三張卡的空狀態是這個版面存在的理由（v6.0 §V.11.4 Delivery truth）：
 * 沒有回價就顯示 0 筆、成交價格「尚未成立」、訂單「尚未建立」。story 的 play
 * 就是在守這件事。
 *
 * ⛔ 面板上沒有撤回入口：`quote-batches/{id}/withdraw` 是 doc 64 §7 明列退場的
 * legacy 端點，publication 側沒有對應的撤回，所以不畫一顆停用的按鈕假裝有。
 */

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

const meta = {
	title: "Web/Procurement/QuoteBatchSummary",
	component: QuoteBatchSummary,
	args: {
		slots: {
			item: "蘋果",
			quantity: 100,
			unit: "公斤",
			deliveryCity: "臺北市",
			deliveryDate: "2026-08-25",
		},
		qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
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
		onViewSnapshot: fn(),
		onCompareQuotes: fn(),
	},
	parameters: {
		docs: {
			description: {
				component:
					"詢價快照、六步驟流程進度與「目前交易真相」三個區塊。沒有回價時誠實顯示 0 筆、成交價格尚未成立、訂單尚未建立；讀不到的事實顯示「尚未取得」而不是 0。固定 fixture，不建立也不發布任何詢價。",
			},
		},
	},
	decorators: [
		(Story) => (
			<div style={{ width: "100%", maxWidth: 1100, padding: 16 }}>
				<Story />
			</div>
		),
	],
} satisfies Meta<typeof QuoteBatchSummary>;

export default meta;
type Story = StoryObj<typeof QuoteBatchSummary>;

export const PublishedNoQuotes: Story = {
	name: "已發布・尚未收到回價",
	play: async ({ canvas, canvasElement }) => {
		await expect(
			canvas.getByRole("heading", { name: "詢價已發布" }),
		).toBeVisible();
		await expect(
			canvas.getByText("已建立 1 組詢價・5 筆邀請（100 公斤）"),
		).toBeVisible();
		await expect(canvas.getByText("0 筆")).toBeVisible();
		await expect(canvas.getByText("尚未成立")).toBeVisible();
		await expect(canvas.getByText("尚未建立")).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "比較供應商回價" }),
		).toBeDisabled();
		// ⛔ 沒有回價就不得出現任何成交語氣。
		await expect(canvasElement.textContent).not.toMatch(/已成交|已下單/);
	},
};

export const ProgressTimeline: Story = {
	name: "六步驟流程進度",
	play: async ({ canvas }) => {
		await expect(canvas.getByText("已確認 RFQ 快照")).toBeVisible();
		await expect(canvas.getByText("已發布詢價")).toBeVisible();
		await expect(canvas.getByText("2026/08/20 20:36")).toBeVisible();
		await expect(canvas.getByText("等待供應商正式回價")).toBeVisible();
		await expect(canvas.getByText("付款成功後正式成立")).toBeVisible();
		// 第一步沒有伺服器時間戳可寫（後端目前不產 CONFIRMED_RFQ artifact），
		// 所以是「已完成」而不是一個補出來的時間。
		await expect(canvas.getAllByText("已完成")).toHaveLength(1);
	},
};

export const QuotesReceived: Story = {
	name: "收到回價後開啟比較",
	args: { counts: { responseCount: 3, shortlistCount: 1, error: null } },
	play: async ({ canvas }) => {
		await expect(canvas.getByText("3 筆")).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "比較供應商回價" }),
		).toBeEnabled();
		// 成交價格仍未成立：接受回執才是唯一來源。
		await expect(canvas.getByText("尚未成立")).toBeVisible();
	},
};

export const AcceptedCreatingOrder: Story = {
	name: "已接受・訂單建立中",
	args: {
		counts: { responseCount: 3, shortlistCount: 1, error: null },
		acceptance: acceptedCreatingOrder,
	},
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.getByText("NT$18,900")).toBeVisible();
		await expect(canvas.getByText("建立中")).toBeVisible();
		// ⛔ D3：CREATING_ORDER 不得渲染成已成交或已下單，也不得憑空長出單號。
		await expect(canvasElement.textContent).not.toMatch(/已成交|已下單/);
	},
};

export const RecipientBreakdown: Story = {
	name: "收件人分布是意圖不是投遞結果",
	play: async ({ canvas, canvasElement }) => {
		await expect(canvas.getByText("這次發布的收件人")).toBeVisible();
		await expect(
			canvas.getByText("這三個數字是發布當下的收件人分布，不是投遞結果。"),
		).toBeVisible();
		// ⛔ queuedCount 不得被講成「已送出」；面板上也沒有任何撤回動作。
		// （「已送達」只出現在既有那句否定式界線句裡，不在這條擋的範圍。）
		await expect(canvasElement.textContent).not.toMatch(/已送出|已寄送/);
		await expect(canvasElement.textContent).not.toMatch(/撤回|撤銷/);
	},
};

export const WishPoolZeroCounts: Story = {
	name: "許願池三欄皆為 0",
	args: { publication: publicationWith() },
	play: async ({ canvas }) => {
		const recipients = canvas.getByRole("region", {
			name: "這次發布的收件人",
		});
		// 0 是一次成功發布的結論，不是讀不到——所以照樣顯示，不整區藏起來。
		// 範圍鎖在這一區：第三張卡的 shortlist 也是 0，不能混進來當證據。
		await expect(within(recipients).getAllByText("0")).toHaveLength(3);
	},
};

export const CountsUnavailable: Story = {
	name: "筆數讀不到時說尚未取得",
	args: {
		counts: {
			responseCount: null,
			shortlistCount: null,
			error: {
				code: "SERVER_ERROR",
				detail: "伺服器暫時無法處理，請稍後再試。",
				retryable: true,
			},
		},
	},
	play: async ({ canvas }) => {
		await expect(canvas.getAllByText("尚未取得")).toHaveLength(2);
		await expect(
			canvas.getByText("伺服器暫時無法處理，請稍後再試。"),
		).toBeVisible();
	},
};

export const NotPublished: Story = {
	name: "尚未取得發布結果",
	args: {
		publication: null,
		counts: { responseCount: null, shortlistCount: null, error: null },
		buyerRequestedRange: null,
	},
	play: async ({ canvas }) => {
		await expect(
			canvas.getByRole("heading", { name: "詢價建立摘要" }),
		).toBeVisible();
		await expect(canvas.queryByText("這次發布的收件人")).toBeNull();
		await expect(
			canvas.getByText("後端未提供不可變核准證據；此畫面不代表核准已完成。"),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: /檢視不可變 RFQ 快照/ }),
		).toBeDisabled();
	},
};
