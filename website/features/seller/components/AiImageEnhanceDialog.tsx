"use client";

import { errorText } from "@hawkr/api/connect/errors";
import * as React from "react";
import { createPortal } from "react-dom";
import AiSparkleIcon from "@/components/shared/AiSparkleIcon";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { buildImageUrl } from "@/lib/media/cdn";
import {
	AI_IMAGE_STYLES_BY_CATEGORY,
	AiEnhanceError,
	type AiImageStyle,
	AiQuotaExceededError,
	type AiStyleCategory,
	enhanceImage,
	fetchAiStyles,
} from "@/lib/seller/ai-image";

interface Props {
	open: boolean;
	/** Which preset prompt list to use — fixed by the calling screen's context. */
	category: AiStyleCategory;
	/** The original image bytes to enhance. */
	source: Blob | null;
	/** Called with the enhanced bytes when the user accepts the result. */
	onApply: (enhanced: Blob) => void;
	onClose: () => void;
}

/**
 * AI image enhancement dialog — mirrors the seller app's flow: pick a preset
 * style (with an example thumbnail) or type a custom prompt, run it, compare
 * before/after, then apply. Preset styles + the weekly quota come live from
 * `ListAiStyles` (falling back to the bundled list); the transform is
 * `TransformImage`. Operates on raw bytes so callers can hand
 * it a freshly-picked File (no CDN round-trip / CORS needed).
 */
