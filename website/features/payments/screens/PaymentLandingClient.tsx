"use client";

import { errorText } from "@hawkr/api/connect/errors";
import {
	Elements,
	PaymentElement,
	useElements,
	useStripe,
} from "@stripe/react-stripe-js";
import { loadStripe } from "@stripe/stripe-js";
import { useTranslations } from "next-intl";
import * as React from "react";
import Button from "@/components/ui/Button";
import PaymentStatusCard from "@/features/payments/components/PaymentStatusCard";
import { useAuth } from "@/lib/auth/context";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { activeLocale } from "@/lib/intl/active-locale";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { getPayment, openPayment } from "@/lib/payments/api";
import { PAYMENT_ELEMENT_APPEARANCE } from "@/lib/payments/appearance";
import {
	type ConfirmOutcome,
	confirmAndSettle,
	paymentReturnUrl,
} from "@/lib/payments/confirm";
import type { PollResult } from "@/lib/payments/poll";

interface PaymentLandingClientProps {
	orderNo: string;
}

type Step =
	| { kind: "loading" }
	| {
			kind: "pay";
			clientSecret: string;
			publishableKey: string;
			notice?: string;
	  }
	| { kind: "retry" }
	| { kind: "anomaly" }
	| { kind: "timeout" }
	| { kind: "succeeded" }
	| { kind: "error"; message: string };

/**
 * 結帳確認後的付款步驟（`/orders/{orderNo}`）：CreatePayment 取得 session，以 Stripe Payment Element 收款。
 * 付款結果只認 GetPayment；`confirmPayment` 的回報不是結果（見 `lib/payments/confirm.ts`）。
 */
export default function PaymentLandingClient({
	orderNo,
}: PaymentLandingClientProps) {
	const router = useObservedRouter("payment_landing");
	const { status } = useAuth();
	const t = useTranslations("payment");
	const defaultErrors = useDefaultErrorText();
	const defaultErrorsRef = React.useRef(defaultErrors);
	defaultErrorsRef.current = defaultErrors;
	const [step, setStep] = React.useState<Step>({ kind: "loading" });
	const [attempt, setAttempt] = React.useState(0);

	const apply = React.useCallback(
		(result: PollResult | ConfirmOutcome, notice?: string) => {
			if (result.kind === "timeout") return setStep({ kind: "timeout" });
			if (result.kind === "declined") {
				return setStep((prev) =>
					prev.kind === "pay" ? { ...prev, notice: result.message } : prev,
				);
			}
			const view = result.view;
			if (view.kind === "pay") {
				return setStep({
					kind: "pay",
					clientSecret: view.clientSecret,
					publishableKey: view.publishableKey,
					notice,
				});
			}
			setStep({ kind: view.kind });
		},
		[],
	);

	React.useEffect(() => {
		if (status === "guest") {
			router.replace(`/account/orders/${encodeURIComponent(orderNo)}`);
		}
	}, [status, orderNo, router]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: attempt 只用來重跑
	React.useEffect(() => {
		if (status !== "signedIn") return;
		const abort = new AbortController();
		setStep({ kind: "loading" });
		openPayment(orderNo, abort.signal)
			.then((result) => {
				if (!abort.signal.aborted) apply(result);
			})
			.catch((e) => {
				if (!abort.signal.aborted) {
					setStep({
						kind: "error",
						message: errorText(e, defaultErrorsRef.current),
					});
				}
			});
		return () => abort.abort();
	}, [status, orderNo, attempt, apply]);

	React.useEffect(() => {
		if (step.kind === "succeeded") {
			router.replace(`/checkout/success?order=${encodeURIComponent(orderNo)}`);
		}
	}, [step.kind, orderNo, router]);

	const publishableKey = step.kind === "pay" ? step.publishableKey : null;
	const stripePromise = React.useMemo(
		() => (publishableKey ? loadStripe(publishableKey) : null),
		[publishableKey],
	);

	if (step.kind === "loading") {
		return (
			<PaymentStatusCard kind="waiting" orderNo={orderNo} message={undefined} />
		);
	}
	if (step.kind === "succeeded") {
		return <PaymentStatusCard kind="succeeded" orderNo={orderNo} />;
	}
	if (step.kind === "anomaly" || step.kind === "timeout") {
		return <PaymentStatusCard kind={step.kind} orderNo={orderNo} />;
	}
	if (step.kind === "error") {
		return (
			<section className="max-w-[580px] mx-auto py-10 px-5 flex flex-col gap-4">
				<PaymentStatusCard
					kind="error"
					orderNo={orderNo}
					message={step.message}
				/>
				<Button type="button" onClick={() => setAttempt((n) => n + 1)}>
					{t("retry")}
				</Button>
			</section>
		);
	}
	if (step.kind === "retry") {
		return (
			<section className="max-w-[580px] mx-auto py-10 px-5 flex flex-col gap-4">
				<h1 className="font-display text-xl md:text-2xl m-0 text-foreground">
					{t("title")}
				</h1>
				<p role="alert" className="m-0 text-sm text-foreground-soft">
					{t("notCompleted")}
				</p>
				<Button type="button" onClick={() => setAttempt((n) => n + 1)}>
					{t("retry")}
				</Button>
			</section>
		);
	}

	return (
		<section className="max-w-[580px] mx-auto py-10 px-5 flex flex-col gap-4">
			<h1 className="font-display text-xl md:text-2xl m-0 text-foreground">
				{t("title")}
			</h1>
			<p className="m-0 text-xs font-bold text-foreground-muted">
				{t("orderNo")} {orderNo}
			</p>
			<Elements
				stripe={stripePromise}
				options={{
					clientSecret: step.clientSecret,
					appearance: PAYMENT_ELEMENT_APPEARANCE,
					locale: activeLocale() === "en" ? "en" : "zh-TW",
				}}
			>
				<PayForm
					orderNo={orderNo}
					notice={step.notice}
					onOutcome={(outcome) => apply(outcome, t("notCompleted"))}
					onError={(e) =>
						setStep({
							kind: "error",
							message: errorText(e, defaultErrorsRef.current),
						})
					}
				/>
			</Elements>
		</section>
	);
}

function PayForm({
	orderNo,
	notice,
	onOutcome,
	onError,
}: {
	orderNo: string;
	notice: string | undefined;
	onOutcome: (outcome: ConfirmOutcome) => void;
	onError: (e: unknown) => void;
}) {
	const t = useTranslations("payment");
	const stripe = useStripe();
	const elements = useElements();
	const [busy, setBusy] = React.useState(false);

	const submit = async (event: React.FormEvent) => {
		event.preventDefault();
		if (!stripe || !elements || busy) return;
		setBusy(true);
		try {
			onOutcome(
				await confirmAndSettle({
					stripe,
					elements,
					returnUrl: paymentReturnUrl(
						window.location.origin,
						activeLocale(),
						orderNo,
					),
					read: () => getPayment(orderNo),
				}),
			);
		} catch (e) {
			onError(e);
		} finally {
			setBusy(false);
		}
	};

	return (
		<form onSubmit={submit} className="flex flex-col gap-4">
			<PaymentElement />
			{notice ? (
				<p role="alert" className="m-0 text-sm text-danger-600">
					{notice}
				</p>
			) : null}
			<Button
				type="submit"
				size="lg"
				fullWidth
				disabled={!stripe || !elements || busy}
				isLoading={busy}
				loadingLabel={t("submitting")}
			>
				{t("submit")}
			</Button>
		</form>
	);
}
