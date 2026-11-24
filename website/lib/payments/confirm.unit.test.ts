import { create } from "@bufbuild/protobuf";
import {
	PaymentSchema,
	PaymentStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import type { StripeElements } from "@stripe/stripe-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { confirmAndSettle, paymentReturnUrl } from "./confirm";
import { pollPayment } from "./poll";

const payment = (status: PaymentStatus) =>
	create(PaymentSchema, { orderNo: "O1", status });

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("paymentReturnUrl", () => {
	it("carries only the order number, locale-aware", () => {
		expect(paymentReturnUrl("https://hawkr.test", "zh-TW", "O 1")).toBe(
			"https://hawkr.test/payment/return?order=O%201",
		);
		expect(paymentReturnUrl("https://hawkr.test", "en", "O1")).toBe(
			"https://hawkr.test/en/payment/return?order=O1",
		);
	});
});

describe("confirmAndSettle", () => {
	const elements = {} as StripeElements;

	it("passes the return url to confirmPayment and does not redirect when not required", async () => {
		const confirmPayment = vi.fn().mockResolvedValue({});
		const read = vi.fn().mockResolvedValue(payment(PaymentStatus.SUCCEEDED));
		const returnUrl = paymentReturnUrl("https://hawkr.test", "zh-TW", "O1");
		await confirmAndSettle({
			stripe: { confirmPayment },
			elements,
			returnUrl,
			read,
		});
		const args = confirmPayment.mock.calls[0][0];
		expect(args.confirmParams.return_url).toContain("order=O1");
		expect(args.redirect).toBe("if_required");
		expect(args.elements).toBe(elements);
	});

	it("does not report success from the client result alone: it waits for GetPayment to say SUCCEEDED", async () => {
		const confirmPayment = vi
			.fn()
			.mockResolvedValue({ paymentIntent: { status: "succeeded" } });
		const read = vi
			.fn()
			.mockResolvedValueOnce(payment(PaymentStatus.PROCESSING))
			.mockResolvedValueOnce(payment(PaymentStatus.PROCESSING))
			.mockResolvedValueOnce(payment(PaymentStatus.SUCCEEDED));
		let settled: unknown;
		const run = confirmAndSettle({
			stripe: { confirmPayment },
			elements,
			returnUrl: "https://hawkr.test/payment/return?order=O1",
			read,
			intervalMs: 1000,
		}).then((r) => {
			settled = r;
		});
		await vi.advanceTimersByTimeAsync(0);
		expect(settled).toBeUndefined();
		await vi.advanceTimersByTimeAsync(1000);
		expect(settled).toBeUndefined();
		await vi.advanceTimersByTimeAsync(1000);
		await run;
		expect(settled).toEqual({ kind: "settled", view: { kind: "succeeded" } });
		expect(read).toHaveBeenCalledTimes(3);
	});

	it("returns the Stripe decline without reading the payment", async () => {
		const confirmPayment = vi
			.fn()
			.mockResolvedValue({ error: { message: "card declined" } });
		const read = vi.fn();
		const outcome = await confirmAndSettle({
			stripe: { confirmPayment },
			elements,
			returnUrl: "https://hawkr.test/payment/return?order=O1",
			read,
		});
		expect(outcome).toEqual({ kind: "declined", message: "card declined" });
		expect(read).not.toHaveBeenCalled();
	});
});

describe("pollPayment", () => {
	it("times out after the attempt cap instead of reporting success", async () => {
		const read = vi.fn().mockResolvedValue(payment(PaymentStatus.PROCESSING));
		const run = pollPayment({ read, intervalMs: 1000, maxAttempts: 3 });
		await vi.advanceTimersByTimeAsync(5000);
		expect(await run).toEqual({ kind: "timeout" });
		expect(read).toHaveBeenCalledTimes(3);
	});

	it("stops on a state that needs the buyer, and keeps going through UNSPECIFIED", async () => {
		const read = vi
			.fn()
			.mockResolvedValueOnce(payment(PaymentStatus.UNSPECIFIED))
			.mockResolvedValueOnce(payment(PaymentStatus.CANCELED));
		const run = pollPayment({ read, intervalMs: 1000 });
		await vi.advanceTimersByTimeAsync(1000);
		expect(await run).toEqual({ kind: "settled", view: { kind: "retry" } });
	});

	it("stops reading after the signal aborts", async () => {
		const read = vi.fn().mockResolvedValue(payment(PaymentStatus.PROCESSING));
		const abort = new AbortController();
		const run = pollPayment({ read, intervalMs: 1000, signal: abort.signal });
		await vi.advanceTimersByTimeAsync(0);
		abort.abort();
		expect(await run).toEqual({ kind: "timeout" });
		expect(read).toHaveBeenCalledTimes(1);
	});
});
