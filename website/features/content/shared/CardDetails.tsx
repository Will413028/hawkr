import Link from "@/components/shared/Link";

/**
 * Body of a description card: summary paragraph, notes, numbered steps and an
 * optional deep link. Shared by the collapsible FAQ card and the seller guide,
 * which shows it expanded.
 */
export default function CardDetails({
	id,
	body,
	notes,
	steps,
	stepsLabel = "操作步驟",
	href,
	hrefLabel = "前往這個頁面",
}: {
	id: string;
	body: string;
	notes?: string[];
	steps?: string[];
	stepsLabel?: string;
	href?: string;
	hrefLabel?: string;
}) {
	const allNotes = notes ?? [];
	return (
		<>
			<p className="font-[var(--font-chinese)] text-[length:var(--text-base)] font-normal leading-[26px] text-[var(--ink-body)] m-0">
				{body}
			</p>
			{allNotes.map((n) => (
				<div
					key={`${id}-note-${n}`}
					className="mt-4 flex items-start gap-2 rounded-[14px] border border-hk-border-soft bg-hk-surface-glass-hover p-[12px_14px]"
				>
					<svg
						aria-hidden="true"
						className="shrink-0 mt-[2px] [&_*]:stroke-hk-primary"
						width="18"
						height="18"
						viewBox="0 0 24 24"
						fill="none"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="8" />
						<line x1="12" y1="11" x2="12" y2="16" />
					</svg>
					<p className="font-[var(--font-chinese)] text-[length:var(--text-sm)] font-normal leading-[24px] text-hk-body m-0">
						<strong className="font-bold text-hk-primary">請注意：</strong>
						{n}
					</p>
				</div>
			))}
			{steps && steps.length > 0 && (
				<div className="mt-4">
					<p className="font-[var(--font-chinese)] text-[length:var(--text-sm)] font-bold text-hk-primary m-0 mb-3">
						{stepsLabel}
					</p>
					<ol className="flex flex-col gap-3 m-0 p-0 list-none">
						{steps.map((step, stepIdx) => (
							<li key={`${id}-step-${step}`} className="flex items-start gap-3">
								<span className="flex items-center justify-center shrink-0 w-6 h-6 rounded-full bg-hk-primary text-hk-inverse font-[var(--font-chinese)] text-[length:var(--text-sm)] font-bold leading-none">
									{stepIdx + 1}
								</span>
								<span className="font-[var(--font-chinese)] text-[length:var(--text-base)] font-normal leading-[26px] text-[var(--ink-body)] pt-[1px]">
									{step}
								</span>
							</li>
						))}
					</ol>
				</div>
			)}
			{href && (
				<Link
					href={href}
					className="mt-5 inline-flex items-center gap-2 rounded-[10px] border border-hk-border-soft bg-hk-surface-glass-hover p-[8px_14px] font-[var(--font-chinese)] text-[length:var(--text-sm)] font-bold !text-hk-primary no-underline transition-colors duration-200 hover:bg-hk-surface-glass-active"
				>
					{hrefLabel}
					<svg
						aria-hidden="true"
						width="16"
						height="16"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<line x1="5" y1="12" x2="19" y2="12" />
						<polyline points="12 5 19 12 12 19" />
					</svg>
				</Link>
			)}
		</>
	);
}
