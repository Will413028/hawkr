"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { Dialog } from "@/components/ui/Dialog";
import type { AfterSaleEvidence } from "@/lib/after-sales/types";
import { resolveCdnUrl } from "@/lib/media/cdn";

interface Props {
	photos: ReadonlyArray<AfterSaleEvidence>;
	emptyHint?: string;
	className?: string;
}

/** Photo evidence thumbnails (buyer or seller). Tap to open a lightbox. */
export function AfterSaleEvidenceGrid({
	photos,
	emptyHint = "尚無照片",
	className = "",
}: Props) {
	const [active, setActive] = React.useState<string | null>(null);

	if (!photos.length) {
		return <p className="text-sm text-foreground-soft">{emptyHint}</p>;
	}

	return (
		<>
			<div className={`flex flex-wrap gap-2 ${className}`}>
				{photos.map((p, i) => {
					const src = resolveCdnUrl(p.url);
					return (
						<button
							type="button"
							// biome-ignore lint/suspicious/noArrayIndexKey: fixed photo list, never reordered
							key={`${p.url}-${i}`}
							onClick={() => setActive(src)}
							className="relative w-20 h-20 rounded-hkui-md overflow-hidden border border-divider bg-content2 transition-transform hover:scale-[1.03] focus:outline-none focus-visible:ring-2 focus-visible:ring-hkui-primary"
							aria-label={p.label ?? `查看照片 ${i + 1}`}
						>
							<AppImage
								src={src}
								alt={p.label ?? `照片 ${i + 1}`}
								fill
								sizes="80px"
								className="object-cover"
							/>
						</button>
					);
				})}
			</div>
			<Dialog
				open={active !== null}
				onClose={() => setActive(null)}
				size="lg"
				showCloseButton
			>
				{active && (
					// biome-ignore lint/performance/noImgElement: lightbox needs intrinsic-size preview, not layout-bound Next Image
					<img
						src={active}
						alt="售後照片"
						className="w-full h-auto max-h-[70vh] object-contain rounded-hkui-md"
					/>
				)}
			</Dialog>
		</>
	);
}

export default AfterSaleEvidenceGrid;
