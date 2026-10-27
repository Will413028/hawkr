import {
	createContext,
	type KeyboardEvent as ReactKeyboardEvent,
	type RefObject,
	useCallback,
	useContext,
	useEffect,
	useRef,
} from "react";

/**
 * 回傳 onKeyDown handler:Enter / Space 觸發傳入的 click handler。
 *
 * 搭配 role="button" + tabIndex={0} 讓 div / span 等非互動元素具鍵盤可操作性,
 * 滿足 a11y/useKeyWithClickEvents 與 a11y/noStaticElementInteractions。
 *
 * @example
 * <div role="button" tabIndex={0} onClick={open} onKeyDown={onActivateKey(open)} />
 */
export function onActivateKey(handler: () => void) {
	return (event: ReactKeyboardEvent) => {
		if (event.key === "Enter" || event.key === " ") {
			event.preventDefault();
			handler();
		}
	};
}

const alwaysActive = () => true;

/**
 * modal / dialog 開啟期間,按 Escape 呼叫 onEscape 關閉。
 *
 * 這是 dialog 的標準鍵盤關閉方式(WAI-ARIA APG),取代「點遮罩關閉」在 a11y 上
 * 需要的處理;搭配 role="dialog" + aria-modal="true" 即為無障礙 modal 的核心。
 *
 * @param onEscape 關閉 modal 的 callback
 * @param enabled  是否啟用(條件渲染的 modal 可傳 isOpen;掛載即開啟者用預設 true)
 * @param isActive 疊著開多個 modal 時,只有回傳 true 的那個處理 Escape(見 `useModalStack` 的 `isTopmost`);省略就一律處理
 */
export function useEscapeKey(
	onEscape: () => void,
	enabled = true,
	isActive: () => boolean = alwaysActive,
) {
	const cb = useRef(onEscape);
	cb.current = onEscape;
	const active = useRef(isActive);
	active.current = isActive;
	useEffect(() => {
		if (!enabled) return;
		const handler = (event: KeyboardEvent) => {
			if (event.key === "Escape" && active.current()) cb.current();
		};
		document.addEventListener("keydown", handler);
		return () => document.removeEventListener("keydown", handler);
	}, [enabled]);
}

/**
 * Modal / dialog 的 focus 管理:開啟時聚焦第一個可聚焦元素、Tab / Shift+Tab 在 dialog
 * 內循環(focus trap)、關閉時還焦回原本聚焦的觸發元素。搭配 role="dialog" +
 * aria-modal="true" + useEscapeKey 即為完整的無障礙 modal。
 *
 * dialog 容器(modal-content)需設 tabIndex={-1},作為無可聚焦子元素時的後備焦點。
 *
 * @param ref     指向 dialog 容器的 ref
 * @param enabled 是否啟用(條件渲染的 modal 傳 isOpen;掛載即開啟者用預設 true)
 * @param isActive 疊著開多個 modal 時,只有回傳 true 的那個做 Tab 循環(見 `useModalStack` 的 `isTopmost`);省略就一律處理
 */
export function useFocusTrap<T extends HTMLElement>(
	ref: RefObject<T | null>,
	enabled = true,
	isActive: () => boolean = alwaysActive,
) {
	const active = useRef(isActive);
	active.current = isActive;
	const returnFocusRef = useRef<HTMLElement | null>(null);
	const effectGenerationRef = useRef(0);

	useEffect(() => {
		if (!enabled) return;
		const container = ref.current;
		if (!container) return;
		const effectGeneration = ++effectGenerationRef.current;
		const activeElement = document.activeElement as HTMLElement | null;
		if (activeElement && !container.contains(activeElement)) {
			returnFocusRef.current = activeElement;
		}
		const selector =
			'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';
		const getItems = () =>
			Array.from(container.querySelectorAll<HTMLElement>(selector)).filter(
				(el) => el.offsetParent !== null,
			);
		(getItems()[0] ?? container).focus();
		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key !== "Tab" || !active.current()) return;
			const items = getItems();
			if (items.length === 0) {
				event.preventDefault();
				return;
			}
			const first = items[0];
			const last = items[items.length - 1];
			if (event.shiftKey && document.activeElement === first) {
				event.preventDefault();
				last.focus();
			} else if (!event.shiftKey && document.activeElement === last) {
				event.preventDefault();
				first.focus();
			}
		};
		container.addEventListener("keydown", handleKeyDown);
		return () => {
			container.removeEventListener("keydown", handleKeyDown);
			// Modal isolation may still have the trigger inside an inert subtree while
			// sibling effects are cleaning up. Restore focus after those synchronous
			// cleanups complete so browsers do not discard the focus request. The
			// generation guard also ignores React Strict Mode's setup/cleanup replay.
			queueMicrotask(() => {
				if (effectGenerationRef.current !== effectGeneration) return;
				const returnFocus = returnFocusRef.current;
				if (
					returnFocus &&
					document.contains(returnFocus) &&
					!returnFocus.inert
				) {
					returnFocus.focus();
				}
			});
		};
	}, [enabled, ref]);
}

/** 帶這個屬性的元素（例如 toast 區）不被 modal 隔離，開著 modal 時它的內容仍要被輔助科技讀到。 */
export const MODAL_EXEMPT_ATTRIBUTE = "data-modal-exempt";

interface SavedState {
	inert: boolean;
	ariaHidden: string | null;
}

interface ModalEntry {
	id: object;
	/** 巢狀時外層 modal 的 id（由 `ModalParentContext` 提供）；沒有外層就是 null。 */
	parent: object | null;
	overlay: HTMLElement;
}

