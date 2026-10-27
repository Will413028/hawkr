"use client";

import * as React from "react";
import { ossImageUrl } from "@/lib/media/cdn";
import { placeholders } from "@/lib/media/public-assets";
import { cn } from "@/lib/utils/cn";

type AvatarEntity = "buyer" | "seller";

type AvatarProps = {
	/** Raw avatar URL from the backend. May be relative CDN path, absolute URL, empty, or the literal strings "null"/"undefined"/"none". */
	src?: string | null;
	/** Display name. When provided and the image is missing/broken, the first character is rendered as the fallback initial. */
	name?: string | null;
	/** Image alt text. Defaults to `name`, then empty string. */
	alt?: string;
	/** Identifies buyer vs seller behavior. Seller logos never use a fallback. */
	entity?: AvatarEntity;
	/** Class applied to the rendered <img> AND to the fallback element so callers can keep one CSS rule for size/shape. */
	className?: string;
	/** Extra class merged onto the fallback element only (e.g. `foo__avatar--fallback`). */
	fallbackClassName?: string;
	/** Override the default fallback strategy for buyer avatars. Seller logos
	 * always render empty when absent/broken so no client placeholder can mask
	 * a missing canonical `logo_url` / `seller_logo_url`. */
	fallback?: "initial" | "placeholder" | "empty";
	/** Defaults to `"lazy"` — an avatar is never an LCP candidate, and a long
	 * buyer/seller list used to open every request at once. Pass `"eager"` for
	 * the handful that are visible before any scroll (a profile hero). */
	loading?: "eager" | "lazy";
	/** Passed straight to the <img>. `low` keeps an eager avatar out of React 19's
	 * automatic SSR image preloads (it is never an LCP candidate). */
	fetchPriority?: "high" | "low" | "auto";
	/** Rendered box in CSS px. Sizing still comes from `className`; this only
	 * corrects the intrinsic width/height pair the browser reserves, which was
	 * hard-coded to 80×80 regardless of the real slot. */
	size?: number;
};

function normalize(src: string | null | undefined): string {
	const trimmed = src?.trim() ?? "";
	if (!trimmed) return "";
	const lower = trimmed.toLowerCase();
	if (lower === "null" || lower === "undefined" || lower === "none") return "";
	return ossImageUrl(trimmed, "avatar");
}

/**
 * The buyer placeholder painted as a CSS layer on the <img> itself, so it shows
 * through for as long as the real avatar has no pixels — and disappears under
 * them the moment it does.
 *
 * It has to be a background rather than a sibling element: `className` is what
 * every caller sizes the avatar with, and a wrapper would take that sizing away
 * from the element the callers' CSS actually targets (`.hkui-seller-profile__
 * avatar img`, `.seller-business-card__avatar > span`, flex children carrying
 * `shrink-0`). One element in, one element out.
 */
const PLACEHOLDER_LAYER: React.CSSProperties = {
	backgroundImage: `url(${placeholders.buyerAvatar})`,
	backgroundSize: "cover",
	backgroundPosition: "center",
	backgroundRepeat: "no-repeat",
};

/**
 * Unified avatar renderer for buyers and sellers.
 *
 * Resolves backend CDN paths through `ossImageUrl`, normalises sentinel strings
 * ("null"/"undefined"/"none") to empty. Buyer avatars may use an initial or
 * buyer placeholder; seller logos render an empty slot when the canonical URL
 * is absent or broken.
 *
 * Renders ONLY the inner element (img/span). Wrappers like
 * `.seller-hero__avatar` stay in the caller's JSX so existing layouts and
 * borders keep working.
 *
 * Lifecycle: once there is a src, the <img> is the rendered element and stays
 * mounted for as long as the request is alive — slowness is never treated as
 * failure. The fallback is a layer beneath it (`PLACEHOLDER_LAYER`) rather than
 * a replacement for it. The fallback ELEMENT is reached in exactly two cases:
 * there was never a usable src, or the browser reported a real `error`.
 *
 * The `initial` strategy is the one fallback with no under-layer: a glyph
 * cannot be painted as a background, and the second element it would take is
 * exactly what the "renders only the inner element" contract above rules out.
 * Its callers give the slot its own resting colour via `className`, and a
 * letter that flips to a photo mid-load reads worse than a calm slot anyway.
 */
