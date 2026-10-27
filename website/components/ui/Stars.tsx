const STAR_PATH =
	"M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z";

const MAX_STARS = 5;

/**
 * Filled/empty 5-star row. `count` is the number of filled stars (0–5; values
 * outside that range are clamped, fractions round down to the filled star).
 *
 * The row is a single `role="img"` with a text alternative: the individual
 * stars stay `aria-hidden`, so a rating is announced once as "4 顆星，滿分 5
 * 顆" instead of either five times or — as before — not at all.
 *
 * Filled and empty stars differ by SHAPE (solid vs outline), not only by
 * colour, so the rating survives colour-blindness and low-contrast screens.
 *
 * `gap: inherit` keeps the existing callers' `flex gap-*` spacing between the
 * stars now that they sit one level deeper.
 * @import import { Stars } from '@/components/ui/Stars';
 */
export function Stars({
	count,
	size = 18,
	color = "var(--page-rating-star)",
}: {
	count: number;
	size?: number;
	color?: string;
}) {
	const filled = Math.max(0, Math.min(MAX_STARS, Math.floor(count) || 0));

	return (
		<span
			role="img"
			aria-label={`${filled} 顆星，滿分 ${MAX_STARS} 顆`}
			className="inline-flex items-center gap-[inherit]"
		>
			{[1, 2, 3, 4, 5].map((s) => {
				const isFilled = s <= filled;
				return (
					<svg
						key={s}
						width={size}
						height={size}
						viewBox="0 0 24 24"
						fill={isFilled ? color : "none"}
						stroke={isFilled ? "none" : "var(--theme-border-input)"}
						strokeWidth={isFilled ? undefined : 1}
						aria-hidden="true"
						focusable="false"
					>
						<path d={STAR_PATH} />
					</svg>
				);
			})}
		</span>
	);
}
