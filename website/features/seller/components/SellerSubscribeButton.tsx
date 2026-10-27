"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { useSetFollowedSeller } from "@/lib/hooks/useFollowedSellers";
import { useSellerSubscription } from "@/lib/hooks/useSellerSubscription";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

interface SellerSubscribeButtonProps {
	sellerId: string;
	className?: string;
	label: React.ReactNode;
	activeLabel?: React.ReactNode;
	/** Buyer-view preview preserves the buyer CTA appearance without writing. */
	previewMode?: boolean;
}

/** Minimum ms between two effective taps. The optimistic flip means there is
 *  no `disabled` dim to block a fast double-tap, so this absorbs the natural
 *  human double-tap window before a second POST/DELETE can fire. */
const CLICK_THROTTLE_MS = 350;

export default function SellerSubscribeButton({
	sellerId,
	className,
	label,
	activeLabel = "已訂閱",
	previewMode = false,
}: SellerSubscribeButtonProps) {
	const { isLoggedIn } = useAuth();
	const setFollowed = useSetFollowedSeller();
	const { active, toggle, pending } = useSellerSubscription(sellerId);
	const [notice, setNotice] = React.useState<string | null>(null);
	const lastClickAtRef = React.useRef(0);
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可訂閱商家",
			description: "訂閱後可以更快收到商家的新品、出貨與優惠動態。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(() => {
		setNotice(null);
		openLoginRequired();
	});

	React.useEffect(() => {
		if (!notice) return;
		const timer = window.setTimeout(() => setNotice(null), 5200);
		return () => window.clearTimeout(timer);
	}, [notice]);

	const runToggle = React.useCallback(async () => {
		const next = !active;
		// Bridge the shared set so sibling follow chips (home/market) agree
		// immediately; `toggle()` owns the authoritative per-seller cache.
		setFollowed(sellerId, next);
		try {
			const result = await toggle();
			setFollowed(sellerId, result);
		} catch (error) {
			setFollowed(sellerId, active);
			if (error instanceof Error && error.name === "UnauthorizedError") {
				openLoginRequired();
			} else {
				setNotice("訂閱暫時無法使用，請稍後再試。");
			}
		}
	}, [active, setFollowed, openLoginRequired, sellerId, toggle]);

	const handleClick = () => {
		if (previewMode) return;
		const now = Date.now();
		if (isLoggedIn && now - lastClickAtRef.current < CLICK_THROTTLE_MS) {
			return;
		}
		lastClickAtRef.current = now;
		if (pending) return;

		// Role comes from the gate's post-hydration snapshot — the render-time
		// value is stale when the click happened before the session resolved.
		authGate(({ activeRole: resolvedRole }) => {
			if (resolvedRole !== "buyer") {
				setNotice("請切換到買家帳號後再訂閱商家。");
				return;
			}
			void runToggle();
		});
	};

	return (
		<div className="hkui-seller-profile__subscribe">
			<Button
				type="button"
				variant="light"
				color="default"
				size="sm"
				className={`h-auto ${className ?? ""}`.trim()}
				onClick={handleClick}
				aria-pressed={active}
				aria-label={active ? "取消訂閱商家" : "訂閱商家"}
				aria-disabled={previewMode || undefined}
				title={previewMode ? "買家視角預覽不會變更訂閱狀態" : undefined}
				aria-describedby={
					notice ? `seller-subscribe-notice-${sellerId}` : undefined
				}
				disabled={authWaiting || pending}
				isLoading={pending}
				loadingLabel={active ? "訂閱商家中…" : "取消訂閱商家中…"}
				loadingReservationLabel="取消訂閱商家中…"
			>
				<AppIcon
					name="notification"
					size={18}
					/* Match the label colour per state: cream on the filled 訂閱
					   pill, sage on the outlined 已訂閱 pill. */
					style={{
						color: active
							? "var(--hkui-primary)"
							: "var(--hkui-primary-foreground)",
					}}
				/>
				{active ? activeLabel : label}
			</Button>
			<AuthGateWaitModal open={authWaiting} />
			{notice ? (
				<div
					id={`seller-subscribe-notice-${sellerId}`}
					className="hkui-seller-profile__login-popover"
					role="alert"
				>
					<strong>{notice}</strong>
				</div>
			) : null}
		</div>
	);
}
