"use client";

import { TruckMark } from "@/components/shared/TruckSpinner";
import {
	FEATURE_CARD_BG,
	FEATURE_CARD_CHROME,
	FEATURE_CARD_WELL,
} from "./featureCard";

/**
 * Loading placeholder for the megamenu's right-slot banner. Shown while
 * `ListLiveBanners` is still in flight so the card area never flashes
 * empty: the delivery truck (the same mark used by `TruckOverlay`)
 * drives over a dashed road inside the same title, media and indicator chrome
 * as the loaded banner. The initial media well is 16:9; a banner with another
 * natural aspect ratio can still change height once its image metadata loads.
 *
 * The truck is coloured through the shared truck-loading tokens, so it stays
 * aligned with the light/dark design spec.
 */
export default function BannerLoadingCard({
	heading = "最新消息",
}: {
	heading?: string;
}) {
	return (
		<section
			className={`w-full ${FEATURE_CARD_CHROME} p-[var(--hkui-space-5)] flex flex-col gap-[var(--hkui-space-4)] overflow-hidden`}
			style={FEATURE_CARD_BG}
			role="status"
			aria-live="polite"
		>
			{/* No `aria-label` on the live region: it would win over the contents
			    and make the heading + status line get read twice on mount. The
			    visible 「新鮮內容運送中…」 line already says what is happening. */}
			{heading ? (
				<h3 className="m-0 text-[length:var(--text-sm)] font-bold leading-normal text-[var(--page-banner-heading-fg)] tracking-[0.5px]">
					{heading}
				</h3>
			) : null}
			<div className={`${FEATURE_CARD_WELL} aspect-[16/9]`}>
				{/* Truck bobs in place while the dashed road scrolls beneath it —
				 *  the same "driving" illusion as TruckOverlay. */}
				<span
					className="absolute bottom-3 left-1/2 -translate-x-1/2"
					aria-hidden="true"
				>
					<TruckMark size={66} />
				</span>
				<span className="absolute inset-x-3 top-3 text-center text-[length:var(--text-sm)] font-medium text-[var(--hkui-foreground-muted)]">
					新鮮內容運送中…
				</span>
				<div
					className="truck-loading__road-line absolute right-0 bottom-2 left-0 h-1 animate-truck-road opacity-60 motion-reduce:animate-none"
					aria-hidden="true"
				/>
			</div>
			<span className="flex h-2.5 items-end justify-center" aria-hidden="true">
				<span className="inline-block w-1.5 h-1.5 rounded-full bg-[var(--page-accent-marker)] animate-pulse motion-reduce:animate-none" />
			</span>
		</section>
	);
}
