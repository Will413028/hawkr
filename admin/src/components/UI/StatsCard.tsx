import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import type React from "react";

export type StatCardVariant =
	| "store"
	| "warning"
	| "danger"
	| "success"
	| "trending";

export interface StatsCardProps {
	label: string;
	value: string | number;
	icon?: MaterialSymbolName;
	variant?: StatCardVariant;
	valueClassName?: string; // Optional class for value styling
}

/**
 * Shared StatCard component matching Figma design system.
 * Features a flag-shaped icon hanging from the top-right corner.
 *
 * Variants:
 * - store: Gray (#565656) - for total/general stats
 * - warning: Orange (#EE964B) - for pending items
 * - danger: Red (#E7000B) - for rejected/critical items
 * - success: Green (status success) - for approved/active items
 * - trending: Gray (#565656) - for trending/growth stats
 */
const StatsCard: React.FC<StatsCardProps> = ({
	label,
	value,
	icon,
	variant = "store",
	valueClassName,
}) => {
	// Determine if value should be colored (warning and danger variants)
	const valueColorClass =
		variant === "warning" || variant === "danger" ? variant : "";

	return (
		<div className="stat-card-v2">
			<div className="stat-card-v2-content">
				<div className="stat-card-v2-label">{label}</div>
				<div
					className={`stat-card-v2-value ${valueColorClass} ${valueClassName || ""}`}
				>
					{value}
				</div>
			</div>
			<div className={`stat-card-v2-icon ${variant}`}>
				{icon ? (
					<MaterialSymbol name={icon} size={20} />
				) : (
					<span className="stat-card-v2-icon-placeholder" />
				)}
			</div>
		</div>
	);
};

export default StatsCard;
