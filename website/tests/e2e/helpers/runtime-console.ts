const GOOGLE_PAY_REPORT_ONLY_FRAME_ANCESTORS =
	/Framing 'https:\/\/pay\.google\.com\/' violates the following report-only Content Security Policy directive: "frame-ancestors 'self'"/;

/**
 * Browser diagnostics that are known to be non-enforcing and therefore do not
 * represent a Hawkr runtime failure. Keep every exception exact so an
 * enforced policy, another directive, or another origin still fails smoke.
 */
export function isKnownNonEnforcingConsoleError(message: string): boolean {
	return GOOGLE_PAY_REPORT_ONLY_FRAME_ANCESTORS.test(message);
}
