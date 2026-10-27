/**
 * 把 overlay 背後的頁面主體標成 `inert`。
 *
 * `aria-hidden` 只擋讀屏，擋不住 Tab；`inert` 兩者都擋，是全螢幕 modal 唯一
 * 正確的隔離方式（Dialog.tsx 的 applyOverlayIsolation 用的就是這一招）。這裡
 * 抽成共用工具，讓 Dialog 以外的全螢幕 overlay（DatePicker 手機版 modal 等）
 * 不必各自複製一份，也不會忘記還原。
 *
 * 巢狀使用是安全的：release 還原的是「呼叫當下的舊值」，所以在 Dialog 裡再開
 * 一個 modal，內層關閉時會把 inert 還原成 true（Dialog 仍然開著），而不是誤把
 * 背景解鎖。
 */

/** 需要被隔離的頁面根節點。與 Dialog.tsx 的 overlayIsolationRoots 同一組。 */
function isolationRoots(): HTMLElement[] {
	return Array.from(document.querySelectorAll<HTMLElement>(".page-wrapper"));
}

/**
 * 將 `.page-wrapper`（overlay 以外的頁面主體）設為 inert，回傳還原函式。
 *
 * @param exclude 若 overlay 本身就在某個 root 內（沒有 portal 出去），傳它進來
 *                以免把自己也一起 inert 掉。
 */
export function setInertOutside(exclude?: HTMLElement | null): () => void {
	if (typeof document === "undefined") return () => {};

	const previous: Array<[HTMLElement, boolean]> = [];
	for (const root of isolationRoots()) {
		if (exclude && root.contains(exclude)) continue;
		previous.push([root, root.inert]);
		root.inert = true;
	}

	let released = false;
	return () => {
		if (released) return;
		released = true;
		for (const [root, state] of previous) {
			root.inert = state;
		}
	};
}
