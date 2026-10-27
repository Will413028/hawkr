export type InputVariant = "flat" | "bordered" | "faded" | "underlined";
export type InputSize = "sm" | "md" | "lg";

// gap + font-size per size (always applied; never conflicts with anything).
const SIZE_TYPO: Record<InputSize, string> = {
	sm: "gap-1.5 text-xs",
	md: "gap-2 text-sm",
	lg: "gap-2.5 text-base",
};
// min-height per size (single-line, non-floating, non-textarea).
// md is the app-wide default and now clears the 44px touch target outright.
// sm is a deliberately dense desktop-workspace size: it keeps its 32px on a
// mouse and only grows to 44px on coarse pointers (touch).
const SIZE_MINH: Record<InputSize, string> = {
	sm: "min-h-8 pointer-coarse:min-h-11",
	md: "min-h-11",
	lg: "min-h-12",
};
// horizontal padding per size (lg breathes wider). 12/12/16px.
const SIZE_PX: Record<InputSize, string> = {
	sm: "px-3",
	md: "px-3",
	lg: "px-4",
};
// corner radius per size (sm 8px, md/lg 12px).
const SIZE_RADIUS: Record<InputSize, string> = {
	sm: "rounded-lg",
	md: "rounded-hkui-md",
	lg: "rounded-hkui-md",
};

// --- Floating-label vertical layout (2026-09-10 產品負責人拍板：浮動標籤填入後上浮
// 保持可見). The floated label parks at `top-2` and is clamped to the 12px font
// floor, so it owns 8–20px of the field measured from the wrapper's PADDING box
// — the control has to start below that.
//
// The room is carved out of the CONTROL, not the wrapper: leading / trailing
// adornments (DatePicker's calendar button, the dropdown chevron) must stay
// centred on the whole field box, and wrapper padding would shift them ~5px
// down with the shortened content box. Both the label (`absolute`, so also
// padding-box relative) and the control therefore measure from the same origin,
// which makes the label/text gap identical across 1px and 2px border variants.
//
// The targets are addressed by `data-slot`, never by BEM class: Tailwind turns
// `_` inside an arbitrary variant into a whitespace combinator, so
// `[&_.hkui-dropdown__value]` silently compiles to `& .hkui-dropdown  value`
// and matches nothing. Attribute selectors also win over `CONTROL_RESET`'s
// `p-0` on specificity (0,2,0 vs 0,1,0), so this never depends on Tailwind's
// generation order.
const SIZE_FLOATING_PAD: Record<InputSize, string> = {
	// 20 + 16 (text-xs line) + 4 = 40 → min-h-11 boxes it at 44.
	sm: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-1 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-1",
	// 20 + 20 (text-sm line) + 4 = 44 → 46/48 with a 1px/2px border, min-h-12
	// lands every variant on the same 48px.
	md: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-1 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-1",
	// 20 + 24 (text-base line) + 0 = 44. `leading-6` already carries 4px of
	// half-leading under the glyphs, so lg keeps its existing 48px height
	// instead of growing to 56.
	lg: "[&>[data-slot=control]]:pt-5 [&>[data-slot=control]]:pb-0 [&_[data-slot=dropdown-value]]:pt-5 [&_[data-slot=dropdown-value]]:pb-0",
};
// Floating min-height per size. sm's dense 32px cannot hold a 12px label plus a
// 12px value, so floating sm goes to the 44px touch target outright; md is the
// documented 44 → 48 step; lg is unchanged.
const SIZE_FLOATING_MINH: Record<InputSize, string> = {
	sm: "min-h-11",
	md: "min-h-12",
	lg: "min-h-12",
};
// Textarea floating: the label pins top-left inside the wrapper's own py-2.5
// (it shrinks in place rather than travelling), so the first text line only has
// to clear the 12px label — 10px wrapper padding + 12px here = 22px.
// Highlighted editors reserve this space OUTSIDE their scrolling viewport so
// both the visible backdrop and the native caret start below the label.
const TEXTAREA_FLOATING_PAD =
	"[&>[data-slot=textarea]]:pt-3 [&>[data-slot=textarea-editor]]:pt-3";
