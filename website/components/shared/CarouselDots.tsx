"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

export interface CarouselDotsProps {
	/** Number of slides. */
	count: number;
	/** Zero-based index of the visible slide. */
	active: number;
	onSelect: (index: number) => void;
	/** Accessible name of the tablist itself, e.g. "banner 切換". */
	label: string;
	/** Accessible name of one dot, e.g. `第 ${i + 1} 張：${title}`. */
	dotLabel: (index: number) => string;
	/** `id` of the slide region the tabs drive (`aria-controls`). */
	controls: string;
	/** Stable React key per dot; falls back to the index. */
	dotKey?: (index: number) => React.Key;
	className?: string;
}

/**
 * Shared carousel indicator row for the banner and megamenu carousels, which
 * previously each carried their own copy of the same 6×6px dots.
 *
 * Three things the copies got wrong and this one does not:
 *   - **Target size**: the dot stays 6px (the design), but each button carries
 *     a 44×44 hit area through a `before:` pseudo-element, so touch works.
 *   - **Keyboard**: a `role="tablist"` without arrow keys is a lie. Roving
 *     tabindex + Arrow/Home/End makes the tab list behave as announced, and
 *     `aria-controls` finally points at the slide region.
 *   - **Motion**: the old rule transitioned `width`, i.e. animated layout on
 *     every rotation. The width now swaps instantly; only colour transitions.
 */
export default function CarouselDots({
	count,
	active,
	onSelect,
	label,
	dotLabel,
	controls,
	dotKey,
	className,
}: CarouselDotsProps) {
	const listRef = React.useRef<HTMLDivElement | null>(null);

	const focusDot = (index: number) => {
		const next = ((index % count) + count) % count;
		onSelect(next);
		listRef.current
			?.querySelectorAll<HTMLButtonElement>('[role="tab"]')
			.item(next)
			?.focus();
	};

	const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
		if (event.key === "ArrowRight" || event.key === "ArrowDown") {
			event.preventDefault();
			focusDot(active + 1);
		} else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
			event.preventDefault();
			focusDot(active - 1);
		} else if (event.key === "Home") {
			event.preventDefault();
			focusDot(0);
		} else if (event.key === "End") {
			event.preventDefault();
			focusDot(count - 1);
		}
	};

	if (count < 2) return null;

	return (
		<div
			ref={listRef}
			role="tablist"
			aria-label={label}
			className={cn("flex justify-center gap-2 self-center", className)}
			onKeyDown={onKeyDown}
		>
			{Array.from({ length: count }, (_, index) => (
				<button
					key={dotKey ? dotKey(index) : index}
					type="button"
					role="tab"
					aria-selected={index === active}
					aria-controls={controls}
					aria-label={dotLabel(index)}
					tabIndex={index === active ? 0 : -1}
					data-active={index === active}
					// `before:-inset-[19px]` grows the 6px dot's hit area to 44×44
					// without changing a single painted pixel; `relative` is what
					// the pseudo-element anchors to.
					className={cn(
						"relative h-1.5 cursor-pointer rounded-full border-0 p-0",
						"transition-colors duration-[220ms] ease-[var(--hkui-ease)] motion-reduce:transition-none",
						"before:absolute before:-inset-[19px] before:content-['']",
						"focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]",
						index === active
							? "w-5 bg-[var(--page-accent-marker)]"
							: "w-1.5 bg-[var(--page-carousel-dot-idle)] hover:bg-[var(--page-carousel-dot-hover)]",
					)}
					onClick={() => onSelect(index)}
				/>
			))}
		</div>
	);
}
