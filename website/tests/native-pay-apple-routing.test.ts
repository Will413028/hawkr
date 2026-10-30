import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";

const realFetch = globalThis.fetch;
const globals = globalThis as Record<string, unknown>;

let fetchCalls: Array<{ url: string; init: RequestInit }>;

const fakeLocalStorage = {
	store: { "hkui:auth:active-role": "buyer" } as Record<string, string>,
	getItem(key: string) {
		return this.store[key] ?? null;
	},
	setItem(key: string, value: string) {
		this.store[key] = value;
	},
	removeItem(key: string) {
		delete this.store[key];
	},
};

function decodeToken(token: string): unknown {
	// caller 只送 paymentData 物件的 JSON 字串（2026-06-06 PayUni 實證契約），
	// base64 由後端 payuni-java-client 統一處理。
	return JSON.parse(token);
}

function installBrowserGlobals({
	ApplePaySession,
	PaymentRequest,
	sdkScriptPresent = false,
}: {
	ApplePaySession?: unknown;
	PaymentRequest?: unknown;
	sdkScriptPresent?: boolean;
}) {
	const windowStub: Record<string, unknown> = {
		ApplePaySession,
		PaymentRequest,
		location: { hostname: "web-dev.example.com" },
		dispatchEvent() {},
		localStorage: fakeLocalStorage,
	};
	globals.window = windowStub;
	globals.localStorage = fakeLocalStorage;
	if (PaymentRequest) globals.PaymentRequest = PaymentRequest;
	else delete globals.PaymentRequest;

	const listeners = new Map<string, () => void>();
	globals.document = {
		getElementById(id: string) {
			if (!sdkScriptPresent || id !== "hawkr-apple-pay-sdk") return null;
			return {
				dataset: { hawkrStatus: "loaded" },
				addEventListener() {},
			};
		},
		createElement() {
			return {
				id: "",
				src: "",
				async: false,
				crossOrigin: "",
				dataset: {},
				addEventListener(type: string, cb: () => void) {
					listeners.set(type, cb);
				},
			};
		},
		head: {
			appendChild() {
				queueMicrotask(() => listeners.get("load")?.());
			},
		},
	};
}

function makeApplePaySessionClass({
	token,
	beginCalls,
	completionStatuses,
}: {
	token: unknown;
	beginCalls: { count: number };
	completionStatuses: number[];
}) {
	return class FakeApplePaySession {
		static STATUS_SUCCESS = 1;
		static STATUS_FAILURE = 0;
		static canMakePayments() {
			return false;
		}
		static async applePayCapabilities() {
			return { paymentCredentialStatus: "paymentCredentialStatusUnknown" };
		}

		onvalidatemerchant: ((event: { validationURL: string }) => void) | null =
			null;
		onpaymentauthorized:
			| ((event: { payment: { token: unknown } }) => void)
			| null = null;
		oncancel: ((event: Event) => void) | null = null;

		begin() {
			beginCalls.count += 1;
			queueMicrotask(() => {
				this.onvalidatemerchant?.({
					validationURL: "https://apple-pay-gateway.test/session",
				});
			});
		}
		abort() {}
		completeMerchantValidation() {
			this.onpaymentauthorized?.({ payment: { token } });
		}
		completePayment(result: { status: number }) {
			completionStatuses.push(result.status);
		}
	};
}

function makePaymentRequestClass({
	canMakePayment,
	canMakePaymentCalls,
	showCalls,
}: {
	canMakePayment: boolean;
	canMakePaymentCalls?: { count: number };
	showCalls: { count: number };
}) {
	return class FakePaymentRequest {
		async canMakePayment() {
			if (canMakePaymentCalls) canMakePaymentCalls.count += 1;
			return canMakePayment;
		}

		async show() {
			showCalls.count += 1;
			// New architecture does not exercise this path during checkout; if
			// the test reaches here it's a regression.
			throw new Error("PaymentRequest.show should not be invoked");
		}
	};
}

beforeEach(() => {
	fetchCalls = [];
	__setFlavorForTest("dev");
	__setEnvForTest(null);
	globalThis.fetch = async (
		url: string | URL | Request,
		init: RequestInit = {},
	) => {
		fetchCalls.push({ url: String(url), init });
		// Mirror real backend contract: `data.merchantSessionJson` is a string
		// containing Apple's signed merchant session dict (PKCS#7). Frontend
		// JSON.parses it before handing to `completeMerchantValidation`.
		return new Response(
			JSON.stringify({
				code: 0,
				data: {
					merchantSessionJson: JSON.stringify({
						merchantSessionIdentifier: "merchant-session",
					}),
				},
			}),
			{ status: 200, headers: { "content-type": "application/json" } },
		);
	};
});

