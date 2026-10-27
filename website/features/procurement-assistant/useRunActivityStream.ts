"use client";

/**
 * 候選搜尋 run 的 Activity 訂閱。
 *
 * 掛在 AssistantWorkspace 而不是 AgentTaskPanel：後者是主區的處理展示元件，
 * 網路訂閱放進去會讓「誰擁有這條連線」從擁有 runId／stage 的那一層看不見。
 * 也不放 useProcurementSession——那支 2490 行、liveActivity 有八個 reset 點，
 * 而搜尋列的生命週期屬 run 不屬 turn。
 *
 * 回傳的是這個 run 目前為止的列，已按 rowId 收斂（後端開跑時先送整份 PENDING
 * 清單，之後每一筆都是就地更新某一列的狀態）。
 */

import * as React from "react";
import { type AgentActivityEvent, mergeLiveActivity } from "./activity";
import { openRunActivityStream } from "./run-events-stream";

export function useRunActivityStream({
	runId,
	enabled,
}: {
	runId: string | null;
	enabled: boolean;
}): AgentActivityEvent[] {
	const [activity, setActivity] = React.useState<AgentActivityEvent[]>([]);

	React.useEffect(() => {
		// 換 run（或關閉）先清空：上一次搜尋的列不得混進這一次。
		setActivity([]);
		if (!enabled || !runId) return;
		const stream = openRunActivityStream(runId, {
			onActivity: (event) =>
				setActivity((rows) => mergeLiveActivity(rows, event)),
			// 終態就把連線收掉——伺服器會一直開著到 120 秒 deadline 為止。
			// 已收到的列暫留 state，離開搜尋階段後由 enabled 關閉並清空；
			// AgentTaskPanel 也會立即移除完成後的處理 surface，不保留快照。
			onTerminal: () => stream.close(),
			// 拿不到列時什麼都不做：畫面的降級判斷是「有沒有列」，
			// 不需要第二個旗標（run-events-stream 已在非 prod 記了一行）。
			onUnavailable: () => {},
		});
		return () => stream.close();
	}, [runId, enabled]);

	return activity;
}
