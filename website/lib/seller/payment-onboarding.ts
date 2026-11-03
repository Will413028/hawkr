/**
 * 收款帳戶開通的流程（純邏輯，元件只負責接線）。
 *
 * - HOSTED：呼叫 `CreatePaymentOnboardingSession`，整頁導到金流商的連結；回到 `?payment=return` 時重讀帳戶並輪詢幾次
 *   （狀態由金流商的事件非同步同步，剛回來時可能還是舊的）；回到 `?payment=refresh`（連結過期）時重新取得連結再導。
 * - EMBEDDED：不在這裡建立 session；內嵌元件自己呼叫 `fetchClientSecret`（每次都是新的 session）。
 */

import {
	OnboardingSurface,
	type PaymentAccount,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
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

/** 回到網頁之後狀態已經離開「待設定」與「準備中」：不必再等同步。 */
export function paymentStatusSettled(account: PaymentAccount | null): boolean {
	switch (account?.status) {
		case PaymentAccountStatus.UNDER_REVIEW:
		case PaymentAccountStatus.ENABLED:
		case PaymentAccountStatus.DISABLED:
			return true;
		default:
			return false;
	}
}

export const CONFIRM_ATTEMPTS = 5;
export const CONFIRM_INTERVAL_MS = 3_000;

interface ConfirmDeps {
	/** 重讀帳戶並把結果交給畫面（寫進快取）。 */
	readAccount: () => Promise<PaymentAccount | null>;
	sleep: (ms: number) => Promise<void>;
	attempts?: number;
	intervalMs?: number;
}

/** 回到網頁後重讀帳戶，最多 `attempts` 次，狀態離開「待設定」就停；回傳最後讀到的帳戶。 */
export async function confirmPaymentReturn(
	deps: ConfirmDeps,
): Promise<PaymentAccount | null> {
	const attempts = deps.attempts ?? CONFIRM_ATTEMPTS;
	const interval = deps.intervalMs ?? CONFIRM_INTERVAL_MS;
	let last: PaymentAccount | null = null;
	for (let i = 0; i < attempts; i++) {
		if (i > 0) await deps.sleep(interval);
		last = await deps.readAccount();
		if (paymentStatusSettled(last)) break;
	}
	return last;
}

export type ParamOutcome =
	| { kind: "confirmed"; account: PaymentAccount | null }
	| { kind: "redirect"; url: string }
	| { kind: "embedded" };

/**
 * 網址上的 `?payment=`：`return` 重讀帳戶，`refresh` 重新開始開通（連結過期，HOSTED 取得新連結）。
 * `refresh` 需要伺服器告知的介面；沒有介面（帳戶還沒讀到或沒有設定）丟錯。
 */
export async function handlePaymentParam(
	param: PaymentParam,
	deps: ConfirmDeps & StartDeps & { surface: OnboardingSurface | undefined },
): Promise<ParamOutcome> {
	if (param === "return") {
		return { kind: "confirmed", account: await confirmPaymentReturn(deps) };
	}
	return startOnboarding(deps.surface ?? OnboardingSurface.UNSPECIFIED, deps);
}
