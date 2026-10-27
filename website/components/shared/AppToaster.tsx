"use client";

import * as React from "react";
import ToastIcon from "@/components/shared/ToastIcon";
import { markToastViewportReady } from "@/lib/feedback/toast";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";

type SonnerToaster = typeof import("sonner")["Toaster"];

interface AppToasterProps {
	id?: string;
	containerAriaLabel?: string;
}

// 底部停靠的條件必須與底部導覽列**同真同假**：底部 offset（下方
// MOBILE_DOCK_BOTTOM_OFFSET）整條都是為了讓開那根導覽列，導覽列不在就沒有東西要讓。
// `MobileBottomNav.tsx` 用的是 `max-[840px]:grid`，Tailwind v4 為它產生的 media
// query 正是 `not (min-width: 840px)`（**不含** 840），所以這裡逐字用同一句，
// navigation-final.css 宣告那兩個變數的區塊也是同一句。
// ⚠️ 不要「順手」改成 `(max-width: 840px)` —— 那是頂部 navbar
// （`Navbar.tsx` 的 `[@media(max-width:840px)]:`）的斷點，**含** 840，兩者差一個
// 像素寬的區間：在正好 840px 時導覽列已 display:none，用它會讓 toast 為一根不存在
// 的導覽列往上讓 90px。三處的對應關係由 tests/mobile-bottom-nav-tokens.test.ts 釘住。
const MOBILE_DOCK_QUERY = "not (min-width: 840px)";

// 桌機：落在浮動 navbar 之下。navbar 佔位 = --floating-nav-height +
// --floating-nav-top-gap（floating-navigation.css 的 `.auto-nav` min-height），
// 再加 12px 呼吸空間；--hk-notif-toast-h 見下方 offset 註解。
const DESKTOP_TOP_OFFSET =
	"calc(var(--floating-nav-height, 84px) + var(--floating-nav-top-gap, 16px) + 12px + var(--hk-notif-toast-h, 0px))";

// 手機：改停畫面底部，讓開底部導覽列（高度 + 離底距離皆為 navigation-final.css
// 既有變數），再加 12px 呼吸空間。
// fallback 值必須與 navigation-final.css 的 `:root` 宣告逐字相同（變數缺席時
// calc 才不會失效），由 tests/mobile-bottom-nav-tokens.test.ts 釘住；那兩個變數
// 本身又由 tests/mobile-bottom-nav-geometry.spec.ts 對實際渲染幾何釘住。
const MOBILE_DOCK_BOTTOM_OFFSET =
	"calc(var(--mobile-bottom-nav-height, 68px) + var(--mobile-bottom-nav-bottom, 10px) + 12px)";

/**
 * Shared Sonner presentation. A scoped `id` lets top-layer surfaces such as a
 * native modal dialog own their notifications without duplicating them in the
 * app-level toaster behind the dialog backdrop.
 */
