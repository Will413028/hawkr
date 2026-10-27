import { STRIP_LOCALE_SCRIPT } from "../intl/paths";

/** React starts the graceful fade before the browser-level hard deadline. */
export const INITIAL_LOADER_REACT_FALLBACK_MS = 5_500;

/**
 * Document-bootstrap deadline outside the loader component lifecycle. It is not
 * reset by a Strict Mode effect replay or a Fast Refresh remount. CSS carries
 * the same deadline for the case where the Next.js bootstrap never runs.
 */
export const INITIAL_LOADER_WATCHDOG_MS = 6_000;
export const INITIAL_LOADER_TIMEOUT_CLASS = "hk-initial-loader-timeout";
export const INITIAL_LOADER_SKIP_CLASS = "hk-skip-initial-loader";

export const INITIAL_LOADER_WATCHDOG_SCRIPT = `!function(){var r=document.documentElement,strip=${STRIP_LOCALE_SCRIPT};if(window.location&&/^\\/(?:login|register)(?:\\/|$)/.test(strip(window.location.pathname))){r.classList.remove("initial-loader-active");r.classList.add(${JSON.stringify(
	INITIAL_LOADER_SKIP_CLASS,
)});return}window.setTimeout(function(){r.classList.add(${JSON.stringify(
	INITIAL_LOADER_TIMEOUT_CLASS,
)})},${INITIAL_LOADER_WATCHDOG_MS})}();`;
