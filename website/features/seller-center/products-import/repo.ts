/**
 * 商品匯入精靈 — 17 個 endpoint 的 client adapter。
 *
 * 對應 backend 40 號設計書 §4 + gateway handler json tag。
 *
 * 命名規則：
 *   - request / response JSON：camelCase（backend/docs/開發規範/API 設計規範.md + backend json tag）
 *   - URL query params：snake_case（API 設計規範 + gateway 用 c.Query("page_size") 等）
 */

import { ApiClientError, apiClient } from "@/lib/api/client";
import type {
	BatchUpdateCategoryInput,
	BatchUpdateShippingInput,
	BatchUpdateTargetStatusInput,
	CategoryConfirmationToken,
	CommitImportJobInput,
	CommitImportJobResponse,
	CreateImportJobResponse,
	ImportItemBrief,
	ImportItemDetail,
	ImportItemPreview,
	ImportJobBrief,
	ImportJobDetail,
	ImportPreviewListing,
	ListImportItemsParams,
	ListImportJobsParams,
	ListPreviewParams,
	PageResult,
	UpdateImportItemInput,
	ValidationError,
} from "./types";

// Re-export so callers can `import { CreateImportJobResponse } from "../repo"`.
export type { CreateImportJobResponse } from "./types";

const BASE = "/app/product_import_jobs";

function flattenStatus(status?: string[]): Record<string, string> | undefined {
	if (!status || status.length === 0) return undefined;
	return { status: status.join(",") };
}

export function normalizeImportPageResult<T>(
	value: unknown,
	endpointLabel: string,
): PageResult<T> {
	if (
		!value ||
		typeof value !== "object" ||
		!Array.isArray((value as { records?: unknown }).records)
	) {
		throw new Error(`${endpointLabel}回應格式不正確，請稍後再試`);
	}
	const result = value as Partial<PageResult<T>> & { records: T[] };
	return {
		records: result.records,
		total: result.total ?? result.records.length,
		page: result.page ?? 1,
		pageSize: result.pageSize ?? result.records.length,
	};
}

/* ===== Endpoint #1: CreateImportJob (multipart) ===== */
export interface CreateImportJobOptions {
	file: File;
	/** form field name 對齊 backend multipart 解析（snake_case，與 query 同規則）。 */
	targetStatus?: string;
	/** 對應 backend allow_duplicate_filename — 強制覆寫同名 active job 衝突。 */
	allowDuplicateFilename?: boolean;
}

export class L1ValidationError extends ApiClientError {
	public validationErrors: ValidationError[];
	constructor(message: string, status: number, errors: ValidationError[]) {
		super(message, status, undefined, { validationErrors: errors });
		this.name = "L1ValidationError";
		this.validationErrors = errors;
	}
}

export async function createImportJob(
	opts: CreateImportJobOptions,
): Promise<CreateImportJobResponse> {
	const form = new FormData();
	form.append("file", opts.file);
	if (opts.targetStatus) form.append("target_status", opts.targetStatus);
	if (opts.allowDuplicateFilename) {
		form.append("allow_duplicate_filename", "true");
	}
	try {
		const env = await apiClient.authed.post<CreateImportJobResponse>(BASE, {
			body: form,
			throwOnHttpError: true,
			throwOnEnvelopeError: true,
		});
		return env.data;
	} catch (e) {
		if (e instanceof ApiClientError && e.status === 400) {
			const payload = (e.payload ?? {}) as {
				validationErrors?: ValidationError[];
			};
			throw new L1ValidationError(
				e.message,
				e.status,
				payload.validationErrors ?? [],
			);
		}
		throw e;
	}
}

/* ===== Endpoint #2: ReUploadCsv (PUT multipart) ===== */
export async function reuploadImportCsv(
	jobId: string,
	file: File,
): Promise<CreateImportJobResponse> {
	const form = new FormData();
	form.append("file", file);
	try {
		const env = await apiClient.authed.put<CreateImportJobResponse>(
			`${BASE}/${encodeURIComponent(jobId)}/csv`,
			{
				body: form,
				throwOnHttpError: true,
				throwOnEnvelopeError: true,
			},
		);
		return env.data;
	} catch (e) {
		if (e instanceof ApiClientError && e.status === 400) {
			const payload = (e.payload ?? {}) as {
				validationErrors?: ValidationError[];
			};
			throw new L1ValidationError(
				e.message,
				e.status,
				payload.validationErrors ?? [],
			);
		}
		throw e;
	}
}

