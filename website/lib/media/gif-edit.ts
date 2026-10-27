import { isGifUploadFile } from "@/lib/media/gif";
import {
	GifPreflightError,
	preflightAnimatedGif,
} from "@/lib/media/gif-crop-preflight";
import { GIF_CROP_MAX_BYTES } from "@/lib/media/gif-crop-protocol";
import {
	GIF_MIN_PLAYBACK_DELAY_MS,
	getGifPlaybackDelayMs,
} from "@/lib/media/gif-timing";

export interface GifEditSettings {
	/** Omit to preserve source frames using their browser playback duration. */
	maxFps?: number;
	startTimeMs?: number;
	endTimeMs?: number;
}

export interface GifEditMetadata {
	width: number;
	height: number;
	frameCount: number;
	/** One loop at browser playback speed, including short-delay normalization. */
	durationMs: number;
}

export interface GifPlannedFrame {
	sourceFrameIndex: number;
	delayMs: number;
}

export interface GifEditPlan {
	frames: GifPlannedFrame[];
	/** Composite from source frame zero through here, including skipped frames. */
	decodeThroughIndex: number;
	durationMs: number;
}

/** Reads bounded structure and timing without loading a GIF decoder or encoder. */
export async function inspectGifFile(
	file: File,
	signal?: AbortSignal,
): Promise<GifEditMetadata> {
	if (signal?.aborted) throw new DOMException("GIF 處理已取消", "AbortError");
	if (!isGifUploadFile(file)) {
		throw new GifPreflightError("invalid-file", "選取的檔案不是 GIF 動畫。");
	}
	if (file.size > GIF_CROP_MAX_BYTES) {
		throw new GifPreflightError(
			"resource-limit",
			"GIF 超過 10 MB，請先縮小檔案後再試。",
		);
	}
	const buffer = await file.arrayBuffer();
	if (signal?.aborted) throw new DOMException("GIF 處理已取消", "AbortError");
	const { width, height, frameCount, frameDelaysMs } =
		preflightAnimatedGif(buffer);
	const durationMs = frameDelaysMs.reduce(
		(sum, delay) => sum + getGifPlaybackDelayMs(delay),
		0,
	);
	return { width, height, frameCount, durationMs };
}

/** Keep short slices visible when adjacent holds can share their duration. */
function makeHoldsPlayable(
	frames: readonly GifPlannedFrame[],
): GifPlannedFrame[] {
	const playable = frames.map((frame) => ({ ...frame }));
	let index = 0;
	while (index < playable.length) {
		const frame = playable[index];
		const missingDelayMs = GIF_MIN_PLAYBACK_DELAY_MS - frame.delayMs;
		if (missingDelayMs <= 0) {
			index += 1;
			continue;
		}
		const previous = playable[index - 1];
		const next = playable[index + 1];
		const donor =
			previous && previous.delayMs - missingDelayMs >= GIF_MIN_PLAYBACK_DELAY_MS
				? previous
				: next && next.delayMs - missingDelayMs >= GIF_MIN_PLAYBACK_DELAY_MS
					? next
					: null;
		if (donor) {
			// Moving one boundary by 10 ms preserves both authored poses and the
			// selected duration, without triggering the browser's 100 ms clamp.
			donor.delayMs -= missingDelayMs;
			frame.delayMs += missingDelayMs;
			index += 1;
		} else if (previous) {
			previous.delayMs += frame.delayMs;
			playable.splice(index, 1);
		} else if (next) {
			next.delayMs += frame.delayMs;
			playable.splice(index, 1);
		} else {
			// The selected-range check normally catches this before planning.
			throw new Error("GIF 至少需保留 0.02 秒，請延長選取範圍。");
		}
	}
	return playable;
}

/**
 * Plan output frames on the browser playback timeline, including its 0/10 ms
 * GIF delay normalization. Sampled and clipped holds never trigger the browser's
 * minimum-delay expansion, preserving the selected playback duration.
 * The compositor must still decode intervening source frames: GIF patches and
 * disposal can affect a later retained frame even when they are never encoded.
 */
