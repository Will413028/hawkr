/**
 * 賣家的平台詢問（`SellerAppService.ListMyInquiries`、`GetMyInquiry`、`ReplyToInquiry`）。賣家路徑直接使用生成的
 * `Inquiry`：能不能回覆讀 `canReply`、附件上限讀 `replyAttachmentMax`、逾期只是顯示標記（逾期後仍可回覆），client 不推導
 * 能不能回覆。附件是機敏檔，賣家只看得到數量。名稱與句子在呈現層（`features/seller/inquiries/inquiryDisplay.ts`）。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import { type DefaultErrorText, errorText } from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import {
	type Inquiry,
	InquiryState,
	SellerAppService,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerAppClient } from "@/lib/connect/seller";

export type SellerInquiry = Inquiry;

type SellerClient = ReturnType<typeof sellerAppClient>;

/** 一頁最多 100 筆（契約上限）。 */
const INQUIRIES_PAGE_SIZE = 100;

/** 回覆內文的字數上限（契約 `ReplyToInquiryRequest.body`），以 rune 計。 */
export const INQUIRY_REPLY_MAX_LENGTH = 4000;

export interface SellerInquiryList {
	inquiries: SellerInquiry[];
	/** 帳號全部開放中的詢問數，不隨 `state` 篩選變動。 */
	pendingReplyCount: number;
}

/** 讀完詢問的全部分頁；`state` 是 `UNSPECIFIED`（預設）時不篩。 */
export async function fetchMyInquiries(
	state: InquiryState = InquiryState.UNSPECIFIED,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerInquiryList> {
	const inquiries: SellerInquiry[] = [];
	let pendingReplyCount = 0;
	let pageToken = "";
	do {
		const res = await client().listMyInquiries({
			pageSize: INQUIRIES_PAGE_SIZE,
			pageToken,
			state,
		});
		inquiries.push(...res.inquiries);
		pendingReplyCount = res.pendingReplyCount;
		if (res.nextPageToken !== "" && res.nextPageToken === pageToken) {
			throw new Error("ListMyInquiries 的 next_page_token 沒有前進");
		}
		pageToken = res.nextPageToken;
	} while (pageToken !== "");
	return { inquiries, pendingReplyCount };
}

/** 導覽徽章只要待回覆數：一筆就夠。 */
export async function fetchPendingInquiryCount(
	client: () => SellerClient = sellerAppClient,
): Promise<number> {
	const res = await client().listMyInquiries({
		pageSize: 1,
		state: InquiryState.OPEN,
	});
	return res.pendingReplyCount;
}

export async function fetchMyInquiry(
	inquiryId: string,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerInquiry> {
	const res = await client().getMyInquiry({ inquiryId });
	if (!res.inquiry) throw new Error("GetMyInquiry 的回應沒有詢問");
	return res.inquiry;
}

export interface ReplyInput {
	body: string;
	/** 機敏上傳（`uploadSecureDocument`）回的 staged token。 */
	evidenceStagedTokens: string[];
}

export async function submitInquiryReply(
	inquiryId: string,
	input: ReplyInput,
	client: () => SellerClient = sellerAppClient,
): Promise<SellerInquiry> {
	const res = await client().replyToInquiry({
		inquiryId,
		body: input.body,
		evidenceStagedTokens: input.evidenceStagedTokens,
	});
	if (!res.inquiry) throw new Error("ReplyToInquiry 的回應沒有詢問");
	return res.inquiry;
}

export interface ReplyFieldErrors {
	body?: string;
	/** 整個附件欄位的錯誤（`evidence_staged_tokens`，沒有索引）。 */
	evidence?: string;
	/** 以送出的 token 清單位置（`evidence_staged_tokens[i]`）指出的單張錯誤。 */
	evidenceByIndex: Record<number, string>;
	/** 不屬於以上欄位的伺服器說明，沒有可標的欄位時顯示。 */
	other?: string;
}

const EVIDENCE_ITEM_FIELD = /^evidence_staged_tokens\[(\d+)\]/;

/** 把伺服器的欄位錯誤（proto 路徑）對到畫面欄位，以精確的 key 判斷。 */
export function replyFieldErrors(
	fields: Record<string, string>,
	message?: string,
): ReplyFieldErrors {
	const result: ReplyFieldErrors = { evidenceByIndex: {} };
	let matched = false;
	for (const [field, text] of Object.entries(fields)) {
		const item = EVIDENCE_ITEM_FIELD.exec(field);
		if (field === "body") {
			result.body = text;
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

export type ReplyFailureOutcome =
	| { kind: "closed"; latest: SellerInquiry }
	| { kind: "fields"; fields: ReplyFieldErrors }
	| { kind: "message"; text: string };

/**
 * 回覆失敗的處理：failed_precondition（已回覆、已撤回，或另一個分頁剛回覆過）重讀這一筆，由伺服器的 `canReply`
 * 決定：不能回覆就關閉表單改顯示最新狀態，仍可回覆就當一般訊息。重讀失敗時只顯示原本的伺服器文字。
 * 欄位錯誤與其餘失敗顯示伺服器的文字。
 */
export async function settleReplyFailure(
	e: unknown,
	defaults: DefaultErrorText,
	inquiryId: string,
	reload: (inquiryId: string) => Promise<SellerInquiry> = fetchMyInquiry,
): Promise<ReplyFailureOutcome> {
	const failure = submitError(
		e,
		defaults,
		SellerAppService.method.replyToInquiry,
	);
	if (ConnectError.from(e).code === Code.FailedPrecondition) {
		try {
			const latest = await reload(inquiryId);
			if (!latest.canReply) return { kind: "closed", latest };
		} catch {
		}
	}
	if (failure.kind === "fields") {
		return {
			kind: "fields",
			fields: replyFieldErrors(failure.fields, failure.message),
		};
	}
	return { kind: "message", text: errorText(e, defaults) };
}
