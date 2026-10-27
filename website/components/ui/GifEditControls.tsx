"use client";

import * as React from "react";
import { Input, Select } from "@/components/ui/Input";
import { GifPreflightError } from "@/lib/media/gif-crop-preflight";
import { inspectGifFile } from "@/lib/media/gif-edit";

type GifMetadata = Awaited<ReturnType<typeof inspectGifFile>>;
type GifOutputSize = "auto" | 360 | 540 | 720;
type GifFrameRate = 8 | 12 | "original";

export interface GifEditorOptions {
	maxOutput: number;
	autoFit?: boolean;
	maxFps?: number;
	startTimeMs?: number;
	endTimeMs?: number;
}

interface GifEditorInputs {
	file: File | undefined;
	maxOutput: GifOutputSize;
	maxFps: GifFrameRate;
	startSeconds: string;
	endSeconds: string;
}

interface GifInspection {
	file: File;
	loading: boolean;
	metadata: GifMetadata | null;
	error: string | null;
}

function defaultInputs(
	file: File | undefined,
	metadata: GifMetadata | null,
): GifEditorInputs {
	return {
		file,
		maxOutput: "auto",
		maxFps: 12,
		startSeconds: "0",
		endSeconds: String((metadata?.durationMs ?? 0) / 1000),
	};
}

/** Keeps source inspection and editable settings scoped to the actual File. */
export function useGifEditor(file: File | undefined, enabled: boolean) {
	const [inspection, setInspection] = React.useState<GifInspection | null>(
		null,
	);
	const [inputs, setInputs] = React.useState<GifEditorInputs | null>(null);
	const activeInspection =
		enabled && inspection?.file === file ? inspection : null;
	const metadata = activeInspection?.metadata ?? null;
	const currentInputs =
		enabled && inputs && inputs.file === file
			? inputs
			: defaultInputs(file, metadata);

	React.useEffect(() => {
		setInputs(null);
		if (!enabled || !file) {
			setInspection(null);
			return;
		}
		const controller = new AbortController();
		setInspection({ file, loading: true, metadata: null, error: null });
		void inspectGifFile(file, controller.signal).then(
			(nextMetadata) => {
				if (controller.signal.aborted) return;
				setInspection({
					file,
					loading: false,
					metadata: nextMetadata,
					error: null,
				});
			},
			(error: unknown) => {
				if (controller.signal.aborted) return;
				setInspection({
					file,
					loading: false,
					metadata: null,
					error:
						error instanceof GifPreflightError
							? error.message
							: "無法讀取這個 GIF，請更換圖片後再試。",
				});
			},
		);
		return () => controller.abort();
	}, [file, enabled]);

	const updateInputs = React.useCallback(
		(update: Partial<Omit<GifEditorInputs, "file">>) => {
			if (!metadata) return;
			setInputs((previous) => ({
				...(previous && previous.file === file
					? previous
					: defaultInputs(file, metadata)),
				...update,
			}));
		},
		[file, metadata],
	);
	const reset = React.useCallback(() => {
		setInputs(metadata ? defaultInputs(file, metadata) : null);
	}, [file, metadata]);

	const { maxOutput, maxFps, startSeconds, endSeconds } = currentInputs;
	const start = Number(startSeconds);
	const end = Number(endSeconds);
	// GIF timing has 10 ms precision; reject ranges that would snap to zero.
	const startTimeMs = Math.round(start * 100) * 10;
	const endTimeMs = Math.round(end * 100) * 10;
	let rangeError: string | null = null;
	if (metadata && metadata.durationMs > 0) {
		if (
			startSeconds.trim() === "" ||
			endSeconds.trim() === "" ||
			!Number.isFinite(start) ||
			!Number.isFinite(end)
		) {
			rangeError = "請填寫開始與結束秒數。";
		} else if (
			start < 0 ||
			end <= start ||
			endTimeMs <= startTimeMs ||
			end > metadata.durationMs / 1000
		) {
			rangeError = "結束秒數須大於開始秒數，且不能超過原動畫長度。";
		} else if (endTimeMs - startTimeMs < 20) {
			rangeError = "請至少保留 0.02 秒，讓動畫能正常播放。";
		}
	}

	const options: GifEditorOptions = {
		maxOutput: maxOutput === "auto" ? 540 : maxOutput,
		...(maxOutput === "auto" ? { autoFit: true } : {}),
		...(maxFps !== "original" ? { maxFps } : {}),
		...(metadata &&
		metadata.durationMs > 0 &&
		!rangeError &&
		(startTimeMs !== 0 || endTimeMs !== metadata.durationMs)
			? { startTimeMs, endTimeMs }
			: {}),
	};
	const error = activeInspection?.error ?? rangeError;

	return {
		ready: Boolean(enabled && file && metadata && !error),
		loading: Boolean(
			enabled && file && (!activeInspection || activeInspection.loading),
		),
		error,
		rangeError,
		metadata,
		sourceSizeBytes: file?.size ?? 0,
		options,
		key: JSON.stringify({ options, startSeconds, endSeconds }),
		reset,
		maxOutput,
		maxFps,
		startSeconds,
		endSeconds,
		setMaxOutput: (value: GifOutputSize) => updateInputs({ maxOutput: value }),
		setMaxFps: (value: GifFrameRate) => updateInputs({ maxFps: value }),
		setStartSeconds: (value: string) => updateInputs({ startSeconds: value }),
		setEndSeconds: (value: string) => updateInputs({ endSeconds: value }),
	};
}

