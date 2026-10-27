import type { RoleSlot } from "@/lib/auth/active-role";
import type { MyStore } from "@/lib/seller/store";

export type SellerChatDecision =
	| "open"
	| "switch_role"
	| "own_store"
	| "checking_identity"
	| "identity_unavailable";

interface SellerChatDecisionInput {
	activeRole: RoleSlot | null;
	hasSellerRole: boolean;
	sellerProfileMissing: boolean;
	sellerSelf: MyStore | null;
	sellerSelfLoading: boolean;
	targetSellerId: string;
}

function normalizedSellerId(value: unknown): string {
	return String(value ?? "").trim();
}

/**
 * Chat history, presence and websocket routing are keyed by the internal
 * storeId. A public storefront route may use storeCode, but that display-safe
 * route id must not leak into the chat room identity.
 */
export function resolveSellerChatTargetId(seller: {
	storeId?: string | null;
}): string | null {
	return normalizedSellerId(seller.storeId) || null;
}

/**
 * Public seller routes may carry either the canonical storeCode or a legacy
 * storeId. `GetMyStore` is the authority for both, so compare both exact
 * identifiers instead of deriving ownership from the active UI role.
 */
export function isOwnSellerTarget(
	targetSellerId: string,
	sellerSelf: MyStore | null,
): boolean {
	const target = normalizedSellerId(targetSellerId);
	if (!target || !sellerSelf) return false;

	return [sellerSelf.store?.id, sellerSelf.store?.storeCode]
		.map(normalizedSellerId)
		.some((selfId) => selfId !== "" && selfId === target);
}

/**
 * Backend keeps the final self-message invariant, but the buyer UI should stop
 * the impossible action before opening a conversation. A dual-role account
 * must wait for its authoritative seller profile; unknown identity fails
 * closed rather than guessing that the viewed store belongs to somebody else.
 */
export function decideSellerChatAction({
	activeRole,
	hasSellerRole,
	sellerProfileMissing,
	sellerSelf,
	sellerSelfLoading,
	targetSellerId,
}: SellerChatDecisionInput): SellerChatDecision {
	if (activeRole !== "buyer") return "switch_role";
	if (!hasSellerRole) return "open";

	if (sellerSelf) {
		return isOwnSellerTarget(targetSellerId, sellerSelf) ? "own_store" : "open";
	}

	// A seller-role account with no store profile cannot own the target store.
	if (sellerProfileMissing) return "open";
	if (sellerSelfLoading) return "checking_identity";
	return "identity_unavailable";
}
