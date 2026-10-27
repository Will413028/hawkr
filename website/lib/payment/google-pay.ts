/**
 * Google Pay on Web — driver-based abstraction.
 *
 * Mirrors apple-pay.ts: sheet is opened SYNCHRONOUSLY from a user gesture,
 * async work happens inside the driver callbacks. Google Pay's user-gesture
 * restriction (Chromium requires `loadPaymentData` to originate from a gesture
 * handler) is less strict than mobile Safari's, but the structural model is
 * the same — and consistency means callers don't have to reason about two
 * different shapes.
 *
 * Architecture:
 * - `useNativePayAvailability` triggers SDK load + capability detection at
 *   mount; the PaymentsClient is cached in a module-local so click-time
 *   `openGooglePaySheet` can call `loadPaymentData` synchronously.
 * - Google Pay environment (TEST/PRODUCTION) follows `env.flavor`; the
 *   gateway merchant id is fixed per Google's PayUni gateway registration.
 * - Cancellation is signalled by `loadPaymentData` rejecting with
 *   `statusCode: "CANCELED"`; routed to `driver.onCancel`. Any other reject
 *   goes through `driver.onError`.
 */

import { env } from "@/lib/config/env";

/* ============================================================
 *                  Constants
 * ============================================================ */

/**
 * Google Pay Business Console 發放的「Google 商家 ID」。**只在 PROD 環境**
 * 才會塞入 `merchantInfo.merchantId`；TEST 環境留空（Google Pay TEST SDK 對
 * merchantInfo.merchantId 寬鬆，prod 必填否則按鈕無法點）。
 *
 * 透過 env var 注入避免硬編碼且方便日後輪替。dev / TEST 環境 env 留空即可。
 */
const GOOGLE_PAY_BUSINESS_ID =
	process.env.NEXT_PUBLIC_GOOGLE_PAY_BUSINESS_ID ?? "";
export const GOOGLE_PAY_MERCHANT_NAME = "Hawkr";

/** Google Pay environment — sandbox on non-prod flavours. */
export function googlePayEnvironment(): "TEST" | "PRODUCTION" {
	return env.flavor === "prod" ? "PRODUCTION" : "TEST";
}

/**
 * 條件式組裝 `merchantInfo` — prod 必填 `merchantId`，TEST 留空。
 * 詳見 lib/payment/google-pay.ts §Constants 註解。
 */
function buildMerchantInfo() {
	const info: Record<string, string> = {
		merchantName: GOOGLE_PAY_MERCHANT_NAME,
	};
	if (env.flavor === "prod" && GOOGLE_PAY_BUSINESS_ID) {
		info.merchantId = GOOGLE_PAY_BUSINESS_ID;
	}
	return info;
}

const GOOGLE_PAY_JS_URL = "https://pay.google.com/gp/p/js/pay.js";

/* ============================================================
 *                Google Pay SDK — types
 * ============================================================ */

interface GooglePayClient {
	isReadyToPay(req: unknown): Promise<{ result?: boolean }>;
	loadPaymentData(req: unknown): Promise<{
		paymentMethodData?: {
			tokenizationData?: { token?: string };
		};
	}>;
}

interface GooglePayApi {
	PaymentsClient: new (opts: {
		environment: "TEST" | "PRODUCTION";
	}) => GooglePayClient;
}

/** `window.google` is already declared by `@types/google.accounts` for the
 * Identity Services SDK; we reach into the nested `payments.api` via an
 * `unknown` cast to avoid stomping that declaration. */
function getGooglePayApi(): GooglePayApi | undefined {
	if (typeof window === "undefined") return undefined;
	const g = (window as unknown as { google?: unknown }).google as
		| { payments?: { api?: GooglePayApi } }
		| undefined;
	return g?.payments?.api;
}

/* ============================================================
 *                SDK loading + cached client
 * ============================================================ */

let googleClientPromise: Promise<GooglePayClient> | null = null;
let googleClient: GooglePayClient | null = null;

