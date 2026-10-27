import { stripLocale } from "@/lib/intl/paths";
import { isStandaloneFlowTransition } from "@/lib/navigation/standalone-flow";

// Every classifier below takes the path the browser shows (`/en/feed` as well as `/feed`) and judges it
// on the locale-free route, so the rules are written once for every language.
export function tabKeyFor(rawPath: string | null | undefined): string | null {
	if (!rawPath) return null;
	const path = stripLocale(rawPath);
	if (path === "/" || path === "/feed" || path === "/search") return path;
	if (path === "/market" || path === "/market/products") return "/market";
	return null;
}

export function isTabToTabNav(
	sourcePath: string | null | undefined,
	targetPath: string | null | undefined,
) {
	const sourceTab = tabKeyFor(sourcePath);
	const targetTab = tabKeyFor(targetPath);
	return Boolean(sourceTab && targetTab && sourceTab !== targetTab);
}

// Same-tab legacy hops such as /market -> /market/products must remain an
// instant continuation of the tab switch that started them.
export function isSameTabHop(
	sourcePath: string | null | undefined,
	targetPath: string | null | undefined,
) {
	const sourceTab = tabKeyFor(sourcePath);
	return Boolean(
		sourceTab &&
			sourcePath !== targetPath &&
			sourceTab === tabKeyFor(targetPath),
	);
}

export function isProductDetailPath(rawPathname: string) {
	const pathname = stripLocale(rawPathname);
	return (
		/^\/products\/[^/]+/.test(pathname) ||
		/^\/sellers\/[^/]+\/products\/[^/]+/.test(pathname)
	);
}

export function getPersistentShell(
	rawPathname: string | null | undefined,
): string | null {
	const pathname = rawPathname ? stripLocale(rawPathname) : rawPathname;
	if (pathname?.startsWith("/seller-center")) return "seller-center";
	if (pathname?.startsWith("/account")) return "account";
	// Storefront tabs share the seller profile chrome. Key by seller id so a
	// cross-seller navigation is still treated as a new page.
	const storefrontTab = pathname?.match(
		/^\/sellers\/([^/]+)(?:\/(?:posts|reviews|messages))?\/?$/,
	);
	if (storefrontTab) return `seller-storefront:${storefrontTab[1]}`;
	return null;
}

export function hasSamePersistentShell(
	sourcePath: string | null | undefined,
	targetPath: string | null | undefined,
) {
	const sourceShell = getPersistentShell(sourcePath);
	const targetShell = getPersistentShell(targetPath);
	return Boolean(sourceShell && targetShell && sourceShell === targetShell);
}

/**
 * Programmatic navigation keeps the current page live while RSC data loads,
 * then gives an ordinary destination a short commit-arrival fade. Surfaces
 * with their own spatial or shell continuity deliberately stay instant.
 */
export function shouldAnimateProgrammaticRoute(
	sourcePath: string | null | undefined,
	targetPath: string | null | undefined,
): boolean {
	if (!sourcePath || !targetPath || sourcePath === targetPath) return false;
	if (isStandaloneFlowTransition(sourcePath, targetPath)) return false;
	if (isTabToTabNav(sourcePath, targetPath)) return false;
	if (isSameTabHop(sourcePath, targetPath)) return false;
	if (hasSamePersistentShell(sourcePath, targetPath)) return false;
	if (isProductDetailPath(targetPath)) return false;
	// Storefront data can be cold. Keep the source interactive until commit and
	// land immediately; its progress rail already owns the wait feedback.
	if (/^\/sellers\/[^/]+/.test(stripLocale(targetPath))) return false;
	return true;
}
