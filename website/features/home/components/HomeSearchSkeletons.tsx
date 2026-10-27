const BONE = "animate-pulse rounded bg-default-100 motion-reduce:animate-none";

/** Mirrors the loaded product list's gutter, row gap and typography boxes. */
export function HomeSearchProductSkeleton() {
	return (
		<ul
			className="flex min-h-0 flex-col gap-1 overflow-y-auto pr-1"
			aria-hidden="true"
		>
			{[0, 1, 2].map((index) => (
				<li key={index}>
					<div className="flex w-full gap-3 rounded-hk-md p-2">
						<div className={`h-16 w-20 shrink-0 rounded-hk-md ${BONE}`} />
						<div className="min-w-0 flex-1">
							<div className="text-sm font-semibold leading-snug">
								<span
									className={`inline-block h-[1em] w-4/5 align-middle ${BONE}`}
								/>
							</div>
							<div className="mt-1 text-xs">
								<span
									className={`inline-block h-[1em] w-1/2 align-middle ${BONE}`}
								/>
							</div>
							<div className="mt-1 text-sm font-extrabold">
								<span
									className={`inline-block h-[1em] w-1/3 align-middle ${BONE}`}
								/>
							</div>
						</div>
					</div>
				</li>
			))}
		</ul>
	);
}

/** Shared chunk fallback: same columns and section geometry as the live panel. */
export function HomeSearchPanelSkeleton({ hasQuery }: { hasQuery: boolean }) {
	return (
		<div
			className={`grid h-full grid-cols-1 ${hasQuery ? "md:grid-cols-[1.05fr_0.95fr]" : ""}`}
		>
			{hasQuery ? (
				<div className="flex min-h-0 flex-col gap-3 border-hk-border-subtle p-4 max-md:border-b md:border-r">
					<div className="flex items-baseline justify-between gap-2">
						<strong className="font-display text-hk-heading text-sm font-bold">
							相關商品
						</strong>
						<span
							className="text-hk-primary text-xs font-semibold"
							aria-hidden="true"
						>
							查看全部 →
						</span>
					</div>
					<HomeSearchProductSkeleton />
				</div>
			) : null}
			<div className="flex min-h-0 flex-col gap-3 p-4">
				<strong className="font-display text-hk-heading text-sm font-bold">
					{hasQuery ? "關鍵字建議" : "熱門搜尋"}
				</strong>
				<p className="m-0 text-hk-body text-xs">
					{hasQuery ? "搜尋中…" : "熱門標籤載入中…"}
				</p>
			</div>
		</div>
	);
}
