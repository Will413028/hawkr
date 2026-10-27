/**
 * Apple Pay on Web — driver-based abstraction.
 *
 * Apple Pay is a sheet-driven payment state machine started by a user gesture.
 * `new ApplePaySession()` MUST be called synchronously inside the click
 * handler — any await before construction will throw on mobile WebKit:
 *
 *   "Must create a new ApplePaySession from a user gesture handler."
 *
 * The API is therefore a synchronous `openApplePaySheet(params, driver)` that
 * returns a sheet handle immediately. All async work (merchant validation
 * against our backend, caller's order creation + payment confirmation) happens
 * inside the sheet's lifecycle callbacks, NOT in the gesture chain.
 *
 * Architecture:
 * - `useNativePayAvailability` triggers capability detection at mount,
 *   caching the route hint so click-time `openApplePaySheet` can pick the
 *   right `ApplePaySession` class without awaiting.
 * - Two routes share the same `ApplePaySession` class after detection:
 *   native Safari (no SDK injection) and the Apple Pay JS SDK scan-code flow
 *   for desktop third-party browsers. The hint just records origin.
 * - Merchant IDs are per-env (sandbox vs prod) and MUST match backend's
 *   loaded cert (services/payment-service ApplePayProperties).
 * - W3C PaymentRequest path is intentionally NOT supported as a checkout
 *   route (Chromium emits payment-manifest console errors for Apple's method
 *   URL even when the SDK scan-code path works). The probe API
 *   `isApplePayUsableViaPaymentRequest` is kept for callers that want to
 *   check that surface directly (tests cover it).
 */

import { apiClient } from "@/lib/api/client";
import { env } from "@/lib/config/env";

/* ============================================================
 *                  Constants
 * ============================================================ */

/**
 * Apple Pay merchant identifier per environment.
 * - prod → "merchant.io.github.will413028.hawkr"
 * - dev → "merchant.io.github.will413028.hawkr.sandbox"
 *
 * Must match backend `APPLE_PAY_MERCHANT_ID` env + merchant identity cert
 * (services/payment-service ApplePayProperties). Each env needs its own
 * web-domain registration on Apple Developer console +
 * `.well-known/apple-developer-merchantid-domain-association` deployed.
 */
export function applePayMerchantId(): string {
	return env.flavor === "prod"
		? "merchant.io.github.will413028.hawkr"
		: "merchant.io.github.will413028.hawkr.sandbox";
}

const APPLE_PAY_JS_URL =
	"https://applepay.cdn-apple.com/jsapi/1.latest/apple-pay-sdk.js";
const APPLE_PAY_SCRIPT_ID = "hawkr-apple-pay-sdk";

/* ============================================================
 *                Apple Pay JS — types
 * ============================================================ */

interface ApplePaySessionClass {
	new (version: number, request: unknown): ApplePaySessionInstance;
	canMakePayments(): boolean;
	applePayCapabilities?(
		merchantId: string,
	): Promise<ApplePayPaymentCredentialStatusResponse>;
	canMakePaymentsWithActiveCard?(merchantId: string): Promise<boolean>;
	STATUS_SUCCESS: number;
	STATUS_FAILURE: number;
}

type ApplePayPaymentCredentialStatus =
	| "paymentCredentialsAvailable"
	| "paymentCredentialStatusUnknown"
	| "paymentCredentialsUnavailable"
	| "applePayUnsupported";

interface ApplePayPaymentCredentialStatusResponse {
	paymentCredentialStatus?: ApplePayPaymentCredentialStatus;
}

interface ApplePaySessionInstance {
	begin(): void;
	abort(): void;
	completeMerchantValidation(session: unknown): void;
	completePayment(result: { status: number }): void;
	onvalidatemerchant: ((event: { validationURL: string }) => void) | null;
	onpaymentauthorized:
		| ((event: { payment: { token: unknown } }) => void)
		| null;
	oncancel: ((event: Event) => void) | null;
}

interface AppleWindow extends Window {
	ApplePaySession?: ApplePaySessionClass;
}

function appleWindow(): AppleWindow | null {
	return typeof window === "undefined" ? null : (window as AppleWindow);
}

function applePaySessionClass(): ApplePaySessionClass | undefined {
	return appleWindow()?.ApplePaySession;
}

/* ============================================================
 *                SDK loading + route resolution
 * ============================================================ */

let applePaySdkPromise: Promise<ApplePaySessionClass | null> | null = null;
type ApplePayRouteKind = "native-session" | "sdk-session";
interface ApplePayRoute {
	kind: ApplePayRouteKind;
	cls: ApplePaySessionClass;
}
let applePayRouteHint: ApplePayRouteKind | null = null;

