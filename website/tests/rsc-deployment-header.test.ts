import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { hermeticEnv } from "./helpers/hermetic-env";

type HeaderRule = {
	source: string;
	has?: Array<{ type: string; key: string; value?: string }>;
	missing?: Array<{ type: string; key: string; value?: string }>;
	headers: Array<{ key: string; value: string }>;
};

const require = createRequire(import.meta.url);
const {
	matchHas,
} = require("next/dist/shared/lib/router/utils/prepare-destination.js");
const {
	getPathMatch,
} = require("next/dist/shared/lib/router/utils/path-match.js");

function loadConfig(deploymentId?: string): {
	deploymentId?: string;
	rules: HeaderRule[];
} {
	const base = { ...process.env };
	delete base.DEPLOYMENT_ID;
	if (deploymentId) base.DEPLOYMENT_ID = deploymentId;
	// Import in a fresh process: deploymentId and headers must use the same
	// build-time value, independent of module caching or the runner's env.
	const child = spawnSync(
		process.execPath,
		[
			"--import",
			"tsx",
			"--input-type=module",
			"-e",
			'import config from "./next.config.ts"; process.stdout.write(JSON.stringify({deploymentId:config.deploymentId,rules:await config.headers()}));',
		],
		{
			cwd: fileURLToPath(new URL("../", import.meta.url)),
			env: hermeticEnv({}, base),
			encoding: "utf8",
			timeout: 30_000,
		},
	);
	assert.equal(child.status, 0, child.stderr || child.error?.message);
	return JSON.parse(child.stdout);
}

function navigationHeader(
	rules: HeaderRule[],
	pathname: string,
	headers: Record<string, string>,
) {
	let value: string | undefined;
	for (const rule of rules) {
		if (!getPathMatch(rule.source)(pathname)) continue;
		if (matchHas({ headers }, {}, rule.has, rule.missing) === false) continue;
		value =
			rule.headers.find((header) => header.key === "x-nextjs-deployment-id")
				?.value ?? value;
	}
	return value;
}

test("cached RSC navigation identifies the serving deployment, including for stale clients", () => {
	const servingId = "serving-build-123-2";
	const { deploymentId, rules } = loadConfig(servingId);
	assert.equal(deploymentId, servingId);

	for (const pathname of [
		"/",
		"/feed",
		"/market/products",
		"/search",
		"/products/123",
		"/account/orders",
	]) {
		for (const clientId of [servingId, "older-client-build"]) {
			for (const extra of [{}, { "next-router-prefetch": "1" }]) {
				assert.equal(
					navigationHeader(rules, pathname, {
						rsc: "1",
						"x-deployment-id": clientId,
						...extra,
					}),
					servingId,
					`${pathname}: the response must identify its build, never echo the client`,
				);
			}
		}
	}
	for (const headers of [{}, { accept: "text/html" }, { rsc: "2" }]) {
		assert.equal(navigationHeader(rules, "/feed", headers), undefined);
	}
});

test("local builds without a deployment id retain Next's own build-id handling", () => {
	const { deploymentId, rules } = loadConfig();
	assert.equal(deploymentId, undefined);
	assert.equal(navigationHeader(rules, "/feed", { rsc: "1" }), undefined);
});
