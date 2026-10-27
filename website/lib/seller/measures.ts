/**
 * 賣家的措施與申覆（`SellerAppService.ListMyMeasures`、`GetMyMeasure`、`SubmitAppeal`）。賣家路徑直接使用
 * 生成的 `Measure`：`requests` 是申覆與解除請求（新到舊，每筆的狀態是伺服器算好的生效狀態），列舉是 UNSPECIFIED 表示沒有值，不是有效的狀態。能不能申覆讀
 * `canAppeal`（伺服器判斷，與 SubmitAppeal 的前置檢查同一個規則），生效與否讀 `state`（伺服器依時間推導），
 * client 都不自己推導。依據的類別、風險類型與狀態的顯示文字在呈現層（`features/seller/measures/measureDisplay.ts`）。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import type { DefaultErrorText } from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import {
	type Measure,
	MeasureState,
	SellerAppService,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerAppClient } from "@/lib/connect/seller";

export type SellerMeasure = Measure;

type SellerClient = ReturnType<typeof sellerAppClient>;

/** 一頁最多 100 筆（契約上限）；用 `next_page_token` 讀完全部分頁。 */
const MEASURES_PAGE_SIZE = 100;

/**
 * 讀完措施的全部分頁。`state` 是 `UNSPECIFIED`（預設）時不篩；公告頁傳 `ACTIVE` 只讀生效中的停權。
 */
