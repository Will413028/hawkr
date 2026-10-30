import type { Page } from "@playwright/test";
import { SESSION_COOKIE_NAME } from "../../../lib/auth/session-cookie";

/**
 * Hermetic sign-in for the e2e (ADR 2026-10-30-identity-self-hosted-federated-login).
 *
 * The credential is the HttpOnly `__Host-hk_session` cookie, and the site reads it in exactly two
 * places: middleware.ts only checks that the cookie is PRESENT (an optimistic gate that never
 * calls the API), and the browser asks GetSession for who the account is — which the suite
 * intercepts (helpers/mock-gateway.ts). So signing in here is: set the cookie the middleware wants,
 * and let the mock answer GetSession with the account. The value is opaque to both and never
 * verified. Guest specs set no cookie and the mock answers GetSession `unauthenticated`.
 */

/**
 * Make `page` a signed-in browser. Call before the first navigation.
 *
 * The cookie is set by a Set-Cookie response header on a route-fulfilled same-origin page, not by
 * `addCookies`: Chromium's CDP `Storage.setCookies` rejects `__Host-` cookies whose origin is http
 * ("Invalid cookie fields"), while a response header from http://localhost is accepted.
 */
export async function signInWithSessionCookie(
	page: Page,
	accountId: string,
): Promise<void> {
	const origin = `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;
	const url = `${origin}/__e2e/session`;
	await page.route(url, (route) =>
		route.fulfill({
			status: 200,
			contentType: "text/html",
			headers: {
				"set-cookie": `${SESSION_COOKIE_NAME}=e2e-session-${accountId}; Path=/; Secure; HttpOnly; SameSite=Lax`,
			},
			body: "<!doctype html><title>e2e session</title>",
		}),
	);
	try {
		await page.goto(url);
	} finally {
		await page.unroute(url);
	}
	const cookies = await page.context().cookies(origin);
	if (!cookies.some((c) => c.name === SESSION_COOKIE_NAME)) {
		throw new Error(
			`signInWithSessionCookie: ${SESSION_COOKIE_NAME} was not stored after the Set-Cookie response from ${url}`,
		);
	}
}
