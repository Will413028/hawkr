import {
	formatTaipeiMonthDayTime,
	formatTaipeiTime,
	taipeiParts,
} from "@/lib/date/taipei";
import type { ProcurementChatTurn } from "./types";

/**
 * 對話紀錄的時間分組（iMessage 式）。
 *
 * 每一則訊息不再自己掛時間戳——那讓每個氣泡都多長一截、右下角永遠有一塊視覺
 * 噪音。改成只在「這一則跟上一則隔得夠久」時，於兩者之間插一條置中的時間分隔
 * 線；同一批連續對話共用一個時間。
 *
 * 分組門檻取 15 分鐘。採購對話的節奏是一問一答，秒級間隔佔絕大多數；門檻太小
 * 會讓分隔線變成另一種每則都有的噪音，太大則會讓「隔天回來繼續談」看起來像同
 * 一批。跨日一律另起一組，不管間隔多短——「昨天 23:58 / 今天 00:03」中間沒有
 * 分隔線的話，日期就完全讀不出來了。
 */
export const TRANSCRIPT_GROUP_GAP_MS = 15 * 60 * 1000;

/**
 * 這一則要不要在它前面插分隔線。
 *
 * `createdAt` 為 0 ＝ 還原舊對話時後端沒給時間（見 ProcurementChatTurn）。那種
 * 訊息不插分隔線也不當作分組基準，否則會印出由 epoch 0 推出來的「1970/01/01」。
 */
export function shouldStartGroup(
	turn: ProcurementChatTurn,
	previous: ProcurementChatTurn | undefined,
): boolean {
	if (!turn.createdAt) return false;
	if (!previous?.createdAt) return true;
	if (turn.createdAt - previous.createdAt >= TRANSCRIPT_GROUP_GAP_MS) {
		return true;
	}
	const current = taipeiParts(turn.createdAt);
	const before = taipeiParts(previous.createdAt);
	if (!current || !before) return false;
	return (
		current.year !== before.year ||
		current.month !== before.month ||
		current.day !== before.day
	);
}

/**
 * 分隔線文字。今天只印時間（「07:30」），其他日子連日期一起印
 *（「8/17 07:30」）——同一天的對話裡重複印日期是多餘的。
 *
 * 一律走 taipeiParts／formatTaipei*，不用 Date 的 getFullYear 家族：那些方法讀
 * 的是執行環境的時區，CI 與本機不同會讓同一筆資料印出不同日期。
 */
export function groupLabel(createdAt: number, now: number = createdAt): string {
	const turnDay = taipeiParts(createdAt);
	const today = taipeiParts(now);
	if (!turnDay || !today) return "";
	const sameDay =
		turnDay.year === today.year &&
		turnDay.month === today.month &&
		turnDay.day === today.day;
	return sameDay
		? formatTaipeiTime(createdAt)
		: formatTaipeiMonthDayTime(createdAt);
}
