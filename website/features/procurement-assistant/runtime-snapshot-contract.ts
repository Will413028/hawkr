import type { GetRuntimeSnapshotResponse } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import type { BuyerQuoteAcceptanceResponse } from "./buyer-quote-contract";
import {
	openConfirmationFromProto,
	publishedRfqFromProto,
	requestStatusCode,
	runViewFromProto,
} from "./proto-map";
import type {
	ProcurementRequestStatus,
	ProcurementRunView,
	ProcurementSnapshot,
	PublishedRfqPublication,
} from "./types";

/**
 * 對話 runtime 快照（`GetRuntimeSnapshot`）：reload 或串流中斷之後唯一的還原來源，
 * 回答「現在到底成立了什麼」。讀不到的事實不補值、不猜、不降級：一個補出來的 0 或猜出來的
 * 時間，會讓買家把「還沒發生」讀成「已經發生」。
 */
export interface RuntimeSnapshot {
	conversationId: string;
	requestId: string;
	status: ProcurementRequestStatus;
	title: string;
	runs: ProcurementRunView[];
	activeRunId: string;
	openConfirmation: ProcurementSnapshot["openConfirmation"];
	/** 最新一次的發布結果；還沒有發布時是 null。 */
	publication: PublishedRfqPublication | null;
}

export function runtimeSnapshotFromProto(
	res: GetRuntimeSnapshotResponse,
): RuntimeSnapshot {
	return {
		conversationId: res.conversationId,
		requestId: res.requestId,
		status: requestStatusCode(res.status),
		title: res.title,
		runs: res.runs.map(runViewFromProto),
		activeRunId: res.activeRunId,
		openConfirmation: openConfirmationFromProto(res.openConfirmation),
		publication: publishedRfqFromProto(res.publishedRfq),
	};
}

/**
 * 這個 conversation 還有沒有沒跑完的 run。用 `runs[].terminal`（伺服器判定），
 * 不自己用 status 推導第二套終態清單。
 */
export function hasLiveRun(snapshot: RuntimeSnapshot): boolean {
	return snapshot.runs.some((run) => !run.terminal);
}

/** 可以對它下取消命令的那一個 run。 */
export interface CancellableRun {
	runId: string;
}

/**
 * 這個對話現在有沒有一個「按下停止會真的停到」的 run：伺服器的 `activeRunId` 指的那一個，
 * 而且 `runs[]` 裡沒有標成終態。任何一道不過就回 null（呼叫端讓按鈕維持不可用）。
 */
export function selectCancellableRun(
	snapshot: RuntimeSnapshot,
): CancellableRun | null {
	if (!snapshot.activeRunId) return null;
	const row = snapshot.runs.find((run) => run.runId === snapshot.activeRunId);
	if (!row || row.terminal) return null;
	return { runId: row.runId };
}

/**
 * 取消命令的處置。
 *
 * 四個都是成功回應，不是四種錯誤；`TOO_LATE` 尤其不是失敗：側效已經 commit 了，
 * 所以伺服器拒絕把它講成已取消，畫面必須照講。
 */
export const RUN_CANCEL_DISPOSITIONS = [
	"REQUESTED",
	"ALREADY_CANCELLED",
	"ALREADY_TERMINAL",
	"TOO_LATE",
] as const;

export type RunCancelDisposition = (typeof RUN_CANCEL_DISPOSITIONS)[number];

/**
 * 逐字固定的買家可見文案。
 *
 * 只有 `REQUESTED` 是「正在停止」。另外三個講的都是**這一輪已經到終點**，
 * 而且都刻意避開「已取消」這個主詞：買家按下停止並不會讓一個已經跑完的
 * 結果作廢，把 `ALREADY_TERMINAL` 寫成「已為您取消」就是謊報。
 */
export const RUN_CANCEL_COPY: Record<RunCancelDisposition, string> = {
	REQUESTED: "已送出停止，正在收尾這一輪。",
	ALREADY_CANCELLED: "這一輪先前已經停止，這次沒有重複送出。",
	ALREADY_TERMINAL:
		"這一輪已經跑完了，已完成的結果仍然有效，沒有被這次停止作廢。",
	TOO_LATE: "這一輪已經完成並送出，來不及停止；已完成的結果仍然有效。",
};

/** 六步驟的識別碼；順序即畫面順序（稿 R-02 ②）。 */
export type RfqProgressStepKey =
	| "SNAPSHOT_CONFIRMED"
	| "PUBLISHED"
	| "AWAITING_QUOTES"
	| "BUYER_ACCEPTANCE"
	| "ORDER_CREATED"
	| "PAYMENT_SETTLED";

