export type InputVariant = "flat" | "bordered" | "faded" | "underlined";
export type InputSize = "sm" | "md" | "lg";

const SIZE_TYPO: Record<InputSize, string> = {
	sm: "gap-1.5 text-xs",
	md: "gap-2 text-sm",
	lg: "gap-2.5 text-base",
};
const SIZE_MINH: Record<InputSize, string> = {
	sm: "min-h-8 pointer-coarse:min-h-11",
	md: "min-h-11",
	lg: "min-h-12",
};
const SIZE_PX: Record<InputSize, string> = {
	sm: "px-3",
	md: "px-3",
	lg: "px-4",
};
const SIZE_RADIUS: Record<InputSize, string> = {
	sm: "rounded-lg",
	md: "rounded-hkui-md",
	lg: "rounded-hkui-md",
};

const SIZE_FLOATING_PAD: Record<InputSize, string> = {
	sm: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-1 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-1",
	md: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-1 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-1",
	lg: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-0 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-0",
};
const SIZE_FLOATING_MINH: Record<InputSize, string> = {
	sm: "min-h-11",
	md: "min-h-12",
	lg: "min-h-12",
};
const TEXTAREA_FLOATING_PAD =
	"[&>[data-slot=textarea]]:pt-3 [&>[data-slot=textarea-editor]]:pt-3";
const VARIANT_BORDER_WIDTH: Record<InputVariant, string> = {
	flat: "border",
	bordered: "border-2",
	faded: "border",
	underlined: "border-b-2",
};

// data-slot=textarea). There is NO global control font reset,
const RESET_BASE =
	"flex-1 bg-transparent border-none outline-none text-inherit w-full min-w-0 p-0 placeholder:text-[var(--hkui-field-copy)]";
export const CONTROL_RESET = `${RESET_BASE} [font:inherit]`;
export const TEXTAREA_TEXT_LAYOUT =
	"[font-family:inherit] [font-size:inherit] [font-weight:inherit] leading-6";
export const TEXTAREA_RESET = `${RESET_BASE} ${TEXTAREA_TEXT_LAYOUT} resize-y`;

export interface InputWrapperOptions {
	variant: InputVariant;
	size: InputSize;
	focused: boolean;
	invalid: boolean;
	disabled: boolean;
	/** Floating-label mode. `inputWrapperData` emits `data-floating` from it and
	 *  the class builder switches to the floating vertical layout (room at the
	 *  top for the floated label). Optional: callers with no floating label may
	 *  omit it. */
	floating?: boolean;
	textarea?: boolean;
	wrapperClassName?: string;
}

/**
 * Builds the input-wrapper className: Tailwind utilities for the look.
 * Context-scoped CSS (seller-shell dark ink-glass, shipping-row layout)
 * attaches via data attributes emitted by `inputWrapperData`.
 * Every conflicting CSS property is emitted at most once (see CRITICAL note).
 */
export function inputWrapperClasses({
	variant,
	size,
	focused,
	invalid,
	disabled,
	floating = false,
	textarea = false,
	wrapperClassName = "",
}: InputWrapperOptions): string {
	if (variant === "bordered") {
		const stroke = disabled
			? "border-[color:var(--hkui-input-disabled-border)]"
			: invalid
				? "border-danger"
				: focused
					? "border-[color:var(--theme-text-accent)]"
					: "border-[color:var(--theme-border-input)]";
		const focusStroke =
			!disabled && focused
				? invalid
					? "shadow-[inset_0_0_0_1px_var(--hkui-danger)]"
					: "shadow-[inset_0_0_0_0.5px_var(--theme-text-accent)]"
				: "shadow-none";
		return [
			"relative flex border font-[family-name:var(--hkui-font-sans)] bg-[var(--hkui-input-surface)] transition-[border-color,box-shadow] duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none",
			"text-[color:var(--hkui-field-copy)]",
			textarea ? "items-start py-3" : "items-center",
			size === "sm" ? "gap-1.5 text-xs px-3" : "gap-2 text-base px-4",
			SIZE_RADIUS[size],
			textarea
				? "min-h-[auto]"
				: size === "sm"
					? SIZE_MINH.sm
					: "min-h-[calc(1.5em_+_var(--hkui-space-6)/var(--hk-text-scale,1))]",
			stroke,
			focusStroke,
			disabled ? "cursor-not-allowed" : "",
			wrapperClassName,
		]
			.filter(Boolean)
			.join(" ");
	}
	const underlined = variant === "underlined";

	let bg: string;
	if (disabled) bg = "bg-default-100/55";
	else if (variant === "flat")
		bg = focused ? "bg-default-50" : "bg-default-100";
	else if (variant === "faded") bg = "bg-default-50";
	else bg = "bg-transparent";

	const borderSide = underlined ? "border-b" : "border";
	let borderColor: string;
	if (disabled) {
		if (underlined) borderColor = "border-[color:var(--hkui-input-border)]/55";
		else if (variant === "faded") borderColor = "border-default-200/55";
		else borderColor = "border-transparent";
	} else if (invalid) borderColor = "border-danger";
	else if (focused) borderColor = `${borderSide}-hkui-primary`;
	else if (underlined) borderColor = "border-[color:var(--hkui-input-border)]";
	else if (variant === "faded") borderColor = "border-default-200";
	else borderColor = "border-transparent";

	let hover = "";
	if (!disabled && !focused && !invalid) {
		if (variant === "flat") hover = "hover:bg-default-200";
		else if (variant === "faded") hover = "hover:bg-default-100";
		else if (underlined) hover = "hover:border-b-default-400";
	}

	// (--hkui-input-focus-ring, mirrored by the global bare-control baseline);
	let halo = "";
	if (focused && invalid) halo = "shadow-[var(--page-input-focus-ring-danger)]";
	else if (focused && variant === "flat")
		halo = "shadow-[var(--hkui-input-focus-ring)]";

	const align = textarea ? "items-start" : "items-center";
	const px = underlined ? "px-0" : textarea ? "px-3" : SIZE_PX[size];
	const radius = underlined ? "rounded-none" : SIZE_RADIUS[size];
	let vbox: string;
	if (textarea)
		vbox = floating
			? `min-h-[auto] py-2.5 ${TEXTAREA_FLOATING_PAD}`
			: "min-h-[auto] py-2.5";
	else if (floating)
		vbox = `${SIZE_FLOATING_MINH[size]} ${SIZE_FLOATING_PAD[size]}`;
	else vbox = SIZE_MINH[size];

	const textTone = disabled
		? "text-[var(--hkui-foreground-muted)]"
		: "text-foreground";

	return [
		"relative flex font-[family-name:var(--hkui-font-sans)] transition-[background-color,border-color,box-shadow] duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none",
		textTone,
		align,
		SIZE_TYPO[size],
		px,
		radius,
		vbox,
		VARIANT_BORDER_WIDTH[variant],
		borderColor,
		bg,
		hover,
		halo,
		disabled ? "cursor-not-allowed" : "",
		wrapperClassName,
	]
		.filter(Boolean)
		.join(" ");
}

