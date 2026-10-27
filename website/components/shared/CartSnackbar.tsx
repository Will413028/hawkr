"use client";

import * as React from "react";
import Link from "@/components/shared/Link";

export const CART_TOAST_EVENT = "hawkr:cart-toast";

/**
 * A refusal is not a confirmation. Every failed add ("不可購買自己上架的商品",
 * "商品已下架") used to arrive on the success pill — green surface, lime tick —
 * so the only thing separating "we did it" from "we refused" was the sentence.
 * The tick actively contradicted the words.
 */
export type CartToastTone = "success" | "error";

export interface CartToastDetail {
	message?: string;
	tone?: CartToastTone;
	actionLabel?: string;
	/**
	 * Where the toast CTA links. Pass `null` to drop the CTA entirely — used by
	 * message-only toasts (e.g. 補貨通知) where "查看購物車" would make no sense.
	 * Error toasts default to no CTA: nothing was added, so "查看購物車" would
	 * send the buyer to look for an item that is not there.
	 */
	actionHref?: string | null;
}

interface ToastState {
	message: string;
	tone: CartToastTone;
	actionLabel: string;
	actionHref: string | null;
	key: number;
}

const DEFAULT_MESSAGE = "已加入購物車";
const DEFAULT_ACTION_LABEL = "查看購物車";
const DEFAULT_ACTION_HREF = "/cart";
/** Errors carry more to read, and re-reading a vanished refusal is not possible. */
const DISMISS_MS: Record<CartToastTone, number> = {
	success: 3500,
	error: 6000,
};

// 15 個原始色收斂成 --page-cart-snackbar-* 一族（明暗兩套值逐字沿用原字面值）。
const TONE_SURFACE_CLASS: Record<CartToastTone, string> = {
	success:
		"bg-[var(--page-cart-snackbar-success-bg)] text-[var(--page-cart-snackbar-success-fg)] shadow-[var(--page-cart-snackbar-success-shadow)] dark:border dark:border-[var(--page-cart-snackbar-success-border)]",
	// Same pill geometry, warm-dark surface + danger rim: reads as "stop"
	// at a glance without inventing a second toast component.
	error:
		"bg-[var(--page-cart-snackbar-error-bg)] text-[var(--page-cart-snackbar-error-fg)] border border-[var(--page-cart-snackbar-error-border)] shadow-[var(--page-cart-snackbar-error-shadow)]",
};

const TONE_ICON_CLASS: Record<CartToastTone, string> = {
	success: "text-[var(--page-cart-snackbar-success-icon)]",
	error: "text-[var(--page-cart-snackbar-error-icon)]",
};

/**
 * 手機的 bottom 讓開底部導覽列：引用 navigation-final.css 的那兩個變數，不再抄
 * 一份數字。這裡曾寫死 `calc(env(safe-area-inset-bottom,0px)+80px)`，而 80px 對不上
 * 導覽列的實際佔用（68px 高 + 10px 離底距離 = 78px）——是本 repo 第七份互相漂移的
 * 導覽列常數（web#417）。
 *
 * fallback 必須逐字等於宣告值：那兩個變數只宣告在
 * `@media not (min-width: 840px)` 內，≥840px 不存在，少了 fallback 整條 calc()
 * 會失效。由 tests/mobile-bottom-nav-tokens.test.ts 釘住。
 *
 * `env(safe-area-inset-bottom)` 一併移除：本站 viewport（components/shared/RootDocument.tsx）沒有
 * `viewport-fit: cover`，該值在任何環境（含真機）都解析為 0，是死碼（web#417）。
 *
 * 手機／桌機的分界必須是**導覽列的斷點**，不是 Tailwind 的 `md`。這裡曾走 `md:`
 * （768px），而導覽列顯示到 839px 為止（`MobileBottomNav.tsx` 的 `max-[840px]:grid`）
 * ——768–839px（平板直立的常見寬度）因此拿桌機的 24px 位移，直接被導覽列蓋住。
 * 現在兩側都用 840px：`max-[840px]:` 編成 `not (min-width: 840px)`、`min-[840px]:`
 * 編成 `(min-width: 840px)`，互為補集，正好 840px 那一像素不重疊也不落空。
 *
 * 840 這個數字的唯一真實來源是 `MobileBottomNav.tsx`（本 repo 無具名斷點，
 * 慣例即 `max-[840px]:` 任意值）；兩處同步由 tests/mobile-bottom-nav-tokens.test.ts
 * 從元件字面值反推後釘住，改斷點只要改元件，這裡會被測試逼著跟上。
 */
const SNACKBAR_POSITION_CLASS =
	"fixed z-[var(--z-toast)] pointer-events-none flex " +
	"bottom-[calc(var(--mobile-bottom-nav-height,68px)+var(--mobile-bottom-nav-bottom,10px))] " +
	"left-4 right-4 justify-stretch min-[840px]:left-1/2 min-[840px]:right-auto " +
	"min-[840px]:-translate-x-1/2 min-[840px]:bottom-6 " +
	"min-[840px]:max-w-[calc(100vw-32px)] min-[840px]:justify-center group";

