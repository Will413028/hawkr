import localFont from "next/font/local";

/**
 * Zen Maru Gothic — rounded Japanese gothic, Latin safety net behind
 * openhuninn in `--font-body` (rounded feel on Latin / digit / English
 * fragments; Chinese cascades through openhuninn → OS CJK stack).
 *
 * Self-hosted via next/font/local rather than next/font/google: the
 * Google loader makes Turbopack fetch the WOFF2 from fonts.gstatic.com
 * at compile/request time, which fails sporadically in CI / offline
 * builds ("Can't resolve '@vercel/turbopack-next/internal/font/google/
 * font'", vercel/next.js#61886). Vendoring the files removes that
 * network dependency and makes builds deterministic. The browser-side
 * behaviour is identical — next/font self-hosts either way.
 *
 * Files are the Latin-subset WOFF2 (~11 KB each) Google serves for each
 * weight, so rendering is byte-identical to the previous setup. Zen Maru
 * Gothic has discrete weights (not variable), hence one file per weight.
 * Regen procedure lives in `lib/fonts/README.md`.
 *
 * Variable lands on `--font-zen-maru` so home.css can reference it from
 * a stable name.
 */
export const zenMaruGothic = localFont({
	src: [
		{
			path: "./files/zen-maru-gothic-latin-400.woff2",
			weight: "400",
			style: "normal",
		},
		{
			path: "./files/zen-maru-gothic-latin-500.woff2",
			weight: "500",
			style: "normal",
		},
		{
			path: "./files/zen-maru-gothic-latin-700.woff2",
			weight: "700",
			style: "normal",
		},
		{
			path: "./files/zen-maru-gothic-latin-900.woff2",
			weight: "900",
			style: "normal",
		},
	],
	variable: "--font-zen-maru",
	display: "optional",
	// This helper is imported by the shared public route graph, including
	// pages that never apply `--font-zen-maru`. Preloading all four weights
	// globally therefore produces Chrome's "preloaded but not used" warning on
	// /feed and /market/products. Let CSS request it only where used and keep a
	// slow cold visit on the metric-compatible fallback instead of swapping
	// after the stable content handoff.
	preload: false,
});
