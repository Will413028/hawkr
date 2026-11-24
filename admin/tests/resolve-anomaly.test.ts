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
	PaymentAnomalyResolutionReason,
	PaymentRefundStatus,
	PaymentReversalStatus,
	PaymentStatus,
	PaymentTopupStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	EMPTY_RESOLVE_DRAFT,
	NOTE_MAX_RUNES,
	paymentResolveTargets,
	RESOLVE_INVALID_TEXT,
	reasonOptions,
	reasonText,
	refundResolveTargets,
	resolveAnomalyRequest,
	transferResolveTargets,
	validateResolveDraft,
} from "../src/app/admin/anomalies/resolve-anomaly";

const R = PaymentAnomalyResolutionReason;
const OK = { reason: R.OTHER, note: "checked at provider" } as const;

test("an outcome is required", () => {
	assert.equal(validateResolveDraft(EMPTY_RESOLVE_DRAFT), "outcome");
	assert.match(RESOLVE_INVALID_TEXT.outcome, /核對的結果/);
});

test("succeeded needs a non-blank evidence id; failed does not", () => {
	const succeeded = { outcome: "succeeded", evidence: "  ", ...OK } as const;
	assert.equal(validateResolveDraft(succeeded), "evidence");
	assert.match(RESOLVE_INVALID_TEXT.evidence, /物件 ID/);
	assert.equal(validateResolveDraft({ ...succeeded, evidence: "tr_1" }), null);
	assert.equal(
		validateResolveDraft({ outcome: "failed", evidence: "", ...OK }),
		null,
	);
});

test("the request carries kind, id, outcome and the trimmed evidence", () => {
	assert.deepEqual(
		resolveAnomalyRequest(
			{ slot: "transfer", id: "t1" },
			{
				outcome: "succeeded",
				evidence: " tr_9 ",
				reason: R.DUPLICATE,
				note: "  dup  ",
			},
		),
		{
			kind: PaymentAnomalyKind.TRANSFER,
			id: "t1",
			outcome: PaymentAnomalyOutcome.SUCCEEDED,
			evidence: "tr_9",
			reason: R.DUPLICATE,
			note: "dup",
		},
	);
});

test("a failed resolution never sends leftover evidence", () => {
	const req = resolveAnomalyRequest(
		{ slot: "payment", id: "p1" },
		{ outcome: "failed", evidence: "pi_typed_before", ...OK },
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
				{ outcome: "failed", evidence: "", ...OK },
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

test("a reason compatible with the outcome is required", () => {
	const base = { outcome: "succeeded", evidence: "tr_1", note: "n" } as const;
	assert.equal(
		validateResolveDraft({ ...base, reason: R.UNSPECIFIED }),
		"reason",
	);
	assert.equal(
		validateResolveDraft({ ...base, reason: R.PROVIDER_NOT_EXECUTED }),
		"reason",
	);
	assert.equal(
		validateResolveDraft({
			outcome: "failed",
			evidence: "",
			note: "n",
			reason: R.PROVIDER_EXECUTED_NOT_RECORDED,
		}),
		"reason",
	);
	assert.deepEqual(reasonOptions("succeeded"), [
		R.PROVIDER_EXECUTED_NOT_RECORDED,
		R.DUPLICATE,
		R.OTHER,
	]);
	assert.deepEqual(reasonOptions("failed"), [
		R.PROVIDER_NOT_EXECUTED,
		R.DUPLICATE,
		R.OTHER,
	]);
	assert.equal(reasonOptions("").length, 4);
});

test("reason labels come from the shared catalog, not UNKNOWN", () => {
	for (const reason of reasonOptions("")) {
		assert.notEqual(reasonText(reason), "未知");
	}
});

test("the note is required, measured after Go-style trim in runes, at most 255", () => {
	const draft = { outcome: "failed", evidence: "", reason: R.OTHER } as const;
	assert.equal(validateResolveDraft({ ...draft, note: " \u3000 " }), "note");
	assert.equal(
		validateResolveDraft({ ...draft, note: "字".repeat(NOTE_MAX_RUNES) }),
		null,
	);
	assert.equal(
		validateResolveDraft({ ...draft, note: "字".repeat(NOTE_MAX_RUNES + 1) }),
		"noteTooLong",
	);
	assert.equal(
		validateResolveDraft({
			...draft,
			note: ` ${"😀".repeat(NOTE_MAX_RUNES)} `,
		}),
		null,
	);
});
