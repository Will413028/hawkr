import type { DelegationAuthorizationSource } from "./authorization";
import type {
	DelegatedAuditResourceType,
	DelegatedAuditResult,
	DelegatedAuditRiskLevel,
	DelegatedSessionStatus,
} from "./governance";
import type {
	DelegatedOperationCapability,
	DelegatedOperationState,
} from "./model";

export const DELEGATED_OPERATION_CAPABILITY_LABELS: Record<
	DelegatedOperationCapability,
	string
> = {
	"storefront.read": "查看店面內容",
	"storefront.write": "編輯店面內容",
	"products.read": "查看商品",
	"products.write": "編輯商品",
	"products.publish": "上架與下架商品",
	"posts.read": "查看貼文",
	"posts.write": "編輯貼文",
	"posts.publish": "發布與下架貼文",
	"orders.read": "查看訂單（唯讀）",
};

/**
 * 授權設定畫面的能力顯示名。
 *
 * 能力目錄由後端下發，而**目錄可能比本檔的封閉九值新**（後端加了一個能力、ADM 還
 * 沒跟上）。那時退回機器名而不是畫一個空白：讀的人至少看得出是哪一項，也看得出
 * ADM 該更新了。`DELEGATED_OPERATION_CAPABILITY_LABELS` 本身不改成 `Record<string>`
 * ——它在 preflight 那一側是被封閉型別保護的，放寬會讓漏翻譯不再是型別錯。
 */
export function delegationCapabilityLabel(value: string): string {
	return (
		DELEGATED_OPERATION_CAPABILITY_LABELS[
			value as DelegatedOperationCapability
		] ?? value
	);
}

/**
 * `chk_sda_source` 三值的顯示名。
 *
 * 這一組沒有後端下發的 label（授權列的 DTO 只回機器名），與稽核列的 `actionLabel`
 * 不同——那裡有權威中文，所以前端不得自建；這裡沒有，值域又是 DB CHECK 封閉的三值。
 * 若哪天後端開始下發，改讀它並刪掉本表。
 */
export const DELEGATION_AUTHORIZATION_SOURCE_LABELS: Record<
	DelegationAuthorizationSource,
	string
> = {
	contract: "合作合約",
	ticket: "工單",
	platform_policy: "平台政策",
};

export const DELEGATION_AUTHORIZATION_SOURCE_HINTS: Record<
	DelegationAuthorizationSource,
	string
> = {
	contract: "填合約編號",
	ticket: "填工單或案件編號",
	platform_policy: "填政策文號",
};

export const DELEGATED_OPERATION_PROHIBITED_LABELS = [
	"備貨、出貨與物流狀態",
	"取消、退款與售後決策",
	"付款、結算、銀行與財務資料",
	"身分、認證、密碼與帳號安全",
] as const;

/**
 * 稽核列的三個封閉值域各自的顯示文字。
 *
 * ⚠️ 這裡**沒有** `action → 中文` 的對照表，那一份由後端逐列下發
 * （`actionLabel`／`changedFields[].label`）：前端自己建一份，等於新增一個動作要
 * 改兩個 repo，而漏改的那一邊會在稽核紀錄上顯示英文常數。
 *
 * ⚠️ 唯一例外在別的檔案、界線也不同：`src/lib/codemeta/generated.ts` 的
 * `DELEGATION_AUDIT_ACTION_LABEL` 是 backend CodeMeta 的生成物，**只供動作篩選下拉在資料
 * 到達前列出選項**（那時 `actionLabel` 還不存在）。它不參與任何一列紀錄的顯示，
 * 所以上面那句「漏改會讓稽核紀錄顯示英文常數」對它不成立。要加 action → 中文，
 * 是回 backend 改 `.md` 再重新生成覆蓋（那個檔 DO NOT EDIT），不要加回本檔。
 *
 * ⚠️ `PENDING` 刻意既非成功也非失敗樣式。把它併進成功，操作者就再也看不到
 * 「結果未確認」，而那正是他最該去對帳的那一列。
 */
export const DELEGATED_AUDIT_RESULT_LABELS: Record<
	DelegatedAuditResult,
	string
> = {
	SUCCEEDED: "成功",
	FAILED: "失敗",
	PENDING: "結果未確認",
};

export const DELEGATED_AUDIT_RESOURCE_TYPE_LABELS: Record<
	DelegatedAuditResourceType,
	string
> = {
	storefront: "店面",
	product: "商品",
	post: "內容",
	order: "訂單",
};

/**
 * 稽核列「目標資源」那一格的顯示值。
 *
 * `resourceName` 是快照名，**不保證有值**：訂單面的 `resource_id` 是 bigint 而訂單
 * 編號是 `O` ＋ 序號（後端存 NULL），而遭拒的讀取列連 name 都還沒讀到就結束了。
 * 這種時候只畫一個「—」等於宣稱「這一列沒有目標」，但 API 明明回了 `resourceId`
 * ——稽核列指不回對象，它就不再是舉證。
 *
 * 兩處消費（列表列與詳情面板）共用這一支：分開寫必然漂移成兩種說法，而讀的人
 * 會以為那是兩筆不同的紀錄。
 */
export function delegatedAuditResourceDisplay(entry: {
	resourceName: string;
	resourceId: string;
}): string {
	if (entry.resourceName) return entry.resourceName;
	// 加 `#` 是為了讓人一眼看出這是識別碼而不是一個叫這串數字的資源。
	if (entry.resourceId) return `#${entry.resourceId}`;
	return "—";
}

export const DELEGATED_AUDIT_RISK_LABELS: Record<
	DelegatedAuditRiskLevel,
	string
> = {
	low: "一般",
	medium: "需留意",
	high: "風險操作",
};

export const DELEGATED_SESSION_STATUS_LABELS: Record<
	DelegatedSessionStatus,
	string
> = {
	active: "進行中",
	expired: "已結束",
	revoked: "已撤銷",
};

export function delegatedOperationStateCopy(state: DelegatedOperationState): {
	label: string;
	title: string;
	description: string;
} {
	switch (state) {
		case "active":
			return {
				label: "ADM 代行中",
				title: "代行工作區",
				description: "工作階段有效，所有動作會記錄在目前的 ADM 身分下。",
			};
		case "expiring":
			return {
				label: "代行即將到期",
				title: "工作階段即將到期",
				description: "請先完成並儲存目前工作；到期後會立即停止所有代行操作。",
			};
		case "expired":
			return {
				label: "代行已到期",
				title: "代行工作階段已到期",
				description:
					"已停止所有代行操作。請返回賣家資料後重新確認資格，再建立新的工作階段。",
			};
		case "revoked":
			return {
				label: "代行已撤銷",
				title: "代行權限已被撤銷",
				description:
					"伺服器已撤銷此工作階段，所有代行操作都已停止。請返回賣家資料確認原因。",
			};
		case "end_unknown":
			return {
				label: "結束結果待確認",
				title: "工作階段結束結果尚未確認",
				description:
					"為避免在狀態不明時繼續操作，已停止所有代行寫入。請重試結束，或返回賣家資料。",
			};
		case "target_mismatch":
			return {
				label: "目標不相符",
				title: "工作階段與目前店家不相符",
				description:
					"為避免操作錯誤店家，已停止所有代行操作。請返回賣家列表重新選擇目標。",
			};
		default:
			return {
				label: "代行不可用",
				title: "目前無法確認代行工作階段",
				description:
					"工作階段資料不完整或服務暫時無法使用，已停止所有代行操作。請稍後重試。",
			};
	}
}
