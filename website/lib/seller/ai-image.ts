/**
 * Seller AI image enhancement (圖片美化).
 *
 * Web port of the seller app's `ai_image_service.dart` flow. Sends the raw
 * image bytes (multipart) to the gateway's AI endpoint and gets a transformed
 * image back (base64). Unlike the app — which authenticates with a Bearer JWT —
 * the website rides the gateway's httpOnly cookie session via `apiClient.authed`,
 * which attaches the cookie and the active-role hint.
 *
 * Endpoints (both under `/app`, cookie-authed):
 *   GET  /app/ai_styles            → preset styles (label/prompt/thumbnailUrl) + weekly quota
 *   POST /app/image_transformations → multipart {image, prompt} → {imageBase64, mimeType, ...}
 *
 * `AI_IMAGE_STYLES_BY_CATEGORY` below is a local fallback, keyed by category
 * (product styles copied verbatim from the seller app's
 * `image_editor_screen.dart`), used until `fetchAiStyles` resolves or if it
 * fails — the live prompt list comes from the backend. Example thumbnails come
 * from `AI_STYLE_THUMBNAIL` (bundled placeholder) unless the backend supplies its own.
 */

import { ApiClientError, type ApiEnvelope, apiClient } from "../api/client";

export interface AiImageStyle {
	id: string;
	label: string;
	prompt: string;
	/**
	 * Example thumbnail for the style — the backend's `/app/ai_styles` sample
	 * when it has one, otherwise the bundled {@link AI_STYLE_THUMBNAIL} placeholder.
	 */
	thumbnailUrl?: string;
}

/** Result of `fetchAiStyles`: the live preset list plus weekly-quota counters. */
export interface AiStyleList {
	defaultPrompt: string;
	styles: AiImageStyle[];
	weeklyLimit?: number;
	weeklyUsage?: number;
}

/** Enhanced image plus the post-transform weekly-quota counters (if returned). */
export interface AiEnhanceResult {
	blob: Blob;
	weeklyLimit?: number;
	weeklyUsage?: number;
}

/**
 * AI 修圖業務碼。SSOT 是後端的 `docs/代碼管理（CodeMeta）/錯誤碼.md`
 * 「AI 修圖業務碼（9250~9259）」；這裡只鏡射需要分流的碼值，**文案一律用後端
 * 回傳的 message，不在前端複製一份**，否則兩邊會各改各的。
 */
export const AI_ENHANCE_CODE = {
	/** 模型該次沒產出圖片。後端已同步重試過一次，使用者再送一次仍可能成功。 */
	NO_IMAGE: 9250,
	/** 照片或描述未通過安全性審查——重送同一張圖不會有不同結果。 */
	SAFETY_BLOCKED: 9251,
	/** 產出的圖與受版權保護內容過於相似——同上。 */
	RECITATION: 9252,
	/** 上游 AI 服務暫時不可用，稍後可再試。 */
	PROVIDER_UNAVAILABLE: 9253,
} as const;

/**
 * 本週 AI 美化額度已用完。走 HTTP 429，但 HTTP 429 不是判準——限流器也回 429
 * （envelope `code` 為 429），只看 status 會把「請求太頻繁」誤判成「額度用完」。
 */
export const AI_QUOTA_EXCEEDED_CODE = 4291;

const AI_ENHANCE_CODES: ReadonlySet<number> = new Set(
	Object.values(AI_ENHANCE_CODE),
);

/** 值得原封不動再送一次的碼；其餘是對內容本身的確定性判定，重試不會有不同結果。 */
const RETRYABLE_CODES: ReadonlySet<number> = new Set([
	AI_ENHANCE_CODE.NO_IMAGE,
	AI_ENHANCE_CODE.PROVIDER_UNAVAILABLE,
]);

/**
 * AI 美化失敗。`code` 是後端業務碼（判不出來時為 null），`retryable` 表示
 * 「同一張圖原封不動再送一次」是否可能成功——呼叫端據此決定要引導使用者重試
 * 還是換照片／改描述，不要再去比對錯誤訊息字串。
 */
export class AiEnhanceError extends Error {
	readonly code: number | null;
	readonly retryable: boolean;
	constructor(message: string, code: number | null, retryable: boolean) {
		super(message);
		this.name = "AiEnhanceError";
		this.code = code;
		this.retryable = retryable;
	}
}