export async function fetchMyMeasures(
	state: MeasureState = MeasureState.UNSPECIFIED,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerMeasure[]> {
	const measures: SellerMeasure[] = [];
	let pageToken = "";
	do {
		const res = await client().listMyMeasures({
			pageSize: MEASURES_PAGE_SIZE,
			pageToken,
			state,
		});
		measures.push(...res.measures);
		if (res.nextPageToken !== "" && res.nextPageToken === pageToken) {
			throw new Error("ListMyMeasures 的 next_page_token 沒有前進");
		}
		pageToken = res.nextPageToken;
	} while (pageToken !== "");
	return measures;
}

/** 開店頁只問「有沒有措施」：一筆就夠。 */
export async function fetchHasMeasures(
	client: () => SellerClient = sellerAppClient,
): Promise<boolean> {
	const res = await client().listMyMeasures({ pageSize: 1 });
	return res.measures.length > 0;
}

export async function fetchMyMeasure(
	measureId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerMeasure> {
	const res = await client().getMyMeasure({ measureId });
	if (!res.measure) throw new Error("GetMyMeasure 的回應沒有措施");
	return res.measure;
}

export interface SubmitAppealInput {
	reason: string;
	/** 機敏上傳（`uploadSecureDocument`）回的 staged token。 */
	evidenceStagedTokens: string[];
}

/**
 * 以讀到的措施的 etag 提出申覆；措施在這之前被改過時伺服器回 VERSION_CONFLICT，不寫入。提出申覆不改
 * etag，所以同一筆重送（例如另一個分頁已送出）回 failed_precondition 而不是 VERSION_CONFLICT。
 */
export async function submitMeasureAppeal(
	measure: Pick<SellerMeasure, "id" | "etag">,
	input: SubmitAppealInput,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerMeasure> {
	const res = await client().submitAppeal({
		measureId: measure.id,
		reason: input.reason,
		evidenceStagedTokens: input.evidenceStagedTokens,
		etag: measure.etag,
	});
	if (!res.measure) throw new Error("SubmitAppeal 的回應沒有措施");
	return res.measure;
}

/**
 * 以讀到的措施的 etag 請求解除（`RequestRemediation`，只有 RISK 依據的措施）。規則與申覆相同：etag 不符回
 * VERSION_CONFLICT；提出請求不改 etag，重送同一筆回 failed_precondition，都不寫入。
 */
export async function submitMeasureRemediation(
	measure: Pick<SellerMeasure, "id" | "etag">,
	input: SubmitAppealInput,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerMeasure> {
	const res = await client().requestRemediation({
		measureId: measure.id,
		reason: input.reason,
		evidenceStagedTokens: input.evidenceStagedTokens,
		etag: measure.etag,
	});
	if (!res.measure) throw new Error("RequestRemediation 的回應沒有措施");
	return res.measure;
}

/** 申覆失敗的呈現分類（衝突保留輸入並重新載入、其餘顯示伺服器的文字）。 */
export function appealFailure(e: unknown, defaults: DefaultErrorText) {
	return submitError(e, defaults, SellerAppService.method.submitAppeal);
}

/** 解除請求送出成功的提示（與 seller-app 同字）。 */
export const REMEDIATION_SUBMITTED_NOTICE = "解除請求已送出，請等候審核";

/** 解除請求失敗的呈現分類，規則同申覆。 */
export function remediationFailure(e: unknown, defaults: DefaultErrorText) {
	return submitError(e, defaults, SellerAppService.method.requestRemediation);
}

/** 申覆與解除請求的理由字數上限（契約 `SubmitAppealRequest.reason`、`RequestRemediationRequest.reason`），以 rune 計。 */
export const APPEAL_REASON_MAX_LENGTH = 2000;
/** 佐證圖片的張數上限（契約 `evidence_staged_tokens`）。 */
export const APPEAL_MAX_EVIDENCE = 5;

export interface AppealFieldErrors {
	/** 理由欄的錯誤。 */
	reason?: string;
	/** 整個佐證欄位的錯誤（`evidence_staged_tokens`，沒有索引）。 */
	evidence?: string;
	/** 以送出的 token 清單位置（`evidence_staged_tokens[i]`）指出的單張錯誤。 */
	evidenceByIndex: Record<number, string>;
	/** 不屬於以上欄位的伺服器說明，沒有可標的欄位時顯示。 */
	other?: string;
}

const EVIDENCE_ITEM_FIELD = /^evidence_staged_tokens\[(\d+)\]/;

/**
 * 把伺服器的欄位錯誤（proto 路徑）對到畫面欄位。以精確的 key 判斷：`reason`、無索引的
 * `evidence_staged_tokens`、有索引的 `evidence_staged_tokens[i]`（不用前綴比對，否則整欄與單張會混在一起）。
 */
export function appealFieldErrors(
	fields: Record<string, string>,
	message?: string,
): AppealFieldErrors {
	const result: AppealFieldErrors = { evidenceByIndex: {} };
	let matched = false;
	for (const [field, text] of Object.entries(fields)) {
		const item = EVIDENCE_ITEM_FIELD.exec(field);
		if (field === "reason") {
			result.reason = text;
			matched = true;
		} else if (field === "evidence_staged_tokens") {
			result.evidence = text;
			matched = true;
		} else if (item) {
			result.evidenceByIndex[Number(item[1])] = text;
			matched = true;
		}
	}
	if (!matched) result.other = message ?? Object.values(fields)[0];
	return result;
}

/** 衝突後仍可申覆時的提示（與 seller-app 同字）。 */
export const APPEAL_CONFLICT_NOTICE =
	"這項措施已更新，已載入最新內容，請確認後再送出申覆";
/** 沒有伺服器句子時，申覆不能用的通用說明。 */
export const APPEAL_UNAVAILABLE_NOTICE = "這項措施目前不能提出申覆";
export const APPEAL_RELOAD_FAILED_NOTICE = "無法載入最新版本，請稍後再試。";

export const REMEDIATION_CONFLICT_NOTICE =
	"這項措施已更新，已載入最新內容，請確認後再送出請求";
/** 沒有伺服器句子時，解除請求不能用的通用說明。 */
export const REMEDIATION_UNAVAILABLE_NOTICE = "這項措施目前不能請求解除";

/**
 * 不能做某個動作的原因給使用者看的一句話：依優先序排好的第一筆的 `message.message`（伺服器組好的，與動作被拒絕時錯誤的
 * LocalizedMessage 同一份文字）。沒有原因或沒有句子（空字串也算）回 undefined，由呼叫端用該動作的通用說明。
 */
function firstUnavailableMessage(
	unavailabilities: readonly { message?: { message: string } }[],
): string | undefined {
	const text = unavailabilities[0]?.message?.message;
	return text ? text : undefined;
}

/**
 * 申覆入口為什麼不能用（`canAppeal` 為否時）：伺服器依優先序排好的 `appealUnavailabilities` 的第一筆句子，
 * client 不從 state 或 requests 推導，也不依 `reason` 組字（`reason` 只用來分支）。沒有句子時沒有可顯示的原因。
 */
export function appealBlockedReason(
	m: Pick<SellerMeasure, "appealUnavailabilities">,
): string | undefined {
	return firstUnavailableMessage(m.appealUnavailabilities);
}

/** 這項措施不能（再）申覆時要告訴賣家的原因：第一筆伺服器句子，沒有時用通用說明。 */
export function appealClosedNotice(
	measure: Pick<SellerMeasure, "appealUnavailabilities">,
): string {
	return appealBlockedReason(measure) ?? APPEAL_UNAVAILABLE_NOTICE;
}

/**
 * 解除請求入口為什麼不能用（`canRequestRemediation` 為否時）：伺服器依優先序排好的 `remediationUnavailabilities`
 * 的第一筆句子（冷卻的時間已由伺服器組進句子），client 不自己推導原因、優先序或時間。沒有句子時沒有可顯示的原因。
 */
export function remediationBlockedReason(
	m: Pick<SellerMeasure, "remediationUnavailabilities">,
): string | undefined {
	return firstUnavailableMessage(m.remediationUnavailabilities);
}

/** 解除請求不能（再）提出時要告訴賣家的原因：第一筆伺服器句子，沒有時用通用說明。 */
export function remediationClosedNotice(
	m: Pick<SellerMeasure, "remediationUnavailabilities">,
): string {
	return remediationBlockedReason(m) ?? REMEDIATION_UNAVAILABLE_NOTICE;
}

/** 送出申覆或解除請求失敗後給請求頁的處理結果。 */
export type AppealFailureOutcome =
	| { kind: "closed"; latest: SellerMeasure; notice: string }
	| { kind: "refreshed"; latest: SellerMeasure; notice: string }
	| { kind: "fields"; fields: AppealFieldErrors }
	| { kind: "message"; text: string };

/** 請求種類的差異：失敗分類、伺服器判斷能不能再提出、衝突提示與不能提出時的說明。 */
interface RequestFailureSpec {
	failure: (
		e: unknown,
		defaults: DefaultErrorText,
	) => ReturnType<typeof appealFailure>;
	canSubmit: (m: SellerMeasure) => boolean;
	conflictNotice: string;
	closedNotice: (m: SellerMeasure) => string;
}

const APPEAL_SPEC: RequestFailureSpec = {
	failure: appealFailure,
	canSubmit: (m) => m.canAppeal,
	conflictNotice: APPEAL_CONFLICT_NOTICE,
	closedNotice: appealClosedNotice,
};

const REMEDIATION_SPEC: RequestFailureSpec = {
	failure: remediationFailure,
	canSubmit: (m) => m.canRequestRemediation,
	conflictNotice: REMEDIATION_CONFLICT_NOTICE,
	closedNotice: remediationClosedNotice,
};

/**
 * 失敗的處理：VERSION_CONFLICT（平台改過內容）與 failed_precondition（已有請求或已撤銷；提出請求不改 etag，
 * 重送同一筆也是這個）都重讀這一筆，由伺服器判斷的 `canAppeal`／`canRequestRemediation` 決定：不能提出就依狀態關閉並說明，
 * 仍可提出就保留輸入並載入新的 etag。其餘失敗顯示伺服器的文字。重讀失敗時提示稍後再試。
 */
async function settleRequestFailure(
	spec: RequestFailureSpec,
	e: unknown,
	defaults: DefaultErrorText,
	measureId: string,
	reload: (measureId: string) => Promise<SellerMeasure>,
): Promise<AppealFailureOutcome> {
	const failure = spec.failure(e, defaults);
	const failedPrecondition =
		ConnectError.from(e).code === Code.FailedPrecondition;
	if (failure.kind !== "conflict" && !failedPrecondition) {
		if (failure.kind === "fields") {
			return {
				kind: "fields",
				fields: appealFieldErrors(failure.fields, failure.message),
			};
		}
		return { kind: "message", text: failure.message };
	}
	let latest: SellerMeasure;
	try {
		latest = await reload(measureId);
	} catch {
		return { kind: "message", text: APPEAL_RELOAD_FAILED_NOTICE };
	}
	if (spec.canSubmit(latest)) {
		return { kind: "refreshed", latest, notice: spec.conflictNotice };
	}
	return { kind: "closed", latest, notice: spec.closedNotice(latest) };
}

export function settleAppealFailure(
	e: unknown,
	defaults: DefaultErrorText,
	measureId: string,
	reload: (measureId: string) => Promise<SellerMeasure> = fetchMyMeasure,
): Promise<AppealFailureOutcome> {
	return settleRequestFailure(APPEAL_SPEC, e, defaults, measureId, reload);
}

export function settleRemediationFailure(
	e: unknown,
	defaults: DefaultErrorText,
	measureId: string,
	reload: (measureId: string) => Promise<SellerMeasure> = fetchMyMeasure,
): Promise<AppealFailureOutcome> {
	return settleRequestFailure(REMEDIATION_SPEC, e, defaults, measureId, reload);
}
