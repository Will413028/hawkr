import type React from "react";
import "./StatusBadge.css";

export type BadgeVariant = "success" | "danger" | "warning" | "gray" | "info";
export type BadgeSize = "sm" | "md";

export interface StatusBadgeProps {
	children: React.ReactNode;
	variant?: BadgeVariant;
	size?: BadgeSize;
	className?: string;
}

const StatusBadge: React.FC<StatusBadgeProps> = ({
	children,
	variant = "success",
	size = "md",
	className = "",
}) => {
	const classes = [
		"hk-badge",
		`hk-badge--${variant}`,
		`hk-badge--${size}`,
		className,
	]
		.filter(Boolean)
		.join(" ");

	return <span className={classes}>{children}</span>;
};

export default StatusBadge;
