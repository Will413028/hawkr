import type { Page, Response } from "@playwright/test";

/**
 * Navigation for e2e specs. Every route streams (RootDocument wraps the
 * `<main>` children in Suspense), and `domcontentloaded` fires before React
 * reveals the streamed content: until then a hidden staging copy
 * `body > div[hidden][id^="S:"]` coexists with the hydrated page, so text and
 * CSS locators can hit strict-mode duplicates. These helpers resolve only once
 * no staging copy remains. Specs must not pass `waitUntil: "domcontentloaded"`
 * themselves (scripts/check-e2e-streamed-goto.ts).
 */
const STAGING_SELECTOR = 'body > div[hidden][id^="S:"]';

type GotoOptions = NonNullable<Parameters<Page["goto"]>[1]>;
type ReloadOptions = NonNullable<Parameters<Page["reload"]>[0]>;

async function waitForStreamReveal(page: Page, timeout?: number) {
	await page.waitForFunction(
		(selector) => document.querySelector(selector) === null,
		STAGING_SELECTOR,
		timeout === undefined ? undefined : { timeout },
	);
}

export async function gotoStreamed(
	page: Page,
	url: string,
	options: Omit<GotoOptions, "waitUntil"> = {},
): Promise<Response | null> {
	const response = await page.goto(url, {
		...options,
		waitUntil: "domcontentloaded",
	});
	await waitForStreamReveal(page, options.timeout);
	return response;
}

export async function reloadStreamed(
	page: Page,
	options: Omit<ReloadOptions, "waitUntil"> = {},
): Promise<Response | null> {
	const response = await page.reload({
		...options,
		waitUntil: "domcontentloaded",
	});
	await waitForStreamReveal(page, options.timeout);
	return response;
}