function loadGooglePayClient(): Promise<GooglePayClient> {
	if (googleClientPromise) return googleClientPromise;
	if (typeof window === "undefined") {
		return Promise.reject(new Error("Google Pay 僅支援瀏覽器端"));
	}
	googleClientPromise = new Promise<GooglePayClient>((resolve, reject) => {
		const make = () => {
			const api = getGooglePayApi();
			if (!api?.PaymentsClient) {
				reject(new Error("Google Pay SDK 載入失敗"));
				return;
			}
			const client = new api.PaymentsClient({
				environment: googlePayEnvironment(),
			});
			googleClient = client;
			resolve(client);
		};
		if (getGooglePayApi()?.PaymentsClient) {
			make();
			return;
		}
		const script = document.createElement("script");
		script.src = GOOGLE_PAY_JS_URL;
		script.async = true;
		script.onload = make;
		script.onerror = () => {
			googleClientPromise = null; // allow retry on next call
			reject(new Error("無法載入 Google Pay SDK"));
		};
		document.head.appendChild(script);
	});
	return googleClientPromise;
}

/**
 * Tokenization spec 必填 `gatewayMerchantId`（PayUni 商店代號）— **動態值**，
 * 由 caller 從 backend preview/orderDetail 拿到 `paymentMerId` 後傳入。
 * 能力檢測 (`isReadyToPay`) 階段 gateway 不真的解密，使用 placeholder 即可。
 */
function googlePayCardMethod(gatewayMerchantId: string) {
	return {
		type: "CARD",
		tokenizationSpecification: {
			type: "PAYMENT_GATEWAY",
			parameters: {
				gateway: "payuni",
				gatewayMerchantId,
			},
		},
		parameters: {
			allowedCardNetworks: ["VISA", "MASTERCARD", "JCB", "AMEX"],
			allowedAuthMethods: ["PAN_ONLY", "CRYPTOGRAM_3DS"],
		},
	};
}

/* ============================================================
 *                Capability detection
 * ============================================================ */

async function probeGooglePayReady(): Promise<boolean> {
	const client = await loadGooglePayClient();
	// 能力檢測時 gatewayMerchantId 可以是任意值 — Google `isReadyToPay`
	// 只驗使用者/裝置是否可用 Google Pay（綁卡狀態），不打 PayUni gateway。
	// 實際開 sheet 時 caller 會傳入真實的 paymentMerId。
	const ready = await client.isReadyToPay({
		apiVersion: 2,
		apiVersionMinor: 0,
		allowedPaymentMethods: [googlePayCardMethod("PLACEHOLDER")],
	});
	return Boolean(ready.result);
}

export async function isGooglePayUsable(): Promise<boolean> {
	// Detection 失敗時必須清 cached promise 並重試一次:Google PaymentsClient
	// 在 sheet 用過後內部 state 偶爾會讓 `isReadyToPay` reject;舊版做法是
	// 直接 catch return false,接著模組級 promise 永久 cache,使用者非 full
	// refresh 不會恢復按鈕(對應 Bug 3 症狀)。重試 + reset cache 把 transient
	// 失敗收斂回可恢復狀態,真不能用才回 false。第二次失敗保留 log 留證據。
	try {
		return await probeGooglePayReady();
	} catch (firstError) {
		googleClientPromise = null;
		// 不清 googleClient:openGooglePaySheet 同步讀此值,保留 last-known
		// handle 直到 retry 結束。loadGooglePayClient 成功時會覆蓋 googleClient。
		try {
			return await probeGooglePayReady();
		} catch (secondError) {
			if (typeof console !== "undefined" && console.warn) {
				console.warn(
					"[google-pay] isReadyToPay failed after retry",
					firstError,
					secondError,
				);
			}
			return false;
		}
	}
}

/** Test-only reset for module-level Google Pay state. */
export function __resetGooglePayForTest() {
	googleClientPromise = null;
	googleClient = null;
}

/* ============================================================
 *                Driver pattern — sheet handle + open API
 * ============================================================ */

