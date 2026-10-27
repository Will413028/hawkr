import { createSign, generateKeyPairSync } from "node:crypto";
import type { Page } from "@playwright/test";

/**
 * Hermetic Clerk sign-in for the e2e (ADR 2026-09-27-identity-clerk).
 *
 * The sign-in state is Clerk's on both sides, and the suite has no Clerk
 * instance (the key in CI is a fake one), so both sides are stood in for:
 *
 * - Server: `clerkMiddleware` verifies the `__session` cookie locally when the
 *   instance's public key is given as `CLERK_JWT_KEY` (Clerk's networkless
 *   verification: no call to Clerk's Frontend API). `ensureClerkSigningKey()`,
 *   run from playwright.config.ts, makes a throwaway RSA key pair per run and
 *   puts the public half into `CLERK_JWT_KEY` for the webServer; the private
 *   half only lives in this process tree and signs the session JWT below.
 * - Browser: the real clerk-js would load from the (fake) Frontend API domain.
 *   `window.Clerk` is installed before any app code instead, so
 *   `@clerk/react`'s IsomorphicClerk sees a ready, signed-in instance and
 *   never fetches the script. It implements what the app uses: the listener
 *   state behind `useAuth`, `session.getToken()` (the Bearer the browser sends
 *   to the monolith; every such call is intercepted by page.route anyway) and
 *   `signOut`.
 *
 * Guest specs install none of this and stay signed out.
 */

const PRIVATE_KEY_ENV = "PLAYWRIGHT_CLERK_PRIVATE_KEY";
const TOKEN_TTL_SECONDS = 60 * 60;

/** Called from playwright.config.ts; a worker re-evaluating the config keeps the key its parent made. */
export function ensureClerkSigningKey(): void {
	if (process.env[PRIVATE_KEY_ENV] && process.env.CLERK_JWT_KEY) return;
	const { publicKey, privateKey } = generateKeyPairSync("rsa", {
		modulusLength: 2048,
		publicKeyEncoding: { type: "spki", format: "pem" },
		privateKeyEncoding: { type: "pkcs8", format: "pem" },
	});
	process.env[PRIVATE_KEY_ENV] = privateKey;
	process.env.CLERK_JWT_KEY = publicKey;
}

function b64url(input: string | Buffer): string {
	return Buffer.from(input).toString("base64url");
}

function signSessionJwt(userId: string, sessionId: string, iat: number) {
	const privateKey = process.env[PRIVATE_KEY_ENV];
	if (!privateKey) {
		throw new Error(
			"PLAYWRIGHT_CLERK_PRIVATE_KEY is not set: playwright.config.ts must call ensureClerkSigningKey()",
		);
	}
	const header = b64url(JSON.stringify({ alg: "RS256", typ: "JWT" }));
	const payload = b64url(
		JSON.stringify({
			iss: "https://clerk.example.org",
			sub: userId,
			sid: sessionId,
			iat,
			nbf: iat,
			exp: iat + TOKEN_TTL_SECONDS,
		}),
	);
	const signature = createSign("RSA-SHA256")
		.update(`${header}.${payload}`)
		.sign(privateKey);
	return `${header}.${payload}.${b64url(signature)}`;
}

/** Installed with addInitScript, so it must be self-contained. */
function installClerkStub(args: {
	token: string;
	userId: string;
	sessionId: string;
}) {
	const { token, userId, sessionId } = args;
	const user = { id: userId, organizationMemberships: [] };
	const session: Record<string, unknown> = {
		id: sessionId,
		status: "active",
		user,
		factorVerificationAge: null,
		lastActiveToken: { jwt: { claims: { sub: userId, sid: sessionId } } },
		getToken: async () => token,
	};
	const client = { sessions: [session], lastActiveSessionId: sessionId };
	let resources: Record<string, unknown> = {
		client,
		session,
		user,
		organization: null,
	};
	const listeners = new Set<(r: Record<string, unknown>) => void>();
	const emit = () => {
		clerk.__internal_lastEmittedResources = resources;
		for (const listener of listeners) listener(resources);
	};
	const clerk: Record<string, unknown> = {
		loaded: false,
		status: "loading",
		version: "e2e-stub",
		isSignedIn: true,
		client,
		session,
		user,
		organization: null,
		__internal_lastEmittedResources: undefined,
		async load() {
			clerk.loaded = true;
			clerk.status = "ready";
			emit();
		},
		addListener(
			listener: (r: Record<string, unknown>) => void,
			options?: { skipInitialEmit?: boolean },
		) {
			listeners.add(listener);
			if (clerk.loaded && !options?.skipInitialEmit) listener(resources);
			return () => listeners.delete(listener);
		},
		on() {},
		off() {},
		__internal_getOption: () => undefined,
		async signOut(callback?: () => unknown) {
			for (const name of ["__session", "__client_uat"]) {
				document.cookie = `${name}=; Max-Age=0; path=/`;
			}
			clerk.isSignedIn = false;
			clerk.session = null;
			clerk.user = null;
			resources = { client, session: null, user: null, organization: null };
			emit();
			await callback?.();
		},
	};
	// Anything else clerk-js offers (navigation helpers, UI openers, ...) is a
	// resolved no-op: this stub only serves a signed-in app shell.
	(window as unknown as { Clerk: unknown }).Clerk = new Proxy(clerk, {
		get(target, prop) {
			if (prop in target) return target[prop as string];
			if (typeof prop === "symbol" || prop === "then") return undefined;
			return () => Promise.resolve();
		},
	});
}

/** Make `page` a Clerk-signed-in browser for account `accountId`. Call before the first navigation. */
export async function signInWithClerk(
	page: Page,
	accountId: string,
): Promise<void> {
	const userId = `user_e2e_${accountId}`;
	const sessionId = `sess_e2e_${accountId}`;
	const iat = Math.floor(Date.now() / 1000);
	const token = signSessionJwt(userId, sessionId, iat);
	const origin = `http://localhost:${process.env.PLAYWRIGHT_PORT || "3000"}`;
	await page.context().addCookies([
		{ name: "__session", value: token, url: origin },
		// Clerk treats a token issued before the client_uat as stale and handshakes.
		{ name: "__client_uat", value: String(iat - 60), url: origin },
	]);
	await page.addInitScript(installClerkStub, { token, userId, sessionId });
}
