import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	DelegatedOperationSchema,
	type Post,
	PostSchema,
	PostStatus,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { imageUri } from "@/lib/media/image";
import { DelegatedRequestRejectedError } from "../_csrf";
import type { DelegatedConnectInput } from "../_forward";

/**
 * 代行貼文 BFF 的瀏覽器面形狀（`lib/seller-data/delegated-repos.ts` 讀的那一份）與 `SocialDelegatedService`
 * 訊息之間的轉換。這裡只組訊息、整形回應，不打上游（上游轉送點是 `_upstream.ts`）。
 */

type Wire = Record<string, unknown>;

function bad(code: "INVALID_BODY" | "INVALID_QUERY" = "INVALID_BODY"): never {
	throw new DelegatedRequestRejectedError(code, 400);
}

function optionalString(value: unknown): string | undefined {
	if (value === undefined || value === null) return undefined;
	if (typeof value !== "string") return bad();
	return value;
}

function stringList(value: unknown): string[] | undefined {
	if (value === undefined || value === null) return undefined;
	if (!Array.isArray(value) || value.some((v) => typeof v !== "string")) {
		return bad();
	}
	return value as string[];
}

/**
 * 每個寫入共同的稽核資料：request_id 來自瀏覽器的 `Idempotency-Key`（必填，缺了是 400，
 * BFF 不代為生成）、操作原因必填（伺服器驗長度）、`notifySeller` 不設是通知。
 */
export function operationOf(input: DelegatedConnectInput, body: Wire) {
	if (input.requestId === null) {
		throw new DelegatedRequestRejectedError("INVALID_IDEMPOTENCY_KEY", 400);
	}
	const reason = body.operationReason;
	if (typeof reason !== "string") return bad();
	const notify = body.notifySeller;
	if (notify !== undefined && typeof notify !== "boolean") return bad();
	return create(DelegatedOperationSchema, {
		requestId: input.requestId,
		operationReason: reason,
		...(notify === undefined ? {} : { notifySeller: notify }),
	});
}

export function bodyRecord(input: DelegatedConnectInput): Wire {
	if (input.json === undefined) return bad();
	if (
		!input.json ||
		typeof input.json !== "object" ||
		Array.isArray(input.json)
	) {
		return bad();
	}
	return input.json as Wire;
}

export function etagOf(body: Wire): string {
	const v = body.expectedVersion;
	if (typeof v !== "string" || v === "") return bad();
	return v;
}

const STATUS_BY_QUERY: Record<string, PostStatus> = {
	draft: PostStatus.DRAFT,
	published: PostStatus.PUBLISHED,
	unpublished: PostStatus.UNPUBLISHED,
};

export function statusFilter(query: URLSearchParams): PostStatus {
	const raw = query.get("status");
	if (raw === null || raw === "") return PostStatus.UNSPECIFIED;
	const status = STATUS_BY_QUERY[raw];
	if (status === undefined) return bad("INVALID_QUERY");
	return status;
}

export function pageSizeOf(query: URLSearchParams): number {
	const raw = query.get("page_size");
	if (raw === null) return 0;
	const n = Number(raw);
	if (!Number.isInteger(n) || n < 1 || n > 50) return bad("INVALID_QUERY");
	return n;
}

/** 建立：BFF 只收一張圖與一個商品（伺服器的貼文是單圖單商品）。 */
export function createFields(body: Wire) {
	const tokens = stringList(body.stagedTokens) ?? [];
	const products = stringList(body.productIds) ?? [];
	if (tokens.length > 1 || products.length > 1) return bad();
	return {
		body: optionalString(body.content) ?? "",
		imageStagedToken: tokens[0] ?? "",
		...(products[0] ? { productId: products[0] } : {}),
		hashtagInputs: stringList(body.tags) ?? [],
	};
}

/** 修改：只有請求帶的欄位進 update_mask。 */
export function updateFields(postId: string, etag: string, body: Wire) {
	const paths: string[] = [];
	const post = create(PostSchema, { id: postId, etag });
	if (body.content !== undefined) {
		post.body = optionalString(body.content) ?? "";
		paths.push("body");
	}
	const tokens = stringList(body.stagedTokens);
	if (tokens !== undefined) {
		if (tokens.length > 1) return bad();
		post.imageStagedToken = tokens[0] ?? "";
		paths.push("image_staged_token");
	}
	const products = stringList(body.productIds);
	if (products !== undefined) {
		if (products.length > 1) return bad();
		if (products[0]) post.productId = products[0];
		paths.push("product_id");
	}
	const tags = stringList(body.tags);
	if (tags !== undefined) {
		post.hashtagInputs = tags;
		paths.push("hashtag_inputs");
	}
	return { post, updateMask: fieldMask(PostSchema, paths) };
}

/** 排程：`publishMode: "scheduled"` 加 RFC3339 的 `scheduledPublishAt`；其他（含 immediate）是立即發布。 */
export function scheduleOf(body: Wire) {
	if (body.publishMode !== "scheduled") return undefined;
	const at = body.scheduledPublishAt;
	if (typeof at !== "string") return bad();
	const ms = Date.parse(at);
	if (!Number.isFinite(ms)) return bad();
	return timestampFromDate(new Date(ms));
}

const PUBLISH_STATUS = {
	[PostStatus.DRAFT]: "4401",
	[PostStatus.PUBLISHED]: "4402",
	[PostStatus.UNPUBLISHED]: "4403",
} as const;

function publishStatusOf(status: PostStatus): string | undefined {
	return PUBLISH_STATUS[status as keyof typeof PUBLISH_STATUS];
}

/** 一篇貼文 → 瀏覽器面的 DTO（讀取與寫入共用）。`version` 是 etag，接著要打 publish／修改時帶回。 */
export function postDto(post: Post): Wire {
	const image = imageUri(post.image, "card");
	const publishStatus = publishStatusOf(post.status);
	return {
		postId: post.id,
		version: post.etag,
		content: post.body,
		imageKeys: image ? [image] : [],
		tags: post.hashtags.map((h) => h.display || h.name),
		products: post.productId ? [{ productId: post.productId }] : [],
		createdAt: post.createTime
			? new Date(Number(post.createTime.seconds) * 1000).toISOString()
			: "",
		...(publishStatus ? { publishStatus } : {}),
		...(post.takedownMeasureId
			? { takedownMeasureId: post.takedownMeasureId }
			: {}),
	};
}

/** 寫入的統一回應：`{auditLogId, version, resource}`（`resource` 是寫入後的貼文）。 */
export function mutationDto(post: Post | undefined, auditLogId: string): Wire {
	const resource = post ? postDto(post) : {};
	return {
		auditLogId,
		version: post?.etag ?? "",
		resource,
	};
}

export function listDto(posts: Post[], nextPageToken: string): Wire {
	return { records: posts.map(postDto), nextPageToken };
}

/** 動態數：每個狀態的篇數、待發布排程篇數，與不含已刪除的總篇數。 */
export function summaryDto(message: {
	postCounts: { status: PostStatus; count: bigint }[];
	scheduledPostCount: bigint;
}): Wire {
	const counts: Wire = {};
	let total = 0;
	for (const c of message.postCounts) {
		const code = publishStatusOf(c.status);
		if (code === undefined) continue;
		counts[code] = Number(c.count);
		total += Number(c.count);
	}
	return {
		countsByPublishStatus: counts,
		scheduledPostCount: Number(message.scheduledPostCount),
		total,
	};
}
