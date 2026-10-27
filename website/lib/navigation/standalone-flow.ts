import { stripLocale } from "@/lib/intl/paths";

const STANDALONE_FLOW_ROOTS = [
	"/seller/onboarding",
	"/hawkr-agent-rfq-preview/external-vendor",
] as const;

/**
 * Focused account-transition flows suppress surrounding page chrome such as
 * the footer, role switcher, and mobile bottom nav. A surface may deliberately
 * retain a minimal top account escape (seller onboarding keeps the seller
 * header so a user can sign out) while still using this route/transition rule.
 */
export function isStandaloneFlowPath(
	rawPathname: string | null | undefined,
): boolean {
	if (!rawPathname) return false;
	const pathname = stripLocale(rawPathname);
	return STANDALONE_FLOW_ROOTS.some(
		(root) => pathname === root || pathname.startsWith(`${root}/`),
	);
}

/**
 * Standalone flows replace the complete page chrome. Cross-fading a normal
 * page snapshot with that different shell briefly exposes stale navbar,
 * footer, and loading-layer pixels, so either side of the navigation opts out
 * of native View Transitions entirely.
 */
export function isStandaloneFlowTransition(
	sourcePath: string | null | undefined,
	targetPath: string | null | undefined,
): boolean {
	return isStandaloneFlowPath(sourcePath) || isStandaloneFlowPath(targetPath);
}
