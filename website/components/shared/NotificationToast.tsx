"use client";

/**
 * In-app drop-down notification toasts — the web counterpart of the seller /
 * buyer apps' `NotificationBanner`. Each toast slides down from the top with a
 * type-tinted icon, title + body, an optional avatar, a close button, and a
 * countdown bar synced to its auto-dismiss. Tapping the body navigates to
 * `href`.
 *
 * Toasts that share a `groupKey` (same chat sender, same notification title)
 * are collapsed into ONE iOS-style stack: the newest card sits on top with the
 * older ones peeking behind and a count badge. Hovering (desktop) or tapping
 * the stack (touch) fans the cards out into a list; tapping the badge again —
 * or moving away — collapses them. Every card can be flicked sideways to
 * dismiss. All motion runs through GSAP.
 *
 * Rendered through a portal by `NotificationToastViewport` so it floats above
 * the whole app. Triggers are wired in `NotificationToastBridge`.
 */

import type { gsap } from "gsap";
import * as React from "react";
import { createPortal } from "react-dom";
import ToastIcon, { type ToastIconKind } from "@/components/shared/ToastIcon";
import { prefersReducedMotion } from "@/lib/a11y/reduced-motion";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";
import { buildImageUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/* ── GSAP 延後載入 ───────────────────────────────────────────── */

type Gsap = typeof import("gsap")["gsap"];

// 本元件掛在 root layout，靜態 import 會讓 gsap（gzip 約 46KB）進到每一頁的
// layout chunk，而它只在有通知卡片時才會動。改為在 whenDocumentSettled 之後
// 才下載，並且由 NotificationToastViewport 把「模組已載入」當成 portal 的前置
// 條件：任何 ToastCard / ToastStack 被渲染時 gsap 一定已經在記憶體裡，下面每個
// 呼叫點都走同步路徑，進場、倒數、滑動、堆疊動畫的時序與改動前完全相同。
let gsapModule: Gsap | null = null;
let gsapLoading: Promise<Gsap> | null = null;

function loadGsap(): Promise<Gsap> {
	gsapLoading ??= import("gsap").then((mod) => {
		gsapModule = mod.gsap;
		return mod.gsap;
	});
	return gsapLoading;
}

/** Runs synchronously when gsap is already loaded (the normal case, see
 *  above); otherwise queues after the load so call order is preserved. */
function withGsap(run: (g: Gsap) => void): void {
	if (gsapModule) {
		run(gsapModule);
		return;
	}
	void loadGsap().then(run);
}

/**
 * Every animation in this file is GSAP-driven, so `motion-reduce:` variants
 * cannot reach any of it. Callers pass the tween they want and the final pose
 * it lands on; under reduced motion the pose is `set` instantly and the
 * tween's `onComplete` still runs, so dismissal / layout logic is identical —
 * only the movement disappears.
 */
function animate(
	target: gsap.TweenTarget,
	vars: gsap.TweenVars,
	options: { onComplete?: () => void } = {},
): void {
	const { onComplete } = options;
	withGsap((g) => {
		if (prefersReducedMotion()) {
			const { duration, ease, overwrite, onComplete: _drop, ...pose } = vars;
			void duration;
			void ease;
			void _drop;
			g.set(target, { ...pose, overwrite });
			onComplete?.();
			return;
		}
		g.to(target, { ...vars, onComplete });
	});
}

export type ToastKind = "order" | "message" | "system";
/**
 * Semantic colour palette for outcome-oriented toasts (付款成功 / 失敗 等)。
 * `default` 保留既有「主色綠 icon + 主色 countdown bar」行為,所有原 caller
 * 預設走這條,零回歸風險。指定 variant 時覆蓋 icon + 顏色 + countdown bar。
 */
export type ToastVariant = "default" | "success" | "error" | "warning";

export interface ToastView {
	id: number;
	kind?: ToastKind;
	/**
	 * Semantic colour. 預設 default = 既有主色;指定 success/error/warning
	 * 時 icon 換成對應的 ✓/✗/! 且 icon 容器、countdown bar 換對應色系。
	 */
	variant?: ToastVariant;
	title: string;
	body?: string;
	imageUrl?: string;
	/** Navigation target when the toast body is tapped. */
	href?: string;
	durationMs?: number;
	/** Suppress duplicates while an identical toast is still showing. */
	dedupeKey?: string;
	/**
	 * Collapse toasts sharing this key into one expandable stack (e.g.
	 * `chat:<senderId>`, `notif:<title>`). Omit → the toast is its own
	 * singleton stack (every action toast keeps standing alone).
	 */
	groupKey?: string;
}

const DEFAULT_DURATION = 5000;

/* Stack geometry (px) — how far each card behind the front one peeks out, how
 * many peek, the gap once fanned out, and the per-step scale / opacity falloff. */
const PEEK = 10;
const MAX_PEEK = 2;
const EXPANDED_GAP = 10;
const SCALE_STEP = 0.05;
const OPACITY_STEP = 0.16;
/** 堆疊底緣與 sonner toaster 的間距（=viewport 容器 gap-2）。 */
const SONNER_CLEARANCE_PX = 8;

/**
 * 通知堆疊的上緣淨空（#374）：原本只留「安全區 + 12px」的 padding-top，
 * 把卡片貼在畫面最上緣，整疊直接蓋住 navbar（桌機浮動膠囊 y16–100、手機 56px
 * 平頂條）約 4 秒。改成讓開 navbar 佔位再加 12px 呼吸空間。
 * 其中的安全區項已於 web#417 移除：本站 viewport（components/shared/RootDocument.tsx）沒有
 * `viewport-fit: cover`，那個值在任何環境（含真機）都是 0。
 *
 * ⚠️ 註解裡**不要**寫出完整的 Tailwind class 字面值（`pt-[ … ]` 那種形式，
 * 中括號裡刻意留空白才不會被當成候選字）：Tailwind v4 的候選字掃描是純正則、
 * 不跳過註解，寫出來就會把那個已經沒人用的 utility 重新編進 CSS ——被移除的
 * 死碼會從註解裡復活（web#417 實測）。若寫的是 `…` 之類的佔位符，產出的還會是
 * `padding-top: …` 這種**不合法的 CSS**，Lightning CSS 直接 parse 失敗
 * （見 tests/tailwind-comment-candidates.test.ts）。
 *
 * 幾何沿用 navbar 自己的 token（`floating-navigation.css`）而非自訂數字：
 *   - `--floating-nav-height` / `--floating-nav-top-gap` 就是 `.auto-nav`
 *     min-height 的兩個項，≤768px 時同檔會把它們換成 `var(--nav-height,56px)`
 *     與 `0px`，所以同一條 calc 在桌機得 84+16+12、手機得 56+0+12，不需要 JS
 *     斷點，也不會出現第二套幾何。
 *   - fallback 值對齊該檔宣告，變數缺席時 calc 仍成立。
 *
 * 與 sonner 的關係：`AppToaster` 的 `DESKTOP_TOP_OFFSET` = 同一條 calc 再加
 * `var(--hk-notif-toast-h)`。本容器的 padding 不計入 ResizeObserver 的
 * `contentRect.height`（見下方 setStackContainer），因此 sonner 首卡正好落在
 * 通知堆疊底緣下方 SONNER_CLEARANCE_PX —— 兩者同時可見且都不壓 navbar。
 * 手機（≤840px）sonner 改停底部，與本堆疊不再共用區域。
 */
const NAVBAR_CLEARANCE_CLASS =
	"pt-[calc(var(--floating-nav-height,84px)+var(--floating-nav-top-gap,16px)+12px)]";

/* ── type → icon ─────────────────────────────────────────────── */

const KIND_ICON: Record<ToastKind, ToastIconKind> = {
	order: "order",
	message: "message",
	system: "system",
};

interface VariantStyle {
	icon: ToastIconKind;
	/** icon container 背景 (color-mix tint) + icon stroke 色 */
	containerClass: string;
	/** countdown bar 顏色 (Tailwind class with arbitrary value) */
	barClass: string;
}

// default 留空走原 caller-decided kind icon + 主色容器,維持既有行為。
// 其餘 variant 覆蓋 icon + 容器色 + countdown bar 色。
const VARIANT_STYLE: Record<Exclude<ToastVariant, "default">, VariantStyle> = {
	success: {
		icon: "success",
		containerClass:
			"bg-[color-mix(in_srgb,var(--hkui-success)_14%,transparent)] text-[var(--hkui-success-700)]",
		barClass: "bg-[color-mix(in_srgb,var(--hkui-success)_55%,transparent)]",
	},
	error: {
		icon: "error",
		containerClass:
			"bg-[color-mix(in_srgb,var(--hkui-danger)_14%,transparent)] text-[var(--hkui-danger-700)]",
		barClass: "bg-[color-mix(in_srgb,var(--hkui-danger)_55%,transparent)]",
	},
	warning: {
		icon: "warning",
		containerClass:
			"bg-[color-mix(in_srgb,var(--hkui-warning)_14%,transparent)] text-[var(--hkui-warning-700)]",
		barClass: "bg-[color-mix(in_srgb,var(--hkui-warning)_55%,transparent)]",
	},
};

const DEFAULT_CONTAINER_CLASS =
	"bg-[color-mix(in_srgb,var(--hkui-primary)_14%,transparent)] text-hk-primary";
const DEFAULT_BAR_CLASS =
	"bg-[color-mix(in_srgb,var(--hkui-primary)_55%,transparent)]";

/* ── single toast card ───────────────────────────────────────── */

interface ToastCardProps {
	toast: ToastView;
	/** Dismiss just this card. */
	onDismiss: (id: number) => void;
	/** Body tap — expand the stack (collapsed multi) or navigate. */
	onActivate: (toast: ToastView) => void;
	/** Freeze the countdown (stack hovered / expanded) so it can be read. */
	paused: boolean;
	/** Hide the countdown bar for cards tucked behind a collapsed stack. */
	showProgress: boolean;
	/** Allow flick-to-dismiss only on interactive (front / expanded) cards. */
	swipeable: boolean;
}

const ToastCard = React.forwardRef<HTMLDivElement, ToastCardProps>(
	function ToastCard(
		{ toast, onDismiss, onActivate, paused, showProgress, swipeable },
		ref,
	) {
		const duration = toast.durationMs ?? DEFAULT_DURATION;
		const elRef = React.useRef<HTMLDivElement | null>(null);
		const barRef = React.useRef<HTMLDivElement>(null);
		const tweenRef = React.useRef<gsap.core.Tween | null>(null);
		const dismissingRef = React.useRef(false);

		const setRefs = (node: HTMLDivElement | null) => {
			elRef.current = node;
			if (typeof ref === "function") ref(node);
			else if (ref) ref.current = node;
		};

		const variantStyle =
			toast.variant && toast.variant !== "default"
				? VARIANT_STYLE[toast.variant]
				: null;
		const iconKind = variantStyle?.icon ?? KIND_ICON[toast.kind ?? "system"];
		const containerClass =
			variantStyle?.containerClass ?? DEFAULT_CONTAINER_CLASS;
		const barClass = variantStyle?.barClass ?? DEFAULT_BAR_CLASS;
		const imageSrc = buildImageUrl(toast.imageUrl, "thumb");

		const close = React.useCallback(() => {
			if (dismissingRef.current) return;
			dismissingRef.current = true;
			const el = elRef.current;
			if (!el) {
				onDismiss(toast.id);
				return;
			}
			animate(
				el,
				{ opacity: 0, scale: 0.92, duration: 0.2, ease: "power1.in" },
				{ onComplete: () => onDismiss(toast.id) },
			);
		}, [onDismiss, toast.id]);

		// Countdown bar doubles as the auto-dismiss timer — when it empties the
		// card closes. Pausing the tween pauses both at once (no drift).
		const pausedRef = React.useRef(paused);
		pausedRef.current = paused;

		React.useEffect(() => {
			const bar = barRef.current;
			if (!bar) return;
			let cancelled = false;
			withGsap((g) => {
				if (cancelled) return;
				// The bar IS the timer, so it stays a tween even under reduced
				// motion — a linear 5s scaleX is a progress indicator, not
				// decoration, and killing it would remove auto-dismiss entirely.
				// What reduced motion drops is the bar's visibility, below.
				const tween = g.fromTo(
					bar,
					{ scaleX: 1 },
					{
						scaleX: 0,
						duration: duration / 1000,
						ease: "none",
						onComplete: close,
					},
				);
				// 建立當下已處於暫停狀態（堆疊展開中）就直接停住，不留一段漂移。
				if (pausedRef.current) tween.pause();
				tweenRef.current = tween;
			});
			return () => {
				cancelled = true;
				tweenRef.current?.kill();
				tweenRef.current = null;
			};
		}, [duration, close]);

		React.useEffect(() => {
			const tween = tweenRef.current;
			if (!tween) return;
			if (paused) tween.pause();
			else tween.resume();
		}, [paused]);

		/* ── flick-to-dismiss ─────────────────────────────────────── */
		const dragRef = React.useRef<{
			id: number;
			startX: number;
			dx: number;
		} | null>(null);

		const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
			if (!swipeable) return;
			if (e.button !== 0 && e.button !== undefined) return;
			dragRef.current = { id: e.pointerId, startX: e.clientX, dx: 0 };
		};
		const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
			const d = dragRef.current;
			if (!d || e.pointerId !== d.id) return;
			d.dx = e.clientX - d.startX;
			if (Math.abs(d.dx) > 4) {
				try {
					e.currentTarget.setPointerCapture(d.id);
				} catch {
					/* older Safari may refuse capture for touch */
				}
			}
			const el = elRef.current;
			if (el)
				withGsap((g) =>
					g.set(el, {
						x: d.dx,
						opacity: 1 - Math.min(Math.abs(d.dx) / 320, 0.6),
					}),
				);
		};
		const onPointerEnd = (e: React.PointerEvent<HTMLDivElement>) => {
			const d = dragRef.current;
			if (!d || e.pointerId !== d.id) return;
			dragRef.current = null;
			const el = elRef.current;
			if (!el) return;
			if (Math.abs(d.dx) > 96) {
				dismissingRef.current = true;
				animate(
					el,
					{
						x: d.dx > 0 ? 440 : -440,
						opacity: 0,
						duration: 0.26,
						ease: "power2.in",
					},
					{ onComplete: () => onDismiss(toast.id) },
				);
			} else {
				animate(el, { x: 0, opacity: 1, duration: 0.3, ease: "power2.out" });
			}
		};

		const isTap = () => Math.abs(dragRef.current?.dx ?? 0) < 6;

		return (
			// No per-card live region: a three-card stack meant three regions
			// announcing at once, including the cards hidden behind the collapsed
			// pile. The viewport owns one shared announcer instead.
			// biome-ignore lint/a11y/noStaticElementInteractions: the card is a swipe surface; every action inside it (open, close) is a real button, and this handler only adds Escape as a shortcut.
			<div
				ref={setRefs}
				className="hk-notification-toast absolute inset-x-0 top-0 w-full overflow-hidden rounded-[14px] border will-change-transform"
				data-toast-variant={toast.variant ?? "default"}
				data-toast-kind={toast.kind ?? "system"}
				style={{ transformOrigin: "top center", touchAction: "pan-y" }}
				onKeyDown={(event) => {
					// Escape while focus is inside the card dismisses it — the only
					// keyboard exit used to be tabbing to a 24px close button.
					if (event.key !== "Escape" || event.defaultPrevented) return;
					event.preventDefault();
					close();
				}}
				onPointerDown={onPointerDown}
				onPointerMove={onPointerMove}
				onPointerUp={onPointerEnd}
				onPointerCancel={onPointerEnd}
			>
				<div className="flex min-h-12 items-center gap-2.5 px-3 py-2.5">
					<button
						type="button"
						onClick={() => {
							if (isTap()) onActivate(toast);
						}}
						className="flex min-w-0 flex-1 items-center gap-2.5 text-left"
					>
						<span
							className={`grid h-8 w-8 shrink-0 place-items-center rounded-lg ${containerClass} [&_svg]:h-[18px] [&_svg]:w-[18px] [&_svg]:fill-none [&_svg]:stroke-current [&_svg]:[stroke-width:1.9] [&_svg]:[stroke-linecap:round] [&_svg]:[stroke-linejoin:round]`}
						>
							<ToastIcon kind={iconKind} size={18} />
						</span>
						<span className="flex min-w-0 flex-1 flex-col gap-[3px]">
							<strong className="truncate text-[0.84375rem] font-semibold leading-[1.35] text-hk-heading">
								{toast.title}
							</strong>
							{toast.body ? (
								<span className="line-clamp-2 text-xs leading-[1.45] text-hk-muted">
									{toast.body}
								</span>
							) : null}
						</span>
						{imageSrc ? (
							// biome-ignore lint/performance/noImgElement: per-event avatar/thumb URLs aren't pre-known to next/image
							<img
								src={imageSrc}
								alt=""
								width={36}
								height={36}
								className="h-9 w-9 shrink-0 rounded-lg object-cover"
								loading="lazy"
							/>
						) : null}
					</button>
					<button
						type="button"
						aria-label="關閉通知"
						onClick={close}
						// `before:-inset-[10px]` lifts the 24px control to a 44×44 hit
						// area without changing a painted pixel.
						className="relative grid shrink-0 place-items-center self-center rounded-md p-1 text-hk-muted/70 transition-colors before:absolute before:-inset-[10px] before:content-[''] hover:bg-hk-primary/8 hover:text-hk-heading motion-reduce:transition-none"
					>
						<svg
							viewBox="0 0 24 24"
							width="16"
							height="16"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							aria-hidden="true"
						>
							<path d="M6 6l12 12M18 6 6 18" />
						</svg>
					</button>
				</div>
				<div
					className="h-0.5 w-full bg-transparent"
					style={{ opacity: showProgress ? 1 : 0 }}
				>
					<div
						className={`h-full w-full origin-left ${barClass}`}
						ref={barRef}
					/>
				</div>
			</div>
		);
	},
);

