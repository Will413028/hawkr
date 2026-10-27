import type { CSSProperties } from "react";

/**
 * Shared "feature card" chrome for the megamenu right-slot surfaces — the
 * banner carousel, the 本週主題 weekly-post carousel, their empty states, and
 * the loading card. Centralising it keeps every right-slot card
 * visually identical: a warm cream fill, a hairline inset ring, and a soft
 * low-elevation shadow — the same low-elevation, rounded card language the
 * brand already leans on (cf. sweetgreen / Seed / Limón).
 */
export const FEATURE_CARD_CHROME =
	"bg-[var(--hkui-content2)] ring-1 ring-inset ring-[rgb(var(--color-ink-800-rgb)/0.07)] dark:ring-[rgba(245,222,165,0.12)] rounded-[var(--hkui-radius-xl)] shadow-[0_18px_40px_-26px_rgb(var(--color-ink-950-rgb)/0.5)]";

/**
 * Layered soft-light glow painted on top of the flat `--hkui-content2` fill:
 * a low-alpha sage glow (top-left) + warm honey glow (bottom-right) + a faint
 * top sheen. Gives the cream card paper-like depth without reading as a hard
 * gradient, over both the light and dark base colours.
 */
export const FEATURE_CARD_BG: CSSProperties = {
	backgroundImage:
		"radial-gradient(120% 84% at 0% 0%, rgb(var(--color-ink-600-rgb) / 0.14), transparent 56%)," +
		"radial-gradient(124% 92% at 100% 100%, rgba(241,205,112,0.18), transparent 60%)," +
		"linear-gradient(158deg, rgba(255,255,255,0.10), rgba(255,255,255,0) 42%)",
};

/**
 * Inner image / media well used inside a feature card — soft rounded corners,
 * a faint sage wash, and a hairline inset ring so photography and the loading
 * truck both sit on the same surface.
 */
export const FEATURE_CARD_WELL =
	"relative w-full overflow-hidden rounded-[var(--hkui-radius-md)] bg-[rgb(var(--color-ink-800-rgb)/0.06)] ring-1 ring-inset ring-[rgb(var(--color-ink-950-rgb)/0.06)]";
