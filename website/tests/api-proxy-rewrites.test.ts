import assert from "node:assert/strict";
import test from "node:test";
import {
	apiProxyRewrites,
	apiProxyTarget,
	authProxyTarget,
	connectProxyTarget,
	isLocalApiProxyBuild,
} from "../lib/api/local-proxy";

const MONOLITH = "https://api.example.org/api/v1";

test("the rewrites forward the three browser prefixes to the monolith", () => {
	assert.deepEqual(
		apiProxyRewrites({ NODE_ENV: "production", API_BASE_URL: MONOLITH }),
		[
			{
				source: "/api/connect/:path*",
				destination: "https://api.example.org/:path*",
			},
			{
				source: "/api/auth/:path*",
				destination: "https://api.example.org/api/auth/:path*",
			},
			{
				source: "/api/v1/:path*",
				destination: "https://api.example.org/api/v1/:path*",
			},
		],
	);
});

test("the rewrites honour a base path and a trailing slash of API_BASE_URL", () => {
	const rewrites = apiProxyRewrites({
		NODE_ENV: "production",
		API_BASE_URL: "http://127.0.0.1:9/edge/api/v1/",
	});
	assert.deepEqual(
		rewrites.map((r) => r.destination),
		[
			"http://127.0.0.1:9/edge/:path*",
			"http://127.0.0.1:9/api/auth/:path*",
			"http://127.0.0.1:9/edge/api/v1/:path*",
		],
	);
});

test("a rewrite destination and the route handler target agree for every prefix", () => {
	const env = {
		NODE_ENV: "production",
		API_BASE_URL: "http://127.0.0.1:9/edge/api/v1",
	};
	const requests = [
		["/api/connect/hawkr.v1.Svc/Method", connectProxyTarget],
		["/api/auth/google/start", authProxyTarget],
		["/api/v1/public/products", apiProxyTarget],
	] as const;
	for (const [path, target] of requests) {
		const rewrite = apiProxyRewrites(env).find((r) =>
			path.startsWith(r.source.replace("/:path*", "/")),
		);
		assert.ok(rewrite, path);
		const rest = path.slice(rewrite.source.replace("/:path*", "").length);
		assert.equal(
			rewrite.destination.replace("/:path*", rest),
			target(`http://localhost:3000${path}`, env.API_BASE_URL),
			path,
		);
	}
});

test("the rewrites are off exactly where the local route handlers are the proxy", () => {
	assert.equal(isLocalApiProxyBuild({ NODE_ENV: "development" }), true);
	assert.equal(
		isLocalApiProxyBuild({
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
		}),
		true,
	);
	assert.equal(isLocalApiProxyBuild({ NODE_ENV: "production" }), false);
	assert.equal(
		isLocalApiProxyBuild({
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "0",
		}),
		false,
	);
	assert.deepEqual(apiProxyRewrites({ NODE_ENV: "development" }), []);
	assert.deepEqual(
		apiProxyRewrites({
			NODE_ENV: "production",
			PLAYWRIGHT_E2E_API_PROXY: "1",
			API_BASE_URL: "http://127.0.0.1:9/api/v1",
		}),
		[],
	);
});

test("an enabled build without a valid absolute API_BASE_URL fails", () => {
	for (const bad of [
		undefined,
		"",
		"  ",
		"/api/v1",
		"api.example.org/api/v1",
		"ftp://x/api/v1",
	]) {
		assert.throws(
			() => apiProxyRewrites({ NODE_ENV: "production", API_BASE_URL: bad }),
			/API_BASE_URL must be an absolute http\(s\) URL/,
			String(bad),
		);
	}
});
