"use client";

import type * as React from "react";
import { cn } from "@/lib/utils/cn";

type ChipVariant = "solid" | "flat" | "bordered" | "dot";
type ChipColor =
	| "default"
	| "primary"
	| "secondary"
	| "success"
	| "warning"
	| "danger";
type ChipSize = "sm" | "md" | "lg";

interface ChipProps {
	variant?: ChipVariant;
	color?: ChipColor;
	size?: ChipSize;
	startContent?: React.ReactNode;
	endContent?: React.ReactNode;
	onClose?: () => void;
	className?: string;
	children: React.ReactNode;
}

const CloseIcon = () => (
	<svg
		width="10"
		height="10"
		viewBox="0 0 24 24"
		fill="none"
		stroke="currentColor"
		strokeWidth="3"
		strokeLinecap="round"
		strokeLinejoin="round"
		aria-hidden="true"
	>
		<path d="M18 6 6 18M6 6l12 12" />
	</svg>
);

/** @import import { Chip } from '@/components/ui/Chip'; */
export const Chip: React.FC<ChipProps> = ({
	variant = "solid",
	color = "default",
	size = "md",
	startContent,
	endContent,
	onClose,
	className = "",
	children,
}) => {
	// A removable chip carries a touch target; a purely decorative status chip
	// does not. Only the former grows to 44px, and only on coarse pointers, so
	// dense desktop rows keep their density.
	const interactive = Boolean(onClose);
	const classes = cn(
		"inline-flex max-w-full shrink-0 items-center justify-center whitespace-nowrap border font-medium tracking-normal transition-[background,border-color,color,opacity] duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none",
		chipSizeClass[size],
		interactive && "pointer-coarse:min-h-11",
		chipVariantClass[variant][color],
		className,
	);

	return (
		<span className={classes}>
			{variant === "dot" && (
				<span
					className={cn("shrink-0 rounded-full", chipDotClass[size])}
					aria-hidden="true"
				/>
			)}
			{startContent}
			<span>{children}</span>
			{endContent}
			{onClose && (
				<button
					type="button"
					// The visible dot stays 20px so the chip keeps its density;
					// the `before:` pseudo-element grows the *hit area* past
					// 40px (48px on touch) without changing a single pixel of
					// the layout. `text-current` at full strength on hover/focus
					// keeps the glyph readable on the flat variants.
					className="relative -mr-1 inline-grid size-5 place-items-center rounded-full text-current/70 transition-colors hover:bg-current/10 hover:text-current focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--hkui-focus)] motion-reduce:transition-none before:absolute before:-inset-2.5 before:content-[''] pointer-coarse:before:-inset-3"
					onClick={onClose}
					aria-label="移除"
				>
					<CloseIcon />
				</button>
			)}
		</span>
	);
};

const chipSizeClass: Record<ChipSize, string> = {
	sm: "h-6 gap-1 rounded-[var(--hkui-radius-sm)] px-2 text-xs",
	md: "h-7 gap-1.5 rounded-[var(--hkui-radius-md)] px-2.5 text-sm",
	lg: "h-8 gap-2 rounded-[var(--hkui-radius-md)] px-3 text-base",
};

const chipDotClass: Record<ChipSize, string> = {
	sm: "size-1.5",
	md: "size-2",
	lg: "size-2.5",
};

const chipSolidColorClass: Record<ChipColor, string> = {
	default:
		"border-transparent bg-[var(--hkui-default-100)] text-[var(--hkui-default-foreground)]",
	primary:
		"border-transparent bg-[var(--hkui-primary)] text-[var(--hkui-primary-foreground)]",
	secondary:
		"border-transparent bg-[var(--hkui-secondary)] text-[var(--hkui-secondary-foreground)]",
	success:
		"border-transparent bg-[var(--hkui-success)] text-[var(--hkui-success-foreground)]",
	warning:
		"border-transparent bg-[var(--hkui-warning)] text-[var(--hkui-warning-foreground)]",
	danger:
		"border-transparent bg-[var(--hkui-danger)] text-[var(--hkui-danger-foreground)]",
};

const chipFlatColorClass: Record<ChipColor, string> = {
	default:
		"border-transparent bg-[var(--hkui-default-100)] text-[var(--hkui-default-foreground)]",
	primary:
		"border-transparent bg-[var(--hkui-primary-50)] text-[var(--hkui-primary)]",
	secondary:
		"border-transparent bg-[var(--hkui-secondary-50)] text-[var(--hkui-secondary-700)]",
	success:
		"border-transparent bg-[var(--hkui-success-50)] text-[var(--hkui-success-700)]",
	warning:
		"border-transparent bg-[var(--hkui-warning-50)] text-[var(--hkui-warning-foreground)]",
	danger:
		"border-transparent bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)]",
};

const chipBorderedColorClass: Record<ChipColor, string> = {
	default:
		"border-[var(--hkui-default-300)] bg-transparent text-[var(--hkui-default-foreground)]",
	primary:
		"border-[var(--hkui-primary)] bg-transparent text-[var(--hkui-primary)]",
	secondary:
		"border-[var(--hkui-secondary)] bg-transparent text-[var(--hkui-secondary-700)]",
	success:
		"border-[var(--hkui-success)] bg-transparent text-[var(--hkui-success-700)]",
	warning:
		"border-[var(--hkui-warning)] bg-transparent text-[var(--hkui-warning-700)]",
	danger:
		"border-[var(--hkui-danger)] bg-transparent text-[var(--hkui-danger-700)]",
};

const chipDotColorClass: Record<ChipColor, string> = {
	default:
		"border-transparent bg-[var(--hkui-default-100)] text-[var(--hkui-default-foreground)] [&>span:first-child]:bg-[var(--hkui-default-500)]",
	primary:
		"border-transparent bg-[var(--hkui-primary-50)] text-[var(--hkui-primary)] [&>span:first-child]:bg-[var(--hkui-primary)]",
	secondary:
		"border-transparent bg-[var(--hkui-secondary-50)] text-[var(--hkui-secondary-700)] [&>span:first-child]:bg-[var(--hkui-secondary)]",
	success:
		"border-transparent bg-[var(--hkui-success-50)] text-[var(--hkui-success-700)] [&>span:first-child]:bg-[var(--hkui-success)]",
	warning:
		"border-transparent bg-[var(--hkui-warning-50)] text-[var(--hkui-warning-foreground)] [&>span:first-child]:bg-[var(--hkui-warning)]",
	danger:
		"border-transparent bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)] [&>span:first-child]:bg-[var(--hkui-danger)]",
};

const chipVariantClass: Record<ChipVariant, Record<ChipColor, string>> = {
	solid: chipSolidColorClass,
	flat: chipFlatColorClass,
	bordered: chipBorderedColorClass,
	dot: chipDotColorClass,
};

export type { ChipColor, ChipProps, ChipSize, ChipVariant };
