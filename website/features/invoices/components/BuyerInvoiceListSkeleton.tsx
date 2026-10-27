const BONE =
	"block animate-pulse rounded-hkui-md bg-content2 motion-reduce:animate-none";

export function BuyerInvoiceSkeletonRows() {
	return (
		<>
			{[0, 1, 2, 3, 4].map((row) => (
				<tr key={row} className="border-b border-divider last:border-b-0">
					{[0, 1, 2, 3, 4, 5].map((cell) => (
						<td key={cell} className="px-3 py-3 align-middle text-hk-body">
							<div
								aria-hidden="true"
								className={`flex h-6 items-center ${cell === 4 ? "justify-end" : ""}`}
							>
								<span
									className={`${BONE} ${cell === 3 || cell === 5 ? "h-6 w-14 rounded-full" : "h-4 w-full max-w-[8rem]"}`}
								/>
							</div>
						</td>
					))}
				</tr>
			))}
		</>
	);
}

/** Matches the mobile document card's date/status, number, issuer and amount. */
export function BuyerInvoiceSkeletonCards() {
	return (
		<>
			{[0, 1, 2, 3, 4].map((row) => (
				<li
					key={row}
					className="border-b border-divider p-4 last:border-b-0"
					aria-hidden="true"
				>
					<div className="flex flex-col gap-2">
						<div className="flex h-6 items-center justify-between gap-2">
							<span className={`${BONE} h-3 w-24`} />
							<span className={`${BONE} h-6 w-14 rounded-full`} />
						</div>
						<div className="flex h-6 items-center">
							<span className={`${BONE} h-4 w-36 max-w-full`} />
						</div>
						<div className="flex h-5 items-center justify-between gap-2">
							<span className={`${BONE} h-3.5 w-12`} />
							<span className={`${BONE} h-3.5 w-1/2`} />
						</div>
						<div className="flex h-6 items-center justify-between gap-2">
							<span className={`${BONE} h-6 w-14 rounded-full`} />
							<span className={`${BONE} h-3.5 w-20`} />
						</div>
					</div>
				</li>
			))}
		</>
	);
}
