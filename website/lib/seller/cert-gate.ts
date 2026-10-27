/**
 * Seller certification gate.
 *
 * 「已認證」是店家的公開曝光條件之一：`GetMyStore.capabilities` 中 `PUBLIC_LISTING` 缺的條件不含
 * `KYC_APPROVED`（含 PayUni 審核）。細分的狀態（審核中、補件、PayUni 審核…）來自
 * `GetMyKYC.application.flow.status`。這兩個來源只有這裡讀，頁面不自己拼。
 *
 * 認證不再是上架商品、建立優惠券與發票的前置條件（它們只看 `capabilities`）；它是公開曝光、收款的條件
 * （`approved`，「已認證」只有 `isStoreCertified` 一個來源）。物流設定的閘門另看 `SHIPPING_SETUP`
 * 能力（`shippingGate`），伺服器依能力的缺項告訴 client 是缺認證還是停權、等待刪除，client 不自己推前置條件。
 *
 * Caching note: `useSellerSelf` 與 `useMyKycFlow` 各是一條 identity-scoped server-state，
 * 所有消費者共用一次請求。`refresh()` 重新讀店家（例如補件之後）。
 */

import type {
	KYCFlow,
	KYCFlowStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useMyKycFlow, useSellerSelf } from "@/lib/hooks/useSellerSelf";
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
	/** 流程狀態（來自 GetMyKYC）；沒讀到時是 undefined。 */
	flowStatus: KYCFlowStatus | undefined;
	/** 流程（來自 GetMyKYC，含 `payuni` 與 `resubmitUnavailabilities`）；沒讀到時是 null。 */
	flow: KYCFlow | null;
	/** 賣家可以把申請改回草稿重新送出（`flow.can_resubmit`）。 */
	canResubmit: boolean;
	/** 店家（GetMyStore 的回應），沒讀到時是 null。 */
	seller: MyStore | null;
	error: string | null;
	refresh: () => void;
}

export function useCertGate(): CertGate {
	const { data: seller, loading, error, refresh } = useSellerSelf();
	const { flow, loading: flowLoading } = useMyKycFlow();
	const reload = () => {
		void refresh();
	};

	if (loading || flowLoading) {
		return {
			loading: true,
			approved: false,
			shippingGate: "unknown",
			flowStatus: undefined,
			flow: null,
			canResubmit: false,
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
			shippingGate: "unknown",
			flowStatus: undefined,
			flow: null,
			canResubmit: false,
			seller: null,
			error: error ? error.message : null,
			refresh: reload,
		};
	}

	const approved = isStoreCertified(seller);
	const flowStatus = flow?.status;
	return {
		loading: false,
		approved,
		shippingGate: shippingGateOf(seller),
		flowStatus,
		flow,
		canResubmit: flow?.canResubmit ?? false,
		seller,
		error: null,
		refresh: reload,
	};
}

export const CERT_PAGE_HREF = "/seller-center/me/certifications";
