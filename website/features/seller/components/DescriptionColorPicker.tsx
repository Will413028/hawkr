"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import {
	type DescriptionHsvColor,
	descriptionColorPoint,
	descriptionHexToHsv,
	descriptionHexToRgb,
	descriptionHsvToHex,
	descriptionRgbToHex,
	normalizeDescriptionHex,
	parseDescriptionRgb,
} from "@/lib/products/descriptionEditorColor";
import styles from "@/styles/features/seller/DescriptionColorPicker.module.css";

export interface DescriptionColorPickerProps {
	/** Current opaque text color. Accepts 3- or 6-digit hex; outputs #rrggbb. */
	value: string;
	/** Runs once on Apply; pointer/keyboard/input changes remain a local draft. */
	onApply: (hex: string) => void;
	/** Capture the editor selection before pointer focus enters the picker. */
	onOpen?: () => void;
	disabled?: boolean;
	/** Use the editor's native dialog to remain in its top layer in fullscreen. */
	portalContainer?: HTMLElement | null;
}

// Hawkr brand and status colors from DESIGN.md, used as editable content.
const PRESETS = [
	{ value: "#191c18", label: "墨黑" },
	{ value: "#42493f", label: "正文灰" },
	{ value: "#b8441f", label: "品牌深橘" },
	{ value: "#ea6021", label: "柿橙" },
	{ value: "#f2a541", label: "品牌輔橘" },
	{ value: "#1e5fab", label: "深藍" },
] as const;

const RGB_LABELS = ["紅色 R", "綠色 G", "藍色 B"] as const;
const FALLBACK = "#42493f";
const clampPercent = (value: number) => Math.min(100, Math.max(0, value));