/* ===== Endpoint #3: ListImportJobs ===== */
export async function listImportJobs(
	params: ListImportJobsParams = {},
): Promise<PageResult<ImportJobBrief>> {
	const env = await apiClient.authed.get<PageResult<ImportJobBrief>>(BASE, {
		query: {
			...flattenStatus(params.status),
			page: params.page ?? 1,
			page_size: params.pageSize ?? 20,
		},
	});
	return normalizeImportPageResult<ImportJobBrief>(env.data, "匯入任務");
}

/* ===== Endpoint #4: GetImportJob ===== */
export async function getImportJob(jobId: string): Promise<ImportJobDetail> {
	const env = await apiClient.authed.get<ImportJobDetail>(
		`${BASE}/${encodeURIComponent(jobId)}`,
	);
	return env.data;
}

/* ===== Endpoint #5: GetImportJobPreviewListing ===== */
export async function getImportJobPreviewListing(
	jobId: string,
	params: ListPreviewParams = {},
): Promise<ImportPreviewListing> {
	const env = await apiClient.authed.get<ImportPreviewListing>(
		`${BASE}/${encodeURIComponent(jobId)}/preview_listing`,
		{
			query: {
				include_pending: params.includePending ? "true" : undefined,
				page: params.page ?? 1,
				page_size: params.pageSize ?? 24,
			},
		},
	);
	return env.data;
}

/* ===== Endpoint #6: CancelImportJob ===== */
export async function cancelImportJob(jobId: string): Promise<void> {
	await apiClient.authed.post(`${BASE}/${encodeURIComponent(jobId)}/cancel`);
}

/* ===== Endpoint #7: CommitImportJob ===== */
export async function commitImportJob(
	jobId: string,
	input: CommitImportJobInput = {},
): Promise<CommitImportJobResponse> {
	const env = await apiClient.authed.post<CommitImportJobResponse>(
		`${BASE}/${encodeURIComponent(jobId)}/commit`,
		{
			json: input,
		},
	);
	return env.data;
}

/* ===== Endpoint #8: ListImportItems ===== */
export async function listImportItems(
	jobId: string,
	params: ListImportItemsParams = {},
): Promise<PageResult<ImportItemBrief>> {
	const env = await apiClient.authed.get<PageResult<ImportItemBrief>>(
		`${BASE}/${encodeURIComponent(jobId)}/items`,
		{
			query: {
				...flattenStatus(params.status),
				only_with_errors: params.onlyWithErrors ? "true" : undefined,
				include_removed: params.includeRemoved ? "true" : undefined,
				page: params.page ?? 1,
				page_size: params.pageSize ?? 50,
			},
		},
	);
	return normalizeImportPageResult<ImportItemBrief>(env.data, "匯入商品");
}

/* ===== Endpoint #9: GetImportItem ===== */
export async function getImportItem(
	jobId: string,
	itemId: string,
): Promise<ImportItemDetail> {
	const env = await apiClient.authed.get<ImportItemDetail>(
		`${BASE}/${encodeURIComponent(jobId)}/items/${encodeURIComponent(itemId)}`,
	);
	return env.data;
}

/* ===== Endpoint #10: GetImportItemPreview ===== */
export async function getImportItemPreview(
	jobId: string,
	itemId: string,
): Promise<ImportItemPreview> {
	const env = await apiClient.authed.get<ImportItemPreview>(
		`${BASE}/${encodeURIComponent(jobId)}/items/${encodeURIComponent(
			itemId,
		)}/preview`,
	);
	return env.data;
}

/* ===== Endpoint #11: UpdateImportItem ===== */
export async function updateImportItem(
	jobId: string,
	itemId: string,
	input: UpdateImportItemInput,
): Promise<ImportItemDetail> {
	const env = await apiClient.authed.put<ImportItemDetail>(
		`${BASE}/${encodeURIComponent(jobId)}/items/${encodeURIComponent(itemId)}`,
		{ json: input },
	);
	return env.data;
}

