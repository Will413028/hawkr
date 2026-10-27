"use client";

import type * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";

interface SendButtonProps {
	onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
	disabled?: boolean;
	loading?: boolean;
	type?: "button" | "submit";
	ariaLabel?: string;
	className?: string;
}

/**
 * Chat compose send button. The glyph is a static icon. `currentColor`
 * keeps it paired with light/dark/disabled foreground tokens without a
 * MutationObserver or first-frame dependency.
 */
export default function SendButton({
	onClick,
	disabled,
	loading,
	type = "submit",
	ariaLabel = "傳送",
	className,
}: SendButtonProps) {
	const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
		if (disabled || loading) return;
		onClick?.(event);
	};

	return (
		<Button
			type={type}
			className={`thread-page__send${className ? ` ${className}` : ""}`}
			onClick={handleClick}
			disabled={disabled}
			isLoading={loading}
			loadingLabel="傳送訊息中…"
			isIconOnly
			radius="full"
			aria-label={loading ? "傳送訊息中…" : ariaLabel}
		>
			<AppIcon name="send" size={22} fill />
		</Button>
	);
}