/**
 * Thrown when the weekly AI transform quota is already exhausted (envelope
 * `code` 4291). Mirrors the seller app's `AiQuotaExceededException` — callers
 * can branch on `instanceof` to sync UI quota state on failure, not just on
 * success.
 */
export class AiQuotaExceededError extends AiEnhanceError {
	constructor(message: string) {
		super(message, AI_QUOTA_EXCEEDED_CODE, false);
		this.name = "AiQuotaExceededError";
	}
}

/**
 * 判斷「同一張圖原封不動再送一次」是否可能成功。
 *
 * 已知業務碼直接查表；其餘退回 transport 層判斷——網路失敗（status 0）與 5xx
 * 屬暫時性，4xx 是請求本身的問題，重試不會有不同結果。
 */
function isEnhanceRetryable(code: number | null, status: number): boolean {
	if (code != null && AI_ENHANCE_CODES.has(code)) {
		return RETRYABLE_CODES.has(code);
	}
	return status === 0 || status >= 500;
}

const AI_IMAGE_STYLES_PRODUCT: AiImageStyle[] = [
	{
		id: "natural",
		label: "新鮮自然",
		prompt:
			"為上傳的農產品照片進行自然清新修圖。提升蔬果的飽和度、明亮度及鮮度呈現，使綠色更翠綠、紅色更亮而不失真。光線自然柔和、整體呈現清爽健康的農作物形象。背景保持乾淨並略微柔化。Enhance the uploaded produce photo with a fresh and natural look. Boost color saturation and brightness while keeping the image realistic. Make greens more vibrant and reds naturally bright. Apply soft natural lighting and keep the background clean and slightly blurred.",
	},
	{
		id: "warm",
		label: "陽光暖色",
		prompt:
			"將上傳的農產品照片修成陽光暖色調。增加金黃色與橙色色溫，使畫面呈現午後陽光般的溫暖氛圍。降低對比、柔化陰影，讓農產品看起來更柔和、溫潤且具農場自然感。Edit the uploaded produce photo with a warm sunlight tone. Add golden and orange hues to create a late-afternoon farm atmosphere. Use soft shadows, lower contrast, and warm lighting for a gentle, sunlit look.",
	},
	{
		id: "crisp",
		label: "清脆鮮豔",
		prompt:
			"將上傳的農產品照片修成清晰鮮豔的高對比風格。強化蔬果表面細節、增加亮度與鮮明色彩，使紅色、綠色與黃色更突出。整體呈現乾淨、清晰、電商感強烈的風格。Apply a crisp and vibrant style to the uploaded produce photo. Increase contrast, enhance surface details, and boost vivid colors. Make reds, greens, and yellows pop while keeping the image clean and sharp—ideal for e-commerce.",
	},
	{
		id: "giftbox",
		label: "高級禮盒",
		prompt:
			"將上傳的農產品照片修成高級禮盒風格。使用柔和光線、細緻反光、略帶暖調，提升高級感。增加立體度與質感，使農產品呈現精品禮盒般的精緻視覺效果。Transform the uploaded produce photo into a premium giftbox style. Use soft lighting, refined highlights, and mild warm tones. Enhance depth, smoothness, and elegance to make the produce look like a luxury gift set.",
	},
	{
		id: "documentary",
		label: "自然寫真",
		prompt:
			"將上傳的農產品照片修成寫實農場感。微降低飽和度、增加顆粒與細節，保留些許自然瑕疵，使畫面呈現樸實、真實、有鄉村農場氛圍。Edit the uploaded produce photo with a documentary farm look. Slightly lower saturation, add texture and realism, and keep some natural imperfections. The result should feel authentic, rural, and grounded.",
	},
	{
		id: "kitchen",
		label: "料理素材",
		prompt:
			"將上傳的農產品照片修成料理備料風格。加強切面細節、提升質地呈現，色溫略暖。光線柔和不刺眼，使農作物看起來像在料理檯上剛被處理，適合食譜與料理內容。Apply a kitchen prep style to the uploaded produce photo. Highlight cut-surface details and textures with a slightly warm tone. Use soft, natural lighting to create a fresh cooking-table look suitable for recipes and food content.",
	},
	{
		id: "clean",
		label: "白底極淨",
		prompt:
			"將上傳的農產品照片修成純白極淨電商風。背景轉換為 #FFFFFF 純白，提升產品亮度與對比，使蔬果輪廓清晰、色彩乾淨飽滿，呈現專業商品主圖效果。Edit the uploaded produce photo in a pure white e-commerce style. Convert the background to #FFFFFF white, enhance brightness and contrast, and make the produce edges clean and colors vivid. The result should look like a professional product main image.",
	},
];

