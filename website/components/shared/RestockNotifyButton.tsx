"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import { emitCartToast } from "@/components/shared/CartSnackbar";
import Button from "@/components/ui/Button";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import {
	subscribeRestock,
	unsubscribeRestock,
} from "@/lib/restock-subscriptions/repo";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

interface RestockNotifyButtonProps {
	productId: string | number;
	productName?: string;
	/**
	 * Initial subscribed state. The market list wire doesn't carry it yet, so it
	 * defaults to `false`; subscribe is idempotent on the backend, so a buyer who
	 * already subscribed just re-confirms safely on first tap.
	 */
	initialSubscribed?: boolean;
	className?: string;
	/** Stop the click bubbling to a wrapping <Link> (product cards). */
	stopPropagation?: boolean;
}

/**
 * Out-of-stock "補貨通知" subscribe toggle. Mirrors AddToCartButton's success
 * choreography so the sold-out card feels alive: tap → the brand LOGO reports
 * the request immediately → a thumb-up caps it. Tapping again once subscribed
 * quietly unsubscribes (no celebration).
 *
 * Distinct from favouriting — see lib/restock-subscriptions/repo.ts.
 */
type Phase = "idle" | "subscribing" | "unsubscribing" | "done";

// Hold the LOGO a beat so the pending state remains readable even when the API
// answers instantly, then let the success mark finish its acknowledgement.
const MIN_PENDING_MS = 460;
const DONE_MS = 1400;

function isUnauthorized(err: unknown): boolean {
	return err instanceof Error && err.name === "UnauthorizedError";
}

export default function RestockNotifyButton({
	productId,
	productName,
	initialSubscribed = false,
	className,
	stopPropagation = false,
}: RestockNotifyButtonProps) {
	const [subscribed, setSubscribed] = React.useState(initialSubscribed);
	const [phase, setPhase] = React.useState<Phase>("idle");
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可開啟補貨通知",
			description:
				"開啟補貨通知，商品補貨時我們會第一時間通知你，不錯過想買的好物。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);
	const timersRef = React.useRef<number[]>([]);
	const pendingRef = React.useRef(false);
	const doneTimerRef = React.useRef<number | null>(null);

	React.useEffect(() => {
		setSubscribed(initialSubscribed);
	}, [initialSubscribed]);

	// Don't fire setState after unmount if the card scrolls out mid-animation.
	React.useEffect(
		() => () => {
			for (const t of timersRef.current) window.clearTimeout(t);
			timersRef.current = [];
		},
		[],
	);

	const delay = (ms: number) =>
		new Promise<void>((resolve) => {
			const t = window.setTimeout(resolve, ms);
			timersRef.current.push(t);
		});

	const loading = phase === "subscribing" || phase === "unsubscribing";

	const onClick = (event: React.MouseEvent<HTMLButtonElement>) => {
		if (stopPropagation) event.stopPropagation();
		event.preventDefault();
		// Only a request in flight blocks input. The 1.4s success acknowledgement
		// used to lock the control too, so a mis-tap could not be taken back until
		// the celebration finished; tapping through it now cancels the
		// subscription, which is what a buyer reaching for it means.
		if (loading || authWaiting) return;
		if (doneTimerRef.current != null) {
			window.clearTimeout(doneTimerRef.current);
			doneTimerRef.current = null;
			setPhase("idle");
		}
		authGate(() => void run());
	};

	const run = async () => {
		if (pendingRef.current) return;
		pendingRef.current = true;
		try {
			// Already on → quiet unsubscribe (no celebration), optimistic with rollback.
			if (subscribed) {
				setPhase("unsubscribing");
				setSubscribed(false);
				try {
					await unsubscribeRestock(productId);
					emitCartToast({ message: "已取消補貨通知", actionHref: null });
				} catch (err) {
					setSubscribed(true);
					if (isUnauthorized(err)) openLoginRequired();
					else
						emitCartToast({
							message: "取消補貨通知失敗，請稍後再試",
							tone: "error",
							actionHref: null,
						});
				} finally {
					setPhase("idle");
				}
				return;
			}

			// Subscribe: report the request immediately, then acknowledge success.
			setPhase("subscribing");
			try {
				await Promise.all([subscribeRestock(productId), delay(MIN_PENDING_MS)]);
				setSubscribed(true);
				setPhase("done");
				emitCartToast({
					message: "已開啟補貨通知",
					actionHref: null,
				});
				// The acknowledgement runs on its own timer rather than holding this
				// call open: `pendingRef` must be released now so a second tap during
				// the celebration can start the un-subscribe (see `onClick`).
				if (doneTimerRef.current != null)
					window.clearTimeout(doneTimerRef.current);
				const timer = window.setTimeout(() => {
					doneTimerRef.current = null;
					setPhase("idle");
				}, DONE_MS);
				doneTimerRef.current = timer;
				timersRef.current.push(timer);
			} catch (err) {
				setPhase("idle");
				if (isUnauthorized(err)) openLoginRequired();
				else
					emitCartToast({
						message: "設定補貨通知失敗，請稍後再試",
						tone: "error",
						actionHref: null,
					});
			}
		} finally {
			pendingRef.current = false;
		}
	};

	const label = subscribed ? "已開啟通知" : "補貨通知";
	// Bell/check match the button text colour: cream on the active (primary)
	// pill in both themes; idle pill uses the themed primary so the icon stays
	// primary in light mode but lifts to primary on the dark pill.
	const iconColor = subscribed ? "var(--color-hk-cream)" : "var(--icon-accent)";

	return (
		<>
			<Button
				type="button"
				onClick={onClick}
				disabled={authWaiting}
				isLoading={loading}
				loadingLabel={
					phase === "unsubscribing" ? "取消補貨通知中…" : "設定補貨通知中…"
				}
				loadingReservationLabel="取消補貨通知中…"
				aria-pressed={subscribed}
				aria-busy={loading || authWaiting || undefined}
				aria-label={
					subscribed || phase === "unsubscribing"
						? `取消補貨通知${productName ? `：${productName}` : ""}`
						: `補貨通知${productName ? `：${productName}` : ""}`
				}
				data-on={subscribed ? "true" : "false"}
				data-busy={loading ? "true" : "false"}
				className={`bg-hk-secondary-soft text-hk-primary-strong dark:bg-white/10 dark:text-[var(--page-restock-fg-on-dark)] dark:shadow-[inset_0_0_0_1px_var(--page-restock-ring-on-dark)] data-[on=true]:bg-hk-primary-strong data-[on=true]:text-hk-cream dark:data-[on=true]:bg-[var(--page-restock-on-bg-on-dark)] dark:data-[on=true]:text-hk-cream ${className ?? ""}`.trim()}
			>
				{phase === "done" ? (
					<span className="inline-flex animate-hk-fade-in items-center justify-center motion-reduce:animate-none">
						<AppIcon
							name="check"
							size={24}
							style={{ color: "var(--color-hk-cream)" }}
						/>
						<span className="sr-only">已開啟補貨通知</span>
					</span>
				) : (
					<>
						<span className="market-product-card__cart-btn-label">{label}</span>
						<AppIcon
							className="market-product-card__cart-btn-icon"
							name="notification"
							size={22}
							style={{ color: iconColor }}
						/>
					</>
				)}
			</Button>
			<AuthGateWaitModal open={authWaiting} />
		</>
	);
}
