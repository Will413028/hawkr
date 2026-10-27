import localFont from "next/font/local";

/**
 * Roboto — base UI/body face, exposed on `--font-roboto`
 * (styles/foundation/01-tokens-theme.css).
 *
 * Self-hosted via next/font/local rather than next/font/google for the
 * same reason as Zen Maru Gothic: the Google loader makes Turbopack
 * fetch the font at compile/request time, which flakes in CI / offline
 * builds (vercel/next.js#61886). Vendoring makes builds deterministic;
 * browser-side self-hosting behaviour is unchanged.
 *
 * Modern Roboto is a single variable font (wght axis 100–900), so the
 * Latin-subset WOFF2 Google serves for 400/500/700 is byte-identical —
 * one file covers the whole range. Regen procedure: lib/fonts/README.md.
 */
export const roboto = localFont({
	src: "./files/roboto-latin.woff2",
	weight: "100 900",
	style: "normal",
	variable: "--font-roboto",
	display: "optional",
	// Route-specific brand surfaces replace the base body stack, so a global
	// preload is unused on those pages. `optional` also prevents a slow cold
	// font response from swapping metrics after the page loader has left.
	preload: false,
});