/**
 * Data attributes for the input wrapper, spread onto the wrapper <div>.
 * These are the attachment point for context-scoped CSS (seller-shell
 * dark ink-glass, shipping-row layout, dropdown chevron/floating):
 *   data-slot="input-wrapper"  — always present
 *   data-variant               — "flat" | "bordered" | "faded" | "underlined"
 *   data-focus                 — present (value "") when focused, omitted otherwise
 *   data-floating              — present (value "") when floating-label is active
 * Boolean attrs use the React idiom: "" when true (selector `[data-focus]` matches),
 * undefined when false (attribute omitted).
 */
export function inputWrapperData({
	variant,
	focused,
	floating,
}: Pick<InputWrapperOptions, "variant" | "focused" | "floating">): {
	"data-slot": "input-wrapper";
	"data-variant": InputVariant;
	"data-focus"?: "";
	"data-floating"?: "";
} {
	return {
		"data-slot": "input-wrapper",
		"data-variant": variant,
		"data-focus": focused ? "" : undefined,
		"data-floating": floating ? "" : undefined,
	};
}

const FLOATING_LABEL_BASE =
	"absolute left-3 origin-left pointer-events-none leading-none bg-transparent px-1 whitespace-nowrap font-normal transition-[top,transform,font-size,color] duration-200 motion-reduce:transition-none";

const FLOATING_LABEL_SIZE: Record<InputSize, string> = {
	sm: "text-xs",
	md: "text-sm",
	lg: "text-base",
};

export function floatingLabelClasses({
	active,
	focused = false,
	textarea = false,
	size = "md",
	variant = "flat",
}: {
	active: boolean;
	/** Focus state, used only for the active tone (brand green vs muted). */
	focused?: boolean;
	textarea?: boolean;
	size?: InputSize;
	variant?: InputVariant;
}): string {
	if (variant === "bordered") {
		return [
			"absolute origin-left pointer-events-none leading-none px-1 max-w-[calc(100%-2rem)] truncate font-normal transition-[top,transform,font-size] duration-200 motion-reduce:transition-none text-[color:var(--hkui-field-copy)]",
			size === "sm" ? "left-2" : "left-3",
			active
				? "top-0 -translate-y-1/2 text-xs bg-inherit"
				: `${textarea ? "top-3" : "top-1/2 -translate-y-1/2"} ${size === "sm" ? "text-xs" : "text-base"}`,
		].join(" ");
	}
	const top = textarea ? "top-2.5" : active ? "top-2" : "top-1/2";
	const translate = textarea || active ? "translate-y-0" : "-translate-y-1/2";
	const fontSize = active ? "text-xs" : FLOATING_LABEL_SIZE[size];
	let tone: string;
	if (focused) tone = "text-[color:var(--theme-text-accent)]";
	else tone = "text-[color:var(--hkui-field-copy)]";
	return [FLOATING_LABEL_BASE, fontSize, top, translate, tone].join(" ");
}