export default function Avatar({
	src,
	name,
	alt,
	entity = "seller",
	className,
	fallbackClassName,
	fallback,
	// No default: an avatar that turns out to be in the first render must not
	// be lazy. Lazy is opt-in per call site (see BrandLogo for the same rule).
	loading,
	fetchPriority,
	size = 80,
}: AvatarProps) {
	const resolved = React.useMemo(() => normalize(src), [src]);
	const [broken, setBroken] = React.useState(false);
	const [lastResolved, setLastResolved] = React.useState(resolved);
	if (lastResolved !== resolved) {
		setLastResolved(resolved);
		setBroken(false);
	}

	// An <img> rendered on the server can finish — or fail — before hydration
	// attaches the handlers below, and React never replays the event it missed.
	// Read the element's own state once per src so a server-rendered avatar that
	// already failed still reaches its fallback instead of sitting blank.
	const imgRef = React.useRef<HTMLImageElement | null>(null);
	React.useEffect(() => {
		const img = imgRef.current;
		if (resolved && img?.complete && img.naturalWidth === 0) setBroken(true);
	}, [resolved]);

	const fallbackClasses = cn(className, fallbackClassName) || undefined;
	const altText = alt ?? name ?? "";
	// Buyer avatars only: a seller slot with no logo is deliberately empty (see
	// the component docblock), and must never gain a generic name.
	const unnamed = entity === "buyer" && alt === undefined && !name?.trim();

	const strategy: NonNullable<AvatarProps["fallback"]> =
		entity === "seller"
			? "empty"
			: (fallback ?? (name?.trim() ? "initial" : "placeholder"));

	// While the browser is still working on the request the <img> STAYS — the
	// fallback is painted under it, never in place of it. A cold CDN
	// image-transform cache miss can leave an avatar pending for many seconds
	// with neither load nor error firing; the previous version of this component
	// gave that a 4s grace period and then swapped the element out for the
	// fallback, which is a one-way door: the swap unmounts the <img>, the
	// in-flight request is abandoned, and the photo can never come back even
	// though it was seconds away (#410). Only a real `error` — which is terminal
	// for a given src, and which a new src resets — replaces the image.
	if (resolved && !broken) {
		return (
			// biome-ignore lint/performance/noImgElement: Avatars are user-supplied CDN paths, not statically known at build time.
			<img
				ref={imgRef}
				src={resolved}
				alt={altText}
				// `alt=""` passed explicitly is a decorative decision and is honoured.
				// alt AND name simply *missing* is not a decision — the avatar silently
				// became decorative — so name it generically instead.
				role={unnamed ? "img" : undefined}
				aria-label={unnamed ? "使用者頭像" : undefined}
				width={size}
				height={size}
				className={className}
				style={strategy === "placeholder" ? PLACEHOLDER_LAYER : undefined}
				decoding="async"
				loading={loading}
				fetchPriority={fetchPriority}
				referrerPolicy="no-referrer"
				onError={() => setBroken(true)}
			/>
		);
	}

	if (strategy === "initial" && name?.trim()) {
		const initial = name.trim().charAt(0).toUpperCase();
		return (
			<span
				className={fallbackClasses}
				role="img"
				aria-label={altText || name.trim()}
			>
				{initial}
			</span>
		);
	}

	if (strategy === "placeholder") {
		return (
			// biome-ignore lint/performance/noImgElement: Local placeholder PNG under public/.
			<img
				src={placeholders.buyerAvatar}
				alt={altText}
				role={unnamed ? "img" : undefined}
				aria-label={unnamed ? "使用者頭像" : undefined}
				width={size}
				height={size}
				className={className}
				decoding="async"
				loading={loading}
				fetchPriority={fetchPriority}
			/>
		);
	}

	return <span className={fallbackClasses} aria-hidden="true" />;
}
