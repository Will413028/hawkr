/**
 * Seller AI image enhancement (圖片美化) over the generated Connect client
 * (`ImageAiAppService`, same-origin proxy `/api/connect/*`).
 *
 * `ListAiStyles` gives the preset styles (label / prompt / thumbnail) plus the
 * weekly quota; `TransformImage` sends the source bytes inline (A9-D37: at most
 * 2 MiB, so the client downsizes first — see `ai-image-downsize.ts`) and returns
 * the transformed bytes plus the post-transform quota counters.
 *
 * `AI_IMAGE_STYLES_BY_CATEGORY` below is a local fallback, keyed by category,
 * used until `fetchAiStyles` resolves or if it fails — the live prompt list comes
 * from the backend. Example thumbnails come from `AI_STYLE_THUMBNAIL` (bundled
 * placeholder) unless the backend supplies its own.
 */

import { Code, ConnectError } from "@connectrpc/connect";
import {
	classifyError,
	type DefaultErrorText,
	errorText,
	factsFromError,
	isNoResponse,
} from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	ImageAiAppService,
	AiStyleCategory as ProtoAiStyleCategory,
} from "@hawkr/api/gen/hawkr/imageai/v1alpha1/imageai_pb";
import { imageAiAppClient } from "../connect/imageai";
import { imageUri } from "../media/image";
import { fitImageToLimit } from "./ai-image-downsize";

