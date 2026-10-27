let settledPromise: Promise<void> | null = null;

/**
 * A single browser-wide boundary for third-party code that imperatively inserts
 * nodes into <head> or <body>.
 *
 * `window.load` alone is not a React-hydration boundary: a protected route can
 * still be admitting its authenticated/workspace subtree behind the branded
 * initial loader after the document and all subresources have loaded. Injecting
 * Sonner/analytics nodes in that window occasionally changes the document while
 * React is still claiming it and produces recoverable hydration error #418.
 *
 * Wait for both the native load event and the loader's explicit application
 * handoff, then allow two paint frames for that commit to settle before an
 * unmanaged node is introduced.
 */
export function whenDocumentSettled(): Promise<void> {
	if (typeof window === "undefined" || typeof document === "undefined") {
		return Promise.resolve();
	}
	if (settledPromise) return settledPromise;

	settledPromise = new Promise((resolve) => {
		let scheduled = false;
		const root = document.documentElement;
		const observer = new MutationObserver(() => trySettle());

		const finish = () => {
			observer.disconnect();
			window.removeEventListener("load", trySettle);
			window.requestAnimationFrame(() => {
				window.requestAnimationFrame(() => {
					// Observable proof this barrier released — third-party mounts
					// gated behind it (GA4, Sonner) are otherwise invisible from
					// outside this module.
					root.setAttribute("data-hk-document-settled", "true");
					resolve();
				});
			});
		};

		function trySettle() {
			if (
				scheduled ||
				document.readyState !== "complete" ||
				root.classList.contains("initial-loader-active")
			) {
				return;
			}
			scheduled = true;
			finish();
		}

		observer.observe(root, {
			attributes: true,
			attributeFilter: ["class"],
		});
		window.addEventListener("load", trySettle, { once: true });
		trySettle();
	});

	return settledPromise;
}
