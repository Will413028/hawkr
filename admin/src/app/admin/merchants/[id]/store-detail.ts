import { Code, ConnectError } from "@connectrpc/connect";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	type Closure,
	StoreCloser,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import type { PostalAddress } from "@hawkr/api/gen/hawkr/common/v1alpha1/postal_address_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { StoreStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { formatAddress } from "@hawkr/twaddr/lookup";

// 商家詳情（SellerAdminService.GetStore）的顯示判斷與文案。畫面只管呈現；哪個操作在哪個狀態可用、停權原因怎麼
// 顯示、違規停權去哪裡、輸入怎麼驗證都在這裡，才能不靠畫面測試（決策簡報 K10、D4 第 2、3、13 點）。

// ---------------------------------------------------------------------------
// D4 的文案（已核准，照原文；標註「實作者補」的是 D4 沒有原文的部分）

export const SUSPEND_WARNING_TEXT =
	"停權後買家看不到這家店與它的商品，未付款訂單會被取消，優惠券會停用。";
/** 實作者補：建立風險措施成功後的頁首提示；後半句沿用 D4 第 2 點停權成功的原文。 */
export const MEASURE_CREATED_TEXT =
	"已建立風險措施，賣家已停權。買家已看不到這家店；未付款訂單與優惠券會陸續取消、停用。";
/** 實作者補：建立措施（違規依據的警告或停權）成功後的頁首提示。 */
export const VIOLATION_MEASURE_CREATED_TEXT = "已建立措施。";
export const LIFT_SUCCESS_TEXT =
	"已解除停權。店家符合公開條件時商品會恢復顯示，優惠券會自動恢復；已取消的訂單不會恢復。";
/** 實作者補：結束措施對話框的說明；第一句與 LIFT_SUCCESS_TEXT 的內容一致（沿用 D4 第 2 點），第二句因為帳號可以同時有多個生效中的措施而補。 */
export const END_HINT_TEXT =
	"結束後店家符合公開條件時商品會恢復顯示，優惠券會自動恢復；已取消的訂單不會恢復。帳號還有其他生效中的措施時，要全部結束或撤銷後才會恢復公開。";
export const DELETE_ACCOUNT_WARNING_TEXT =
	"刪除整個帳號（買家與店家）。沒有未完成的訂單時立即刪除，否則等最後一筆訂單完成後自動刪除。無法復原。";

/** 刪除帳號的結果（DeleteAccount）。UNSPECIFIED 不是結果，回 undefined，畫面不能宣告成功。 */
export function deletionOutcome(
	state: DeleteAccountResponse_DeletionState,
): { title: string; text: string } | undefined {
	switch (state) {
		case DeleteAccountResponse_DeletionState.DELETED:
			return {
				title: "已刪除這個帳號",
				text: "帳號與其賣家、買家角色都已移除，這一頁的資料不再有效。",
			};
		case DeleteAccountResponse_DeletionState.PENDING_ORDERS:
			return {
				title: "刪除申請已受理",
				text: "已受理。帳號有未完成的訂單，完成後會自動刪除；期間賣家只能處理訂單與售後。",
			};
		default:
			return undefined;
	}
}

// ---------------------------------------------------------------------------
// 生效中的措施與狀態卡

/** 結束措施後的成功訊息：重讀後帳號仍有生效中的措施就照實說；沒有時用 D4 第 2 點的原文。 */
export const END_SUCCESS_STILL_SUSPENDED_TEXT =
	"已結束這個措施。帳號仍有其他生效中的措施，要全部結束或撤銷後才會恢復公開。";

/** 結束後重讀失敗時的成功訊息：不斷言仍有或沒有其他生效中的措施（D4 第 2 點的原文只在確認沒有其他措施時用）。 */
export const END_SUCCESS_NEUTRAL_TEXT = "已結束措施。";

export function endSuccessText(remaining: readonly Measure[]): string {
	return remaining.length > 0
		? END_SUCCESS_STILL_SUSPENDED_TEXT
		: LIFT_SUCCESS_TEXT;
}

// ---------------------------------------------------------------------------
// 店家的生命週期（呈現）。動作能不能做與不能的原因都來自伺服器（GetStore 的 can_*、*_unavailabilities），不在這裡推導。

/** 列表與詳情頁共用的生命週期徽章；營運中不另外顯示。 */
export function lifecycleBadge(
	status: StoreStatus | undefined,
): { label: string; variant: "gray" | "warning" } | undefined {
	switch (status) {
		case StoreStatus.CLOSED:
			return { label: "已關閉", variant: "gray" };
		case StoreStatus.PENDING_DELETION:
			return { label: "等待刪除", variant: "warning" };
		default:
			return undefined;
	}
}

/** 關店的原因。closer 是 TERMINATION 時帶造成關店的終止合作措施 id（沒有 id 的 TERMINATION 不是有效資料，回 `-`）。 */
export function closerLabel(closure: Closure | undefined): string {
	if (!closure) return "-";
	switch (closure.closer) {
		case StoreCloser.ACCOUNT_OWNER:
			return "賣家自己關店";
		case StoreCloser.TERMINATION:
			return closure.measureId ? `終止合作（措施 ${closure.measureId}）` : "-";
		case StoreCloser.ACCOUNT_DELETION:
			return "隨帳號刪除";
		default:
			return "-";
	}
}

/** 地址：縣市＋鄉鎮市區＋明細（不認得的代碼只回明細）；沒有地址回 `-`。 */
export function addressText(address: PostalAddress | undefined): string {
	if (!address) return "-";
	return formatAddress(address.townCode, address.addressDetail) || "-";
}

// ---------------------------------------------------------------------------
// 輸入驗證（刪帳號與顯示全文的理由上限 1000 字；措施的欄位上限在 measure.ts）

export const REASON_MAX_LENGTH = 1000;

/** 刪帳號與顯示全文的理由：必填（去掉前後空白後非空），最多 1000 字。回傳錯誤文字，沒有錯誤回 undefined。 */
export function validateReason(
	raw: string,
	emptyMessage: string,
): string | undefined {
	const trimmed = trimGoSpace(raw);
	if (trimmed.length === 0) return emptyMessage;
	if (goTrimmedRuneCount(trimmed) > REASON_MAX_LENGTH) {
		return `理由最多 ${REASON_MAX_LENGTH} 字。`;
	}
	return undefined;
}

// ---------------------------------------------------------------------------
// 登入信箱

/**
 * 登入信箱（IdentityAdminService.GetAccount）的讀取狀態。notFound 是帳號不存在或已刪除；ready 且 email 為空字串時，
 * 畫面不顯示該列（K6）。
 */
export type AccountEmail =
	| { state: "loading" }
	| { state: "error" }
	| { state: "notFound" }
	| { state: "ready"; email: string };

/**
 * 把 GetAccount 的讀取結果轉成信箱狀態。查不到帳號（不存在或已刪除）是 notFound；其他錯誤，以及成功卻沒有帳號，都是 error
 * （可重試），不當成空信箱。信箱本身是空字串時才是 ready 且 email 為空（畫面不顯示該列）。
 */
export function accountEmailOf(result: {
	error: unknown;
	data: { account?: { email: string } } | undefined;
}): AccountEmail {
	if (result.error != null) {
		return result.error instanceof ConnectError &&
			result.error.code === Code.NotFound
			? { state: "notFound" }
			: { state: "error" };
	}
	if (result.data === undefined) return { state: "loading" };
	if (!result.data.account) return { state: "error" };
	return { state: "ready", email: result.data.account.email };
}