/* ── grouped stack ───────────────────────────────────────────── */

function ToastStack({
	items,
	onDismiss,
}: {
	/** Newest-first; index 0 renders on top of the collapsed pile. */
	items: ToastView[];
	onDismiss: (id: number) => void;
}) {
	const router = useObservedRouter("notification_toast");
	const stackRef = React.useRef<HTMLDivElement>(null);
	const cardEls = React.useRef<Map<number, HTMLDivElement>>(new Map());
	const cardRefCbs = React.useRef<
		Map<number, (node: HTMLDivElement | null) => void>
	>(new Map());
	const knownIds = React.useRef<Set<number>>(new Set());
	const [expanded, setExpanded] = React.useState(false);
	const [hovered, setHovered] = React.useState(false);
	const [focused, setFocused] = React.useState(false);

	const count = items.length;
	const multi = count > 1;
	const open = multi && (expanded || hovered);

	// Position every card for the current (collapsed / open) state. Newly
	// arrived cards drop in from above; the rest tween to their slot. Driving
	// all transforms from one place keeps the swipe `x` and stack `y/scale`
	// from fighting (GSAP composes per-property on the same target).
	const layout = React.useCallback(() => {
		const g = gsapModule;
		// Viewport 只在 gsap 載入後才 portal（見 NotificationToastViewport），所以
		// 這裡不會是 null；留守衛只是不讓一個理論上的競態變成例外。
		if (!g) return;
		const heights = items.map(
			(t) => cardEls.current.get(t.id)?.offsetHeight ?? 0,
		);
		items.forEach((t, i) => {
			const el = cardEls.current.get(t.id);
			if (!el) return;
			const isNew = !knownIds.current.has(t.id);
			if (isNew) g.set(el, { x: 0, y: -18, opacity: 0, scale: 0.96 });
			el.style.zIndex = String(100 - i);
			el.style.pointerEvents = open || i === 0 ? "auto" : "none";
			const target = open
				? {
						y:
							heights.slice(0, i).reduce((a, b) => a + b, 0) + i * EXPANDED_GAP,
						scale: 1,
						opacity: 1,
					}
				: {
						y: i * PEEK,
						scale: 1 - i * SCALE_STEP,
						opacity: i <= MAX_PEEK ? 1 - i * OPACITY_STEP : 0,
					};
			animate(el, {
				...target,
				duration: isNew ? 0.5 : 0.42,
				ease: "power3.out",
				overwrite: "auto",
			});
		});
		const containerH = open
			? heights.reduce((a, b) => a + b, 0) + (count - 1) * EXPANDED_GAP
			: (heights[0] ?? 0) + Math.min(count - 1, MAX_PEEK) * PEEK;
		if (stackRef.current) {
			animate(stackRef.current, {
				height: containerH,
				duration: 0.42,
				ease: "power3.out",
			});
		}
		for (const t of items) knownIds.current.add(t.id);
	}, [items, open, count]);

	React.useLayoutEffect(() => {
		layout();
	}, [layout]);

	// Re-flow if a card's content reflows after mount (body wrap / late avatar).
	React.useEffect(() => {
		if (typeof ResizeObserver === "undefined") return;
		const ro = new ResizeObserver(() => layout());
		for (const el of cardEls.current.values()) ro.observe(el);
		return () => ro.disconnect();
	}, [layout]);

	// Hover fans the stack out and pauses the countdown — a desktop-only
	// enhancement, so it's wired as native listeners on the container rather
	// than interactive handlers on a static element (the count badge is the
	// keyboard / touch path). Touch users tap the badge instead.
	React.useEffect(() => {
		const el = stackRef.current;
		if (!el) return;
		const onEnter = () => setHovered(true);
		const onLeave = () => setHovered(false);
		// Keyboard parity: tabbing into a card must stop the countdown too,
		// otherwise the control being read closes itself mid-read. Focus only
		// pauses (it does not fan the stack out) so Tab order stays stable.
		const onFocusIn = () => setFocused(true);
		const onFocusOut = () => setFocused(false);
		el.addEventListener("mouseenter", onEnter);
		el.addEventListener("mouseleave", onLeave);
		el.addEventListener("focusin", onFocusIn);
		el.addEventListener("focusout", onFocusOut);
		return () => {
			el.removeEventListener("mouseenter", onEnter);
			el.removeEventListener("mouseleave", onLeave);
			el.removeEventListener("focusin", onFocusIn);
			el.removeEventListener("focusout", onFocusOut);
		};
	}, []);

	// Stable ref callback per id so a re-render doesn't churn the element map
	// (a transient null between commits could desync the GSAP layout).
	const registerCard = (id: number) => {
		let cb = cardRefCbs.current.get(id);
		if (!cb) {
			cb = (node: HTMLDivElement | null) => {
				if (node) cardEls.current.set(id, node);
				else {
					cardEls.current.delete(id);
					cardRefCbs.current.delete(id);
					knownIds.current.delete(id);
				}
			};
			cardRefCbs.current.set(id, cb);
		}
		return cb;
	};

	const activate = (toast: ToastView) => {
		if (multi && !open) {
			setExpanded(true);
			return;
		}
		if (toast.href) router.push(toast.href);
		// Opening one clears the whole source — matches tapping into the thread.
		for (const t of items) onDismiss(t.id);
	};

	return (
		<div
			className="pointer-events-auto relative w-full max-w-[380px]"
			ref={stackRef}
		>
			{items.map((t, i) => (
				<ToastCard
					key={t.id}
					ref={registerCard(t.id)}
					toast={t}
					onDismiss={onDismiss}
					onActivate={activate}
					paused={open || hovered || focused}
					showProgress={open || i === 0}
					swipeable={open || i === 0}
				/>
			))}
			{multi ? (
				<button
					type="button"
					onClick={() => setExpanded((v) => !v)}
					aria-label={open ? "收合通知" : `展開 ${count} 則通知`}
					// The badge is the ONLY touch path into a collapsed stack (hover
					// is desktop-only), so its 22px box gets a 44×44 hit area from
					// `before:-inset-[11px]` while staying visually 22px.
					className="absolute right-2 top-2 z-[200] grid h-[22px] min-w-[22px] place-items-center rounded-full bg-hk-primary px-1.5 text-xs font-bold leading-none text-white shadow-sm transition-transform before:absolute before:-inset-[11px] before:content-[''] hover:scale-105 motion-reduce:transition-none"
				>
					{open ? "收合" : count}
				</button>
			) : null}
		</div>
	);
}

