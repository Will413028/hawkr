import type { Page } from "@playwright/test";
import { SESSION_COOKIE_NAME } from "../../../lib/auth/session-cookie";
import {
	encodeSessionValue,
	type SessionIdentity,
} from "./fake-session-upstream";

/**
 * Hermetic sign-in for the e2e (ADR 2026-10-30-identity-self-hosted-federated-login).
 *
 * The credential is the HttpOnly `__Host-hk_session` cookie, and the site reads it in exactly two
 * places: middleware.ts only checks that the cookie is PRESENT (an optimistic gate that never
 * calls the API), and the server asks GetSession for who the account is (lib/auth/dal.ts) — which the
 * e2e webServer answers from helpers/fake-session-upstream.ts, decoding the account out of this very
 * cookie. So signing in here is: set the cookie that carries the identity. Guest specs set no cookie, so
 * the server answers guest without asking anyone.
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
	identity: SessionIdentity,
): Promise<void> {
	const origin = `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;
	const url = `${origin}/__e2e/session`;
	await page.route(url, (route) =>
		route.fulfill({
			status: 200,
			contentType: "text/html",
			headers: {
				"set-cookie": `${SESSION_COOKIE_NAME}=${encodeSessionValue(identity)}; Path=/; Secure; HttpOnly; SameSite=Lax`,
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
