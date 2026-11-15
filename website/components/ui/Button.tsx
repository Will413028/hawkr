"use client";

import * as React from "react";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import Link from "@/components/shared/Link";
import { cn } from "@/lib/utils/cn";

type ButtonVariant =
	| "solid"
	| "bordered"
	| "light"
	| "flat"
	| "faded"
	| "shadow"
	| "ghost";
type ButtonColor =
	| "default"
	| "primary"
	| "secondary"
	| "success"
	| "warning"
	| "danger";
type ButtonSize = "sm" | "md" | "lg";
type ButtonRadius = "none" | "sm" | "md" | "lg" | "full";

interface ButtonStyleOpts {
	variant?: ButtonVariant;
	color?: ButtonColor;
	size?: ButtonSize;
	radius?: ButtonRadius;
	fullWidth?: boolean;
	isIconOnly?: boolean;
	isLoading?: boolean;
}

interface ButtonProps
	extends Omit<
			React.ButtonHTMLAttributes<HTMLButtonElement>,
			"children" | "color" | "size"
		>,
		ButtonStyleOpts {
	/** When set, the button renders as a Next.js `<Link>` (navigation CTA). */
	href?: string;
	/** Anchor-only — only meaningful when `href` is set (renders as `<Link>` /
	 *  `<a>`). Forwarded onto the rendered anchor; ignored on `<button>`. */
	target?: React.AnchorHTMLAttributes<HTMLAnchorElement>["target"];
	rel?: React.AnchorHTMLAttributes<HTMLAnchorElement>["rel"];
	startContent?: React.ReactNode;
	endContent?: React.ReactNode;
	/** Visible while `isLoading`; keep it short and action-specific. */
	loadingLabel?: React.ReactNode;
	/** Stable widest pending label used only for idle width reservation. Supply
	 *  this when a toggle's visible `loadingLabel` changes with its direction. */
	loadingReservationLabel?: React.ReactNode;
	/** Optional semantic loader (for example Upload). Defaults to the brand LOGO. */
	loadingIndicator?: React.ReactNode;
	children?: React.ReactNode;
}

// md is the app-wide default and clears the 44px touch target outright. sm is
// a deliberately dense desktop size (toolbars, table rows): it keeps its 32px
// under a mouse and only grows on coarse pointers.
const buttonSizeClass: Record<ButtonSize, string> = {
	sm: "h-8 gap-1.5 text-xs pointer-coarse:min-h-11 pointer-coarse:min-w-11",
	md: "h-11 gap-2 text-sm",
	lg: "h-12 gap-2.5 text-base",
};

const buttonPaddingClass: Record<ButtonSize, string> = {
	sm: "px-3",
	md: "px-4",
	lg: "px-6",
};

const buttonIconSizeClass: Record<ButtonSize, string> = {
	sm: "w-8 px-0",
	md: "w-11 px-0",
	lg: "w-12 px-0",
};

const buttonDefaultRadiusClass: Record<ButtonSize, string> = {
	sm: "rounded-[var(--hkui-control-radius-sm)]",
	md: "rounded-[var(--hkui-control-radius-md)]",
	lg: "rounded-[var(--hkui-control-radius-lg)]",
};

const buttonRadiusClass: Record<ButtonRadius, string> = {
	none: "rounded-none",
	sm: "rounded-[var(--hkui-radius-sm)]",
	md: "rounded-[var(--hkui-radius-md)]",
	lg: "rounded-[var(--hkui-radius-lg)]",
	full: "rounded-full",
};

const buttonLoadingIndicatorSize: Record<ButtonSize, number> = {
	sm: 16,
	md: 18,
	lg: 20,
};

function reservedIndicatorSize(
	indicator: React.ReactNode,
	size: ButtonSize,
): number {
	if (
		React.isValidElement<{ size?: unknown }>(indicator) &&
		typeof indicator.props.size === "number"
	) {
		return indicator.props.size;
	}
	return buttonLoadingIndicatorSize[size];
}

const buttonLoadingStateClass =
	"cursor-wait border-[var(--hkui-default-400)]! bg-[var(--hkui-default-300)]! text-[var(--hkui-foreground)]! shadow-none! hover:bg-[var(--hkui-default-300)]! active:scale-100 disabled:opacity-100! aria-disabled:opacity-100! motion-reduce:transition-none";