const AI_IMAGE_STYLES_POST: AiImageStyle[] = [
	{
		id: "post_japanese_poster",
		label: "日系排版海報風",
		prompt:
			"以高調映像風格優化這張照片，增強自然光線與溫暖奶油黃色調，營造日系雜誌報導感，保持人物或商品主體清晰。Enhance this photo with a Japanese high-key editorial style — warm natural light, soft creamy-yellow tones, and a clean magazine-report mood, while keeping the main subject sharp.",
	},
	{
		id: "post_mixed_collage",
		label: "混合媒體拼貼風",
		prompt:
			"以混合媒體拼貼風格優化這張照片，提升色彩飽和度與對比，加入復古膠卷顆粒質感，營造活潑、有層次的社群視覺氛圍，保持主體清晰可辨。Enhance this photo with a mixed-media collage mood — boosted saturation and contrast, subtle vintage film grain, and a lively layered social-media feel, while keeping the subject clearly recognizable.",
	},
	{
		id: "post_grid_minimal",
		label: "九宮格網格風",
		prompt:
			"以簡約網格美學優化這張照片，提升整體明亮度與通透感，柔化色調並降低雜訊，營造乾淨、輕盈的社群九宮格質感，保持構圖穩定。Enhance this photo with a clean grid-aesthetic look — brighter and airier overall tone, softened colors, reduced noise, for a tidy minimal social-feed feel while keeping the composition stable.",
	},
	{
		id: "post_event_pixel",
		label: "活動海報風",
		prompt:
			"以活動海報風格優化這張照片，強化黑白對比與銳利度，加入鮮明色塊氛圍與略帶復古螢幕質感的色調，營造吸睛、有活動宣傳感的視覺效果，保持主體清晰。Enhance this photo with a bold event-poster style — stronger black-and-white contrast and sharpness, punchy color-block mood with a subtle retro-screen tint, for an eye-catching promotional feel while keeping the subject clear.",
	},
];

export type AiStyleCategory = "product" | "post";

/**
 * Bundled example thumbnail for every preset: one shared placeholder
 * (`public/placeholders/ai-style.svg`, generated by `pnpm gen:placeholders`).
 *
 * The style picker shows a sample of what each preset does, and
 * `/app/ai_styles` only carries `thumbnailUrl` for styles someone has uploaded
 * a sample for. This is the fallback whenever the backend has no thumbnail for
 * that id, so a backend-provided sample always wins.
 */
const AI_STYLE_THUMBNAIL = "/placeholders/ai-style.svg";

/**
 * Backend thumbnail if present, else the bundled placeholder.
 *
 * §231: the backend value is `thumbnailKey` (bare objectKey, SSOT) — its
 * Gateway-composed `thumbnailUrl` twin is not read. The bundled placeholder is
 * a local `/placeholders/…` asset; `buildImageUrl` resolves both forms, so a
 * single slot can carry either.
 */
function resolveStyleThumbnail(backend?: string) {
	return backend || AI_STYLE_THUMBNAIL;
}

function withThumbnails(styles: AiImageStyle[]): AiImageStyle[] {
	return styles.map((s) => ({
		...s,
		thumbnailUrl: resolveStyleThumbnail(s.thumbnailUrl),
	}));
}

/** Local fallback styles, keyed by category — used until `fetchAiStyles` resolves or fails. */
export const AI_IMAGE_STYLES_BY_CATEGORY: Record<
	AiStyleCategory,
	AiImageStyle[]
> = {
	product: withThumbnails(AI_IMAGE_STYLES_PRODUCT),
	post: withThumbnails(AI_IMAGE_STYLES_POST),
};

