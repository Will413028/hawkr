import { useEffect, useRef } from "react";

/**
 * 網址帶錨點（`#inquiry-7`）而目標在資料載入後才渲染的頁面：瀏覽器在載入當下找不到元素，不會自己捲過去。
 * `ready` 變成 true（目標已渲染）時，若 `location.hash` 對應的元素存在就捲到它；整個頁面生命週期只做一次，
 * 之後的背景重讀不會把使用者拉回去。
 */
export function useScrollToHash(ready: boolean): void {
	const done = useRef(false);
	useEffect(() => {
		if (!ready || done.current) return;
		done.current = true;
		const raw = window.location.hash.slice(1);
		if (!raw) return;
		let id = raw;
		try {
			id = decodeURIComponent(raw);
		} catch {
			// 不合法的編碼照原字串找。
		}
		document.getElementById(id)?.scrollIntoView({ block: "start" });
	}, [ready]);
}
