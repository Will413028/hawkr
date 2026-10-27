/**
 * "The item went *there*" — the add-to-cart confirmation.
 *
 * A toast tells you an add succeeded; it does not tell you *where* the thing
 * landed, so the cart badge silently ticks up somewhere the buyer isn't
 * looking. Flying the added quantity from the button into the cart icon makes
 * the destination part of the confirmation, and lands right as the badge
 * increments so the two read as one event.
 *
 * Built on the Web Animations API rather than a motion library: this runs on a
 * tap, from a component that is already on the critical path of every product
 * card, and it must not pull a runtime into that bundle. Everything below is
 * compositor-only (`transform` / `opacity`).
 */

/** Marks the cart icon that added items should fly into. */
export const CART_FLY_TARGET_ATTR = "data-cart-fly-target";

/** Marks the count bubble on that icon, so it can pop as the chip lands. */
export const CART_FLY_BADGE_ATTR = "data-cart-fly-badge";

const FLIGHT_MS = 900;
/**
 * The chip is born on the button and holds there for this share of the flight
 * before departing. Without it the eye has nothing to lock onto: the chip
 * appears already in motion, and a moving object that was never stationary
 * reads as a flicker rather than as *the thing you just added*.
 */
const LIFTOFF_OFFSET = 0.16;
/** The badge pops just before the chip fully lands — they read as one beat. */
const LANDING_LEAD_MS = 110;

function prefersReducedMotion(): boolean {
	return (
		typeof window !== "undefined" &&
		window.matchMedia("(prefers-reduced-motion: reduce)").matches
	);
}

function visibleRect(el: Element): DOMRect | null {
	const rect = el.getBoundingClientRect();
	if (rect.width <= 0 || rect.height <= 0) return null;
	return rect;
}

/**
 * The cart icon renders in more than one shell (desktop rail, compact mobile
 * header). Only one is laid out at a time, so pick the first that actually has
 * a box — `display: none` gives a zero rect, not a missing element.
 */
function findCartTargetRect(): DOMRect | null {
	const candidates = document.querySelectorAll(`[${CART_FLY_TARGET_ATTR}]`);
	for (const el of candidates) {
		const rect = visibleRect(el);
		if (rect) return rect;
	}
	return null;
}

/** Pop the count bubble. Called on landing, and on its own when motion is reduced. */
export function bumpCartBadge(): void {
	const badge = document.querySelector(`[${CART_FLY_BADGE_ATTR}]`);
	if (!(badge instanceof HTMLElement)) return;
	if (prefersReducedMotion()) return;
	badge.animate(
		[
			{ transform: "scale(1)" },
			{ transform: "scale(1.38)", offset: 0.42 },
			{ transform: "scale(1)" },
		],
		{
			duration: 420,
			// Overshoot past 1 and settle — the badge lands like a physical object
			// rather than easing politely into place.
			easing: "cubic-bezier(0.34, 1.56, 0.64, 1)",
		},
	);
}

/**
 * Fly `quantity` from `origin` into the cart icon.
 *
 * Every failure mode here is a no-op by design: this is decoration on top of an
 * add that already succeeded, so a missing target, a hidden icon, or a browser
 * without WAAPI must never surface as an error.
 */
