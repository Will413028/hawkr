/**
 * Validate an absolute (or origin-relative) URL before handing it to a
 * navigation sink (`location.assign` / `location.replace` / `window.open`).
 *
 * Only `http:` / `https:` targets are allowed. A `javascript:` / `data:` /
 * `vbscript:` value — which executes script when navigated to via
 * `location.assign` in some engines — therefore can never reach the sink.
 * Returns "" for a missing value or a non-http(s) scheme; callers treat "" as
 * "refuse to navigate" (show an error instead of blindly redirecting).
 *
 * Single source of truth for the check the checkout flow used to inline; reused
 * by the payment / refund / store-map redirects so a compromised or malformed
 * backend `redirectUrl` can't turn a navigation into script execution.
 *
 * Browser-only — resolves relative URLs against `window.location.origin`.
 */
export function safeRedirectUrl(value: string | null | undefined): string {
	if (!value) return "";
	try {
		const url = new URL(value, window.location.origin);
		if (url.protocol !== "https:" && url.protocol !== "http:") return "";
		return url.toString();
	} catch {
		return "";
	}
}
