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
		// Instrumentation is observational. It must never block Next from
		// dispatching the real navigation action.
	}
}

// Compose Sentry's transition capture with the navigation watchdog so a hung
// soft-nav (RSC fetch that never rejects) can't leave the user stuck on the
// route skeleton — see lib/loading/navigation-watchdog.ts. The top progress
// rail hold also starts here: transition start is the only signal that fires
// at navigation BEGIN for programmatic router.push (bottom-nav 滑動手勢、
// 分類 chip、搜尋鈕)——anchor 點擊與其一視同仁（lib/loading/nav-pending.ts）。
export function onRouterTransitionStart(
	...args: Parameters<typeof Sentry.captureRouterTransitionStart>
): void {
	runNavigationHookSafely(() => Sentry.captureRouterTransitionStart?.(...args));
	runNavigationHookSafely(() => markRouterTransitionStarted(args[0], args[1]));
	runNavigationHookSafely(() => armNavigationWatchdog(args[0]));
	runNavigationHookSafely(() => beginRouterTransitionPending(args[0], args[1]));
	runNavigationHookSafely(() => beginMainTabPending(args[0], args[1]));
}

// NODE_ENV is always "production" for both the dev and prod deployment
// (both are production builds — see sentry.server.config.ts), so it can't
// distinguish them. NEXT_PUBLIC_FLAVOR can, and is what `environment` below
// already keys off of.
const isProdFlavor =
	(process.env.NEXT_PUBLIC_APP_ENV ?? process.env.NEXT_PUBLIC_FLAVOR) ===
	"prod";

if (process.env.NEXT_PUBLIC_SENTRY_DSN) {
	Sentry.init({
		dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
		// 本機 next dev（NODE_ENV=development）不上報：dev 專案曾被本機 runtime
		// 噪音污染（DEV-2 全部來自 MacBook）。部署版（含 dev flavor）NODE_ENV
		// 恆為 production，不受影響。
		enabled: process.env.NODE_ENV !== "development",
		environment:
			process.env.NEXT_PUBLIC_APP_ENV ??
			process.env.NEXT_PUBLIC_FLAVOR ??
			process.env.NODE_ENV,
		// Pre-launch, dev traffic is low — sample every transaction to get a
		// statistically meaningful Web Vitals baseline. Dial back once real
		// prod traffic makes 100% too expensive.
		tracesSampleRate: isProdFlavor ? 0.1 : 1.0,
		// Session Replay：白屏那刻的 DOM/互動時間線是本站前端事故唯一的事後重建
		// 手段（#339）。這是金流 / PII 站，rrweb 錄製一律 deny-by-default 三重遮罩：
		// maskAllText（含以文字呈現的欄位）、maskAllInputs（所有 input 值）、
		// blockAllMedia（不錄商品圖 / 頭像）。networkDetailAllowUrls 留空＝不錄
		// request/response body。與既有 beforeSend→scrubSentryEvent 正交、缺一不可
		// （scrub 只作用於 error event 結構化欄位，不管 Replay 的 DOM 錄製）。
		integrations: [
			Sentry.replayIntegration({
				maskAllText: true,
				maskAllInputs: true,
				blockAllMedia: true,
			}),
		],
		// Replay 配額是**硬上限**，不是計費彈性：org 方案 Team 的 replay
		// reserved = **50 / 每計費週期**（2026-07-29 實測 `/customers/<org>/`：
		// reserved=50、used=52）。50 筆撐不起任何比例的 session 取樣。
		//
		// 實證：dev 以 `1.0` 取樣，於 07-28 單日錄滿 52 筆用盡整個週期配額；07-29 起
		// org 層 429（`x-sentry-rate-limits: …:replay:organization:replay_usage_exceeded`）
		// 擋掉 234 筆。rate limit 的 scope 是 **organization**、兩 project 共用同一
		// 個 org → **dev 的取樣率直接決定 prod 有沒有事後重建能力**。#339 白屏案賴以
		// 閉合的手段，就是這樣被 dev 的背景錄製吃光的。
		//
		// 因此 session 取樣一律 0：配額全額保留給 `replaysOnErrorSampleRate`，
		// 也就是「真的出事」那 50 筆。這不是省錢，是把稀缺資源放在唯一有事後價值的地方。
		//
		// ⚠️ 調高之前必須先加購配額，否則調高＝把 prod 的觀測能力吃掉。
		// `tests/sentry-replay-quota.test.ts` 以契約測試釘住此值，防止默默調回。
		// 已知代價：**不報錯的壞畫面**（例如卡在 skeleton 而未拋例外的白屏）不會觸發
		// onError buffer，因此錄不到。要覆蓋該類情境必須加購配額後才談恢復 session 取樣。
		replaysSessionSampleRate: 0,
		replaysOnErrorSampleRate: 1.0,
		// 多送 W3C traceparent，讓 gateway（只讀 traceparent）續同一條 trace。
		// 設了 tracePropagationTargets 後同源預設會被取代，故 dev 同源路徑也要列。
		propagateTraceparent: true,
		tracePropagationTargets: [
			/^\/api\/v1/,
			...(process.env.NEXT_PUBLIC_API_BASE_URL?.startsWith("http")
				? [process.env.NEXT_PUBLIC_API_BASE_URL.trim()]
				: []),
		],
		// 已確認的非錯誤噪音，不上報：
		// - AbortError：導航/切頁取消 in-flight fetch 的正常訊號
		//   （Safari「Fetch is aborted」/ Chrome「The user aborted a request」）
		// - ["@context"]：Safari 擴充套件解析頁面 JSON-LD 失敗，
		//   codebase 沒有任何讀取 @context 的程式
		ignoreErrors: [
			"AbortError",
			'["@context"]',
			// Native WKWebView / browser extensions inject these scripts; the web
			// bundle contains neither API and cannot repair their host-side failures.
			/window\.webkit\.messageHandlers/,
			/this\.config\.endpoints\.byUrl/,
		],
		// 先丟棄 *.workers.dev 預覽網域的事件（CI cf-smoke 自產噪音：該網域結構上
		// 沒有 first-party API proxy，/api/v1/* 必 404 且屬設計行為），再對真正要
		// 上報的事件補後端 trace tag 與去識別化。正式網域一律不受影響——那裡的同類
		// 錯誤才是真缺陷。詳見 lib/sentry/client-before-send.ts。
		beforeSend: (event, hint) => clientBeforeSend(event, hint),
	});
}

