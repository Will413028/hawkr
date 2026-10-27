"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

/**
 * The shared panel card. Triggers swap `activeItem` while the panel stays
 * open, so the card's box changes size between contents.
 *
 * It used to animate that with `transition-[width,height]` driven by a
 * ResizeObserver→setState loop: layout thrashing by construction (every frame
 * of the morph invalidated the layout that the observer then re-measured), and
 * the two animated properties are exactly the two you must never animate. The
 * size transition is gone — the card is `w-fit` and takes its content's box
 * immediately — and the content swap is carried by an opacity crossfade on the
 * inner div instead, which composites.
 *
 * The inner div is `w-max` (not `w-fit`) on desktop so a wide panel is not
 * capped by a narrower sibling's intrinsic width.
 */

const SURFACE_LAYOUT =
	"pointer-events-auto w-fit max-w-[calc(100vw-32px)] mt-2 overflow-hidden";

const SURFACE_CHROME =
	"bg-[var(--page-mega-surface-bg)] bg-[image:var(--page-mega-surface-image)] border border-[var(--page-mega-surface-border)] rounded-2xl shadow-[var(--page-mega-surface-shadow)]";

const SURFACE_CHROME_DARK =
	"dark:bg-none dark:backdrop-blur-[28px] dark:backdrop-saturate-[1.4] dark:text-(--hk-text-body)";

/** ≤768px the card goes full-bleed: no radius, no side/top border, no shadow. */
const SURFACE_MOBILE =
	"max-[768px]:mt-0 max-[768px]:max-w-full max-[768px]:w-full max-[768px]:rounded-none max-[768px]:border-x-0 max-[768px]:border-t-0 max-[768px]:shadow-none";

const SURFACE_INNER =
	"w-max max-w-[calc(100vw-32px)] p-8 pb-10 flex items-start gap-8 max-[1024px]:gap-6 max-[1024px]:p-6 max-[1024px]:px-5 [@media(max-width:840px)]:w-auto [@media(max-width:840px)]:max-w-full [@media(max-width:840px)]:flex-col max-[768px]:w-full max-[768px]:p-4 max-[768px]:gap-4";

const SURFACE_INNER_FADE =
	"transition-opacity duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] motion-reduce:transition-none";

export default function MegaMenuSurface({
	children,
	contentKey,
	onMouseEnter,
	onMouseLeave,
	onFocus,
}: {
	children: React.ReactNode;
	/** Changes when the rendered panel changes; drives the crossfade. */
	contentKey?: string;
	onMouseEnter: () => void;
	onMouseLeave: () => void;
	onFocus: () => void;
}) {
	// Start each content at 0 and raise it on the next frame, so a swap reads as
	// a crossfade rather than a hard cut. No layout is touched.
	const [shown, setShown] = React.useState(false);
	React.useEffect(() => {
		void contentKey; // the swap itself is the trigger; nothing else is read
		setShown(false);
		const frame = requestAnimationFrame(() => setShown(true));
		return () => cancelAnimationFrame(frame);
	}, [contentKey]);

	return (
		// biome-ignore lint/a11y/noStaticElementInteractions: hover/focus shell keeps the menu open only while the pointer is on the visible card.
		<div
			className={cn(
				SURFACE_LAYOUT,
				SURFACE_CHROME,
				SURFACE_CHROME_DARK,
				SURFACE_MOBILE,
			)}
			onMouseEnter={onMouseEnter}
			onMouseLeave={onMouseLeave}
			onFocus={onFocus}
		>
			<div
				className={cn(
					SURFACE_INNER,
					SURFACE_INNER_FADE,
					shown ? "opacity-100" : "opacity-0",
				)}
			>
				{children}
			</div>
		</div>
	);
}
