"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import Button from "@/components/ui/Button";
import type { CartItem } from "@/lib/cart";
import { isFreeShippingForSubtotal } from "@/lib/checkout/free-shipping";
import type { AvailableShippingOption } from "@/lib/checkout/repo";
import { useShippingOptions } from "@/lib/hooks/useShippingOptions";
import { sellerPath } from "@/lib/seo/urls";
import { CartProductTile } from "./CartProductTile";

interface SellerGroup {
	sellerName: string;
	sellerStoreId: string;
	/** Canonical storefront segment (`store_code`) when the cart wire carries
	 *  one — see `sellerPath` (#365). */
	sellerStoreCode?: string;
	sellerLogoUrl?: string;
	sellerCity?: string;
	sellerDistrict?: string;
	items: CartItem[];
	subtotal: number;
}

interface Props {
	group: SellerGroup;
	groupKey: string;
	isLocked: boolean;
	isLoggedIn: boolean;
	selected: Record<string, boolean>;
	preferredMethod: string;
	editingQty: Record<string, string>;
	selectionBusy?: boolean;
	onToggleGroup: (groupKey: string, next: boolean) => void;
	onToggleItem: (item: CartItem, next: boolean) => void;
	onChangeQty: (id: string, raw: string) => void;
	onCommitQty: (id: string) => void;
	onUpdateQty: (item: CartItem, next: number) => void;
	onSaveForLater: (item: CartItem) => void;
	onRemove: (item: CartItem) => void;
}

function ntFormat(n: number) {
	return `NT$ ${Math.max(0, Math.round(n)).toLocaleString()}`;
}

/**
 * One seller's bucket in the cart: header (avatar + name + location +
 * selection state) → per-seller free-shipping meter → item tiles.
 *
 * The meter fires its own `/app/checkout/validate` with *only* this
 * seller's selected items so the threshold we display is the seller's
 * own setting, not a cross-seller aggregate.
 */