interface ButtonLoadingContentProps {
	label?: React.ReactNode;
	indicator?: React.ReactNode;
	size?: ButtonSize;
	isIconOnly?: boolean;
	announce?: boolean;
	constrain?: boolean;
}

/**
 * Shared pending content for action buttons. The surrounding button owns the
 * fixed geometry; this content only communicates the in-flight state.
 */
export function ButtonLoadingContent({
	label = "處理中…",
	indicator,
	size = "md",
	isIconOnly = false,
	announce = true,
	constrain = true,
}: ButtonLoadingContentProps) {
	return (
		<span
			className={cn(
				"inline-flex items-center justify-center gap-2",
				constrain ? "min-w-0 max-w-full" : "w-max shrink-0",
			)}
			role={announce ? "status" : undefined}
			aria-live={announce ? "polite" : undefined}
			aria-atomic={announce ? "true" : undefined}
		>
			<span className="inline-flex shrink-0" aria-hidden="true">
				{/* tone="mono"：按鈕的底色多半就是品牌色，原色商標疊在同色的 solid primary
				    上等於隱形。改吃 currentColor，讓每顆按鈕用它自己已定義的可讀
				    前景色，淺底／深色底／深色主題的輔色底都自動正確。動態不變。 */}
				{indicator ?? (
					<BrandLogoLoader
						size={buttonLoadingIndicatorSize[size]}
						tone="mono"
					/>
				)}
			</span>
			{/* `hkui-button__loading-label` is the hook for buttons whose icon-only
			    state is decided by CSS rather than by the `isIconOnly` prop — a
			    container query can't set a React prop, so it collapses this label
			    the same way it collapses the button's idle label (see
			    `.market-product-card__cart-btn` in styles/features/market.css).
			    Hiding it also collapses the idle width reservation that renders
			    the same node, so a 32px chip reserves 32px, not the text's width. */}
			<span
				className={cn(
					"hkui-button__loading-label",
					isIconOnly
						? "sr-only"
						: constrain
							? "min-w-0 truncate"
							: "whitespace-nowrap",
				)}
			>
				{label}
			</span>
		</span>
	);
}

// These text colours are deliberately NOT `!important`. A solid button is a
// painted surface, and callers that repaint the surface (`bg-[#c4a93a]`) must
// be able to repaint the label with it — `!important` here silently pinned
// every solid button's text to the token colour, so a custom light surface
// kept white text at ~2.2:1.
//
// The dark-mode anchor rule `[data-theme="dark"] body a { color }` is what
// originally forced the `!`; it is now excluded for `a[data-hkui-button]` in
// styles/foundation/01-tokens-theme.css, which is the correct place to say
// "a button is not body text".
const solidColorClass: Record<ButtonColor, string> = {
	default:
		"bg-[var(--hkui-default-100)] text-[var(--hkui-foreground)] hover:bg-[var(--hkui-default-200)]",
	primary:
		"bg-[var(--hkui-primary)] text-[var(--hkui-primary-foreground)] hover:bg-[var(--hkui-primary-600)]",
	secondary:
		"bg-[var(--hkui-secondary)] text-[var(--hkui-secondary-foreground)] hover:bg-[var(--hkui-secondary-600)]",
	success:
		"bg-[var(--hkui-success)] text-[var(--hkui-success-foreground)] hover:bg-[var(--hkui-success-600)]",
	warning:
		"bg-[var(--hkui-warning)] text-[var(--hkui-warning-foreground)] hover:bg-[var(--hkui-warning-600)]",
	danger:
		"bg-[var(--hkui-danger-600)] text-[var(--hkui-danger-foreground)] hover:bg-[var(--hkui-danger-700)]",
};

const borderedColorClass: Record<ButtonColor, string> = {
	default:
		"border-2 border-[var(--hkui-default-300)] bg-transparent text-[var(--hkui-foreground)] hover:bg-[var(--hkui-default-100)] dark:border-[var(--page-button-bordered-border)] dark:text-[var(--page-button-bordered-fg)] dark:hover:bg-[var(--page-button-bordered-hover-bg)]",
	primary:
		"border-2 border-[var(--hkui-primary)] bg-transparent text-[var(--hkui-primary)] hover:bg-[var(--hkui-primary-50)]",
	secondary:
		"border-2 border-[var(--hkui-secondary)] bg-transparent text-[var(--hkui-secondary)] hover:bg-[var(--hkui-secondary-50)]",
	success:
		"border-2 border-[var(--hkui-success)] bg-transparent text-[var(--hkui-success)] hover:bg-[var(--hkui-success-50)]",
	warning:
		"border-2 border-[var(--hkui-warning)] bg-transparent text-[var(--hkui-warning-700)] hover:bg-[var(--hkui-warning-50)]",
	danger:
		"border-2 border-[var(--hkui-danger)] bg-transparent text-[var(--hkui-danger-text)] hover:bg-[var(--hkui-danger-50)] dark:hover:bg-[rgb(var(--color-status-danger-rgb)/0.16)]",
};