function loadApplePaySdk(): Promise<ApplePaySessionClass | null> {
	const existing = applePaySessionClass();
	if (existing) return Promise.resolve(existing);
	if (typeof window === "undefined" || typeof document === "undefined") {
		return Promise.resolve(null);
	}
	if (applePaySdkPromise) return applePaySdkPromise;

	applePaySdkPromise = new Promise<ApplePaySessionClass | null>(
		(resolve, reject) => {
			const complete = (script?: HTMLScriptElement) => {
				if (script) script.dataset.hawkrStatus = "loaded";
				const cls = applePaySessionClass();
				if (!cls) {
					applePaySdkPromise = null;
					reject(new Error("Apple Pay SDK 載入後未提供 ApplePaySession"));
					return;
				}
				resolve(cls);
			};
			const fail = () => {
				applePaySdkPromise = null;
				reject(new Error("無法載入 Apple Pay SDK"));
			};

			const currentScript = document.getElementById(
				APPLE_PAY_SCRIPT_ID,
			) as HTMLScriptElement | null;
			if (currentScript) {
				if (currentScript.dataset.hawkrStatus === "loaded") {
					queueMicrotask(() => complete(currentScript));
					return;
				}
				currentScript.addEventListener("load", () => complete(currentScript), {
					once: true,
				});
				currentScript.addEventListener("error", fail, { once: true });
				return;
			}

			const script = document.createElement("script");
			script.id = APPLE_PAY_SCRIPT_ID;
			script.src = APPLE_PAY_JS_URL;
			script.async = true;
			script.crossOrigin = "anonymous";
			script.dataset.hawkrStatus = "loading";
			script.addEventListener("load", () => complete(script), { once: true });
			script.addEventListener("error", fail, { once: true });
			document.head.appendChild(script);
		},
	);
	return applePaySdkPromise;
}

/**
 * Safari exposes `window.ApplePaySession` natively without us injecting the
 * Apple Pay JS SDK <script>. If our SDK script tag IS present the global came
 * from the SDK injection (third-party browser using the cross-device QR
 * flow), so it's NOT a reliable "this is Safari" signal — return null and
 * let the SDK branch decide.
 */
function nativeApplePaySession(): ApplePaySessionClass | null {
	if (
		typeof document !== "undefined" &&
		document.getElementById(APPLE_PAY_SCRIPT_ID)
	) {
		return null;
	}
	return applePaySessionClass() ?? null;
}

async function applePaySessionUsable(
	cls: ApplePaySessionClass,
): Promise<boolean> {
	// Preferred: applePayCapabilities() reports for both native Safari AND
	// third-party-browser cross-device flow. Must run before any
	// canMakePayments() gate — third-party browsers report false there even
	// when the QR hand-off IS available.
	if (cls.applePayCapabilities) {
		try {
			const capabilities = await cls.applePayCapabilities(applePayMerchantId());
			return capabilities.paymentCredentialStatus !== "applePayUnsupported";
		} catch {
			// Fall through to legacy Safari checks if the new API throws.
		}
	}

	// Legacy Safari path. `canMakePayments()` THROWS (not returns false) on
	// an insecure document — treat any throw as "not usable here" rather
	// than letting it escape and break the availability probe.
	try {
		if (!cls.canMakePayments?.()) return false;
		if (!cls.canMakePaymentsWithActiveCard) return true;
		try {
			return await cls.canMakePaymentsWithActiveCard(applePayMerchantId());
		} catch {
			// A hard rejection here usually means merchant/domain setup
			// trouble; keep Apple Pay visible so the sheet surfaces the real
			// error rather than silently hiding the row.
			return true;
		}
	} catch {
		return false;
	}
}

async function resolveApplePayRoute(): Promise<ApplePayRoute | null> {
	// Hint-cached fast path (subsequent calls after first detection).
	const cached = syncApplePayRoute();
	if (cached) return cached;

	// 1) Safari native ApplePaySession (no SDK injection needed).
	const native = nativeApplePaySession();
	if (native && (await applePaySessionUsable(native))) {
		applePayRouteHint = "native-session";
		return { kind: "native-session", cls: native };
	}

	// 2) Apple Pay JS SDK scan-code flow for third-party browsers.
	if (!native) {
		const cls = await loadApplePaySdk().catch(() => null);
		if (cls && (await applePaySessionUsable(cls))) {
			applePayRouteHint = "sdk-session";
			return { kind: "sdk-session", cls };
		}
	}

	return null;
}

