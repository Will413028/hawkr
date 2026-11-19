"use client";

import { cn } from "@/lib/utils/cn";

interface PaginationProps {
	current: number;
	total: number;
	onPageChange: (page: number) => void;
}

function getVisiblePages(current: number, total: number): (number | string)[] {
	const pages: (number | string)[] = [];
	if (total <= 7) {
		for (let i = 1; i <= total; i++) pages.push(i);
		return pages;
	}
	pages.push(1);
	if (current > 3) pages.push("...");
	for (
		let i = Math.max(2, current - 1);
		i <= Math.min(total - 1, current + 1);
		i++
	) {
		pages.push(i);
	}
	if (current < total - 2) pages.push("...");
	pages.push(total);
	return pages;
}

export default function Pagination({
	current,
	total,
	onPageChange,
}: PaginationProps) {
	if (total <= 1) return null;

	const pages = getVisiblePages(current, total);
	const navButtonClass =
		"inline-flex h-11 items-center shrink-0 whitespace-nowrap rounded-hk-pill border-[1.5px] border-hk-border-input bg-hk-surface-elevated px-[18px] text-sm font-medium text-hk-body transition-[background-color,border-color,color,opacity] duration-200 ease-[ease] motion-reduce:transition-none enabled:hover:border-hk-primary enabled:hover:text-hk-primary disabled:cursor-not-allowed disabled:opacity-40 max-[480px]:px-3 max-[480px]:text-[0.8125rem]";

	return (
		<nav
			aria-label="分頁"
			className="mt-6 max-w-full min-w-0 overflow-x-auto overscroll-x-contain py-4"
		>
			<div className="flex w-max min-w-full flex-nowrap items-center justify-center gap-2">
				<button
					type="button"
					disabled={current <= 1}
					className={navButtonClass}
					onClick={() => onPageChange(current - 1)}
				>
					上一頁
				</button>
				{pages.map((p, i) => {
					if (p === "...") {
						const previous = pages[i - 1] ?? "start";
						const next = pages[i + 1] ?? "end";
						return (
							<span
								key={`dots-${previous}-${next}`}
								aria-hidden="true"
								className="min-w-8 shrink-0 text-center text-sm text-hk-muted! tracking-[2px]"
							>
								...
							</span>
						);
					}
					const page = p as number;
					const isCurrent = page === current;
					return (
						<button
							type="button"
							key={`page-${page}`}
							aria-label={`第 ${page} 頁`}
							aria-current={isCurrent ? "page" : undefined}
							className={cn(
								"flex h-11 min-w-11 shrink-0 items-center justify-center rounded-hk-md border-[1.5px] text-sm transition-[background-color,border-color,color] duration-200 ease-[ease] motion-reduce:transition-none",
								isCurrent
									? "border-hk-primary bg-hk-primary font-bold text-hk-inverse"
									: "border-transparent bg-hk-surface-glass-subtle font-medium text-hk-body hover:border-hk-primary hover:text-hk-primary",
							)}
							onClick={() => onPageChange(page)}
						>
							{page}
						</button>
					);
				})}
				<button
					type="button"
					disabled={current >= total}
					className={navButtonClass}
					onClick={() => onPageChange(current + 1)}
				>
					下一頁
				</button>
			</div>
		</nav>
	);
}
