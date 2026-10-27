import {
  ORIGINAL_PRESET_NAME,
  PRESETS,
  isAnimatedPreset,
  isKnownPreset,
  isTransformPreset,
  type PresetName,
  type TransformPresetName,
} from "./presets";

export interface Env {
  MEDIA_BUCKET: R2Bucket;
  IMAGES: ImagesBinding;
}

/** secure/、tmp/ 不可經 Worker 讀取（fail-closed，49 號 §6.5/§8.1）。 */
const PUBLIC_KEY_PREFIX = "public/";

const IMMUTABLE_CACHE_CONTROL = "public, max-age=31536000, immutable";

/**
 * 公開圖資產 CORS：對外一律 `*`。這些是公開讀取的圖片（`<img src>` 本就能任意站嵌入），
 * 補 ACAO 只是讓瀏覽器端 `fetch()` 能讀回 bytes 與狀態碼（賣家編輯器 resolveStagedImages
 * 需要把既有圖 fetch 回瀏覽器重傳、AI 美化以既有圖為源等）。secure/、tmp/ 前綴在路由層
 * 本就 404，不因 CORS 放寬而暴露。
 *
 * Vary：ACAO 為固定常數 `*`（不反射 Origin），回應內容與請求 Origin 無關，故**不加**
 * `Vary: Origin`——否則會把共享 edge cache 依 Origin 無謂分裂。轉換類 preset 的 Accept
 * 協商已由 cache key 併入 `__fmt` 處理（見 buildCacheKeyUrl），同樣不需 `Vary: Accept`。
 */
const CORS_ALLOW_ORIGIN = "*";
const PREFLIGHT_MAX_AGE = "86400"; // 24h：preset/路由穩定，preflight 可長快取

/** `/i/{preset}/{objectKey}`，objectKey 本身可含 `/`。 */
const ROUTE_PATTERN = /^\/i\/([^/]+)\/(.+)$/;

type NegotiatedFormat = "image/avif" | "image/webp" | "image/jpeg" | "image/gif";

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    // CORS preflight：實際 GET/HEAD 圖片請求多為 simple request（無自訂 header）不觸發
    // preflight，但前端若帶非 safelisted header 會先送 OPTIONS——回 204 放行 GET/HEAD。
    if (request.method === "OPTIONS") {
      return preflightResponse();
    }

    if (request.method !== "GET" && request.method !== "HEAD") {
      return errorResponse("Not Found", 404);
    }

    const url = new URL(request.url);
    const match = ROUTE_PATTERN.exec(url.pathname);
    if (!match) {
      return errorResponse("Not Found", 404);
    }
    const [, presetParam, objectKey] = match;

    if (!isKnownPreset(presetParam)) {
      return errorResponse("Unknown preset", 400);
    }
    const preset = presetParam;

    if (!objectKey.startsWith(PUBLIC_KEY_PREFIX)) {
      return errorResponse("Not Found", 404);
    }

    const format = isTransformPreset(preset)
    ? negotiateFormat(request.headers.get("Accept"), isAnimatedPreset(preset))
    : null;
    const cache = caches.default;
    const cacheKey = new Request(buildCacheKeyUrl(url, format), { method: "GET", headers: request.headers });

    const cached = await cache.match(cacheKey);
    if (cached) {
      return finalizeResponse(cached, request.method);
    }

    const response = await buildResponse(env, objectKey, preset, format);
    if (response.status === 200) {
      ctx.waitUntil(cache.put(cacheKey, response.clone()));
    }
    return finalizeResponse(response, request.method);
  },
} satisfies ExportedHandler<Env>;

async function buildResponse(
  env: Env,
  objectKey: string,
  preset: PresetName,
  format: NegotiatedFormat | null,
): Promise<Response> {
  const object = await env.MEDIA_BUCKET.get(objectKey);
  if (object === null) {
    return new Response("Not Found", { status: 404 });
  }

  const contentType = object.httpMetadata?.contentType ?? "application/octet-stream";
  const contentDisposition = object.httpMetadata?.contentDisposition;
  // 一次讀完 body 到記憶體：objectKey 前綴白名單只放行 public/（有上傳大小上限，49 號
  // §8.1），且轉換失敗時需要原封不動 fallback 直出——緩衝一份可重複使用的 bytes，
  // 避免對同一個單次可讀 ReadableStream 重複讀取。
  const bytes = await object.arrayBuffer();

  if (preset === ORIGINAL_PRESET_NAME || !contentType.startsWith("image/") || format === null) {
    return originalResponse(bytes, contentType, contentDisposition);
  }

  try {
    return await transformedResponse(env, bytes, preset, format);
  } catch (err) {
    // Images binding transform 失敗（例如來源非 Images 支援的編碼）→ degrade 直出原檔，
    // 不破圖（49 號 §7.3）。
    console.error(`ImageGateway.transform.fallback preset=${preset} key=${objectKey}`, err);
    return originalResponse(bytes, contentType, contentDisposition);
  }
}

function originalResponse(bytes: ArrayBuffer, contentType: string, contentDisposition: string | undefined): Response {
  const headers = new Headers();
  headers.set("Content-Type", contentType);
  headers.set("Cache-Control", IMMUTABLE_CACHE_CONTROL);
  if (contentDisposition) {
    headers.set("Content-Disposition", contentDisposition);
  }
  return new Response(bytes, { status: 200, headers });
}

