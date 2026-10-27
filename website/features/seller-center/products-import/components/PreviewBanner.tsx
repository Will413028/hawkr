"use client";

import { UI_TEXT } from "../i18n";

interface Props {
	jobFilename: string;
	totalEligible?: number;
	totalNeedsFix?: number;
	totalAssetsDownloading?: number;
	/** detail 模式：通用文字；listing 模式：含檔名 */
	variant?: "listing" | "detail";
}

export function PreviewBanner({
	jobFilename,
	totalEligible,
	totalNeedsFix,
	totalAssetsDownloading,
	variant = "listing",
}: Props) {
	return (
		<section
			role="status"
			className="rounded-[var(--hkui-radius-md)] border border-[var(--hkui-warning)]/70 bg-[var(--hkui-warning-50)] px-4 py-3"
		>
			<div className="flex flex-wrap items-center justify-between gap-2">
				<p className="font-semibold text-[var(--hkui-warning-700)]">
					{variant === "listing"
						? UI_TEXT.preview.bannerListing(jobFilename)
						: UI_TEXT.preview.bannerDetail}
				</p>
			</div>
			{variant === "listing" ? (
				<p className="mt-1 text-sm text-[var(--hkui-warning-700)]/85">
					{UI_TEXT.preview.eligibleCount(totalEligible ?? 0)}
					{totalNeedsFix
						? ` · ${UI_TEXT.preview.needsFixCount(totalNeedsFix)}`
						: ""}
					{totalAssetsDownloading
						? ` · ${UI_TEXT.preview.downloadingCount(totalAssetsDownloading)}`
						: ""}
				</p>
			) : null}
		</section>
	);
}
