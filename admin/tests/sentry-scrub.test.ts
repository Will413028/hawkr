import assert from "node:assert/strict";
import test from "node:test";
import { scrubSentryEvent } from "../src/lib/sentry/scrub";

test("removes request cookies", () => {
	const event = scrubSentryEvent({ request: { cookies: "a=b" } });
	assert.equal(event.request?.cookies, undefined);
});

test("deletes sensitive headers case-insensitively, keeps others", () => {
	const event = scrubSentryEvent({
		request: {
			headers: {
				Authorization: "Bearer x",
				"X-Operation-Session-Id": "operation-session-id",
				"X-Operation-Credential": "operation-credential",
				"X-HK-Operation-Session": "opaque-operation-secret",
				"X-HK-Operation-Credential": "another-secret",
				"content-type": "application/json",
			},
		},
	});
	assert.equal(event.request?.headers?.Authorization, undefined);
	assert.equal(event.request?.headers?.["X-Operation-Session-Id"], undefined);
	assert.equal(event.request?.headers?.["X-Operation-Credential"], undefined);
	assert.equal(event.request?.headers?.["X-HK-Operation-Session"], undefined);
	assert.equal(
		event.request?.headers?.["X-HK-Operation-Credential"],
		undefined,
	);
	assert.equal(event.request?.headers?.["content-type"], "application/json");
});

test("recursively filters sensitive body keys, keeps others", () => {
	const event = scrubSentryEvent({
		request: {
			data: {
				password: "secret",
				nested: {
					token: "t",
					operationCredential: "opaque-operation-secret",
					Operation_Session: "opaque-operation-session",
					operation_session_id: "operation-session-id",
					OPERATION_CREDENTIAL_ID: "operation-credential-id",
					name: "ok",
				},
				list: [{ access_token: "a" }],
			},
		},
	});
	const data = event.request?.data as Record<string, unknown>;
	const nested = data.nested as Record<string, unknown>;
	const first = (data.list as unknown[])[0] as Record<string, unknown>;
	assert.equal(data.password, "[Filtered]");
	assert.equal(nested.token, "[Filtered]");
	assert.equal(nested.operationCredential, "[Filtered]");
	assert.equal(nested.Operation_Session, "[Filtered]");
	assert.equal(nested.operation_session_id, "[Filtered]");
	assert.equal(nested.OPERATION_CREDENTIAL_ID, "[Filtered]");
	assert.equal(nested.name, "ok");
	assert.equal(first.access_token, "[Filtered]");
});

test("filters raw request bodies instead of retaining unparsed credentials", () => {
	const event = scrubSentryEvent({
		request: {
			data: '{"operationCredential":"opaque-operation-secret"}',
		},
	});
	assert.equal(event.request?.data, "[Filtered]");
});

test("strips user PII but keeps non-PII", () => {
	const event = scrubSentryEvent({
		user: { email: "a@b.c", ip_address: "1.2.3.4", id: "u1" },
	});
	assert.equal(event.user?.email, undefined);
	assert.equal(event.user?.ip_address, undefined);
	assert.equal(event.user?.id, "u1");
});
