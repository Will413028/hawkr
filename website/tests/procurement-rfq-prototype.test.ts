import assert from "node:assert/strict";
import test from "node:test";
import {
	EXTERNAL_VENDOR_ACCESS_STEPS,
	EXTERNAL_VENDOR_BUYER_OFFER,
	EXTERNAL_VENDOR_STEPS,
	EXTERNAL_VENDOR_TRANSACTION_STEPS,
	externalVendorStepPath,
	isExternalVendorStep,
	isRfqPrototypeScreenSlug,
	RFQ_PROTOTYPE_QUOTES,
	RFQ_PROTOTYPE_SCREENS,
	rfqPrototypeScreenPath,
} from "../features/procurement-rfq-prototype/data";
import { isStandaloneFlowPath } from "../lib/navigation/standalone-flow";

test("external vendor portal exposes independent access and transaction pages", () => {
	const accessSteps = EXTERNAL_VENDOR_ACCESS_STEPS.map((step) => step.slug);
	const transactionSteps = EXTERNAL_VENDOR_TRANSACTION_STEPS.map(
		(step) => step.slug,
	);
	const allSteps = EXTERNAL_VENDOR_STEPS.map((step) => step.slug);

	assert.deepEqual(accessSteps, [
		"invitation",
		"exchange",
		"sign-in",
		"verify-code",
		"session-expired",
		"invite-unavailable",
		"portal",
	]);
	assert.deepEqual(transactionSteps, [
		"business-verification",
		"rfq",
		"quote",
		"review",
		"transaction",
	]);
	assert.equal(allSteps.length, 12);
	assert.equal(new Set(allSteps).size, allSteps.length);
	assert.equal(
		externalVendorStepPath("invitation"),
		"/hawkr-agent-rfq-preview/external-vendor",
	);
	for (const step of allSteps.slice(1)) {
		assert.equal(
			externalVendorStepPath(step),
			`/hawkr-agent-rfq-preview/external-vendor/${step}`,
		);
		assert.equal(isExternalVendorStep(step), true);
	}
	assert.equal(isExternalVendorStep("paid-without-order"), false);
	assert.equal(
		isStandaloneFlowPath(externalVendorStepPath("invitation")),
		true,
	);
	assert.equal(
		isStandaloneFlowPath(externalVendorStepPath("transaction")),
		true,
	);
	assert.equal(isStandaloneFlowPath(externalVendorStepPath("portal")), true);
});

test("RFQ prototype exposes unique routable screens", () => {
	const slugs = RFQ_PROTOTYPE_SCREENS.map((screen) => screen.slug);
	assert.equal(new Set(slugs).size, slugs.length);
	assert.equal(slugs.length, 11);
	for (const slug of slugs) {
		assert.equal(isRfqPrototypeScreenSlug(slug), true);
		assert.equal(
			rfqPrototypeScreenPath(slug),
			`/hawkr-agent-rfq-preview/${slug}`,
		);
	}
	assert.equal(isRfqPrototypeScreenSlug("payment-is-live"), false);
});

test("Wave 1 stops at shortlist and later screens retain v5 target ids", () => {
	const waveOne = RFQ_PROTOTYPE_SCREENS.filter(
		(screen) => screen.wave === "wave-1",
	);
	assert.deepEqual(
		waveOne.map((screen) => screen.slug),
		[
			"candidate-handoff",
			"rfq-compose",
			"seller-inbox",
			"seller-response",
			"quote-compare",
		],
	);
	assert.equal(waveOne.at(-1)?.targetId, "B-02");

	const targetIds = RFQ_PROTOTYPE_SCREENS.flatMap((screen) =>
		screen.targetId ? [screen.targetId] : [],
	);
	assert.deepEqual(targetIds, [
		"B-02",
		"B-04/B-05",
		"B-06",
		"E-03",
		"S-01",
		"B-08",
		"F-01",
	]);
});

test("quote fixtures preserve immutable version and qualification signals", () => {
	assert.equal(RFQ_PROTOTYPE_QUOTES.length, 3);
	assert.ok(
		RFQ_PROTOTYPE_QUOTES.every((quote) => quote.version.startsWith("v")),
	);
	assert.ok(
		RFQ_PROTOTYPE_QUOTES.some((quote) =>
			quote.notes.some((note) => note.includes("尚不可接受")),
		),
	);
	assert.ok(RFQ_PROTOTYPE_QUOTES.every((quote) => quote.submittedAt));
});

test("buyer-confirmed offer preserves Pricebook bounds and retail formula", () => {
	const offer = EXTERNAL_VENDOR_BUYER_OFFER;

	assert.equal(offer.status, "buyer-confirmed");
	assert.ok(offer.pricebookVersion);
	assert.ok(offer.pricingRuleVersion);
	assert.ok(offer.pricebookFloor <= offer.buyerMinimumUnitPrice);
	assert.ok(offer.buyerMinimumUnitPrice <= offer.transactionUnitPrice);
	assert.ok(offer.transactionUnitPrice <= offer.buyerMaximumUnitPrice);
	assert.ok(offer.buyerMaximumUnitPrice <= offer.pricebookCeiling);
	assert.equal(
		offer.retailUnitPrice,
		offer.transactionUnitPrice * offer.retailMultiplier,
	);
	assert.equal(offer.subtotal, offer.quantity * offer.transactionUnitPrice);
	assert.equal(offer.total, offer.subtotal + offer.freight);
});
