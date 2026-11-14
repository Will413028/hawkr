/**
 * Seller certification gate.
 *
 * 「已認證」是店家的公開曝光條件之一：`GetMyStore.capabilities` 中 `PUBLIC_LISTING` 缺的條件不含
 * `ONBOARDING_COMPLETE`（平台審核核准且收款帳戶啟用）。細分的狀態來自三條：申請 `GetMyKYC.application`（只作申請本身的資料）、
 * 開通摘要 `GetMyOnboarding`（平台審核是否完成由它決定，收款帳戶在那之後才讀），與收款帳戶 `GetMyPaymentAccount.account.status`。這些來源只有這裡讀，頁面不自己拼。
 *
 * 認證不再是上架商品、建立優惠券與發票的前置條件（它們只看 `capabilities`）；它是公開曝光、收款的條件
 * （`approved`，「已認證」只有 `isStoreCertified` 一個來源）。物流設定的閘門另看 `SHIPPING_SETUP`
 * 能力（`shippingGate`），伺服器依能力的缺項告訴 client 是缺認證還是停權、等待刪除，client 不自己推前置條件。
 *
 * Caching note: `useSellerSelf`、`useMyKyc`、`useMyOnboarding` 與 `useMyPaymentAccount` 各是一條 identity-scoped server-state，
 * 所有消費者共用一次請求。`refresh()` 重新讀店家（例如補件之後）。
 */

import type {
	GetMyOnboardingResponse,
	KYCApplication,
	PaymentAccount,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	useMyKyc,
	useMyOnboarding,
	useMyPaymentAccount,
	useSellerSelf,
} from "@/lib/hooks/useSellerSelf";
import {
	paymentAccountCompleted,
	platformReviewCompleted,
} from "./onboarding-summary";
import {
	isStoreCertified,
	type MyStore,
	type ShippingGate,
	shippingGateOf,
} from "./store";

export interface CertGate {
	loading: boolean;
	/** 商家認證已核准：公開曝光的 KYC 條件已滿足。 */
	approved: boolean;
	/** 物流設定的閘門（`SHIPPING_SETUP` 的啟用與缺項）。 */
	shippingGate: ShippingGate;
	/** 平台審核已完成（開通摘要 PLATFORM_REVIEW 是 COMPLETED）；沒讀到摘要時是 false。 */
	platformReviewCompleted: boolean;
	/** 收款帳戶任務已完成（開通摘要 PAYMENT_ACCOUNT 是 COMPLETED）；沒讀到摘要時是 false。 */
	paymentAccountCompleted: boolean;
	/** 開通摘要（GetMyOnboarding）；沒讀到時是 null。 */
	onboarding: GetMyOnboardingResponse | null;
	/** 申請（GetMyKYC）；沒有申請或沒讀到時是 null。 */
	application: KYCApplication | null;
	/** 收款帳戶（GetMyPaymentAccount，平台審核核准後才讀）；還沒有或沒讀到時是 null。 */
	paymentAccount: PaymentAccount | null;
	/** 店家（GetMyStore 的回應），沒讀到時是 null。 */
	seller: MyStore | null;
	error: string | null;
	refresh: () => void;
}

export function useCertGate(): CertGate {
	const { data: seller, loading, error, refresh } = useSellerSelf();
	const { application, loading: kycLoading } = useMyKyc();
	const { onboarding, loading: onboardingLoading } = useMyOnboarding();
	// 收款帳戶在平台審核完成之後才讀；「完成」是開通摘要（伺服器）的答案，不從申請狀態推導。
	const { account: paymentAccount, loading: paymentLoading } =
		useMyPaymentAccount({ enabled: platformReviewCompleted(onboarding) });
	const reload = () => {
		void refresh();
	};

	if (loading || kycLoading || onboardingLoading || paymentLoading) {
		return {
			loading: true,
			approved: false,
			platformReviewCompleted: false,
			paymentAccountCompleted: false,
			onboarding: null,
			shippingGate: "unknown",
			application: null,
			paymentAccount: null,
			seller: null,
			error: null,
			refresh: reload,
		};
	}

	// No usable data — first-load failure / not found / no store — reads as
	// "not started". NOTE: we branch on `!seller`, not `error`, on purpose: the
	// query is SHARED across the navbar / dashboard / gates, and TanStack keeps the
	// last-good `data` while setting `error` on a *background* refetch failure.
	// Preferring last-good data means a transient blip can't re-lock an already
	// certified seller everywhere at once.
	if (!seller) {
		return {
			loading: false,
			approved: false,
			platformReviewCompleted: false,
			paymentAccountCompleted: false,
			onboarding: null,
			shippingGate: "unknown",
			application: null,
			paymentAccount: null,
			seller: null,
			error: error ? error.message : null,
			refresh: reload,
		};
	}

	const approved = isStoreCertified(seller);
	return {
		loading: false,
		approved,
		platformReviewCompleted: platformReviewCompleted(onboarding),
		paymentAccountCompleted: paymentAccountCompleted(onboarding),
		onboarding,
		shippingGate: shippingGateOf(seller),
		application,
		paymentAccount,
		seller,
		error: null,
		refresh: reload,
	};
}

export const CERT_PAGE_HREF = "/seller-center/me/certifications";