export default function AppToaster({
	id,
	containerAriaLabel = "系統通知",
}: AppToasterProps) {
	const [Toaster, setToaster] = React.useState<SonnerToaster | null>(null);
	// Sonner arrives over the network. A failed chunk request used to return
	// null forever, which silently disabled EVERY toast in the app — including
	// the error toasts that tell a buyer an action did not happen.
	const [loadFailed, setLoadFailed] = React.useState(false);

	// #374：top-center 的 toast 在手機上幾乎完整遮住整條 navbar 約 4 秒。改停
	// 底部即可完全避開頂部 chrome。只有 app 層 toaster 需要——scoped 實例（帶
	// id）活在 <dialog> 的瀏覽器 top layer，底下的 navbar 本來就看不到，沒有
	// 遮蔽問題，維持 top-center 不動。
	const [dockBottom, setDockBottom] = React.useState(false);

	React.useEffect(() => {
		if (id) return;
		const mql = window.matchMedia(MOBILE_DOCK_QUERY);
		const sync = () => setDockBottom(mql.matches);
		sync();
		mql.addEventListener("change", sync);
		return () => mql.removeEventListener("change", sync);
	}, [id]);

	React.useEffect(() => {
		let active = true;

		// Sonner 2 injects its stylesheet into <head> as soon as the module is
		// evaluated. A static import can therefore add an unexpected <style>
		// while another streamed boundary is still hydrating. Use the shared
		// browser-wide barrier before evaluating the package so every toast entry
		// point follows the same document ownership rule.
		void whenDocumentSettled()
			.then(() => import("sonner"))
			// One retry: the realistic failure is a transient chunk fetch, and
			// a second attempt costs nothing when the first succeeded.
			.catch(() => import("sonner"))
			.then((module) => {
				if (active) setToaster(() => module.Toaster);
			})
			.catch(() => {
				if (active) setLoadFailed(true);
			});

		return () => {
			active = false;
		};
	}, []);

	React.useEffect(() => {
		if (!Toaster) return;
		// Sonner subscribes from its own passive effect. A following paint frame
		// guarantees that subscription exists before the deferred queue flushes.
		const frame = window.requestAnimationFrame(markToastViewportReady);
		return () => window.cancelAnimationFrame(frame);
	}, [Toaster]);

	// Minimal inline fallback (`<output>` carries role="status" natively). The
	// queued messages themselves live in `lib/feedback/toast.ts` and stay
	// queued — nothing is dropped — but the user must at least learn that
	// confirmations are not going to appear. Scoped instances skip it: they
	// live inside a <dialog> that owns its own surface.
	if (!Toaster && loadFailed && !id) {
		return (
			<output
				aria-live="polite"
				className="fixed inset-x-3 bottom-3 z-[var(--z-toast)] mx-auto block w-fit max-w-[calc(100vw-1.5rem)] rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-xs text-hk-body"
			>
				通知元件載入失敗，操作結果可能不會顯示，請重新整理頁面。
			</output>
		);
	}
	if (!Toaster) return null;

	return (
		<Toaster
			id={id}
			className="hk-sonner-toaster"
			position={dockBottom ? "bottom-center" : "top-center"}
			closeButton
			gap={8}
			visibleToasts={4}
			// --hk-notif-toast-h：NotificationToastViewport 寫入的堆疊佔用高度
			// （無通知時為 0px）。同佔 top-center 的通知卡 z 較高會蓋住 sonner，
			// 以 CSS 變數把 sonner 即時下移到通知堆疊下方，兩者同時可見；變數
			// 變化由 CSS 直接重算，不需 re-render。只有 app 層 toaster 需要讓位
			// ——scoped 實例（帶 id，如 DescriptionEditor 的 <dialog>）活在瀏覽器
			// top layer、蓋在通知堆疊之上，跟著下移只會憑空懸空。底部停靠時通知
			// 堆疊仍在頂端，兩者不再相撞，因此不需要這個讓位項。
			// sonner 在 >600px 讀 offset、≤600px 讀 mobileOffset，而底部停靠的斷點
			// 是 840px，中間區段（601~840px）走 offset —— 兩份都要給底部值。
			offset={
				dockBottom
					? { bottom: MOBILE_DOCK_BOTTOM_OFFSET }
					: {
							top: id ? 12 : DESKTOP_TOP_OFFSET,
						}
			}
			mobileOffset={
				dockBottom
					? { bottom: MOBILE_DOCK_BOTTOM_OFFSET, left: 12, right: 12 }
					: {
							// 只有 scoped 實例會走到這裡（app 層 ≤840px 一律底部停靠，
							// 而 mobileOffset 只在 ≤600px 生效）。
							top: 8,
							left: 12,
							right: 12,
						}
			}
			swipeDirections={
				dockBottom ? ["bottom", "left", "right"] : ["top", "left", "right"]
			}
			containerAriaLabel={containerAriaLabel}
			icons={{
				success: <ToastIcon kind="success" />,
				error: <ToastIcon kind="error" />,
				warning: <ToastIcon kind="warning" />,
				info: <ToastIcon kind="info" />,
				loading: <ToastIcon kind="loading" />,
			}}
			toastOptions={{
				closeButtonAriaLabel: "關閉通知",
				classNames: {
					toast: "hk-sonner-toast",
					content: "hk-sonner-toast__content",
					title: "hk-sonner-toast__title",
					description: "hk-sonner-toast__description",
					icon: "hk-sonner-toast__icon",
					closeButton: "hk-sonner-toast__close",
					actionButton: "hk-sonner-toast__action",
					cancelButton: "hk-sonner-toast__cancel",
				},
			}}
		/>
	);
}
