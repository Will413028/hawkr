import { brandAssets } from "@/lib/media/public-assets";

type BrandLogoVariant = "horizontal" | "mark" | "grass";
/** `fill` stretches the logo to the parent box, which then owns the size. */
type BrandLogoSize = "xs" | "sm" | "md" | "lg" | "xl" | "fill";
type BrandLogoSurface = "plain" | "soft" | "pill" | "solid";

interface BrandLogoProps {
	tone?: "dark" | "light";
	variant?: BrandLogoVariant;
	size?: BrandLogoSize;
	surface?: BrandLogoSurface;
	className?: string;
	/**
	 * Defaults to the browser's own scheduling (eager). Lazy is opt-in per call
	 * site, never the default: an eagerly loaded logo is at worst a little
	 * bandwidth, while a lazy one that turns out to be in the first render is a
	 * real defect — site-stability fails every non-progressive public route over
	 * it ("left an initially-rendered image lazy after ready"), which is how
	 * this shipped to main and blocked four production deploys.
	 *
	 * The lockup already declares `fetchPriority="low"`, so eager here does not
	 * compete with the LCP candidate.
	 */
	loading?: "eager" | "lazy";
}

const SOURCES: Record<
	BrandLogoVariant,
	{
		src: string;
		/** Wheat-cream variant — rendered when `[data-theme="dark"]`. */
		darkSrc?: string;
		alt: string;
		width: number;
		height: number;
	}
> = {
	horizontal: {
		src: brandAssets.logoHorizontal,
		darkSrc: brandAssets.logoHorizontalLight,
		alt: "Hawkr",
		width: 502,
		height: 120,
	},
	mark: {
		src: brandAssets.logoMark,
		alt: "Hawkr",
		width: 120,
		height: 120,
	},
	grass: {
		src: brandAssets.logoGrass,
		alt: "Hawkr",
		width: 144,
		height: 144,
	},
};

/**
 * Renders the official brand lockup. Variants come straight from the Figma
 * design tokens file (node 17:252 horizontal, 17:262 vertical, 17:233 mark,
 * 17:245 horizontal-light wheat variant).
 *
 * Dark-mode swap: when a `darkSrc` is available we render BOTH images
 * stacked and let CSS pick which one is visible based on
 * `[data-theme="dark"]`. This way the swap happens with no JS / no
 * hydration flash.
 *
 * Accessible name: it lives on the DEFAULT lockup and the dark twin stays
 * `aria-hidden`, in both themes. That is deliberate — the swap is an `opacity`
 * change (02-reset-layout.css), not `display`, so the default image never
 * leaves the accessibility tree and the lockup is named exactly once. Putting
 * `alt` on both would announce Hawkr twice in dark mode.
 *
 * The legacy `tone` prop still works — passing `tone="light"` forces the
 * wheat variant regardless of theme (used by the Footer dark slab).
 */
export default function BrandLogo({
	tone = "dark",
	variant = "horizontal",
	size = "md",
	surface = "plain",
	className = "",
	loading,
}: BrandLogoProps) {
	const { src, darkSrc, alt, width, height } = SOURCES[variant];
	const classes = [
		"brand-logo",
		`brand-logo--${tone}`,
		`brand-logo--${variant}`,
		`brand-logo--size-${size}`,
		`brand-logo--surface-${surface}`,
		darkSrc ? "brand-logo--has-dark" : "",
		className,
	]
		.filter(Boolean)
		.join(" ");

	return (
		<span className={classes}>
			{/* biome-ignore lint/performance/noImgElement: Logo SVGs are local Figma exports and must keep their intrinsic ratio. */}
			<img
				src={src}
				alt={alt}
				width={width}
				height={height}
				className="brand-logo__lockup brand-logo__lockup--default"
				decoding="async"
				loading={loading}
				draggable={false}
				// 兩個 SVG 都不是 LCP 候選；low 拿掉 React 19 SSR 自動塞的 preload
				// （每頁 4 條），瀏覽器仍在首次繪製就抓，畫面行為不變。
				fetchPriority="low"
			/>
			{darkSrc && (
				/* biome-ignore lint/performance/noImgElement: same as above; CSS-controlled visibility */
				<img
					src={darkSrc}
					alt=""
					aria-hidden="true"
					width={width}
					height={height}
					className="brand-logo__lockup brand-logo__lockup--dark"
					decoding="async"
					loading={loading}
					draggable={false}
					fetchPriority="low"
				/>
			)}
		</span>
	);
}

export type {
	BrandLogoProps,
	BrandLogoSize,
	BrandLogoSurface,
	BrandLogoVariant,
};
