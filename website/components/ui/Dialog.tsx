"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { wrapTabFocus } from "@/lib/a11y/focusable";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { retainOverlayOpenAttribute } from "@/lib/state/overlay-open-attribute";
import { cn } from "@/lib/utils/cn";

let activeOverlayCount = 0;
const inertRoots = new Map<HTMLElement, boolean>();

/** Fallback exit durations (ms) if the CSS custom properties cannot be read
 *  (SSR-adjacent first paint, or a browser that returns "" for the var). */
const EXIT_FALLBACK_REDUCED_MS = 180;
const EXIT_FALLBACK_MS = 400;
/** The fallback timer must outlast the CSS transition it is backstopping, or it
 *  would clip the exit it exists to guarantee. */
const EXIT_TIMEOUT_BUFFER_MS = 80;

/** Reads a CSS time token ("200ms" / "0.32s") off the document root. */
function readDurationMs(token: string, fallback: number): number {
	if (typeof window === "undefined") return fallback;
	const raw = window
		.getComputedStyle(document.documentElement)
		.getPropertyValue(token)
		.trim();
	if (!raw) return fallback;
	const value = Number.parseFloat(raw);
	if (!Number.isFinite(value)) return fallback;
	return raw.endsWith("ms") ? value : value * 1000;
}

type DialogSize = "sm" | "md" | "lg" | "xl" | "full";
type DialogPlacement = "center" | "top" | "bottom";
type SheetSide = "bottom" | "right" | "left";
type OverlayMotion =
	| "dialog-center"
	| "dialog-top"
	| "dialog-bottom"
	| "sheet-bottom"
	| "sheet-right"
	| "sheet-left";

const dialogMotion: Record<DialogPlacement, OverlayMotion> = {
	center: "dialog-center",
	top: "dialog-top",
	bottom: "dialog-bottom",
};

const sheetMotion: Record<SheetSide, OverlayMotion> = {
	bottom: "sheet-bottom",
	right: "sheet-right",
	left: "sheet-left",
};

/**
 * Everything a Dialog and a Sheet share — which is all of it bar the
 * placement/size vocabulary. `aria-label` comes from `React.HTMLAttributes`
 * and is honoured as the panel's accessible name when there is no `title`.
 */
interface OverlayBaseProps
	extends Omit<React.HTMLAttributes<HTMLDivElement>, "children" | "title"> {
	open: boolean;
	onClose: () => void;
	title?: React.ReactNode;
	description?: React.ReactNode;
	children: React.ReactNode;
	footer?: React.ReactNode;
	closeLabel?: string;
	showCloseButton?: boolean;
	isDismissable?: boolean;
	overlayClassName?: string;
	bodyClassName?: string;
	/**
	 * 開啟時是否把焦點移進面板（預設 true）。純狀態 / 等待型 overlay（例如
	 * AuthGateWaitModal）設 false：它不搶焦點，後續接手的 overlay 才能把焦點
	 * 還給使用者真正操作的元素，而不是還給一個即將卸載的面板。
	 */
	autoFocus?: boolean;
}

interface DialogProps extends OverlayBaseProps {
	size?: DialogSize;
	placement?: DialogPlacement;
}

interface SheetProps extends OverlayBaseProps {
	side?: SheetSide;
}

/** @import import { Dialog } from '@/components/ui/Dialog'; */
export function Dialog({
	open,
	onClose,
	title,
	description,
	children,
	footer,
	size = "md",
	placement = "center",
	closeLabel = "關閉",
	showCloseButton = true,
	isDismissable = true,
	autoFocus = true,
	overlayClassName,
	bodyClassName,
	className,
	"aria-label": ariaLabel,
	...rest
}: DialogProps) {
	const titleId = React.useId();
	const descriptionId = React.useId();

	return (
		<OverlayRoot
			open={open}
			onClose={onClose}
			isDismissable={isDismissable}
			autoFocus={autoFocus}
			motion={dialogMotion[placement]}
			panelAlignClass={dialogPlacementClass[placement]}
			panelClassName={cn(dialogSizeClass[size], className)}
			overlayClassName={overlayClassName}
			ariaLabelledBy={title ? titleId : undefined}
			ariaLabel={title ? undefined : (ariaLabel ?? "對話框")}
			ariaDescribedBy={description ? descriptionId : undefined}
			closeButton={
				showCloseButton ? (
					<OverlayCloseButton label={closeLabel} onClose={onClose} />
				) : null
			}
			{...rest}
		>
			{title || description ? (
				<DialogHeader
					title={title}
					titleId={titleId}
					description={description}
					descriptionId={descriptionId}
				/>
			) : null}
			<DialogBody className={bodyClassName}>{children}</DialogBody>
			{footer ? <DialogFooter>{footer}</DialogFooter> : null}
		</OverlayRoot>
	);
}

