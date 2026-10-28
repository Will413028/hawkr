/**
 * Hawkr Agent 的 motion-state 契約（設計書 v6 §V.10.10、§V.10.10A）。
 *
 * 這支檔案只放**對映表**，不含 React——contract test 才能在不掛載元件的前提下
 * 斷言「Entry 不循環、terminal 單次」。
 *
 * 兩條不變式（違反其一就是設計書層級的 bug，不是樣式問題）：
 *
 * 1. §V.10.10-2：Entry `IDLE` 與 `SUBMITTING` 一律 `loop: false`。ACK 之前
 *    持續循環會假裝伺服器已經接受了這一輪。
 * 2. §V.10.10-5：記號是情緒輔助，不是 status truth。每個 state 都對應一個
 *    **已經由 canonical event 決定**的狀態，沒有「猜下一步」的中間態。
 */

import type { ProcurementStage } from "./types";

/**
 * §V.10.10-2 的 allowlisted motion states，外加 `SUBMITTING`。
 *
 * `SUBMITTING` 在 §V.10.10-2 只以否定句出現（「不得在尚未 ACK 的 SUBMITTING
 * 階段 loop」），但 §V.11.0「First submit」要求 SUBMITTING 與 RUN_ACCEPTED 分開
 * 呈現——所以它必須是一個真的 state，否則那條規則沒有地方可以被違反或被測。
 */
export type AgentMotionState =
	| "IDLE"
	| "SUBMITTING"
	| "UNDERSTANDING"
	| "SEARCHING"
	| "VERIFYING"
	| "BUILDING_ARTIFACT"
	| "WAITING_FOR_INPUT"
	| "SUCCESS"
	| "RECOVERABLE_ERROR"
	| "RECONNECTING";

export interface SpriteMotion {
	/** 只有 authoritative active state 才為真（§V.10.10-2／-3）；為真時畫載入動態，斷線不得冒充工作仍在執行。 */
	loop: boolean;
	/** 給 `role="img"` 的靜態標籤。不進 aria-live（§V.10.10-3）。 */
	label: string;
}

/** state → 是否循環與靜態標籤。 */
export const SPRITE_MOTION: Readonly<Record<AgentMotionState, SpriteMotion>> =
	Object.freeze({
		IDLE: { loop: false, label: "Hawkr Agent 待命中" },
		SUBMITTING: {
			loop: false,
			label: "Hawkr Agent 正在送出你的需求",
		},
		UNDERSTANDING: {
			loop: true,
			label: "Hawkr Agent 正在讀你的需求",
		},
		SEARCHING: { loop: true, label: "Hawkr Agent 正在找貨" },
		VERIFYING: { loop: true, label: "Hawkr Agent 正在核對" },
		BUILDING_ARTIFACT: {
			loop: true,
			label: "Hawkr Agent 正在整理結果",
		},
		WAITING_FOR_INPUT: {
			loop: false,
			label: "Hawkr Agent 需要你補一項資訊",
		},
		SUCCESS: { loop: false, label: "Hawkr Agent 已完成" },
		RECOVERABLE_ERROR: {
			loop: false,
			label: "Hawkr Agent 遇到需要你處理的狀況",
		},
		RECONNECTING: {
			loop: false,
			label: "Hawkr Agent 正在重新連線",
		},
	});

/**
 * ACK 之前不得循環的 state（§V.10.10-2）。contract test 直接斷言 Entry
 * `IDLE` 與送出中的 `SUBMITTING` 都是 `loop: false`，不是靠讀元件實作。
 */
export const PRE_ACK_MOTION_STATES: readonly AgentMotionState[] = Object.freeze(
	["IDLE", "SUBMITTING"],
);

export function spriteMotion(state: AgentMotionState): SpriteMotion {
	return SPRITE_MOTION[state];
}

