import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import type { ReactNode } from "react";

/** 詳情頁的欄位：標籤加值，值可以跨整列（fullWidth）。 */
export function Field({
	label,
	children,
	fullWidth,
}: {
	label: string;
	children: ReactNode;
	fullWidth?: boolean;
}) {
	return (
		<div className={`info-field${fullWidth ? " full-width" : ""}`}>
			<div className="field-label">{label}</div>
			<div className="field-value">{children}</div>
		</div>
	);
}

/** 詳情頁的區塊：圖示加標題（id 給 aria-labelledby），內容在卡片裡。 */
export function Section({
	id,
	icon,
	title,
	children,
}: {
	id: string;
	icon: MaterialSymbolName;
	title: string;
	children: ReactNode;
}) {
	return (
		<section className="detail-card" aria-labelledby={id}>
			<div className="section-title">
				<MaterialSymbol name={icon} />
				<h2 id={id}>{title}</h2>
			</div>
			<div className="card-body">{children}</div>
		</section>
	);
}