/** @import import { Sheet } from '@/components/ui/Dialog'; */
export function Sheet({
	open,
	onClose,
	title,
	description,
	children,
	footer,
	side = "bottom",
	closeLabel = "關閉",
	showCloseButton = true,
	isDismissable = true,
	autoFocus = true,
	overlayClassName,
	bodyClassName,
	className,
	"aria-label": ariaLabel,
	...rest
}: SheetProps) {
	const titleId = React.useId();
	const descriptionId = React.useId();

	return (
		<OverlayRoot
			open={open}
			onClose={onClose}
			isDismissable={isDismissable}
			autoFocus={autoFocus}
			motion={sheetMotion[side]}
			panelAlignClass={sheetSideClass[side]}
			panelClassName={cn(sheetPanelClass[side], className)}
			overlayClassName={overlayClassName}
			ariaLabelledBy={title ? titleId : undefined}
			ariaLabel={title ? undefined : (ariaLabel ?? "對話框")}
			ariaDescribedBy={description ? descriptionId : undefined}
			closeButton={
				showCloseButton ? (
					<OverlayCloseButton label={closeLabel} onClose={onClose} />
				) : null
			}
			{...rest}
		>
			{title || description ? (
				<DialogHeader
					title={title}
					titleId={titleId}
					description={description}
					descriptionId={descriptionId}
				/>
			) : null}
			<DialogBody className={bodyClassName}>{children}</DialogBody>
			{footer ? <DialogFooter>{footer}</DialogFooter> : null}
		</OverlayRoot>
	);
}

interface OverlayRootProps extends React.HTMLAttributes<HTMLDivElement> {
	open: boolean;
	onClose: () => void;
	isDismissable: boolean;
	autoFocus: boolean;
	motion: OverlayMotion;
	panelAlignClass: string;
	panelClassName?: string;
	overlayClassName?: string;
	ariaLabelledBy?: string;
	/** Used only when there is no `ariaLabelledBy`; an `aria-modal` dialog must
	 *  never be nameless. */
	ariaLabel?: string;
	ariaDescribedBy?: string;
	closeButton?: React.ReactNode;
	children: React.ReactNode;
}

