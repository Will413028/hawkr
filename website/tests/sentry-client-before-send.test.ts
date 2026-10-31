import assert from "node:assert/strict";
import { test } from "node:test";
import { clientBeforeSend } from "../lib/sentry/client-before-send";

test("reports events from every host, including *.vercel.app", () => {
	for (const hostname of ["example.com", "hawkr.vercel.app", "localhost"]) {
		const event = clientBeforeSend({
			request: {
				url: `https://${hostname}/feed`,
				cookies: { session: "secret" },
				headers: { authorization: "Bearer token", "x-request-id": "req_1" },
			},
			user: { email: "buyer@example.com", id: "42" },
		});

		assert.equal(event.request?.url, `https://${hostname}/feed`);
		assert.equal(event.request?.headers?.["x-request-id"], "req_1");
		assert.equal(event.request?.cookies, undefined, hostname);
		assert.equal(event.request?.headers?.authorization, undefined, hostname);
		assert.equal(event.user?.email, undefined, hostname);
		assert.equal(event.user?.id, "42", hostname);
	}
});