// Border WIDTH per variant (static — set once so it never conflicts with a
// `border` from BASE; there is intentionally no border on BASE). Border
// COLOR is resolved separately by state below.
const VARIANT_BORDER_WIDTH: Record<InputVariant, string> = {
	flat: "border", // 1px (transparent until focus, reserves no-shift space)
	bordered: "border-2",
	faded: "border",
	underlined: "border-b-2",
};

// --- Control/textarea reset utilities (the elements carry data-slot=control /
// data-slot=textarea). There is NO global control font reset,
// so the font inheritance below is load-bearing, not decorative. ---
const RESET_BASE =
	"flex-1 bg-transparent border-none outline-none text-inherit w-full min-w-0 p-0 placeholder:text-[var(--hkui-field-copy)]";
// `[font:inherit]` inherits the wrapper's CJK font + per-size font-size. Safe
// here: controls set no line-height, so the shorthand's line-height is inert.
export const CONTROL_RESET = `${RESET_BASE} [font:inherit]`;
// Font LONGHANDS (not the `font` shorthand) so the shorthand's implicit
// line-height cannot race `leading-6` (Tailwind conflict = generated-order).
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
	// The outlined default follows the Apps' InputDecoration geometry. Keep
	// explicit flat/faded/underlined variants available for compact surfaces.
	if (variant === "bordered") {
		const stroke = disabled
			? "border-[color:var(--hkui-input-disabled-border)]"
			: invalid
				? "border-danger"
				: focused
					? "border-[color:var(--theme-text-accent)]"
					: "border-[color:var(--theme-border-input)]";
		// Add only the inner half/one pixel: the stroke thickens without moving
		// the text or adding an outer halo (App focus 1.5px, focused error 2px).
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

	// --- background (once; disabled & flat-focus override the variant base) ---
	// Disabled dims the CHROME only (background + border). The old
	// `opacity-[0.55]` sat on the wrapper and therefore also faded the value
	// text — a mid-tone token at 55% falls well under 4.5:1, and `busy`/pending
	// fields render in this same state, so a submitting form went unreadable.
	let bg: string;
	if (disabled) bg = "bg-default-100/55";
	else if (variant === "flat")
		bg = focused ? "bg-default-50" : "bg-default-100";
	else if (variant === "faded") bg = "bg-default-50";
	else bg = "bg-transparent"; // bordered, underlined

	// --- border color (once; invalid > focused > variant base) ---
	const borderSide = underlined ? "border-b" : "border";
	let borderColor: string;
	if (disabled) {
		// Literal class strings (never built by concatenation) so Tailwind's
		// source scanner can see every one of them.
		if (underlined) borderColor = "border-[color:var(--hkui-input-border)]/55";
		else if (variant === "faded") borderColor = "border-default-200/55";
		else borderColor = "border-transparent";
	} else if (invalid) borderColor = "border-danger";
	else if (focused) borderColor = `${borderSide}-hkui-primary`;
	else if (underlined) borderColor = "border-[color:var(--hkui-input-border)]";
	else if (variant === "faded") borderColor = "border-default-200";
	else borderColor = "border-transparent"; // flat idle

	// --- hover (pseudo-class layer — safe; suppressed once active/invalid) ---
	let hover = "";
	if (!disabled && !focused && !invalid) {
		if (variant === "flat") hover = "hover:bg-default-200";
		else if (variant === "faded") hover = "hover:bg-default-100";
		else if (underlined) hover = "hover:border-b-default-400";
	}

	// --- focus halo (box-shadow). flat + bordered share the brand ink glow
	// (--hkui-input-focus-ring, mirrored by the global bare-control baseline);
	// invalid gets the SAME geometry in 柿橘 (--page-input-focus-ring-danger)
	// rather than the old bare 1px ring, so the error state is never the
	// weaker focus indicator. ---
	let halo = "";
	if (focused && invalid) halo = "shadow-[var(--page-input-focus-ring-danger)]";
	else if (focused && variant === "flat")
		halo = "shadow-[var(--hkui-input-focus-ring)]";

	// --- box metrics (align / px / radius / min-h+py — each once) ---
	const align = textarea ? "items-start" : "items-center";
	const px = underlined ? "px-0" : textarea ? "px-3" : SIZE_PX[size];
	const radius = underlined ? "rounded-none" : SIZE_RADIUS[size];
	// 2026-09-10 產品負責人拍板：浮動標籤填入後上浮保持可見 — a floating field reserves
	// the top strip for the parked label (see SIZE_FLOATING_PAD). Non-floating
	// fields keep exactly the metrics they had.
	let vbox: string;
	if (textarea)
		vbox = floating
			? `min-h-[auto] py-2.5 ${TEXTAREA_FLOATING_PAD}`
			: "min-h-[auto] py-2.5";
	else if (floating)
		vbox = `${SIZE_FLOATING_MINH[size]} ${SIZE_FLOATING_PAD[size]}`;
	else vbox = SIZE_MINH[size];

	// Text tone is emitted exactly once (base OR disabled), never both, so the
	// Tailwind generation order cannot decide which `color` wins.
	const textTone = disabled
		? "text-[var(--hkui-foreground-muted)]"
		: "text-foreground";

	return [
		// structural base (NO border/align/bg/color here — per state above)
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

// Floating label element (data-slot="floating-label"). Idle = sits over the
// field as a placeholder; active = travels to the top of the field, shrinks to
// the 12px floor and STAYS VISIBLE. top / translate / font-size / tone are each
// resolved exactly once below, so no two utilities fight over one property.
const FLOATING_LABEL_BASE =
	"absolute left-3 origin-left pointer-events-none leading-none bg-transparent px-1 whitespace-nowrap font-normal transition-[top,transform,font-size,color] duration-200 motion-reduce:transition-none";

// Placeholder 字級要跟欄位內實際輸入的字同大（wrapper 的 SIZE_TYPO）——
// 否則 lg 欄位「未填的 placeholder 14px」對上「選完值的 16px」會看起來
// 不同字體設定（開店精靈的下拉 vs 輸入框就是這樣被抓包的）。
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
	// 2026-09-10 產品負責人拍板：浮動標籤填入後上浮保持可見。
	//
	// Idle (empty + unfocused): the label sits over the field as a placeholder —
	// centred on a single-line control, on the first text line of a textarea.
	// Active (focused OR filled): it travels to the top of the field, shrinks to
	// the 12px floor and stays fully opaque, so the field keeps its name while
	// you type. (It used to fade to opacity-0, which left filled fields
	// anonymous.) The wrapper reserves the matching top strip — see
	// SIZE_FLOATING_PAD / TEXTAREA_FLOATING_PAD.
	//
	// Textarea active keeps `top-2.5`: its idle position is already the top of
	// the field, so the label shrinks in place instead of travelling.
	const top = textarea ? "top-2.5" : active ? "top-2" : "top-1/2";
	const translate = textarea || active ? "translate-y-0" : "-translate-y-1/2";
	// Exactly one font-size class: the 12px floor when floated, otherwise the
	// wrapper's own size so the placeholder matches the value text.
	const fontSize = active ? "text-xs" : FLOATING_LABEL_SIZE[size];
	let tone: string;
	// Small labels use the field-copy role at rest; focused labels follow
	// theme accent so the primary action color does not disappear on dark fields.
	if (focused) tone = "text-[color:var(--theme-text-accent)]";
	else tone = "text-[color:var(--hkui-field-copy)]";
	return [FLOATING_LABEL_BASE, fontSize, top, translate, tone].join(" ");
}
