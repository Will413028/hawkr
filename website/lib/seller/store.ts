/**
 * 賣家中心讀到的店家（`SellerAppService.GetMyStore`）。賣家路徑直接使用生成的 `GetMyStoreResponse`，
 * 這裡只放對它的唯讀判斷：是否已認證、停權、等待刪除、能力與圖片網址。proto 沒設的欄位不是 0 或空字串，
 * 判斷一律從「有沒有設」開始。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import {
	type CapabilityStatus,
	type GetMyStoreResponse,
	SellerCapability,
	SellerRequirement,
	type Store,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { imageUri } from "../media/image";

export type MyStore = GetMyStoreResponse;

/** GetMyStore 回 not_found：帳號還沒有店家，呼叫端該導去開店流程，不是顯示錯誤。 */
export function isStoreMissing(error: unknown): boolean {
	return error != null && ConnectError.from(error).code === Code.NotFound;
}

/** 店家 id；沒有店家（或沒有 id）時是 undefined，不是空字串。 */
export function storeIdOf(my: MyStore | null | undefined): string | undefined {
	return my?.store?.id || undefined;
}

/** 一個能力的狀態；伺服器沒有回報這個能力（代行的店面、尚未載入）時是 undefined，不是「沒有啟用」。 */
export function capabilityStatus(
	my: MyStore | null | undefined,
	capability: SellerCapability,
): CapabilityStatus | undefined {
	return my?.capabilities.find((status) => status.capability === capability);
}

/** 目前啟用的能力（`GetMyStore.capabilities` 中 `active` 的那些，含店家狀態 PUBLIC_LISTING）。 */
export function activeCapabilities(
	my: MyStore | null | undefined,
): readonly SellerCapability[] {
	return (my?.capabilities ?? [])
		.filter(
			(status) =>
				status.active && status.capability !== SellerCapability.UNSPECIFIED,
		)
		.map((status) => status.capability);
}

/** 一個能力缺的條件；伺服器沒有回報這個能力時回 undefined（不是「什麼都不缺」）。 */
export function missingOf(
	my: MyStore | null | undefined,
	capability: SellerCapability,
): readonly SellerRequirement[] | undefined {
	return capabilityStatus(my, capability)?.missing;
}

/**
 * 公開曝光（`PUBLIC_LISTING`）還缺的條件；伺服器沒有回報 `PUBLIC_LISTING` 時回 undefined（不是「什麼都不缺」）。
 */
export function missingRequirements(
	my: MyStore | null | undefined,
): readonly SellerRequirement[] | undefined {
	return missingOf(my, SellerCapability.PUBLIC_LISTING);
}

/** 無法確認商店狀態時的提示（開店流程與物流頁共用）。 */
export const STORE_STATE_UNKNOWN_MESSAGE =
	"目前無法確認商店狀態，請重新整理後再試一次";

/**
 * 物流設定（`SHIPPING_SETUP`）的閘門，只由伺服器回報的啟用與缺的條件決定，優先序固定：
 * 等待刪除 > 停權 > 認證。啟用就開放；缺 `KYC_APPROVED` 是認證閘門；店家還沒讀到、伺服器沒回報
 * 這個能力、缺 `STORE_ACTIVE` 或缺項不認得，都是 `unknown`（不退回認證閘門）。
 */
export type ShippingGate =
	| "open"
	| "suspended"
	| "pending_deletion"
	| "certification"
	| "unknown";

export function shippingGateOf(my: MyStore | null | undefined): ShippingGate {
	const status = capabilityStatus(my, SellerCapability.SHIPPING_SETUP);
	if (!status) return "unknown";
	if (status.active) return "open";
	const missing = status.missing;
	if (missing.includes(SellerRequirement.NOT_PENDING_DELETION)) {
		return "pending_deletion";
	}
	if (missing.includes(SellerRequirement.NOT_SUSPENDED)) return "suspended";
	if (missing.includes(SellerRequirement.STORE_ACTIVE)) return "unknown";
	if (missing.includes(SellerRequirement.KYC_APPROVED)) return "certification";
	return "unknown";
}

/**
 * 商家認證（含 PayUni 審核）已核准：`PUBLIC_LISTING` 缺的條件不含 `KYC_APPROVED`。
 * 沒有回報 `PUBLIC_LISTING`（代行的店面、尚未載入）不算已認證。
 */
export function isStoreCertified(my: MyStore | null | undefined): boolean {
	const missing = missingRequirements(my);
	return (
		missing !== undefined && !missing.includes(SellerRequirement.KYC_APPROVED)
	);
}

/** 生效中的停權（任何一筆生效中的停權措施）；與能力的 `NOT_SUSPENDED` 同一個來源。原因看 `ListMyMeasures`。 */
export function isStoreSuspended(my: MyStore | null | undefined): boolean {
	return my?.suspended === true;
}

export function isStorePendingDeletion(
	my: MyStore | null | undefined,
): boolean {
	return my?.store?.status === StoreStatus.PENDING_DELETION;
}

/** 開店流程還沒完成：沒有店家，或店家沒有標記完成新手引導。 */
export function needsOnboarding(my: MyStore | null | undefined): boolean {
	return !my?.store?.onboardingCompleted;
}

export function hasCapability(
	my: MyStore | null | undefined,
	capability: SellerCapability,
): boolean {
	return (
		capability !== SellerCapability.UNSPECIFIED &&
		(capabilityStatus(my, capability)?.active ?? false)
	);
}

/** 平台核准的食品業者登錄字號（沒有核准的登錄或空白時回空字串）；待審與被退回的請求不算。 */
export function foodRegistrationNumber(store: Store | undefined): string {
	return (store?.approvedFoodRegistration?.registrationNumber ?? "").trim();
}

/** 賣家中心顯示用的 logo 與封面網址；沒設時是 undefined。 */
export function storeImageUrls(store: Store | undefined): {
	logoUrl: string | undefined;
	coverUrl: string | undefined;
} {
	return {
		logoUrl: imageUri(store?.logo, "avatar"),
		coverUrl: imageUri(store?.coverImage, "banner"),
	};
}