export function flyToCart(origin: Element | null, quantity: number): void {
	if (typeof window === "undefined" || typeof document === "undefined") return;
	if (!origin || quantity < 1) return;

	if (prefersReducedMotion()) {
		bumpCartBadge();
		return;
	}

	const originRect = visibleRect(origin);
	const targetRect = findCartTargetRect();
	if (!originRect || !targetRect) {
		bumpCartBadge();
		return;
	}

	const from = {
		x: originRect.left + originRect.width / 2,
		y: originRect.top + originRect.height / 2,
	};
	const to = {
		x: targetRect.left + targetRect.width / 2,
		y: targetRect.top + targetRect.height / 2,
	};

	// A single element can only ease one way. Splitting the axes across a
	// wrapper (x) and a child (y) lets each carry its own easing, and an
	// ease-out y against an ease-in x *is* the arc — no path keyframes, no
	// per-frame JS. The chip leaps off the button and decelerates while the
	// horizontal travel has barely started, so it reads as "up, then across
	// into the cart".
	//
	// Both axes stay monotonic between the endpoints on purpose. An explicit
	// apex past `min(0, dy)` was tried first and is wrong: the cart icon
	// already sits near the top of the viewport, so arcing *above* it threw the
	// chip off-screen — measured apex y = -72 on a 500px-wide product page.
	// There is no room above the target to arc into.
	const dy = to.y - from.y;

	const carrier = document.createElement("div");
	carrier.style.cssText = [
		"position:fixed",
		"left:0",
		"top:0",
		`transform:translate3d(${from.x}px, ${from.y}px, 0)`,
		"pointer-events:none",
		"z-index:var(--z-toast, 90)",
		"will-change:transform",
	].join(";");
	carrier.setAttribute("aria-hidden", "true");

	const chip = document.createElement("div");
	chip.textContent = quantity > 99 ? "99+" : `+${quantity}`;
	chip.style.cssText = [
		"display:flex",
		"align-items:center",
		"justify-content:center",
		"min-width:26px",
		"height:26px",
		"padding:0 8px",
		"margin:-13px 0 0 -13px",
		"border-radius:999px",
		"background:var(--hkui-danger, #ea6021)",
		"color:var(--hkui-danger-foreground, #fff)",
		"font:700 13px/1 var(--hkui-font-sans, system-ui)",
		"letter-spacing:0.2px",
		"box-shadow:0 8px 20px -6px rgba(0,0,0,0.45)",
		"will-change:transform, opacity",
	].join(";");

	carrier.appendChild(chip);
	document.body.appendChild(carrier);

	const cleanup = () => {
		carrier.remove();
	};

	try {
		const horizontal = carrier.animate(
			[
				{ transform: `translate3d(${from.x}px, ${from.y}px, 0)` },
				{
					transform: `translate3d(${from.x}px, ${from.y}px, 0)`,
					offset: LIFTOFF_OFFSET,
					// One smooth cubic for the whole sweep. The gap between this
					// easing and the vertical one *is* the bow, so keeping the two
					// close keeps the path shallow — and keeping each axis to a
					// single cubic keeps it one continuous curve.
					easing: "cubic-bezier(0.55, 0, 0.3, 1)",
				},
				{ transform: `translate3d(${to.x}px, ${from.y}px, 0)` },
			],
			{ duration: FLIGHT_MS },
		);

		chip.animate(
			[
				// Born small on the button, so there is a frame where it is clearly
				// *at* the button before it is anywhere else.
				{
					transform: "translate3d(0, 0, 0) scale(0.55)",
					opacity: 0,
					easing: "cubic-bezier(0.34, 1.56, 0.64, 1)",
				},
				{
					transform: "translate3d(0, 0, 0) scale(1.12)",
					opacity: 1,
					offset: LIFTOFF_OFFSET,
					// A single cubic from liftoff to landing. An intermediate
					// keyframe at 62% used to sit here to hang at the arc's peak,
					// but two easings meeting mid-path is a corner: the chip visibly
					// stalled (59% → 63% of the vertical travel over 170ms) and then
					// re-accelerated, which reads as a kink rather than as a throw.
					//
					// Not easeOutExpo either — expo put ~70% of the vertical travel
					// into the first 80ms, so the flight was nominally 620ms and
					// visually instant.
					easing: "cubic-bezier(0.42, 0, 0.25, 1)",
				},
				{ transform: `translate3d(0, ${dy}px, 0) scale(0.42)`, opacity: 0 },
			],
			{ duration: FLIGHT_MS },
		);

		window.setTimeout(bumpCartBadge, FLIGHT_MS - LANDING_LEAD_MS);
		horizontal.finished.then(cleanup, cleanup);
	} catch {
		// No WAAPI (or an animation the engine refused): drop the chip rather
		// than leaving it parked over the page.
		cleanup();
		bumpCartBadge();
	}
}
