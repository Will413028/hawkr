"use client";

import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";

export type ToastIconKind =
	| "success"
	| "error"
	| "warning"
	| "info"
	| "loading"
	| "order"
	| "message"
	| "system";

interface ToastIconProps {
	kind: ToastIconKind;
	size?: number;
	className?: string;
}

const TOAST_ICON_NAMES: Record<ToastIconKind, AppIconName> = {
	success: "success",
	error: "fail",
	warning: "warning",
	info: "info",
	loading: "spinner",
	order: "bag",
	message: "chat",
	system: "notification",
};

/** Glyph for the Hawkr toast system; colour comes from `currentColor` so semantic toast colours and dark mode stay token-driven. */
export default function ToastIcon({
	kind,
	size = 20,
	className,
}: ToastIconProps) {
	return (
		<AppIcon
			name={TOAST_ICON_NAMES[kind]}
			size={size}
			className={className}
			spin={kind === "loading"}
		/>
	);
}
