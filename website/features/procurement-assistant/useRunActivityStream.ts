"use client";

/**
 * 候選搜尋 run 的 Activity 訂閱（`WatchRun`）。
 *
 * 掛在 AssistantWorkspace 而不是 AgentTaskPanel：後者是主區的處理展示元件，
 * 網路訂閱放進去會讓「誰擁有這條連線」從擁有 runId／stage 的那一層看不見。
 * 也不放 useProcurementSession：搜尋列的生命週期屬 run 不屬 turn。
 *
 * 回傳的是這個 run 目前為止的列，已按 rowId 收斂（後端開跑時先送整份 PENDING
 * 清單，之後每一筆都是就地更新某一列的狀態）。接續與去重由 `watchRun` 負責。
 */

import * as React from "react";
import { buyerProcurementStreamClient } from "@/lib/connect/procurement";
import { type AgentActivityEvent, mergeLiveActivity } from "./activity";
import { activityFromProto } from "./proto-map";
import { watchRun } from "./watch-run";

export function useRunActivityStream({
	runId,
	enabled,
}: {
	runId: string | null;
	enabled: boolean;
}): AgentActivityEvent[] {
	const [activity, setActivity] = React.useState<AgentActivityEvent[]>([]);

	React.useEffect(() => {
		setActivity([]);
		if (!enabled || !runId) return;
		const controller = new AbortController();
		const client = buyerProcurementStreamClient();
		void watchRun({
			open: (request, signal) => client.watchRun(request, { signal }),
			runId,
			signal: controller.signal,
			onEvent: (event) => {
				if (event.event.case !== "activity") return;
				const row = activityFromProto(event.event.value);
				if (row) setActivity((rows) => mergeLiveActivity(rows, row));
			},
		});
		return () => controller.abort();
	}, [runId, enabled]);

	return activity;
}
