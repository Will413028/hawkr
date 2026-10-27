import type { Page } from "@playwright/test";

declare global {
	interface Window {
		/** Test-only bridge: captures the GSI callback so the mock button can invoke it. */
		__gsiCb?: (response: { credential: string }) => void;
		/** Test-only: the `nonce` of every `initialize` call, in order (undefined = none given). */
		__gsiNonces?: (string | undefined)[];
	}
}

/** Stub window.google.accounts.id so GIS buttons (the vendor portal's step-up) emit a test idToken. */
export async function installProviderMocks(
	page: Page,
	idToken = "test-id-token",
) {
	await page.addInitScript((tok) => {
		window.google = {
			accounts: {
				id: {
					initialize(cfg: {
						callback: (r: { credential: string }) => void;
						nonce?: string;
					}) {
						window.__gsiCb = cfg.callback;
						window.__gsiNonces = [...(window.__gsiNonces ?? []), cfg.nonce];
					},
					renderButton(el: HTMLElement) {
						const b = document.createElement("button");
						b.textContent = "Mock Google";
						b.setAttribute("data-testid", "mock-google");
						b.onclick = () => {
							window.__gsiCb?.({ credential: tok });
						};
						el.appendChild(b);
					},
					prompt() {},
				},
			},
		};
	}, idToken);
}
