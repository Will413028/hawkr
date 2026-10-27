import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import {
	buildImageUrl,
	isMediaGatewayEnabled,
	ossImageUrl,
	resolveCdnUrl,
	toObjectKey,
} from "../cdn";

const MEDIA_BASE = "https://test-media.example.com";

/** Force the media-gateway gate on/off by overriding the memoised env. */
function setGate(
	mediaBaseUrl: string,
	flavor: "dev" | "prod" = getEnv().flavor,
): void {
	const base = getEnv();
	__setEnvForTest({ ...base, flavor, mediaBaseUrl });
}

afterEach(() => {
	__setEnvForTest(null); // restore real env resolution
});

describe("buildImageUrl — no media origin configured", () => {
	beforeEach(() => setGate("", "prod"));

	it("reports the gateway as disabled", () => {
		expect(isMediaGatewayEnabled()).toBe(false);
	});

	it("builds no URL for a bare object key: the image is absent, never on a guessed host", () => {
		expect(buildImageUrl("public/product/1/a.jpg", "card")).toBe("");
		expect(buildImageUrl("public/x.jpg")).toBe("");
		expect(buildImageUrl("/public/x.jpg", "card")).toBe("");
		expect(buildImageUrl("/secure/id/1.jpg", "card")).toBe("");
	});

	it("never turns a protocol-relative value into a URL on our origin", () => {
		expect(buildImageUrl("//evil.example/x.png", "card")).toBe("");
	});

	it("passes foreign absolute / data / blob URLs through untouched", () => {
		expect(buildImageUrl("https://images.example.org/a.jpg", "card")).toBe(
			"https://images.example.org/a.jpg",
		);
		expect(
			buildImageUrl(`${MEDIA_BASE}/i/original/public/x.jpg?v=1`, "card"),
		).toBe(`${MEDIA_BASE}/i/original/public/x.jpg?v=1`);
		expect(buildImageUrl("data:image/png;base64,AAA", "card")).toBe(
			"data:image/png;base64,AAA",
		);
		expect(buildImageUrl("blob:abc", "card")).toBe("blob:abc");
	});

	it("leaves local /public assets on the page origin", () => {
		expect(buildImageUrl("/defaults/buyer_avatar.png", "avatar")).toBe(
			"/defaults/buyer_avatar.png",
		);
		expect(buildImageUrl("/brand/logo.svg", "avatar")).toBe("/brand/logo.svg");
		// Any rooted path that is not a backend namespace is local — folders added
		// to public/ later must work without touching the classifier.
		expect(buildImageUrl("/generated/lucky-badge.png", "avatar")).toBe(
			"/generated/lucky-badge.png",
		);
		expect(buildImageUrl("/folder-added-tomorrow/a.png", "avatar")).toBe(
			"/folder-added-tomorrow/a.png",
		);
	});

	it("returns empty string for empty / whitespace / nullish input", () => {
		expect(buildImageUrl("", "card")).toBe("");
		expect(buildImageUrl("   ", "card")).toBe("");
		expect(buildImageUrl(null, "card")).toBe("");
		expect(buildImageUrl(undefined, "card")).toBe("");
	});

	it("still reduces a bare key to itself for the write path", () => {
		expect(toObjectKey("public/x.jpg")).toBe("public/x.jpg");
	});
});

describe("buildImageUrl — origin configured (image gateway presets)", () => {
	beforeEach(() => setGate(MEDIA_BASE));

	it("gate reports enabled", () => {
		expect(isMediaGatewayEnabled()).toBe(true);
	});

	it("builds /i/{preset}/{key} for an objectKey", () => {
		expect(buildImageUrl("public/product/1/a.jpg", "card")).toBe(
			`${MEDIA_BASE}/i/card/public/product/1/a.jpg`,
		);
	});

	it("defaults to the original preset", () => {
		expect(buildImageUrl("public/x.jpg")).toBe(
			`${MEDIA_BASE}/i/original/public/x.jpg`,
		);
	});

	it("strips a single leading slash from the key", () => {
		expect(buildImageUrl("/public/x.jpg", "thumb")).toBe(
			`${MEDIA_BASE}/i/thumb/public/x.jpg`,
		);
	});

	it("passes through already-absolute / data / blob URLs (not objectKeys)", () => {
		expect(buildImageUrl("https://other.example/a.jpg", "card")).toBe(
			"https://other.example/a.jpg",
		);
		expect(buildImageUrl("data:image/png;base64,AAA", "card")).toBe(
			"data:image/png;base64,AAA",
		);
	});

	it("leaves local /public assets alone", () => {
		expect(buildImageUrl("/defaults/buyer_avatar.png", "avatar")).toBe(
			"/defaults/buyer_avatar.png",
		);
		expect(buildImageUrl("/generated/lucky-badge.png", "avatar")).toBe(
			"/generated/lucky-badge.png",
		);
		expect(buildImageUrl("/folder-added-tomorrow/a.png", "card")).toBe(
			"/folder-added-tomorrow/a.png",
		);
	});

	it("returns empty string for nullish input", () => {
		expect(buildImageUrl(null, "card")).toBe("");
	});
});

