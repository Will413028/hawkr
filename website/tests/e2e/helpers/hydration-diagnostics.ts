import { type Page, test } from "@playwright/test";

export const HYDRATION_ERROR =
	/Hydration failed|react\.dev\/errors\/(?:418|419|422|423|425)\b|Minified React error #(?:418|419|422|423|425)\b/;

const CLIENT_RERENDER_DELAY_MS = 300;

function withoutHash(url: string): string {
	const index = url.indexOf("#");
	return index === -1 ? url : url.slice(0, index);
}

/**
 * Names the mismatch behind a hydration error, which the production build
 * reports only as a minified code. Records the SSR HTML of main-frame
 * navigations and, when a hydration error appears as a pageerror or a console
 * error, attaches the SSR HTML, the client DOM and a short info file to the
 * running test. The capture is awaited before the page's context closes, and
 * it never throws or alters what a spec asserts.
 */
export function captureHydrationDiagnostics(page: Page): void {
	const testInfo = test.info();
	const ssrByUrl = new Map<string, { url: string; body: string }>();
	let latestSsr: { url: string; body: string } | undefined;
	let reported = false;
	let pending: Promise<void> = Promise.resolve();

	const context = page.context();
	const closeContext = context.close.bind(context);
	context.close = async (options) => {
		await pending;
		return closeContext(options);
	};

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
				const entry = { url: response.url(), body };
				ssrByUrl.set(withoutHash(response.url()), entry);
				latestSsr = entry;
			},
			() => {},
		);
	});

	const onHydrationError = (message: string) => {
		if (reported) return;
		reported = true;
		const detectedAt = new Date().toISOString();
		pending = (async () => {
			try {
				await new Promise((resolve) =>
					setTimeout(resolve, CLIENT_RERENDER_DELAY_MS),
				);
				const url = page.url();
				const ssr = ssrByUrl.get(withoutHash(url)) ?? latestSsr;
				const client = await page.content();
				const capturedAt = new Date().toISOString();
				await testInfo.attach("hydration-ssr.html", {
					body: ssr?.body ?? "SSR HTML not captured",
					contentType: ssr ? "text/html" : "text/plain",
				});
				await testInfo.attach("hydration-client.html", {
					body: client,
					contentType: "text/html",
				});
				await testInfo.attach("hydration-info.txt", {
					body: [
						`error: ${message}`,
						`page url: ${url}`,
						`ssr source url: ${ssr?.url ?? "none"}`,
						`error detected at: ${detectedAt}`,
						`client html captured at: ${capturedAt}`,
						"",
					].join("\n"),
					contentType: "text/plain",
				});
			} catch {
			}
		})();
	};

	page.on("pageerror", (error) => {
		if (HYDRATION_ERROR.test(error.message)) onHydrationError(error.message);
	});
	page.on("console", (message) => {
		if (message.type() === "error" && HYDRATION_ERROR.test(message.text())) {
			onHydrationError(message.text());
		}
	});
}
