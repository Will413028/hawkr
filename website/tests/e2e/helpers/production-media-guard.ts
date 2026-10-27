/**
 * The media host the non-production (test) artifact is built with, i.e. the one a
 * production artifact must never reference. One source: the hostname of
 * `NEXT_PUBLIC_MEDIA_BASE_URL`, the same variable the app reads (tests/setup/hermetic.mjs
 * and .env.example set it to the reserved test name); when it is unset or not a URL,
 * the reserved test name itself.
 */
export const DEFAULT_TEST_MEDIA_HOSTNAME = "test-media.example.com";

export function testMediaHostname(
	env: Record<string, string | undefined> = process.env,
): string {
	const base = env.NEXT_PUBLIC_MEDIA_BASE_URL?.trim();
	if (base) {
		try {
			return new URL(base).hostname.toLowerCase().replace(/\.+$/, "");
		} catch {
			// fall through to the reserved name
		}
	}
	return DEFAULT_TEST_MEDIA_HOSTNAME;
}

export const TEST_MEDIA_HOSTNAME = testMediaHostname();

function normalizedHostname(url: string, baseUrl?: string): string | null {
	try {
		return new URL(url, baseUrl).hostname.toLowerCase().replace(/\.+$/, "");
	} catch {
		return null;
	}
}

export function isProductionArtifact(flavor: string | undefined): boolean {
	return flavor?.trim().toLowerCase() === "prod";
}

export function isForbiddenProductionMediaReference(
	flavor: string | undefined,
	url: string,
	baseUrl?: string,
): boolean {
	return (
		isProductionArtifact(flavor) &&
		normalizedHostname(url, baseUrl) === TEST_MEDIA_HOSTNAME
	);
}