/* ── viewport (portal) ───────────────────────────────────────── */

/** Group toasts by `groupKey` (singletons get their own key), newest card
 *  first within a group, newest group on top. */
function groupToasts(toasts: ToastView[]): ToastView[][] {
	const groups = new Map<string, ToastView[]>();
	for (const t of toasts) {
		const key = t.groupKey || `__one:${t.id}`;
		const bucket = groups.get(key);
		if (bucket) bucket.push(t);
		else groups.set(key, [t]);
	}
	return [...groups.values()]
		.map((arr) => [...arr].sort((a, b) => b.id - a.id))
		.sort((ga, gb) => gb[0].id - ga[0].id);
}

export function NotificationToastViewport({
	toasts,
	onDismiss,
}: {
	toasts: ToastView[];
	onDismiss: (id: number) => void;
}) {
	const [documentSettled, setDocumentSettled] = React.useState(false);
	React.useEffect(() => {
		let active = true;

		// Realtime events can arrive immediately after authentication. Keep their
		// queue in context, but do not portal a new child into <body> until React
		// has finished claiming the initial streamed document.
		// settle 之後才開始下載 gsap（不進首屏的 layout chunk、不與初始載入搶頻寬），
		// 而 portal 要等它到齊才掛：卡片渲染時動畫引擎一定就緒。
		const ready = whenDocumentSettled().then(loadGsap);
		void ready.then(() => {
			if (active) setDocumentSettled(true);
		});

		return () => {
			active = false;
		};
	}, []);

	// One shared polite announcer for every stack (see the region below).
	// Ids are a monotonic counter, so "already announced" is a set of ids; it
	// is cleared wholesale once it grows past the cap rather than tracked
	// per-toast, which cannot resurrect an announcement for a live toast.
	const [announcement, setAnnouncement] = React.useState("");
	const announcedIdsRef = React.useRef<Set<number>>(new Set());
	React.useEffect(() => {
		const fresh = toasts.filter((t) => !announcedIdsRef.current.has(t.id));
		if (fresh.length === 0) return;
		if (announcedIdsRef.current.size > 200) announcedIdsRef.current.clear();
		for (const t of fresh) announcedIdsRef.current.add(t.id);
		setAnnouncement(fresh.map((t) => t.title).join("；"));
	}, [toasts]);

	// 佔位協調：通知堆疊與 sonner Toaster 同佔 top-center（0151c0ec 統一視覺
	// 語言），通知 z-[1100] 會完整蓋住 sonner（z=--z-toast:700）——error toast
	// 被推播蓋掉時使用者會誤以為操作成功。把堆疊實際內容高度寫進
	// `--hk-notif-toast-h`，AppToaster 的 offset 以 calc(var()) 讀取即時下移，
	// 兩者同時可見。ResizeObserver 的 contentRect 不含 padding（safe-area 上緣
	// 不重複計入）；GSAP 展開/收合 tween 期間父容器高度跟著子層變化，RO 逐幀
	// 回報。清空 → 本元件 return null 卸載 portal → React 19 ref cleanup →
	// 變數歸零，sonner 滑回原位（過渡在 ui-kit.css 的 toaster 上）。
	const setStackContainer = React.useCallback((node: HTMLElement) => {
		if (typeof ResizeObserver === "undefined") return;
		const observer = new ResizeObserver((entries) => {
			const height = entries[entries.length - 1]?.contentRect.height ?? 0;
			document.documentElement.style.setProperty(
				"--hk-notif-toast-h",
				// SONNER_CLEARANCE_PX：堆疊底緣到 sonner 首卡的間距，對齊容器的
				// Tailwind gap-2（8px）。
				height > 0 ? `${Math.round(height) + SONNER_CLEARANCE_PX}px` : "0px",
			);
		});
		observer.observe(node);
		return () => {
			observer.disconnect();
			document.documentElement.style.setProperty("--hk-notif-toast-h", "0px");
		};
	}, []);

	if (!documentSettled || toasts.length === 0) return null;

	const groups = groupToasts(toasts);

	return createPortal(
		// One labelled region (`section` + label = role="region") for the whole
		// notification area. Cards used to carry `role="status"` each, so a
		// three-card stack announced three times over — including the two hidden
		// behind the collapsed pile.
		<section
			ref={setStackContainer}
			aria-label="通知"
			className={`pointer-events-none fixed inset-x-0 top-0 z-[1100] flex flex-col items-center gap-2 px-3 ${NAVBAR_CLEARANCE_CLASS}`}
		>
			{/* The single announcer for every stack: only titles that have not
			    been read out yet reach it. */}
			<span className="sr-only" aria-live="polite">
				{announcement}
			</span>
			{groups.map((group) => (
				<ToastStack
					key={group[0].groupKey || group[0].id}
					items={group}
					onDismiss={onDismiss}
				/>
			))}
		</section>,
		document.body,
	);
}