// Deploy-skew self-heal. Cloudflare Workers Assets replaces every static asset on
// each deploy and keeps no previous version, so a tab still holding an earlier
// build 404s on its now-deleted `/_next/static/*` chunks — and on a soft
// navigation this hangs forever on the loading skeleton (the "refresh sometimes
// fixes it" bug). Next's deploymentId only self-heals the RSC/document layer;
// these listeners cover the raw chunk 404s that slip through, reloading once to
// pull the current build. Both a <script>/<link> resource error and a dynamic
// import() rejection are handled; neither calls preventDefault, so Sentry still
// reports. Two guards stop a reload loop: a module-scope flag caps each page
// runtime to a single auto-reload (covers the storage-blocked case where one
// deploy fires several chunk 404s at once), and a sessionStorage stamp suppresses
// another reload within 10s across the reload boundary so a genuinely broken
// build (not a stale one) can't loop-reload the page.
if (typeof window !== "undefined") {
	const RELOAD_STAMP_KEY = "__chunk_reload_at";
	const RELOAD_DEBOUNCE_MS = 10_000;
	// One auto-reload per page runtime. Without this, a blocked sessionStorage
	// (private mode / partitioned) drops the debounce below, and several
	// simultaneous chunk 404s would each fire their own reload.
	let reloadedInThisRuntime = false;

	const reloadForStaleChunk = () => {
		if (reloadedInThisRuntime) return;
		try {
			const last = Number(
				window.sessionStorage.getItem(RELOAD_STAMP_KEY) ?? "0",
			);
			// Reloaded within the window and still failing → this build is genuinely
			// broken (a chunk truly missing), not version skew. Stop reloading.
			if (last && Date.now() - last < RELOAD_DEBOUNCE_MS) return;
			window.sessionStorage.setItem(RELOAD_STAMP_KEY, String(Date.now()));
			// One-shot UI handoff for the next document. The normal reload stamp
			// above remains in place to prevent loops; this separate marker is
			// consumed before first paint so a technical self-heal does not replay
			// the full branded first-visit splash.
			window.sessionStorage.setItem(
				STALE_CHUNK_RELOAD_MARKER_KEY,
				String(Date.now()),
			);
		} catch {
			// sessionStorage blocked (private mode / partitioned storage): the
			// module-scope flag above still caps this runtime to one reload.
		}
		reloadedInThisRuntime = true;
		window.location.reload();
	};

	// (a) A <script>/<link> pointing at a deleted build chunk 404s. Resource load
	// errors don't bubble, so listen in the capture phase.
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

	// (b) A dynamic import() of a deleted chunk rejects with a ChunkLoadError.
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
