import { getEnv } from "@/lib/config/env";

/**
 * Whether a hostname produced by `new URL()` is actually a hostname: DNS labels
 * (letters, digits, `-`, plus the `_` some hosts still publish), an optional
 * root dot, or a bracketed IPv6 literal. IDN needs no special case — both
 * engines punycode it during parsing (`例子.tw` → `xn--fsqu00a.tw`).
 *
 * This second layer is not redundant with `new URL()`, because `new URL()`
 * disagrees with itself across engines on a malformed host. Measured
 * 2026-08-20 with `https://not a url`:
 *   Chrome 151 → parses, hostname `not%20a%20url`
 *   Node 24    → throws `Invalid URL`
 * So a check built only on the throw passes every unit test and lets
 * everything through in the browser — i.e. no client-side check at all. The
 * parsed hostname is the same string in both engines for anything legitimate,
 * so validating it here is what makes the two agree.
 *
 * The pattern is written inline rather than hoisted to a module constant so
 * the function stays self-contained under `Function.prototype.toString()` —
 * `tests/e2e/seller-website-url-engine.spec.ts` injects this exact source into
 * a real browser rather than re-deriving the rule there.
 */
export function isValidHostname(hostname: string): boolean {
	return /^(?:[a-z0-9_-]+(?:\.[a-z0-9_-]+)*\.?|\[[0-9a-f:.]+\])$/i.test(
		hostname,
	);
}

/**
 * Mirrors the backend's validateWebsiteURL (seller module): empty string is
 * valid (clears the field), non-empty must be an absolute http(s) URL with a
 * host. Client-side pre-check only — the backend PUT is still the final gate.
 */
export function isValidWebsiteUrl(raw: string): boolean {
	if (raw === "") return true;
	try {
		const u = new URL(raw);
		if (u.protocol !== "http:" && u.protocol !== "https:") return false;
		return isValidHostname(u.hostname);
	} catch {
		return false;
	}
}

/**
 * Add a scheme when the stored value omits it (e.g. `example.com`) so the link
 * resolves to the external site rather than a relative in-app path.
 */
export function normalizeWebsiteUrl(raw: string): string {
	const trimmed = raw.trim();
	if (trimmed === "") return "";
	return /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
}

/**
 * A seller's 官方網站 is by definition an *off-platform* site. The seller
 * registration flow, however, seeds `website_url` with the seller's own
 * Hawkr storefront (`<web origin>/sellers/{id}`), which would make the
 * "訪問官網" link circular — it just reloads the same store page. Treat any URL
 * on the configured web origin's host (or a subdomain of it) as "no external
 * website" so the affordance only appears when the seller actually provided
 * their own site.
 */
export function isExternalWebsiteUrl(raw: string): boolean {
	const normalized = normalizeWebsiteUrl(raw);
	if (normalized === "") return false;
	try {
		const u = new URL(normalized);
		if (u.protocol !== "http:" && u.protocol !== "https:") return false;
		const host = u.hostname.toLowerCase();
		// Same engine split as isValidHostname guards against, with a worse
		// symptom: in the browser a stored `https://not a url` parses to the host
		// `not%20a%20url`, which is neither empty nor ours, so the storefront
		// would render 「訪問官網」 pointing at a link that cannot resolve.
		if (!isValidHostname(host) || host === "localhost") return false;
		const own = new URL(getEnv().webOrigin).hostname
			.toLowerCase()
			.replace(/^www\./, "");
		const bare = host.replace(/^www\./, "");
		return bare !== own && !bare.endsWith(`.${own}`);
	} catch {
		return false;
	}
}
