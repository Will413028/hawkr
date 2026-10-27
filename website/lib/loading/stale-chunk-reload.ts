export const STALE_CHUNK_RELOAD_MARKER_KEY = "__hk_stale_chunk_reload_at";
export const STALE_CHUNK_RELOAD_CLASS = "hk-stale-chunk-reload";
export const STALE_CHUNK_RELOAD_PENDING_CLASS = "hk-stale-chunk-reload-pending";

// The old page remains visible while a slow connection downloads the new
// document. Give that handoff a full minute so the marker still reaches the
// boot script on constrained mobile networks; it is consumed on first read.
const STALE_CHUNK_RELOAD_MARKER_TTL_MS = 60_000;

/**
 * Runs before first paint. A stale-chunk recovery is a technical refresh, not a
 * fresh visit, so keep the full branded splash hidden and expose only the slim
 * progress rail while the new document streams in.
 *
 * The one-shot marker is consumed here. `__chunk_reload_at` remains separate in
 * instrumentation-client.ts because it protects genuinely broken builds from
 * entering a reload loop.
 */
export const STALE_CHUNK_RELOAD_BOOT_SCRIPT = `!function(){try{var k=${JSON.stringify(
	STALE_CHUNK_RELOAD_MARKER_KEY,
)},v=Number(window.sessionStorage.getItem(k)||"0");window.sessionStorage.removeItem(k);if(v&&Date.now()-v<${STALE_CHUNK_RELOAD_MARKER_TTL_MS}){document.documentElement.classList.add(${JSON.stringify(
	STALE_CHUNK_RELOAD_CLASS,
)},${JSON.stringify(STALE_CHUNK_RELOAD_PENDING_CLASS)})}}catch(e){}}();`;
