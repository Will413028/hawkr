/**
 * Stat card — icon + big value + label + optional change indicator.
 *
 * Used across the seller-center dashboard, inventory, and customers
 * pages. The CSS hooks (.seller-stat / .seller-inventory__stat /
 * .seller-customers__stats > div) are similar enough that this single
 * atom can absorb them as we migrate consumers.
 *
 * For now the atom emits the .seller-stat shape; consumers using the
 * other class names can either keep their JSX inline or override
 * `className` to plug into a different visual.
 */

import type * as React from "react";

export interface StatCardProps {
	/** Inline-rendered icon — usually a small svg. */
	icon?: React.ReactNode;
	/** Headline value (already formatted, e.g. "NT$ 12,800" or "184"). */
	value: React.ReactNode;
	/** Label shown below the value. */
	label: string;
	/** Trailing growth / context line (e.g. "較前 7 日 +12%" or "近 7 日"). */
	change?: React.ReactNode;
	/**
	 * When the change indicator is informational rather than directional
	 * (e.g. "近 7 日" instead of "+5%"), set this so the styles render in a
	 * neutral colour rather than positive/negative green/red.
	 */
	changeNeutral?: boolean;
	/** Period the value covers (e.g. "9/30–10/6"), shown under the change line. */
	caption?: string;
	/** Override the wrapper class — defaults to `.seller-stat`. */
	className?: string;
}

export function StatCard({
	icon,
	value,
	label,
	change,
	changeNeutral,
	caption,
	className = "seller-stat",
}: StatCardProps) {
	return (
		<div className={className}>
			{icon ? <div className="seller-stat__icon">{icon}</div> : null}
			<div className="seller-stat__value">{value}</div>
			<div className="seller-stat__label">{label}</div>
			{change !== undefined && change !== null && change !== "" ? (
				<div
					className={
						changeNeutral
							? "seller-stat__change seller-stat__change--neutral"
							: "seller-stat__change"
					}
				>
					{change}
				</div>
			) : null}
			{caption ? (
				<div className="truncate text-xs text-[var(--hkui-foreground-muted)]">
					{caption}
				</div>
			) : null}
		</div>
	);
}
