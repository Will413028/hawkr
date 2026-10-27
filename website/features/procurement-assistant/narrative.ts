import type { AppIconName } from "@/components/shared/AppIcon";

/**
 * Public Narrative（設計書 v6 §V.10.6-4、§V.11.1 Message source、§V.11.1A）。
 *
 * 這是 v6 對「客製化思考過程」的兩個安全表面之一：Live Activity Rail 只存在於
 * 進行中的那一輪，Public Narrative 才是可保存、使用者可讀的正文。兩者都**不是**
 * provider reasoning token、ADK part.Thought 或模型原始 Chain-of-Thought。
 *
 * 三條規則直接決定這個模組的形狀：
 *
 * 1. 欄位只有 kind、text 與可選 evidenceRefs——沒有 icon 欄位。icon 由 kind 在
 *    前端查表決定，因為 §V.10.6-4 明文禁止「由模型傳入任意 SVG、URL 或 icon
 *    name」。模型能選的只有 allowlisted 的語意種類。
 * 2. icon 只標示公開語意，不是 status truth。旁邊的文字與 canonical event 才是
 *    權威（§V.10.10-5 同一原則）。
 * 3. 「舊 reply string fallback 只轉成單一 SUMMARY block」——#2549 Phase 2 起
 *    後端已會送真正的 typed `blocks[]`，這裡優先消費；`legacyReplyToBlocks`
 *    降級為 fallback，只服務 pre-Phase-2 回應或舊資料列重播，不得自己把字串
 *    切成好幾個 block 再猜語意。
 */

/** §F（issue #2549）的 6 個 block kind，外加只給 legacy fallback 用的 SUMMARY。 */
export type PublicNarrativeKind =
	| "UNDERSTOOD"
	| "CONTEXT"
	| "RATIONALE"
	| "WARNING"
	| "NEXT_STEP"
	| "COMPLETE"
	| "SUMMARY";

export interface PublicNarrativeBlock {
	kind: PublicNarrativeKind;
	text: string;
}

/**
 * kind → AppIcon key。
 *
 * 每個 §F block kind 對應一個語意 glyph。整張表是封閉的：
 * 未知 kind 不渲染 icon，而不是掉到一個看起來像「某種狀態」的預設圖示。
 */
const NARRATIVE_ICON: Readonly<Record<PublicNarrativeKind, AppIconName>> =
	Object.freeze({
		UNDERSTOOD: "message",
		CONTEXT: "clipboard",
		RATIONALE: "info",
		WARNING: "warning",
		NEXT_STEP: "arrow-right",
		COMPLETE: "check",
		SUMMARY: "message",
	});

export function narrativeIcon(kind: PublicNarrativeKind): AppIconName | null {
	return NARRATIVE_ICON[kind] ?? null;
}

/**
 * 把整包 `reply` 字串轉成單一 SUMMARY block。#2549 Phase 2 起這是 fallback
 * 路徑（真正的 typed `blocks[]` 缺席時才用），服務 pre-Phase-2 回應或舊資料
 * 列重播，不是預設路徑。
 *
 * 空字串回空陣列——沒有內容就不要渲染一個空的 icon 列。
 */
export function legacyReplyToBlocks(reply: string): PublicNarrativeBlock[] {
	const text = reply.trim();
	return text ? [{ kind: "SUMMARY", text }] : [];
}
