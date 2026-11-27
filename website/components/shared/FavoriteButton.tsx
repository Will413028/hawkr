"use client";

import * as React from "react";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import Button from "@/components/ui/Button";
import { trackAddToWishlist } from "@/lib/analytics/events";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { toggleFavorite } from "@/lib/favorites/repo";
import { useIsFavorited, useSetFavorited } from "@/lib/hooks/useFavoriteIds";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

interface FavoriteButtonProps {
	productId: string | number;
	productName?: string;
	/**
	 * Initial favourited state taken from the product-list wire
	 * (`handler.ProductItem.isFavorited`). Pass this on grids so the heart
	 * renders the right state with **zero** extra requests — never fan out a
	 * `checkFavorite` per card.
	 */
	initialFavorited?: boolean;
	/**
	 * Per-product favourite total (被收藏數). When a number is supplied it shows
	 * beside the heart and is optimistically ±1 on toggle. Leave `undefined`
	 * (the field isn't on the wire yet — see B-192) and only the heart renders.
	 */
	count?: number | null;
	/** Positioning / layout classes from the caller (e.g. `absolute right-3 top-3`). */
	className?: string;
	size?: number;
}

/** Mirrors `SellerSubscribeButton`: 350ms absorbs the human double-tap window
 *  so a fast double-click can't fire add+remove back to back. */
const CLICK_THROTTLE_MS = 350;

/** Matches the feed card's `formatCount` (萬 / k) so counts read the same
 *  everywhere in the product. */
function formatCount(n: number): string {
	if (n >= 10000) return `${(n / 10000).toFixed(1)}萬`;
	if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
	return String(n);
}

export default function FavoriteButton({
	productId,
	productName,
	initialFavorited = false,
	count = null,
	className,
	size = 16,
}: FavoriteButtonProps) {
	const { status } = useAuth();
	const favFlag = useIsFavorited(productId);
	const setFavorited = useSetFavorited();
	const [active, setActive] = React.useState(initialFavorited);
	const [favCount, setFavCount] = React.useState<number | null>(count);
	const [pendingAction, setPendingAction] = React.useState<
		"add" | "remove" | null
	>(null);
	const loading = pendingAction !== null;
	const lastClickAtRef = React.useRef(0);
	const inFlightRef = React.useRef(false);
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可收藏商品",
			description:
				"收藏喜歡的商品，登入後就能在「我的收藏」隨時找回，也不會錯過降價與補貨通知。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);

	React.useEffect(() => {
		setActive(initialFavorited);
	}, [initialFavorited]);

	React.useEffect(() => {
		setFavCount(count);
	}, [count]);

	React.useEffect(() => {
		if (typeof favFlag === "boolean") setActive(favFlag);
	}, [favFlag]);

	const bumpCount = (delta: number) =>
		setFavCount((c) => (c == null ? c : Math.max(0, c + delta)));

	const handleClick = (event: React.MouseEvent) => {
		event.preventDefault();
		event.stopPropagation();

		if (authWaiting || inFlightRef.current) return;
		const now = Date.now();
		if (
			status === "signedIn" &&
			now - lastClickAtRef.current < CLICK_THROTTLE_MS
		)
			return;
		lastClickAtRef.current = now;

		authGate(() => void performToggle());
	};

	const performToggle = async () => {
		if (inFlightRef.current) return;
		inFlightRef.current = true;
		const previous = active;
		const nextActive = !previous;
		const delta = nextActive ? 1 : -1;
		setPendingAction(nextActive ? "add" : "remove");
		setActive(nextActive);
		setFavorited(productId, nextActive);
		bumpCount(delta);
		try {
			const serverNext = await toggleFavorite(productId);
			setActive(serverNext);
			setFavorited(productId, serverNext);
			if (serverNext) {
				trackAddToWishlist({ productId, productName });
			}
			if (serverNext !== nextActive) bumpCount((serverNext ? 1 : -1) - delta);
		} catch (error) {
			setActive(previous);
			setFavorited(productId, previous);
			bumpCount(-delta);
			if (error instanceof Error && error.name === "UnauthorizedError") {
				openLoginRequired();
			}
		} finally {
			inFlightRef.current = false;
			setPendingAction(null);
		}
	};

	const label = active
		? `取消收藏${productName ? `：${productName}` : ""}`
		: `收藏${productName ? `：${productName}` : ""}`;
	const showCount = favCount != null && favCount > 0;

	return (
		<>
			<Button
				type="button"
				variant="light"
				color="default"
				size="sm"
				radius="full"
				isIconOnly
				onClick={handleClick}
				aria-busy={loading || authWaiting || undefined}
				aria-pressed={active}
				aria-label={label}
				data-active={active ? "true" : undefined}
				data-pending={loading ? "true" : undefined}
				className={`group/fav h-auto w-auto inline-flex items-center gap-[5px] rounded-full px-2.5 py-1.5 bg-[var(--page-favorite-bg)] backdrop-blur-md backdrop-saturate-150 text-white text-xs font-semibold tabular-nums shadow-[var(--page-favorite-shadow)] cursor-pointer transition-[background,color,transform] duration-150 ease-out before:absolute before:-inset-2 before:content-[''] hover:bg-[var(--page-favorite-bg-hover)] active:scale-90 motion-reduce:transition-none motion-reduce:translate-none motion-reduce:scale-100 data-[active=true]:text-[var(--page-favorite-active-fg)] ${className ?? ""}`}
			>
				<span className="inline-flex items-center gap-px">
					<svg
						viewBox="0 0 24 24"
						width={size}
						height={size}
						fill={active ? "currentColor" : "none"}
						stroke="currentColor"
						strokeWidth="2.2"
						strokeLinecap="round"
						strokeLinejoin="round"
						aria-hidden="true"
						className="shrink-0 transition-transform duration-150 ease-out group-active/fav:scale-110 motion-reduce:transition-none motion-reduce:translate-none motion-reduce:scale-100"
					>
						<path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
					</svg>
					{showCount ? (
						<span className="leading-none">{formatCount(favCount)}</span>
					) : null}
				</span>
			</Button>
			<AuthGateWaitModal open={authWaiting} />
		</>
	);
}
