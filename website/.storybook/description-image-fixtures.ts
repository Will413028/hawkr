import { HttpResponse, http } from "msw";

declare const __STORYBOOK_BASE_PATH__: string;

export const descriptionImageSvg =
	'<svg xmlns="http://www.w3.org/2000/svg" width="640" height="400" viewBox="0 0 640 400"><rect width="640" height="400" fill="#e1dbce"/><circle cx="230" cy="200" r="105" fill="#334c0f"/><rect x="370" y="110" width="90" height="180" rx="45" fill="#682e0e"/></svg>';
export const descriptionImageDataUrl = `data:image/svg+xml,${encodeURIComponent(descriptionImageSvg)}`;

/** Only these isolated stories activate the worker. CDN parsing stays real;
 * fixture image responses are generated locally before any network request. */
export async function startDescriptionImageFixtures() {
	const { setupWorker } = await import("msw/browser");
	const worker = setupWorker(
		http.get(
			/^https:\/\/(?:test-)?media\.example\.com\/(?:i\/original\/)?public\/storybook\/(?:image-(?:old|new)|deletion-fixture)\.svg$/,
			() =>
				new HttpResponse(descriptionImageSvg, {
					headers: { "Content-Type": "image/svg+xml" },
				}),
		),
	);
	await worker.start({
		quiet: true,
		onUnhandledRequest: "bypass",
		serviceWorker: { url: `${__STORYBOOK_BASE_PATH__}mockServiceWorker.js` },
	});
	return () => worker.stop();
}