afterEach(async () => {
	const applePay = await import("../lib/payment/apple-pay");
	applePay.__resetApplePayRoutingForTest();
	globalThis.fetch = realFetch;
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete globals.window;
	delete globals.document;
	delete globals.localStorage;
	delete globals.PaymentRequest;
});

test("openApplePaySheet drives the SDK route end-to-end and resolves a token", async () => {
	const showCalls = { count: 0 };
	const canMakePaymentCalls = { count: 0 };
	const beginCalls = { count: 0 };
	const sessionCompletionStatuses: number[] = [];
	const sdkToken = { paymentData: { data: "sdk-token" } };

	installBrowserGlobals({
		ApplePaySession: makeApplePaySessionClass({
			token: sdkToken,
			beginCalls,
			completionStatuses: sessionCompletionStatuses,
		}),
		PaymentRequest: makePaymentRequestClass({
			canMakePayment: false,
			canMakePaymentCalls,
			showCalls,
		}),
		sdkScriptPresent: true,
	});

	const applePay = await import("../lib/payment/apple-pay");
	applePay.__resetApplePayRoutingForTest();

	// Warm the route hint as `useNativePayAvailability` does on mount.
	assert.equal(await applePay.isApplePayUsable(), true);

	// Drive the driver through openApplePaySheet; surface the token to the
	// test via a Promise wrapper. `queueMicrotask` defers the resolve so that
	// session.completePayment fires before assertions run.
	const token = await new Promise<string>((resolve, reject) => {
		const sheet = applePay.openApplePaySheet(
			{ amount: 120, label: "Hawkr 測試商品" },
			{
				onAuthorize: async (t) => {
					queueMicrotask(() => resolve(t));
					return "success";
				},
				onCancel: () => reject(new Error("unexpected cancel")),
				onError: (e) => reject(e),
			},
		);
		if (!sheet) reject(new Error("openApplePaySheet returned null"));
	});

	// caller 只送 paymentData JSON（不含 paymentMethod / transactionIdentifier）。
	assert.deepEqual(decodeToken(token), sdkToken.paymentData);
	assert.equal(beginCalls.count, 1);
	assert.deepEqual(sessionCompletionStatuses, [1]);
	assert.equal(
		canMakePaymentCalls.count,
		0,
		"SDK-capable browsers should not probe PaymentRequest",
	);
	assert.equal(showCalls.count, 0, "PaymentRequest.show must not run");
	assert.equal(fetchCalls.length, 1);
	assert.equal(
		fetchCalls[0].url,
		`${getEnv().apiBaseUrl}/app/payments/apple_pay/validate_merchant`,
	);
});

test("isApplePayUsable does not probe PaymentRequest when ApplePaySession is unavailable", async () => {
	const showCalls = { count: 0 };
	const canMakePaymentCalls = { count: 0 };

	installBrowserGlobals({
		PaymentRequest: makePaymentRequestClass({
			canMakePayment: true,
			canMakePaymentCalls,
			showCalls,
		}),
	});

	const applePay = await import("../lib/payment/apple-pay");
	applePay.__resetApplePayRoutingForTest();

	assert.equal(await applePay.isApplePayUsable(), false);
	assert.equal(
		canMakePaymentCalls.count,
		0,
		"background availability should not touch PaymentRequest",
	);
	assert.equal(showCalls.count, 0);
	assert.equal(fetchCalls.length, 0);
});

test("isApplePayUsableViaPaymentRequest remains available as a direct probe", async () => {
	const showCalls = { count: 0 };
	const canMakePaymentCalls = { count: 0 };

	installBrowserGlobals({
		PaymentRequest: makePaymentRequestClass({
			canMakePayment: true,
			canMakePaymentCalls,
			showCalls,
		}),
	});

	const applePay = await import("../lib/payment/apple-pay");
	applePay.__resetApplePayRoutingForTest();

	assert.equal(await applePay.isApplePayUsableViaPaymentRequest(), true);
	assert.equal(canMakePaymentCalls.count, 1);
	assert.equal(showCalls.count, 0);
	assert.equal(fetchCalls.length, 0);
});

test("openApplePaySheet returns null when no Apple Pay route is available", async () => {
	installBrowserGlobals({}); // no ApplePaySession, no PaymentRequest

	const applePay = await import("../lib/payment/apple-pay");
	applePay.__resetApplePayRoutingForTest();

	// No prior detection — hint is null, sync route lookup returns null.
	const sheet = applePay.openApplePaySheet(
		{ amount: 100, label: "test" },
		{
			onAuthorize: async () => "success",
			onCancel: () => {},
			onError: () => {},
		},
	);
	assert.equal(sheet, null);
});
