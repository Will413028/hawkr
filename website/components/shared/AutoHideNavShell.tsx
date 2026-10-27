"use client";

import * as React from "react";
import { useAutoHideNav } from "@/lib/hooks/useAutoHideNav";

/**
 * Shell that wraps the site navbar in an auto-hide container + a top
 * reveal zone, and exposes a `pin()` API to children via context so the
 * MegaMenu can keep the bar shown while its panel is open.
 *
 * Markup:
 *   <div class="auto-nav …" data-hidden=…>
 *     <slot/>                        ← actual <nav>
 *   </div>
 *   <div aria-hidden … />            ← inert fixed reveal strip at top of viewport
 *
 * The reveal strip is rendered separately so it stays in place even when the
 * wrapper is translated off-screen. It is `pointer-events-none`: the reveal is
 * decided by useAutoHideNav's document-level pointer listener, so the strip
 * never has to receive an event — and must not intercept one.
 */

interface NavPinContextValue {
	pin: () => () => void;
}

const NavPinContext = React.createContext<NavPinContextValue | null>(null);

/** Children that need to keep the nav visible (e.g. mega menu) call this. */
export function useNavPin(): NavPinContextValue["pin"] | null {
	const ctx = React.useContext(NavPinContext);
	return ctx?.pin ?? null;
}

export default function AutoHideNavShell({
	autoHide = true,
	children,
}: {
	autoHide?: boolean;
	children: React.ReactNode;
}) {
	// Threads-style tuning: reveal/hide reacts to small, deliberate flicks and
	// holds visible only briefly so the bar feels responsive rather than sticky.
	const { hidden, pin } = useAutoHideNav({
		disabled: !autoHide,
		topThreshold: 56, // hide a touch sooner after leaving the very top
		directionDelta: 4, // smaller delta → flips on a lighter up/down flick
		minVisibleMs: 160, // shorter visible-hold → snappier, still no rubber-band
	});
	const wrapperRef = React.useRef<HTMLDivElement>(null);

	// Keep visible while hovering the navbar itself.
	const releaseRef = React.useRef<(() => void) | null>(null);
	const onMouseEnter = React.useCallback(() => {
		releaseRef.current?.();
		releaseRef.current = pin();
	}, [pin]);
	const onMouseLeave = React.useCallback(() => {
		releaseRef.current?.();
		releaseRef.current = null;
	}, []);
	React.useEffect(() => () => releaseRef.current?.(), []);

	// Keyboard equivalent of the hover pin. Without it a keyboard user could
	// tab into the bar and have it scroll itself away underneath them; the
	// pointer path had been pinned since day one.
	const onBlurCapture = React.useCallback(
		(event: React.FocusEvent<HTMLDivElement>) => {
			const next = event.relatedTarget as Node | null;
			if (next && event.currentTarget.contains(next)) return;
			releaseRef.current?.();
			releaseRef.current = null;
		},
		[],
	);

	const ctx = React.useMemo(() => ({ pin }), [pin]);

	return (
		<NavPinContext.Provider value={ctx}>
			{/* `auto-nav` is retained as a hook only (descendant position:relative
			 *  in floating-navigation.css + seller-center.css padding/display +
			 *  navigation-final.css dark bg); its own styling now lives here.
			 *  --anv-hide is the off-screen travel: -100% fully clears the flat
			 *  full-width mobile bar, -120% adds slack for the desktop pill. */}
			{/* biome-ignore lint/a11y/noStaticElementInteractions: hover-only wrapper keeps the shared navbar visible while pointer users traverse mega-menu content. */}
			<div
				ref={wrapperRef}
				className="auto-nav sticky top-0 z-[200] pt-[var(--floating-nav-top-gap)] will-change-transform [transform:translate3d(0,0,0)] [--anv-hide:-120%] max-[768px]:[--anv-hide:-100%] data-[hidden=true]:[transform:translate3d(0,var(--anv-hide),0)] data-[hidden=true]:pointer-events-none [transition:transform_var(--hkui-dur-slow)_var(--hkui-ease)] max-[768px]:[transition:transform_var(--hkui-dur-base)_var(--hkui-ease)] motion-reduce:[transition:none]"
				data-hidden={hidden ? "true" : "false"}
				// Off-screen chrome must leave the tab order too. `pointer-events`
				// alone only stopped the mouse: Tab still walked into a bar
				// translated 120% above the viewport with no way to see it.
				inert={hidden}
				onMouseEnter={onMouseEnter}
				onMouseLeave={onMouseLeave}
				onFocusCapture={onMouseEnter}
				onBlurCapture={onBlurCapture}
			>
				{children}
			</div>
			{autoHide ? (
				/* Top reveal strip — a LAYOUT marker only. `pointer-events-none`
				 * because the reveal is driven by useAutoHideNav's document-level
				 * mousemove/pointermove listener (`revealZone`, default 64px — the
				 * same number as this strip's height), not by this element. Without
				 * that it was a 64px invisible band that swallowed every click near
				 * the top of the page whenever the bar was hidden. */
				<div
					className="fixed inset-x-0 top-0 z-[199] h-16 bg-transparent pointer-events-none max-[768px]:h-6"
					aria-hidden="true"
				/>
			) : null}
		</NavPinContext.Provider>
	);
}
