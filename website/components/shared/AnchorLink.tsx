"use client";

import type * as React from "react";
import { prefersReducedMotion } from "@/lib/a11y/reduced-motion";

/**
 * 頁內錨點連結（href="#section"）。
 *
 * 全域 `html { scroll-behavior: smooth }` 已移除（它讓所有程式化捲動 —
 * 導航捲頂、列表捲動還原、modal 歸位 — 都變成滑行，互相拉扯造成抖動），
 * 頁內錨點改在這裡自行 opt-in 平滑捲動。JS 的 scrollIntoView 不會自動
 * 尊重 prefers-reduced-motion（CSS scroll-behavior 才會），所以這裡手動判斷。
 *
 * 捲動之後焦點也一起搬過去：只捲視窗的話，鍵盤與讀屏使用者還留在連結上，
 * 下一次 Tab 會回到頁首而不是剛剛跳到的段落——等於這個連結對他們沒有作用。
 */
export default function AnchorLink({
	href,
	children,
	onClick,
	...rest
}: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) {
	return (
		// `rest` 先展開：href / onClick 是本元件的契約，呼叫端傳同名 prop 時
		// 應該被「合成」而不是「取代」——原本的順序會讓一個 onClick 整段換掉
		// 平滑捲動與 reduced-motion 判斷。
		<a
			{...rest}
			href={href}
			onClick={(event) => {
				onClick?.(event);
				if (event.defaultPrevented || !href.startsWith("#")) return;
				const target = document.getElementById(href.slice(1));
				if (!target) return;
				event.preventDefault();
				target.scrollIntoView({
					behavior: prefersReducedMotion() ? "auto" : "smooth",
					block: "start",
				});
				// 區塊本身多半不可聚焦；給它一個程式化的落腳點，用完即拿掉，
				// 這樣它不會出現在 Tab 順序裡，只是接住這一次跳轉。
				if (!target.hasAttribute("tabindex")) {
					target.setAttribute("tabindex", "-1");
					target.addEventListener(
						"blur",
						() => target.removeAttribute("tabindex"),
						{ once: true },
					);
				}
				target.focus({ preventScroll: true });
				history.replaceState(null, "", href);
			}}
		>
			{children}
		</a>
	);
}
