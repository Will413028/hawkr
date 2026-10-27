"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import type { AvailableShippingOption } from "@/lib/checkout/repo";
import {
	SHIPPING_METHOD_LABEL,
	ShippingFeePayer,
	ShippingMethod,
	type ShippingMethodValue,
} from "@/lib/codemeta/enums";
import { formatTaipeiMonthDay } from "@/lib/date/taipei";

interface Props {
	options: AvailableShippingOption[];
	selectedMethod: string;
	/**
	 * 配送溫層 of the active row. Required because TAQBIN returns three
	 * rows (常溫/冷藏/冷凍) sharing the same `shippingMethod` but with
	 * different fees — picking only by method always resolves to the
	 * first temp, so the cart summary displayed e.g. NT$205 for an
	 * NT$290 frozen pick.
	 */
	selectedDeliveryTemp: string;
	onSelect: (shippingMethod: string, deliveryTemp: string) => void;
	addressLabel?: string | null;
	storePickupLabel?: string | null;
	onPickAddress?: () => void;
	onPickStorePickup?: () => void;
	addressRequired: boolean;
	storePickupRequired: boolean;
}

function ntFormat(n: number) {
	return `NT$ ${Math.max(0, Math.round(n)).toLocaleString()}`;
}

function isSevenEleven(method: string): boolean {
	return method === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP;
}

function isHomeDelivery(method: string): boolean {
	return method === ShippingMethod.TAQBIN;
}

/** Sums backend options by `shippingMethod`. */
function groupOptions(options: AvailableShippingOption[]) {
	const grouped = new Map<string, AvailableShippingOption[]>();
	for (const o of options) {
		const arr = grouped.get(o.shippingMethod) ?? [];
		arr.push(o);
		grouped.set(o.shippingMethod, arr);
	}
	return grouped;
}

function estimateDelivery(method: string): string {
	const now = Date.now();
	let minDays = 0;
	let maxDays = 0;
	if (method === ShippingMethod.TAQBIN) {
		minDays = 1;
		maxDays = 3;
	} else if (method === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP) {
		minDays = 2;
		maxDays = 4;
	} else {
		return "";
	}
	// 「+N 天」是純毫秒位移（與時區無關），呈現才固定台北牆上時間。
	const from = formatTaipeiMonthDay(now + minDays * 86400000);
	const to = formatTaipeiMonthDay(now + maxDays * 86400000);
	return `${from} – ${to}`;
}

function MethodIcon() {
	return <AppIcon name="truck" size={20} className="shipping-group__icon" />;
}