const lightColorClass: Record<ButtonColor, string> = {
	default:
		"bg-transparent text-[var(--hkui-foreground)] hover:bg-[var(--hkui-default-100)]",
	primary:
		"bg-transparent text-[var(--hkui-primary)] hover:bg-[var(--hkui-primary-50)]",
	secondary:
		"bg-transparent text-[var(--hkui-secondary)] hover:bg-[var(--hkui-secondary-50)]",
	success:
		"bg-transparent text-[var(--hkui-success)] hover:bg-[var(--hkui-success-50)]",
	warning:
		"bg-transparent text-[var(--hkui-warning-700)] hover:bg-[var(--hkui-warning-50)]",
	danger:
		"bg-transparent text-[var(--hkui-danger-text)] hover:bg-[var(--hkui-danger-50)] dark:hover:bg-[rgb(var(--color-status-danger-rgb)/0.16)]",
};

// flat / faded / shadow / ghost are unused by migrated call sites (0 uses) but
// kept as the design-system API surface; reuse solid/bordered/light values.
const buttonVariantClass: Record<ButtonVariant, Record<ButtonColor, string>> = {
	solid: solidColorClass,
	bordered: borderedColorClass,
	light: lightColorClass,
	flat: lightColorClass,
	faded: borderedColorClass,
	shadow: solidColorClass,
	ghost: borderedColorClass,
};

export function buttonClasses({
	variant = "solid",
	color = "primary",
	size = "md",
	radius,
	fullWidth = false,
	isIconOnly = false,
	isLoading = false,
}: ButtonStyleOpts = {}): string {
	return cn(
		"relative inline-flex select-none items-center justify-center whitespace-nowrap border border-transparent font-semibold tracking-normal outline-none [font-family:var(--hkui-font-sans)] transition-[background,border-color,color,box-shadow,transform,opacity] duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none",
		// Two rings, deliberately: the box-shadow halo is the brand look, and a
		// real `outline` is the one a caller cannot erase. Loading (and any
		// caller passing `shadow-none!`) wipes the box-shadow, which used to
		// leave the focused button with NO indicator at all.
		"focus-visible:![box-shadow:var(--hkui-focus-ring)]",
		"focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--hkui-focus)]",
		"disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
		"aria-disabled:pointer-events-none aria-disabled:cursor-not-allowed aria-disabled:opacity-50",
		"active:scale-[0.97] motion-reduce:translate-none motion-reduce:scale-100",
		buttonSizeClass[size],
		isIconOnly ? buttonIconSizeClass[size] : buttonPaddingClass[size],
		radius ? buttonRadiusClass[radius] : buttonDefaultRadiusClass[size],
		buttonVariantClass[variant][color],
		fullWidth && "w-full",
		// Every declaration in this class carries `!`, so it beats caller
		// decoration wherever it lands — it does NOT need applying a second
		// time after `className` (which is what the component used to do).
		isLoading && buttonLoadingStateClass,
	);
}

/** @import import Button from '@/components/ui/Button'; */
const Button = React.forwardRef<
	HTMLButtonElement & HTMLAnchorElement,
	ButtonProps
