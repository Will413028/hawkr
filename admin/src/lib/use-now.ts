import { useEffect, useState } from "react";

/** 清單每 30 秒自動更新，相對時間也跟著走。 */
export const LIST_REFRESH_MS = 30 * 1000;

/** 每 `intervalMs` 毫秒更新一次的現在時間：清單的相對時間（剩 N 分、已逾期 N 分）跟著走，不必等資料重讀。 */
export function useNow(intervalMs: number): Date {
	const [now, setNow] = useState(() => new Date());
	useEffect(() => {
		const timer = setInterval(() => setNow(new Date()), intervalMs);
		return () => clearInterval(timer);
	}, [intervalMs]);
	return now;
}
