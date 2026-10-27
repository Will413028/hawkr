import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import type React from "react";
import { onActivateKey } from "@/lib/a11y";

export type StatCardVariant = "success" | "warning" | "danger" | "default";

export interface StatCardProps {
	label: string;
	value: string | number;
	icon?: MaterialSymbolName;
	variant?: StatCardVariant;
	prefix?: string; // e.g., "NT$"
	onClick?: () => void;
	/**
	 * 值右側的行內動作（目前唯一的用例是複製鍵）。省略時不產生任何節點，因此對既有
	 * 呼叫端完全無感；它不是 `onClick` 的替代品——那個讓整張卡可點，這個只掛在值上。
	 */
	valueAction?: React.ReactNode;
}

/**
 * Unified StatCard component for dashboard statistics.
 * Used across all admin pages for consistent design.
 *
 * Features:
 * - Flag-shaped icon hanging from top-right corner
 * - Material Symbol name for the glyph
 * - Color variants: success (green), warning (orange), danger (red)
 * - Optional click handler
 *
 * Usage:
 * <StatCard label="今日交易額" value={2140} prefix="NT$" icon="paid" variant="success" />
 * <StatCard label="待付款" value={1} icon="schedule" variant="warning" />
 */
const StatCard: React.FC<StatCardProps> = ({
	label,
	value,
	icon,
	variant = "default",
	prefix,
	onClick,
	valueAction,
}) => {
	// Determine value color class
	const valueColorClass =
		variant === "warning" ? "warning" : variant === "danger" ? "danger" : "";

	// Determine icon background class
	const iconVariant = variant === "default" ? "success" : variant;

	const displayValue = prefix
		? `${prefix} ${typeof value === "number" ? value.toLocaleString() : value}`
		: typeof value === "number"
			? value.toLocaleString()
			: value;

	const interactiveProps = onClick
		? {
				role: "button" as const,
				tabIndex: 0,
				onClick,
				onKeyDown: onActivateKey(onClick),
			}
		: {};

	return (
		<div
			className={`stat-card ${onClick ? "clickable" : ""}`}
			{...interactiveProps}
		>
			<div className="stat-content">
				<div className="stat-label">{label}</div>
				<div className={`stat-value ${valueColorClass}`}>
					{displayValue}
					{valueAction}
				</div>
			</div>
			<div className="stat-icon">
				<div className={`stat-icon-inner ${iconVariant}`}>
					{icon ? <MaterialSymbol name={icon} /> : null}
				</div>
			</div>
		</div>
	);
};

export default StatCard;
