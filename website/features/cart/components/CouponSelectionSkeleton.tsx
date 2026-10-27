/** Shares the coupon row grid so lead, details and radio stay aligned at 560px. */
export function CouponSelectionSkeleton() {
	return (
		<div className="coupon-dialog__section" role="status" aria-busy="true">
			<span className="sr-only">優惠券載入中</span>
			<div className="coupon-dialog__section-header" aria-hidden="true">
				<h3>
					<span className="coupon-dialog__skeleton-bone inline-block h-[1em] w-14 align-middle" />
				</h3>
				<span>
					<span className="coupon-dialog__skeleton-bone inline-block h-[1em] w-8 align-middle" />
				</span>
			</div>
			<ul className="coupon-sheet__list" aria-hidden="true">
				{[0, 1, 2].map((row) => (
					<li key={row}>
						<div className="coupon-sheet__row">
							<div className="coupon-sheet__lead">
								<span className="coupon-sheet__tag">
									<span className="coupon-dialog__skeleton-bone inline-block h-[1em] w-12 max-w-full align-middle" />
								</span>
								<strong className="coupon-sheet__discount">
									<span className="coupon-dialog__skeleton-bone inline-block h-[1em] w-14 max-w-full align-middle" />
								</strong>
							</div>
							<div className="coupon-sheet__body">
								<strong>
									<span className="coupon-dialog__skeleton-bone inline-block h-[1em] w-4/5 align-middle" />
								</strong>
								{[0, 1, 2].map((line) => (
									<small key={line}>
										<span
											className={`coupon-dialog__skeleton-bone inline-block h-[1em] align-middle ${line === 2 ? "w-full" : "w-2/3"}`}
										/>
									</small>
								))}
							</div>
							<span className="coupon-sheet__selection" />
						</div>
					</li>
				))}
			</ul>
		</div>
	);
}
