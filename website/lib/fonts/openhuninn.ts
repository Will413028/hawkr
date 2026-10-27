/**
 * Stable root-class bridge for the unicode-range-split jf-openhuninn 2.0.
 *
 * The original outlines are retained. Derived font files use the primary
 * name "Hawkr Rounded" to respect the upstream SIL OFL reserved names.
 * styles/foundation/fonts.css owns the family, swap display policy and
 * the same metric-adjusted Arial fallback previously emitted by next/font.
 *
 * Keep this class on <html>: foundation tokens resolve --font-openhuninn
 * at :root. Native CSS selects only the required shards, including new API
 * text, without a JS font loader, font preload, or per-page text subset.
 */
export const openhuninn = { variable: "hk-font-rounded" } as const;
