import { z } from "zod";
import { ProcurementArtifactType } from "@/lib/codemeta/enums";
import type { BuyerQuoteAcceptanceResponse } from "./buyer-quote-contract";
import { idSchema, payloadHashSchema } from "./rfq-price-contract";

/**
 * 對話 runtime 快照的 wire 契約（doc 57 §5.6，稿 R-02 的資料來源）。
 *
 * `GET /app/buyers/me/procurement/conversations/{id}/runtime-snapshot`
 * 是 reload／SSE 中斷之後唯一的 owner-scoped 還原來源（V.11.4 Restore source：
 * 「成功 hydrate 後才可顯示『已從伺服器恢復』」）。本檔是這支端點在本 repo 的
 * 第一個消費端，所以每一條解析規則都寫成 fail closed：
 *
 *   ⛔ **缺欄位不補值、不猜、不降級。** 這個畫面要回答的是「現在到底成立了
 *      什麼」，一個補出來的 0 或一個猜出來的時間戳，會讓買家把「還沒發生」
 *      讀成「已經發生」。讀不到就讓對應的列不顯示，或顯示「尚未取得」。
 *
 * 三件與直覺相反、但由後端明文決定的事：
 *
 *   1. **要用 `runs[]`，不是 `activeRunId`。** 後端的 active-run 掃描刻意排除
 *      `CANDIDATE_SEARCH`／`QUOTE_APPROVAL`（ai-service
 *      `buildConversationRuntimeView`）：candidate search 有兩條刻意跳過
 *      `TerminalizeRun` 的分支，會留下永遠 QUEUED 的 zombie row，算進去會
 *      永久佔住 `activeRunId`。`runs[]` 本身仍是完整清單。
 *   2. **`runs[]` 與 `artifacts[]` 都是最新排最前**（`ListRunsByConversation`／
 *      `ListLatestProcurementAgentRunArtifactsByConversation` 的
 *      `ORDER BY ... DESC`）。本檔仍不倚賴這個順序挑 artifact，改用
 *      `createdAt` 比大小——順序是實作細節，時間戳是資料。
 *   3. **同一型 artifact 可以有多列**：去重鍵是 `artifact_id` 而不是
 *      `(run_id, artifact_type)`，所以同一型在不同 run 各有一個 artifact_id，
 *      快照裡會同時看到它們。
 */

/**
 * 一列 run。
 *
 * `runType`／`status` 是**開放值域**（domain/run.go：「client 需 default
 * 分支」），所以這裡收一般字串而不是 enum：把未登記的新值當成解析失敗，會讓
 * 後端加一個 run type 就整份快照讀不到。`terminal` 由伺服器判定，前端不得自己
 * 用 status 推導第二套終態清單。
 */
export const runtimeSnapshotRunSchema = z.object({
	runId: idSchema,
	runType: z.string().min(1).max(64),
	status: z.string().min(1).max(64),
	lastSeqId: z.string().max(32).default(""),
	minAvailableSeqId: z.string().max(32).default(""),
	terminal: z.boolean(),
	/**
	 * 這一列 run 的 CAS token（gateway `ProcurementAgentRunSummaryDTO.runVersion`，
	 * 十進位字串）。取消命令的 `expectedRunVersion` **只能**來自這裡——瀏覽器沒有
	 * 第二個地方拿得到它，而沒有它的取消請求會被伺服器以 400 擋下。
	 *
	 * 與鄰近欄位同樣寬鬆：舊版 gateway 不回這個欄位，缺席不該讓整份快照解析
	 * 失敗、把發布結果與進度一起藏掉。空字串＝**伺服器沒給**，不是 0；兩者的
	 * 差別正好就是「停止鈕能不能按」，所以這裡不補值，由
	 * `selectCancellableRun` 明確分辨。
	 */
	runVersion: z.string().max(32).default(""),
});

export type RuntimeSnapshotRun = z.infer<typeof runtimeSnapshotRunSchema>;

/**
 * 一列 typed artifact。
 *
 * `data` 是 gateway 原樣嵌進來的 JSON（`json.RawMessage`），形狀依
 * `artifactType` 而定，所以這一層只當 `unknown` 收下，由各型自己的 schema
 * 再解一次；在這裡先鎖形狀等於替十三種 artifact 都預先押一個猜測。
 *
 * `domainRefType`／`domainRefId` 在 wire 上是 `omitempty`，缺席即空字串。
 */