/**
 * 一步的狀態。
 *
 * `failed` 只給「伺服器明說這一步沒成功」用（目前只有
 * `orderProvisioningState=ORDER_FAILED`）——它不是 `pending` 的強調版，兩者
 * 在畫面上要講不同的話。
 */
export type RfqProgressStepState = "done" | "current" | "pending" | "failed";

export interface RfqProgressStep {
	key: RfqProgressStepKey;
	label: string;
	state: RfqProgressStepState;
	/** 伺服器給的時間戳；空字串＝沒有這個事實，該格留白，不補當下時間。 */
	at: string;
}

/** 逐字固定的六步驟標籤（稿 R-02 ②）。「已發布詢價」不指涉通路，只講確實成立的事。 */
const PROGRESS_LABELS: Record<RfqProgressStepKey, string> = {
	SNAPSHOT_CONFIRMED: "已確認 RFQ 快照",
	PUBLISHED: "已發布詢價",
	AWAITING_QUOTES: "等待供應商正式回價",
	BUYER_ACCEPTANCE: "買家比較並接受指定報價版本",
	ORDER_CREATED: "建立待付款訂單",
	PAYMENT_SETTLED: "付款成功後正式成立",
};

export interface RfqProgressInput {
	/** 發布結果；null＝伺服器沒有這筆事實。 */
	publication: PublishedRfqPublication | null;
	/**
	 * 伺服器回的正式回價筆數。
	 *
	 * ⛔ `null`（還沒讀到／讀失敗）**不得**當成 0：那正好是把「不知道」講成
	 * 「確定沒有」。0 只能來自一次成功的清單讀取。
	 */
	responseCount: number | null;
	/** 接受回執；null＝這個畫面沒有收到過權威 ACK。 */
	acceptance: BuyerQuoteAcceptanceResponse | null;
}

function step(
	key: RfqProgressStepKey,
	state: RfqProgressStepState,
	at = "",
): RfqProgressStep {
	return { key, label: PROGRESS_LABELS[key], state, at };
}

/**
 * 由伺服器事實推導六步驟進度。每一條「已完成」都必須是蘊含關係而不是推測：
 *
 *   - 有發布結果 ⇒ 發布核准存在過，所以第一步標「已完成」；沒有時間戳，那一格留白，不拿發布時間頂替。
 *   - 沒有發布結果時，第一步是「進行中」：把未經伺服器確認的步驟畫成已完成，正是這個版面要避免的樂觀文案。
 *   - 收到過任何一份正式回價 ⇒ 第三步「等待回價」已完成；0 筆（且確實讀到）時它才是「進行中」。
 *   - 第六步「付款成功後正式成立」永遠不會在這個畫面標成已完成：付款事實不在任何資料來源裡。
 */
export function deriveRfqProgress(input: RfqProgressInput): RfqProgressStep[] {
	const { publication, responseCount, acceptance } = input;
	if (!publication) {
		return [
			step("SNAPSHOT_CONFIRMED", "current"),
			step("PUBLISHED", "pending"),
			step("AWAITING_QUOTES", "pending"),
			step("BUYER_ACCEPTANCE", "pending"),
			step("ORDER_CREATED", "pending"),
			step("PAYMENT_SETTLED", "pending"),
		];
	}

	const confirmed = step("SNAPSHOT_CONFIRMED", "done");
	const published = step("PUBLISHED", "done", publication.publishedAt);
	const hasQuotes = responseCount !== null && responseCount > 0;

	if (!acceptance) {
		return [
			confirmed,
			published,
			step("AWAITING_QUOTES", hasQuotes ? "done" : "current"),
			step("BUYER_ACCEPTANCE", hasQuotes ? "current" : "pending"),
			step("ORDER_CREATED", "pending"),
			step("PAYMENT_SETTLED", "pending"),
		];
	}

	// ⛔ 有 ACK 也只到「已凍結成交價格」為止。CREATING_ORDER 是買家看得到的
	// 中間狀態，不是「已下單」；ORDER_FAILED 是明確的失敗，不能畫成待進行。
	const orderState: RfqProgressStepState =
		acceptance.orderProvisioningState === "ORDER_CREATED"
			? "done"
			: acceptance.orderProvisioningState === "ORDER_FAILED"
				? "failed"
				: "current";
	return [
		confirmed,
		published,
		step("AWAITING_QUOTES", "done"),
		step("BUYER_ACCEPTANCE", "done", acceptance.acceptedAt),
		step("ORDER_CREATED", orderState),
		step("PAYMENT_SETTLED", "pending"),
	];
}
