import assert from "node:assert/strict";
import test from "node:test";
import {
	canRenderDelegatedWorkspace,
	deriveDelegatedOperationState,
	formatRemainingDuration,
	isSameDelegatedOperationSession,
	parseDelegatedOperationPreflight,
	parseDelegatedOperationSessionSummary,
	parseStartDelegatedOperationRequest,
} from "../src/features/delegated-operations/model";
import {
	DELEGATED_OPERATION_PROHIBITED_AREAS,
	KNOWN_DELEGATED_OPERATION_CAPABILITIES,
} from "../src/features/delegated-operations/policy";

// This is the proposed production backend contract. It is contract evidence,
// not a development fixture and never makes the runtime report fake success.
const summary = {
	target: {
		storeId: "store_123",
		sellerAccountId: "account_456",
		storeName: "範例好物",
	},
	actor: { adminId: "admin_789", displayName: "ADM 王小明" },
	reason: { code: "seller-onboarding", label: "協助上架" },
	caseReference: "CRM-2026-0188",
	capabilities: ["products.read", "products.write", "orders.read"],
	issuedAt: "2026-08-06T04:05:00.000Z",
	expiresAt: "2026-08-06T04:15:00.000Z",
	status: "active",
	revision: 3,
	operationCredential: "must-never-cross-the-browser-boundary",
};

test("capability and prohibited-area policies are closed enumerations", () => {
	assert.deepEqual(KNOWN_DELEGATED_OPERATION_CAPABILITIES, [
		"storefront.read",
		"storefront.write",
		"products.read",
		"products.write",
		"products.publish",
		"posts.read",
		"posts.write",
		"posts.publish",
		"orders.read",
	]);
	assert.deepEqual(DELEGATED_OPERATION_PROHIBITED_AREAS, [
		"fulfillment",
		"shipment",
		"cancel",
		"refund",
		"payment",
		"finance",
		"identity",
		"certification",
		"account-security",
	]);
});

test("safe summary parser constructs a credential-free projection", () => {
	const parsed = parseDelegatedOperationSessionSummary(summary);
	assert.ok(parsed);
	assert.equal(parsed.target.storeId, "store_123");
	assert.equal("operationCredential" in parsed, false);
	assert.equal("token" in parsed, false);
});

test("non-string IDs fail closed", () => {
	assert.equal(
		parseDelegatedOperationSessionSummary({
			...summary,
			target: { ...summary.target, storeId: 123 },
		}),
		null,
	);
	assert.equal(
		parseStartDelegatedOperationRequest({
			storeId: 123,
			reasonCode: "seller-onboarding",
			caseReference: null,
			capabilities: ["products.read"],
			ttlSeconds: 600,
		}),
		null,
	);
});

test("unknown, duplicate, and dependency-less capabilities are dropped one by one", () => {
	const grantedOf = (capabilities: unknown) =>
		parseDelegatedOperationSessionSummary({ ...summary, capabilities })
			?.capabilities ?? null;

	// One unusable entry must not void the rest of the grant: the backend CHECK
	// closes the value domain but never enforces the dependency closure.
	assert.deepEqual(grantedOf(["products.read", "shipment.write"]), [
		"products.read",
	]);
	assert.deepEqual(grantedOf(["products.read", "products.publish"]), [
		"products.read",
	]);
	assert.deepEqual(grantedOf(["posts.read", "posts.publish", "orders.read"]), [
		"posts.read",
		"orders.read",
	]);
	assert.deepEqual(grantedOf(["products.read", "products.read"]), [
		"products.read",
	]);
	assert.deepEqual(
		grantedOf(["products.read", "products.write", "products.publish"]),
		["products.read", "products.write", "products.publish"],
	);
});

test("a grant with nothing usable left still fails closed", () => {
	for (const capabilities of [
		[],
		["storefront.write"],
		["products.write"],
		["posts.write"],
		["products.bulk"],
		["audit.read"],
	]) {
		assert.equal(
			parseDelegatedOperationSessionSummary({ ...summary, capabilities }),
			null,
		);
	}
});

test("browser start request cannot choose capabilities, only a shorter TTL", () => {
	assert.deepEqual(
		parseStartDelegatedOperationRequest({
			storeId: "store_123",
			reasonCode: "seller-onboarding",
			caseReference: "CRM-2026-0188",
			capabilities: ["storefront.write"],
			ttlSeconds: 900,
		}),
		{
			storeId: "store_123",
			reasonCode: "seller-onboarding",
			caseReference: "CRM-2026-0188",
			ttlSeconds: 900,
		},
	);
	assert.deepEqual(
		parseStartDelegatedOperationRequest({
			storeId: "store_123",
			reasonCode: "seller-onboarding",
			caseReference: null,
		}),
		{
			storeId: "store_123",
			reasonCode: "seller-onboarding",
			caseReference: null,
		},
	);
	for (const ttlSeconds of [0, -1, 1.5, 1801, "900"]) {
		assert.equal(
			parseStartDelegatedOperationRequest({
				storeId: "store_123",
				reasonCode: "seller-onboarding",
				caseReference: null,
				ttlSeconds,
			}),
			null,
		);
	}
});

