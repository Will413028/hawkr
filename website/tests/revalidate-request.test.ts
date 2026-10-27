import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
	applyRevalidation,
	DEFERRED_PROFILE,
	IMMEDIATE_PROFILE,
	MAX_REVALIDATION_TAGS,
	parseRevalidateRequest,
	revalidationProfileFor,
} from "../lib/api/revalidate-request.ts";
import { PUBLIC_CACHE_TAGS } from "../lib/seo/tags.ts";

const projectRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const ROUTE_PATH = "app/api/revalidate/route.ts";

const SECRET = "s3cr3t";

test("rejects a missing or wrong secret with 401", () => {
	assert.deepEqual(
		parseRevalidateRequest(null, { tags: ["product:1"] }, SECRET),
		{
			ok: false,
			status: 401,
			error: "unauthorized",
		},
	);
	assert.deepEqual(
		parseRevalidateRequest("nope", { tags: ["product:1"] }, SECRET),
		{ ok: false, status: 401, error: "unauthorized" },
	);
});

test("rejects an empty/invalid tag list with 400", () => {
	assert.deepEqual(parseRevalidateRequest(SECRET, { tags: [] }, SECRET), {
		ok: false,
		status: 400,
		error: "no tags",
	});
	assert.deepEqual(parseRevalidateRequest(SECRET, null, SECRET), {
		ok: false,
		status: 400,
		error: "no tags",
	});
	assert.deepEqual(parseRevalidateRequest(SECRET, { tags: [""] }, SECRET), {
		ok: false,
		status: 400,
		error: "invalid tag",
	});
});

test("accepts entity/domain tags, deduplicates and expands to embedding domains only", () => {
	assert.deepEqual(
		parseRevalidateRequest(
			SECRET,
			{
				tags: ["product:1", "seller:2", "products:list", "product:1"],
			},
			SECRET,
		),
		{
			ok: true,
			status: 200,
			tags: [
				"product:1",
				"seller:2",
				"products:list",
				"posts:list",
				"sellers:list",
			],
		},
	);
});

test("expands each entity to the domains that embed it, and nothing else", () => {
	const expand = (tag: string) => {
		const result = parseRevalidateRequest(SECRET, { tags: [tag] }, SECRET);
		assert.equal(result.ok, true);
		return result.ok ? result.tags : [];
	};
	assert.deepEqual(expand("post:9"), ["post:9", "posts:list"]);
	assert.deepEqual(expand("hashtag:9"), ["hashtag:9", "hashtags:list"]);
	assert.deepEqual(expand("category:9"), ["category:9", "categories"]);
	// Hashtags / categories embed no product or seller fields, so a
	// commerce mutation must not reach them.
	for (const tag of ["product:1", "seller:2"]) {
		const expanded = expand(tag);
		for (const absent of ["hashtags:list", "categories"]) {
			assert.ok(!expanded.includes(absent), `${tag} must not expand ${absent}`);
		}
	}
});

test("the tags no producer emits are rejected instead of silently no-oping", () => {
	// Accepting a tag no fetcher reads makes the endpoint answer 200
	// "invalidated" while doing nothing. A producer that starts emitting one
	// must get 400, not silence.
	//
	// `home` joined this list once prod's product-service actually ran a build
	// containing 72b2b9a7a — verified by the image sha the pod reports
	// (884c95a2f), not by the commit being on `main` and not by a deploy run
	// reporting success. Until that was true, removing it would have 400'd the
	// whole payload prod still sends and silently dropped the entity tags
	// travelling with it.
	for (const tag of [
		"home",
		"marketplace",
		"tab:home",
		"tab:feed",
		"tab:market",
		"tab:search",
	]) {
		const result = parseRevalidateRequest(SECRET, { tags: [tag] }, SECRET);
		assert.deepEqual(
			result,
			{ ok: false, status: 400, error: "invalid tag" },
			`${tag} must no longer be accepted`,
		);
	}
});

test("rejects unknown, non-ASCII and oversized tag contracts", () => {
	for (const tag of [
		"account:1",
		"product:有毒",
		`product:${"a".repeat(129)}`,
		5,
	]) {
		assert.deepEqual(parseRevalidateRequest(SECRET, { tags: [tag] }, SECRET), {
			ok: false,
			status: 400,
			error: "invalid tag",
		});
	}
	assert.deepEqual(
		parseRevalidateRequest(
			SECRET,
			{
				tags: Array.from(
					{ length: MAX_REVALIDATION_TAGS + 1 },
					(_, index) => `product:${index}`,
				),
			},
			SECRET,
		),
		{ ok: false, status: 400, error: "too many tags" },
	);
});

