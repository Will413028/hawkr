/**
 * 單元測試用的 `GetMyStoreResponse.capabilities` 組裝：照 proto 的形狀（`CapabilityStatus`）建出
 * 啟用的能力與店家的公開曝光狀態，測試不必各自拼 wire 形狀。
 */

import { create } from "@bufbuild/protobuf";
import {
	type CapabilityStatus,
	CapabilityStatusSchema,
	SellerCapability,
	type SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";

/** 這些能力都啟用（沒有缺項）。 */
export function activeStatuses(
	capabilities: readonly SellerCapability[],
): CapabilityStatus[] {
	return capabilities.map((capability) =>
		create(CapabilityStatusSchema, { capability, active: true }),
	);
}

/** 店家狀態 `PUBLIC_LISTING`：缺的條件為空才啟用。 */
export function publicListingStatus(
	missing: readonly SellerRequirement[],
): CapabilityStatus {
	return create(CapabilityStatusSchema, {
		capability: SellerCapability.PUBLIC_LISTING,
		active: missing.length === 0,
		missing: [...missing],
	});
}

/**
 * 一整組能力狀態：`granted` 啟用，其餘的能力都沒啟用並缺 `missing`（伺服器的契約：沒啟用的能力至少缺一項）。
 * `extraMissing` 可以替個別能力指定不同的缺項（例如物流建店缺 KYC_APPROVED）。
 */
export function capabilityStatuses(
	granted: readonly SellerCapability[],
	missing: readonly SellerRequirement[],
	extraMissing: Partial<
		Record<SellerCapability, readonly SellerRequirement[]>
	> = {},
): CapabilityStatus[] {
	const all = Object.values(SellerCapability).filter(
		(v): v is SellerCapability =>
			typeof v === "number" &&
			v !== SellerCapability.UNSPECIFIED &&
			v !== SellerCapability.PUBLIC_LISTING,
	);
	return all.map((capability) => {
		const active = granted.includes(capability);
		return create(CapabilityStatusSchema, {
			capability,
			active,
			missing: active ? [] : [...(extraMissing[capability] ?? missing)],
		});
	});
}