test("non-canonical, impossible, and missing expiry values fail closed", () => {
	for (const expiresAt of [
		"2026-08-06 04:15:00",
		"not-a-date",
		"2026-02-31T04:15:00Z",
		"2026-99-99T04:15:00Z",
		undefined,
	]) {
		assert.equal(
			parseDelegatedOperationSessionSummary({ ...summary, expiresAt }),
			null,
		);
	}
	assert.equal(
		parseDelegatedOperationSessionSummary({
			...summary,
			issuedAt: undefined,
		}),
		null,
	);
	// 1800s + 1ms: the session lifetime ceiling, not a formatting problem.
	assert.equal(
		parseDelegatedOperationSessionSummary({
			...summary,
			expiresAt: "2026-08-06T04:35:00.001Z",
		}),
		null,
	);
	assert.ok(
		parseDelegatedOperationSessionSummary({
			...summary,
			expiresAt: "2026-08-06T04:35:00.000Z",
		}),
	);
});

test("preflight validates target binding, reasons, known capabilities, 30m max TTL and idle window", () => {
	const preflight = {
		allowed: true,
		target: summary.target,
		actor: summary.actor,
		reasonOptions: [{ code: "seller-onboarding", label: "協助上架" }],
		capabilities: ["storefront.read", "products.read"],
		ttlSeconds: 1800,
		idleSeconds: 900,
		caseReferenceRequired: true,
	};
	assert.equal(parseDelegatedOperationPreflight(preflight)?.idleSeconds, 900);
	assert.equal(
		parseDelegatedOperationPreflight({ ...preflight, allowed: false }),
		null,
	);
	assert.equal(
		parseDelegatedOperationPreflight({ ...preflight, ttlSeconds: 1801 }),
		null,
	);
	for (const idleSeconds of [undefined, 0, 59, 1801, "900"]) {
		assert.equal(
			parseDelegatedOperationPreflight({ ...preflight, idleSeconds }),
			null,
		);
	}
	assert.equal(
		parseDelegatedOperationPreflight({
			...preflight,
			reasonOptions: [...preflight.reasonOptions, ...preflight.reasonOptions],
		}),
		null,
	);
});

test("create and current summaries must describe the exact same audit context", () => {
	const parsed = parseDelegatedOperationSessionSummary(summary);
	assert.ok(parsed);
	assert.equal(isSameDelegatedOperationSession(parsed, { ...parsed }), true);
	assert.equal(
		isSameDelegatedOperationSession(parsed, {
			...parsed,
			caseReference: "CRM-2026-OLD",
		}),
		false,
	);
	assert.equal(
		isSameDelegatedOperationSession(parsed, {
			...parsed,
			issuedAt: "2026-08-06T04:05:01.000Z",
		}),
		false,
	);
	assert.equal(
		isSameDelegatedOperationSession(parsed, {
			...parsed,
			capabilities: ["products.read", "orders.read"],
		}),
		false,
	);
});

test("state derivation blocks mismatch/revocation/expiry and warns for the final 5m", () => {
	const now = Date.parse("2026-08-06T04:10:00.000Z");
	const parsed = parseDelegatedOperationSessionSummary(summary);
	assert.ok(parsed);
	assert.equal(
		deriveDelegatedOperationState(parsed, "other", now),
		"target_mismatch",
	);
	assert.equal(
		deriveDelegatedOperationState(
			{ ...parsed, status: "revoked" },
			"store_123",
			now,
		),
		"revoked",
	);
	assert.equal(
		deriveDelegatedOperationState(
			{ ...parsed, expiresAt: "2026-08-06T04:09:59.000Z" },
			"store_123",
			now,
		),
		"expired",
	);
	assert.equal(
		deriveDelegatedOperationState(
			{ ...parsed, expiresAt: "2026-08-06T04:14:59.000Z" },
			"store_123",
			now,
		),
		"expiring",
	);
	assert.equal(
		deriveDelegatedOperationState(
			{ ...parsed, expiresAt: "2026-08-06T04:15:01.000Z" },
			"store_123",
			now,
		),
		"active",
	);
	// The fourth independent guard on the TTL ceiling: a full-length session
	// must derive as usable, not as "invalid".
	assert.equal(
		deriveDelegatedOperationState(
			{ ...parsed, expiresAt: "2026-08-06T04:35:00.000Z" },
			"store_123",
			now,
		),
		"active",
	);
	assert.equal(canRenderDelegatedWorkspace("expiring"), true);
	assert.equal(canRenderDelegatedWorkspace("expired"), false);
	assert.equal(
		formatRemainingDuration("2026-08-06T04:11:30.000Z", now),
		"01:30",
	);
});
