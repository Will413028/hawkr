/**
 * 共用的可聚焦元素工具。所有 overlay（Dialog / Sheet / 各種 modal / popover）
 * 的 focus trap 都應該從這裡拿 selector 與過濾邏輯，而不是各自複製一份：
 * 過去各 overlay 各有一份，其中只有 Dialog
 * 會過濾掉 display:none / 零尺寸的元素，其餘幾份會被隱藏的可聚焦元素困住 Tab。
 */

export const FOCUSABLE_SELECTOR = [
	"a[href]",
	"button:not([disabled])",
	"input:not([disabled])",
	"select:not([disabled])",
	"textarea:not([disabled])",
	'[tabindex]:not([tabindex="-1"])',
	'[contenteditable="true"]',
].join(",");

/**
 * 元素是否真的被畫出來。瀏覽器裡以 getClientRects 為準（display:none 與
 * 祖先 display:none 都會是 0 個 rect）。jsdom 沒有 layout，所有元素的 rect 都是
 * 0 個，若照瀏覽器邏輯就會把整個 overlay 判成「沒有可聚焦元素」，讓 overlay 的
 * vitest 測試的 focus trap 全部失效；jsdom 下退回看 computed style。
 */
function isRendered(element: HTMLElement): boolean {
	if (element.hidden || element.closest("[hidden]")) return false;
	if (element.getClientRects().length > 0) return true;
	if (typeof navigator !== "undefined" && /jsdom/i.test(navigator.userAgent)) {
		const style = window.getComputedStyle(element);
		return style.display !== "none" && style.visibility !== "hidden";
	}
	return false;
}

/** 回傳 root 內目前「真的可以被 Tab 到」的元素（可見、不在 inert / aria-hidden 子樹內）。 */
export function getFocusableElements(root: HTMLElement): HTMLElement[] {
	return Array.from(
		root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
	).filter(
		(element) =>
			isRendered(element) && !element.closest('[inert], [aria-hidden="true"]'),
	);
}

/**
 * 在 root 內把 Tab / Shift+Tab 首尾相接。呼叫端在 keydown handler 裡先確認
 * `event.key === "Tab"` 再呼叫；回傳 true 表示已處理（並已 preventDefault）。
 *
 * 焦點若不在 root 內（例如落在巢狀 portal 的 listbox），不搶焦點、回傳 false，
 * 讓內層 overlay 自己處理，避免 Dialog 把 Dropdown 選單的焦點拉回第一個元素。
 */
export function wrapTabFocus(
	event: KeyboardEvent | React.KeyboardEvent,
	root: HTMLElement,
): boolean {
	const focusable = getFocusableElements(root);
	const active = document.activeElement as HTMLElement | null;
	if (focusable.length === 0) {
		event.preventDefault();
		root.focus({ preventScroll: true });
		return true;
	}
	const inside = active ? root.contains(active) : false;
	if (active && !inside && active !== document.body) return false;
	const index = active ? focusable.indexOf(active) : -1;
	if (event.shiftKey && (index <= 0 || !inside)) {
		event.preventDefault();
		focusable.at(-1)?.focus();
		return true;
	}
	if (!event.shiftKey && (index === focusable.length - 1 || !inside)) {
		event.preventDefault();
		focusable[0]?.focus();
		return true;
	}
	return false;
}