export interface GooglePayParams {
	amount: number;
	label?: string;
	/**
	 * PayUni 商店代號（必填）。從 backend preview / orderDetail response 的
	 * `paymentMerId` 拿到。Google SDK 用此值決定用哪把 PayUni 公鑰加密 token。
	 * 詳見 20 設計書 §4.13 和單與 paymentMerId 決策。
	 */
	gatewayMerchantId: string;
}

/**
 * Callbacks the caller supplies to orchestrate the sheet's async work.
 * All callbacks run AFTER the user gesture has elapsed (inside the
 * `loadPaymentData` Promise resolution), so they may await freely.
 */
export interface GooglePayDriver {
	/**
	 * User authorized payment; caller completes its order / payment workflow
	 * with the token. Throwing from this callback routes the failure through
	 * `onError` (sheet is already closed at this point).
	 */
	onAuthorize(token: string): Promise<void>;
	/** User cancelled the sheet. */
	onCancel(): void;
	/** Authorize callback threw, or token retrieval failed. */
	onError(error: Error): void;
}

/**
 * Marker handle returned when the sheet has been opened. Google Pay's
 * PaymentsClient does NOT expose a programmatic abort, so unlike
 * `ApplePaySheetHandle` there is no `abort()` method — the asymmetry is
 * intentional and reflects the SDK's actual capability.
 */
export interface GooglePaySheetHandle {
	readonly kind: "google-pay";
}

/**
 * Open a Google Pay sheet. MUST be called from a user gesture handler.
 *
 * Returns null when the cached PaymentsClient is missing (capability
 * detection in `useNativePayAvailability` should have warmed it; if not,
 * caller should display "Google Pay 不可用" and fall back).
 */
export function openGooglePaySheet(
	params: GooglePayParams,
	driver: GooglePayDriver,
): GooglePaySheetHandle | null {
	const client = googleClient;
	if (!client) return null;

	if (!params.gatewayMerchantId) {
		// Defensive guard — caller forgot to pass paymentMerId from preview.
		// Without it Google encrypts the token for an unknown merchant and
		// PayUni `/api/google` will reject; abort cleanly via driver.onError.
		driver.onError(
			new Error(
				"Google Pay 缺少 gatewayMerchantId (paymentMerId 未從 preview 取得)",
			),
		);
		return { kind: "google-pay" };
	}

	// loadPaymentData returns a Promise but the sheet appears synchronously
	// during the call — we do NOT await here so the call stays inside the
	// same gesture call stack.
	void client
		.loadPaymentData({
			apiVersion: 2,
			apiVersionMinor: 0,
			allowedPaymentMethods: [googlePayCardMethod(params.gatewayMerchantId)],
			merchantInfo: buildMerchantInfo(),
			transactionInfo: {
				countryCode: "TW",
				currencyCode: "TWD",
				totalPriceStatus: "FINAL",
				totalPrice: params.amount.toFixed(2),
			},
		})
		.then(async (data) => {
			const rawToken = data?.paymentMethodData?.tokenizationData?.token;
			if (!rawToken) {
				driver.onError(new Error("Google Pay 未回傳 token"));
				return;
			}
			try {
				// PayUni §5.6：GoogleObject 統一由 backend 做 Base64Encode，
				// caller 必須送原始 token JSON 字串，否則 backend base64 一次後變
				// 成雙重 base64，PayUni 解碼失敗。
				// 對應 PaymentClient.java createGooglePayPayment 的契約。
				await driver.onAuthorize(rawToken);
			} catch (e) {
				driver.onError(e instanceof Error ? e : new Error(String(e)));
			}
		})
		.catch((e: unknown) => {
			// Google Pay SDK rejects with `{ statusCode: "CANCELED" }` when the
			// user dismisses the sheet — route to onCancel so caller treats it
			// as a clean retry-able cancellation rather than a hard error.
			const code = (e as { statusCode?: string })?.statusCode ?? "";
			if (code === "CANCELED" || code === "CANCELLED") {
				driver.onCancel();
				return;
			}
			driver.onError(e instanceof Error ? e : new Error(String(e)));
		});

	return { kind: "google-pay" };
}