export function SellerGroupCard({
	group,
	groupKey,
	isLocked,
	isLoggedIn,
	selected,
	preferredMethod,
	editingQty,
	selectionBusy = false,
	onToggleGroup,
	onToggleItem,
	onChangeQty,
	onCommitQty,
	onUpdateQty,
	onSaveForLater,
	onRemove,
}: Props) {
	const selectionInputRef = React.useRef<HTMLInputElement | null>(null);
	const restoreSelectionFocusRef = React.useRef(false);
	React.useEffect(() => {
		if (!isLocked && restoreSelectionFocusRef.current) {
			selectionInputRef.current?.focus();
			restoreSelectionFocusRef.current = false;
		}
	}, [isLocked]);
	const groupAllSelected =
		!isLocked && group.items.every((it) => selected[it.id]);
	const groupSomeSelected =
		!isLocked && group.items.some((it) => selected[it.id]);
	const selectedItemCount = isLocked
		? 0
		: group.items.filter((it) => selected[it.id]).length;
	const selectedSubtotal = isLocked
		? 0
		: group.items
				.filter((it) => selected[it.id] && it.purchasable !== false)
				.reduce((s, it) => s + it.priceValue * it.quantity, 0);

	// Per-seller shipping options for this seller's selected items.
	const checkoutItems = React.useMemo(
		() =>
			group.items
				.filter(
					(it) =>
						selected[it.id] && it.purchasable !== false && it.productId != null,
				)
				.map((it) => ({
					skuId: it.skuId,
					productId: it.productId,
					quantity: it.quantity,
				})),
		[group.items, selected],
	);
	const shippingState = useShippingOptions(
		checkoutItems,
		isLoggedIn && checkoutItems.length > 0 && !isLocked,
	);
	const options = shippingState.options ?? [];
	const method: AvailableShippingOption | undefined = React.useMemo(
		() =>
			options.find((o) => o.shippingMethod === preferredMethod) ?? options[0],
		[options, preferredMethod],
	);

	const threshold = method?.freeShippingThreshold ?? 0;
	const freeHit = isFreeShippingForSubtotal(method, selectedSubtotal);
	const remain = Math.max(0, threshold - selectedSubtotal);
	const pct =
		threshold > 0
			? Math.min(100, Math.round((selectedSubtotal / threshold) * 100))
			: 0;

	const locationText = [group.sellerCity, group.sellerDistrict]
		.filter(Boolean)
		.join("");

	return (
		<section
			aria-label={`${group.sellerName}購物車`}
			className="cart-page__seller-group overflow-hidden rounded-[22px] border border-hk-border-soft bg-hk-surface-card shadow-hk-card"
		>
			<header className="cart-page__seller-header">
				<div className="cart-page__seller-identity">
					<Avatar
						src={group.sellerLogoUrl}
						alt=""
						entity="seller"
						className="cart-page__shop-icon"
						loading="eager"
						fallback="empty"
					/>
					<div className="cart-page__seller-name">
						<strong className="cart-type-subtitle">{group.sellerName}</strong>
					</div>
				</div>
				{group.sellerStoreId || group.sellerStoreCode ? (
					<Button
						href={sellerPath(group.sellerStoreCode, group.sellerStoreId)}
						variant="light"
						size="md"
						className="cart-page__shop-link"
					>
						賣家商店
					</Button>
				) : null}
				{locationText ? (
					<small className="cart-page__seller-location">
						<AppIcon name="location" size={20} />
						<span>{locationText}</span>
					</small>
				) : null}
			</header>
			<div className="cart-page__seller-controls">
				<div className="cart-page__seller-selection">
					{isLocked ? (
						<Button
							type="button"
							variant="light"
							size="md"
							className="cart-page__seller-switch"
							disabled={selectionBusy}
							onClick={() => {
								restoreSelectionFocusRef.current = true;
								onToggleGroup(groupKey, true);
							}}
						>
							改選本店商品
						</Button>
					) : (
						<label className="cart-page__seller-select">
							<input
								type="checkbox"
								className="size-[18px] accent-hk-primary-strong"
								aria-label={`選取${group.sellerName}的全部商品`}
								checked={groupAllSelected}
								disabled={selectionBusy}
								ref={(el) => {
									selectionInputRef.current = el;
									if (el)
										el.indeterminate = !groupAllSelected && groupSomeSelected;
								}}
								onChange={(e) => onToggleGroup(groupKey, e.target.checked)}
							/>
							<span>本店全選（{group.items.length} 項）</span>
						</label>
					)}
					<span
						className="cart-page__seller-summary cart-type-support"
						aria-live="polite"
					>
						{selectedItemCount > 0
							? `已選 ${selectedItemCount} 項・${ntFormat(selectedSubtotal)}`
							: `本店 ${group.items.length} 項商品`}
					</span>
				</div>
			</div>

			{!isLocked && checkoutItems.length > 0 && threshold > 0 ? (
				<div
					className={`cart-page__group-threshold${
						freeHit ? " cart-page__group-threshold--hit" : ""
					}`}
					role={freeHit ? "status" : undefined}
				>
					<div className="cart-page__group-threshold-main">
						{freeHit ? (
							<span
								className="cart-page__group-free-hit-icon"
								aria-hidden="true"
							>
								<AppIcon
									name="party"
									size={20}
									className="cart-page__group-free-hit-glyph"
								/>
							</span>
						) : null}
						<span className="cart-page__group-meter-text">
							{freeHit ? (
								<>
									已達{method?.label ?? "本店"}免運門檻
									<small>已符合免運條件</small>
								</>
							) : (
								<>
									本店再加購 <strong>{ntFormat(remain)}</strong> 即享
									{method?.label ?? "本店"}免運
									<small>（滿 {ntFormat(threshold)}）</small>
								</>
							)}
						</span>
					</div>
					<div
						className="cart-page__group-meter-bar"
						role="progressbar"
						aria-label={`${group.sellerName} 免運進度`}
						aria-valuemin={0}
						aria-valuemax={100}
						aria-valuenow={freeHit ? 100 : pct}
					>
						<span style={{ width: `${freeHit ? 100 : pct}%` }} />
					</div>
				</div>
			) : null}

			<ul className="list-none m-0 p-0 flex flex-col">
				{group.items.map((item) => (
					<CartProductTile
						key={item.id}
						item={item}
						isChecked={!!selected[item.id]}
						isLocked={isLocked}
						editingValue={editingQty[item.id]}
						onCheck={(checked) => onToggleItem(item, checked)}
						onChangeQty={(raw) => onChangeQty(item.id, raw)}
						onCommitQty={() => onCommitQty(item.id)}
						onIncrement={() => onUpdateQty(item, item.quantity + 1)}
						onDecrement={() =>
							onUpdateQty(item, Math.max(0, item.quantity - 1))
						}
						onSaveForLater={() => onSaveForLater(item)}
						onRemove={() => onRemove(item)}
					/>
				))}
			</ul>
		</section>
	);
}
