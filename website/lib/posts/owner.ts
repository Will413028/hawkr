/**
 * 店主自己的貼文（SocialAppService 的 owner 方法）：建立、讀、列出、修改、發布、下架、刪除。
 * 寫入帶 etag（樂觀鎖）：etag 必須是畫面載入時拿到的版本，寫入前不重讀；
 * 版本已被改過時伺服器回 ABORTED（VERSION_CONFLICT），由呼叫端的錯誤路徑顯示。
 */

import { create } from "@bufbuild/protobuf";
import type { Client } from "@connectrpc/connect";
import {
	type Post,
	PostSchema,
	PostStatus,
	type SocialAppService,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import type { MyPostDetail, PostListItem } from "../api/openapi-types";
import { PostPublishStatus } from "../codemeta/enums";
import { imageUri } from "../media/image";
import { postToDetail, postToListItem } from "./repo";

export type OwnerPostClient = Client<typeof SocialAppService>;

/** 貼文狀態 → 賣家中心畫面用的發布狀態碼（4401 未發布、4402 已發布、4403 已下架）。 */
export function publishStatusOf(status: PostStatus): string | undefined {
	switch (status) {
		case PostStatus.DRAFT:
			return PostPublishStatus.DRAFT;
		case PostStatus.PUBLISHED:
			return PostPublishStatus.PUBLISHED;
		case PostStatus.UNPUBLISHED:
			return PostPublishStatus.UNPUBLISHED;
		default:
			// 不認得的狀態不猜：畫面只在有值時畫狀態標籤。
			return undefined;
	}
}

export function ownerListItem(post: Post): PostListItem {
	const item = postToListItem(post);
	// 店主自己的列表：沒有圖就是沒有圖，不補預設圖（草稿可以沒有圖）。
	const image = imageUri(post.image, "card");
	const publishStatus = publishStatusOf(post.status);
	return {
		...item,
		imageKeys: image ? [image] : [],
		...(publishStatus ? { publishStatus } : {}),
		...(post.takedownMeasureId
			? { takedownMeasureId: post.takedownMeasureId }
			: {}),
	};
}

export function ownerDetail(post: Post): MyPostDetail {
	const detail = postToDetail(post);
	const publishStatus = publishStatusOf(post.status);
	return {
		...detail,
		// 商品摘要讀不到（已刪除）時編輯頁仍以 ID 預選，名稱與價格留空。
		products:
			detail.products.length > 0
				? detail.products
				: post.productId
					? [{ id: post.productId, imageKey: "", name: "", price: "" }]
					: [],
		...(post.etag ? { etag: post.etag } : {}),
		...(publishStatus ? { publishStatus } : {}),
		...(post.takedownMeasureId
			? { takedownMeasureId: post.takedownMeasureId }
			: {}),
	};
}

export interface OwnerCreateInput {
	/** 店主的建立就是發布（賣家中心只有一種送出）；代行路徑建的是草稿，不走這裡。 */
	content: string;
	stagedToken?: string;
	productId?: string;
	tags?: string[];
}

export interface OwnerUpdateInput {
	content: string;
	/** 換新圖時的 staged token；沒帶就沿用目前的圖（update_mask 不含圖片）。 */
	newStagedToken?: string;
	/** undefined＝不變；null＝清除商品（product_id 在 update_mask 內但不帶值）。 */
	productId?: string | null;
	tags?: string[];
}

export function createOwnerPostApi(client: OwnerPostClient) {
	function requireEtag(etag: string | undefined): string {
		if (!etag) throw new Error("post etag missing");
		return etag;
	}

	return {
		async create(body: OwnerCreateInput): Promise<{ id: string }> {
			const res = await client.createPost({
				body: body.content,
				imageStagedToken: body.stagedToken ?? "",
				...(body.productId ? { productId: body.productId } : {}),
				hashtagInputs: body.tags ?? [],
				publish: true,
				requestId: crypto.randomUUID(),
			});
			if (!res.post) throw new Error("post missing");
			return { id: res.post.id };
		},

		async update(
			id: string,
			body: OwnerUpdateInput,
			etag: string | undefined,
		): Promise<void> {
			const version = requireEtag(etag);
			const paths = ["body"];
			if (body.newStagedToken) paths.push("image_staged_token");
			// null 與 undefined 要分開：進 mask 但不帶值才會清掉商品。
			if (body.productId !== undefined) paths.push("product_id");
			if (body.tags) paths.push("hashtag_inputs");
			await client.updatePost({
				post: create(PostSchema, {
					id,
					body: body.content,
					etag: version,
					imageStagedToken: body.newStagedToken ?? "",
					...(body.productId ? { productId: body.productId } : {}),
					hashtagInputs: body.tags ?? [],
				}),
				updateMask: fieldMask(PostSchema, paths),
			});
		},

		async setPublished(
			id: string,
			published: boolean,
			etag: string | undefined,
		): Promise<void> {
			const version = requireEtag(etag);
			if (published) await client.publishPost({ postId: id, etag: version });
			else await client.unpublishPost({ postId: id, etag: version });
		},

		async remove(id: string, etag?: string): Promise<void> {
			await client.deletePost({ postId: id, etag: etag ?? "" });
		},

		async detail(id: string): Promise<MyPostDetail> {
			const res = await client.getMyPost({ postId: id });
			if (!res.post) throw new Error("post missing");
			return ownerDetail(res.post);
		},

		/** 各狀態篇數加總（伺服器的 `GetStoreSocialSummary`，不含已刪除）。 */
		async count(): Promise<number> {
			const res = await client.getStoreSocialSummary({});
			return res.postCounts.reduce((sum, c) => sum + Number(c.count), 0);
		},

		async list(pageSize: number): Promise<PostListItem[]> {
			const res = await client.listMyPosts({ pageSize });
			return res.posts.map(ownerListItem);
		},

		/** 只列已發布的貼文（買家看得到的那些），以 page_token 分頁；`nextPageToken` 空字串是沒有下一頁。 */
		async listPublished(
			pageSize: number,
			pageToken = "",
		): Promise<{ items: PostListItem[]; nextPageToken: string }> {
			const res = await client.listMyPosts({
				pageSize,
				pageToken,
				status: PostStatus.PUBLISHED,
			});
			return {
				items: res.posts.map(ownerListItem),
				nextPageToken: res.nextPageToken,
			};
		},
	};
}