test("ignores a legacy mode field so in-flight payloads keep working", () => {
	// Every value has to pass: 400-ing an unknown mode would reject the whole
	// payload and the valid tags with it, so the invalidation would disappear
	// instead of merely ignoring a field nobody reads any more.
	for (const mode of ["soft", "hard", "instant", 5, null]) {
		assert.deepEqual(
			parseRevalidateRequest(
				SECRET,
				{
					tags: ["product:1", "products:list"],
					mode,
					event_id: "inventory-1-revision-9",
				},
				SECRET,
			),
			{
				ok: true,
				status: 200,
				tags: ["product:1", "products:list", "posts:list"],
				eventId: "inventory-1-revision-9",
			},
		);
	}
});

test("rejects event-id aliases that disagree", () => {
	assert.deepEqual(
		parseRevalidateRequest(
			SECRET,
			{
				tags: ["product:1"],
				eventId: "event-a",
				event_id: "event-b",
			},
			SECRET,
		),
		{ ok: false, status: 400, error: "invalid event id" },
	);
});

test("expires an entity on write instead of opening a stale window", () => {
	// `{expire: 0}` writes the tag row with expire=now, which OpenNext reads as
	// "every entry generated before this bust is a miss". `"max"` writes
	// expire=now+1y and only marks the entry stale — the next reader gets the
	// pre-mutation copy, and for a single record that reader is usually the
	// seller who just saved.
	assert.deepEqual(IMMEDIATE_PROFILE, { expire: 0 });
	assert.equal(DEFERRED_PROFILE, "max");
});

test("expires entity tags immediately and leaves aggregates stale-servable", () => {
	// The split is the whole point: a hard expiry on the lists would bypass
	// OpenNext's regeneration dedupe (it only runs on the STALE path) and drop
	// stale-if-error, and one edit expands into three list tags — the #338
	// amplification. Their readers are not the writer.
	for (const tag of [
		"product:1",
		"seller:2",
		"post:3",
		"hashtag:5",
		"category:6",
	]) {
		assert.deepEqual(
			revalidationProfileFor(tag),
			IMMEDIATE_PROFILE,
			`${tag} must expire immediately`,
		);
	}
	for (const tag of Object.values(PUBLIC_CACHE_TAGS)) {
		assert.equal(
			revalidationProfileFor(tag),
			DEFERRED_PROFILE,
			`${tag} must stay stale-servable`,
		);
	}
});

test("routes every accepted tag through the shape-derived profile", () => {
	const entityTags = ["product:1", "seller:2"];
	const aggregateTags = [...Object.values(PUBLIC_CACHE_TAGS)];
	const tags = [...entityTags, ...aggregateTags];
	const calls: Array<[string, unknown]> = [];
	applyRevalidation(tags, (tag, profile) => {
		calls.push([tag, profile]);
	});

	assert.deepEqual(
		calls.map(([tag]) => tag),
		tags,
		"every tag must be invalidated, in order",
	);
	for (const [tag, profile] of calls) {
		assert.deepEqual(
			profile,
			revalidationProfileFor(tag),
			`${tag} routed to the wrong profile`,
		);
	}
});

/**
 * The profile split only exists where `revalidateTag` is actually called. Unit
 * tests on the helper cannot see a route that stops calling it, and an
 * independent review proved the point: swapping the route back to a bare
 * `revalidateTag(tag, "max")` loop — reinstating the exact stale-window bug
 * this split removes — left the whole suite green. Source-text assertions on
 * the single call site are the repo's existing answer to that (see
 * `tests/seller-profile-contract.test.ts`).
 */
test("the revalidate route delegates to the shape-derived profile mapping", () => {
	const route = readFileSync(join(projectRoot, ROUTE_PATH), "utf8");

	assert.match(
		route,
		/applyRevalidation\(\s*result\.tags,\s*revalidateTag,?\s*\)/,
		`${ROUTE_PATH} must invalidate through applyRevalidation, or the ` +
			"entity/aggregate split silently stops applying",
	);
	assert.doesNotMatch(
		route,
		/revalidateTag\(\s*tag/,
		`${ROUTE_PATH} must not call revalidateTag per tag with its own profile ` +
			"— that is how the stale-window bug was originally written",
	);
	assert.doesNotMatch(
		route,
		/["']max["']/,
		`${ROUTE_PATH} must not name a profile literal; the profile belongs to ` +
			"revalidationProfileFor so one tag shape cannot be special-cased here",
	);
});

test("treats a missing configured secret as misconfiguration (401, never open)", () => {
	assert.deepEqual(
		parseRevalidateRequest("anything", { tags: ["product:1"] }, undefined),
		{ ok: false, status: 401, error: "unauthorized" },
	);
});