>(function Button(
	{
		variant = "solid",
		color = "primary",
		size = "md",
		radius,
		fullWidth = false,
		isLoading = false,
		isIconOnly = false,
		href,
		target,
		rel,
		startContent,
		endContent,
		loadingLabel,
		loadingReservationLabel,
		loadingIndicator,
		className = "",
		children,
		disabled,
		type = "button",
		...rest
	},
	ref,
) {
	const classes = cn(
		buttonClasses({
			variant,
			color,
			size,
			radius,
			fullWidth,
			isIconOnly,
			isLoading,
		}),
		className,
	);
	const { "aria-busy": ariaBusy, ...restProps } = rest;
	const resolvedAriaBusy = isLoading ? true : ariaBusy;
	const resolvedLoadingLabel = loadingLabel ?? "處理中…";
	const resolvedLoadingReservationLabel =
		loadingReservationLabel ?? resolvedLoadingLabel;
	// Explicit loading content participates in layout before the request starts.
	// The button therefore reserves the larger of its normal and pending labels
	// and never jumps — or clips a useful status — when the state flips.
	const reserveLoadingWidth =
		loadingLabel !== undefined ||
		loadingReservationLabel !== undefined ||
		loadingIndicator !== undefined ||
		isLoading;
	const indicatorReservationSize = reservedIndicatorSize(
		loadingIndicator,
		size,
	);
	// The idle sizing clone must not mount a real LOGO/CSS animation.
	// A same-size inert box preserves geometry without doing invisible work.
	const loadingIndicatorReservation = (
		<span
			className="block shrink-0"
			style={{
				width: indicatorReservationSize,
				height: indicatorReservationSize,
			}}
		/>
	);

	const inner = (
		// `grid-cols-[minmax(0,1fr)]`: the reservation clone below is `w-max`, so an
		// `auto` track floors at its min-content width. Callers that hard-lock the
		// button's geometry (e.g. `.market-product-card__cart-btn`'s 32/40px
		// icon-only chip, set from a container query) then get a track wider than
		// the button — and `justify-items-center` centres the *visible* content in
		// that oversized track, pushing the icon outside the pill. `minmax(0, …)`
		// lets the track collapse to the button; an `fr` max still resolves to the
		// max-content contribution when the button sizes itself, so idle-width
		// reservation is unaffected.
		<span className="relative grid min-w-0 grid-cols-[minmax(0,1fr)] items-center justify-items-center">
			<span
				className={cn(
					"col-start-1 row-start-1 inline-flex items-center justify-center gap-[inherit]",
					isLoading && "opacity-0",
				)}
				aria-hidden={isLoading || undefined}
			>
				{startContent}
				{children}
				{endContent}
			</span>
			{reserveLoadingWidth && (
				<span
					// `max-w-full` keeps the (invisible) clone from spilling out of a
					// hard-locked button. Percentage max-width is ignored during
					// intrinsic sizing, so it still reserves its full width whenever
					// the button is free to size itself.
					className="pointer-events-none col-start-1 row-start-1 inline-flex max-w-full items-center justify-center text-current opacity-0"
					aria-hidden="true"
				>
					<ButtonLoadingContent
						label={resolvedLoadingReservationLabel}
						indicator={loadingIndicatorReservation}
						size={size}
						isIconOnly={isIconOnly}
						announce={false}
						constrain={false}
					/>
				</span>
			)}
			{isLoading && (
				<span className="pointer-events-none absolute inset-0 inline-flex min-w-0 max-w-full items-center justify-center text-current">
					<ButtonLoadingContent
						label={resolvedLoadingLabel}
						indicator={loadingIndicator}
						size={size}
						isIconOnly={isIconOnly}
					/>
				</span>
			)}
		</span>
	);

	if (href !== undefined) {
		const {
			onClick,
			"aria-disabled": ariaDisabled,
			...anchorRest
		} = restProps as React.AnchorHTMLAttributes<HTMLAnchorElement>;
		const linkDisabled =
			disabled || isLoading || ariaDisabled === true || ariaDisabled === "true";
		return (
			<Link
				{...anchorRest}
				ref={ref as React.Ref<HTMLAnchorElement>}
				href={href}
				target={target}
				rel={rel}
				data-hkui-button=""
				className={classes}
				aria-disabled={linkDisabled || undefined}
				aria-busy={resolvedAriaBusy}
				onClick={linkDisabled ? (e) => e.preventDefault() : onClick}
			>
				{inner}
			</Link>
		);
	}

	return (
		<button
			{...restProps}
			ref={ref as React.Ref<HTMLButtonElement>}
			type={type}
			data-hkui-button=""
			className={classes}
			disabled={disabled || isLoading}
			aria-busy={resolvedAriaBusy}
		>
			{inner}
		</button>
	);
});

export default Button;
export type {
	ButtonColor,
	ButtonProps,
	ButtonRadius,
	ButtonSize,
	ButtonVariant,
};
