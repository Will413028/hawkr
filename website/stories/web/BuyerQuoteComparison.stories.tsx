import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { ViewTransitions } from "@/components/shared/view-transitions";
import BuyerQuoteComparison from "@/features/procurement-assistant/BuyerQuoteComparison";
import type {
	BuyerFacingSellerResponse,
	BuyerQuoteAcceptanceResponse,
} from "@/features/procurement-assistant/buyer-quote-contract";
import { buyerQuoteFailure } from "@/features/procurement-assistant/buyer-quote-contract";

/**
 * 買家端報價比較與接受（issue #2538 A8 前半）。
 *
 * 這個面板目前沒有掛在任何路由上——A8 的三區版面要等 runtime-snapshot DTO 定案，
 * 所以瀏覽器層的證據由這幾個 story 承擔，而不是 Playwright。所有資料都是固定
 * fixture，不建立詢價、不接受任何真實報價、也不代表現行行情。
 */

const SNAPSHOT_HASH = "b".repeat(64);

function response(
	overrides: Partial<BuyerFacingSellerResponse> = {},
	quoteOverrides: Partial<BuyerFacingSellerResponse["quote"]> = {},
): BuyerFacingSellerResponse {
	return {
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		status: "SUBMITTED",
		payloadHash: "a".repeat(64),
		submittedAt: "2026-09-13T02:00:00Z",
		seller: {
			displayName: "範例鮮果合作社",
			partyType: "PLATFORM_SELLER",
			storeCode: "greenfield",
		},
		quote: {
			responseType: "MANUAL_QUOTE",
			offeredQuality: { grade: "特級", inspectionNotes: "出貨前抽驗三箱" },
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "500",
			offeredQuantityUnit: "KG",
			conversionBasis: "",
			currency: "TWD",
			goodsAmount: "18000",
			shippingAmount: "0",
			taxAmount: "900",
			taxInclusionBasis: "TAX_EXCLUDED",
			totalAmount: "18900",
			leadTimeDays: "3",
			promisedDeliveryAt: "2026-09-20T02:00:00Z",
			validUntil: "2026-09-18T02:00:00Z",
			termsNote: "需自備棧板",
			deviationDisclosure: "",
			...quoteOverrides,
		},
		outsideBuyerRequestedRange: false,
		shortlisted: false,
		acceptability: "ACCEPTABLE",
		...overrides,
	};
}

const externalVendorCatty = response(
	{
		sellerResponseId: "1234567890123456789",
		sellerResponseVersion: "1234567890123456792",
		submittedAt: "2026-09-13T04:30:00Z",
		seller: {
			displayName: "豐田產地直送",
			partyType: "EXTERNAL_VENDOR",
			storeCode: "",
		},
		outsideBuyerRequestedRange: true,
	},
	{
		unitBasis: { quantity: "1", unit: "CATTY" },
		offeredQuantityUnit: "CATTY",
		conversionBasis: "1 台斤 = 0.6 公斤",
		goodsAmount: "12600",
		shippingAmount: "800",
		taxAmount: "0",
		taxInclusionBasis: "TAX_INCLUDED",
		totalAmount: "13400",
		leadTimeDays: "5",
		deviationDisclosure: "採收量受天候影響，可能短交一成",
	},
);

const supersededVersion = response({
	sellerResponseId: "1234567890123456790",
	sellerResponseVersion: "1234567890123456793",
	submittedAt: "2026-09-12T09:00:00Z",
	seller: {
		displayName: "山城果園",
		partyType: "PLATFORM_SELLER",
		storeCode: "mountain",
	},
	acceptability: "SUPERSEDED",
});

function ack(
	overrides: Partial<BuyerQuoteAcceptanceResponse> = {},
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: "1234567890123456805",
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		acceptedDealPrice: "18900",
		currency: "TWD",
		acceptedAt: "2026-09-13T06:00:00Z",
		orderProvisioningState: "CREATING_ORDER",
		orderNo: "",
		paymentDeadline: "",
		replayed: false,
		...overrides,
	};
}

const meta = {
	title: "Web/Procurement/BuyerQuoteComparison",
	component: BuyerQuoteComparison,
	args: {
		responses: [response(), externalVendorCatty, supersededVersion],
		rfqSnapshotHash: SNAPSHOT_HASH,
		onToggleShortlist: fn(async (_target, shortlisted: boolean) => ({
			ok: true as const,
			data: shortlisted,
		})),
		onAcceptQuote: fn(async () => ({ ok: true as const, data: ack() })),
		createIdempotencyKey: () => "bqa-storybook",
		onRefresh: fn(),
	},
	parameters: {
		docs: {
			description: {
				component:
					"賣家正式回價的比較表、單一版本明細、exact-version 接受確認與接受後狀態。可否接受完全由伺服器的 acceptability 決定，畫面不比對有效期也不比版本號；不可接受的版本留在清單供稽核。不同計價基準之間不提供任何自算單價。固定 fixture，不建立詢價也不接受真實報價。",
			},
		},
	},
	decorators: [
		// 付款入口是 `Button href`，也就是共用的 <Link>；它從 root layout 的
		// ViewTransitions provider 取 context，隔離預覽必須自己補上這一層。
		(Story) => (
			<ViewTransitions>
				<div style={{ width: "100%", maxWidth: 1100, padding: 16 }}>
					<Story />
				</div>
			</ViewTransitions>
		),
	],
} satisfies Meta<typeof BuyerQuoteComparison>;