export const runtimeSnapshotArtifactSchema = z.object({
	artifactId: idSchema,
	runId: idSchema,
	artifactType: z.string().min(1).max(64),
	version: z.string().regex(/^\d{1,9}$/, "artifact version must be decimal"),
	status: z.string().min(1).max(64),
	data: z.unknown(),
	checksum: payloadHashSchema,
	domainRefType: z.string().max(64).default(""),
	domainRefId: z.string().max(32).default(""),
	asOf: z.string().max(64).default(""),
	createdAt: z.string().max(64).default(""),
});

export type RuntimeSnapshotArtifact = z.infer<
	typeof runtimeSnapshotArtifactSchema
>;

/** 未解的中斷。`expiresAt` 在 v1 恆為空字串（interrupt 不過期，Task 14b M6）。 */
export const runtimeSnapshotOpenConfirmationSchema = z.object({
	interruptId: z.string().min(1).max(64),
	confirmationKind: z.string().min(1).max(64),
	expiresAt: z.string().max(64).default(""),
});

export type RuntimeSnapshotOpenConfirmation = z.infer<
	typeof runtimeSnapshotOpenConfirmationSchema
>;

/**
 * 快照外層。
 *
 * `messages` 刻意收 `z.array(z.unknown())`：這個畫面不讀對話內容，而替它宣告
 * 一份形狀等於多開一條「訊息 DTO 改欄位就讓發布結果整個讀不到」的耦合。真正
 * 要讀 transcript 的地方走 `conversations.ts` 自己的解析。
 *
 * `activeRunId`／`openConfirmation` 在 wire 上是 `omitempty`。
 */
export const runtimeSnapshotSchema = z.object({
	conversationId: idSchema,
	requestId: idSchema,
	status: z.string().max(64).default(""),
	title: z.string().max(200).default(""),
	messages: z.array(z.unknown()).default([]),
	runs: z.array(runtimeSnapshotRunSchema),
	activeRunId: z.string().max(32).default(""),
	artifacts: z.array(runtimeSnapshotArtifactSchema),
	openConfirmation: runtimeSnapshotOpenConfirmationSchema.optional(),
});

export type RuntimeSnapshot = z.infer<typeof runtimeSnapshotSchema>;

/**
 * `RFQ_PUBLICATION_RESULT` 的 `data`——恰好六欄的 allowlist（ai-service
 * `publicationResultData`，#2450 Scope 9）。
 *
 * 三個 count 是**發布當下的意圖分布，不是投遞結果**：後端刻意不放 `sentCount`
 * （artifact 有 checksum、不隨 worker 進度重算，放一個永遠 0 的欄位會誤導）。
 * 所以這裡也不得把 `queuedCount` 呈現成「已送出」。WISH_POOL 三欄皆為 0。
 */
export const publicationResultDataSchema = z.object({
	publicationId: idSchema,
	version: z.string().min(1).max(32),
	sellerFacingPayloadHash: payloadHashSchema,
	recipientCount: z.number().int().min(0),
	queuedCount: z.number().int().min(0),
	notSentCount: z.number().int().min(0),
});

export type PublicationResultData = z.infer<typeof publicationResultDataSchema>;

/** 已成立的發布結果：artifact 的身分＋它承載的 allowlisted payload。 */
export interface PublicationResultView {
	/** artifact 版本；買家可見文案用「結果版本」（V.11.4 Copy freeze）。 */
	artifactVersion: string;
	/** artifact 的 `createdAt`；空字串＝伺服器沒給，畫面不補當下時間。 */
	publishedAt: string;
	data: PublicationResultData;
}

/**
 * 從快照挑出目前的發布結果。
 *
 * 挑「最新」用 `createdAt` 字串比大小（ISO8601 可字典序比較），不倚賴陣列順序
 * ——順序是 SQL 的 `ORDER BY`，是實作細節；時間戳才是資料。時間戳相同或缺席時
 * 退回先出現的那一筆，與 wire 的最新排最前一致。
 *
 * `data` 解不出六欄就**整筆跳過**，不退化成「有發布但內容未知」：這個畫面唯一
 * 會用到發布結果的地方就是宣告「已發布詢價」，而那句話不該由一個讀不懂的
 * payload 撐著。
 */
