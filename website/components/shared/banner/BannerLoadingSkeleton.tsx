import {
	FEATURE_CARD_BG,
	FEATURE_CARD_CHROME,
	FEATURE_CARD_WELL,
} from "./featureCard";

/** Lightweight chunk fallback; keeps the carousel title and indicator slots. */
export default function BannerLoadingSkeleton() {
	return (
		<section
			className={`w-full overflow-hidden p-[var(--hkui-space-5)] flex flex-col gap-[var(--hkui-space-4)] ${FEATURE_CARD_CHROME}`}
			style={FEATURE_CARD_BG}
			aria-hidden="true"
		>
			<div className="h-[1lh] text-[length:var(--text-sm)] leading-normal flex items-center">
				<div className="h-3.5 w-20 rounded bg-[var(--page-mega-skeleton-bone)] animate-pulse motion-reduce:animate-none" />
			</div>
			<div
				className={`${FEATURE_CARD_WELL} aspect-[16/9] animate-pulse motion-reduce:animate-none`}
			/>
			<div className="flex h-2.5 items-end justify-center">
				<div className="h-1.5 w-16 rounded-full bg-[var(--page-mega-skeleton-bone)] animate-pulse motion-reduce:animate-none" />
			</div>
		</section>
	);
}
