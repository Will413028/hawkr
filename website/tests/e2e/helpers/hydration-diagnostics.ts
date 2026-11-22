import { type Page, test } from "@playwright/test";
import {
	firstTextDifferences,
	formatTextDifferences,
} from "../../../scripts/hydration-text-diff";

const HYDRATION_ERROR = /Minified React error #(418|423|425)|Hydration failed/;

/**
 * Names the component-level mismatch behind a hydration pageerror, which the
 * production build reports only as a minified code. Records the SSR HTML of
 * main-frame navigations and, when a hydration error fires, attaches that HTML,
 * the client-regenerated DOM and the first differing text tokens to the running
 * test. It never throws and does not alter what a spec asserts.
 */
export function captureHydrationDiagnostics(page: Page): void {
	const testInfo = test.info();
	const ssrByUrl = new Map<string, string>();
	let reported = false;

	page.on("response", (response) => {
		const request = response.request();
		if (
			!request.isNavigationRequest() ||
			request.frame() !== page.mainFrame()
		) {
			return;
		}
		void response.text().then(
			(body) => {
				ssrByUrl.set(response.url(), body);
			},
			() => {},
		);
	});

	page.on("pageerror", (error) => {
		if (reported || !HYDRATION_ERROR.test(error.message)) return;
		reported = true;
		void (async () => {
			try {
				await new Promise((resolve) => setTimeout(resolve, 500));
				const url = page.url();
				const ssr = ssrByUrl.get(url) ?? "";
				const client = await page.content();
				await testInfo.attach("hydration-ssr.html", {
					body: ssr,
					contentType: "text/html",
				});
				await testInfo.attach("hydration-client.html", {
					body: client,
					contentType: "text/html",
				});
				await testInfo.attach("hydration-diff.txt", {
					body: `url: ${url}\n${formatTextDifferences(firstTextDifferences(ssr, client))}\n`,
					contentType: "text/plain",
				});
				console.log(
					`hydration mismatch at ${url}: see attachments hydration-ssr.html, hydration-client.html, hydration-diff.txt`,
				);
			} catch {
				// Diagnostics must never affect the test outcome.
			}
		})();
	});
}
