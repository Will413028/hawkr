import { expect, test } from "@playwright/test";
import {
	fulfillConnect,
	paymentMethod,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { installStripeStub } from "./helpers/mock-stripe";

/**
 * 付款步驟（`/orders/{orderNo}`）：Payment Element 由替身 Stripe.js 承載（`helpers/mock-stripe.ts`），
 * 結果只由 GetPayment 的回應決定。回應是 PaymentAppService 的 protobuf JSON 線上形狀（enum 是名稱字串、int64 是字串）。
 */

const ORDER_NO = "O-PAY-STEP-1";

const awaitingPayment = {
	payment: {
		orderNo: ORDER_NO,
		status: "PAYMENT_STATUS_AWAITING_PAYMENT",
		amount: "1280",
		session: {
			stripe: {
				clientSecret: "pi_test_secret_e2e",
				publishableKey: "pk_test_e2e",
			},
		},
	},
};

const withStatus = (status: string) => ({
	payment: { orderNo: ORDER_NO, status, amount: "1280" },
});

test("pay step goes to the success page only after GetPayment reports SUCCEEDED", {
	tag: ["@ui"],
}, async ({ page }) => {
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "300", roles: ["buyer"] },
	});
	await seedStoredActiveRole(page, "buyer");
	await installStripeStub(page);

	let reads = 0;
	await page.route(paymentMethod("CreatePayment"), (route) =>
		fulfillConnect(route, awaitingPayment),
	);
	await page.route(paymentMethod("GetPayment"), (route) => {
		reads += 1;
		return fulfillConnect(
			route,
			withStatus(
				reads < 2 ? "PAYMENT_STATUS_PROCESSING" : "PAYMENT_STATUS_SUCCEEDED",
			),
		);
	});

	await page.goto(`/orders/${ORDER_NO}`);
	await expect(page.getByTestId("stub-payment-element")).toBeVisible();
	await page.getByRole("button", { name: "確認付款" }).click();

	await expect(page).toHaveURL(/\/checkout\/success\?order=O-PAY-STEP-1/);
	const calls = await page.evaluate(() => window.__stripeConfirmCalls ?? []);
	expect(calls).toHaveLength(1);
	expect(calls[0].return_url).toContain("/payment/return?order=O-PAY-STEP-1");
	expect(reads).toBeGreaterThanOrEqual(2);
});

test("an ANOMALY payment shows the support message and no payment form", {
	tag: ["@ui"],
}, async ({ page }) => {
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "300", roles: ["buyer"] },
	});
	await seedStoredActiveRole(page, "buyer");
	await installStripeStub(page);
	await page.route(paymentMethod("CreatePayment"), (route) =>
		fulfillConnect(route, withStatus("PAYMENT_STATUS_ANOMALY")),
	);

	await page.goto(`/orders/${ORDER_NO}`);
	await expect(page.getByText("這筆付款需要人工處理")).toBeVisible();
	await expect(page.getByTestId("stub-payment-element")).toHaveCount(0);
});

test("the return page ignores a success status in the URL and waits for GetPayment", {
	tag: ["@ui"],
}, async ({ page }) => {
	await setupGatewayMocks(page, {
		session: "identity",
		identity: { accountId: "300", roles: ["buyer"] },
	});
	await seedStoredActiveRole(page, "buyer");
	await installStripeStub(page);
	await page.route(paymentMethod("GetPayment"), (route) =>
		fulfillConnect(route, withStatus("PAYMENT_STATUS_PROCESSING")),
	);

	await page.goto(
		`/payment/return?order=${ORDER_NO}&redirect_status=succeeded&status=1`,
	);
	await expect(page.getByText("確認付款中…").first()).toBeVisible();
	await expect(page).not.toHaveURL(/checkout\/success/);
});
