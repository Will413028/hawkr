import * as Sentry from "@sentry/nextjs";
import { scrubSentryEvent } from "@/lib/sentry/scrub";
import { tagBackendTrace } from "@/lib/sentry/trace-tag";

export const onRouterTransitionStart = Sentry.captureRouterTransitionStart;

if (process.env.NEXT_PUBLIC_SENTRY_DSN) {
	Sentry.init({
		dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
		environment: process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NODE_ENV,
		tracesSampleRate: process.env.NEXT_PUBLIC_APP_ENV === "prod" ? 0.1 : 0,
		// 多送 W3C traceparent，讓 gateway（只讀 traceparent）續同一條 trace。
		// admin 走同源 BFF，故只傳播到 /api/bff（非直打 gateway）。
		propagateTraceparent: true,
		tracePropagationTargets: [/^\/api\/bff/],
		beforeSend: (event, hint) => scrubSentryEvent(tagBackendTrace(event, hint)),
	});
}

// Deploy-skew self-heal（見 hawkr-website 的 instrumentation-client.ts）。
// Cloudflare Workers Assets 每次部署整批替換靜態資源、不保留舊版本，停留在
// 舊頁面的分頁會對已刪除的 `/_next/static/*` chunk 404。偵測到即 reload 一次
// 拉最新版本。module 旗標把每個 page runtime 限一次 reload（涵蓋
// sessionStorage 被封鎖時，一次部署觸發多個 chunk 404 仍只 reload 一次）；
// sessionStorage 戳記在 reload 邊界外再擋 10 秒內的重複 reload，避免真的壞掉
// 的 build（非版本落差）造成 reload 迴圈。
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
		} catch {
			// sessionStorage 被擋（無痕模式等）：module 旗標仍把這次 runtime 限一次 reload。
		}
		reloadedInThisRuntime = true;
		window.location.reload();
	};

	// (a) 指向已刪除 build chunk 的 <script>/<link> 會 404。resource load error
	// 不會冒泡，故用 capture phase 監聽。
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

	// (b) 對已刪除 chunk 的動態 import() 會以 ChunkLoadError reject。
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
