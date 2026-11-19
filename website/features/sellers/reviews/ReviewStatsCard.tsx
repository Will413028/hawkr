import { StarsSvg } from "./ReviewList";

export interface SellerReviewStats {
	avgRating?: number | string;
	rating1Count?: number | string;
	rating2Count?: number | string;
	rating3Count?: number | string;
	rating4Count?: number | string;
	rating5Count?: number | string;
	totalCount?: number | string;
}

export function computeRatingPercents(stats: SellerReviewStats): number[] {
	const total = Number(stats.totalCount) || 0;
	if (total === 0) return [0, 0, 0, 0, 0];
	const counts = [
		Number(stats.rating5Count) || 0,
		Number(stats.rating4Count) || 0,
		Number(stats.rating3Count) || 0,
		Number(stats.rating2Count) || 0,
		Number(stats.rating1Count) || 0,
	];
	return counts.map((c) => Math.round((c / total) * 100));
}

/** 店家評分摘要卡（平均、星數分佈）。公開店面與賣家預覽共用。 */
export default function ReviewStatsCard({
	stats,
}: {
	stats: SellerReviewStats | null;
}) {
	const totalCount = stats ? Number(stats.totalCount) || 0 : 0;
	const avgRating = stats ? Number(stats.avgRating) || 0 : 0;
	const ratingPercents = stats ? computeRatingPercents(stats) : [0, 0, 0, 0, 0];
	const ratingBars = [5, 4, 3, 2, 1].map((stars, index) => ({
		stars,
		pct: ratingPercents[index] ?? 0,
	}));

	return (
		<div className="flex gap-[var(--space-10)] items-start mb-[var(--space-8)] p-[var(--space-8)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-xl)] bg-[var(--hkui-content1)] bg-gradient-to-br from-[rgba(255,255,255,0.8)] to-[rgba(248,246,236,0.66)] dark:from-[rgb(var(--color-ink-900-rgb)/0.9)] dark:to-[rgb(var(--color-ink-950-rgb)/0.9)] dark:border-[rgba(242,165,65,0.24)] shadow-[0_14px_38px_rgb(var(--color-ink-850-rgb)/0.08)] dark:shadow-[0_18px_46px_rgba(0,0,0,0.24)] max-[760px]:flex-col max-[760px]:items-stretch max-[760px]:gap-[var(--space-5)] max-[760px]:p-[var(--space-5)]">
			<div className="flex flex-col items-center justify-center flex-[0_0_128px] min-w-[128px] p-[var(--space-4)] rounded-[var(--hkui-radius-lg)] bg-[rgba(255,255,255,0.76)] dark:bg-[rgb(var(--color-ink-950-rgb)/0.48)] max-[760px]:w-full max-[760px]:min-w-0 max-[760px]:flex-none">
				<span className="mb-[var(--space-2)] text-[color:var(--text-heading)] text-[length:var(--text-5xl)] font-bold leading-none">
					{avgRating > 0 ? avgRating.toFixed(1) : "0"}
				</span>
				<div className="flex gap-[2px] mb-[var(--space-1)] text-[var(--accent-warm)]">
					<StarsSvg count={Math.round(avgRating)} />
				</div>
				<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)]">
					{totalCount} 則評價
				</span>
			</div>
			<div className="flex-1 flex flex-col gap-[var(--space-2)] pt-[var(--space-1)] max-[760px]:w-full max-[760px]:min-w-0 max-[760px]:flex-none">
				{ratingBars.map(({ stars, pct }) => (
					<div
						key={`bar-${stars}`}
						className="grid grid-cols-[42px_minmax(0,1fr)_44px] items-center gap-[var(--space-3)] max-[760px]:grid-cols-[38px_minmax(0,1fr)_38px] max-[760px]:gap-[var(--space-2)]"
					>
						<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)] whitespace-nowrap text-right">
							{stars} 星
						</span>
						<div className="w-full h-3 min-w-[24px] overflow-hidden rounded-full bg-[rgba(150,122,56,0.18)] dark:bg-[rgba(245,222,165,0.22)]">
							<div
								className="h-full min-w-0 rounded-inherit bg-gradient-to-r from-[var(--color-brand-secondary)] to-[var(--color-brand-primary)] transition-[width] duration-[var(--transition-base)]"
								style={{ width: `${pct}%` }}
							/>
						</div>
						<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)] whitespace-nowrap tabular-nums text-right">
							{pct}%
						</span>
					</div>
				))}
			</div>
		</div>
	);
}
