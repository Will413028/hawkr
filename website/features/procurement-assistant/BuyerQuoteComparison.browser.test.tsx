import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { ViewTransitions } from "@/components/shared/view-transitions";
import { renderWithIntl } from "@/tests/browser/render";
import BuyerQuoteComparison from "./BuyerQuoteComparison";
import type {
	BuyerFacingSellerResponse,
	BuyerQuoteAcceptanceResponse,
} from "./buyer-quote-contract";

function buyerQuoteFailure(
	code: BuyerQuoteFailure["code"],
	detail: string,
	orderNo?: string,
): BuyerQuoteFailure {
	return { code, detail, retryable: false, ...(orderNo ? { orderNo } : {}) };
}

const STALE_RESPONSE_DETAIL =
	"這家賣家已經送出新的報價版本。系統不會自動幫你接受新版本，請重新比較之後再決定。";

// 付款入口是 `Button href`（共用 <Link>），它經 `@/lib/intl/use-router` 取 router；
// 測試不掛 App Router，改用靜態 router。
vi.mock("@/lib/intl/use-router", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/intl/use-router")>();
	const { staticRouter } = await import("@/tests/browser/static-router");
	return { ...actual, useRouter: () => staticRouter };
});

const SELLER_FACING_HASH = "b".repeat(64);

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
			specNote: "出貨前抽驗三箱",
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

type Props = React.ComponentProps<typeof BuyerQuoteComparison>;

function setup(overrides: Partial<Props> = {}) {
	const onAcceptQuote = vi.fn<Props["onAcceptQuote"]>(async () => ({
		ok: true as const,
		data: ack(),
	}));
	const props: Props = {
		responses: [response(), externalVendorCatty, supersededVersion],
		sellerFacingHash: SELLER_FACING_HASH,
		onToggleShortlist: vi.fn(async (_target, shortlisted: boolean) => ({
			ok: true as const,
			data: shortlisted,
		})),
		onAcceptQuote,
		createClientRequestId: () => "bqa-test",
		onRefresh: vi.fn(),
		...overrides,
	};
	// 付款入口的 <Link> 要有 root layout 的 ViewTransitions provider。
	const view = renderWithIntl(
		<ViewTransitions>
			<div style={{ width: "100%", maxWidth: 1100, padding: 16 }}>
				<BuyerQuoteComparison {...props} />
			</div>
		</ViewTransitions>,
	);
	return { ...view, props };
}

const acceptButton = () =>
	page.getByRole("button", { name: "接受這個版本", exact: true });
const confirmButton = () =>
	page.getByRole("button", { name: "確認接受這個版本", exact: true });

describe("BuyerQuoteComparison", () => {
	it("compares three quotes and keeps the superseded one unacceptable", async () => {
		setup();
		await expect
			.element(page.getByText("收到 3 份正式回價", { exact: true }))
			.toBeVisible();
		// 計價基準不同（KG vs CATTY）時必須明講不可直接比單價。
		await expect
			.element(
				page.getByText("各家的計價基準不同，單價不可直接相比", { exact: true }),
			)
			.toBeVisible();
		// 不可接受的版本留在清單裡，只是按不下去。
		await expect
			.element(page.getByText("已被新版本取代", { exact: true }))
			.toBeVisible();
		await expect.poll(() => acceptButton().elements().length).toBe(3);
		await expect.element(acceptButton().nth(0)).toBeEnabled();
		await expect.element(acceptButton().nth(2)).toBeDisabled();
		// 沒有任何自算單價：18900 / 500 = 37.8。
		await expect.element(page.getByText(/37\.8/)).not.toBeInTheDocument();
	});

	it("shows the version detail", async () => {
		setup({ responses: [externalVendorCatty] });
		await userEvent.click(
			page.getByRole("button", { name: "查看明細", exact: true }),
		);
		await expect
			.element(page.getByText("換算依據", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByText("1 台斤 = 0.6 公斤", { exact: true }))
			.toBeVisible();
		await expect
			.element(
				page.getByText("採收量受天候影響，可能短交一成", { exact: true }),
			)
			.toBeVisible();
		await expect
			.element(page.getByText("超出你的希望詢價區間", { exact: true }))
			.toBeVisible();
	});

	it("confirms before accepting and freezes the price without a payment link", async () => {
		const { props } = setup({ responses: [response()] });
		await userEvent.click(acceptButton());
		// 確認對話框走 portal 掛在 document.body 上；page 查整份 document。
		const confirm = confirmButton();
		await expect.element(confirm).toBeVisible();
		expect(props.onAcceptQuote).not.toHaveBeenCalled();
		await userEvent.click(confirm);
		await expect
			.element(
				page.getByText("已接受這個報價版本，成交價格已凍結，尚未付款", {
					exact: true,
				}),
			)
			.toBeVisible();
		await expect.element(page.getByText(/正在建立待付款訂單/)).toBeVisible();
		// ACK 不帶 orderNo，畫面就不得出現付款入口。
		await expect
			.element(page.getByRole("link", { name: "前往付款", exact: true }))
			.not.toBeInTheDocument();
		expect(props.onAcceptQuote).toHaveBeenCalledOnce();
		const command = vi.mocked(props.onAcceptQuote).mock.calls[0][0];
		expect(Object.keys(command).sort()).toEqual(["clientRequestId", "expectedSellerFacingHash", "expectedSellerResponseVersion", "sellerResponseId"]);
	});

	it("says the quote changed and does not accept a newer version on its own", async () => {
		const { props } = setup({
			responses: [response()],
			onAcceptQuote: vi.fn(async () => ({
				ok: false as const,
				error: buyerQuoteFailure("STALE_SELLER_RESPONSE", STALE_RESPONSE_DETAIL),
			})),
		});
		await userEvent.click(acceptButton());
		await userEvent.click(confirmButton());
		await expect
			.element(page.getByText("沒有接受這個報價版本", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByText(/系統不會自動幫你接受新版本/))
			.toBeVisible();
		expect(props.onAcceptQuote).toHaveBeenCalledOnce();
		await expect
			.element(
				page.getByText("已接受這個報價版本，成交價格已凍結，尚未付款", {
					exact: true,
				}),
			)
			.not.toBeInTheDocument();
	});

	it("shows the payment link only after the order is created", async () => {
		setup({
			responses: [response()],
			onAcceptQuote: vi.fn(async () => ({
				ok: true as const,
				data: ack({
					orderProvisioningState: "ORDER_CREATED",
					orderNo: "HK20260913001",
					paymentDeadline: "2026-09-14T03:00:00Z",
				}),
			})),
		});
		await userEvent.click(acceptButton());
		await userEvent.click(confirmButton());
		await expect
			.element(page.getByText("HK20260913001", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByText("付款期限", { exact: true }))
			.toBeVisible();
		const link = page.getByRole("link", { name: "前往付款", exact: true });
		await expect.poll(() => link.elements().length).toBeGreaterThan(0);
		await expect
			.element(link.nth(0))
			.toHaveAttribute("href", "/account/orders/HK20260913001");
	});

	it("says no quote has arrived yet", async () => {
		setup({ responses: [] });
		await expect
			.element(page.getByText("收到 0 份正式回價", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByText(/已發布不代表賣家已經看到或已經報價/))
			.toBeVisible();
	});
});
