import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
	isAssistantAnalyticsRoute,
	privacySafeRoutePathname,
} from "@/lib/analytics/page-view-privacy";
import { requiredAccessForPath, workspaceOfPath } from "@/lib/auth/redirect";
import { __setFlavorForTest } from "@/lib/config/flavor";
import {
	productIdFromPathname,
	routeContentReadySelectors,
} from "@/lib/loading/initial-route-ready";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";
import { isNavPathActive } from "./active-link";
import { isDelegatedWorkspacePath } from "./delegated-workspace";
import { footerPerspectiveForPathname } from "./footer-perspective";
import { isAccountMessageThreadPath } from "./message-thread";
import { navigationRecoveryTimeout } from "./recovery-state";
import {
	getPersistentShell,
	hasSamePersistentShell,
	isProductDetailPath,
	isSameTabHop,
	isTabToTabNav,
	shouldAnimateProgrammaticRoute,
	tabKeyFor,
} from "./route-transition-policy";
import {
	isStandaloneFlowPath,
	isStandaloneFlowTransition,
} from "./standalone-flow";

/**
 * Every classifier that reads a path the browser shows must judge `/en/feed` exactly as it judges
 * `/feed`: the rules are written once, on the locale-free route (lib/intl/paths.ts `stripLocale`).
 * A classifier that forgets to strip works in zh-TW and silently changes behavior in English.
 */
const ROUTES = [
	"/",
	"/feed",
	"/market",
	"/market/products",
	"/search",
	"/products/12",
	"/products",
	"/sellers/9",
	"/sellers/9/posts",
	"/sellers/9/reviews",
	"/sellers/9/products/5",
	"/account",
	"/account/orders/HK-1",
	"/account/messages",
	"/account/messages/77",
	"/account/messages/assistant/procurement",
	"/seller-center",
	"/seller-center/orders",
	"/seller-center/preview/3",
	"/seller-center/messages/4",
	"/seller/onboarding",
	"/seller/measures/1",
	"/hawkr-agent-rfq-preview/external-vendor/x",
	"/adm/1/products",
	"/checkout",
	"/login",
	"/privacy",
] as const;

const en = (route: string) => (route === "/" ? "/en" : `/en${route}`);

beforeEach(() => __setFlavorForTest("dev"));
afterEach(() => __setFlavorForTest(null));

describe("one-path classifiers treat /en/<route> like /<route>", () => {
	const unary: Array<[string, (path: string) => unknown]> = [
		["tabKeyFor", tabKeyFor],
		["isProductDetailPath", isProductDetailPath],
		["getPersistentShell", getPersistentShell],
		["isStandaloneFlowPath", isStandaloneFlowPath],
		["isAccountMessageThreadPath", isAccountMessageThreadPath],
		["isDelegatedWorkspacePath", isDelegatedWorkspacePath],
		["requiredAccessForPath", requiredAccessForPath],
		["workspaceOfPath", workspaceOfPath],
		["routeContentReadySelectors", routeContentReadySelectors],
		["productIdFromPathname", productIdFromPathname],
		["notificationWorkspaceRole", notificationWorkspaceRole],
		["navigationRecoveryTimeout", navigationRecoveryTimeout],
		["isAssistantAnalyticsRoute", isAssistantAnalyticsRoute],
		["footerPerspective (buyer)", (p) => footerPerspectiveForPathname(p)],
		[
			"footerPerspective (seller)",
			(p) => footerPerspectiveForPathname(p, "seller"),
		],
		["isNavPathActive (/feed)", (p) => isNavPathActive(p, "/feed")],
		["isNavPathActive (/)", (p) => isNavPathActive(p, "/")],
		[
			"isNavPathActive (/account)",
			(p) => isNavPathActive(p, "/account?tab=orders"),
		],
	];

	for (const [name, classify] of unary) {
		it(name, () => {
			for (const route of ROUTES) {
				expect(classify(en(route)), `${name}(${en(route)})`).toEqual(
					classify(route),
				);
			}
		});
	}

	it("privacySafeRoutePathname keeps the language prefix and classifies the route under it", () => {
		for (const route of ROUTES) {
			const safe = privacySafeRoutePathname(route);
			expect(privacySafeRoutePathname(en(route)), route).toBe(
				safe === "/" ? "/en" : `/en${safe}`,
			);
		}
	});
});

describe("two-path classifiers treat /en/<a> -> /en/<b> like /<a> -> /<b>", () => {
	const binary: Array<[string, (a: string, b: string) => unknown]> = [
		["isTabToTabNav", isTabToTabNav],
		["isSameTabHop", isSameTabHop],
		["hasSamePersistentShell", hasSamePersistentShell],
		["isStandaloneFlowTransition", isStandaloneFlowTransition],
		["shouldAnimateProgrammaticRoute", shouldAnimateProgrammaticRoute],
	];

	for (const [name, classify] of binary) {
		it(name, () => {
			for (const from of ROUTES) {
				for (const to of ROUTES) {
					expect(
						classify(en(from), en(to)),
						`${name}(${en(from)}, ${en(to)})`,
					).toEqual(classify(from, to));
				}
			}
		});
	}
});
