"use client";

import * as React from "react";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Button from "@/components/ui/Button";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { toggleFollow } from "@/lib/follow/repo";
import {
	useIsFollowedSeller,
	useSetFollowedSeller,
} from "@/lib/hooks/useFollowedSellers";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

/**
 * Compact follow CTA for seller cards (home 推薦商家 rail, market seller
 * grid). Same backend follow as the storefront's SellerSubscribeButton —
 * this replaced a localStorage-only ToggleActionButton that never actually
 * followed anyone (audit 2026-06, High A1).
 *
 * State comes from the per-seller follow query; the cards mounted in one
 * render are checked together by one batch request, not a per-card fan-out.
 */

/** Mirrors SellerSubscribeButton: absorbs the human double-tap window. */
const CLICK_THROTTLE_MS = 350;

interface SellerFollowChipProps {
	sellerId: string | number;
	className?: string;
	label?: React.ReactNode;
	activeLabel?: React.ReactNode;
}

export default function SellerFollowChip({
	sellerId,
	className,
	label = "追蹤",
	activeLabel = "已追蹤",
}: SellerFollowChipProps) {
	const followedFlag = useIsFollowedSeller(sellerId);
	const setFollowed = useSetFollowedSeller();
	const [localActive, setLocalActive] = React.useState<boolean | null>(null);
	const [pendingAction, setPendingAction] = React.useState<
		"follow" | "unfollow" | null
	>(null);
	const loading = pendingAction !== null;
	const [roleNotice, setRoleNotice] = React.useState(false);
	const lastClickAtRef = React.useRef(0);
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可追蹤商家",
			description: "追蹤後可以更快收到商家的新品、出貨與優惠動態。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);

	React.useEffect(() => {
		if (!roleNotice) return;
		const timer = window.setTimeout(() => setRoleNotice(false), 2800);
		return () => window.clearTimeout(timer);
	}, [roleNotice]);

	// Local toggle wins (it is the freshest), then the shared set, then 未追蹤.
	const active = localActive ?? followedFlag ?? false;

	const handleClick = () => {
		const now = Date.now();
		if (now - lastClickAtRef.current < CLICK_THROTTLE_MS) return;
		lastClickAtRef.current = now;

		authGate(({ activeRole }) => {
			if (activeRole !== "buyer") {
				setRoleNotice(true);
				return;
			}
			if (loading) return;
			const previous = active;
			setPendingAction(previous ? "unfollow" : "follow");
			setLocalActive(!previous);
			setFollowed(sellerId, !previous);
			void (async () => {
				try {
					// toggleFollow re-checks server state first, so a stale shared
					// set can't cause a double-subscribe error.
					const next = await toggleFollow(sellerId, "seller");
					setLocalActive(next);
					setFollowed(sellerId, next);
				} catch {
					setLocalActive(previous);
					setFollowed(sellerId, previous);
				} finally {
					setPendingAction(null);
				}
			})();
		});
	};

	return (
		<>
			<Button
				type="button"
				variant="light"
				color="default"
				size="sm"
				className={`h-auto ${className ?? ""}`.trim()}
				onClick={handleClick}
				disabled={loading || authWaiting}
				isLoading={loading}
				loadingLabel={
					pendingAction === "unfollow" ? "取消追蹤商家中…" : "追蹤商家中…"
				}
				loadingReservationLabel="取消追蹤商家中…"
				aria-pressed={active}
				data-active={active ? "true" : undefined}
			>
				{roleNotice ? "請用買家帳號" : active ? activeLabel : label}
			</Button>
			<AuthGateWaitModal open={authWaiting} />
		</>
	);
}
