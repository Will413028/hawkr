"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

interface ScrollableTableRegionProps {
	label: string;
	className?: string;
	children: React.ReactNode;
}

/**
 * A horizontally scrollable table container. It becomes a labelled region and
 * a keyboard stop only while its content actually overflows horizontally, so
 * a keyboard user can scroll the table without a pointer and a non-scrolling
 * container never leaves an empty tab stop.
 */
export function ScrollableTableRegion({
	label,
	className,
	children,
}: ScrollableTableRegionProps) {
	const ref = React.useRef<HTMLDivElement>(null);
	const [overflowing, setOverflowing] = React.useState(false);

	React.useEffect(() => {
		const node = ref.current;
		if (!node) return;
		const measure = () => setOverflowing(node.scrollWidth > node.clientWidth);
		measure();
		if (typeof ResizeObserver === "undefined") return;
		const observer = new ResizeObserver(measure);
		observer.observe(node);
		for (const child of Array.from(node.children)) observer.observe(child);
		return () => observer.disconnect();
	}, []);

	const regionProps: React.HTMLAttributes<HTMLDivElement> = overflowing
		? { role: "region", "aria-label": label, tabIndex: 0 }
		: {};

	return (
		<div
			ref={ref}
			className={cn("seller-orders__table-wrap", className)}
			{...regionProps}
		>
			{children}
		</div>
	);
}