export function planGifEdits(
	frameDelaysMs: readonly number[],
	settings: GifEditSettings = {},
): GifEditPlan {
	if (frameDelaysMs.length === 0) {
		throw new Error("GIF 動畫時間無效，請更換檔案。");
	}
	const playbackDelaysMs = frameDelaysMs.map(getGifPlaybackDelayMs);
	const { maxFps } = settings;
	if (
		maxFps !== undefined &&
		(!Number.isFinite(maxFps) || maxFps <= 0 || maxFps > 100)
	) {
		throw new Error("請選擇有效的 GIF 動畫流暢度。");
	}
	const totalDurationMs = playbackDelaysMs.reduce(
		(sum, delay) => sum + delay,
		0,
	);
	const requestedStart = settings.startTimeMs ?? 0;
	const requestedEnd = settings.endTimeMs ?? totalDurationMs;
	if (
		!Number.isFinite(requestedStart) ||
		!Number.isFinite(requestedEnd) ||
		requestedStart < 0 ||
		requestedEnd > totalDurationMs
	) {
		throw new Error("請填寫有效的 GIF 開始與結束時間。");
	}
	// GIF stores delays in centiseconds. Snap boundaries once, never per frame.
	const startTimeMs = Math.round(requestedStart / 10) * 10;
	const endTimeMs = Math.round(requestedEnd / 10) * 10;
	if (
		startTimeMs < 0 ||
		endTimeMs > totalDurationMs ||
		endTimeMs <= startTimeMs
	) {
		throw new Error("GIF 結束時間須晚於開始時間，且不能超過原動畫長度。");
	}
	if (endTimeMs - startTimeMs < GIF_MIN_PLAYBACK_DELAY_MS) {
		throw new Error("GIF 至少需保留 0.02 秒，請延長選取範圍。");
	}
	const frames: GifPlannedFrame[] = [];
	let sourceTimeMs = 0;
	let lastRetainedTimeMs = startTimeMs;
	let nextSampleTimeMs = startTimeMs;
	for (
		let sourceFrameIndex = 0;
		sourceFrameIndex < playbackDelaysMs.length;
		sourceFrameIndex += 1
	) {
		const delay = playbackDelaysMs[sourceFrameIndex];
		const clipStart = Math.max(sourceTimeMs, startTimeMs);
		const clipEnd = Math.min(sourceTimeMs + delay, endTimeMs);
		const clippedDelayMs = Math.max(0, clipEnd - clipStart);
		if (clippedDelayMs > 0) {
			if (maxFps === undefined) {
				frames.push({ sourceFrameIndex, delayMs: clippedDelayMs });
			} else if (nextSampleTimeMs < clipEnd) {
				// Sample within a source hold too: dropping a frame merely because its
				// start precedes a tick could otherwise hide a long still pose entirely.
				if (frames.length > 0) {
					frames[frames.length - 1].delayMs =
						nextSampleTimeMs - lastRetainedTimeMs;
				}
				frames.push({ sourceFrameIndex, delayMs: 0 });
				lastRetainedTimeMs = nextSampleTimeMs;
				// Jump directly past all sampling ticks inside this same source frame.
				// No repeated frames and no loop proportional to a long GIF delay.
				const elapsedCentiseconds = (clipEnd - startTimeMs) / 10;
				const nextTick =
					Math.floor(((elapsedCentiseconds - 1) * maxFps) / 100) + 1;
				nextSampleTimeMs =
					startTimeMs + Math.ceil((nextTick * 100) / maxFps) * 10;
			}
		}
		sourceTimeMs += delay;
	}
	if (frames.length === 0) {
		throw new Error("選取的時間範圍沒有 GIF 畫面，請重新調整。");
	}
	if (maxFps !== undefined) {
		frames[frames.length - 1].delayMs = endTimeMs - lastRetainedTimeMs;
	}
	const playableFrames = makeHoldsPlayable(frames);
	return {
		frames: playableFrames,
		decodeThroughIndex:
			playableFrames[playableFrames.length - 1].sourceFrameIndex,
		durationMs: endTimeMs - startTimeMs,
	};
}
