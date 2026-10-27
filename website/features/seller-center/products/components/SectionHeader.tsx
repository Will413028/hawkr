import type * as React from "react";

/**
 * Consistent section header for the product form — aligned to the seller
 * coupon form's "N · 標題" rhythm (see CouponFormClient). Renders a numbered
 * title + helper line with an optional right-aligned action slot (e.g. the
 * SKU editor's 新增規格 button). The parent <section> supplies `flex flex-col
 * gap-4`, so this header carries no bottom margin of its own.
 */
export function SectionHeader({
	step,
	title,
	desc,
	action,
}: {
	/** Optional leading number. Omitted in the two-column layout where
	 * sections no longer read as a single 1→N sequence. */
	step?: number;
	title: string;
	desc: React.ReactNode;
	action?: React.ReactNode;
}) {
	return (
		<header className="flex items-start justify-between gap-3 flex-wrap">
			<div className="flex flex-col gap-1 min-w-0">
				<h2 className="text-lg font-bold m-0 tracking-tight text-hk-heading">
					{step ? `${step} · ${title}` : title}
				</h2>
				<p className="m-0 text-sm text-hk-muted leading-relaxed">{desc}</p>
			</div>
			{action}
		</header>
	);
}