export function selectPublicationResult(
	snapshot: RuntimeSnapshot,
): PublicationResultView | null {
	let best: PublicationResultView | null = null;
	for (const artifact of snapshot.artifacts) {
		if (
			artifact.artifactType !== ProcurementArtifactType.RFQ_PUBLICATION_RESULT
		)
			continue;
		const parsed = publicationResultDataSchema.safeParse(artifact.data);
		if (!parsed.success) continue;
		const candidate: PublicationResultView = {
			artifactVersion: artifact.version,
			publishedAt: artifact.createdAt,
			data: parsed.data,
		};
		if (best === null || candidate.publishedAt > best.publishedAt)
			best = candidate;
	}
	return best;
}

/**
 * 這個 conversation 還有沒有沒跑完的 run。
 *
 * 用 `runs[].terminal`（伺服器判定）而不是 `activeRunId`：後者刻意排除
 * `CANDIDATE_SEARCH`／`QUOTE_APPROVAL`，拿它當「還在忙嗎」的答案會漏。
 */
export function hasLiveRun(snapshot: RuntimeSnapshot): boolean {
	return snapshot.runs.some((run) => !run.terminal);
}

/** run 版本是十進位計數器（proto `AgentRunSummary.version`）。 */
const RUN_VERSION_RE = /^\d{1,9}$/;

/** 可以對它下取消命令的那一個 run：身分＋當下的 CAS token。 */
export interface CancellableRun {
	runId: string;
	/** `expectedRunVersion`；保證是十進位數字字串（空字串不會走到這裡）。 */
	runVersion: string;
	runType: string;
	status: string;
}

/**
 * 這個對話現在有沒有一個「按下停止會真的停到」的 run。
 *
 * 用 `activeRunId` 而不是自己掃 `runs[]` 找非終態的那幾列——這裡與
 * `hasLiveRun` 的取捨相反，而且是刻意的：
 *
 *   - `hasLiveRun` 問的是「還有沒有事情在跑」，漏掉 candidate search 會把
 *     忙碌講成閒置，所以它必須看完整的 `runs[]`。
 *   - 這支問的是「該對哪一個 run 下命令」。`activeRunId` 正是伺服器自己的
 *     答案，而且它排除的 `CANDIDATE_SEARCH`／`QUOTE_APPROVAL` 恰好就是會留下
 *     永遠 QUEUED 的 zombie 列的那兩型。自己掃非終態列會挑中 zombie，變成
 *     對一個沒人在等的 run 送出取消，然後跟買家說「正在停止」。
 *
 * 三道 fail closed，任何一道不過就回 null（呼叫端讓按鈕維持不可用）：
 *   1. 沒有 `activeRunId`：伺服器說現在沒有執行中的 run。
 *   2. `runs[]` 裡找不到那一列、或它已經 `terminal`：拿不到 CAS token，或
 *      這一輪已經到終點。
 *   3. `runVersion` 不是十進位字串（含舊版後端的空字串）：**沒有 token 就
 *      不送**。送出去只會換回一個 400，而畫面上買家看到的是「停止失敗」。
 */
export function selectCancellableRun(
	snapshot: RuntimeSnapshot,
): CancellableRun | null {
	if (!snapshot.activeRunId) return null;
	const row = snapshot.runs.find((run) => run.runId === snapshot.activeRunId);
	if (!row || row.terminal) return null;
	if (!RUN_VERSION_RE.test(row.runVersion)) return null;
	return {
		runId: row.runId,
		runVersion: row.runVersion,
		runType: row.runType,
		status: row.status,
	};
}

/**
 * 取消命令的處置（gateway `cancelDisposition`）。
 *
 * ⛔ **四個都是成功回應，不是四種錯誤。** 後端回 200 並明說這次命令落在哪一
 * 種結果上；`TOO_LATE` 尤其不是失敗，它是設計書「不得謊報取消成功」的落實：
 * 側效已經 commit 了，所以伺服器拒絕把它講成已取消。畫面必須照講。
 */
export const RUN_CANCEL_DISPOSITIONS = [
	"REQUESTED",
	"ALREADY_CANCELLED",
	"ALREADY_TERMINAL",
	"TOO_LATE",
] as const;

