import {
	activeReveal,
	type KycReveal,
	revealRemainingMs,
} from "./kyc-application";

// 顯示全文（K15）之後畫面上有兩樣東西依附在它的授權上：明文，與放大檢視的證件影像。它們一起出現、一起消失：到期、收起、換店
// 時明文被清掉，已經打開的放大圖也要一起關掉（影像網址的授權同時失效），不能只清其中一個。

export interface PreviewImage {
	src: string;
	alt: string;
}

/** 顯示全文的畫面狀態：此刻的授權與放大中的影像。沒有授權就沒有放大圖。 */
export interface RevealState {
	reveal: KycReveal | null;
	preview: PreviewImage | null;
}

export const EMPTY_REVEAL_STATE: RevealState = { reveal: null, preview: null };

/** 顯示全文成功：新的授權，沒有放大圖。 */
export function showReveal(reveal: KycReveal): RevealState {
	return { reveal, preview: null };
}

/** 到期（含剛好到期的那一刻）：授權與放大圖一起清掉；還沒到期原封不動（回傳同一個物件）。 */
export function expireReveal(state: RevealState, now: Date): RevealState {
	if (!state.reveal) return state;
	return activeReveal(state.reveal, state.reveal.applicationId, now)
		? state
		: EMPTY_REVEAL_STATE;
}

/** 收起：授權與放大圖一起清掉。 */
export function hideReveal(): RevealState {
	return EMPTY_REVEAL_STATE;
}

/** 放大一張影像：只有授權還在時才能放大。 */
export function openPreview(
	state: RevealState,
	preview: PreviewImage,
): RevealState {
	return state.reveal ? { ...state, preview } : state;
}

export function closePreview(state: RevealState): RevealState {
	return state.preview ? { ...state, preview: null } : state;
}

export interface ExpiryTimers {
	now: () => Date;
	setTimer: (callback: () => void, ms: number) => unknown;
	clearTimer: (handle: unknown) => void;
}

/**
 * 排程在授權到期時呼叫 onExpire。計時器可能提早觸發（或在背景分頁被延後之後才觸發）：觸發時還沒到期就依剩餘時間重排，不是排一次
 * 就算數；觸發時已到期才呼叫 onExpire（只呼叫一次）。回傳的 recheck 讓呼叫端在別的時機（例如分頁回到前景）立刻再檢查一次，
 * cancel 取消排程。
 */
export function scheduleRevealExpiry(
	reveal: KycReveal,
	onExpire: () => void,
	timers: ExpiryTimers,
): { recheck: () => void; cancel: () => void } {
	let handle: unknown;
	let done = false;
	const check = () => {
		if (done) return;
		timers.clearTimer(handle);
		const remaining = revealRemainingMs(reveal, timers.now());
		if (remaining > 0) {
			handle = timers.setTimer(check, remaining);
			return;
		}
		done = true;
		onExpire();
	};
	handle = timers.setTimer(check, revealRemainingMs(reveal, timers.now()));
	return {
		recheck: check,
		cancel: () => {
			done = true;
			timers.clearTimer(handle);
		},
	};
}