/**
 * 開著的 modal，最後一個是最上層。**整個後台唯一的一份堆疊**：Escape／Tab 的「誰是最上層」與背景隔離都從它推導。
 * 模組層級，因為 modal 之間沒有共同的 React 祖先（都 portal 到 body）。
 */
const modalStack: ModalEntry[] = [];
/** 隔離期間被動過的元素與它們「本來」的狀態；只在第一次碰到時記錄，全部關閉時才清空。 */
const originals = new Map<HTMLElement, SavedState>();
let savedBodyOverflow: string | null = null;

const modalListeners = new Set<() => void>();

/** 堆疊變動時通知（給需要跟著最上層 modal 走的元件，例如 toast 的 live region）。 */
export function subscribeModalStack(listener: () => void): () => void {
	modalListeners.add(listener);
	return () => modalListeners.delete(listener);
}

/** 目前最上層 modal 的 overlay；沒有開著的 modal 時是 null。 */
export function getTopModalOverlay(): HTMLElement | null {
	return modalStack[modalStack.length - 1]?.overlay ?? null;
}

function notifyModalStack() {
	for (const listener of modalListeners) listener();
}

/** 巢狀 modal 的外層 id。Dialog 把自己的 id 放進 provider，裡面再開的 modal 就知道誰是它的外層。 */
export const ModalParentContext = createContext<object | null>(null);

function isDescendantOf(entry: ModalEntry, ancestor: object): boolean {
	let parent: object | null = entry.parent;
	while (parent) {
		if (parent === ancestor) return true;
		parent = modalStack.find((e) => e.id === parent)?.parent ?? null;
	}
	return false;
}

/**
 * 登記順序 = 開啟順序，但同一個 commit 內巢狀掛載時 React 先跑子層的 effect：
 * 外層晚登記，卻必須排在自己的後代之下。所以外層插在「最早的後代」前面，沒有後代才 push。
 */
function pushModal(entry: ModalEntry) {
	const firstDescendant = modalStack.findIndex((e) =>
		isDescendantOf(e, entry.id),
	);
	if (firstDescendant >= 0) modalStack.splice(firstDescendant, 0, entry);
	else modalStack.push(entry);
}

function restoreIsolated() {
	for (const [element, previous] of originals) {
		element.inert = previous.inert;
		if (previous.ariaHidden === null) element.removeAttribute("aria-hidden");
		else element.setAttribute("aria-hidden", previous.ariaHidden);
	}
	originals.clear();
}

/**
 * 隔離狀態由「現在開著哪些 modal」推導，不由各 modal 各存一份「前一個值」：
 * 先把所有被動過的元素還原成本來的樣子，再只對最上層的 overlay 重新套用。
 * 所以任意順序關閉、同一個 commit 一起卸載，結果都一樣；全部關閉時 inert、aria-hidden、body 捲動都回到原狀。
 */
function applyIsolation() {
	restoreIsolated();
	const top = modalStack[modalStack.length - 1];
	if (!top) {
		if (savedBodyOverflow !== null) {
			document.body.style.overflow = savedBodyOverflow;
			savedBodyOverflow = null;
		}
		return;
	}
	if (savedBodyOverflow === null) {
		savedBodyOverflow = document.body.style.overflow;
	}
	document.body.style.overflow = "hidden";

	let current: HTMLElement = top.overlay;
	let parent = current.parentElement;
	while (parent) {
		for (const sibling of Array.from(parent.children)) {
			if (!(sibling instanceof HTMLElement) || sibling === current) continue;
			if (sibling.hasAttribute(MODAL_EXEMPT_ATTRIBUTE)) continue;
			if (!originals.has(sibling)) {
				originals.set(sibling, {
					inert: sibling.inert,
					ariaHidden: sibling.getAttribute("aria-hidden"),
				});
			}
			sibling.inert = true;
			sibling.setAttribute("aria-hidden", "true");
		}
		if (parent === document.body) break;
		current = parent;
		parent = parent.parentElement;
	}
}

/**
 * 登記這個 modal 進堆疊：回傳「我現在是不是最上層」與自己的 id（巢狀時放進 `ModalParentContext`）。
 *
 * - 疊著開多個 modal 時，只有最上層處理 Escape 與 Tab 循環（把 `isTopmost` 傳給 `useEscapeKey`／`useFocusTrap`），
 *   下層保持沉默，直到上層卸載。
 * - 同時負責背景隔離：從最上層 overlay 一路往 body 把每一層 siblings（含下層的 modal）設為 inert 與 aria-hidden，
 *   並鎖住 body 捲動。`aria-modal` 只描述 dialog，並不會真的擋住輔助科技或滑鼠操作背景內容。
 *   帶 `data-modal-exempt` 的 sibling 不隔離（toast 區）。
 *
 * 必須在同一個元件的 `useEscapeKey`／`useFocusTrap` 之前呼叫。
 */
export function useModalStack<T extends HTMLElement>(
	overlayRef: RefObject<T | null>,
	enabled = true,
): { isTopmost: () => boolean; id: object } {
	const idRef = useRef<object>({});
	const parent = useContext(ModalParentContext);
	useEffect(() => {
		if (!enabled) return;
		const overlay = overlayRef.current;
		if (!overlay) return;
		const entry: ModalEntry = { id: idRef.current, parent, overlay };
		pushModal(entry);
		applyIsolation();
		notifyModalStack();
		return () => {
			const index = modalStack.indexOf(entry);
			if (index >= 0) modalStack.splice(index, 1);
			applyIsolation();
			notifyModalStack();
		};
	}, [enabled, overlayRef, parent]);
	const isTopmost = useCallback(
		() => modalStack[modalStack.length - 1]?.id === idRef.current,
		[],
	);
	return { isTopmost, id: idRef.current };
}
