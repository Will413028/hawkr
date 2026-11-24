import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	PaymentRefundSchema,
	PaymentSchema,
	PaymentTransferSchema,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentRefundStatus,
	PaymentReversalStatus,
	PaymentStatus,
	PaymentTopupStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	DEFAULT_PAYMENT_FILTERS,
	DEFAULT_REFUND_FILTERS,
	listPaymentRefundsRequest,
	listPaymentsRequest,
	listPaymentTransfersRequest,
	PAYMENT_STATUS_OPTIONS,
	paymentRow,
	refundRow,
	transferRow,
} from "../src/app/admin/anomalies/payment-lists";
import { currentToken, firstPage, nextPage } from "../src/lib/keyset-pages";

test("selecting ANOMALY sends that status; all sends none", () => {
	const anomaly = listPaymentsRequest(
		{ status: PaymentStatus.ANOMALY, orderNo: "" },
		"",
	);
	assert.equal(anomaly.status, PaymentStatus.ANOMALY);
	const all = listPaymentsRequest(
		{ status: PaymentStatus.UNSPECIFIED, orderNo: "O1" },
		"",
	);
	assert.equal(all.status, undefined);
	assert.equal(all.orderNo, "O1");
});

test("default payment filter is ANOMALY and options exclude UNSPECIFIED, leading with ANOMALY and CREATE_FAILED", () => {
	assert.equal(DEFAULT_PAYMENT_FILTERS.status, PaymentStatus.ANOMALY);
	assert.deepEqual(PAYMENT_STATUS_OPTIONS.slice(0, 2), [
		PaymentStatus.ANOMALY,
		PaymentStatus.CREATE_FAILED,
	]);
	assert.equal(
		PAYMENT_STATUS_OPTIONS.includes(PaymentStatus.UNSPECIFIED),
		false,
	);
	assert.equal(PAYMENT_STATUS_OPTIONS.length, 7);
});

test("refund anomaly_only follows the checkbox", () => {
	assert.equal(
		listPaymentRefundsRequest(DEFAULT_REFUND_FILTERS, "").anomalyOnly,
		true,
	);
	const off = listPaymentRefundsRequest(
		{ ...DEFAULT_REFUND_FILTERS, anomalyOnly: false },
		"",
	);
	assert.equal(off.anomalyOnly, false);
	assert.equal(off.status, undefined);
});

test("page token is passed through and bound to the page", () => {
	let tokens = firstPage();
	assert.equal(
		listPaymentTransfersRequest(
			{ status: PaymentTransferStatus.FAILED, orderNo: "" },
			currentToken(tokens),
		).pageToken,
		"",
	);
	tokens = nextPage(tokens, "tok2");
	const req = listPaymentTransfersRequest(
		{ status: PaymentTransferStatus.FAILED, orderNo: "" },
		currentToken(tokens),
	);
	assert.equal(req.pageToken, "tok2");
	assert.equal(req.status, PaymentTransferStatus.FAILED);
});

test("payment row shows int64 amount with separators and dashes for empty fields", () => {
	const row = paymentRow(
		create(PaymentSchema, {
			id: "p1",
			orderNo: "",
			amount: BigInt(1234567),
			status: PaymentStatus.ANOMALY,
			failureReason: "mismatch",
			createTime: timestampFromDate(new Date("2026-01-01T00:00:00Z")),
		}),
	);
	assert.equal(row.amount, "NT$ 1,234,567");
	assert.equal(row.orderNo, "—");
	assert.equal(row.statusVariant, "danger");
	assert.equal(row.failureReason, "mismatch");
	assert.equal(row.createTime, "2026/01/01 08:00:00");
});

test("refund row maps statuses and the auto flag; no create time shows a dash", () => {
	const row = refundRow(
		create(PaymentRefundSchema, {
			id: "r1",
			amount: BigInt(100),
			auto: true,
			status: PaymentRefundStatus.SUCCEEDED,
			reversalStatus: PaymentReversalStatus.ANOMALY,
			topupStatus: PaymentTopupStatus.NONE,
		}),
	);
	assert.equal(row.auto, "自動");
	assert.equal(row.statusVariant, "success");
	assert.notEqual(row.reversalStatus, row.topupStatus);
	assert.equal(row.createTime, "—");
});

test("transfer row keeps reversed amount separate", () => {
	const row = transferRow(
		create(PaymentTransferSchema, {
			id: "t1",
			amount: BigInt(500),
			reversedAmount: BigInt(200),
			status: PaymentTransferStatus.PENDING,
		}),
	);
	assert.equal(row.amount, "NT$ 500");
	assert.equal(row.reversedAmount, "NT$ 200");
	assert.equal(row.statusVariant, "warning");
});
