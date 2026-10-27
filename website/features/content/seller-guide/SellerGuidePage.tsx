"use client";

import CardDetails from "@/features/content/shared/CardDetails";
import ContentIcon from "@/features/content/shared/ContentIcon";
import { useAuth } from "@/lib/auth/context";
import { sellerGuideChapters } from "@/lib/content/seller-guide-data";

/**
 * Seller guide as a plain text manual: every chapter and step is on the page
 * (searchable, copyable, readable by screen readers). The content is
 * `lib/content/seller-guide-data.ts`.
 */
export default function SellerGuidePage() {
	const { activeRole, hydrated } = useAuth();
	const frameWidthClass =
		hydrated && activeRole === "seller"
			? "w-[calc(100%_-_88px)]"
			: "w-[calc(100%_-_48px)]";

	const totalSteps = sellerGuideChapters.reduce(
		(sum, chapter) =>
			sum +
			chapter.items.reduce(
				(itemTotal, item) => itemTotal + (item.steps?.length ?? 0),
				0,
			),
		0,
	);

	return (
		<section
			className="section relative [overflow:visible] [overflow-x:clip]"
			style={{ paddingTop: "var(--space-4)" }}
		>
			<div
				className={`mx-auto max-w-[960px] ${frameWidthClass} max-[768px]:w-[calc(100%_-_32px)]`}
			>
				<div className="flex flex-col gap-2">
					<h1 className="m-0 font-[var(--font-brand)] text-[length:var(--text-2xl)] font-bold leading-8 text-[var(--ink-heading)]">
						賣家操作指南
					</h1>
					<p className="m-0 font-[var(--font-chinese)] text-[length:var(--text-base)] leading-7 text-[var(--ink-body)]">
						{sellerGuideChapters.length} 章 · {totalSteps}{" "}
						個步驟。可使用瀏覽器頁面搜尋快速找到功能名稱與操作說明。
					</p>
				</div>

				<div className="mt-8 flex flex-col gap-12">
					{sellerGuideChapters.map((chapter) => {
						const chapterHeadingId = `guide-text-${chapter.id}`;
						return (
							<section key={chapter.id} aria-labelledby={chapterHeadingId}>
								<div className="mb-4 flex items-center gap-3">
									<span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[12px] bg-[var(--color-primary)] text-hk-inverse">
										<ContentIcon icon={chapter.icon} />
									</span>
									<div className="min-w-0">
										<h2
											id={chapterHeadingId}
											className="m-0 font-[var(--font-chinese)] text-[length:var(--text-xl)] font-bold leading-7 text-[var(--ink-body)]"
										>
											{chapter.name}
										</h2>
										<p className="m-0 font-[var(--font-chinese)] text-[length:var(--text-sm)] leading-6 text-hk-body">
											{chapter.items.length} 個主題
										</p>
									</div>
								</div>

								<div className="flex flex-col gap-4">
									{chapter.items.map((item) => (
										<article
											key={item.id}
											id={`guide-${item.id}`}
											className="overflow-hidden rounded-[20px] glass-card glass-card--solid p-[17px_25px] max-[768px]:p-[14px_20px]"
										>
											<span className="font-[var(--font-primary)] text-[length:var(--text-sm)] font-bold leading-none text-hk-primary">
												{item.no}
											</span>
											<h3 className="m-0 mb-3 mt-1 font-[var(--font-chinese)] text-[length:var(--text-lg)] font-bold leading-7 text-hk-heading">
												{item.title}
											</h3>
											<CardDetails
												id={`guide-${item.id}`}
												body={item.summary}
												notes={item.notes}
												steps={item.steps}
												href={item.path}
											/>
										</article>
									))}
								</div>
							</section>
						);
					})}
				</div>
			</div>
		</section>
	);
}
