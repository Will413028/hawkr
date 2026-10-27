"use client";

import * as React from "react";
import {
	TEXT_SCALE_LABELS,
	TEXT_SCALES,
	useTextScale,
} from "@/lib/theme/text-scale";
import { cn } from "@/lib/utils/cn";

/**
 * Segmented 小 / 中 / 大 picker for 設定.
 *
 * Visually this is `components/ui/Tabs` in its `solid` variant at size `md` —
 * the class strings below are lifted verbatim from `tabsListVariantClass.solid`
 * / `tabsSizeClass.md` / `tabsTabVariantClass.solid` so the two cannot look
 * different.
 *
 * It is NOT built ON Tabs, because tab semantics would be wrong: nothing here
 * shows or hides a panel. Choosing one of three mutually exclusive settings is
 * a radio group, so that is what this is — real `<input type="radio">`
 * controls behind `sr-only`, which is how the rest of the repo builds pill
 * choosers (see `SellerPricingPage`'s plan cards). Native radios bring the
 * whole APG keyboard contract with them: one tab stop for the group, arrows
 * that wrap and select, Home/End — none of it re-implemented, none of it able
 * to drift from the platform.
 */
export default function TextScaleControl({
	className = "",
}: {
	className?: string;
}) {
	const { scale, setScale } = useTextScale();
	// Two of these can be on one page (設定 plus anything embedding it); a
	// shared `name` would merge them into a single radio group.
	const groupName = `${React.useId()}-text-scale`;

	return (
		<div
			role="radiogroup"
			aria-label="字體大小"
			className={cn(
				"relative inline-flex items-center",
				"bg-hk-surface-muted p-1 rounded-hk-md gap-1.5",
				className,
			)}
		>
			{TEXT_SCALES.map((option) => {
				const selected = option === scale;
				return (
					<label
						key={option}
						data-selected={selected}
						className={cn(
							"relative inline-flex items-center justify-center gap-2 cursor-pointer font-medium text-hk-muted transition-colors duration-200 motion-reduce:transition-none",
							// 44px touch target, matching Tabs' md size.
							"h-11 min-w-11 px-4 text-sm rounded-hk-md",
							// The ring follows the visually-hidden input's focus, and is
							// inset: the global 2px outline sits OUTSIDE the pill and
							// would be clipped by the `p-1` container.
							"has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-[-2px] has-[:focus-visible]:outline-[var(--hkui-focus)]",
							"data-[selected=true]:bg-hk-surface-elevated data-[selected=true]:text-hk-heading data-[selected=true]:shadow-hk-sm",
						)}
					>
						<input
							type="radio"
							name={groupName}
							value={option}
							checked={selected}
							onChange={() => setScale(option)}
							className="sr-only"
						/>
						{TEXT_SCALE_LABELS[option]}
					</label>
				);
			})}
		</div>
	);
}
