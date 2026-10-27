import { expect, type Page } from "@playwright/test";

export const RESIDUAL_LOADING_SELECTOR = [
	".initial-loader",
	".route-loading-skeleton",
	".market-grid-skeleton",
	".auth-brand-loading",
	".thread-page--loading",
	'[aria-busy="true"]',
	'[role="status"][aria-label*="載入"]',
	'[aria-label="首頁內容載入中"]',
	'[aria-label="商品載入中"]',
	'[data-pending="true"]',
].join(",");

export interface VisibleLoadingSurface {
	tag: string;
	className: string;
	label: string;
	role: string;
}

export async function visibleLoadingSurfaces(
	page: Page,
): Promise<VisibleLoadingSurface[]> {
	return page.locator(RESIDUAL_LOADING_SELECTOR).evaluateAll((elements) =>
		elements
			.filter((element) => {
				const style = getComputedStyle(element);
				const rect = element.getBoundingClientRect();
				return (
					!element.hasAttribute("hidden") &&
					rect.width > 0 &&
					rect.height > 0 &&
					style.display !== "none" &&
					style.visibility !== "hidden" &&
					Number(style.opacity || "1") > 0
				);
			})
			.map((element) => ({
				tag: element.tagName.toLowerCase(),
				className:
					typeof element.className === "string" ? element.className : "",
				label: element.getAttribute("aria-label") ?? "",
				role: element.getAttribute("role") ?? "",
			})),
	);
}

export async function assertNoResidualLoading(
	page: Page,
	route: string,
): Promise<void> {
	await expect
		.poll(() => visibleLoadingSurfaces(page), {
			timeout: 5_000,
			message: `${route} left a fallback or loading surface visible`,
		})
		.toEqual([]);
}
