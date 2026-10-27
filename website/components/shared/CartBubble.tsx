"use client";

import type * as React from "react";
import NavBubbleLink from "@/components/shared/NavBubbleLink";
import {
	CART_FLY_BADGE_ATTR,
	CART_FLY_TARGET_ATTR,
} from "@/lib/cart/flyToCart";
import { useCart } from "@/lib/hooks/useCart";

const itemsLabel = (display: string) => `${display} 件商品`;

// Landing pad for the add-to-cart flight (lib/cart/flyToCart.ts). It sits on
// the inner icon box, not the link: the link carries the nav's own padding and
// label, so its centre is off the glyph.
const FLY_TARGET_PROPS = { [CART_FLY_TARGET_ATTR]: "" };
const FLY_BADGE_PROPS = { [CART_FLY_BADGE_ATTR]: "" };

export default function CartBubble({
	href,
	className,
	"aria-label": ariaLabel = "購物車",
	children,
	onClick,
	onFocus,
	onMouseEnter,
}: {
	href: string;
	className?: string;
	"aria-label"?: string;
	children?: React.ReactNode;
	onClick?: React.MouseEventHandler<HTMLAnchorElement>;
	onFocus?: React.FocusEventHandler<HTMLAnchorElement>;
	onMouseEnter?: React.MouseEventHandler<HTMLAnchorElement>;
}) {
	const { totalQuantity } = useCart();

	return (
		<NavBubbleLink
			href={href}
			className={className}
			label={ariaLabel}
			count={totalQuantity}
			countLabel={itemsLabel}
			onClick={onClick}
			onFocus={onFocus}
			onMouseEnter={onMouseEnter}
			innerProps={FLY_TARGET_PROPS}
			badgeProps={FLY_BADGE_PROPS}
		>
			{children}
		</NavBubbleLink>
	);
}