export function ShippingMethodGroup({
	options,
	selectedMethod,
	selectedDeliveryTemp,
	onSelect,
	addressLabel,
	storePickupLabel,
	onPickAddress,
	onPickStorePickup,
	addressRequired,
	storePickupRequired,
}: Props) {
	const grouped = React.useMemo(() => groupOptions(options), [options]);
	const groupKeys = Array.from(grouped.keys());

	// 預估到貨日只在 client 掛載後計算：estimateDelivery 用 new Date()，
	// server 端 render 與 client 端 render 取到的「今天」會不同，直接算會造成
	// hydration mismatch。故 server 端一律不算、等 client 掛載後才填。
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => setMounted(true), []);

	// Row identity matches BOTH method and deliveryTemp — TAQBIN returns
	// three rows sharing the same method, so a method-only compare
	// activated all three rows (and the cart summary picked the first
	// match's fee, which is why a frozen NT$290 pick rendered as 205).
	const isRowActive = React.useCallback(
		(o: AvailableShippingOption) =>
			o.shippingMethod === selectedMethod &&
			(o.deliveryTemp ?? "") === (selectedDeliveryTemp ?? ""),
		[selectedMethod, selectedDeliveryTemp],
	);

	// First render: expand the group containing the selected method, or the first.
	const initialExpand = React.useMemo(() => {
		const own = groupKeys.find((k) =>
			(grouped.get(k) ?? []).some((o) => o.shippingMethod === selectedMethod),
		);
		const fallback = groupKeys[0] ?? "";
		return new Set([own ?? fallback].filter(Boolean));
	}, [grouped, groupKeys, selectedMethod]);
	const [expanded, setExpanded] = React.useState<Set<string>>(initialExpand);

	const toggleExpand = (key: string) => {
		setExpanded((prev) => {
			const next = new Set(prev);
			if (next.has(key)) {
				next.delete(key);
			} else {
				next.add(key);
			}
			return next;
		});
	};

	if (groupKeys.length === 0) return null;

	return (
		<div className="shipping-group">
			{groupKeys.map((methodKey) => {
				const opts = grouped.get(methodKey) ?? [];
				if (opts.length === 0) return null;
				const isExpanded = expanded.has(methodKey);
				const label =
					SHIPPING_METHOD_LABEL[methodKey as ShippingMethodValue] ??
					opts[0]?.label ??
					methodKey;
				const eta = mounted ? estimateDelivery(methodKey) : "";
				const groupHasSelected = opts.some(isRowActive);

				return (
					<section
						key={methodKey}
						className={`shipping-group__item${
							groupHasSelected ? " shipping-group__item--active" : ""
						}`}
					>
						<button
							type="button"
							className="shipping-group__header"
							onClick={() => toggleExpand(methodKey)}
							aria-expanded={isExpanded}
						>
							<MethodIcon />
							<div className="shipping-group__head-text">
								<strong>{label}</strong>
								{eta ? <small>預計配送時間：{eta}</small> : null}
							</div>
							<AppIcon
								name="chevron-down"
								size={20}
								className={`shipping-group__chev${isExpanded ? " shipping-group__chev--open" : ""}`}
							/>
						</button>

						{isExpanded ? (
							<div className="shipping-group__body">
								{opts.map((o) => {
									const active = isRowActive(o);
									return (
										<label
											key={`${o.shippingMethod}-${o.deliveryTemp ?? ""}-${o.label}`}
											className={`shipping-group__row${active ? " shipping-group__row--active" : ""}`}
										>
											<input
												type="radio"
												name="cart-shipping"
												checked={active}
												onChange={() =>
													onSelect(o.shippingMethod, o.deliveryTemp ?? "")
												}
											/>
											<span className="shipping-group__row-body">
												<strong>{o.label}</strong>
												{o.feePayer === ShippingFeePayer.SELLER &&
												o.freeShippingThreshold &&
												o.freeShippingThreshold > 0 ? (
													<small>
														滿 {ntFormat(o.freeShippingThreshold)} 免運
													</small>
												) : null}
											</span>
											<b>{o.freeShippingEligible ? "免運" : ntFormat(o.fee)}</b>
										</label>
									);
								})}

								{groupHasSelected ? (
									<div className="shipping-group__target">
										{isHomeDelivery(methodKey) && addressRequired ? (
											<AddressTarget
												label={addressLabel}
												onPick={onPickAddress}
											/>
										) : null}
										{isSevenEleven(methodKey) && storePickupRequired ? (
											<StorePickupTarget
												label={storePickupLabel}
												onPick={onPickStorePickup}
											/>
										) : null}
									</div>
								) : null}
							</div>
						) : null}
					</section>
				);
			})}
		</div>
	);
}

function AddressTarget({
	label,
	onPick,
}: {
	label?: string | null;
	onPick?: () => void;
}) {
	return (
		<button
			type="button"
			className={`shipping-group__target-row${label ? "" : " shipping-group__target-row--missing"}`}
			onClick={onPick}
		>
			<AppIcon name="location" size={20} />
			<div className="shipping-group__target-text">
				<strong>收件地址</strong>
				<span>{label || "請選擇收件地址"}</span>
			</div>
			<AppIcon name="chevron-right" size={20} />
		</button>
	);
}

function StorePickupTarget({
	label,
	onPick,
}: {
	label?: string | null;
	onPick?: () => void;
}) {
	return (
		<button
			type="button"
			className={`shipping-group__target-row${label ? "" : " shipping-group__target-row--missing"}`}
			onClick={onPick}
		>
			<AppIcon name="bag" size={20} />
			<div className="shipping-group__target-text">
				<strong>取貨門市</strong>
				<span>{label || "請選擇取貨門市"}</span>
			</div>
			<AppIcon name="chevron-right" size={20} />
		</button>
	);
}