function OverlayRoot({
	open,
	onClose,
	isDismissable,
	autoFocus,
	motion,
	panelAlignClass,
	panelClassName,
	overlayClassName,
	ariaLabelledBy,
	ariaLabel,
	ariaDescribedBy,
	closeButton,
	children,
	...rest
}: OverlayRootProps) {
	const [mounted, setMounted] = React.useState(false);
	const [present, setPresent] = React.useState(false);
	const [visualState, setVisualState] = React.useState<"closed" | "open">(
		"closed",
	);
	const overlayRef = React.useRef<HTMLDivElement | null>(null);
	const panelRef = React.useRef<HTMLDivElement | null>(null);
	// 生命週期 effect 的依賴固定為 [mounted, present]（tests/dialog-focus-lifecycle
	// 釘住），autoFocus 走 ref 讀最新值，避免切換它時重新取得 scroll lock。
	const autoFocusRef = React.useRef(autoFocus);
	autoFocusRef.current = autoFocus;

	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!mounted) return;
		if (open) {
			setPresent(true);
			return;
		}
		setVisualState("closed");
	}, [mounted, open]);

	React.useEffect(() => {
		if (!mounted || !present) return;
		if (!open) {
			// transitionend normally owns the unmount. This fallback covers browsers
			// that suppress the event and keeps the exit duration bounded.
			const reducedMotion = window.matchMedia?.(
				"(prefers-reduced-motion: reduce)",
			).matches;
			const duration = reducedMotion
				? readDurationMs("--hkui-dur-base", EXIT_FALLBACK_REDUCED_MS)
				: readDurationMs("--hkui-dur-slow", EXIT_FALLBACK_MS);
			const timer = window.setTimeout(
				() => setPresent(false),
				duration + EXIT_TIMEOUT_BUFFER_MS,
			);
			return () => window.clearTimeout(timer);
		}

		// Commit the closed pose for one frame before retargeting to open. This also
		// makes a rapid close -> reopen resume from its current visual position.
		const frame = window.requestAnimationFrame(() => setVisualState("open"));
		return () => window.cancelAnimationFrame(frame);
	}, [mounted, open, present]);

	React.useEffect(() => {
		if (!present) return;
		const previouslyFocused =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		return () => {
			queueMicrotask(() => {
				if (previouslyFocused?.isConnected) {
					previouslyFocused.focus({ preventScroll: true });
				}
			});
		};
	}, [present]);

	const handleDocumentKeyDown = React.useEffectEvent((event: KeyboardEvent) => {
		const overlays = document.querySelectorAll<HTMLElement>(
			"[data-hk-overlay-root]",
		);
		if (overlays.item(overlays.length - 1) !== overlayRef.current) return;
		if (event.key === "Escape" && open && isDismissable) {
			if (event.defaultPrevented) return;
			const nestedListboxOpen = panelRef.current?.querySelector(
				'[aria-haspopup="listbox"][aria-expanded="true"]',
			);
			if (nestedListboxOpen) return;
			event.preventDefault();
			onClose();
			return;
		}
		if (event.key !== "Tab" || !panelRef.current) return;
		// Shared trap (lib/a11y/focusable): it leaves focus alone when the
		// active element sits outside the panel — e.g. a nested portal's
		// listbox — instead of yanking it back to the panel's first control.
		wrapTabFocus(event, panelRef.current);
	});

	React.useEffect(() => {
		if (!mounted || !present) return;
		const releaseScrollLock = acquireBodyScrollLock();
		const releaseOverlayIsolation = applyOverlayIsolation();
		const overlay = overlayRef.current;
		const stopBackdropMotionEvent = (event: Event) => {
			const target = event.target;
			// Keep backdrop motion isolated from document-level visual effects, but
			// let gestures that start inside the panel reach document listeners.
			// Libraries such as react-easy-crop continue a mouse drag there.
			if (!(target instanceof Node) || !panelRef.current?.contains(target)) {
				event.stopPropagation();
			}
		};
		overlay?.addEventListener("mousemove", stopBackdropMotionEvent);
		overlay?.addEventListener("pointermove", stopBackdropMotionEvent);
		document.addEventListener("keydown", handleDocumentKeyDown);
		if (autoFocusRef.current) panelRef.current?.focus({ preventScroll: true });
		return () => {
			overlay?.removeEventListener("mousemove", stopBackdropMotionEvent);
			overlay?.removeEventListener("pointermove", stopBackdropMotionEvent);
			releaseOverlayIsolation();
			releaseScrollLock();
			document.removeEventListener("keydown", handleDocumentKeyDown);
		};
	}, [mounted, present]);

	const completeExit = () => {
		if (!open) setPresent(false);
	};

	if (!mounted || !present) return null;

	return createPortal(
		<div
			ref={overlayRef}
			data-hk-overlay-root
			data-hk-overlay-state={visualState}
			data-hk-overlay-motion={motion}
			className={cn(
				"fixed inset-0 z-[1200] flex bg-[rgb(var(--color-neutral-ink-rgb)_/_0.46)] p-4 backdrop-blur-sm",
				overlayClassName,
			)}
			role="presentation"
			onTransitionEnd={(event) => {
				if (
					event.target === event.currentTarget &&
					event.propertyName === "opacity"
				) {
					completeExit();
				}
			}}
		>
			<button
				type="button"
				className="absolute inset-0 size-full cursor-default"
				aria-label={isDismissable ? "關閉" : undefined}
				aria-hidden={isDismissable ? undefined : true}
				disabled={!isDismissable}
				onClick={isDismissable ? onClose : undefined}
				tabIndex={-1}
			/>
			<div
				data-hk-overlay-motion-layer
				className={cn(
					"pointer-events-none relative z-10 flex size-full",
					panelAlignClass,
				)}
			>
				<div
					ref={panelRef}
					data-hk-overlay-panel
					role="dialog"
					aria-modal="true"
					aria-labelledby={ariaLabelledBy}
					aria-label={ariaLabelledBy ? undefined : ariaLabel}
					aria-describedby={ariaDescribedBy}
					tabIndex={-1}
					className={cn(
						"pointer-events-auto relative flex max-h-full flex-col overflow-hidden border border-[var(--hkui-border)] bg-[var(--hkui-surface)] text-[var(--hkui-foreground)] shadow-[var(--hkui-shadow-glass)] outline-none",
						panelPanelRadiusClass,
						panelClassName,
					)}
					{...rest}
				>
					{closeButton}
					{children}
				</div>
			</div>
		</div>,
		document.body,
	);
}