/**
 * 後端 StepCode → motion state。
 *
 * 「一句話講完這一步在做什麼」與「Agent 標誌的狀態」是同一件事的兩種呈現，所以
 * 對映放在這裡而不是散在元件裡：新增 StepCode 時只要補一列，畫面與動畫就同步。
 * 未知 code 落到 `UNDERSTANDING`——那是最中性的「正在處理」，不會冒充成功、
 * 失敗或「找到了」。
 *
 * §V.10.10-5：表情是情緒輔助，不是 status truth。所以這裡只把 step 分成
 * 讀需求／找貨／核對／整理四種節奏，不替任何一步加上伺服器沒說的結論。
 */
const STEP_MOTION: Readonly<Record<string, AgentMotionState>> = Object.freeze({
	UNDERSTAND_INPUT: "UNDERSTANDING",
	EXTRACT_SLOTS: "UNDERSTANDING",
	CAPTURE_ITEM: "UNDERSTANDING",
	CAPTURE_QUANTITY: "UNDERSTANDING",
	CAPTURE_UNIT: "UNDERSTANDING",
	CAPTURE_PACKAGING: "UNDERSTANDING",
	CAPTURE_BUDGET: "UNDERSTANDING",
	CAPTURE_CITY: "UNDERSTANDING",
	CAPTURE_DATE: "UNDERSTANDING",
	CAPTURE_EXACT_MATCH: "UNDERSTANDING",
	PICK_NEXT_QUESTION: "UNDERSTANDING",
	VALIDATE_SLOTS: "VERIFYING",
	SLOTS_REJECTED: "VERIFYING",
	VERIFY_ORIGIN: "VERIFYING",
	CHECK_STOCK: "VERIFYING",
	COMPARE: "VERIFYING",
	SEARCH_PLATFORM: "SEARCHING",
	SEARCH_EXTERNAL: "SEARCHING",
	READ_SELLER: "SEARCHING",
	REQUIREMENT_READY: "BUILDING_ARTIFACT",
	RANK_CANDIDATES: "BUILDING_ARTIFACT",
	SUMMARIZE: "BUILDING_ARTIFACT",
	DEGRADED_FALLBACK: "RECOVERABLE_ERROR",
	FAILED: "RECOVERABLE_ERROR",
});

export function motionForStep(code: string): AgentMotionState {
	return STEP_MOTION[code] ?? "UNDERSTANDING";
}

export interface MotionSignals {
	/** 這一輪有可恢復的錯誤。 */
	hasError: boolean;
	/** 請求已送出、還沒落地。 */
	pending: boolean;
	/** 目前收到的最後一個 public step code；還沒收到任何一步時為 null。 */
	latestStepCode: string | null;
	/** 有等待作答的問題卡。 */
	awaitingInput: boolean;
	stage: ProcurementStage;
}

/**
 * 由這一輪的權威訊號推出 Agent 標誌該呈現的狀態。
 *
 * 順序就是優先權，別調換：
 * 1. 錯誤壓過一切——正在轉圈的標誌配一則錯誤訊息是最糟的組合。
 * 2. `pending` 但還沒收到第一個 step ＝ 尚未 ACK，只能是 `SUBMITTING`
 *    （§V.10.10-2 禁止在這個階段循環播放，那等於用動畫假裝伺服器收下了）。
 * 3. 有 step 就跟著 step 走。
 * 4. 落地後才輪到「等你回答／找貨中／完成」這些靜態結論。
 */
export function deriveMotionState(signals: MotionSignals): AgentMotionState {
	if (signals.hasError) return "RECOVERABLE_ERROR";
	if (signals.pending) {
		return signals.latestStepCode
			? motionForStep(signals.latestStepCode)
			: "SUBMITTING";
	}
	if (signals.awaitingInput) return "WAITING_FOR_INPUT";
	if (signals.stage === "searching") return "SEARCHING";
	if (signals.stage === "candidates") return "SUCCESS";
	return "IDLE";
}