export interface AiImageStyle {
	id: string;
	label: string;
	prompt: string;
	/**
	 * Example thumbnail for the style — the backend's sample when it has one,
	 * otherwise the bundled {@link AI_STYLE_THUMBNAIL} placeholder.
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

/** Enhanced image plus the post-transform weekly-quota counters. */
export interface AiEnhanceResult {
	blob: Blob;
	weeklyLimit?: number;
	weeklyUsage?: number;
}

/**
 * AI 美化失敗。`reason` 是伺服器的 ErrorReason（沒有認得的 reason 時為 null），`retryable` 表示
 * 「同一張圖原封不動再送一次」是否可能成功——呼叫端據此決定要引導使用者重試
 * 還是換照片／改描述，不要比對錯誤訊息字串。文字是伺服器的 LocalizedMessage，沒有時是目錄的預設文字。
 */
export class AiEnhanceError extends Error {
	readonly reason: ErrorReason | null;
	readonly retryable: boolean;
	constructor(
		message: string,
		reason: ErrorReason | null,
		retryable: boolean,
		options?: { cause?: unknown },
	) {
		super(message, options);
		this.name = "AiEnhanceError";
		this.reason = reason;
		this.retryable = retryable;
	}
}

/**
 * 本週 AI 美化額度已用完（reason IMAGE_QUOTA_EXCEEDED）。呼叫端以 `instanceof` 判斷，
 * 失敗時也同步額度顯示。同時進行的修圖超過上限也是 resource_exhausted，但沒有這個 reason，不是額度用完。
 */
export class AiQuotaExceededError extends AiEnhanceError {
	constructor(message: string, options?: { cause?: unknown }) {
		super(message, ErrorReason.IMAGE_QUOTA_EXCEEDED, false, options);
		this.name = "AiQuotaExceededError";
	}
}

/** 值得原封不動再送一次的 reason；其餘 reason 是對內容本身的確定性判定，重試不會有不同結果。 */
const RETRYABLE_REASONS: ReadonlySet<ErrorReason> = new Set([
	ErrorReason.IMAGE_NOT_PRODUCED,
]);

/** 失敗的 ConnectError 轉成 AiEnhanceError：reason 與可重試性由 classifyError 的事實決定，文字不在這裡寫。 */
export function toEnhanceError(
	caught: unknown,
	defaults: DefaultErrorText,
): AiEnhanceError {
	const err = ConnectError.from(caught);
	const message = errorText(err, defaults);
	const decision = classifyError(
		factsFromError(err, ImageAiAppService.method.transformImage),
	);
	const reason =
		decision.action === "reason" ? (decision.reason ?? null) : null;
	if (reason === ErrorReason.IMAGE_QUOTA_EXCEEDED) {
		return new AiQuotaExceededError(message, { cause: err });
	}
	const retryable =
		(reason !== null && RETRYABLE_REASONS.has(reason)) ||
		(reason === null && (err.code === Code.Unavailable || isNoResponse(err)));
	return new AiEnhanceError(message, reason, retryable, { cause: err });
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
 * `ListAiStyles` only carries a thumbnail for styles someone has uploaded a
 * sample for. This is the fallback whenever the backend has no thumbnail for
 * that id, so a backend-provided sample always wins.
 */
const AI_STYLE_THUMBNAIL = "/placeholders/ai-style.svg";

/**
 * Backend thumbnail if present, else the bundled placeholder.
 *
 * The backend value is the `thumb` preset URL of the style's `Image`. The
 * bundled placeholder is a local `/placeholders/…` asset; `buildImageUrl`
 * (in the dialog) resolves both forms, so a single slot can carry either.
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

const CATEGORY_TO_PROTO: Record<AiStyleCategory, ProtoAiStyleCategory> = {
	product: ProtoAiStyleCategory.PRODUCT,
	post: ProtoAiStyleCategory.POST,
};

/**
 * Fetch the live preset styles (label / prompt / example thumbnail) and the
 * caller's weekly-quota counters from `ListAiStyles`. Throws on failure so
 * callers can fall back to {@link AI_IMAGE_STYLES_BY_CATEGORY}.
 */
export async function fetchAiStyles(
	category: AiStyleCategory,
	signal?: AbortSignal,
): Promise<AiStyleList> {
	const res = await imageAiAppClient().listAiStyles(
		{ category: CATEGORY_TO_PROTO[category] },
		{ signal },
	);
	const styles: AiImageStyle[] = res.styles
		.map((s) => ({
			id: s.id || s.name,
			label: s.label || s.name,
			prompt: s.prompt,
			thumbnailUrl: resolveStyleThumbnail(imageUri(s.thumbnail, "thumb")),
		}))
		.filter((s) => s.id && s.label && s.prompt);
	return {
		defaultPrompt: res.defaultPrompt,
		styles,
		weeklyLimit: res.weeklyLimit,
		weeklyUsage: res.weeklyUsage,
	};
}

/**
 * A generative image round-trip runs far longer than a normal API call, so the
 * transport's per-flavor budget aborted mid-generation and surfaced as a bare
 * network failure. Give the transform its own ceiling.
 */
const ENHANCE_TIMEOUT_MS = 180_000;

/**
 * Transform an image through `TransformImage` and resolve to the enhanced image
 * bytes plus the updated weekly-quota counters. `prompt` is the full style prompt
 * (from `fetchAiStyles` / {@link AI_IMAGE_STYLES_BY_CATEGORY}) or a free-text
 * custom prompt. The source is downsized to the contract's 2 MiB limit first.
 * Failure text comes from the server's message or the catalog default for the
 * error code (`defaults`), never from a literal here.
 */
export async function enhanceImage(
	image: Blob,
	opts: {
		prompt?: string;
		signal?: AbortSignal;
		defaults: DefaultErrorText;
	},
): Promise<AiEnhanceResult> {
	const prompt = opts.prompt?.trim() ?? "";
	try {
		const source = await fitImageToLimit(image);
		const res = await imageAiAppClient().transformImage(
			{
				imageData: new Uint8Array(await source.arrayBuffer()),
				mimeType: source.type || "image/jpeg",
				prompt,
			},
			{ signal: opts.signal, timeoutMs: ENHANCE_TIMEOUT_MS },
		);
		return {
			blob: new Blob([new Uint8Array(res.imageData)], {
				type: res.mimeType || "image/jpeg",
			}),
			weeklyLimit: res.weeklyLimit,
			weeklyUsage: res.weeklyUsage,
		};
	} catch (e) {
		if (opts.signal?.aborted) throw e;
		throw toEnhanceError(e, opts.defaults);
	}
}
