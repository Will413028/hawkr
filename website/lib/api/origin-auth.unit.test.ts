import { createHmac } from "node:crypto";
import { afterEach, describe, expect, it } from "vitest";
import { ORIGIN_AUTH_HEADER, signOriginAuth } from "./origin-auth.server";

/**
 * The wire contract with `gateway/internal/middleware/ratelimit_subject.go`.
 *
 * Everything here is asserted against an INDEPENDENT recomputation (node:crypto
 * rather than the WebCrypto path the module uses), because the failure mode this
 * guards is not "throws" — it is "produces a signature the gateway silently
 * rejects and fails open on", which no smoke test would notice.
 */
const DOMAIN = "hkorigin1:";
const SECRET = "unit-test-secret";

function decode(header: string) {
	const [body, mac] = header.split(".");
	const expected = createHmac("sha256", SECRET)
		.update(DOMAIN + body)
		.digest("base64url");
	return {
		body,
		mac,
		macMatches: mac === expected,
		payload: JSON.parse(Buffer.from(body, "base64url").toString("utf8")) as {
			v: number;
			p: string;
			ip?: string;
			ts: number;
			m: string;
			u: string;
		},
	};
}

afterEach(() => {
	// `delete`, not `= undefined`: assigning undefined stores the STRING
	// "undefined", which is truthy and would make the next case sign with a key
	// called "undefined" instead of not signing at all.
	delete process.env.ORIGIN_AUTH_SECRET;
	delete process.env.ORIGIN_AUTH_CI_SECRET;
});

describe("signOriginAuth (backend#2264 wire contract)", () => {
	it("does not sign when no key is present", async () => {
		expect(
			await signOriginAuth({
				method: "GET",
				url: "https://api/api/v1/public/x",
			}),
		).toBeNull();
	});

	it("signs SSR with a MAC the gateway's construction reproduces", async () => {
		process.env.ORIGIN_AUTH_SECRET = SECRET;
		const header = await signOriginAuth({
			method: "get",
			url: "https://test-api.example.com/api/v1/public/products?page=1",
		});
		expect(header).not.toBeNull();
		const { macMatches, payload, body, mac } = decode(header as string);

		expect(macMatches).toBe(true);
		expect(payload).toMatchObject({
			v: 1,
			p: "ssr",
			m: "GET",
			// Path only: the gateway compares `c.Request.URL.Path`, which carries no
			// query string.
			u: "/api/v1/public/products",
		});
		expect(payload.ip).toBeUndefined();
		expect(Math.abs(payload.ts - Math.floor(Date.now() / 1000))).toBeLessThan(
			5,
		);
		// Unpadded base64url on both halves — `base64.RawURLEncoding` rejects "=".
		expect(body).toMatch(/^[A-Za-z0-9_-]+$/);
		expect(mac).toMatch(/^[A-Za-z0-9_-]+$/);
	});

	it("carries the visitor IP as p=visitor", async () => {
		process.env.ORIGIN_AUTH_SECRET = SECRET;
		const header = await signOriginAuth({
			method: "GET",
			url: "https://test-api.example.com/api/v1/public/sellers/2685914682",
			visitorIp: " 203.0.113.7 ",
		});
		const { macMatches, payload } = decode(header as string);
		expect(macMatches).toBe(true);
		expect(payload.p).toBe("visitor");
		expect(payload.ip).toBe("203.0.113.7");
	});

	it("signs p=ci under the CI key and never claims a visitor", async () => {
		process.env.ORIGIN_AUTH_CI_SECRET = SECRET;
		process.env.ORIGIN_AUTH_SECRET = "the-wrong-key";
		const header = await signOriginAuth({
			method: "GET",
			url: "https://test-api.example.com/api/v1/public/products",
			// Even handed one — a runner has no real visitors, and the gateway would
			// not accept a p=visitor claim under the CI key anyway.
			visitorIp: "203.0.113.7",
		});
		const { macMatches, payload } = decode(header as string);
		expect(macMatches).toBe(true); // i.e. signed with the CI key, not the other
		expect(payload.p).toBe("ci");
		expect(payload.ip).toBeUndefined();
	});

	it("uses the header name the gateway reads", () => {
		expect(ORIGIN_AUTH_HEADER).toBe("X-HK-Origin-Auth");
	});
});
