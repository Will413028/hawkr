import assert from "node:assert/strict";
import test from "node:test";
import { tagBackendTrace } from "../src/lib/sentry/trace-tag";
import { ApiClientError } from "../src/services/api-error";

test("tags backend_trace_id from an ApiClientError with traceId", () => {
	const err = new ApiClientError("boom", 5000, "abc123");
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: err,
	});
	assert.equal(event.tags?.backend_trace_id, "abc123");
});

test("preserves existing tags", () => {
	const err = new ApiClientError("boom", 5000, "abc123");
	const event = tagBackendTrace(
		{ tags: { foo: "bar" } },
		{ originalException: err },
	);
	assert.equal(event.tags?.foo, "bar");
	assert.equal(event.tags?.backend_trace_id, "abc123");
});

test("no tag when error is not an ApiClientError", () => {
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: new Error("plain"),
	});
	assert.equal(event.tags, undefined);
});

test("no tag when ApiClientError has no traceId", () => {
	const err = new ApiClientError("boom", 5000);
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> }, {
		originalException: err,
	});
	assert.equal(event.tags?.backend_trace_id, undefined);
});

test("no tag when hint is absent", () => {
	const event = tagBackendTrace({} as { tags?: Record<string, unknown> });
	assert.equal(event.tags, undefined);
});
