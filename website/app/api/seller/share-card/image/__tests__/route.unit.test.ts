import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import { GET } from "../route";

const PROXY = "https://web-dev.example.com/api/seller/share-card/image";

function requestFor(imageUrl: string): NextRequest {
	return new NextRequest(`${PROXY}?url=${encodeURIComponent(imageUrl)}`);
}

/** Stub global fetch so an allowed upstream returns a tiny image — proves the
 *  request cleared the host allowlist without touching the network/MSW. */
function stubImageFetch(): ReturnType<typeof vi.fn> {
	const mock = vi.fn(async () => {
		return new Response(new Uint8Array([1, 2, 3]), {
			status: 200,
			headers: { "content-type": "image/webp" },
		});
	});
	vi.stubGlobal("fetch", mock);
	return mock;
}

afterEach(() => {
	vi.unstubAllGlobals();
	__setEnvForTest(null);
});

const MEDIA = "https://media.example.org";

describe("share-card image proxy host allowlist", () => {
	beforeEach(() => {
		__setEnvForTest({ ...getEnv(), mediaBaseUrl: MEDIA });
	});

	it.each([
		`${MEDIA}/public/seller/1/logo.jpg`,
		`${MEDIA}/i/original/public/seller/1/logo.jpg`,
		"https://MEDIA.example.org/public/seller/1/logo.jpg",
	])("allows %s", async (url) => {
		const fetchMock = stubImageFetch();
		const res = await GET(requestFor(url));
		expect(res.status).toBe(200);
		expect(res.headers.get("content-type")).toBe("image/webp");
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it.each([
		"https://evil.example/public/seller/1/logo.jpg",
		"https://media.example.org.evil.example/a.jpg",
		"https://evil-media.example.org/a.jpg",
		// the origin's scheme is part of the match
		"http://media.example.org/public/seller/1/logo.jpg",
		// another media-looking host is not ours unless it is the configured one
		"https://test-media.example.com/public/seller/1/logo.jpg",
		"https://media.example.com/public/seller/1/logo.jpg",
	])("rejects %s with 400 and never fetches upstream", async (url) => {
		const fetchMock = stubImageFetch();
		const res = await GET(requestFor(url));
		expect(res.status).toBe(400);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("follows the configured origin instead of any built-in host", async () => {
		__setEnvForTest({ ...getEnv(), mediaBaseUrl: "https://cdn.example.net" });
		const fetchMock = stubImageFetch();
		const allowed = await GET(
			requestFor("https://cdn.example.net/public/seller/1/logo.jpg"),
		);
		expect(allowed.status).toBe(200);
		const denied = await GET(requestFor(`${MEDIA}/public/seller/1/logo.jpg`));
		expect(denied.status).toBe(400);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it("allows nothing when no media origin is configured", async () => {
		__setEnvForTest({ ...getEnv(), mediaBaseUrl: "" });
		const fetchMock = stubImageFetch();
		const res = await GET(requestFor(`${MEDIA}/public/seller/1/logo.jpg`));
		expect(res.status).toBe(400);
		expect(fetchMock).not.toHaveBeenCalled();
	});
});