export default meta;
type Story = StoryObj<typeof BuyerQuoteComparison>;

export const Comparison: Story = {
	name: "比較三份回價",
	play: async ({ canvas }) => {
		await expect(canvas.getByText("收到 3 份正式回價")).toBeVisible();
		// 計價基準不同（KG vs CATTY）時必須明講不可直接比單價。
		await expect(
			canvas.getByText("各家的計價基準不同，單價不可直接相比"),
		).toBeVisible();
		// 不可接受的版本留在清單裡，只是按不下去。
		await expect(canvas.getByText("已被新版本取代")).toBeVisible();
		const acceptButtons = canvas.getAllByRole("button", {
			name: "接受這個版本",
		});
		await expect(acceptButtons[0]).toBeEnabled();
		await expect(acceptButtons[2]).toBeDisabled();
		// ⛔ 沒有任何自算單價：18900 / 500 = 37.8。
		await expect(canvas.queryByText(/37\.8/)).not.toBeInTheDocument();
	},
};

export const VersionDetail: Story = {
	name: "單一版本明細",
	args: { responses: [externalVendorCatty] },
	play: async ({ canvas }) => {
		await userEvent.click(canvas.getByRole("button", { name: "查看明細" }));
		await expect(canvas.getByText("換算依據")).toBeVisible();
		await expect(canvas.getByText("1 台斤 = 0.6 公斤")).toBeVisible();
		await expect(
			canvas.getByText("採收量受天候影響，可能短交一成"),
		).toBeVisible();
		await expect(canvas.getByText("超出你的希望詢價區間")).toBeVisible();
	},
};

export const AcceptConfirmation: Story = {
	name: "接受確認與價格凍結",
	args: { responses: [response()] },
	play: async ({ canvas, canvasElement, args }) => {
		// 確認對話框走 portal 掛在 document.body 上，不在 story 的 canvas 裡。
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(canvas.getByRole("button", { name: "接受這個版本" }));
		const confirm = await body.findByRole("button", {
			name: "確認接受這個版本",
		});
		await expect(args.onAcceptQuote).not.toHaveBeenCalled();
		await userEvent.click(confirm);
		await waitFor(() =>
			expect(
				canvas.getByText("已接受這個報價版本，成交價格已凍結，尚未付款"),
			).toBeVisible(),
		);
		await expect(canvas.getByText(/正在建立待付款訂單/)).toBeVisible();
		// ACK 不帶 orderNo，畫面就不得出現付款入口。
		await expect(
			canvas.queryByRole("link", { name: "前往付款" }),
		).not.toBeInTheDocument();
		const command = (
			args.onAcceptQuote as unknown as {
				mock: { calls: Array<[{ [key: string]: string }]> };
			}
		).mock.calls[0][0];
		await expect(Object.keys(command).sort()).toEqual([
			"expectedRfqSnapshotHash",
			"expectedSellerResponseVersion",
			"idempotencyKey",
			"sellerResponseId",
		]);
	},
};

export const StaleSellerResponse: Story = {
	name: "報價已變動時明講，不自動接受新版",
	args: {
		responses: [response()],
		onAcceptQuote: fn(async () => ({
			ok: false as const,
			error: buyerQuoteFailure(409, "STALE_SELLER_RESPONSE"),
		})),
	},
	play: async ({ canvas, canvasElement, args }) => {
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(canvas.getByRole("button", { name: "接受這個版本" }));
		await userEvent.click(
			await body.findByRole("button", { name: "確認接受這個版本" }),
		);
		await waitFor(() =>
			expect(canvas.getByText("沒有接受這個報價版本")).toBeVisible(),
		);
		await expect(canvas.getByText(/系統不會自動幫你接受新版本/)).toBeVisible();
		await expect(args.onAcceptQuote).toHaveBeenCalledOnce();
		await expect(
			canvas.queryByText("已接受這個報價版本，成交價格已凍結，尚未付款"),
		).not.toBeInTheDocument();
	},
};

export const OrderCreated: Story = {
	name: "訂單建立後才出現付款入口",
	args: {
		responses: [response()],
		onAcceptQuote: fn(async () => ({
			ok: true as const,
			data: ack({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260913001",
				paymentDeadline: "2026-09-14T03:00:00Z",
			}),
		})),
	},
	play: async ({ canvas, canvasElement }) => {
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(canvas.getByRole("button", { name: "接受這個版本" }));
		await userEvent.click(
			await body.findByRole("button", { name: "確認接受這個版本" }),
		);
		await waitFor(() =>
			expect(canvas.getByText("HK20260913001")).toBeVisible(),
		);
		await expect(canvas.getByText("付款期限")).toBeVisible();
		await expect(
			canvas.getAllByRole("link", { name: "前往付款" })[0],
		).toHaveAttribute("href", "/account/orders/HK20260913001");
	},
};

export const Empty: Story = {
	name: "尚未收到回價",
	args: { responses: [] },
	play: async ({ canvas }) => {
		await expect(canvas.getByText("收到 0 份正式回價")).toBeVisible();
		await expect(
			canvas.getByText(/已發布不代表賣家已經看到或已經報價/),
		).toBeVisible();
	},
};
