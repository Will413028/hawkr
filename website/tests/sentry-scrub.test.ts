import assert from "node:assert/strict";
import { test } from "node:test";
import { scrubSentryEvent } from "../lib/sentry/scrub";

test("scrubs auth headers cookies and user PII", () => {
	const event = scrubSentryEvent({
		request: {
			cookies: { session: "secret" },
			headers: {
				authorization: "Bearer token",
				Cookie: "session=secret",
				"x-request-id": "req_123",
			},
		},
		user: {
			email: "buyer@example.com",
			ip_address: "203.0.113.1",
			id: "42",
		},
	});

	assert.equal(event.request?.cookies, undefined);
	assert.equal(event.request?.headers?.authorization, undefined);
	assert.equal(event.request?.headers?.Cookie, undefined);
	assert.equal(event.request?.headers?.["x-request-id"], "req_123");
	assert.equal(event.user?.email, undefined);
	assert.equal(event.user?.ip_address, undefined);
	assert.equal(event.user?.id, "42");
});

test("scrubs nested auth, PII, and payment request data", () => {
	const event = scrubSentryEvent({
		request: {
			data: {
				access_token: "token",
				buyer: {
					email: "buyer@example.com",
					recipientName: "Chen",
					phone: "0923456789",
					shipping_address: "Store pickup address",
				},
				payment: {
					payuniTradeNo: "abc",
					cardNumber: "4111111111111111",
					cvv: "123",
				},
				items: [{ productName: "tomato", quantity: 1 }],
			},
		},
	});

	assert.deepEqual(event.request?.data, {
		access_token: "[Filtered]",
		buyer: {
			email: "[Filtered]",
			recipientName: "[Filtered]",
			phone: "[Filtered]",
			shipping_address: "[Filtered]",
		},
		payment: "[Filtered]",
		items: [{ productName: "tomato", quantity: 1 }],
	});
});