/** Feature-local color editor with a reversible draft and an explicit commit. */
export function DescriptionColorPicker({
	value,
	onApply,
	onOpen,
	disabled = false,
	portalContainer,
}: DescriptionColorPickerProps) {
	const current = normalizeDescriptionHex(value) ?? FALLBACK;
	const [open, setOpen] = React.useState(false);
	const [hsv, setHsv] = React.useState(() => descriptionHexToHsv(current));
	const [format, setFormat] = React.useState<"hex" | "rgb">("hex");
	const [hexInput, setHexInput] = React.useState(current);
	const [rgbInputs, setRgbInputs] = React.useState(() =>
		descriptionHexToRgb(current).map(String),
	);
	const [position, setPosition] = React.useState({
		left: 8,
		top: 8,
		width: 320,
		maxHeight: 600,
		ready: false,
	});
	const triggerRef = React.useRef<HTMLButtonElement>(null);
	const panelRef = React.useRef<HTMLDivElement>(null);
	const areaRef = React.useRef<HTMLDivElement>(null);
	const captureRef = React.useRef(false);
	const activePointerRef = React.useRef<number | null>(null);
	const id = React.useId();
	const draftHex = descriptionHsvToHex(hsv);
	const validHex =
		format === "hex"
			? normalizeDescriptionHex(hexInput)
			: (() => {
					const rgb = parseDescriptionRgb(rgbInputs);
					return rgb ? descriptionRgbToHex(rgb) : null;
				})();
	const error = validHex
		? ""
		: format === "hex"
			? "請輸入 3 或 6 位 HEX 色碼，例如 #b8441f。"
			: "R、G、B 請各輸入 0 到 255 的整數。";

	const setColor = (hex: string) => {
		setHsv((previous) => descriptionHexToHsv(hex, previous.h));
		setHexInput(hex);
		setRgbInputs(descriptionHexToRgb(hex).map(String));
	};
	const setAreaColor = (next: DescriptionHsvColor) => {
		setHsv(next);
		const hex = descriptionHsvToHex(next);
		setHexInput(hex);
		setRgbInputs(descriptionHexToRgb(hex).map(String));
	};
	const close = React.useCallback((restoreFocus = true) => {
		setOpen(false);
		activePointerRef.current = null;
		if (restoreFocus) triggerRef.current?.focus({ preventScroll: true });
	}, []);

	React.useLayoutEffect(() => {
		if (!open) return;
		const updatePosition = () => {
			const trigger = triggerRef.current;
			const panel = panelRef.current;
			if (!trigger || !panel) return;
			const viewport = window.visualViewport;
			const leftEdge = viewport?.offsetLeft ?? 0;
			const topEdge = viewport?.offsetTop ?? 0;
			const width = viewport?.width ?? window.innerWidth;
			const height = viewport?.height ?? window.innerHeight;
			const panelWidth = Math.min(320, Math.max(0, width - 16));
			const maxHeight = Math.max(0, height - 16);
			const panelHeight = Math.min(
				panel.getBoundingClientRect().height,
				maxHeight,
			);
			const anchor = trigger.getBoundingClientRect();
			const below = anchor.bottom + 8;
			const above = anchor.top - panelHeight - 8;
			const top =
				below + panelHeight <= topEdge + height - 8
					? below
					: above >= topEdge + 8
						? above
						: topEdge + 8;
			const left = Math.min(
				Math.max(anchor.left, leftEdge + 8),
				leftEdge + width - panelWidth - 8,
			);
			setPosition((previous) =>
				previous.left === left &&
				previous.top === top &&
				previous.width === panelWidth &&
				previous.maxHeight === maxHeight &&
				previous.ready
					? previous
					: { left, top, width: panelWidth, maxHeight, ready: true },
			);
		};
		updatePosition();
		const observer = new ResizeObserver(updatePosition);
		if (panelRef.current) observer.observe(panelRef.current);
		window.addEventListener("resize", updatePosition);
		window.addEventListener("scroll", updatePosition, true);
		window.visualViewport?.addEventListener("resize", updatePosition);
		window.visualViewport?.addEventListener("scroll", updatePosition);
		return () => {
			observer.disconnect();
			window.removeEventListener("resize", updatePosition);
			window.removeEventListener("scroll", updatePosition, true);
			window.visualViewport?.removeEventListener("resize", updatePosition);
			window.visualViewport?.removeEventListener("scroll", updatePosition);
		};
	}, [open]);

	React.useLayoutEffect(() => {
		if (open && position.ready) areaRef.current?.focus({ preventScroll: true });
	}, [open, position.ready]);

	React.useEffect(() => {
		if (!open) return;
		const dismiss = (event: PointerEvent) => {
			const target = event.target as Node;
			if (
				!panelRef.current?.contains(target) &&
				!triggerRef.current?.contains(target)
			)
				close(false);
		};
		const handleEscape = (event: KeyboardEvent) => {
			if (event.key !== "Escape") return;
			event.preventDefault();
			event.stopPropagation();
			close();
		};
		document.addEventListener("pointerdown", dismiss, true);
		document.addEventListener("keydown", handleEscape, true);
		return () => {
			document.removeEventListener("pointerdown", dismiss, true);
			document.removeEventListener("keydown", handleEscape, true);
		};
	}, [open, close]);

	React.useEffect(() => {
		if (disabled && open) close(false);
	}, [disabled, open, close]);

	const target =
		typeof document === "undefined"
			? null
			: (portalContainer ??
				triggerRef.current?.closest("dialog[open]") ??
				document.body);
	const panel =
		open && target
			? createPortal(
					<div
						ref={panelRef}
						id={`${id}-panel`}
						role="dialog"
						aria-label="文字顏色"
						className={styles.panel}
						style={{
							left: position.left,
							top: position.top,
							width: position.width,
							maxHeight: position.maxHeight,
							visibility: position.ready ? "visible" : "hidden",
						}}
						onKeyDown={(event) => {
							if (event.key !== "Tab") return;
							const controls = panelRef.current?.querySelectorAll<HTMLElement>(
								'button:not(:disabled), input:not(:disabled), select:not(:disabled), [tabindex="0"]',
							);
							if (!controls?.length) return;
							const first = controls[0];
							const last = controls[controls.length - 1];
							if (event.shiftKey && document.activeElement === first) {
								event.preventDefault();
								last.focus();
							} else if (!event.shiftKey && document.activeElement === last) {
								event.preventDefault();
								first.focus();
							}
						}}
					>
						<div className={styles.heading}>
							<span>文字顏色</span>
							<button
								type="button"
								className={styles.close}
								aria-label="關閉色盤"
								onClick={() => close()}
							>
								<span aria-hidden="true">×</span>
							</button>
						</div>
						<div
							ref={areaRef}
							className={styles.area}
							role="slider"
							tabIndex={0}
							aria-label="飽和度與亮度"
							aria-valuemin={0}
							aria-valuemax={100}
							aria-valuenow={Math.round(hsv.s)}
							aria-valuetext={`飽和度 ${Math.round(hsv.s)}%，亮度 ${Math.round(hsv.v)}%`}
							aria-describedby={`${id}-area-help`}
							style={{ backgroundColor: `hsl(${hsv.h} 100% 50%)` }}
							onPointerDown={(event) => {
								if (event.button !== 0) return;
								event.preventDefault();
								event.currentTarget.focus({ preventScroll: true });
								event.currentTarget.setPointerCapture(event.pointerId);
								activePointerRef.current = event.pointerId;
								setAreaColor({
									...hsv,
									...descriptionColorPoint(
										event.clientX,
										event.clientY,
										event.currentTarget.getBoundingClientRect(),
									),
								});
							}}
							onPointerMove={(event) => {
								if (activePointerRef.current !== event.pointerId) return;
								setAreaColor({
									...hsv,
									...descriptionColorPoint(
										event.clientX,
										event.clientY,
										event.currentTarget.getBoundingClientRect(),
									),
								});
							}}
							onPointerUp={() => {
								activePointerRef.current = null;
							}}
							onPointerCancel={() => {
								activePointerRef.current = null;
							}}
							onLostPointerCapture={() => {
								activePointerRef.current = null;
							}}
							onKeyDown={(event) => {
								const step = event.shiftKey ? 10 : 1;
								let next = hsv;
								if (event.key === "ArrowLeft")
									next = { ...hsv, s: clampPercent(hsv.s - step) };
								else if (event.key === "ArrowRight")
									next = { ...hsv, s: clampPercent(hsv.s + step) };
								else if (event.key === "ArrowUp")
									next = { ...hsv, v: clampPercent(hsv.v + step) };
								else if (event.key === "ArrowDown")
									next = { ...hsv, v: clampPercent(hsv.v - step) };
								else if (event.key === "Home") next = { ...hsv, s: 0 };
								else if (event.key === "End") next = { ...hsv, s: 100 };
								else return;
								event.preventDefault();
								event.stopPropagation();
								setAreaColor(next);
							}}
						>
							<span
								className={styles.areaThumb}
								aria-hidden="true"
								style={{
									left: `${hsv.s}%`,
									top: `${100 - hsv.v}%`,
									backgroundColor: draftHex,
								}}
							/>
						</div>
						<p className={styles.srOnly} id={`${id}-area-help`}>
							左右方向鍵調整飽和度，上下方向鍵調整亮度；按住 Shift 加快調整。
						</p>
						<label className={styles.hueLabel}>
							<span>色相</span>
							<span>{Math.round(hsv.h)}°</span>
							<input
								className={styles.hue}
								type="range"
								aria-label="色相"
								min={0}
								max={359}
								step={1}
								value={Math.min(359, Math.round(hsv.h))}
								onChange={(event) =>
									setAreaColor({ ...hsv, h: Number(event.target.value) })
								}
							/>
						</label>
						<div className={styles.preview}>
							<span
								className={styles.previewSwatch}
								aria-hidden="true"
								style={{ backgroundColor: draftHex }}
							/>
							<span>選取顏色</span>
							<output aria-label="選取色碼">{draftHex.toUpperCase()}</output>
						</div>
						<div className={styles.values}>
							<label className={styles.format}>
								<span className={styles.srOnly}>色碼格式</span>
								<select
									aria-label="色碼格式"
									value={format}
									onChange={(event) => {
										setFormat(event.target.value as "hex" | "rgb");
										setColor(draftHex);
									}}
								>
									<option value="hex">HEX</option>
									<option value="rgb">RGB</option>
								</select>
							</label>
							{format === "hex" ? (
								<input
									className={styles.hex}
									aria-label="HEX 色碼"
									aria-invalid={!validHex}
									aria-describedby={error ? `${id}-error` : undefined}
									value={hexInput}
									autoComplete="off"
									spellCheck={false}
									onChange={(event) => {
										const raw = event.target.value;
										setHexInput(raw);
										const hex = normalizeDescriptionHex(raw);
										if (hex) {
											setHsv((previous) =>
												descriptionHexToHsv(hex, previous.h),
											);
											setRgbInputs(descriptionHexToRgb(hex).map(String));
										}
									}}
									onBlur={() => {
										const hex = normalizeDescriptionHex(hexInput);
										if (hex) setHexInput(hex);
									}}
								/>
							) : (
								<div className={styles.rgb}>
									{rgbInputs.map((channel, index) => (
										<label key={RGB_LABELS[index]}>
											<span>{["R", "G", "B"][index]}</span>
											<input
												aria-label={RGB_LABELS[index]}
												inputMode="numeric"
												value={channel}
												autoComplete="off"
												spellCheck={false}
												aria-invalid={
													!/^\d{1,3}$/.test(channel.trim()) ||
													Number(channel) > 255
												}
												aria-describedby={error ? `${id}-error` : undefined}
												onChange={(event) => {
													const next = rgbInputs.map((previous, position) =>
														position === index ? event.target.value : previous,
													);
													setRgbInputs(next);
													const rgb = parseDescriptionRgb(next);
													if (rgb) {
														const hex = descriptionRgbToHex(rgb);
														setHsv((previous) =>
															descriptionHexToHsv(hex, previous.h),
														);
														setHexInput(hex);
													}
												}}
											/>
										</label>
									))}
								</div>
							)}
						</div>
						{error && (
							<p id={`${id}-error`} role="status" className={styles.error}>
								{error}
							</p>
						)}
						<div className={styles.presetSection}>
							<span className={styles.sectionLabel}>品牌常用色</span>
							<div className={styles.presets}>
								{PRESETS.map((preset) => (
									<button
										type="button"
										key={preset.value}
										aria-label={`${preset.label} ${preset.value}`}
										aria-pressed={draftHex === preset.value}
										title={preset.label}
										className={styles.preset}
										onClick={() => setColor(preset.value)}
									>
										<span style={{ backgroundColor: preset.value }} />
									</button>
								))}
							</div>
						</div>
						<div className={styles.footer}>
							<button
								type="button"
								className={styles.cancel}
								onClick={() => close()}
							>
								取消
							</button>
							<button
								type="button"
								className={styles.apply}
								disabled={!validHex}
								onClick={() => {
									if (!validHex) return;
									close();
									onApply(validHex);
								}}
							>
								套用顏色
							</button>
						</div>
					</div>,
					target,
				)
			: null;

	return (
		<>
			<button
				ref={triggerRef}
				type="button"
				className={styles.trigger}
				disabled={disabled}
				aria-label="文字顏色"
				data-description-color-trigger
				aria-haspopup="dialog"
				aria-expanded={open}
				aria-controls={open ? `${id}-panel` : undefined}
				title="文字顏色"
				onPointerDown={(event) => {
					if (event.button === 0 && !open && !disabled) {
						onOpen?.();
						captureRef.current = true;
					}
				}}
				onClick={(event) => {
					if (open) {
						close();
						return;
					}
					if (!captureRef.current || event.detail === 0) onOpen?.();
					captureRef.current = false;
					setColor(current);
					setPosition((previous) => ({ ...previous, ready: false }));
					setOpen(true);
				}}
			>
				<span className={styles.triggerLetter} aria-hidden="true">
					A<span style={{ backgroundColor: current }} />
				</span>
				<AppIcon name="chevron-down" size={12} />
			</button>
			{panel}
		</>
	);
}