async function transformedResponse(
  env: Env,
  bytes: ArrayBuffer,
  preset: TransformPresetName,
  format: NegotiatedFormat,
): Promise<Response> {
  const config = PRESETS[preset];
  // 傳 `bytes.slice(0)`（複本）而非 `bytes` 本身：避免任何底層實作萬一轉移
  // （detach）傳入的 ArrayBuffer 所有權，波及 buildResponse() fallback 分支要重複使用的
  // 原始 `bytes`。
  const result = await env.IMAGES.input(new Response(bytes.slice(0)).body!)
    .transform(config.transform)
    .output(outputOptionsFor(preset, format));

  const raw = result.response();
  const outputBytes = await raw.arrayBuffer();
  // 實測（本機 workerd/Miniflare Images 模擬器）：餵入無法解碼的來源時，`.output()`
  // 不會拋 ImagesError，而是靜默回傳 200 + 0 bytes 的「成功」回應。若照字面只信任
  // try/catch 會把這種空殼回應當正常轉換結果快取下來，等同破圖。故顯式檢查輸出長度，
  // 空輸出視同轉換失敗，交由呼叫端 fallback 直出原檔（49 號 §7.3 degrade 語意）。
  if (outputBytes.byteLength === 0) {
    throw new Error("Images binding produced empty output (source likely undecodable)");
  }

  const headers = new Headers(raw.headers);
  headers.set("Cache-Control", IMMUTABLE_CACHE_CONTROL);
  return new Response(outputBytes, { status: raw.status, headers });
}

/**
 * 送給 Images binding `.output()` 的參數。
 *
 * 抽成具名純函式是為了**可驗證**：動畫保不保留取決於 CF 的編碼器，而本機
 * Miniflare 模擬器根本做不到（`image/gif` 直接回 `GIF output is not supported in
 * local mode`；`image/webp` 則靜默吐單幀），所以「輸出真的會動」只能在部署後對真
 * binding 實測。本機能守住的是**我方這一側的兩個前提**——格式沒落到靜態格式、
 * `anim` 有真的送出去——而這兩件事一旦破掉，功能就會靜默變回單張。
 */
export function outputOptionsFor(
  preset: TransformPresetName,
  format: NegotiatedFormat,
): { format: NegotiatedFormat; quality: number; anim?: boolean } {
  const config = PRESETS[preset];
  return isAnimatedPreset(preset)
    ? { format, quality: config.quality, anim: true }
    : { format, quality: config.quality };
}

/**
 * 49 號設計書假設 preset 可對 Images binding 直傳 `format=auto`，但實測
 * `ImageOutputOptions.format`（`wrangler types` 生成的 runtime types，見
 * worker-configuration.d.ts）只接受具體 MIME，無 "auto" 字面值——Worker 內自行依
 * `Accept` header 協商，效果等價於 cf.image 的 format=auto：
 * 支援 avif 優先 avif，其次 webp，都不支援則退回 jpeg（最大相容性，與現況 x-oss-process
 * 預設輸出等價）。
 */
export function negotiateFormat(acceptHeader: string | null, animated = false): NegotiatedFormat {
  const accept = acceptHeader ?? "";
  if (animated) {
    // 動圖 preset 只在能動的格式之間選。avif 不納入：CF Images 的 animated AVIF
    // 支援不穩定，而這裡「動不動」是功能本身，不是畫質偏好——寧可退回 gif 也不能
    // 賭一個會靜默變成單張的格式。
    return accept.includes("image/webp") ? "image/webp" : "image/gif";
  }
  if (accept.includes("image/avif")) {
    return "image/avif";
  }
  if (accept.includes("image/webp")) {
    return "image/webp";
  }
  return "image/jpeg";
}

/**
 * Edge cache key：immutable key 空間（objectKey 含 uuid，內容不變），直接以完整請求
 * URL 當 cache key 即可長期快取。轉換類 preset 額外把協商後的 format 併入 key，避免
 * 不同 Accept 能力的客戶端互相污染彼此的快取內容（Cache API 不對 Vary 做協商比對）。
 */
function buildCacheKeyUrl(url: URL, format: NegotiatedFormat | null): string {
  if (format === null) {
    return url.toString();
  }
  const keyUrl = new URL(url.toString());
  keyUrl.searchParams.set("__fmt", format);
  return keyUrl.toString();
}

/**
 * 統一出口：對每個回應（快取命中或新建、200 或 buildResponse 的 404）補 ACAO，並在
 * HEAD 時去掉 body。快取存的是 finalize 前的 clone（不含 ACAO），故 CORS 一律在讀取時
 * 補上，快取命中與 miss 行為一致。以新 Headers 重建，避免動到 Cache API 回傳的唯讀 headers。
 */
function finalizeResponse(response: Response, method: string): Response {
  const headers = new Headers(response.headers);
  headers.set("Access-Control-Allow-Origin", CORS_ALLOW_ORIGIN);
  const body = method === "HEAD" ? null : response.body;
  return new Response(body, { status: response.status, statusText: response.statusText, headers });
}

/** 錯誤回應（404/400）也帶 ACAO，前端 fetch 才讀得到狀態碼而非被 CORS 吞成 opaque。 */
function errorResponse(message: string, status: number): Response {
  const headers = new Headers();
  headers.set("Access-Control-Allow-Origin", CORS_ALLOW_ORIGIN);
  return new Response(message, { status, headers });
}

/** OPTIONS preflight：204 放行 GET/HEAD。 */
function preflightResponse(): Response {
  const headers = new Headers();
  headers.set("Access-Control-Allow-Origin", CORS_ALLOW_ORIGIN);
  headers.set("Access-Control-Allow-Methods", "GET, HEAD");
  // 反射瀏覽器請求的自訂 header（若有）；GET/HEAD 圖片請求通常無，缺省時不設。
  headers.set("Access-Control-Allow-Headers", "*");
  headers.set("Access-Control-Max-Age", PREFLIGHT_MAX_AGE);
  return new Response(null, { status: 204, headers });
}
