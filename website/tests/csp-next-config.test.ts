import assert from "node:assert/strict";
import { test } from "node:test";

// next.config.ts reads its env when it is evaluated, so each case loads it under its own env.
(process.env as Record<string, string>).NODE_ENV = "production";
process.env.NEXT_PUBLIC_MEDIA_BASE_URL = "https://media.example.org";
process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.org/api/v1";

async function cspHeader(uploadOrigins: string | undefined): Promise<string> {
	if (uploadOrigins === undefined) delete process.env.MEDIA_UPLOAD_ORIGINS;
	else process.env.MEDIA_UPLOAD_ORIGINS = uploadOrigins;
	const { default: nextConfig } = await import(
		`../next.config.ts?upload=${encodeURIComponent(uploadOrigins ?? "unset")}`
	);
	const groups = await nextConfig.headers?.();
	const header = groups
		?.flatMap(
			(group: { headers: { key: string; value: string }[] }) => group.headers,
		)
		.find(
			(entry: { key: string }) =>
				entry.key === "Content-Security-Policy-Report-Only" ||
				entry.key === "Content-Security-Policy",
		);
	assert.ok(header, "next.config.ts emits a CSP header");
	return header.value;
}

function connectSrc(policy: string): string[] {
	const entry = policy.split("; ").find((e) => e.startsWith("connect-src "));
	assert.ok(entry);
	return entry.split(" ");
}

test("next.config wires the configured upload, API and media origins into connect-src", async () => {
	const connect = connectSrc(await cspHeader("https://*.uploads.example.org"));
	assert.ok(connect.includes("https://*.uploads.example.org"));
	assert.ok(connect.includes("https://media.example.org"));
	assert.ok(connect.includes("https://api.example.org"));
	assert.ok(connect.includes("wss://api.example.org"));
});

test("next.config without MEDIA_UPLOAD_ORIGINS leaves no upload origin and no widening", async () => {
	const policy = await cspHeader(undefined);
	assert.doesNotMatch(policy, /cloudflarestorage|uploads\.example/);
	for (const source of connectSrc(policy)) {
		assert.notEqual(source, "*");
		assert.doesNotMatch(source, /^https?:$/);
	}
});
