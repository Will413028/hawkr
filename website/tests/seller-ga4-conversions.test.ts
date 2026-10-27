import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { afterEach, test } from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import { AgreementKind } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { submitSellerLeadRequest } from "../features/seller/pricing/lead-submission";
import {
	trackSellerLead,
	trackSellerOnboardingCompleted,
} from "../lib/analytics/events";
import { __setEnvForTest, getEnv } from "../lib/config/env";
import { __setFlavorForTest } from "../lib/config/flavor";
import { finalizeSellerOnboarding } from "../lib/seller/onboarding";
import type { ApplyStoreInput } from "../lib/seller/repo";

const ONBOARDING_INPUT: ApplyStoreInput = {
	storeName: "測試商店",
	contactPhone: "0912345678",
	acceptedAgreements: [
		{ kind: AgreementKind.SELLER_TERMS, version: "2026-10-16" },
	],
};

afterEach(() => {
	__setEnvForTest(null);
	__setFlavorForTest(null);
	delete process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID;
	delete (globalThis as Record<string, unknown>).window;
});

test("accepted seller lead exposes only non-PII analytics context", async () => {
	let tracked: unknown;
	await submitSellerLeadRequest(
		{
			brandName: "不應進 GA 的品牌",
			contactName: "王小明",
			phone: "0912345678",
			email: "private@example.com",
			productType: "私房食品",
			plan: "starter",
			billingCycle: "monthly",
			serviceInterest: "managed",
		},
		{
			post: async () => ({ ok: true, json: async () => ({ ok: true }) }),
			onAccepted: (context) => {
				tracked = context;
			},
		},
	);

	assert.deepEqual(tracked, {
		plan: "starter",
		billingCycle: "monthly",
		serviceInterest: "managed",
	});
	assert.doesNotMatch(
		JSON.stringify(tracked),
		/不應進 GA 的品牌|王小明|0912345678|private@example\.com|私房食品/,
	);
});

test("accepted seller lead stays successful when analytics throws", async () => {
	await assert.doesNotReject(
		submitSellerLeadRequest(
			{
				brandName: "已收件品牌",
				contactName: "聯絡人",
				phone: "0912345678",
				email: "",
				productType: "食品",
				plan: "starter",
				billingCycle: "monthly",
				serviceInterest: "none",
			},
			{
				post: async () => ({ ok: true, json: async () => ({ ok: true }) }),
				onAccepted: () => {
					throw new Error("analytics unavailable");
				},
			},
		),
	);
});

for (const [label, response] of [
	["HTTP rejection", { ok: false, json: async () => ({ ok: true }) }],
	["body rejection", { ok: true, json: async () => ({ ok: false }) }],
	[
		"invalid response body",
		{
			ok: true,
			json: async () => {
				throw new Error("invalid JSON");
			},
		},
	],
] as const) {
	test(`${label} does not emit a seller lead conversion`, async () => {
		let acceptedCount = 0;
		await assert.rejects(
			submitSellerLeadRequest(
				{
					brandName: "品牌",
					contactName: "聯絡人",
					phone: "0912345678",
					email: "",
					productType: "食品",
					plan: "undecided",
					billingCycle: "annual",
					serviceInterest: "none",
				},
				{
					post: async () => response,
					onAccepted: () => {
						acceptedCount += 1;
					},
				},
			),
			/seller lead was not accepted/,
		);
		assert.equal(acceptedCount, 0);
	});
}

test("seller onboarding callback runs only after persistence and completion", async () => {
	const order: string[] = [];
	await finalizeSellerOnboarding(ONBOARDING_INPUT, {
		fetchStore: async () => {
			order.push("lookup");
			throw new ConnectError("not found", Code.NotFound);
		},
		applyStore: async () => {
			order.push("persist");
			return { etag: "e1" } as never;
		},
		updateStore: async () => {
			throw new Error("unexpected update");
		},
		complete: async () => {
			order.push("complete");
		},
		onCompleted: (result) => {
			order.push(`track:${result}`);
		},
	});

	assert.deepEqual(order, ["lookup", "persist", "complete", "track:created"]);
});

test("seller onboarding callback is skipped when persistence fails", async () => {
	let completed = false;
	let tracked = false;
	await assert.rejects(
		finalizeSellerOnboarding(ONBOARDING_INPUT, {
			fetchStore: async () => {
				throw new ConnectError("not found", Code.NotFound);
			},
			applyStore: async () => {
				throw new Error("persist failed");
			},
			updateStore: async () => ({}),
			complete: async () => {
				completed = true;
			},
			onCompleted: () => {
				tracked = true;
			},
		}),
		/persist failed/,
	);
	assert.equal(completed, false);
	assert.equal(tracked, false);
});

test("seller onboarding callback is skipped when completion fails", async () => {
	let tracked = false;
	await assert.rejects(
		finalizeSellerOnboarding(ONBOARDING_INPUT, {
			fetchStore: async () => {
				throw new ConnectError("not found", Code.NotFound);
			},
			applyStore: async () => ({ etag: "e1" }) as never,
			updateStore: async () => ({}),
			complete: async () => {
				throw new Error("completion failed");
			},
			onCompleted: () => {
				tracked = true;
			},
		}),
		/completion failed/,
	);
	assert.equal(tracked, false);
});

test("completed seller onboarding stays successful when analytics throws", async () => {
	await assert.doesNotReject(
		finalizeSellerOnboarding(ONBOARDING_INPUT, {
			fetchStore: async () => {
				throw new ConnectError("not found", Code.NotFound);
			},
			applyStore: async () => ({ etag: "e1" }) as never,
			updateStore: async () => ({}),
			complete: async () => undefined,
			onCompleted: () => {
				throw new Error("analytics unavailable");
			},
		}),
	);
});

test("seller conversion helpers emit lead and onboarding events without PII", () => {
	const calls: unknown[][] = [];
	process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID = "G-TEST";
	__setEnvForTest(null);
	__setFlavorForTest("prod");
	getEnv();
	(globalThis as Record<string, unknown>).window = {
		gtag: (...args: unknown[]) => calls.push(args),
	};

	trackSellerLead({
		plan: "pro",
		billingCycle: "annual",
		serviceInterest: "managed",
	});
	trackSellerOnboardingCompleted("updated");

	assert.deepEqual(calls, [
		[
			"event",
			"generate_lead",
			{
				lead_source: "seller_pricing",
				plan: "pro",
				billing_cycle: "annual",
				service_interest: "managed",
			},
		],
		[
			"event",
			"seller_onboarding_completed",
			{
				completion_source: "seller_self_service",
				seller_profile_action: "updated",
			},
		],
	]);
	for (const forbiddenKey of [
		"brandName",
		"contactName",
		"phone",
		"email",
		"productType",
	]) {
		assert.doesNotMatch(JSON.stringify(calls), new RegExp(forbiddenKey));
	}
});

test("seller pricing analytics never attach the raw browser referrer", () => {
	const page = readFileSync(
		new URL(
			"../features/seller/pricing/SellerPricingPage.tsx",
			import.meta.url,
		),
		"utf8",
	);
	assert.doesNotMatch(page, /document\.referrer/);
	assert.doesNotMatch(page, /\breferrer\s*:/);
});
