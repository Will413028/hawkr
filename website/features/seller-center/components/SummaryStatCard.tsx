import type * as React from "react";

export type SummaryStatTint = "primary" | "accent" | "info" | "secondary";

/** Return the next index for a single-row tablist, including edge wrapping. */
export function nextSummaryTabIndex(
	key: string,
	currentIndex: number,
	tabCount: number,
): number | null {
	if (tabCount <= 0 || currentIndex < 0 || currentIndex >= tabCount)
		return null;

	switch (key) {
		case "ArrowRight":
			return (currentIndex + 1) % tabCount;
		case "ArrowLeft":
			return (currentIndex - 1 + tabCount) % tabCount;
		case "Home":
			return 0;
		case "End":
			return tabCount - 1;
		default:
			return null;
	}
}

/**
 * One stat tile in the workspace summary row (orders / inventory / etc).
 *
 * Static usage: pass `value`, renders as a plain `<div>`.
 *
 * Clickable usage (e.g. SellerOrdersPage tab-as-card pattern): pass
 * `onClick` to turn the tile into a `<button role="tab">`, and use
 * `active` to highlight the current selection. Values stay mounted for every
 * tile so count refetches and tab changes do not resize the summary row.
 */
export function SummaryStatCard({
	label,
	value,
	tint,
	icon,
	active,
	onClick,
	tabIndex,
}: {
	label: string;
	value?: string;
	tint: SummaryStatTint;
	icon: React.ReactNode;
	active?: boolean;
	onClick?: () => void;
	tabIndex?: 0 | -1;
}) {
	const className = [
		"seller-inventory__stat",
		onClick ? "is-clickable" : "",
		active ? "is-active" : "",
	]
		.filter(Boolean)
		.join(" ");

	const inner = (
		<>
			<span className={`seller-inventory__stat-icon is-${tint}`}>{icon}</span>
			{/* ` ` keeps the line-height when the card has no value to show. */}
			<strong>{value || " "}</strong>
			<small>{label}</small>
		</>
	);

	if (onClick) {
		return (
			<button
				type="button"
				className={className}
				role="tab"
				aria-selected={active}
				tabIndex={tabIndex}
				onClick={onClick}
			>
				{inner}
			</button>
		);
	}

	return <div className={className}>{inner}</div>;
}
