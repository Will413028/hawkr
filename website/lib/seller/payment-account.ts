/**
 * 收款帳戶（`GetMyPaymentAccount`、`CreatePaymentOnboardingSession`）的資料層與呈現。
 *
 * 狀態、說明與動作的文字全部取自共用文案目錄（`seller.PaymentAccountStatus`），這裡只有版面：色調與「哪個狀態
 * 能開始開通」。沒有「值 → 文字」的表。沒有帳戶（平台審核還沒核准）時 `GetMyPaymentAccount` 回 not_found，
 * 那是 null，不是錯誤。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import {
	OnboardingSurface,
	type PaymentAccount,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerPaymentAccountPayoutsInactiveNote,
	sellerPaymentAccountStatus,
	sellerPaymentAccountStatusAction,
	sellerPaymentAccountStatusDescription,
} from "@hawkr/ui-text/keys";
import { sellerAppClient } from "../connect/seller";
import { optionalText, type UiText } from "../i18n/ui-text";
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

/** 開通 session：託管頁面是 `url`，內嵌元件是 `clientSecret`；缺對應欄位的回應是契約被違反。 */
export type OnboardingSession =
	| { surface: "hosted"; url: string }
	| { surface: "embedded"; clientSecret: string };

export async function createPaymentOnboardingSession(
	urls: { returnUrl: string; refreshUrl: string },
	client: () => SellerClient = sellerAppClient,
): Promise<OnboardingSession> {
	const res = await client().createPaymentOnboardingSession({
		returnUrl: urls.returnUrl,
		refreshUrl: urls.refreshUrl,
	});
	switch (res.surface) {
		case OnboardingSurface.HOSTED:
			if (!res.url) throw new Error("開通連結為空");
			return { surface: "hosted", url: res.url };
		case OnboardingSurface.EMBEDDED:
			if (!res.clientSecret) throw new Error("開通 client secret 為空");
			return { surface: "embedded", clientSecret: res.clientSecret };
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

/** 賣家此刻能開始（或繼續）開通：只有 ACTION_REQUIRED，而且伺服器告知了介面。 */
export function paymentAccountCanOnboard(account: PaymentAccount): boolean {
	return (
		account.status === PaymentAccountStatus.ACTION_REQUIRED &&
		(account.onboardingSurface === OnboardingSurface.HOSTED ||
			account.onboardingSurface === OnboardingSurface.EMBEDDED)
	);
}

export interface PaymentAccountCopy {
	badge: string | undefined;
	description: string | undefined;
	action: string | undefined;
	/** 收款已啟用但撥款尚未啟用時接在說明後的一行。 */
	payoutsNote: string | undefined;
}

/** 目錄的文字；沒有文字的值（UNSPECIFIED、不認得、目錄 omit）是 undefined，由呼叫端決定畫面。 */
export function paymentAccountCopy(
	ut: UiText,
	account: PaymentAccount,
): PaymentAccountCopy {
	return {
		badge: optionalText(ut, sellerPaymentAccountStatus(account.status)),
		description: optionalText(
			ut,
			sellerPaymentAccountStatusDescription(account.status),
		),
		action: optionalText(ut, sellerPaymentAccountStatusAction(account.status)),
		payoutsNote:
			account.status === PaymentAccountStatus.ENABLED && !account.payoutsActive
				? ut(sellerPaymentAccountPayoutsInactiveNote())
				: undefined,
	};
}
