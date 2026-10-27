import type * as React from "react";

interface InsightsSectionHeaderProps {
	title: string;
	description?: string;
	action?: React.ReactNode;
}

/**
 * 行情頁的區塊標頭。標題直接說明資料種類，避免用裝飾性的英文眉標混淆
 * 「官方實際價」與「模型預測」兩種不同語意。
 */
export function InsightsSectionHeader({
	title,
	description,
	action,
}: InsightsSectionHeaderProps) {
	return (
		<div className="seller-price-lookup__section-head-row">
			<div className="seller-price-lookup__section-head">
				<h2 className="seller-price-lookup__section-title">{title}</h2>
				{description ? (
					<p className="seller-price-lookup__section-description">
						{description}
					</p>
				) : null}
			</div>
			{action}
		</div>
	);
}
