export const ORDER_INVOICE_HEADER_CLASS =
	"mb-3 flex flex-wrap items-center justify-between gap-2";
export const COMPLAINT_BANNER_LAYOUT = "rounded-hkui-md border p-4 text-sm";

const BONE =
	"block rounded bg-content2 animate-pulse motion-reduce:animate-none";

/** Complaint status always occupies the same bordered banner shell. */
export function InvoiceComplaintLoadingBody() {
	return (
		<div
			className={`${COMPLAINT_BANNER_LAYOUT} border-divider`}
			aria-hidden="true"
		>
			<div className="flex h-6 items-center gap-2">
				<span className={`${BONE} h-6 w-16 rounded-full`} />
			</div>
			<div className="mt-2 flex h-[1lh] items-center text-base">
				<span className={`${BONE} h-[1em] w-2/3`} />
			</div>
			<div className="mt-1 text-sm leading-relaxed">
				{[0, 1].map((line) => (
					<div key={line} className="flex h-[1lh] items-center">
						<span className={`${BONE} h-[1em] ${line ? "w-3/4" : "w-full"}`} />
					</div>
				))}
			</div>
		</div>
	);
}

/** State-dependent document/banner content is unknown; reserve text line boxes. */
export function InvoiceLoadingLines() {
	return (
		<div className="flex flex-col gap-2 text-sm" aria-hidden="true">
			{["w-2/3", "w-1/2", "w-3/4"].map((width) => (
				<div key={width} className="flex h-[1lh] items-center">
					<span className={`${BONE} h-[1em] ${width}`} />
				</div>
			))}
		</div>
	);
}
