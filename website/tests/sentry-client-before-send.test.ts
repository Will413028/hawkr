import assert from "node:assert/strict";
import { test } from "node:test";
import {
	clientBeforeSend,
	isPreviewDeploymentHost,
	parseSyntheticSmokeMarker,
	SYNTHETIC_SMOKE_SOURCE,
	tagSyntheticSmoke,
} from "../lib/sentry/client-before-send";

const PREVIEW_HOSTS = [
	"hawkr-website-prod.example.workers.dev",
	"hawkr-website-dev.example.workers.dev",
	"HAWKR-WEBSITE-PROD.example.workers.dev",
];

// 正式 / 真使用者會用的 origin：這些一定要繼續上報，否則等於把真缺陷靜音。
const REPORTING_HOSTS = [
	"example.com",
	"www.example.com",
	"web-dev.example.com",
	"localhost",
	// 後綴比對不可退化成 includes：這種網域不是 Cloudflare 預覽網域。
	"workers.dev.example.com",
];

test("drops events from *.workers.dev preview hosts", () => {
	for (const hostname of PREVIEW_HOSTS) {
		assert.equal(isPreviewDeploymentHost(hostname), true, hostname);
		assert.equal(
			clientBeforeSend(
				{ request: { url: `https://${hostname}/feed` } },
				{},
				hostname,
			),
			null,
			hostname,
		);
	}
});

test("still reports events from the production and dev domains", () => {
	for (const hostname of REPORTING_HOSTS) {
		assert.equal(isPreviewDeploymentHost(hostname), false, hostname);

		const event = clientBeforeSend(
			{
				request: {
					url: `https://${hostname}/feed`,
					cookies: { session: "secret" },
					headers: { authorization: "Bearer token", "x-request-id": "req_1" },
				},
				user: { email: "buyer@example.com", id: "42" },
			},
			{},
			hostname,
		);

		// 事件仍上報（非 null）……
		assert.notEqual(event, null, hostname);
		assert.equal(event?.request?.url, `https://${hostname}/feed`);
		assert.equal(event?.request?.headers?.["x-request-id"], "req_1");
		// ……且既有的去識別化沒有因為這條守門而被跳過。
		assert.equal(event?.request?.cookies, undefined, hostname);
		assert.equal(event?.request?.headers?.authorization, undefined, hostname);
		assert.equal(event?.user?.email, undefined, hostname);
		assert.equal(event?.user?.id, "42", hostname);
	}
});

test("fails open when the hostname is unavailable", () => {
	assert.equal(isPreviewDeploymentHost(undefined), false);
	assert.equal(isPreviewDeploymentHost(null), false);
	assert.notEqual(clientBeforeSend({ request: {} }, {}, undefined), null);
});

test("adds queryable low-cardinality tags and keeps the full smoke id in context", () => {
	const id =
		"cloudflare-post-deploy/d741da7f29b95e56cfbacf5beccccf2e50a7f454-31442580488-1";
	const event = clientBeforeSend(
		{
			tags: { existing: "kept" },
			contexts: { release: { channel: "dev" } },
		},
		{},
		"web-dev.example.com",
		{ source: SYNTHETIC_SMOKE_SOURCE, id },
	);

	assert.notEqual(event, null);
	assert.deepEqual(event?.tags, {
		existing: "kept",
		traffic_type: "synthetic",
		synthetic_source: "cloudflare-post-deploy",
	});
	assert.deepEqual(event?.contexts?.release, { channel: "dev" });
	assert.deepEqual(event?.contexts?.synthetic_smoke, {
		source: "cloudflare-post-deploy",
		id,
		deployment_id: "d741da7f29b95e56cfbacf5beccccf2e50a7f454-31442580488-1",
	});
	assert.equal(event?.tags?.synthetic_id, undefined);
});

test("leaves normal-user and invalid marker events untagged", () => {
	const invalidMarkers = [
		undefined,
		null,
		"cloudflare-post-deploy/local",
		{ source: "browser-user", id: "cloudflare-post-deploy/local" },
		{ source: SYNTHETIC_SMOKE_SOURCE, id: "other-source/local" },
		{ source: SYNTHETIC_SMOKE_SOURCE, id: "cloudflare-post-deploy/bad id" },
		{
			source: SYNTHETIC_SMOKE_SOURCE,
			id: `cloudflare-post-deploy/${"a".repeat(160)}`,
		},
	];

	for (const marker of invalidMarkers) {
		const event = tagSyntheticSmoke({ request: {} }, marker);
		assert.equal(event.tags, undefined);
		assert.equal(event.contexts, undefined);
	}
	assert.deepEqual(
		parseSyntheticSmokeMarker({
			source: SYNTHETIC_SMOKE_SOURCE,
			id: "cloudflare-post-deploy/local",
		}),
		{
			source: SYNTHETIC_SMOKE_SOURCE,
			id: "cloudflare-post-deploy/local",
		},
	);
});

test("a valid marker never bypasses the workers.dev noise guard", () => {
	assert.equal(
		clientBeforeSend(
			{ request: {} },
			{},
			"hawkr-website-dev.example.workers.dev",
			{
				source: SYNTHETIC_SMOKE_SOURCE,
				id: "cloudflare-post-deploy/local",
			},
		),
		null,
	);
});
