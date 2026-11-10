/**
 * 收款帳戶開通的流程（純邏輯，元件只負責接線）。
 *
 * - HOSTED：呼叫 `CreatePaymentOnboardingSession`，整頁導到金流商的連結；回到 `?payment=return` 時先呼叫
 *   `RefreshMyPaymentAccount`（伺服器同步向金流商重讀，D33）再讀帳戶與開通摘要（`confirmPaymentReturn`），不輪詢；`resource_exhausted`
 *   帶 RetryInfo 由 transport 的 retryInterceptor 重試；回到 `?payment=refresh`（連結過期）時重新取得連結再導。
 * - EMBEDDED：不在這裡建立 session；內嵌元件自己呼叫 `fetchClientSecret`（每次都是新的 session）。
 */

import { OnboardingSurface } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { OnboardingSession } from "./payment-account";

/** 開通完成與離開都回到認證頁，用 query 區分。 */
export const PAYMENT_PARAM = "payment";
export type PaymentParam = "return" | "refresh";

export function paymentParamOf(value: string | null): PaymentParam | null {
	return value === "return" || value === "refresh" ? value : null;
}

/**
 * 金流商要回跳的絕對網址。`localizedPath` 是已經帶語系前綴的認證頁路徑（`useLocalizedHref` 的結果），
 * `origin` 是目前頁面的 origin。
 */
export function onboardingUrls(
	origin: string,
	localizedPath: string,
): { returnUrl: string; refreshUrl: string } {
	return {
		returnUrl: `${origin}${localizedPath}?${PAYMENT_PARAM}=return`,
		refreshUrl: `${origin}${localizedPath}?${PAYMENT_PARAM}=refresh`,
	};
}

export type OnboardingStart =
	| { kind: "redirect"; url: string }
	| { kind: "embedded" };

interface StartDeps {
	createSession: () => Promise<OnboardingSession>;
}

/**
 * 依伺服器告知的介面開始開通。介面不是 HOSTED 或 EMBEDDED（沒有設定收款開通、不認得的值）丟錯：
 * 不猜一個預設介面。
 */
export async function startOnboarding(
	surface: OnboardingSurface,
	deps: StartDeps,
): Promise<OnboardingStart> {
	switch (surface) {
		case OnboardingSurface.HOSTED: {
			const session = await deps.createSession();
			if (session.surface !== "hosted") {
				throw new Error("開通介面與伺服器告知的不一致");
			}
			return { kind: "redirect", url: session.url };
		}
		case OnboardingSurface.EMBEDDED:
			return { kind: "embedded" };
		default:
			throw new Error("不認得的開通介面");
	}
}

/** 內嵌元件的 `fetchClientSecret`：每次都建立新的 session。 */
export async function embeddedClientSecret(
	createSession: () => Promise<OnboardingSession>,
): Promise<string> {
	const session = await createSession();
	if (session.surface !== "embedded") {
		throw new Error("開通介面與伺服器告知的不一致");
	}
	return session.clientSecret;
}

interface ConfirmDeps {
	/** 呼叫 `RefreshMyPaymentAccount`：伺服器同步重讀帳戶。 */
	refreshAccount: () => Promise<void>;
	/** 重新讀帳戶與開通摘要（讓快取與畫面更新）。 */
	readAccount: () => Promise<unknown>;
}

/**
 * 回到網頁後與狀態按鈕共用的一條路徑：先重讀（一次）再讀。重讀失敗也照樣讀（快取不能留在舊的），讀完再把重讀的
 * 錯誤丟出去，讓畫面顯示伺服器的句子。
 */
export async function confirmPaymentReturn(deps: ConfirmDeps): Promise<void> {
	let refreshFailure: { error: unknown } | null = null;
	try {
		await deps.refreshAccount();
	} catch (error) {
		refreshFailure = { error };
	}
	await deps.readAccount();
	if (refreshFailure) throw refreshFailure.error;
}
