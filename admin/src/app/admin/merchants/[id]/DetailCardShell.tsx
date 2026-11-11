import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import { type BadgeVariant, StatusBadge } from "@/components/UI";

/** 商家詳情各卡共用的外框：標題列（圖示、標題、可選徽章）加內容區。 */
export function DetailCardShell({
	id,
	titleId,
	icon,
	title,
	className,
	badge,
	children,
}: {
	id?: string;
	titleId: string;
	icon: MaterialSymbolName;
	title: string;
	className: string;
	badge?: { label: string; variant: BadgeVariant };
	children: React.ReactNode;
}) {
	return (
		<section
			id={id}
			className={`detail-card ${className}`}
			aria-labelledby={titleId}
		>
			<div className="section-title">
				<MaterialSymbol name={icon} />
				<span id={titleId}>{title}</span>
				{badge && (
					<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
				)}
			</div>
			<div className="card-body">{children}</div>
		</section>
	);
}