/**
 * Synchronous route lookup from the cached hint set by capability detection.
 * Returns null if detection hasn't completed — the UI gates Apple Pay row
 * visibility on `nativePay.loaded`, so click-time misses should be rare.
 */
function syncApplePayRoute(): ApplePayRoute | null {
	if (applePayRouteHint === "native-session") {
		const cls = nativeApplePaySession();
		return cls ? { kind: "native-session", cls } : null;
	}
	if (applePayRouteHint === "sdk-session") {
		// Post-detection the SDK class lives on `window.ApplePaySession`; we
		// don't re-distinguish "did it come from the SDK script" here because
		// the class identity is what matters for `new ApplePaySession()`.
		const cls = applePaySessionClass();
		return cls ? { kind: "sdk-session", cls } : null;
	}
	return null;
}

/* ============================================================
 *                Capability detection (exported)
 * ============================================================ */

export async function isApplePayUsable(): Promise<boolean> {
	return Boolean(await resolveApplePayRoute());
}

/* W3C PaymentRequest probe (kept as direct API for callers / tests; NOT used
 * by the checkout route resolution). */

const APPLE_PAY_METHOD_URL = "https://apple.com/apple-pay";

function applePayPaymentRequestMethod(): PaymentMethodData {
	return {
		supportedMethods: APPLE_PAY_METHOD_URL,
		data: {
			version: 3,
			merchantIdentifier: applePayMerchantId(),
			merchantCapabilities: ["supports3DS", "supportsDebit", "supportsCredit"],
			supportedNetworks: ["visa", "masterCard", "jcb", "amex"],
			countryCode: "TW",
		},
	};
}

function supportsApplePayPaymentRequest(): boolean {
	return (
		typeof window !== "undefined" &&
		typeof window.PaymentRequest !== "undefined"
	);
}

export async function isApplePayUsableViaPaymentRequest(): Promise<boolean> {
	if (!supportsApplePayPaymentRequest()) return false;
	try {
		const request = new PaymentRequest(
			[applePayPaymentRequestMethod()],
			// canMakePayment() ignores the amount; a zero total is fine for probing.
			{
				total: {
					label: "Apple Pay",
					amount: { currency: "TWD", value: "0" },
				},
			},
		);
		return await request.canMakePayment();
	} catch {
		return false;
	}
}

/** Test-only reset for module-level Apple Pay routing state. */
export function __resetApplePayRoutingForTest() {
	applePaySdkPromise = null;
	applePayRouteHint = null;
}

/* ============================================================
 *                Backend merchant validation
 * ============================================================ */

/**
 * POST Apple's validationURL to our backend and return the parsed Apple
 * merchant session dictionary, ready to feed `completeMerchantValidation`.
 *
 * Backend response shape: `{ data: { merchantSessionJson: "<json string>" } }`.
 * The string is Apple's signed merchant session (PKCS#7); we must JSON.parse
 * before handing to ApplePaySession — passing the wrapper causes Apple to
 * silently dismiss the sheet (no signature / no identifier visible).
 */
async function validateApplePayMerchant(
	validationURL: string,
	displayName: string,
): Promise<unknown> {
	const envelope = await apiClient.authed.post<{ merchantSessionJson: string }>(
		"/payments/apple_pay/validate_merchant",
		{
			json: {
				validationURL,
				domainName: window.location.hostname,
				displayName,
			},
		},
	);
	return JSON.parse(envelope.data.merchantSessionJson);
}

/* ============================================================
 *                Driver pattern — sheet handle + open API
 * ============================================================ */

export interface ApplePayParams {
	amount: number;
	/** Product description shown on the sheet; also used as merchant `displayName`. */
	label: string;
}

export type ApplePayCompletion = "success" | "failure";

/**
 * Callbacks the caller supplies to orchestrate the sheet's async work.
 * All callbacks run AFTER the user gesture has elapsed (inside Apple's
 * sheet event lifecycle), so they may await freely.
 */
export interface ApplePayDriver {
	/**
	 * The user authorized payment. Caller completes its order / payment
	 * workflow with the token, then returns "success" or "failure" to tell
	 * Apple how to dismiss the sheet.
	 */
	onAuthorize(token: string): Promise<ApplePayCompletion>;
	/** The user cancelled the sheet (dismiss / Cancel button). */
	onCancel(): void;
	/**
	 * The sheet was aborted by an error: merchant validation failure, an
	 * authorize callback throwing, or programmatic `handle.abort()`. The
	 * sheet is dismissed before this fires; oncancel is suppressed so the
	 * caller does not double-handle.
	 */
	onError(error: Error): void;
}

export interface ApplePaySheetHandle {
	/** Programmatically dismiss the sheet (e.g., upstream confirm failed). */
	abort(): void;
}