export default function CartSnackbar() {
	const [toast, setToast] = React.useState<ToastState | null>(null);
	const timerRef = React.useRef<number | null>(null);
	// Monotonic counter, not Date.now(): two toasts fired inside the same
	// millisecond shared a key and the second skipped its entry animation.
	const keySeqRef = React.useRef(0);

	const dismiss = React.useCallback(() => {
		if (timerRef.current != null) {
			window.clearTimeout(timerRef.current);
			timerRef.current = null;
		}
		setToast(null);
	}, []);

	React.useEffect(() => {
		const onToast = (event: Event) => {
			const detail = (event as CustomEvent<CartToastDetail>).detail || {};
			if (timerRef.current != null) {
				window.clearTimeout(timerRef.current);
			}
			const tone: CartToastTone = detail.tone === "error" ? "error" : "success";
			setToast({
				message: detail.message || DEFAULT_MESSAGE,
				tone,
				actionLabel: detail.actionLabel || DEFAULT_ACTION_LABEL,
				// `null` → no CTA; `undefined` → the default cart link, but only on
				// success. A refusal added nothing, so its default is no CTA.
				actionHref:
					detail.actionHref === null
						? null
						: detail.actionHref ||
							(tone === "error" ? null : DEFAULT_ACTION_HREF),
				key: ++keySeqRef.current,
			});
			timerRef.current = window.setTimeout(() => {
				setToast(null);
				timerRef.current = null;
			}, DISMISS_MS[tone]);
		};

		window.addEventListener(CART_TOAST_EVENT, onToast);
		return () => {
			window.removeEventListener(CART_TOAST_EVENT, onToast);
			if (timerRef.current != null) window.clearTimeout(timerRef.current);
		};
	}, []);

	// Escape closes it. The only other exits were a 32px button and waiting
	// out 3.5s / 6s.
	React.useEffect(() => {
		if (!toast) return;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape" && !event.defaultPrevented) dismiss();
		};
		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [toast, dismiss]);

	return (
		<div
			className={SNACKBAR_POSITION_CLASS}
			data-state={toast ? "open" : "closed"}
			data-tone={toast?.tone ?? "success"}
			// A refusal must interrupt: it means the action the buyer just took did
			// not happen. `polite` would queue it behind whatever else is speaking.
			role={toast?.tone === "error" ? "alert" : "status"}
			aria-live={toast?.tone === "error" ? "assertive" : "polite"}
		>
			{toast ? (
				<div
					key={toast.key}
					// `min-w-0` is what keeps the pill inside the viewport. As a flex
					// item it defaults to `min-width: auto`, i.e. its own min-content
					// width — and because the message runs `whitespace-nowrap`, that
					// min-content is the *entire* untruncated string. The pill then
					// refuses to shrink to its `left-4 right-4` track and the CTA is
					// pushed off-screen (mobile), or it spills symmetrically past both
					// edges once the message outgrows `max-w-[calc(100vw-32px)]`
					// (desktop). Same defect, both breakpoints.
					className={`pointer-events-auto inline-flex min-w-0 items-center gap-[14px] py-3 pr-3.5 pl-[18px] min-h-[52px] rounded-full font-sans text-[0.9375rem] font-semibold tracking-[0.2px] animate-cart-snackbar-in group-data-[state=closed]:animate-cart-snackbar-out motion-reduce:animate-none group-data-[state=closed]:motion-reduce:animate-none max-[840px]:flex-1 max-[840px]:justify-between max-[840px]:text-sm ${TONE_SURFACE_CLASS[toast.tone]}`}
				>
					<svg
						className={`flex-shrink-0 ${TONE_ICON_CLASS[toast.tone]}`}
						width="20"
						height="20"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2.2"
						strokeLinecap="round"
						strokeLinejoin="round"
						aria-hidden="true"
					>
						{toast.tone === "error" ? (
							<>
								<circle cx="12" cy="12" r="9" />
								<line x1="12" y1="7.5" x2="12" y2="13" />
								<line x1="12" y1="16.4" x2="12" y2="16.5" />
							</>
						) : (
							<polyline points="20 6 9 17 4 12" />
						)}
					</svg>
					{/* `min-w-0 truncate` is unconditional: an overlong product name
					    overflows at *every* width, so the ellipsis is the guard, not a
					    mobile style. `flex-1` stays mobile-only — that is the phone's
					    full-bleed layout choice (message eats the slack, CTA pinned
					    right), whereas the desktop pill is shrink-to-fit. */}
					<span className="min-w-0 truncate max-[840px]:flex-1">
						{toast.message}
					</span>
					{toast.actionHref ? (
						<Link
							href={toast.actionHref}
							className="inline-flex min-h-11 items-center justify-center px-4 py-2 rounded-full bg-[var(--page-cart-snackbar-cta-bg)] text-[var(--page-cart-snackbar-cta-fg)]! font-bold text-sm no-underline transition-[filter,transform] duration-150 ease-out hover:brightness-105 active:translate-y-[1px] motion-reduce:transition-none motion-reduce:active:translate-y-0"
							onClick={dismiss}
						>
							{toast.actionLabel}
						</Link>
					) : null}
					<button
						type="button"
						// size-11：觸控目標下限；14px 字符與圓形視覺不變（圓形背景只
						// 在 hover 出現，原本就是 32px 的淡色圓，改為 44px 仍在 pill 內）。
						className="inline-flex items-center justify-center size-11 p-0 border-0 bg-transparent text-[var(--page-cart-snackbar-close-fg)] rounded-full cursor-pointer transition-[background,color] duration-150 ease-out hover:bg-white/8 hover:text-[var(--page-cart-snackbar-close-hover-fg)] motion-reduce:transition-none"
						onClick={dismiss}
						aria-label="關閉提示"
					>
						<svg
							width="14"
							height="14"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2.4"
							strokeLinecap="round"
							strokeLinejoin="round"
							aria-hidden="true"
						>
							<line x1="6" y1="6" x2="18" y2="18" />
							<line x1="6" y1="18" x2="18" y2="6" />
						</svg>
					</button>
				</div>
			) : null}
		</div>
	);
}

export function emitCartToast(detail?: CartToastDetail) {
	if (typeof window === "undefined") return;
	window.dispatchEvent(
		new CustomEvent<CartToastDetail>(CART_TOAST_EVENT, { detail }),
	);
}
