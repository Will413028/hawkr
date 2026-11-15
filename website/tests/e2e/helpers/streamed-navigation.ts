import type { Page, Response } from "@playwright/test";

/**
 * Navigation for e2e specs. Pages contain streamed Suspense boundaries, and
 * `domcontentloaded` fires before React reveals the streamed content: until
 * then a hidden staging copy `body > div[hidden][id^="S:"]` coexists with the
 * hydrated page, so text and CSS locators can hit strict-mode duplicates. A
 * boundary still showing its fallback has a `template[id^="B:"]` marker and no
 * staging copy yet, so these helpers resolve only once neither a pending
 * marker nor a staging copy remains. Specs must not pass
 * `waitUntil: "domcontentloaded"` themselves (scripts/check-e2e-streamed-goto.ts).
 * `options.timeout` is one deadline shared by the navigation and the reveal wait.
 */
const STAGING_SELECTOR = 'template[id^="B:"], body > div[hidden][id^="S:"]';

type GotoOptions = NonNullable<Parameters<Page["goto"]>[1]>;
type ReloadOptions = NonNullable<Parameters<Page["reload"]>[0]>;

async function waitForStreamReveal(
	page: Page,
	timeout: number | undefined,
	startedAt: number,
) {
	const remaining =
		timeout === undefined
			? undefined
			: Math.max(1, timeout - (Date.now() - startedAt));
	try {
		await page.waitForFunction(
			(selector) => document.querySelector(selector) === null,
			STAGING_SELECTOR,
			remaining === undefined ? undefined : { timeout: remaining },
		);
	} catch (error) {
		const pending = await page
			.evaluate(
				(selector) =>
					Array.from(document.querySelectorAll(selector)).map((el) => el.id),
				STAGING_SELECTOR,
			)
			.catch(() => ["<unreadable>"]);
		throw new Error(
			`Streamed content not revealed at ${page.url()}; remaining ids: ${pending.join(", ") || "<none>"}`,
			{ cause: error },
		);
	}
}

export async function gotoStreamed(
	page: Page,
	url: string,
	options: Omit<GotoOptions, "waitUntil"> = {},
): Promise<Response | null> {
	const startedAt = Date.now();
	const response = await page.goto(url, {
		...options,
		waitUntil: "domcontentloaded",
	});
	await waitForStreamReveal(page, options.timeout, startedAt);
	return response;
}

export async function reloadStreamed(
	page: Page,
	options: Omit<ReloadOptions, "waitUntil"> = {},
): Promise<Response | null> {
	const startedAt = Date.now();
	const response = await page.reload({
		...options,
		waitUntil: "domcontentloaded",
	});
	await waitForStreamReveal(page, options.timeout, startedAt);
	return response;
}