export default function AiImageEnhanceDialog({
	open,
	category,
	source,
	onApply,
	onClose,
}: Props) {
	const defaultErrors = useDefaultErrorText();
	const fallbackStyles = AI_IMAGE_STYLES_BY_CATEGORY[category];
	const [styles, setStyles] = React.useState<AiImageStyle[]>(fallbackStyles);
	const [styleId, setStyleId] = React.useState<string>(fallbackStyles[0].id);
	const [customPrompt, setCustomPrompt] = React.useState("");
	const [useCustom, setUseCustom] = React.useState(false);
	const [loading, setLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [quota, setQuota] = React.useState<{
		limit?: number;
		usage?: number;
	} | null>(null);
	const [originalUrl, setOriginalUrl] = React.useState<string | null>(null);
	const [resultBlob, setResultBlob] = React.useState<Blob | null>(null);
	const [resultUrl, setResultUrl] = React.useState<string | null>(null);
	const [brokenThumbs, setBrokenThumbs] = React.useState<
		Record<string, boolean>
	>({});
	const [mounted, setMounted] = React.useState(false);
	const abortRef = React.useRef<AbortController | null>(null);

	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!open || !source) return;
		const url = URL.createObjectURL(source);
		setOriginalUrl(url);
		setResultBlob(null);
		setResultUrl(null);
		setError(null);
		return () => URL.revokeObjectURL(url);
	}, [open, source]);

	React.useEffect(() => {
		if (!open) return;
		setStyles(AI_IMAGE_STYLES_BY_CATEGORY[category]);
		const ctrl = new AbortController();
		fetchAiStyles(category, ctrl.signal)
			.then((list) => {
				if (ctrl.signal.aborted) return;
				if (list.styles.length) setStyles(list.styles);
				if (list.weeklyLimit != null || list.weeklyUsage != null) {
					setQuota({ limit: list.weeklyLimit, usage: list.weeklyUsage });
				}
			})
			.catch(() => {});
		return () => ctrl.abort();
	}, [open, category]);

	React.useEffect(() => {
		if (!styles.some((s) => s.id === styleId)) {
			setStyleId(styles[0]?.id ?? "");
		}
	}, [styles, styleId]);

	React.useEffect(() => {
		return () => {
			if (resultUrl) URL.revokeObjectURL(resultUrl);
		};
	}, [resultUrl]);

	React.useEffect(() => {
		return () => abortRef.current?.abort();
	}, []);

	React.useEffect(() => {
		if (!open) return;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") onClose();
		};
		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [open, onClose]);

	if (!open || !mounted) return null;

	const run = async () => {
		if (!source) return;
		const prompt = useCustom
			? customPrompt
			: styles.find((s) => s.id === styleId)?.prompt;
		if (useCustom && !customPrompt.trim()) {
			setError("請先輸入提示詞");
			return;
		}
		setLoading(true);
		setError(null);
		abortRef.current?.abort();
		const ctrl = new AbortController();
		abortRef.current = ctrl;
		try {
			const result = await enhanceImage(source, {
				prompt,
				signal: ctrl.signal,
				defaults: defaultErrors,
			});
			if (resultUrl) URL.revokeObjectURL(resultUrl);
			setResultBlob(result.blob);
			setResultUrl(URL.createObjectURL(result.blob));
			if (result.weeklyLimit != null || result.weeklyUsage != null) {
				setQuota({ limit: result.weeklyLimit, usage: result.weeklyUsage });
			}
		} catch (e) {
			if ((e as Error)?.name === "AbortError") return;
			setError(
				e instanceof AiEnhanceError ? e.message : errorText(e, defaultErrors),
			);
			if (e instanceof AiQuotaExceededError) {
				setQuota((prev) =>
					prev?.limit != null ? { limit: prev.limit, usage: prev.limit } : prev,
				);
			}
		} finally {
			if (abortRef.current === ctrl) setLoading(false);
		}
	};

	const apply = () => {
		if (resultBlob) onApply(resultBlob);
		onClose();
	};

	const quotaText =
		quota?.limit != null
			? `本週 AI 美化 ${quota.usage ?? 0}/${quota.limit}`
			: null;
	const quotaReached =
		quota?.limit != null && (quota.usage ?? 0) >= quota.limit;

	return createPortal(
		<div
			className="fixed inset-0 z-[1200] flex items-start justify-center bg-[rgb(var(--color-neutral-ink-rgb)_/_0.46)] backdrop-blur-sm p-4 pt-[calc(var(--floating-nav-top-gap,16px)+var(--floating-nav-height,84px)+16px)] overflow-y-auto"
			role="dialog"
			aria-modal="true"
			aria-label="AI 圖片美化"
		>
			<div className="w-full max-w-2xl max-h-[calc(100dvh-var(--floating-nav-top-gap,16px)-var(--floating-nav-height,84px)-32px)] overflow-auto rounded-[var(--hkui-radius-2xl)] border border-white/55 bg-[linear-gradient(135deg,rgba(255,253,247,0.82),rgb(var(--color-ink-50-rgb)/0.66))] p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.72),0_24px_60px_-28px_rgb(var(--color-ink-950-rgb)/0.55)] backdrop-blur-[24px] backdrop-saturate-[130%] dark:border-[rgba(245,222,165,0.16)] dark:bg-[linear-gradient(135deg,rgba(255,255,255,0.07),rgba(255,255,255,0.03))] dark:shadow-[inset_0_1px_0_rgba(245,222,165,0.06),0_24px_60px_-28px_rgba(0,0,0,0.6)]">
				<header className="flex items-center justify-between mb-4 gap-3">
					<h2 className="font-display text-xl text-foreground m-0 inline-flex items-center gap-2">
						<AiSparkleIcon size={22} />
						AI 圖片美化
					</h2>
					<div className="flex items-center gap-2">
						{quotaText && (
							<span
								className={`text-xs whitespace-nowrap ${quotaReached ? "text-danger-700 font-semibold" : "text-foreground-soft"}`}
							>
								{quotaReached ? `${quotaText}（下週一重置）` : quotaText}
							</span>
						)}
						<button
							type="button"
							className="text-foreground-soft text-2xl leading-none"
							onClick={onClose}
							aria-label="關閉"
						>
							<AppIcon name="close" size={20} />
						</button>
					</div>
				</header>

				<div className="grid grid-cols-2 gap-3 mb-4 max-[560px]:grid-cols-1">
					<figure className="m-0">
						<figcaption className="text-xs text-foreground-soft mb-1">
							原圖
						</figcaption>
						{originalUrl && (
							// biome-ignore lint/performance/noImgElement: local objectURL preview
							<img
								src={originalUrl}
								alt="原圖"
								className={`w-full aspect-[4/3] rounded-hkui-md border border-divider ${
									category === "product"
										? "bg-hk-surface-muted object-contain"
										: "object-cover"
								}`}
							/>
						)}
					</figure>
					<figure className="m-0">
						<figcaption className="text-xs text-foreground-soft mb-1">
							美化後
						</figcaption>

						<div className="w-full aspect-[4/3] rounded-hkui-md border border-white/50 bg-white/40 dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5 flex flex-col items-center justify-center gap-1.5 overflow-hidden">
							{loading ? (
								<>
									<AiSparkleIcon
										size={64}
										aria-label="美化中"
										className="animate-pulse motion-reduce:animate-none"
									/>
									<span className="text-sm text-foreground-soft">美化中…</span>
								</>
							) : resultUrl ? (
								// biome-ignore lint/performance/noImgElement: local objectURL preview
								<img
									src={resultUrl}
									alt="美化後"
									className={`h-full w-full ${
										category === "product" ? "object-contain" : "object-cover"
									}`}
								/>
							) : (
								<>
									<AiSparkleIcon size={28} />
									<span className="text-sm text-foreground-soft px-3 text-center">
										選擇風格後開始美化
									</span>
								</>
							)}
						</div>
					</figure>
				</div>

				<div className="mb-3">
					<div className="flex gap-2 mb-2" role="tablist">
						<button
							type="button"
							role="tab"
							aria-selected={!useCustom}
							className={`h-8 px-3 rounded-full text-sm font-bold ${!useCustom ? "bg-hkui-primary text-[var(--color-neutral-cream)]" : "bg-white/45 text-foreground-muted dark:bg-white/6"}`}
							onClick={() => setUseCustom(false)}
						>
							預設風格
						</button>
						<button
							type="button"
							role="tab"
							aria-selected={useCustom}
							className={`h-8 px-3 rounded-full text-sm font-bold ${useCustom ? "bg-hkui-primary text-[var(--color-neutral-cream)]" : "bg-white/45 text-foreground-muted dark:bg-white/6"}`}
							onClick={() => setUseCustom(true)}
						>
							自訂提示詞
						</button>
					</div>

					{useCustom ? (
						<textarea
							value={customPrompt}
							onChange={(e) => setCustomPrompt(e.target.value)}
							placeholder="描述你想要的效果，例如：白底、提高亮度、背景柔化"
							rows={3}
							className="w-full rounded-hkui-md border border-white/50 bg-white/45 p-3 text-sm text-foreground focus:border-hkui-primary focus:outline-none dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5"
						/>
					) : (
						<div className="grid grid-cols-4 gap-2 max-[560px]:grid-cols-3 max-[420px]:grid-cols-2">
							{styles.map((s) => {
								const selected = styleId === s.id;
								return (
									<button
										key={s.id}
										type="button"
										aria-pressed={selected}
										className={`group flex flex-col overflow-hidden rounded-hkui-md border text-left transition-colors ${selected ? "border-hkui-primary ring-2 ring-hkui-primary/30" : "border-white/50 hover:border-hkui-primary/50 dark:border-[rgba(245,222,165,0.12)]"}`}
										onClick={() => setStyleId(s.id)}
									>
										<span className="block aspect-square w-full overflow-hidden bg-white/40 dark:bg-white/5">
											{s.thumbnailUrl && !brokenThumbs[s.id] ? (
												// biome-ignore lint/performance/noImgElement: dynamic CDN thumbnail without configured next/image domain
												<img
													src={buildImageUrl(s.thumbnailUrl, "thumb")}
													alt=""
													loading="lazy"
													onError={() =>
														setBrokenThumbs((prev) => ({
															...prev,
															[s.id]: true,
														}))
													}
													className="h-full w-full object-cover"
												/>
											) : (
												<span className="flex h-full w-full items-center justify-center text-foreground-soft">
													<AiSparkleIcon size={22} />
												</span>
											)}
										</span>
										<span
											className={`px-2 py-1.5 text-xs font-semibold ${selected ? "text-hkui-primary" : "text-foreground-muted"}`}
										>
											{s.label}
										</span>
									</button>
								);
							})}
						</div>
					)}
				</div>

				<div className="min-h-5 mb-3">
					{error && (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{error}
						</p>
					)}
				</div>

				<div className="flex items-center justify-end gap-2">
					<Button variant="bordered" color="default" onClick={onClose}>
						取消
					</Button>
					<Button
						onClick={run}
						disabled={loading || quotaReached}
						isLoading={loading}
						loadingLabel={resultUrl ? "重新美化圖片中…" : "圖片美化中…"}
						className="min-w-24"
					>
						{quotaReached ? "本週已用完" : resultUrl ? "重新美化" : "開始美化"}
					</Button>
					<Button onClick={apply} disabled={!resultBlob || loading}>
						套用
					</Button>
				</div>
			</div>
		</div>,
		document.body,
	);
}
