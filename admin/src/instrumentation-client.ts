import * as Sentry from "@sentry/nextjs";
import { scrubSentryEvent } from "@/lib/sentry/scrub";
import { tagBackendTrace } from "@/lib/sentry/trace-tag";

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart;

if (process.env.NEXT_PUBLIC_SENTRY_DSN) {
	Sentry.init({
		dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
		environment: process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NODE_ENV,
		tracesSampleRate: process.env.NEXT_PUBLIC_APP_ENV === "prod" ? 0.1 : 0,
		propagateTraceparent: true,
		tracePropagationTargets: [/^\/api\/bff/],
		beforeSend: (event, hint) => scrubSentryEvent(tagBackendTrace(event, hint)),
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
		} catch {}
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
