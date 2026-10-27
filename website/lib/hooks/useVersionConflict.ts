"use client";

import * as React from "react";

/** 讀不到最新版本時的提示（衝突之後重新載入失敗）。 */
export const VERSION_CONFLICT_RELOAD_FAILED =
	"無法載入最新版本，請重新整理後再試。";

/** 文字型的衝突提示：保留輸入、已載入最新版本，請確認後再送出。 */
export function versionConflictNotice(
	subject: string,
	action = "儲存",
): string {
	return `${subject}已在其他地方更新，已載入最新版本，你輸入的內容仍保留，請確認後再${action}。`;
}

interface Options<T> {
	/** 讀最新版本。 */
	reload: () => Promise<T>;
	/**
	 * 採用最新版本：換 baseline、之後的寫入用新的 etag，使用者改過的欄位留著。回傳字串時，通知用
	 * 它取代預設的提示（例如最新版本是「資源已不存在」）。
	 */
	adopt: (latest: T) => string | undefined;
	/** 顯示訊息給使用者。 */
	notify: (message: string) => void;
	/** 預設的提示。 */
	notice: string;
}

/**
 * 帶 etag 的寫入撞到版本衝突（VERSION_CONFLICT）時的處理（API 設計規範（Connect）§七、K6）：重新載入、
 * 採用最新版本、通知使用者；輸入不重設。回傳的函式在載入失敗時通知 `VERSION_CONFLICT_RELOAD_FAILED`，
 * 回 false。
 */
export function useVersionConflict<T>(
	options: Options<T>,
): (notice?: string) => Promise<boolean> {
	const latest = React.useRef(options);
	latest.current = options;
	return React.useCallback(async (notice?: string) => {
		const { reload, adopt, notify, notice: defaultNotice } = latest.current;
		let fresh: T;
		try {
			fresh = await reload();
		} catch {
			notify(VERSION_CONFLICT_RELOAD_FAILED);
			return false;
		}
		notify(adopt(fresh) ?? notice ?? defaultNotice);
		return true;
	}, []);
}
