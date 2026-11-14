/**
 * 收款帳戶開通的流程（純邏輯，元件只負責接線）。
 *
 * 開始開通只依伺服器回應的 session 形狀行動：
 * - hosted：整頁導到金流商的連結；回到 `?payment=return` 時先呼叫
 *   `RefreshMyPaymentAccount`（伺服器同步向金流商重讀，D33）再讀帳戶與開通摘要（`confirmPaymentReturn`），不輪詢；重讀失敗不顯示；回到 `?payment=refresh`（連結過期）時重新取得連結再導。
 * - embedded：session 帶金流商代碼與公開金鑰（元件要它們才能初始化）；第一次 `fetchClientSecret`
 *   用這個 session 的 client secret，之後每次都是新的 session。
 */

import type { OnboardingSession } from "./payment-account";

/** 開通完成與離開都回到認證頁，用 query 區分。 */
export const PAYMENT_PARAM = "payment";
export type PaymentParam = "return" | "refresh";

export function paymentParamOf(value: string | null): PaymentParam | null {
	return value === "return" || value === "refresh" ? value : null;
}

export type OnboardingStart =
	| { kind: "redirect"; url: string }
	| {
			kind: "embedded";
			provider: "stripe";
			publishableKey: string;
			fetchClientSecret: () => Promise<string>;
	  };

interface StartDeps {
	createSession: () => Promise<OnboardingSession>;
}

/** 依伺服器回的 session 形狀開始開通；不是兩種已知形狀之一（較新的伺服器）丟錯，不猜。 */
export async function startOnboarding(
	deps: StartDeps,
): Promise<OnboardingStart> {
	const session = await deps.createSession();
	switch (session.kind) {
		case "hosted":
			return { kind: "redirect", url: session.url };
		case "embedded": {
			let first: string | null = session.clientSecret;
			return {
				kind: "embedded",
				provider: session.provider,
				publishableKey: session.publishableKey,
				fetchClientSecret: () => {
					if (first !== null) {
						const secret = first;
						first = null;
						return Promise.resolve(secret);
					}
					return embeddedClientSecret(deps.createSession);
				},
			};
		}
		default:
			throw new Error("不認得的開通 session");
	}
}

/** 內嵌元件的 `fetchClientSecret`：每次都建立新的 session。 */
export async function embeddedClientSecret(
	createSession: () => Promise<OnboardingSession>,
): Promise<string> {
	const session = await createSession();
	if (session.kind !== "embedded") {
		throw new Error("開通 session 不是內嵌");
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
 * 進入認證頁與從金流商回來共用的一條路徑：先重讀（一次）再讀。重讀是背景動作：失敗（含 `resource_exhausted`、
 * `permission_denied`）不顯示、不丟出，也不擋讀取，快取不能留在舊的。
 */
export async function confirmPaymentReturn(deps: ConfirmDeps): Promise<void> {
	try {
		await deps.refreshAccount();
	} catch {
		// 重讀失敗不是賣家要處理的事：照常讀。
	}
	await deps.readAccount();
}
