/**
 * Rate-limit subject attestation for server-side calls to the gateway
 * (backend#2264).
 *
 * The gateway meters every per-IP quota against a "subject" — who a request is
 * charged to — rather than the peer address it happens to arrive from. That
 * distinction only matters for OUR OWN server-side callers: this Worker's SSR,
 * its `/api/public/*` route handlers and the CI runner each present ONE address
 * on behalf of an unbounded number of people, so without an attestation every
 * visitor of the site shares a single quota bucket. That is the defect #2264
 * exists to fix.
 *
 * The header is a self-issued, HMAC-signed claim:
 *
 *   X-HK-Origin-Auth: <base64url(payload)>.<base64url(mac)>
 *   payload = {"v":1,"p":"visitor"|"ssr"|"ci","ip":"<visitor ip>","ts":<unix>,
 *              "m":"<METHOD>","u":"<path>"}
 *   mac     = HMAC-SHA256(secret, "hkorigin1:" || base64url(payload))
 *
 * Verified by `gateway/internal/middleware/ratelimit_subject.go`. Every element
 * of that shape is load-bearing there, so treat it as a wire contract: the
 * domain prefix binds the MAC to this purpose, `m`/`u` bind it to one request so
 * a captured signature cannot be pointed at another endpoint, and `ts` bounds
 * its life to ±60s. Because the MAC covers the encoded body verbatim, the JSON
 * field ORDER here is free — but the field NAMES and the base64url (unpadded)
 * encoding are not.
 *
 * Verification is fail-open on the gateway side: a signature it cannot verify
 * degrades to the peer address, i.e. exactly the pre-#2264 behaviour. So a
 * mistake here never takes the site down — it silently stops working, which is
 * why the gateway meters `gateway_origin_auth_total{result=...}` and alerts on
 * the bad_sig/expired/malformed ratio.
 *
 * SERVER ONLY. `lib/api/client.ts` reaches this module through a dynamic
 * `import()` behind `typeof window === "undefined"`, because that file is in the
 * browser graph (hooks, apple-pay, sse) and neither the secret read nor the
 * signing code has any business being shipped there.
 */

/** Bumped by the gateway on any payload-shape change; must match `originAuthVersion`. */
const VERSION = 1;

/** Must match `originAuthDomain` in the gateway. */
const DOMAIN = "hkorigin1:";

const HEADER_NAME = "X-HK-Origin-Auth";

/**
 * Which key signed this, and therefore which claims it is allowed to make.
 *
 * Two keys, not one, and the split is deliberate (gateway §5.2 reasoning):
 * whoever holds the CI key would otherwise be able to sign `p=visitor` carrying
 * any victim's IP and lock that person out of login — while metering as a
 * success, so no alert fires. A GitHub Actions secret and a Cloudflare Worker
 * secret have very different exposure, so they are different keys.
 *
 * Presence of `ORIGIN_AUTH_CI_SECRET` is therefore also the signal that this
 * process IS a CI runner: it is only ever set on a GitHub Actions runner (the
 * deployed Worker never receives it), the gateway only accepts `p=ci` under it,
 * and a runner has no real visitors whose IP would mean anything.
 */
function resolveSigner(): { secret: string; ci: boolean } | null {
	const ci = process.env.ORIGIN_AUTH_CI_SECRET?.trim();
	if (ci) return { secret: ci, ci: true };
	const main = process.env.ORIGIN_AUTH_SECRET?.trim();
	if (main) return { secret: main, ci: false };
	// Unset ⇒ do not sign. The gateway falls back to the peer address, which is
	// the behaviour this site had before #2264 — degraded, never broken.
	return null;
}

// `crypto.subtle` rather than `node:crypto`: it exists on workerd, in Node and
// in the browser, so this module carries no platform-only import that a bundler
// would have to resolve for a graph it is never executed in.
const keyCache = new Map<string, Promise<CryptoKey>>();

function hmacKey(secret: string): Promise<CryptoKey> {
	const cached = keyCache.get(secret);
	if (cached) return cached;
	const imported = crypto.subtle.importKey(
		"raw",
		new TextEncoder().encode(secret),
		{ name: "HMAC", hash: "SHA-256" },
		false,
		["sign"],
	);
	keyCache.set(secret, imported);
	return imported;
}

/** base64url without padding — `base64.RawURLEncoding` on the verifying side. */
function base64url(bytes: Uint8Array): string {
	let binary = "";
	for (const byte of bytes) binary += String.fromCharCode(byte);
	return btoa(binary)
		.replace(/\+/g, "-")
		.replace(/\//g, "_")
		.replace(/=+$/, "");
}

export interface OriginAuthInput {
	/** Uppercase HTTP method — compared against the gateway's `c.Request.Method`. */
	method: string;
	/** Absolute request URL, or the gateway path itself. */
	url: string;
	/**
	 * The end user this call is made on behalf of, when there is one and we can
	 * see it (middleware and route handlers can; a cached RSC render cannot).
	 * Present ⇒ `p=visitor`, absent ⇒ `p=ssr`.
	 */
	visitorIp?: string | null;
}

/**
 * The path the gateway will compare against.
 *
 * `c.Request.URL.Path` is the decoded path, `URL.pathname` is the encoded one.
 * They differ only for a path segment carrying an escaped character, which none
 * of this site's gateway paths (numeric ids and fixed segments) produces; if one
 * ever did, the signature would simply not verify and that request would fall
 * back to the peer address.
 */
function pathOf(url: string): string {
	try {
		return new URL(url, "https://origin-auth.invalid").pathname;
	} catch {
		return url;
	}
}

/**
 * Signs one request, or returns null when this process holds no key.
 *
 * Never throws: a signing failure must degrade to an unsigned request rather
 * than fail the read it was decorating.
 */
export async function signOriginAuth(
	input: OriginAuthInput,
): Promise<string | null> {
	const signer = resolveSigner();
	if (!signer) return null;
	const visitorIp = input.visitorIp?.trim();
	try {
		const payload: Record<string, unknown> = {
			v: VERSION,
			p: signer.ci ? "ci" : visitorIp ? "visitor" : "ssr",
			ts: Math.floor(Date.now() / 1000),
			m: input.method.toUpperCase(),
			u: pathOf(input.url),
		};
		// Only meaningful for p=visitor, and the gateway rejects a p=visitor claim
		// whose ip does not parse — so never send a placeholder.
		if (!signer.ci && visitorIp) payload.ip = visitorIp;
		const body = base64url(new TextEncoder().encode(JSON.stringify(payload)));
		const mac = await crypto.subtle.sign(
			"HMAC",
			await hmacKey(signer.secret),
			new TextEncoder().encode(DOMAIN + body),
		);
		return `${body}.${base64url(new Uint8Array(mac))}`;
	} catch {
		return null;
	}
}

/** Header name, exported so no caller has to spell it twice. */
export const ORIGIN_AUTH_HEADER = HEADER_NAME;
