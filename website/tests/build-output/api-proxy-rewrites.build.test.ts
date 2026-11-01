import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * The same-origin API proxy of a deployed build, read from the routes manifest `next build` wrote
 * (`pnpm run test:build-output`, run after the build). The build must be a non-local-proxy build with
 * `API_BASE_URL` set (website CI pins it to http://127.0.0.1:9/api/v1).
 */
const MANIFEST = fileURLToPath(
	new URL("../../.next/routes-manifest.json", import.meta.url),
);

interface Rewrite {
	source: string;
	destination: string;
}

function beforeFiles(): Rewrite[] {
	assert.ok(
		existsSync(MANIFEST),
		"routes-manifest.json is not in .next: run `pnpm run build` first",
	);
	const manifest = JSON.parse(readFileSync(MANIFEST, "utf8")) as {
		rewrites: Rewrite[] | { beforeFiles: Rewrite[] };
	};
	assert.ok(
		!Array.isArray(manifest.rewrites),
		"rewrites has no beforeFiles phase",
	);
	return manifest.rewrites.beforeFiles;
}

const api = new URL(process.env.API_BASE_URL ?? "");

for (const [source, destination] of [
	["/api/connect/:path*", `${api.origin}/:path*`],
	["/api/auth/:path*", `${api.origin}/api/auth/:path*`],
	["/api/v1/:path*", `${api.origin}${api.pathname.replace(/\/+$/, "")}/:path*`],
] as const) {
	test(`beforeFiles forwards ${source} to the monolith`, () => {
		const rewrite = beforeFiles().find((r) => r.source === source);
		assert.ok(rewrite, `${source} is not a beforeFiles rewrite`);
		assert.equal(rewrite.destination, destination);
	});
}

test("beforeFiles has exactly the three API prefixes among its /api rewrites", () => {
	assert.deepEqual(
		beforeFiles()
			.map((r) => r.source)
			.filter((source) => source.startsWith("/api/"))
			.sort(),
		["/api/auth/:path*", "/api/connect/:path*", "/api/v1/:path*"],
	);
});
