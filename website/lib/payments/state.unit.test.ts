import { create } from "@bufbuild/protobuf";
import {
	PaymentSchema,
	PaymentSessionSchema,
	PaymentStatus,
	StripePaymentSessionSchema,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import { describe, expect, it } from "vitest";
import { paymentView } from "./state";

const session = create(PaymentSessionSchema, {
	provider: {
		case: "stripe",
		value: create(StripePaymentSessionSchema, {
			clientSecret: "pi_1_secret_x",
			publishableKey: "pk_test_1",
		}),
	},
});

const payment = (status: PaymentStatus, withSession = false) =>
	create(PaymentSchema, {
		orderNo: "O1",
		status,
		...(withSession ? { session } : {}),
	});

describe("paymentView", () => {
	it("offers the form only for AWAITING_PAYMENT that carries a Stripe session", () => {
		expect(paymentView(payment(PaymentStatus.AWAITING_PAYMENT, true))).toEqual({
			kind: "pay",
			clientSecret: "pi_1_secret_x",
			publishableKey: "pk_test_1",
		});
	});

	it("waits when AWAITING_PAYMENT has no session (the provider already moved on)", () => {
		expect(paymentView(payment(PaymentStatus.AWAITING_PAYMENT)).kind).toBe(
			"wait",
		);
	});

	it("waits for CREATING and PROCESSING", () => {
		expect(paymentView(payment(PaymentStatus.CREATING)).kind).toBe("wait");
		expect(paymentView(payment(PaymentStatus.PROCESSING)).kind).toBe("wait");
	});

	it("succeeds only for SUCCEEDED", () => {
		expect(paymentView(payment(PaymentStatus.SUCCEEDED)).kind).toBe(
			"succeeded",
		);
	});

	it("lets the buyer retry after CANCELED and CREATE_FAILED", () => {
		expect(paymentView(payment(PaymentStatus.CANCELED)).kind).toBe("retry");
		expect(paymentView(payment(PaymentStatus.CREATE_FAILED)).kind).toBe(
			"retry",
		);
	});

	it("shows the support state for ANOMALY", () => {
		expect(paymentView(payment(PaymentStatus.ANOMALY)).kind).toBe("anomaly");
	});

	it("never treats UNSPECIFIED, unknown values or a missing payment as success", () => {
		for (const view of [
			paymentView(payment(PaymentStatus.UNSPECIFIED)),
			paymentView(payment(99 as PaymentStatus)),
			paymentView(undefined),
		]) {
			expect(view.kind).toBe("unknown");
		}
	});
});
