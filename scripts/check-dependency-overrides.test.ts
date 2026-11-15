import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";

const script = join(import.meta.dirname, "check-dependency-overrides.ts");

function write(root: string, rel: string, content: unknown): void {
	const path = join(root, rel);
	mkdirSync(dirname(path), { recursive: true });
	writeFileSync(path, typeof content === "string" ? content : JSON.stringify(content, null, 2));
}

type Fixture = {
	overrides: Record<string, unknown>;
	notes: unknown;
	/** what "miniflare" declares for sharp in the lockfile */
	declared: string | null;
};

function run(f: Fixture): { code: number; out: string } {
	const root = mkdtempSync(join(tmpdir(), "override-gate-"));
	try {
		// Minimal pnpm side: one override that stays load-bearing, one allowBuilds entry in the tree.
		write(
			root,
			"pnpm-workspace.yaml",
			"overrides:\n  # REMOVE-WHEN: requesters-demand >=1.5.0\n  dep-a: ^1.5.0\nallowBuilds:\n  esbuild: true\n",
		);
		write(root, "node_modules/.pnpm/x@1/node_modules/x/package.json", {
			name: "x",
			version: "1.0.0",
			dependencies: { "dep-a": "^1.0.0" },
		});
		write(root, "node_modules/.pnpm/dep-a@2/node_modules/dep-a/package.json", { name: "dep-a", version: "2.0.0" });
		write(root, "node_modules/.pnpm/esbuild@1/node_modules/esbuild/package.json", { name: "esbuild", version: "1.0.0" });
		const dir = "backend/workers/image-gateway";
		write(root, `${dir}/package.json`, { name: "image-gateway", overrides: f.overrides, "//overrides": f.notes });
		write(root, `${dir}/package-lock.json`, {
			lockfileVersion: 3,
			packages: {
				"": {},
				"node_modules/miniflare": {
					version: "4.0.0",
					dependencies: f.declared === null ? {} : { sharp: f.declared },
				},
			},
		});
		const r = spawnSync("node", [script], { env: { ...process.env, CHECK_OVERRIDES_ROOT: root }, encoding: "utf8" });
		return { code: r.status ?? -1, out: r.stdout + r.stderr };
	} finally {
		rmSync(root, { recursive: true, force: true });
	}
}

const key = "miniflare > sharp";
const note = (s: string) => ({ [key]: s });
const overrides = { miniflare: { sharp: "0.35.5" } };
const good = "EXCEEDS-UPSTREAM: miniflare — pins exactly. REMOVE-WHEN: requesters-demand >=0.35.5";

test("npm override that is still load-bearing passes", () => {
	const r = run({ overrides, notes: note(good), declared: "0.34.5" });
	assert.equal(r.code, 0, r.out);
	assert.match(r.out, /1 npm override/);
});

test("npm override fails once the requester demands the REMOVE-WHEN version", () => {
	const r = run({ overrides, notes: note(good), declared: "0.35.5" });
	assert.equal(r.code, 1);
	assert.match(r.out, /miniflare > sharp[\s\S]*REMOVE-WHEN satisfied/);
});

test("npm override fails when the requester no longer declares the dependency", () => {
	const r = run({ overrides, notes: note(good), declared: null });
	assert.equal(r.code, 1);
	assert.match(r.out, /no longer present/);
});

test("npm override without REMOVE-WHEN fails", () => {
	const r = run({ overrides, notes: note("Security floor."), declared: "0.34.5" });
	assert.equal(r.code, 1);
	assert.match(r.out, /missing REMOVE-WHEN/);
});

test("npm override exceeding the declared range needs EXCEEDS-UPSTREAM", () => {
	const r = run({ overrides, notes: note("REMOVE-WHEN: requesters-demand >=0.35.5"), declared: "0.34.5" });
	assert.equal(r.code, 1);
	assert.match(r.out, /forces a version outside what these requesters declare/);
});

test("stale EXCEEDS-UPSTREAM marker fails", () => {
	const r = run({ overrides, notes: note("EXCEEDS-UPSTREAM: miniflare — x. REMOVE-WHEN: requesters-demand >=0.35.5"), declared: "^0.35.0" });
	assert.equal(r.code, 1);
	assert.match(r.out, /no longer\s+overshot/);
});

test("unscoped npm override fails", () => {
	const r = run({ overrides: { sharp: "0.35.5" }, notes: {}, declared: "0.34.5" });
	assert.equal(r.code, 1);
	assert.match(r.out, /must be scoped under the requester/);
});

test("open-ended npm override value fails", () => {
	const r = run({ overrides: { miniflare: { sharp: ">=0.35.5" } }, notes: note(good), declared: "0.34.5" });
	assert.equal(r.code, 1);
	assert.match(r.out, /open-ended/);
});