interface GifEditControlsProps {
	editor: ReturnType<typeof useGifEditor>;
	disabled?: boolean;
	onChange?: () => void;
}

/** Animation settings share the crop dialog's existing sidebar and fields. */
export default function GifEditControls({
	editor,
	disabled = false,
	onChange,
}: GifEditControlsProps) {
	const sectionId = React.useId();
	const errorId = `${sectionId}-error`;
	const controlsDisabled = disabled || editor.loading || !editor.metadata;
	const durationSeconds = (editor.metadata?.durationMs ?? 0) / 1000;
	const trimDisabled = controlsDisabled || durationSeconds === 0;
	// Two different failures used to share one paragraph and one describedby:
	// a file-inspection failure was announced by the paragraph but described by
	// nothing, while the range error had no message beside the fields at all.
	// Split them — the inspection failure stays a section-level alert, the range
	// error becomes the 結束 field's own message (and the 開始 field points at
	// it, since the constraint spans the pair).
	const startFieldId = `${sectionId}-start`;
	const endFieldId = `${sectionId}-end`;
	const inspectionError =
		editor.error && editor.error !== editor.rangeError ? editor.error : null;
	const startDescribedBy =
		[
			inspectionError ? errorId : null,
			editor.rangeError ? `${endFieldId}-desc` : null,
		]
			.filter(Boolean)
			.join(" ") || undefined;

	return (
		<section aria-labelledby={sectionId} className="grid gap-3">
			<div className="grid gap-1">
				<h3
					id={sectionId}
					className="m-0 text-sm font-semibold text-foreground"
				>
					編輯動畫
				</h3>
				{editor.loading ? (
					<p role="status" className="m-0 text-xs text-foreground-soft">
						正在讀取動畫資訊…
					</p>
				) : editor.metadata ? (
					<p className="m-0 text-xs text-foreground-soft tabular-nums">
						原始 {editor.metadata.width} × {editor.metadata.height} px ·{" "}
						{editor.metadata.frameCount} 幀 · {durationSeconds.toFixed(2)} 秒 ·{" "}
						{(editor.sourceSizeBytes / (1024 * 1024)).toFixed(2)} MiB
					</p>
				) : null}
			</div>

			<div className="grid grid-cols-2 gap-3">
				<Select
					label="輸出尺寸"
					value={editor.maxOutput}
					disabled={controlsDisabled}
					onChange={(event) => {
						editor.setMaxOutput(
							event.target.value === "auto"
								? "auto"
								: (Number(event.target.value) as GifOutputSize),
						);
						onChange?.();
					}}
				>
					<option value="auto">自動調整</option>
					<option value={720}>720 px</option>
					<option value={540}>540 px</option>
					<option value={360}>360 px</option>
				</Select>
				<Select
					label="流暢度"
					value={editor.maxFps}
					disabled={controlsDisabled}
					onChange={(event) => {
						editor.setMaxFps(
							event.target.value === "original"
								? "original"
								: (Number(event.target.value) as 8 | 12),
						);
						onChange?.();
					}}
				>
					<option value="original">保留原始</option>
					<option value={12}>12 fps</option>
					<option value={8}>8 fps</option>
				</Select>
			</div>

			<div className="grid grid-cols-2 gap-3">
				<Input
					id={startFieldId}
					label="開始（秒）"
					aria-describedby={startDescribedBy}
					type="number"
					inputMode="decimal"
					min={0}
					max={durationSeconds}
					step={0.01}
					value={editor.startSeconds}
					isInvalid={Boolean(editor.rangeError)}
					disabled={trimDisabled}
					onChange={(event) => {
						editor.setStartSeconds(event.target.value);
						onChange?.();
					}}
				/>
				<Input
					id={endFieldId}
					label="結束（秒）"
					aria-describedby={inspectionError ? errorId : undefined}
					errorMessage={editor.rangeError ?? undefined}
					type="number"
					inputMode="decimal"
					min={0}
					max={durationSeconds}
					step={0.01}
					value={editor.endSeconds}
					isInvalid={Boolean(editor.rangeError)}
					disabled={trimDisabled}
					onChange={(event) => {
						editor.setEndSeconds(event.target.value);
						onChange?.();
					}}
				/>
			</div>

			{inspectionError && (
				<p id={errorId} role="alert" className="m-0 text-xs text-danger">
					{inspectionError}
				</p>
			)}
			<p className="m-0 text-xs leading-relaxed text-foreground-soft">
				自動調整會在檔案過大時縮小尺寸，保留取景、選取片段與播放長度。仍無法處理時，可降低流暢度或縮短動畫。預覽會顯示實際尺寸與檔案大小，須在
				10 MiB 內才能上傳。
			</p>
		</section>
	);
}
