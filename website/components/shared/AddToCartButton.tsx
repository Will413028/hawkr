"use client";

import * as React from "react";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import { emitCartToast } from "@/components/shared/CartSnackbar";
import Button from "@/components/ui/Button";
import { trackAddToCart } from "@/lib/analytics/events";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { addToCart, CartError, type CartInput } from "@/lib/cart";
import { flyToCart } from "@/lib/cart/flyToCart";
import { resolveCartInputSku } from "@/lib/cart/resolve";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

interface AddToCartButtonProps {
	item: CartInput;
	className?: string;
	children?: React.ReactNode;
	addedLabel?: React.ReactNode;
	stopPropagation?: boolean;
	ariaLabel?: string;
	disabled?: boolean;
	onAdded?: () => void;
}

/**
 * Map a CartError (or any thrown value) to a buyer-facing message.
 * Centralised so the toast text matches design across call sites.
 */
function cartErrorMessage(err: unknown): string {
	if (err instanceof CartError) {
		// 6002 INVALID_SKU / 6007 CART_PRODUCT_DATA_INVALID
		if (err.code === 6002 || err.code === 6007) return "商品已下架或暫不可購買";
		// 6003 CART_ITEM_LOCKED — confirm in flight from another tab/device
		if (err.code === 6003) return "商品結帳處理中,請稍候片刻後再試";
		// 6006 CART_SELF_ITEM_NOT_ALLOWED
		if (err.code === 6006) return "不可購買自己上架的商品";
		return err.message || "加入購物車失敗,請稍後再試";
	}
	return err instanceof Error ? err.message : "加入購物車失敗,請稍後再試";
}

/**
 * A cart add usually resolves faster than the eye reads a state change. Showing
 * the pending skin immediately therefore produced a grey flash on every tap —
 * the button visibly destabilised itself to report work the buyer never waited
 * for. Below this threshold the button simply does not change; above it, the
 * wait is real and the pending state is the honest thing to show.
 */
const PENDING_VISIBLE_AFTER_MS = 260;

export default function AddToCartButton({
	item,
	className,
	children = "加入購物車",
	addedLabel = "已加入",
	stopPropagation = false,
	ariaLabel,
	disabled = false,
	onAdded,
}: AddToCartButtonProps) {
	const [added, setAdded] = React.useState(false);
	const [pending, setPending] = React.useState(false);
	/** `pending` gates input; `showPending` gates the *skin*. See the threshold above. */
	const [showPending, setShowPending] = React.useState(false);
	const buttonRef = React.useRef<HTMLButtonElement & HTMLAnchorElement>(null);
	const pendingTimerRef = React.useRef<number | null>(null);
	const addedTimerRef = React.useRef<number | null>(null);
	const hadFocusRef = React.useRef(false);

	// Restore focus the button lost to the pending state (see `onClick`). Only
	// when nothing else has claimed it — never steal focus back from wherever the
	// buyer moved on to.
	React.useEffect(() => {
		if (showPending || !hadFocusRef.current) return;
		hadFocusRef.current = false;
		const active = document.activeElement;
		if (active && active !== document.body) return;
		buttonRef.current?.focus({ preventScroll: true });
	}, [showPending]);

	React.useEffect(
		() => () => {
			if (pendingTimerRef.current != null)
				window.clearTimeout(pendingTimerRef.current);
			if (addedTimerRef.current != null)
				window.clearTimeout(addedTimerRef.current);
		},
		[],
	);
	/**
	 * Gated login prompt. Cart state lives in localStorage so guests
	 * *technically* could add items, but the rest of the flow (checkout
	 * + shipping) needs an authenticated session. Catching it here
	 * matches the feed's like / save / comment behaviour and avoids a
	 * dead-end when guests reach the cart page.
	 */
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可加入購物車",
			description: "登入 Hawkr 即可下單、追蹤訂單，並把購物車跨裝置同步。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);

	const onClick = (event: React.MouseEvent<HTMLButtonElement>) => {
		if (stopPropagation) event.stopPropagation();
		if (disabled || pending || authWaiting) return;
		// The pending skin routes through Button's `isLoading`, which sets the DOM
		// `disabled` — and a focused element that becomes disabled drops focus to
		// <body>. Remember that the button owned focus so it can be handed back
		// when the request settles; a keyboard buyer keeps their place in the grid.
		hadFocusRef.current =
			typeof document !== "undefined" &&
			document.activeElement === buttonRef.current;
		authGate(() => void performAdd());
	};

	const performAdd = async () => {
		setPending(true);
		pendingTimerRef.current = window.setTimeout(() => {
			setShowPending(true);
			pendingTimerRef.current = null;
		}, PENDING_VISIBLE_AFTER_MS);
		try {
			const resolvedItem = await resolveCartInputSku(item);
			await addToCart(resolvedItem);
			const quantity = Math.max(1, Number(resolvedItem.quantity || 1));
			trackAddToCart(resolvedItem, quantity);
			setAdded(true);
			// Fired before the toast so the chip leaves the button the buyer is
			// still looking at, and lands as the cart badge ticks up.
			flyToCart(buttonRef.current, quantity);
			emitCartToast({ message: `已加入購物車：${resolvedItem.name}` });
			onAdded?.();
			if (addedTimerRef.current != null)
				window.clearTimeout(addedTimerRef.current);
			addedTimerRef.current = window.setTimeout(() => {
				setAdded(false);
				addedTimerRef.current = null;
			}, 1400);
		} catch (err) {
			if (err instanceof CartError && err.status === 401) {
				openLoginRequired();
			} else {
				emitCartToast({ message: cartErrorMessage(err), tone: "error" });
			}
		} finally {
			if (pendingTimerRef.current != null) {
				window.clearTimeout(pendingTimerRef.current);
				pendingTimerRef.current = null;
			}
			setPending(false);
			setShowPending(false);
		}
	};

	return (
		<>
			<Button
				ref={buttonRef}
				type="button"
				className={`transition-colors motion-reduce:transition-none data-[added=true]:bg-[var(--hkui-secondary)] ${className ?? ""}`.trim()}
				onClick={onClick}
				// Double-submit is already blocked by the `pending` guard in
				// `onClick`, and `aria-disabled` (not `disabled`) is what reports the
				// in-flight state: it dims and swallows pointer input exactly like
				// `disabled` while keeping the button focusable, so the tap does not
				// throw the buyer's keyboard position away. Only a caller-declared
				// `disabled` (out of stock, etc.) removes the control outright.
				aria-disabled={showPending || authWaiting || undefined}
				disabled={disabled}
				isLoading={showPending}
				loadingLabel="加入中…"
				aria-label={ariaLabel || `加入購物車：${item.name}`}
				aria-busy={pending || authWaiting || undefined}
				data-added={added ? "true" : "false"}
				data-pending={showPending ? "true" : "false"}
			>
				{added ? addedLabel : children}
			</Button>
			<AuthGateWaitModal open={authWaiting} />
		</>
	);
}
