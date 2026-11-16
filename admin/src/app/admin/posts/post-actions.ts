import type { MessageInitShape } from "@bufbuild/protobuf";
import type { CreateMeasureRequestSchema } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { AdminPost } from "@hawkr/api/gen/hawkr/admin/social/v1alpha1/social_pb";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ContentKind,
	type ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import {
	NOTE_MAX_LENGTH,
	STATEMENT_MAX_LENGTH,
} from "@/app/admin/merchants/[id]/measure";
import { VIOLATION_SEVERITY_OPTIONS } from "@/app/admin/merchants/[id]/violation-measure";
import { CONTENT_TAKEDOWN_CATEGORIES } from "@/lib/policy-category";

export const POSTS_PAGE_SIZE = 20;

/** ListPosts 的請求：page token 綁定篩選條件，換條件時呼叫端回到第一頁。 */
export function listPostsRequest(takenDownOnly: boolean, pageToken: string) {
	return { pageSize: POSTS_PAGE_SIZE, pageToken, takenDownOnly };
}

/** 貼文是否已被內容下架措施下架（恢復要撤銷這個措施）。 */
export function takedownMeasureOf(post: AdminPost): string | undefined {
	return post.takedownMeasureId || undefined;
}

export interface TakedownForm {
	category: PolicyCategory;
	severity: ViolationSeverity;
	/** 給賣家的理由。 */
	statement: string;
	/** 第一則內部註記（賣家看不到）。 */
	note: string;
}

/** 送出前的檢查，與伺服器的規則一致（CreateMeasureRequest）；回傳第一個錯誤，沒有錯誤回 undefined。 */
export function validateTakedown(form: TakedownForm): string | undefined {
	if (!CONTENT_TAKEDOWN_CATEGORIES.includes(form.category)) {
		return "請選擇類別。";
	}
	if (!VIOLATION_SEVERITY_OPTIONS.includes(form.severity)) {
		return "請選擇嚴重度。";
	}
	const statement = trimGoSpace(form.statement);
	if (statement.length === 0) return "請填寫給賣家的理由。";
	if (goTrimmedRuneCount(statement) > STATEMENT_MAX_LENGTH) {
		return `給賣家的理由最多 ${STATEMENT_MAX_LENGTH} 字。`;
	}
	const note = trimGoSpace(form.note);
	if (note.length === 0) return "請填寫內部註記。";
	if (goTrimmedRuneCount(note) > NOTE_MAX_LENGTH) {
		return `內部註記最多 ${NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

/** 輸入內容的鍵，與 request_id 一起用（merchants/[id]/measure.ts 的 requestIdFor）。 */
export function takedownRequestKey(
	post: AdminPost,
	form: TakedownForm,
): string {
	return JSON.stringify([
		post.storeId,
		post.id,
		form.category,
		form.severity,
		trimGoSpace(form.statement),
		trimGoSpace(form.note),
	]);
}

/**
 * CreateMeasure 的請求：以這則貼文為對象的內容下架措施，依據是一筆違規認定。呼叫前須先通過 validateTakedown。
 */
export function buildTakedown(
	requestId: string,
	post: AdminPost,
	form: TakedownForm,
): MessageInitShape<typeof CreateMeasureRequestSchema> {
	const invalid = validateTakedown(form);
	if (invalid) throw new Error(invalid);
	return {
		requestId,
		storeId: post.storeId,
		statement: trimGoSpace(form.statement),
		note: trimGoSpace(form.note),
		measure: {
			case: "contentTakedown",
			value: {
				contents: [{ kind: ContentKind.POST, id: post.id }],
				violation: {
					finding: {
						category: form.category,
						severity: form.severity,
						sourceReportIds: [],
						sourceAuthorityOrderIds: [],
					},
					citedFindingIds: [],
				},
			},
		},
	};
}
