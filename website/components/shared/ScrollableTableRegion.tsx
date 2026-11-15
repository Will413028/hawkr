import type * as React from "react";
import { cn } from "@/lib/utils/cn";

interface ScrollableTableRegionProps {
	label: string;
	className?: string;
	children: React.ReactNode;
}

/**
 * A horizontally scrollable table container. It is a labelled region and a
 * keyboard stop so a keyboard user can scroll the table without a pointer.
 */
export function ScrollableTableRegion({
	label,
	className,
	children,
}: ScrollableTableRegionProps) {
	return (
		<section
			className={cn("seller-orders__table-wrap", className)}
			aria-label={label}
			// biome-ignore lint/a11y/noNoninteractiveTabindex: This horizontally scrollable table needs a keyboard focus target.
			tabIndex={0}
		>
			{children}
		</section>
	);
}
