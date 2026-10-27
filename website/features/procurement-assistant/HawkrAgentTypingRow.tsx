"use client";

import type { AgentMotionState } from "./agent-motion";
import HawkrAgentSprite from "./HawkrAgentSprite";

interface HawkrAgentTypingRowProps {
	/**
	 * 「現在在做什麼」的那一行字。省略＝這一輪的事實還沒產生，退回中性三點
	 * 指示而不編造階段（§V.10.6-2）。順序不可反過來：有字就顯示字，沒有才落回
	 * 三點——反過來等於有資料也不顯示。
	 */
	label?: string;
	/**
	 * Agent 標誌的狀態。由呼叫端的**權威訊號**傳進來，元件自己不猜（§V.10.10-5）。
	 * loop 與否由 `SPRITE_MOTION[state]` 決定，所以 §V.10.10-2「尚未 ACK 的
	 * SUBMITTING 不得循環」在這裡是結構性成立的，不靠這一層再判一次。
	 */
	motion: AgentMotionState;
	/** 這一列給輔助技術的名字。搬需求與找貨是兩件事，由呼叫端命名。 */
	ariaLabel: string;
}

/**
 * 對話流裡的單行進行中指示——設計稿 01 中央畫的那一列。
 *
 * 逐列的步驟軌住在右欄面板，對話流只留這一行「現在在做什麼」。窄視窗面板預設
 * 收起，沒有這一行就完全看不到進度，所以它不是窄視窗的補丁。
 *
 * 抽成共用元件而不是各自複製一份：釐清輪（`ProcurementClarifyChat` 的
 * `pending`）與候選搜尋（`AssistantWorkspace` 的 `searching`）畫的是同一列，
 * 兩份 markup 會讓 `.hawkr-agent-typing-row` 這個 CSS 契約有兩個擁有者，
 * 接著就是只改到其中一份。
 *
 * 這一列**不是** live region：它住在 `role="log" aria-live="off"` 的 transcript
 * 裡，朗讀由 `AssistantWorkspace` 那顆常駐的 sr-only atomic polite status 統一
 * 負責（軌本身 aria-hidden，播報只有那一個來源）。
 */
export default function HawkrAgentTypingRow({
	label,
	motion,
	ariaLabel,
}: HawkrAgentTypingRowProps) {
	return (
		<li className="hawkr-agent-typing-row" aria-label={ariaLabel}>
			<HawkrAgentSprite
				state={motion}
				className="hawkr-agent-rail__sprite"
				label=""
			/>
			{label ? (
				<span className="hawkr-agent-typing-row__label">{label}</span>
			) : (
				<div className="thread-page__typing" aria-hidden="true">
					<span /> <span /> <span />
				</div>
			)}
		</li>
	);
}
