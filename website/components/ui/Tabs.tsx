"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

type TabsVariant = "solid" | "bordered" | "light" | "underlined";
type TabsSize = "sm" | "md" | "lg";

interface TabItem {
	key: string;
	label: React.ReactNode;
	content?: React.ReactNode;
	isDisabled?: boolean;
}

interface TabsProps {
	items: TabItem[];
	defaultSelectedKey?: string;
	selectedKey?: string;
	onSelectionChange?: (key: string) => void;
	variant?: TabsVariant;
	size?: TabsSize;
	fullWidth?: boolean;
	/** 為 true 時所有 panel 都渲染，非 active 的加 hidden。切 tab 不卸載子樹——
	 * 給那些自己持有狀態、卸載就會重置的 panel 用（例如 Hawkr Agent 的成果面板，
	 * 其內部三個視圖本來就是靠 hidden 保留 DOM 的）。 */
	keepMounted?: boolean;
	className?: string;
	"aria-label"?: string;
}

/** @import import { Tabs } from '@/components/ui/Tabs'; */
export const Tabs: React.FC<TabsProps> = ({
	items,
	defaultSelectedKey,
	selectedKey,
	onSelectionChange,
	variant = "solid",
	size = "md",
	fullWidth = false,
	keepMounted = false,
	className = "",
	"aria-label": ariaLabel = "Tabs",
}) => {
	const isControlled = selectedKey !== undefined;
	const [internal, setInternal] = React.useState<string>(
		defaultSelectedKey ?? items[0]?.key ?? "",
	);
	// `items` often arrives asynchronously (fetched tabs). The initial state was
	// computed once, so an empty first render left `internal` as "" forever —
	// activeIndex -1, and a `…-panel--1` id wired to nothing. Re-seed the moment
	// a matching key becomes available. Controlled usage is untouched.
	const hasMatch = items.some((item) => item.key === internal);
	React.useEffect(() => {
		if (isControlled || hasMatch) return;
		const fallback =
			items.find((item) => item.key === defaultSelectedKey)?.key ??
			items[0]?.key;
		if (fallback !== undefined) setInternal(fallback);
	}, [isControlled, hasMatch, items, defaultSelectedKey]);
	const idPrefix = React.useId().replaceAll(":", "");
	const tabRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
	const current = selectedKey ?? internal;

	const select = (key: string) => {
		if (!isControlled) setInternal(key);
		onSelectionChange?.(key);
	};

	const handleKey = (e: React.KeyboardEvent, idx: number) => {
		if (
			e.key !== "ArrowRight" &&
			e.key !== "ArrowLeft" &&
			e.key !== "Home" &&
			e.key !== "End"
		)
			return;
		e.preventDefault();
		let next = idx;
		if (e.key === "Home" || e.key === "End") {
			const ordered = [...items.keys()];
			if (e.key === "End") ordered.reverse();
			for (const candidateIndex of ordered) {
				if (!items[candidateIndex]?.isDisabled) {
					next = candidateIndex;
					break;
				}
			}
		} else {
			const dir = e.key === "ArrowRight" ? 1 : -1;
			for (let step = 0; step < items.length; step++) {
				next = (next + dir + items.length) % items.length;
				if (!items[next]?.isDisabled) break;
			}
		}
		const target = items[next];
		// Every candidate can be disabled (a fully-locked tab set). The loops
		// above then land back on a disabled item; selecting it would move the
		// panel to a tab the user cannot activate with the mouse.
		if (target?.isDisabled) return;
		if (target) {
			select(target.key);
			tabRefs.current[next]?.focus();
		}
	};

	const activePanel = items.find((item) => item.key === current);
	const activeIndex = items.findIndex((item) => item.key === current);

	return (
		<div className={cn("w-full flex flex-col font-sans", className)}>
			<div
				role="tablist"
				aria-orientation="horizontal"
				aria-label={ariaLabel}
				className={cn(
					"relative inline-flex items-center",
					tabsListVariantClass[variant],
					fullWidth && "w-full",
				)}
			>
				{items.map((item, idx) => {
					const selected = item.key === current;
					return (
						<button
							key={item.key}
							type="button"
							role="tab"
							id={`${idPrefix}-tab-${idx}`}
							aria-controls={`${idPrefix}-panel-${idx}`}
							aria-selected={selected}
							data-selected={selected}
							tabIndex={selected ? 0 : -1}
							disabled={item.isDisabled}
							ref={(node) => {
								tabRefs.current[idx] = node;
							}}
							className={cn(
								"relative inline-flex items-center gap-2 bg-transparent border-none cursor-pointer font-medium text-hk-body transition-colors duration-200 motion-reduce:transition-none",
								// Inset the ring: the global 2px outline sits
								// OUTSIDE the tab and was clipped by the `p-1`
								// list container (and overlapped its neighbour).
								"focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-[var(--hkui-focus)]",
								"disabled:opacity-50 disabled:cursor-not-allowed",
								tabsSizeClass[size],
								tabsTabVariantClass[variant],
								fullWidth && "flex-1 justify-center",
							)}
							onClick={() => select(item.key)}
							onKeyDown={(e) => handleKey(e, idx)}
						>
							{item.label}
						</button>
					);
				})}
			</div>
			{keepMounted
				? items.map((item, idx) =>
						item.content === undefined ? null : (
							<div
								key={item.key}
								role="tabpanel"
								id={`${idPrefix}-panel-${idx}`}
								aria-labelledby={`${idPrefix}-tab-${idx}`}
								className="pt-5"
								hidden={item.key !== current}
							>
								{item.content}
							</div>
						),
					)
				: activePanel?.content !== undefined && (
						<div
							role="tabpanel"
							id={`${idPrefix}-panel-${activeIndex}`}
							aria-labelledby={`${idPrefix}-tab-${activeIndex}`}
							className="pt-5"
						>
							{activePanel.content}
						</div>
					)}
		</div>
	);
};

// md is the default and clears 44px outright; sm stays dense on a mouse and
// only grows on coarse pointers.
const tabsSizeClass: Record<TabsSize, string> = {
	sm: "h-8 px-3 text-xs rounded-hk-sm pointer-coarse:min-h-11",
	md: "h-11 px-4 text-sm rounded-hk-md",
	lg: "h-12 px-5 text-base rounded-hk-md",
};

// Adjacent tabs used to butt up against each other (gap-0), so a mistap on the
// seam always hit *a* tab — just not the intended one.
const tabsListVariantClass: Record<TabsVariant, string> = {
	solid: "bg-hk-surface-muted p-1 rounded-hk-md gap-1.5",
	bordered: "border border-hk-border-soft p-1 rounded-hk-md gap-1.5",
	light: "gap-2",
	underlined: "border-b border-hk-border-soft rounded-none gap-4 p-0",
};

const tabsTabVariantClass: Record<TabsVariant, string> = {
	solid:
		"data-[selected=true]:bg-hk-surface-elevated data-[selected=true]:text-hk-heading data-[selected=true]:shadow-hk-sm",
	bordered:
		"data-[selected=true]:bg-hk-surface-muted data-[selected=true]:text-hk-heading",
	light:
		"hover:text-hk-heading data-[selected=true]:text-hk-accent-on-tint data-[selected=true]:bg-hk-primary/10",
	underlined:
		"rounded-none px-0 data-[selected=true]:text-hk-primary after:absolute after:left-0 after:right-0 after:-bottom-[1px] after:h-[2px] after:rounded-full after:bg-hk-primary after:opacity-0 after:content-[''] data-[selected=true]:after:opacity-100",
};

export type { TabItem, TabsProps, TabsSize, TabsVariant };
