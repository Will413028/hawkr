/**
 * 收款帳戶開通的流程（純邏輯，元件只負責接線）。
 *
 * - HOSTED：呼叫 `CreatePaymentOnboardingSession`，整頁導到金流商的連結；回到 `?payment=return` 時先呼叫
 *   `RefreshMyPaymentAccount`（伺服器同步向金流商重讀，D33）再讀帳戶與開通摘要（`confirmPaymentReturn`），不輪詢；重讀失敗不顯示；回到 `?payment=refresh`（連結過期）時重新取得連結再導。
 * - EMBEDDED：先建立 session 取得金流商代碼與公開金鑰（元件要它們才能初始化），依代碼選元件；第一次 `fetchClientSecret`
 *   用這個 session 的 client secret，之後每次都是新的 session。
 */

import { OnboardingSurface } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import type { OnboardingSession } from "./payment-account";

/** 開通完成與離開都回到認證頁，用 query 區分。 */
export const PAYMENT_PARAM = "payment";
export type PaymentParam = "return" | "refresh";

export function paymentParamOf(value: string | null): PaymentParam | null {
	return value === "return" || value === "refresh" ? value : null;
}

/** 這個網站有內嵌元件的金流商；伺服器回其他代碼是不能開通。 */
export type EmbeddedProvider = "stripe";

export function embeddedProviderOf(provider: string): EmbeddedProvider | null {
	return provider === "stripe" ? "stripe" : null;
}

export type OnboardingStart =
	| { kind: "redirect"; url: string }
	| {
			kind: "embedded";
			provider: EmbeddedProvider;
			publishableKey: string;
			fetchClientSecret: () => Promise<string>;
	  };

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
		case OnboardingSurface.EMBEDDED: {
			const session = await deps.createSession();
			if (session.surface !== "embedded") {
				throw new Error("開通介面與伺服器告知的不一致");
			}
			const provider = embeddedProviderOf(session.provider);
			if (!provider) throw new Error("不認得的金流商");
			let first: string | null = session.clientSecret;
			return {
				kind: "embedded",
				provider,
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
