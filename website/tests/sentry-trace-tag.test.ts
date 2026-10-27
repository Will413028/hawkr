import assert from "node:assert/strict";
import { test } from "node:test";
import { ApiClientError } from "../lib/api/client";
import { tagBackendTrace } from "../lib/sentry/trace-tag";

test("tags backend_trace_id from an ApiClientError envelope", () => {
	const err = new ApiClientError("boom", 500, 5000, {
		code: 5000,
		traceId: "abc123",
	});
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: err,
	});
	assert.equal(event.tags?.backend_trace_id, "abc123");
});

test("preserves existing tags", () => {
	const err = new ApiClientError("boom", 500, 5000, {
		code: 5000,
		traceId: "abc123",
	});
	const event = tagBackendTrace(
		{ tags: { foo: "bar" } },
		{ originalException: err },
	);
	assert.equal(event.tags?.foo, "bar");
	assert.equal(event.tags?.backend_trace_id, "abc123");
});

test("tags API request context from a captured ApiClientError", () => {
	const err = new ApiClientError("down", 503, 503, {
		code: 503,
		traceId: "trace-503",
	});
	err.apiEndpoint = "/api/v1/app/media/upload_sessions";
	err.apiMethod = "POST";
	err.apiMode = "authed";
	err.apiRole = "2";
	err.cloudflareRayId = "a1feaf706bfbd6a9";
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: err,
	});
	assert.deepEqual(event.tags, {
		backend_trace_id: "trace-503",
		api_endpoint: "/api/v1/app/media/upload_sessions",
		api_method: "POST",
		api_mode: "authed",
		api_role: "2",
		cloudflare_ray_id: "a1feaf706bfbd6a9",
		http_status: "503",
	});
});

test("no tag when error is not an ApiClientError", () => {
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: new Error("plain"),
	});
	assert.equal(event.tags, undefined);
});

test("no tag when envelope has no traceId", () => {
	const err = new ApiClientError("boom", 500, 5000, { code: 5000 });
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: err,
	});
	assert.equal(event.tags?.backend_trace_id, undefined);
	assert.equal(event.tags?.http_status, "500");
});

test("no tag when hint is absent", () => {
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> });
	assert.equal(event.tags, undefined);
});