/**
 * Open an Apple Pay sheet. MUST be called from a user gesture handler
 * (button click, etc.); the function is intentionally synchronous so the
 * `new ApplePaySession()` call happens in the same call stack as the
 * gesture.
 *
 * Returns null when no Apple Pay route is available (caller should fall
 * back to another payment method or display an error). This happens when
 * capability detection (`useNativePayAvailability`) hasn't completed or
 * concluded that no route is usable.
 */
export function openApplePaySheet(
	params: ApplePayParams,
	driver: ApplePayDriver,
): ApplePaySheetHandle | null {
	const route = syncApplePayRoute();
	if (!route) return null;

	const session = new route.cls(3, {
		countryCode: "TW",
		currencyCode: "TWD",
		supportedNetworks: ["visa", "masterCard", "jcb", "amex"],
		merchantCapabilities: ["supports3DS", "supportsDebit", "supportsCredit"],
		total: {
			label: params.label,
			amount: params.amount.toFixed(2),
			type: "final",
		},
	});

	// Tracks whether the sheet lifecycle has reached any terminal state
	// (merchant validation throw, authorize callback throw/return, or
	// programmatic abort). Once settled, any subsequent `oncancel` from the
	// Apple Pay SDK is a phantom event — Chrome's SDK scan-code flow fires
	// `oncancel` after `completePayment(FAILURE)`, Safari does not. Always
	// suppress it so the caller's already-set error message is not overwritten
	// by a misleading "user cancelled" UX.
	let settled = false;

	session.onvalidatemerchant = async (event) => {
		try {
			const merchantSession = await validateApplePayMerchant(
				event.validationURL,
				params.label,
			);
			session.completeMerchantValidation(merchantSession);
		} catch (e) {
			settled = true;
			session.abort();
			driver.onError(e instanceof Error ? e : new Error(String(e)));
		}
	};

	session.onpaymentauthorized = async (event) => {
		let result: ApplePayCompletion;
		try {
			// PayUni 契約（2026-06-06 實證確認）：caller 端送原始 paymentData
			// 物件的 JSON 字串（不含 paymentMethod / transactionIdentifier），
			// base64 由 backend payuni-java-client 統一處理。詳見
			// PayUni 金流官方 API 總覽 §4.6 與 11 號設計書 §7.4.1.1。
			const tokenObj = event.payment.token as { paymentData: unknown };
			const token = JSON.stringify(tokenObj.paymentData);
			result = await driver.onAuthorize(token);
		} catch (e) {
			settled = true;
			session.completePayment({ status: route.cls.STATUS_FAILURE });
			driver.onError(e instanceof Error ? e : new Error(String(e)));
			return;
		}
		// Authorize completed (success OR caller-reported failure). Mark
		// settled BEFORE completePayment so the phantom `oncancel` some SDKs
		// fire afterwards cannot overwrite caller's error message.
		settled = true;
		session.completePayment({
			status:
				result === "success"
					? route.cls.STATUS_SUCCESS
					: route.cls.STATUS_FAILURE,
		});
	};

	session.oncancel = () => {
		if (settled) return;
		driver.onCancel();
	};

	session.begin();

	return {
		abort: () => {
			// Caller-initiated abort; mark settled so the resulting oncancel
			// doesn't re-trigger driver.onCancel (caller is already handling
			// the failure that triggered the abort).
			settled = true;
			try {
				session.abort();
			} catch {
				// Session may already be aborted; ignore.
			}
		},
	};
}

/* ============================================================
 *                Error code mapping
 * ============================================================ */

/**
 * Translate backend Apple Pay merchant-validation failures (錯誤碼.md §5.11)
 * into a buyer-facing line. Returns null for non-Apple-Pay errors so
 * callers can fall back to their existing copy.
 *
 * - 8028 APPLE_PAY_VALIDATION_URL_REJECTED：白名單不符（後端設定，重試無用）
 * - 8029 APPLE_PAY_CERT_LOAD_FAILED：憑證載入失敗（後端問題，重試無用）
 * - 8030 APPLE_PAY_MERCHANT_VALIDATION_FAILED：merchant 驗證失敗（可能可重試）
 */
export function mapApplePayError(e: unknown): string | null {
	const code = (e as { code?: number } | null | undefined)?.code;
	switch (code) {
		case 8028:
			return "Apple Pay 服務暫時無法使用，請改用其他付款方式";
		case 8029:
			return "Apple Pay 暫時無法使用，請改用其他付款方式";
		case 8030:
			return "Apple Pay 驗證失敗，請重試一次或改用其他付款方式";
		default:
			return null;
	}
}
