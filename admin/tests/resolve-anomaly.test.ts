import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	PaymentRefundSchema,
	PaymentReversalSchema,
	PaymentSchema,
	PaymentTransferSchema,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentAnomalyKind,
	PaymentAnomalyOutcome,
	PaymentRefundStatus,
	PaymentReversalStatus,
	PaymentStatus,
	PaymentTopupStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	EMPTY_RESOLVE_DRAFT,
	paymentResolveTargets,
	RESOLVE_INVALID_TEXT,
	refundResolveTargets,
	resolveAnomalyRequest,
	transferResolveTargets,
	validateResolveDraft,
} from "../src/app/admin/anomalies/resolve-anomaly";

test("an outcome is required", () => {
	assert.equal(validateResolveDraft(EMPTY_RESOLVE_DRAFT), "outcome");
	assert.match(RESOLVE_INVALID_TEXT.outcome, /核對的結果/);
});

test("succeeded needs a non-blank evidence id; failed does not", () => {
	const succeeded = { outcome: "succeeded", evidence: "  " } as const;
	assert.equal(validateResolveDraft(succeeded), "evidence");
	assert.match(RESOLVE_INVALID_TEXT.evidence, /物件 ID/);
	assert.equal(validateResolveDraft({ ...succeeded, evidence: "tr_1" }), null);
	assert.equal(validateResolveDraft({ outcome: "failed", evidence: "" }), null);
});

test("the request carries kind, id, outcome and the trimmed evidence", () => {
	assert.deepEqual(
		resolveAnomalyRequest(
			{ slot: "transfer", id: "t1" },
			{ outcome: "succeeded", evidence: " tr_9 " },
		),
		{
			kind: PaymentAnomalyKind.TRANSFER,
			id: "t1",
			outcome: PaymentAnomalyOutcome.SUCCEEDED,
			evidence: "tr_9",
		},
	);
});

test("a failed resolution never sends leftover evidence", () => {
	const req = resolveAnomalyRequest(
		{ slot: "payment", id: "p1" },
		{ outcome: "failed", evidence: "pi_typed_before" },
	);
	assert.equal(req.evidence, "");
	assert.equal(req.outcome, PaymentAnomalyOutcome.FAILED);
	assert.equal(req.kind, PaymentAnomalyKind.PAYMENT);
});

test("every slot maps to its own proto kind", () => {
	const kinds = (
		["payment", "transfer", "refund", "reversal", "topup"] as const
	).map(
		(slot) =>
			resolveAnomalyRequest(
				{ slot, id: "x" },
				{ outcome: "failed", evidence: "" },
			).kind,
	);
	assert.deepEqual(kinds, [
		PaymentAnomalyKind.PAYMENT,
		PaymentAnomalyKind.TRANSFER,
		PaymentAnomalyKind.REFUND,
		PaymentAnomalyKind.REVERSAL,
		PaymentAnomalyKind.TOPUP,
	]);
});

test("only ANOMALY payments and transfers get a target", () => {
	assert.deepEqual(
		paymentResolveTargets(
			create(PaymentSchema, { id: "p1", status: PaymentStatus.ANOMALY }),
		),
		[{ slot: "payment", id: "p1" }],
	);
	assert.deepEqual(
		paymentResolveTargets(
			create(PaymentSchema, { id: "p2", status: PaymentStatus.SUCCEEDED }),
		),
		[],
	);
	assert.deepEqual(
		transferResolveTargets(
			create(PaymentTransferSchema, {
				id: "t1",
				status: PaymentTransferStatus.ANOMALY,
			}),
		),
		[{ slot: "transfer", id: "t1" }],
	);
	assert.deepEqual(
		transferResolveTargets(
			create(PaymentTransferSchema, {
				id: "t2",
				status: PaymentTransferStatus.FAILED,
			}),
		),
		[],
	);
});

test("a refund targets itself, each ANOMALY reversal row by its own id, and the top-up by the refund id", () => {
	const refund = create(PaymentRefundSchema, {
		id: "r1",
		status: PaymentRefundStatus.ANOMALY,
		topupStatus: PaymentTopupStatus.ANOMALY,
		reversals: [
			create(PaymentReversalSchema, {
				id: "v1",
				status: PaymentReversalStatus.REVERSED,
			}),
			create(PaymentReversalSchema, {
				id: "v2",
				status: PaymentReversalStatus.ANOMALY,
			}),
		],
	});
	assert.deepEqual(refundResolveTargets(refund), [
		{ slot: "refund", id: "r1" },
		{ slot: "reversal", id: "v2" },
		{ slot: "topup", id: "r1" },
	]);
	assert.deepEqual(
		refundResolveTargets(
			create(PaymentRefundSchema, {
				id: "r2",
				status: PaymentRefundStatus.SUCCEEDED,
			}),
		),
		[],
	);
});