describe("buildImageUrl — restores objectKey from URLs on the configured host", () => {
	beforeEach(() => setGate(MEDIA_BASE));

	it("peels a new-format gateway URL and re-applies the requested preset", () => {
		expect(
			buildImageUrl(`${MEDIA_BASE}/i/original/public/product/1/a.jpg`, "card"),
		).toBe(`${MEDIA_BASE}/i/card/public/product/1/a.jpg`);
	});

	it("peels a URL on the configured host that has no /i/ segment to its key", () => {
		expect(buildImageUrl(`${MEDIA_BASE}/public/x.jpg`, "thumb")).toBe(
			`${MEDIA_BASE}/i/thumb/public/x.jpg`,
		);
	});

	it("peels a configured-host URL that already carries a preset", () => {
		expect(buildImageUrl(`${MEDIA_BASE}/i/thumb/public/x.jpg`, "detail")).toBe(
			`${MEDIA_BASE}/i/detail/public/x.jpg`,
		);
	});

	it("matches the configured host through a trailing dot", () => {
		expect(
			buildImageUrl(
				"https://test-media.example.com./i/thumb/public/x.jpg",
				"detail",
			),
		).toBe(`${MEDIA_BASE}/i/detail/public/x.jpg`);
	});

	it("preserves a signed URL on the configured host byte-for-byte", () => {
		const signedUrl = `${MEDIA_BASE}/public/x.jpg?X-Amz-Expires=300&X-Amz-Signature=deadbeef`;
		expect(buildImageUrl(signedUrl, "detail")).toBe(signedUrl);
	});

	it("recognises only the configured host: any other media-looking host is foreign", () => {
		for (const other of [
			"https://media.example.com/public/x.jpg",
			"https://test-media.example.com.evil.test/public/x.jpg",
			"https://evil-test-media.example.com/public/x.jpg",
		]) {
			expect(buildImageUrl(other, "thumb")).toBe(other);
			expect(toObjectKey(other)).toBe("");
		}
	});

	it("follows whichever origin is configured", () => {
		setGate("https://cdn.example.org");
		expect(buildImageUrl("public/x.jpg", "card")).toBe(
			"https://cdn.example.org/i/card/public/x.jpg",
		);
		expect(
			buildImageUrl("https://cdn.example.org/i/thumb/public/x.jpg", "detail"),
		).toBe("https://cdn.example.org/i/detail/public/x.jpg");
		expect(buildImageUrl(`${MEDIA_BASE}/public/x.jpg`, "detail")).toBe(
			`${MEDIA_BASE}/public/x.jpg`,
		);
	});

	it("preserves query and fragment on an existing gateway URL byte-for-byte", () => {
		const identifiedUrl = `${MEDIA_BASE}/i/avatar/public/x.jpg?v=99#preview`;
		expect(buildImageUrl(identifiedUrl, "detail")).toBe(identifiedUrl);
	});

	it("treats even a bare query or fragment delimiter as opaque", () => {
		const bareQueryUrl = `${MEDIA_BASE}/i/avatar/public/x.jpg?`;
		const bareFragmentUrl = `${MEDIA_BASE}/i/avatar/public/x.jpg#`;
		expect(buildImageUrl(bareQueryUrl, "detail")).toBe(bareQueryUrl);
		expect(buildImageUrl(bareFragmentUrl, "detail")).toBe(bareFragmentUrl);
	});

	it("passes through foreign-host absolute URLs (Google avatar) untouched", () => {
		expect(
			buildImageUrl("https://lh3.googleusercontent.com/a/AAcHTt=s96", "avatar"),
		).toBe("https://lh3.googleusercontent.com/a/AAcHTt=s96");
	});

	it("returns empty string for empty input", () => {
		expect(buildImageUrl("", "card")).toBe("");
	});
});

describe("compat wrappers preserve legacy behaviour", () => {
	it("resolveCdnUrl === buildImageUrl(value, 'original')", () => {
		setGate("");
		expect(resolveCdnUrl("public/a.jpg")).toBe("");
		setGate(MEDIA_BASE);
		expect(resolveCdnUrl("public/a.jpg")).toBe(
			`${MEDIA_BASE}/i/original/public/a.jpg`,
		);
	});

	it("ossImageUrl maps legacy preset names onto the §4.4 whitelist", () => {
		setGate(MEDIA_BASE);
		expect(ossImageUrl("public/a.jpg", "avatar")).toBe(
			`${MEDIA_BASE}/i/avatar/public/a.jpg`,
		);
		expect(ossImageUrl("public/a.jpg", "productThumb")).toBe(
			`${MEDIA_BASE}/i/thumb/public/a.jpg`,
		);
		expect(ossImageUrl("public/a.jpg", "feedCard")).toBe(
			`${MEDIA_BASE}/i/card/public/a.jpg`,
		);
		expect(ossImageUrl("public/a.jpg", "cardThumb")).toBe(
			`${MEDIA_BASE}/i/card/public/a.jpg`,
		);
		expect(ossImageUrl("public/a.jpg", "productDetail")).toBe(
			`${MEDIA_BASE}/i/detail/public/a.jpg`,
		);
		expect(ossImageUrl("public/a.jpg", "sellerCover")).toBe(
			`${MEDIA_BASE}/i/banner/public/a.jpg`,
		);
	});
});
