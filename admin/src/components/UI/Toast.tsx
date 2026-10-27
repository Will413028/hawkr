"use client";

import {
	createContext,
	type ReactNode,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useRef,
	useState,
	useSyncExternalStore,
} from "react";
import { createPortal } from "react-dom";
import {
	getTopModalOverlay,
	MODAL_EXEMPT_ATTRIBUTE,
	subscribeModalStack,
} from "@/lib/a11y";
import "./Toast.css";
import { ToastRegions } from "./ToastRegions";

/**
 * 後台唯一的 toast 機制：`<ToastProvider>`（在 `src/app/admin/layout.tsx`）加 `useToast()`。
 * 頁面不得自己放 toast 狀態、計時器或配色。
 *
 * 色調只有 `success`／`warning`／`error`，對應 `--admin-success`／`--admin-warning`／`--admin-danger`（`Toast.css`），
 * 呼叫端不傳顏色。`warning` 用在「請求成功但結果不對勁」（例如公告送出了卻沒有收件人）：不是錯誤，但不能跟成功長得一樣。`success` 用 `role="status"`（polite），`warning` 與 `error` 用 `role="alert"`（立即宣告）。
 *
 * ## 對話框開著時怎麼宣告
 *
 * 對話框開著時頁面其餘內容是 inert 與 aria-hidden，而 `aria-modal` 的對話框之外的內容，
 * 瀏覽器也可能不放進無障礙樹（WAI-ARIA 1.2 `aria-modal`：對話框之外的內容視為不可互動；
 * APG Dialog (Modal) Pattern 同樣要求只有對話框內的內容可操作與可讀）。
 * 所以 live region 不能放在頁面或 body：最上層的 Dialog 在面板裡預先放好（空的）live region
 * （`ToastRegions`），toast 開著對話框時 portal 進**最上層**對話框的 region；沒有對話框時進 body 的 region
 * （帶 `data-modal-exempt`）。region 一律先存在、再塞內容，宣告才可靠（內容插入已存在的 live region）。
 * toast 顯示中對話框開關，內容會換到另一個 region 重新宣告一次，這是接受的。
 */

export type ToastTone = "success" | "warning" | "error";

const TOAST_MS = 3000;

interface ToastApi {
	showToast: (message: string, tone?: ToastTone) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

export function useToast(): ToastApi {
	const api = useContext(ToastContext);
	if (!api) throw new Error("useToast 必須在 <ToastProvider> 底下使用");
	return api;
}

const regionName = (tone: ToastTone) =>
	tone === "success" ? "status" : "alert";

export function ToastProvider({ children }: { children: ReactNode }) {
	const [toast, setToast] = useState<{
		message: string;
		tone: ToastTone;
	} | null>(null);
	const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	const [bodyHost, setBodyHost] = useState<HTMLElement | null>(null);
	const topOverlay = useSyncExternalStore(
		subscribeModalStack,
		getTopModalOverlay,
		() => null,
	);

	useEffect(() => {
		const host = document.createElement("div");
		// body 的直接子項：帶 exempt，否則有 modal（含手機導覽抽屜）開著時整個被 inert 與 aria-hidden。
		host.setAttribute(MODAL_EXEMPT_ATTRIBUTE, "");
		document.body.appendChild(host);
		setBodyHost(host);
		return () => {
			clearTimeout(timer.current);
			host.remove();
		};
	}, []);

	const showToast = useCallback(
		(message: string, tone: ToastTone = "success") => {
			setToast({ message, tone });
			// 新的 toast 取代舊的，舊的計時器不能把新的關掉。
			clearTimeout(timer.current);
			timer.current = setTimeout(() => setToast(null), TOAST_MS);
		},
		[],
	);
	const api = useMemo(() => ({ showToast }), [showToast]);

	let target: Element | null = null;
	if (toast) {
		const name = regionName(toast.tone);
		target =
			topOverlay?.querySelector(`[data-toast-region="${name}"]`) ??
			bodyHost?.querySelector(`[data-toast-region="${name}"]`) ??
			null;
	}

	return (
		<ToastContext.Provider value={api}>
			{children}
			{bodyHost && createPortal(<ToastRegions />, bodyHost)}
			{toast &&
				target &&
				createPortal(
					<div className={`adm-toast adm-toast--${toast.tone}`}>
						{toast.message}
					</div>,
					target,
				)}
		</ToastContext.Provider>
	);
}
