/**
 * 收款帳戶（`GetMyPaymentAccount`、`CreatePaymentOnboardingSession`）的資料層與呈現。
 *
 * 狀態徽章取自共用文案目錄（`payment.PaymentAccountStatus`）；下一步（按鈕）與說明句只讀開通摘要的收款帳戶任務
 * （`GetMyOnboarding`），不由帳戶狀態推導。這裡只有版面：色調。沒有「值 → 文字」的表。沒有帳戶（平台審核還沒核准）時 `GetMyPaymentAccount` 回 not_found，
 * 那是 null，不是錯誤。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import {
	OnboardingSurface,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	type GetMyOnboardingResponse,
	OnboardingReturnDestination,
	type PaymentAccount,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	paymentPaymentAccountStatus,
	sellerPaymentAccountPayoutsInactiveNote,
} from "@hawkr/ui-text/keys";
import { sellerAppClient } from "../connect/seller";
import { optionalText, type UiText } from "../i18n/ui-text";
import type { AppLocale } from "../intl/locales";
import {
	paymentAccountCompleted,
	paymentAccountTask,
} from "./onboarding-summary";
import type { StatusBannerTone } from "./status-tone";

type SellerClient = ReturnType<typeof sellerAppClient>;

/** 目前的收款帳戶；還沒有（平台審核還沒核准）是 null。 */
export async function fetchMyPaymentAccount(
	client: () => SellerClient = sellerAppClient,
): Promise<PaymentAccount | null> {
	try {
		const res = await client().getMyPaymentAccount({});
		return res.account ?? null;
	} catch (e) {
		if (ConnectError.from(e).code === Code.NotFound) return null;
		throw e;
	}
}

/**
 * 開通 session：託管頁面是 `url`，內嵌元件是 `clientSecret` 加金流商代碼 `provider` 與初始化元件用的公開金鑰
 * `publishableKey`；缺對應欄位的回應是契約被違反。
 */
export type OnboardingSession =
	| { surface: "hosted"; url: string }
	| {
			surface: "embedded";
			clientSecret: string;
			provider: string;
			publishableKey: string;
	  };

/** 回跳網址由伺服器依目的地與語系組；網站頁面的回跳目的地是認證頁。 */
export async function createPaymentOnboardingSession(
	locale: AppLocale,
	client: () => SellerClient = sellerAppClient,
): Promise<OnboardingSession> {
	const res = await client().createPaymentOnboardingSession({
		destination: OnboardingReturnDestination.WEBSITE_CERTIFICATION,
		locale,
	});
	switch (res.surface) {
		case OnboardingSurface.HOSTED:
			if (!res.url) throw new Error("開通連結為空");
			return { surface: "hosted", url: res.url };
		case OnboardingSurface.EMBEDDED:
			if (!res.clientSecret) throw new Error("開通 client secret 為空");
			if (!res.provider) throw new Error("開通的金流商為空");
			if (!res.publishableKey) throw new Error("開通的公開金鑰為空");
			return {
				surface: "embedded",
				clientSecret: res.clientSecret,
				provider: res.provider,
				publishableKey: res.publishableKey,
			};
		default:
			throw new Error("不認得的開通介面");
	}
}

/** 狀態卡的色調；沒有色調（UNSPECIFIED、不認得的值）回 undefined。 */
export function paymentAccountTone(
	status: PaymentAccountStatus,
): StatusBannerTone | undefined {
	switch (status) {
		case PaymentAccountStatus.NOT_CREATED:
		case PaymentAccountStatus.UNDER_REVIEW:
			return "info";
		case PaymentAccountStatus.ACTION_REQUIRED:
			return "warning";
		case PaymentAccountStatus.ENABLED:
			return "success";
		case PaymentAccountStatus.DISABLED:
			return "danger";
		default:
			return undefined;
	}
}

export interface PaymentAccountCopy {
	badge: string | undefined;
	/** 收款已啟用但撥款尚未啟用時接在說明後的一行。 */
	payoutsNote: string | undefined;
}

/** 狀態徽章與撥款提示的文字；沒有文字的值（UNSPECIFIED、不認得、目錄 omit）是 undefined，由呼叫端決定畫面。 */
export function paymentAccountCopy(
	ut: UiText,
	account: PaymentAccount,
): PaymentAccountCopy {
	return {
		badge: optionalText(ut, paymentPaymentAccountStatus(account.status)),
		payoutsNote:
			account.status === PaymentAccountStatus.ENABLED && !account.payoutsActive
				? ut(sellerPaymentAccountPayoutsInactiveNote())
				: undefined,
	};
}

/** 儀表板的收款帳戶提示：摘要說收款帳戶任務還沒完成才顯示（不讀 `PaymentAccount.status` 判斷完成）；徽章讀帳戶狀態，說明句讀任務；沒讀到摘要或沒有帳戶不顯示。 */
export function paymentBannerCopy(
	ut: UiText,
	account: PaymentAccount | null | undefined,
	onboarding: GetMyOnboardingResponse | null | undefined,
): (PaymentAccountCopy & { description: string }) | null {
	if (!account || !onboarding || paymentAccountCompleted(onboarding)) {
		return null;
	}
	return {
		...paymentAccountCopy(ut, account),
		description: paymentAccountTask(onboarding)?.status?.message ?? "",
	};
}

/** 客服頁。 */
export const CONTACT_HREF = "/contact";
