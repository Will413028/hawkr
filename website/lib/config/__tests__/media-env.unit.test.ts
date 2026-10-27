import { afterEach, describe, expect, it } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import { __setFlavorForTest } from "@/lib/config/flavor";

const originalMediaBase = process.env.NEXT_PUBLIC_MEDIA_BASE_URL;

function resolveMediaBase(
	flavor: "dev" | "prod",
	value: string | undefined,
): string {
	if (value === undefined) {
		delete process.env.NEXT_PUBLIC_MEDIA_BASE_URL;
	} else {
		process.env.NEXT_PUBLIC_MEDIA_BASE_URL = value;
	}
	__setFlavorForTest(flavor);
	__setEnvForTest(null);
	return getEnv().mediaBaseUrl;
}

afterEach(() => {
	if (originalMediaBase === undefined) {
		delete process.env.NEXT_PUBLIC_MEDIA_BASE_URL;
	} else {
		process.env.NEXT_PUBLIC_MEDIA_BASE_URL = originalMediaBase;
	}
	__setFlavorForTest(null);
	__setEnvForTest(null);
});

describe("configured media origin", () => {
	it("is empty when unset on either flavor: no media URL can be built", () => {
		expect(resolveMediaBase("prod", undefined)).toBe("");
		expect(resolveMediaBase("prod", "")).toBe("");
		expect(resolveMediaBase("dev", undefined)).toBe("");
	});

	it("is exactly the configured origin, whatever the host", () => {
		expect(resolveMediaBase("prod", "https://cdn.example/")).toBe(
			"https://cdn.example",
		);
		expect(resolveMediaBase("dev", "http://localhost:8787")).toBe(
			"http://localhost:8787",
		);
	});

	it("returns the canonical origin for an equivalent valid spelling", () => {
		expect(resolveMediaBase("prod", "HTTPS://CDN.example:443/")).toBe(
			"https://cdn.example",
		);
	});

	it("rejects credentials even though URL.origin omits them", () => {
		expect(() =>
			resolveMediaBase(
				"prod",
				"https://deploy-user:deploy-password@cdn.example",
			),
		).toThrow(/NEXT_PUBLIC_MEDIA_BASE_URL must be a credential-free/);
	});

	it.each([
		"https://cdn.example?",
		"https://cdn.example#",
		"https://cdn.example/media/..",
		"not a url",
		"ftp://cdn.example",
	])("rejects syntax that would corrupt the appended path: %s", (url) => {
		expect(() => resolveMediaBase("prod", url)).toThrow(
			/NEXT_PUBLIC_MEDIA_BASE_URL must be/,
		);
	});
});

describe("isConfiguredMediaUrl compares the full origin", () => {
	it("rejects another port or scheme on the media host, accepts the same origin", async () => {
		const { isConfiguredMediaUrl } = await import("@/lib/media/cdn");
		resolveMediaBase("prod", "https://cdn.example");
		expect(isConfiguredMediaUrl(new URL("https://cdn.example/a.png"))).toBe(
			true,
		);
		expect(isConfiguredMediaUrl(new URL("https://cdn.example:443/a.png"))).toBe(
			true,
		);
		expect(
			isConfiguredMediaUrl(new URL("https://cdn.example:8443/a.png")),
		).toBe(false);
		expect(isConfiguredMediaUrl(new URL("http://cdn.example/a.png"))).toBe(
			false,
		);
		resolveMediaBase("prod", "https://cdn.example:8443");
		expect(
			isConfiguredMediaUrl(new URL("https://cdn.example:8443/a.png")),
		).toBe(true);
		expect(isConfiguredMediaUrl(new URL("https://cdn.example/a.png"))).toBe(
			false,
		);
	});
});
