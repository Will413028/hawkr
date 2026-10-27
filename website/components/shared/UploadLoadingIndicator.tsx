"use client";

import type * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { cn } from "@/lib/utils/cn";

export interface UploadLoadingIndicatorProps
	extends Omit<React.HTMLAttributes<HTMLSpanElement>, "children" | "role"> {
	size?: number;
	/** Pulses while true; a still icon otherwise. */
	active?: boolean;
	ariaLabel?: string;
}

/**
 * Shared upload-state mark.
 *
 * It reports progress, so unlike the decorative icons it carries a name and a
 * `role="status"` wrapper by default — an upload tile with no visible label
 * otherwise showed a glyph that assistive tech never announced. The pulse
 * stops under reduced motion.
 */
export default function UploadLoadingIndicator({
	size = 18,
	active = true,
	ariaLabel = "上傳中",
	className,
	...props
}: UploadLoadingIndicatorProps) {
	return (
		<span
			{...props}
			role="status"
			aria-label={ariaLabel}
			className={cn("inline-flex items-center justify-center", className)}
		>
			<AppIcon
				name="upload"
				size={size}
				className={
					active ? "animate-pulse motion-reduce:animate-none" : undefined
				}
			/>
		</span>
	);
}
