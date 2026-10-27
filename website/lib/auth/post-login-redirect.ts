import {
	canUsePerspective,
	effectiveActiveRole,
	getStoredActiveRole,
	type RoleSlot,
} from "./active-role";
import {
	ACCESS_CLASSES,
	defaultPathForRole,
	requiredAccessForPath,
	safeRedirectPath,
	workspaceOfPath,
} from "./redirect";
import type { Identity } from "./storage";

const SELLER_ONBOARDING_PATH = "/seller/onboarding";

/**
 * A seller login/onboarding flow may only finish inside the seller workspace.
 * Public paths (most notably `/`, carried by the global login link) are not a
 * useful seller return target: loading them first exposes the buyer shell until
 * SellerWorkspaceLock hydrates and performs a second redirect.
 */
export function sellerWorkspaceRedirect(redirectAfter: string): string {
	return (
		requestedSellerWorkspaceRedirect(redirectAfter) ??
		defaultPathForRole("seller")
	);
}

function requestedSellerWorkspaceRedirect(
	redirectAfter: string,
): string | null {
	const safeRedirect = safeRedirectPath(redirectAfter);
	if (!safeRedirect) return null;

	const pathname = new URL(safeRedirect, "https://hawkr.local").pathname;
	if (
		workspaceOfPath(pathname) === "seller" ||
		pathname === SELLER_ONBOARDING_PATH
	) {
		return safeRedirect;
	}

	return null;
}

/**
 * Resolve the destination after a seller OAuth exchange.
 *
 * A missing needsOnboarding value keeps the legacy admission-gate behaviour so
 * a staged frontend deploy cannot accidentally admit an incomplete seller
 * while an older gateway is still serving traffic.
 */
export function sellerPostLoginRedirect(
	redirectAfter: string,
	needsOnboarding: boolean | undefined,
	hasSellerRole = true,
): string {
	const requestedWorkspaceRedirect =
		requestedSellerWorkspaceRedirect(redirectAfter);
	const workspaceRedirect =
		requestedWorkspaceRedirect ?? defaultPathForRole("seller");
	if (needsOnboarding === false && hasSellerRole) return workspaceRedirect;

	const alreadyTargetsOnboarding =
		workspaceRedirect === SELLER_ONBOARDING_PATH ||
		workspaceRedirect.startsWith(`${SELLER_ONBOARDING_PATH}?`);
	if (alreadyTargetsOnboarding) return workspaceRedirect;

	// Only preserve an explicitly requested seller-workspace deep link. Empty,
	// public, buyer, and unsafe targets all converge on the onboarding page; its
	// own default completion target is the seller dashboard.
	return requestedWorkspaceRedirect
		? `${SELLER_ONBOARDING_PATH}?redirect=${encodeURIComponent(requestedWorkspaceRedirect)}`
		: SELLER_ONBOARDING_PATH;
}

function isAccountPath(redirect: string): boolean {
	const safeRedirect = safeRedirectPath(redirect);
	if (!safeRedirect) return false;
	const pathname = new URL(safeRedirect, "https://hawkr.local").pathname;
	const access = requiredAccessForPath(pathname);
	return access !== null && ACCESS_CLASSES[access].workspace === null;
}

interface ResolvePostLoginTargetInput {
	requestedRole: RoleSlot | null;
	redirectAfter: string;
	/** The signed-in account: its role set. */
	account: Pick<Identity, "isSeller">;
	needsOnboarding: boolean | undefined;
	/** The stored workspace (client UI state); defaults to localStorage. */
	storedRole?: RoleSlot | null;
}

export interface PostLoginTarget {
	/** Role the returned account actually owns and should persist as active. */
	activeRole: RoleSlot | null;
	/** Safe destination for that role and the seller-onboarding contract. */
	destination: string;
}

/**
 * Resolve every successful login completion from the backend role set.
 *
 * `requestedRole` is only the tab / provisioning intent. Every account is a
 * buyer, so the only requested role an account may not own is seller; when it
 * does not own it, navigation follows the client default (the stored workspace
 * when the account can use it, else buyer) instead of loading the wrong
 * workspace first.
 *
 * Seller intent is the one exception: a buyer-only account selecting seller
 * is entering the buyer -> seller onboarding flow, so keep the onboarding
 * destination while retaining buyer as the active role until binding succeeds.
 */
export function resolvePostLoginTarget({
	requestedRole,
	redirectAfter,
	account,
	needsOnboarding,
	storedRole = getStoredActiveRole(),
}: ResolvePostLoginTargetInput): PostLoginTarget {
	const requestedRoleIsOwned =
		requestedRole !== null && canUsePerspective(account, requestedRole);
	const activeRole = requestedRoleIsOwned
		? requestedRole
		: effectiveActiveRole(account, storedRole);
	const hasSellerRole = account.isSeller;

	// A signed-in-only page needs no workspace, shop or onboarding: the target
	// is the page itself whatever the seller state.
	if (isAccountPath(redirectAfter)) {
		return { activeRole, destination: safeRedirectPath(redirectAfter) };
	}

	if (requestedRole === "seller" || activeRole === "seller") {
		return {
			activeRole,
			destination: sellerPostLoginRedirect(
				redirectAfter,
				needsOnboarding,
				hasSellerRole,
			),
		};
	}

	return { activeRole, destination: redirectAfter };
}