function base64ToBlob(b64: string, mimeType = "image/jpeg"): Blob {
	// Tolerate a `data:` prefix as well as bare base64.
	const comma = b64.indexOf(",");
	const pure =
		b64.startsWith("data:") && comma >= 0 ? b64.slice(comma + 1) : b64;
	const bin = atob(pure);
	const bytes = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
	return new Blob([bytes], { type: mimeType || "image/jpeg" });
}

/** Parse the backend's stringified quota counters into a number, if present. */
function toNum(value: unknown): number | undefined {
	if (value == null) return undefined;
	const n = Number(value);
	return Number.isFinite(n) ? n : undefined;
}

/**
 * Fetch the live preset styles (label / prompt / example thumbnail) and the
 * caller's weekly-quota counters from `GET /app/ai_styles`. Throws on a non-OK
 * response so callers can fall back to {@link AI_IMAGE_STYLES_BY_CATEGORY}.
 */
export async function fetchAiStyles(
	category: AiStyleCategory,
	signal?: AbortSignal,
): Promise<AiStyleList> {
	// Authed GET via the unified transport (cookie + active-role auto-applied).
	// A non-OK throws (ApiClientError) so callers fall back to AI_IMAGE_STYLES_BY_CATEGORY;
	// suppressAuthExpired keeps a 401 here from triggering a global logout.
	const { data: payload } = await apiClient.authed.get<Record<string, unknown>>(
		"/ai_styles",
		{
			query: { category },
			signal,
			throwOnEnvelopeError: false,
			suppressAuthExpired: true,
		},
	);

	const data = ((payload as Record<string, unknown>) ?? {}) as Record<
		string,
		unknown
	>;
	const rawStyles = Array.isArray(data.styles)
		? (data.styles as Array<Record<string, unknown>>)
		: [];
	const styles: AiImageStyle[] = rawStyles
		.map((s) => {
			const id = String(s.id ?? s.name ?? "");
			return {
				id,
				label: String(s.label ?? s.name ?? ""),
				prompt: String(s.prompt ?? ""),
				thumbnailUrl: resolveStyleThumbnail(s.thumbnailKey as string),
			};
		})
		.filter((s) => s.id && s.label && s.prompt);

	return {
		defaultPrompt: String(data.defaultPrompt ?? ""),
		styles,
		weeklyLimit: toNum(data.weeklyLimit),
		weeklyUsage: toNum(data.weeklyUsage),
	};
}

/**
 * A generative image round-trip runs far longer than a normal API call, so the
 * transport's per-flavor budget (prod 15s / dev 30s) aborted mid-generation and
 * surfaced as a bare network failure. Give the transform its own ceiling.
 */
const ENHANCE_TIMEOUT_MS = 180_000;

/**
 * Turn a transport failure into text a seller can act on.
 *
 * The transport reports an aborted / offline fetch as status 0, so the old
 * `HTTP ${status}` fallback literally rendered "AI 美化失敗 (HTTP 0)" in the
 * dialog. No branch here may leak an HTTP status or any other transport jargon.
 * A message the backend sent is already seller-facing, so it still wins —— 唯一
 * 例外是限流，見下方第一條分支。
 */
function describeEnhanceFailure(
	status: number,
	code: number | null,
	backendMessage?: string,
	cause?: unknown,
): string {
	// 限流的 429 由 Gateway 自己產生，訊息是英文的 "rate limit exceeded"，不是寫
	// 給賣家看的，所以這一條要擋在 backendMessage 之前。額度用完（4291）不在此
	// 列：那句由 ai-service 產生且帶真實用量（「（3/10），下週一重置」）。
	if (status === 429 && code !== AI_QUOTA_EXCEEDED_CODE)
		return "操作太頻繁，請稍候幾秒再試";
	if (backendMessage) return backendMessage;
	if (code === AI_QUOTA_EXCEEDED_CODE)
		return "本週 AI 美化次數已用完，下週一重置";
	if (status === 401 || status === 403)
		return "登入狀態已過期，請重新登入後再試";
	if (status === 413) return "圖片檔案太大，請換一張較小的圖片";
	if (status >= 500) return "AI 美化暫時無法使用，請稍後再試";
	if (status > 0) return "這次沒有成功產生圖片，請再試一次";
	// status 0 — fetch itself never completed.
	if (cause instanceof DOMException && cause.name === "TimeoutError")
		return "AI 美化時間過長，已自動停止，請稍後再試一次";
	return "連線失敗，請確認網路後再試一次";
}