function DialogHeader({
	title,
	titleId,
	description,
	descriptionId,
}: {
	title?: React.ReactNode;
	titleId: string;
	description?: React.ReactNode;
	descriptionId: string;
}) {
	return (
		<header
			data-hk-dialog-header
			// right-4 (16px) + size-11 (44px) = 60px of chrome to clear.
			className="grid gap-2 border-b border-[var(--hkui-border)] px-6 py-5 pr-16"
		>
			{title ? (
				<h2
					id={titleId}
					className="m-0 text-[length:var(--hkui-display-sm)] font-semibold leading-[var(--hkui-line-large)] text-[var(--hkui-foreground)]"
				>
					{title}
				</h2>
			) : null}
			{description ? (
				<p
					id={descriptionId}
					className="m-0 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-small)] text-[var(--hkui-foreground-muted)]"
				>
					{description}
				</p>
			) : null}
		</header>
	);
}

function DialogBody({
	children,
	className,
	...rest
}: React.HTMLAttributes<HTMLDivElement>) {
	return (
		<div
			data-hk-dialog-body
			// `scrollbar-gutter: stable`：抽屜內容非同步載入/展開收合時高度會在
			// 面板高度上下穿越，捲軸出現↔消失會讓內容反覆重排（底部抖動）。
			// 預留 gutter 讓捲軸切換不再改變內容寬度。
			className={cn(
				"overflow-auto px-6 py-5 [scrollbar-gutter:stable]",
				className,
			)}
			{...rest}
		>
			{children}
		</div>
	);
}

function DialogFooter({
	children,
	className,
	...rest
}: React.HTMLAttributes<HTMLDivElement>) {
	return (
		<footer
			data-hk-dialog-footer
			className={cn(
				"flex flex-wrap items-center justify-end gap-3 border-t border-[var(--hkui-border)] px-6 py-4",
				className,
			)}
			{...rest}
		>
			{children}
		</footer>
	);
}

function OverlayCloseButton({
	label,
	onClose,
}: {
	label: string;
	onClose: () => void;
}) {
	return (
		<button
			type="button"
			data-hk-overlay-close
			className="absolute right-4 top-4 z-10 inline-grid size-11 place-items-center rounded-full border border-transparent text-[var(--hkui-foreground-muted)] transition motion-reduce:transition-none hover:border-[var(--hkui-border)] hover:bg-[var(--hkui-default-100)] hover:text-[var(--hkui-foreground)] focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]"
			onClick={onClose}
			aria-label={label}
		>
			<svg
				width="18"
				height="18"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2.3"
				strokeLinecap="round"
				strokeLinejoin="round"
				aria-hidden="true"
			>
				<path d="M18 6 6 18M6 6l12 12" />
			</svg>
		</button>
	);
}

function applyOverlayIsolation(): () => void {
	const releaseOverlayOpenAttribute = retainOverlayOpenAttribute();
	activeOverlayCount += 1;
	if (activeOverlayCount === 1) {
		for (const root of overlayIsolationRoots()) {
			if (!inertRoots.has(root)) {
				inertRoots.set(root, root.inert);
			}
			root.inert = true;
		}
	}

	let released = false;
	return () => {
		if (released) return;
		released = true;
		releaseOverlayOpenAttribute();
		activeOverlayCount = Math.max(0, activeOverlayCount - 1);
		if (activeOverlayCount > 0) return;

		for (const [root, state] of inertRoots) {
			root.inert = state;
		}
		inertRoots.clear();
	};
}

function overlayIsolationRoots(): HTMLElement[] {
	const roots = [document.querySelector<HTMLElement>(".page-wrapper")];
	return roots.filter((root): root is HTMLElement => root !== null);
}

const panelPanelRadiusClass = "rounded-[var(--hkui-radius-2xl)]";

const dialogSizeClass: Record<DialogSize, string> = {
	sm: "w-full max-w-sm",
	md: "w-full max-w-lg",
	lg: "w-full max-w-2xl",
	xl: "w-full max-w-4xl",
	full: "size-full rounded-[var(--hkui-radius-xl)]",
};

const dialogPlacementClass: Record<DialogPlacement, string> = {
	center: "items-center justify-center",
	top: "items-start justify-center pt-10",
	bottom: "items-end justify-center",
};

const sheetSideClass: Record<SheetSide, string> = {
	bottom: "items-end justify-center",
	right: "items-stretch justify-end",
	left: "items-stretch justify-start",
};

const sheetPanelClass: Record<SheetSide, string> = {
	bottom:
		"max-h-[min(82vh,760px)] w-full max-w-3xl rounded-b-none md:rounded-b-[var(--hkui-radius-2xl)]",
	right:
		"h-full w-full max-w-md rounded-r-none md:rounded-r-[var(--hkui-radius-2xl)]",
	left: "h-full w-full max-w-md rounded-l-none md:rounded-l-[var(--hkui-radius-2xl)]",
};

export type { DialogPlacement, DialogProps, DialogSize, SheetProps, SheetSide };
