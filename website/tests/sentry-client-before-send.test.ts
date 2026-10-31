import assert from "node:assert/strict";
import { test } from "node:test";
import {
	clientBeforeSend,
	isPreviewDeploymentHost,
} from "../lib/sentry/client-before-send";

test("drops events from *.vercel.app preview hosts", () => {
	for (const hostname of ["hawkr.vercel.app", "Hawkr-git-x-team.VERCEL.app"]) {
		const event = { request: { url: `https://${hostname}/feed` } };
		assert.equal(clientBeforeSend(event, undefined, hostname), null, hostname);
	}
});

test("preview host check is a hostname suffix match", () => {
	assert.equal(isPreviewDeploymentHost("hawkr.vercel.app"), true);
	for (const hostname of [
		"vercel.app.example.com",
		"example.com",
		"notvercel.app",
		"localhost",
		"",
		null,
		undefined,
	]) {
		assert.equal(isPreviewDeploymentHost(hostname), false, String(hostname));
	}
});

test("reports and scrubs events from non-preview hosts", () => {
	for (const hostname of [
		"example.com",
		"vercel.app.example.com",
		"localhost",
	]) {
		const event = clientBeforeSend(
			{
				request: {
					url: `https://${hostname}/feed`,
					cookies: { session: "secret" },
					headers: { authorization: "Bearer token", "x-request-id": "req_1" },
				},
				user: { email: "buyer@example.com", id: "42" },
			},
			undefined,
			hostname,
		);

		assert.ok(event, hostname);
		assert.equal(event.request?.url, `https://${hostname}/feed`);
		assert.equal(event.request?.headers?.["x-request-id"], "req_1");
		assert.equal(event.request?.cookies, undefined, hostname);
		assert.equal(event.request?.headers?.authorization, undefined, hostname);
		assert.equal(event.user?.email, undefined, hostname);
		assert.equal(event.user?.id, "42", hostname);
	}
});
