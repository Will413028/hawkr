import { brandAssets } from "@/lib/media/public-assets";
import { cn } from "@/lib/utils/cn";

/**
 * The Hawkr wordmark as the loaders draw it: a faint base layer with the real
 * two-tone green wiping in left → right, like a loading bar. Keyframes live in
 * `styles/components/route-loading-skeleton.css` (`.il-word` / `.il-word__*`).
 *
 * It was written out twice — InitialPageLoader and AuthBrandLoading carried the
 * same two stacked `<img>` layers with the same biome ignores, differing only
 * in whether the colour layer sat inside `.il-word__reveal`.
 *
 * Both layers are `alt=""` + `aria-hidden`: this is a picture of the word the
 * surrounding `role="status"` already says, and naming it here made the splash
 * announce its own brand name up to three times.
 */
export default function BrandWordmark({
	className,
	reveal = false,
}: {
	className?: string;
	/** Wrap the colour layer in `.il-word__reveal` (the splash's wipe mask). */
	reveal?: boolean;
}) {
	const colorLayer = (
		// biome-ignore lint/performance/noImgElement: static brand SVG in two overlaid layers (grey base + colour wipe); next/image's wrapper span fights the absolute positioning and mask.
		<img
			className="il-word__layer il-word__color"
			src={brandAssets.wordmark}
			alt=""
			aria-hidden="true"
		/>
	);

	return (
		<div className={cn("il-word", className)}>
			{/* biome-ignore lint/performance/noImgElement: see above. */}
			<img
				className="il-word__layer il-word__base"
				src={brandAssets.wordmark}
				alt=""
				aria-hidden="true"
			/>
			{reveal ? (
				<div className="il-word__reveal">{colorLayer}</div>
			) : (
				colorLayer
			)}
		</div>
	);
}
