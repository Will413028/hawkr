/**
 * Browser GIF decoders display raw 0/10 ms holds for 100 ms.
 * https://github.com/WebKit/WebKit/blob/main/Source/WebCore/platform/graphics/cg/ImageDecoderCG.cpp
 */
export const GIF_BROWSER_SHORT_DELAY_MS = 100;
/** Shorter encoded holds would be expanded by the browser during playback. */
export const GIF_MIN_PLAYBACK_DELAY_MS = 20;

export function getGifPlaybackDelayMs(rawDelayMs: number): number {
	if (
		!Number.isInteger(rawDelayMs) ||
		rawDelayMs < 0 ||
		rawDelayMs % 10 !== 0
	) {
		throw new Error("GIF 動畫時間無效，請更換檔案。");
	}
	return rawDelayMs < GIF_MIN_PLAYBACK_DELAY_MS
		? GIF_BROWSER_SHORT_DELAY_MS
		: rawDelayMs;
}
