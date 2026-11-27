import * as Sentry from "@sentry/nextjs";
import { beginRouterTransitionPending } from "@/lib/loading/nav-pending";
import { armNavigationWatchdog } from "@/lib/loading/navigation-watchdog";
import { STALE_CHUNK_RELOAD_MARKER_KEY } from "@/lib/loading/stale-chunk-reload";
import { beginMainTabPending } from "@/lib/navigation/main-tab-pending";
import { markRouterTransitionStarted } from "@/lib/navigation/observability";
import { clientBeforeSend } from "@/lib/sentry/client-before-send";

function runNavigationHookSafely(callback: () => void): void {
	try {
		callback();
	} catch {
	}
}

export function onRouterTransitionStart(
	...args: Parameters<typeof Sentry.captureRouterTransitionStart>
): void {
	runNavigationHookSafely(() => Sentry.captureRouterTransitionStart?.(...args));
	runNavigationHookSafely(() => markRouterTransitionStarted(args[0], args[1]));
	runNavigationHookSafely(() => armNavigationWatchdog(args[0]));
	runNavigationHookSafely(() => beginRouterTransitionPending(args[0], args[1]));
	runNavigationHookSafely(() => beginMainTabPending(args[0], args[1]));
}

const isProdFlavor =
	(process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NEXT_PUBLIC_FLAVOR) ===
	"prod";

if (process.env.NEXT_PUBLIC_SENTRY_DSN) {
	Sentry.init({
		dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
		enabled: process.env.NODE_ENV !== "development",
		environment:
			process.env.NEXT_PUBLIC_APP_ENV ??
			process.env.NEXT_PUBLIC_FLAVOR ??
			process.env.NODE_ENV,
		tracesSampleRate: isProdFlavor ? 0.1 : 1.0,
		integrations: [
			Sentry.replayIntegration({
				maskAllText: true,
				maskAllInputs: true,
				blockAllMedia: true,
			}),
		],
		replaysSessionSampleRate: 0,
		replaysOnErrorSampleRate: 1.0,
		propagateTraceparent: true,
		tracePropagationTargets: [
			/^\/api\/v1/,
			...(process.env.NEXT_PUBLIC_API_BASE_URL?.startsWith("http")
				? [process.env.NEXT_PUBLIC_API_BASE_URL.trim()]
				: []),
		],
		ignoreErrors: [
			"AbortError",
			'["@context"]',
			/window\.webkit\.messageHandlers/,
			/this\.config\.endpoints\.byUrl/,
		],
		beforeSend: (event, hint) => clientBeforeSend(event, hint),
	});
}

if (typeof window !== "undefined") {
	const RELOAD_STAMP_KEY = "__chunk_reload_at";
	const RELOAD_DEBOUNCE_MS = 10_000;
	let reloadedInThisRuntime = false;

	const reloadForStaleChunk = () => {
		if (reloadedInThisRuntime) return;
		try {
			const last = Number(
				window.sessionStorage.getItem(RELOAD_STAMP_KEY) ?? "0",
			);
			if (last && Date.now() - last < RELOAD_DEBOUNCE_MS) return;
			window.sessionStorage.setItem(RELOAD_STAMP_KEY, String(Date.now()));
			window.sessionStorage.setItem(
				STALE_CHUNK_RELOAD_MARKER_KEY,
				String(Date.now()),
			);
		} catch {
		}
		reloadedInThisRuntime = true;
		window.location.reload();
	};

	window.addEventListener(
		"error",
		(event) => {
			const target = event.target;
			if (
				(target instanceof HTMLScriptElement &&
					target.src.includes("/_next/static/")) ||
				(target instanceof HTMLLinkElement &&
					target.href.includes("/_next/static/"))
			) {
				reloadForStaleChunk();
			}
		},
		true,
	);

	window.addEventListener("unhandledrejection", (event) => {
		const reason = event.reason as
			| { name?: string; message?: string }
			| undefined;
		const isChunkError =
			reason?.name === "ChunkLoadError" ||
			(typeof reason?.message === "string" &&
				/Loading chunk [\w-]+ failed|Failed to fetch dynamically imported module|error loading dynamically imported module/i.test(
					reason.message,
				));
		if (isChunkError) reloadForStaleChunk();
	});
}
