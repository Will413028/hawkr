import { describe, expect, it, vi } from "vitest";
import { ApiClientError, apiClient } from "@/lib/api/client";
import {
	AI_ENHANCE_CODE,
	AiEnhanceError,
	AiQuotaExceededError,
	enhanceImage,
} from "@/lib/seller/ai-image";

// enhanceImage() must classify a quota-exhausted response as
// AiQuotaExceededError specifically, not a generic Error — the dialog uses
// `instanceof` to sync its quota display on failure (2026-07-13: a failed
// transform never updated the "本週已用完" button state, only successes did,
// so the button stayed clickable even after the backend confirmed the quota
// was gone).
//
// 判準是 envelope `code` 4291，**不是** HTTP 429：限流器也回 429（code 429），
// 用 status 判會把「請求太頻繁」誤鎖成「本週已用完」（#2272）。

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: { authed: { post: vi.fn() } },
	};
});

describe("enhanceImage quota error classification", () => {
	it("429 + code 4291 → AiQuotaExceededError carrying the backend message", async () => {
		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError("quota exceeded", 429, 4291, {
				code: 4291,
				message: "本週 AI 修圖額度已用完（10/10），下週一重置",
			}),
		);

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect(err).toBeInstanceOf(AiQuotaExceededError);
		expect((err as AiEnhanceError).code).toBe(4291);
		expect((err as Error).message).toBe(
			"本週 AI 修圖額度已用完（10/10），下週一重置",
		);
	});

	// 限流器的 429：envelope code 是 429，不得被當成額度用完，也不得把 Gateway 的
	// 英文 "rate limit exceeded" 直接顯示給賣家。
	it("429 + code 429 (rate limited) → NOT AiQuotaExceededError", async () => {
		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError("rate limit exceeded", 429, 429, {
				code: 429,
				message: "rate limit exceeded",
			}),
		);

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect(err).toBeInstanceOf(AiEnhanceError);
		expect(err).not.toBeInstanceOf(AiQuotaExceededError);
		expect((err as Error).message).toBe("操作太頻繁，請稍候幾秒再試");
	});

	it("other failures stay a plain Error, not AiQuotaExceededError", async () => {
		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError("boom", 503, undefined, undefined),
		);

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect(err).toBeInstanceOf(Error);
		expect(err).not.toBeInstanceOf(AiQuotaExceededError);
	});
});

// 後端 2026-07 之前把所有 Gemini 側失敗都回 HTTP 500 + 一句寫死的「未通過安全性
// 檢查」，賣家因此被導去換照片，實際上多半只是模型該次沒產圖、重試就好。現在改回
// HTTP 409 + 9250~9253，前端必須讀 code 分流而不是比對訊息字串。
describe("enhanceImage business-code classification", () => {
	const reject = (code: number, message: string, status = 409) =>
		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError(message, status, code, {
				code,
				message,
				messageZh: message,
			}),
		);

	it("9250 沒產圖 → 可重試，訊息用後端的", async () => {
		reject(AI_ENHANCE_CODE.NO_IMAGE, "AI 這次沒有產生圖片，請再試一次");

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect(err).toBeInstanceOf(AiEnhanceError);
		expect((err as AiEnhanceError).code).toBe(AI_ENHANCE_CODE.NO_IMAGE);
		expect((err as AiEnhanceError).retryable).toBe(true);
		expect((err as Error).message).toBe("AI 這次沒有產生圖片，請再試一次");
	});

	it("9251 安全性阻擋 → 不可重試", async () => {
		reject(
			AI_ENHANCE_CODE.SAFETY_BLOCKED,
			"照片或描述未通過安全性審查，請更換照片或修改描述",
		);

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect((err as AiEnhanceError).code).toBe(AI_ENHANCE_CODE.SAFETY_BLOCKED);
		expect((err as AiEnhanceError).retryable).toBe(false);
	});

	it("9252 版權相似 → 不可重試", async () => {
		reject(AI_ENHANCE_CODE.RECITATION, "產生的圖片與受版權保護的內容過於相似");

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect((err as AiEnhanceError).retryable).toBe(false);
	});

	it("9253 上游不可用 → 可重試", async () => {
		reject(AI_ENHANCE_CODE.PROVIDER_UNAVAILABLE, "AI 服務暫時無法使用");

		const err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect((err as AiEnhanceError).retryable).toBe(true);
	});

	it("未知碼退回 transport 判斷：5xx 可重試、4xx 不可", async () => {
		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError("boom", 503, undefined, undefined),
		);
		let err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect((err as AiEnhanceError).retryable).toBe(true);

		vi.mocked(apiClient.authed.post).mockRejectedValueOnce(
			new ApiClientError("bad request", 400, undefined, undefined),
		);
		err = await enhanceImage(new Blob(["x"])).catch((e) => e);
		expect((err as AiEnhanceError).retryable).toBe(false);
	});
});
