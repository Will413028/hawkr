import { stripLocale } from "@/lib/intl/paths";

const ACCOUNT_MESSAGES_ROOT = "/account/messages";

/**
 * Buyer message threads own the full mobile viewport below the top navbar.
 * The list route deliberately keeps the global mobile bottom navigation.
 */
export function isAccountMessageThreadPath(
	rawPathname: string | null | undefined,
): boolean {
	const pathname = rawPathname ? stripLocale(rawPathname) : rawPathname;
	if (!pathname?.startsWith(`${ACCOUNT_MESSAGES_ROOT}/`)) return false;
	return pathname.slice(ACCOUNT_MESSAGES_ROOT.length + 1).split("/")[0] !== "";
}
