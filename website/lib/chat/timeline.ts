/**
 * 聊天時間線的排序與去重（純函式，不碰 React 與網路）。
 *
 * 訊息以對話內的 `seq`（從 1 起連號、沒有空洞）排序，不看時間：斷線補洞後亂序到達的訊息會落在正確的位置。
 * 還沒有 `seq` 的樂觀訊息（剛送出、伺服器還沒回）固定在最下面，依送出時間排；伺服器回應後換成真正的訊息。
 */

import type { ChatMessage } from "./types";

/** 全序：有 seq 的在前（依 seq），樂觀訊息在後（依送出時間）。 */
export function compareMessages(a: ChatMessage, b: ChatMessage): number {
	if (a.seq !== null && b.seq !== null) return a.seq - b.seq;
	if (a.seq !== null) return -1;
	if (b.seq !== null) return 1;
	return (a.timeMs || 0) - (b.timeMs || 0);
}

export function sortTimeline(messages: ChatMessage[]): ChatMessage[] {
	return [...messages].sort(compareMessages);
}

/**
 * 放進一則伺服器的訊息：同 seq 的取代（可見度改變時，墓碑或還原；`MessageEvent.updated`），
 * 同 id 的取代（送出回應與串流的回聲是同一則），否則加入。回新陣列。
 */
export function upsertServerMessage(
	current: ChatMessage[] | undefined,
	next: ChatMessage,
): ChatMessage[] {
	const list = current ?? [];
	const i = list.findIndex(
		(m) => m.id === next.id || (next.seq !== null && m.seq === next.seq),
	);
	if (i < 0) return sortTimeline([...list, next]);
	const merged = [...list];
	merged[i] = next;
	return sortTimeline(merged);
}

/**
 * 送出回應：把樂觀訊息換成伺服器的訊息。串流的回聲先到時，同 seq 已經在清單裡，只丟掉樂觀的那則。
 */
export function reconcileOptimistic(
	current: ChatMessage[] | undefined,
	optimisticId: string,
	server: ChatMessage,
): ChatMessage[] {
	const without = (current ?? []).filter((m) => m.id !== optimisticId);
	return upsertServerMessage(without, server);
}

export function lastSeq(messages: ChatMessage[]): number {
	let max = 0;
	for (const m of messages) if (m.seq !== null && m.seq > max) max = m.seq;
	return max;
}

/**
 * 收到 `incoming` 時是否漏了訊息：它的 seq 大於手上最後一則加一。回要補的起點（`ListMessages` 的 `after_seq`）；
 * 沒有缺口回 null。手上沒有任何訊息時不判斷（整頁會重讀）。
 */
export function gapAfter(
	messages: ChatMessage[],
	incomingSeq: number,
): number | null {
	const last = lastSeq(messages);
	if (last === 0) return null;
	return incomingSeq > last + 1 ? last : null;
}