/* ===== Endpoint #12: RemoveImportItems ===== */
export async function removeImportItems(
	jobId: string,
	itemIds: string[],
): Promise<{ affectedCount: string }> {
	const env = await apiClient.authed.post<{ affectedCount: string }>(
		`${BASE}/${encodeURIComponent(jobId)}/items/remove`,
		{ json: { importItemIds: itemIds } },
	);
	return env.data;
}

/* ===== Endpoint #13: RestoreImportItems ===== */
export async function restoreImportItems(
	jobId: string,
	itemIds: string[],
): Promise<{ affectedCount: string }> {
	const env = await apiClient.authed.post<{ affectedCount: string }>(
		`${BASE}/${encodeURIComponent(jobId)}/items/restore`,
		{ json: { importItemIds: itemIds } },
	);
	return env.data;
}

/* ===== Endpoint #14: RetryAssets ===== */
export async function retryItemAssets(
	jobId: string,
	itemId: string,
	assetIds?: string[],
): Promise<{ queuedCount: string }> {
	const env = await apiClient.authed.post<{ queuedCount: string }>(
		`${BASE}/${encodeURIComponent(jobId)}/items/retry_assets`,
		{
			json: {
				importItemId: itemId,
				assetIds: assetIds,
			},
		},
	);
	return env.data;
}

/** 對「job 內所有有失敗 asset 的 item」一次性重試（前端封裝；後端一筆一筆送）。 */
export async function retryAllFailedAssets(
	jobId: string,
	failedItemIds: string[],
): Promise<{ queuedCount: string }> {
	let total = 0;
	for (const itemId of failedItemIds) {
		const r = await retryItemAssets(jobId, itemId);
		total += Number(r.queuedCount) || 0;
	}
	return { queuedCount: String(total) };
}

/* ===== Endpoint #15: BatchUpdateShipping ===== */
export async function batchUpdateItemsShipping(
	jobId: string,
	input: BatchUpdateShippingInput,
): Promise<{ affectedItems: string; affectedSkus: string }> {
	const env = await apiClient.authed.post<{
		affectedItems: string;
		affectedSkus: string;
	}>(`${BASE}/${encodeURIComponent(jobId)}/items/batch_update_shipping`, {
		json: input,
	});
	return env.data;
}

/* ===== Endpoint #16: BatchUpdateTargetStatus ===== */
export async function batchUpdateItemsTargetStatus(
	jobId: string,
	input: BatchUpdateTargetStatusInput,
): Promise<{ affectedItems: string }> {
	const env = await apiClient.authed.post<{ affectedItems: string }>(
		`${BASE}/${encodeURIComponent(jobId)}/items/batch_update_target_status`,
		{ json: input },
	);
	return env.data;
}

/* ===== Endpoint #17: BatchUpdateCategory ===== */
export async function batchUpdateItemsCategory(
	jobId: string,
	input: BatchUpdateCategoryInput,
): Promise<{
	affectedItems: string;
	catL1Id: string;
	catL1Name: string;
	catL2Id: string;
	catL2Name: string;
	catL3Id: string;
	catL3Name: string;
}> {
	const env = await apiClient.authed.post<{
		affectedItems: string;
		catL1Id: string;
		catL1Name: string;
		catL2Id: string;
		catL2Name: string;
		catL3Id: string;
		catL3Name: string;
	}>(`${BASE}/${encodeURIComponent(jobId)}/items/batch_update_category`, {
		json: input,
	});
	return env.data;
}

/* ===== Endpoint #18: GetCategoryConfirmationToken ===== */
export async function getCategoryConfirmationToken(
	jobId: string,
	categoryId: string,
): Promise<CategoryConfirmationToken> {
	const env = await apiClient.authed.get<CategoryConfirmationToken>(
		`${BASE}/${encodeURIComponent(jobId)}/category_confirmation_tokens`,
		{ query: { category_id: categoryId } },
	);
	return env.data;
}