export type RunCancelDisposition = (typeof RUN_CANCEL_DISPOSITIONS)[number];

/**
 * 取消回執。
 *
 * `cancelDisposition` 刻意是**封閉**值域——與 `runType`／`status` 的開放值域
 * 相反。那兩個欄位只是拿來顯示，多一個沒登記的值頂多少一行字；這個欄位決定
 * 畫面要對買家講哪一句話，讀到沒登記的值時唯一誠實的做法是說「無法確認這次
 * 取消的結果」，而不是挑一句最像的講。
 *
 * 回應另有 `schemaVersion`，但這一層不消費，所以不替它押型別。
 */
export const runCancelResultSchema = z.object({
	cancelDisposition: z.enum(RUN_CANCEL_DISPOSITIONS),
	runId: idSchema,
	runStatus: z.string().min(1).max(64),
	/** 命令之後的新版本；同 `runVersion` 的寬鬆處理，缺席即空字串。 */
	runVersion: z.string().max(32).default(""),
});

export type RunCancelResult = z.infer<typeof runCancelResultSchema>;

/**
 * 逐字固定的買家可見文案。
 *
 * 只有 `REQUESTED` 是「正在停止」。另外三個講的都是**這一輪已經到終點**，
 * 而且都刻意避開「已取消」這個主詞——買家按下停止並不會讓一個已經跑完的
 * 結果作廢，把 `ALREADY_TERMINAL` 寫成「已為您取消」正是設計書禁止的謊報。
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

/**
 * 逐字固定的六步驟標籤（稿 R-02 ②）。
 *
 * ⚠️ `PUBLISHED` 刻意偏離稿的「已投入許願池」，改用不指涉通路的「已發布詢價」。
 * 稿寫這句時預設發布通路是許願池，但本站 `RfqDraftFlow` 的 `audienceMode` 寫死
 * `DIRECT_RECIPIENTS`（pilot 只做定向發布），而 `RFQ_PUBLICATION_RESULT` 的
 * payload 六欄裡**沒有** `audienceMode` ⇒ 這一層拿不到通路事實。與其由「有沒有
 * publication」推導出一個錯的通路宣稱，不如只講確實成立的事。
 */
const PROGRESS_LABELS: Record<RfqProgressStepKey, string> = {
	SNAPSHOT_CONFIRMED: "已確認 RFQ 快照",
	PUBLISHED: "已發布詢價",
	AWAITING_QUOTES: "等待供應商正式回價",
	BUYER_ACCEPTANCE: "買家比較並接受指定報價版本",
	ORDER_CREATED: "建立待付款訂單",
	PAYMENT_SETTLED: "付款成功後正式成立",
};

export interface RfqProgressInput {
	/** 發布結果 artifact；null＝伺服器沒有這筆事實。 */
	publication: PublicationResultView | null;
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
 * 由伺服器事實推導六步驟進度。
 *
 * 每一條「已完成」都必須是**蘊含關係**而不是推測：
 *
 *   - 有 `RFQ_PUBLICATION_RESULT` ⇒ 快照一定確認過（publication execute 的
 *     body 必帶 `confirmedRfqSnapshotId`，沒有確認快照就產不出這個 artifact），
 *     所以第一步標「已完成」。但**沒有時間戳**——後端目前不產
 *     `CONFIRMED_RFQ` artifact（實際寫入的只有 `CANDIDATE_RESULTS`／
 *     `RFQ_PUBLICATION_RESULT`／`ERROR_RECOVERY` 三型），那一格因此留白，
 *     不拿發布時間頂替。
 *   - 沒有發布結果時，第一步是「進行中」而不是「已完成」：伺服器唯一保證的事實
 *     是這張詢價還在買家手上，把未經伺服器確認的快照畫成已確認，正是這個版面
 *     要避免的樂觀文案。
 *   - 收到過任何一份正式回價 ⇒ 第三步「等待回價」已完成。0 筆（且確實讀到）
 *     時它才是「進行中」。
 *   - 第六步「付款成功後正式成立」**永遠不會**在這個畫面標成已完成：付款事實
 *     不在本畫面任何一個資料來源裡（v6.0 §V.11.4／§V.11.5：付款成功前不得
 *     宣稱正式成交）。
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
