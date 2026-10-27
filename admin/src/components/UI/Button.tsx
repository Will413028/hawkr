import type React from "react";
import "./Button.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

export type ButtonVariant = "primary" | "danger" | "outline" | "ghost";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps {
	children: React.ReactNode;
	variant?: ButtonVariant;
	size?: ButtonSize;
	icon?: React.ReactNode;
	disabled?: boolean;
	loading?: boolean;
	onClick?: () => void;
	className?: string;
	type?: "button" | "submit" | "reset";
	/** 停用時說明原因的元素 id（停用的按鈕不能取得焦點，讀屏靠這個讀到原因）。 */
	"aria-describedby"?: string;
}

const Button: React.FC<ButtonProps> = ({
	children,
	variant = "primary",
	size = "md",
	icon,
	disabled = false,
	loading = false,
	onClick,
	className = "",
	type = "button",
	"aria-describedby": ariaDescribedBy,
}) => {
	const classes = [
		"hk-btn",
		`hk-btn--${variant}`,
		`hk-btn--${size}`,
		disabled ? "hk-btn--disabled" : "",
		loading ? "hk-btn--loading" : "",
		className,
	]
		.filter(Boolean)
		.join(" ");

	return (
		<button
			type={type}
			className={classes}
			onClick={onClick}
			disabled={disabled || loading}
			aria-describedby={ariaDescribedBy}
		>
			{loading ? (
				<MaterialSymbol name="progress_activity" className="hk-btn__spinner" />
			) : icon ? (
				<span className="hk-btn__icon">{icon}</span>
			) : null}
			<span className="hk-btn__text">{children}</span>
		</button>
	);
};

export default Button;
