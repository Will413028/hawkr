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

/** Make `page` a signed-in browser. Call before the first navigation. */
export async function signInWithSessionCookie(
	page: Page,
	accountId: string,
): Promise<void> {
	await page.context().addCookies([
		{
			name: SESSION_COOKIE_NAME,
			value: `e2e-session-${accountId}`,
			// `__Host-` requires Secure, Path=/ and no Domain attribute; Chromium accepts
			// Secure cookies on http://localhost, which is where the suite's server runs.
			url: `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`,
			secure: true,
			httpOnly: true,
			sameSite: "Lax",
		},
	]);
}
