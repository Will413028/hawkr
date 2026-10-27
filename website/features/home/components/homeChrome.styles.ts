// Pure Tailwind class strings for the home page's shared chrome primitives.
// No React imports — unit-tested directly. Consumers fold in caller `className`
// via cn() at the component layer.

export function eyebrowClasses(onDark = false): string {
	return [
		"inline-flex items-center gap-2.5 rounded-full px-4 py-2",
		"text-xs font-bold uppercase tracking-[0.26em] whitespace-nowrap",
		// Dark mode: the light `bg-white/70` pill glared on the night canvas, so
		// match the subtle dark `.eyebrow` (glass fill, gold hairline, cream text).
		onDark
			? "bg-white/12 text-hk-home-paper"
			: "bg-white/70 text-hk-home-accent-700 dark:bg-white/[0.06] dark:border dark:border-[rgba(245,222,165,0.14)] dark:text-[rgba(255,253,247,0.78)]",
	].join(" ");
}

export const eyebrowDotClasses = "size-1.5 rounded-full bg-hk-brand-secondary";

export type ButtonVariant = "primary" | "ghost";
export type ButtonSize = "md" | "lg";

export function buttonClasses(opts: {
	variant?: ButtonVariant;
	size?: ButtonSize;
	onDark?: boolean;
}): string {
	const { variant = "primary", size = "md", onDark = false } = opts;
	const base =
		"inline-flex items-center gap-2 rounded-full font-bold no-underline whitespace-nowrap cursor-pointer transition-[transform,background,color] duration-200 ease-[ease] hover:-translate-y-px";
	const sizing =
		size === "lg"
			? "py-[18px] px-[30px] text-base"
			: "py-3.5 px-6 text-[0.9375rem]";
	let tone: string;
	if (variant === "primary") {
		tone = "bg-hk-home-accent-500 text-white hover:bg-hk-home-primary-deep";
	} else if (onDark) {
		tone =
			"border-[1.5px] bg-hk-home-paper/94 text-hk-home-primary-deep border-hk-home-paper/94 hover:bg-hk-home-paper hover:border-hk-home-paper hover:text-hk-home-deep-ink";
	} else {
		tone =
			"border-[1.5px] bg-transparent text-hk-home-ink border-hk-home-ink/22 hover:border-hk-home-ink";
	}
	return [base, sizing, tone].join(" ");
}

export type SectionAlign = "between" | "center";

export function sectionHeaderClasses(align: SectionAlign = "between"): string {
	return align === "center"
		? "max-w-[1180px] mx-auto mb-14 text-center"
		: "flex justify-between items-end gap-8 max-w-[1280px] mx-auto mb-14 px-[6vw] max-[640px]:flex-col max-[640px]:items-start";
}

export function sectionHeadingClasses(hasDescription: boolean): string {
	return [
		"font-black text-[clamp(2.125rem,4.4vw,3.75rem)] leading-[1.05] tracking-[-0.02em] text-hk-home-ink mt-4",
		hasDescription ? "mb-3" : "",
	]
		.filter(Boolean)
		.join(" ");
}

export const sectionDescriptionClasses =
	"text-base leading-[1.6] text-hk-home-ink/65 max-w-[36ch] m-0";

export const sectionMoreClasses =
	"shrink-0 pb-2 border-b-[1.5px] border-current text-sm font-bold tracking-[0.06em] text-hk-home-accent-700 no-underline hover:text-[color:var(--color-brand-primary-deep)]";

// Roboto fallback has no Tailwind token — kept as inline style, matching the
// existing pattern in HomePage.tsx (the category-card meta label).
export const sectionMoreFontStyle = {
	fontFamily: '"Roboto", var(--font-body)',
} as const;
