"use client";

import { useRef } from "react";
import {
	certLabel,
	getProductCertCatalogItem,
} from "@/lib/products/certifications";
import { useProductCertificationSpotlight } from "./useProductCertificationSpotlight";

export interface ProductCertificationSectionProps {
	certifications: Array<Record<string, unknown>>;
}

function text(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

/**
 * Buyer-facing product certification evidence.
 *
 * The public product contract currently owns only `name` (CertificationType
 * code) and `certNo`. We therefore show the official mark, human-readable type,
 * seller-provided number, and an authority lookup route, but deliberately do
 * not claim verification status, certifier, or validity dates that the payload
 * does not provide.
 */
export default function ProductCertificationSection({
	certifications,
}: ProductCertificationSectionProps) {
	const spotlightRootRef = useRef<HTMLUListElement>(null);
	useProductCertificationSpotlight(spotlightRootRef, certifications.length);

	if (certifications.length === 0) return null;

	return (
		<section className="mb-8" aria-labelledby="product-certifications-title">
			<div className="mb-4 flex flex-wrap items-end justify-between gap-x-5 gap-y-1.5">
				<h2
					id="product-certifications-title"
					className="m-0 text-[1.375rem] font-bold text-hk-heading"
				>
					標章認證
				</h2>
				<p className="m-0 text-xs leading-relaxed text-hk-muted">
					標章與認證字號由賣家提供，實際狀態請以官方查詢結果為準
				</p>
			</div>

			<ul
				ref={spotlightRootRef}
				className={`product-certification-spotlight-list m-0 grid list-none gap-4 ${
					certifications.length > 1 ? "lg:grid-cols-2" : "grid-cols-1"
				}`}
			>
				{certifications.map((cert, index) => {
					const code = text(cert.name);
					const certNo = text(cert.certNo) || text(cert.cert_no);
					const meta = getProductCertCatalogItem(code);
					const label = certLabel(code) || "未提供標章名稱";

					return (
						<li
							key={`cert-${code || "unknown"}-${certNo || index}`}
							data-product-cert-spotlight=""
							className="product-certification-spotlight-card flex w-full min-w-0 items-start gap-4 rounded-2xl border border-hk-border-soft bg-hk-surface-card p-4 dark:border-[#f5dea5]/14 dark:bg-white/4 max-sm:gap-3 max-sm:p-3 lg:max-w-[42rem]"
						>
							{meta ? (
								<div className="grid h-[72px] w-[72px] shrink-0 place-items-center rounded-xl border border-hk-border-soft bg-white p-1.5 dark:border-[#f5dea5]/20 max-sm:h-16 max-sm:w-16">
									<span className="text-center text-sm font-bold leading-tight text-hk-ink-950">
										{meta.shortLabel}
									</span>
								</div>
							) : null}

							<div className="min-w-0 flex-1">
								<div className="flex flex-wrap items-center gap-2">
									<h3 className="m-0 text-base font-bold leading-snug text-hk-body dark:text-[#f5e9c3] max-sm:text-sm">
										{label}
									</h3>
									<span className="inline-flex rounded-full bg-hk-surface-muted px-2 py-0.5 text-[0.6875rem] font-semibold leading-5 text-hk-muted dark:bg-white/8 dark:text-hk-ink-100/70">
										賣家提供
									</span>
								</div>

								{meta?.desc ? (
									<p className="mb-0 mt-1 text-[0.8125rem] leading-relaxed text-hk-muted">
										{meta.desc}
									</p>
								) : null}

								{certNo ? (
									<dl className="mb-0 mt-3 grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-3 gap-y-1">
										<dt className="text-xs font-medium text-hk-muted">
											認證字號
										</dt>
										<dd className="m-0 break-all font-mono text-sm font-semibold tracking-[0.025em] text-hk-body dark:text-[#f5e9c3]">
											{certNo}
										</dd>
									</dl>
								) : null}

								{meta?.officialLookup ? (
									<a
										href={meta.officialLookup.href}
										target="_blank"
										rel="noopener noreferrer"
										className="mt-3 inline-flex min-h-11 items-center rounded-full border border-hk-primary/30 px-3.5 py-2 text-sm font-semibold text-hk-accent-on-tint underline-offset-4 transition-colors duration-200 hover:border-hk-primary hover:bg-hk-brand-primary/10 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-hk-primary dark:border-hk-ink-200/35 dark:text-hk-ink-100 dark:hover:bg-hk-ink-300/12"
										aria-label={`${meta.officialLookup.label}（另開新分頁）`}
									>
										{meta.officialLookup.label}
									</a>
								) : null}
							</div>
						</li>
					);
				})}
			</ul>
		</section>
	);
}