/**
 * Transform an image through `POST /app/image_transformations` and resolve to
 * the enhanced image bytes plus the updated weekly-quota counters. `prompt` is
 * the full style prompt (from `fetchAiStyles` / {@link AI_IMAGE_STYLES_BY_CATEGORY}) or a
 * free-text custom prompt — the backend requires it, so we always send it.
 */
export async function enhanceImage(
	image: Blob,
	opts: { prompt?: string; signal?: AbortSignal } = {},
): Promise<AiEnhanceResult> {
	const form = new FormData();
	form.append("image", image, `image_${Date.now()}.jpg`);
	const prompt = opts.prompt?.trim();
	if (prompt) form.append("prompt", prompt);

	// Authed multipart POST. apiClient adds the active-role hint + cookie; the
	// FormData body keeps fetch's auto multipart boundary (no Content-Type).
	let envelope: ApiEnvelope<Record<string, unknown>>;
	try {
		envelope = await apiClient.authed.post<Record<string, unknown>>(
			"/image_transformations",
			{
				body: form,
				signal: opts.signal,
				timeoutMs: ENHANCE_TIMEOUT_MS,
				throwOnEnvelopeError: false,
				suppressAuthExpired: true,
			},
		);
	} catch (e) {
		// A caller-initiated abort (dialog re-run / close / unmount) is not a
		// failure — rethrow it untouched so callers can keep filtering on
		// `name === "AbortError"` instead of showing a phantom error.
		if (e instanceof DOMException && e.name === "AbortError") throw e;
		const status = e instanceof ApiClientError ? e.status : 0;
		const payload =
			e instanceof ApiClientError
				? (e.payload as
						| {
								code?: number;
								message?: string;
								messageZh?: string;
								msg?: string;
						  }
						| undefined)
				: undefined;
		// 失敗原因走業務碼（額度用完 HTTP 429 + 4291、其餘 HTTP 409 + 9250~9253）。
		// 呼叫端讀 code / retryable 決定要引導重試還是請使用者換照片，不要再去比對
		// 錯誤訊息字串，也不要用 HTTP status——429 同時屬於限流器。
		const code =
			payload?.code ?? (e instanceof ApiClientError ? (e.code ?? null) : null);
		const message = describeEnhanceFailure(
			status,
			code ?? null,
			payload?.messageZh || payload?.message || payload?.msg,
			e,
		);
		if (code === AI_QUOTA_EXCEEDED_CODE)
			throw new AiQuotaExceededError(message);
		throw new AiEnhanceError(
			message,
			code ?? null,
			isEnhanceRetryable(code ?? null, status),
		);
	}
	if (envelope.code != null && envelope.code !== 0 && envelope.code !== 200) {
		// throwOnEnvelopeError:false 之下，HTTP 2xx 但 envelope 非 0 才會走到這裡。
		throw new AiEnhanceError(
			envelope.message || envelope.msg || "這次沒有成功產生圖片，請再試一次",
			envelope.code,
			isEnhanceRetryable(envelope.code, 200),
		);
	}

	const data = ((envelope.data as Record<string, unknown>) ?? {}) as Record<
		string,
		unknown
	>;
	const weeklyLimit = toNum(data.weeklyLimit);
	const weeklyUsage = toNum(data.weeklyUsage);
	const b64 = (data.imageBase64 ?? data.image_base64) as string | undefined;
	if (b64) {
		const mimeType = (data.mimeType ?? data.mime_type) as string | undefined;
		return { blob: base64ToBlob(b64, mimeType), weeklyLimit, weeklyUsage };
	}
	const imgUrl = data.imageUrl as string | undefined;
	if (imgUrl) {
		const imgRes = await fetch(imgUrl, { signal: opts.signal });
		if (!imgRes.ok) throw new Error("無法下載 AI 美化結果，請重試一次");
		return { blob: await imgRes.blob(), weeklyLimit, weeklyUsage };
	}
	throw new Error("AI 沒有產生圖片，請換個風格或稍後再試");
}
