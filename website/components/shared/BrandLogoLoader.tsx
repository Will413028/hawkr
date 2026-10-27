/**
 * The initial-page-loader's animated brand mark, extracted for reuse: the
 * Hawkr stall mark (`brand/hawkr/hawkr-mark.svg`) as a line-art outline with
 * the awning and the counter blooming out of the centre, awning first.
 * Styling/keyframes live in
 * styles/components/decorations-overlays.css (`.il-logo` / `.il-blade`),
 * shared with InitialPageLoader so the motion stays identical everywhere.
 *
 * Sizing: with no `size` the CSS default applies (104px, the splash size).
 * Pass a number for a fixed square, or "fit" to fill the parent box — the
 * SVG keeps its aspect and scales to whichever side it hits first.
 *
 * Downsized renders also drop the CSS `non-scaling-stroke` (which pins the
 * outline at 2.4px regardless of scale): at splash size that constant width
 * is the look, but below ~64px it grows relatively fat enough to swallow
 * the gap between the awning and the counter. Letting the stroke scale
 * keeps that negative space visible.
 */

/** The awning (`hawkr-mark.svg`'s clip path) and its two accent stripes. */
const AWNING_PATH =
	"M28 24H92L102 46A10.5 10.5 0 0 1 81 46A10.5 10.5 0 0 1 60 46A10.5 10.5 0 0 1 39 46A10.5 10.5 0 0 1 18 46Z";
const AWNING_STRIPE_PATH =
	"M81 24H92L102 46A10.5 10.5 0 0 1 81 46ZM39 24H60V46A10.5 10.5 0 0 1 39 46Z";
/** The two posts and the bar between them, as one outline. */
const COUNTER_PATH =
	"M36 60A7 7 0 0 1 43 67V76H77V67A7 7 0 0 1 91 67V101A7 7 0 0 1 77 101V90H43V101A7 7 0 0 1 29 101V67A7 7 0 0 1 36 60Z";

/**
 * 標誌各部分的著色方式。token 定義在 styles/foundation/00-color-config.css
 * 的 `--brand-logo-on-*`（一底配一色，呼叫端不自己挑顏色）。
 *
 * - `brand`（預設）：Hawkr 原色（遮雨棚 primary、條紋 secondary、檯面 ink）。
 *   用在**中性／白底**（開場動畫、Modal、認證頁）。
 * - `on-light` / `on-dark`：整枚商標壓成單色。放在實心品牌色表面上時必須指定
 *   其中之一：淺底用 `on-light`（primary），深底（含 primary-strong 的實心底）
 *   用 `on-dark`（ink-on-dark）。
 * - `mono`：走 `currentColor`，給「呼叫端已定義好可讀前景色」的情況用。
 *
 * 所有 tone 共用同一份 `.il-logo` / `.il-blade` keyframes，載入動態完全一致，
 * 差別只有顏色。
 */
type BrandLogoTone = "brand" | "mono" | "on-light" | "on-dark";

interface BrandLogoLoaderProps {
	size?: number | "fit";
	className?: string;
	tone?: BrandLogoTone;
}

/** @import import BrandLogoLoader from '@/components/shared/BrandLogoLoader'; */
export default function BrandLogoLoader({
	size,
	className,
	tone = "brand",
}: BrandLogoLoaderProps) {
	const style =
		size === "fit"
			? { width: "100%", height: "100%" }
			: typeof size === "number"
				? { width: size, height: size }
				: undefined;
	const lineStyle =
		size === undefined ? undefined : { vectorEffect: "none" as const };

	/**
	 * 標誌各部分的顏色（遮雨棚、條紋、檯面）直接讀 token，色票改了這裡跟著動。
	 *
	 * `mono` 走 currentColor：呼叫端（按鈕）已經定義好對比色，交給它決定。
	 */
	const SOLID_TONE: Partial<Record<BrandLogoTone, string>> = {
		mono: "currentColor",
		"on-light": "var(--brand-logo-on-light)",
		"on-dark": "var(--brand-logo-on-dark)",
	};
	const solid = SOLID_TONE[tone];
	const blade = solid
		? ([solid, solid, solid, solid] as const)
		: ([
				"var(--color-brand-primary)",
				"var(--color-brand-secondary)",
				"var(--color-brand-secondary)",
				"var(--brand-logo-counter)",
			] as const);

	return (
		<div
			className={`il-logo ${className ?? ""}`.trim()}
			style={style}
			aria-hidden="true"
		>
			<svg
				className="il-logo__layer il-logo__line"
				viewBox="0 0 120 120"
				aria-hidden="true"
			>
				<path stroke={blade[0]} style={lineStyle} d={AWNING_PATH} />
				<path stroke={blade[3]} style={lineStyle} d={COUNTER_PATH} />
			</svg>
			<svg
				className="il-logo__layer il-logo__fill"
				viewBox="0 0 120 120"
				aria-hidden="true"
			>
				<g className="il-blade il-blade--awning">
					<path fill={blade[0]} d={AWNING_PATH} />
					<path fill={blade[1]} d={AWNING_STRIPE_PATH} />
				</g>
				<path
					className="il-blade il-blade--counter"
					fill={blade[3]}
					d={COUNTER_PATH}
				/>
			</svg>
		</div>
	);
}
