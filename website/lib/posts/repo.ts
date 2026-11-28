/**
 * 貼文的資料層：SocialPublicService／SocialAppService 的 Connect 呼叫與 `Post` → 畫面用 `PostListItem`、
 * `PostDetail` 的映射。列表是 page_token 分頁，沒有總數也沒有頁碼：結束只看 `nextPageToken` 是不是空字串。
 */

import {
	ActorKind,
	type Post,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { imageUri } from "@/lib/media/image";
import { placeholders } from "@/lib/media/public-assets";
import { formatPriceRange } from "@/lib/products/pricing";
import type {
	PostDetail,
	PostListItem,
	PostProductData,
	PostViewerActor,
	PostViewerState,
} from "../api/openapi-types";
import {
	socialAppClient,
	socialPublicClient,
	socialViewerClient,
} from "../connect/social";
import type { PublicTransportOptions } from "../connect/transport";
import { normalizePostTag, uniquePostTags } from "./tags";

export const FALLBACK_POST_IMAGE = placeholders.post;

export interface PostsPage {
	records: PostListItem[];
	/** 空字串是沒有下一頁。一頁可以少於 pageSize（伺服器在分頁之前過濾觀看者封鎖的店）。 */
	nextPageToken: string;
	pageSize: number;
}

const toSeconds = (t: { seconds: bigint } | undefined): number =>
	t ? Number(t.seconds) : 0;

/** 貼文帶的商品摘要（伺服器批次補）→ 卡片與單篇用的商品；商品已刪除、讀不到或下架時是 null。 */
export function postProductOf(post: Post): PostProductData | null {
	const product = post.product;
	if (!product) return null;
	return {
		id: product.id,
		imageKey: imageUri(product.image, "thumb") ?? "",
		name: product.name,
		price: formatPriceRange(Number(product.minPrice), Number(product.maxPrice)),
	};
}

function viewerActorOf(post: Post): PostViewerActor | undefined {
	const actor = post.viewerActor;
	if (!actor) return undefined;
	const kind =
		actor.kind === ActorKind.STORE
			? "store"
			: actor.kind === ActorKind.BUYER
				? "buyer"
				: undefined;
	if (!kind) return undefined;
	const avatarKey = imageUri(actor.avatar, "avatar");
	return {
		kind,
		id: actor.id,
		...(actor.name?.trim() ? { name: actor.name.trim() } : {}),
		...(avatarKey ? { avatarKey } : {}),
	};
}

/**
 * 伺服器依觀看者算的留言權限。匿名讀取（訪客，或進了共用快取的讀取）沒有身分也不能留言，與「讀了但不能留言」
 * 分不出來，所以兩者都沒有時回 undefined（還不知道），由呼叫端用依觀看者的讀取補。
 */
export function postViewerOf(post: Post): PostViewerState | undefined {
	if (!viewerKnown(post)) return undefined;
	return viewerStateOf(post);
}

function viewerKnown(post: Post): boolean {
	return Boolean(viewerActorOf(post) || post.viewerCanComment);
}

function viewerStateOf(post: Post): PostViewerState {
	const actor = viewerActorOf(post);
	return { canComment: post.viewerCanComment, ...(actor ? { actor } : {}) };
}

function viewerField(post: Post): { viewer?: PostViewerState } {
	const viewer = postViewerOf(post);
	return viewer ? { viewer } : {};
}

/** `Post` → 動態卡片用的列表項；缺圖補預設圖，店名缺漏補 Hawkr 選品。 */
export function postToListItem(post: Post): PostListItem {
	const image = imageUri(post.image, "card");
	const store = post.store;
	return {
		id: post.id,
		sellerId: store?.id ?? "",
		storeCode: store?.storeCode ?? "",
		sellerName: store?.name?.trim() || "Hawkr 選品",
		logoKey: imageUri(store?.logo, "avatar") ?? "",
		content: post.body,
		imageKeys: [image?.trim() || FALLBACK_POST_IMAGE],
		tags: uniquePostTags(post.hashtags.map((h) => h.display || h.name)),
		product: postProductOf(post),
		commentCount: Number(post.commentCount),
		likeCount: Number(post.likeCount),
		shareCount: Number(post.shareCount),
		viewCount: Number(post.viewCount),
		isLiked: post.viewerLiked,
		createdAt: toSeconds(post.publishTime ?? post.createTime),
		...(post.etag ? { etag: post.etag } : {}),
		...viewerField(post),
	};
}

/** `Post` → 單篇貼文；商品摘要讀不到時 products 是空的，`productId` 仍照實帶回。 */
export function postToDetail(post: Post): PostDetail & { productId?: string } {
	const product = postProductOf(post);
	const image = imageUri(post.image, "detail");
	const store = post.store;
	return {
		id: post.id,
		sellerId: store?.id ?? "",
		sellerName: store?.name?.trim() || "Hawkr 選品",
		logoKey: imageUri(store?.logo, "avatar") ?? "",
		content: post.body,
		imageKeys: image ? [image] : [],
		tags: uniquePostTags(post.hashtags.map((h) => h.display || h.name)),
		products: product ? [product] : [],
		...(post.productId ? { productId: post.productId } : {}),
		commentCount: Number(post.commentCount),
		likeCount: Number(post.likeCount),
		shareCount: Number(post.shareCount),
		viewCount: Number(post.viewCount),
		isLiked: post.viewerLiked,
		createdAt: toSeconds(post.publishTime ?? post.createTime),
		...viewerField(post),
	};
}

export interface FetchPostsPageOptions extends PublicTransportOptions {
	pageToken?: string;
	pageSize?: number;
	/** 只列這家店的貼文；不能與 tag 同時用（伺服器回 invalid_argument）。 */
	storeId?: string;
	tag?: string;
	signal?: AbortSignal;
	/**
	 * 依觀看者讀（瀏覽器帶 session cookie）：登入者看得到自己的 `viewer_*`，買家封鎖的店的貼文不出現。
	 * 不設是匿名讀取，結果可以進 server 端的共用快取。
	 */
	viewer?: boolean;
	/** 失敗時丟錯，不回合成的空頁（快取重新生成不能把空頁記成新資料）。 */
	strict?: boolean;
}

export async function fetchPostsPage({
	pageToken = "",
	pageSize = 12,
	storeId,
	tag,
	signal,
	viewer = false,
	strict = false,
	...transport
}: FetchPostsPageOptions = {}): Promise<PostsPage> {
	const hashtag = normalizePostTag(tag);
	if (storeId && hashtag) {
		throw new Error("fetchPostsPage: storeId 與 tag 不能同時設");
	}
	const client = viewer
		? socialViewerClient(transport)
		: socialPublicClient(transport);
	try {
		const res = await client.listPosts(
			{
				pageSize,
				pageToken,
				...(storeId ? { storeId } : {}),
				...(hashtag ? { hashtag } : {}),
			},
			{ signal },
		);
		return {
			records: res.posts.map(postToListItem),
			nextPageToken: res.nextPageToken,
			pageSize,
		};
	} catch (error) {
		if (strict) throw error;
		return { records: [], nextPageToken: "", pageSize };
	}
}

/** 單篇貼文（依觀看者；讀不到的貼文丟錯）。 */
export async function fetchPostDetailForViewer(
	postId: string,
	signal?: AbortSignal,
): Promise<PostDetail> {
	const res = await socialViewerClient().getPost({ postId }, { signal });
	if (!res.post) throw new Error("post missing");
	return postToDetail(res.post);
}

/** 觀看者在這則貼文的留言權限與身分（依觀看者讀單篇）；留言框用它決定提示字與要不要出現。 */
export async function fetchPostViewerState(
	postId: string,
	signal?: AbortSignal,
): Promise<PostViewerState> {
	const res = await socialViewerClient().getPost({ postId }, { signal });
	if (!res.post) throw new Error("post missing");
	return viewerStateOf(res.post);
}

/**
 * 「本週主題」：伺服器依最近 30 篇的瀏覽數排序，最多 `count` 篇；失敗回空陣列。
 * 匿名讀取，megamenu 與共用快取共用。
 */
export async function fetchTrendingPosts(
	count = 5,
	opts: PublicTransportOptions & {
		signal?: AbortSignal;
		/** 失敗時丟錯，不回空陣列（快取重新生成不能把空結果記成新資料）。 */
		strict?: boolean;
	} = {},
): Promise<PostListItem[]> {
	const { signal, strict = false, ...transport } = opts;
	try {
		const res = await socialPublicClient(transport).suggestTrendingPosts(
			{ maxResults: count },
			{ signal },
		);
		return res.posts.map(postToListItem);
	} catch (error) {
		if (strict) throw error;
		return [];
	}
}

/** 記一次單篇頁瀏覽：匿名也能呼叫，登入者帶 cookie 由伺服器去重；失敗不影響頁面。 */
export async function recordPostView(postId: string): Promise<void> {
	try {
		await socialViewerClient().recordPostView({ postId });
	} catch {}
}

/** 對貼文按讚或取消：兩個明確的方法，呼叫端要先知道目前的狀態。 */
export async function setPostLiked(
	postId: string,
	liked: boolean,
): Promise<{ liked: boolean }> {
	const client = socialAppClient();
	if (liked) await client.likePost({ postId });
	else await client.unlikePost({ postId });
	return { liked };
}

export async function sharePost(postId: string): Promise<void> {
	await socialAppClient().sharePost({ postId });
}

export interface PostTagSuggestion {
	name: string;
	postCount: number;
}

export interface TrendingPostTag {
	name: string;
	label: string;
	count: number;
}

export async function suggestPostTags({
	q,
	mode = "prefix",
	limit = 10,
	signal,
}: {
	q: string;
	mode?: "prefix" | "contain";
	limit?: number;
	signal?: AbortSignal;
}): Promise<PostTagSuggestion[]> {
	const query = normalizePostTag(q);
	if (!query) return [];
	try {
		const res = await socialPublicClient().suggestTags(
			{
				query,
				contains: mode === "contain",
				maxResults: Math.min(Math.max(limit, 1), 20),
			},
			{ signal },
		);
		return res.tags.flatMap((tag) => {
			const name = normalizePostTag(tag.name);
			return name ? [{ name, postCount: Number(tag.postCount) }] : [];
		});
	} catch {
		return [];
	}
}

export async function fetchTrendingPostTags({
	limit = 20,
	signal,
	...transport
}: {
	limit?: number;
	signal?: AbortSignal;
} & PublicTransportOptions = {}): Promise<TrendingPostTag[]> {
	try {
		const res = await socialPublicClient(transport).suggestTrendingTags(
			{ maxResults: Math.min(Math.max(limit, 1), 50) },
			{ signal },
		);
		return res.tags.flatMap((tag) => {
			const name = normalizePostTag(tag.name);
			return name
				? [{ name, label: `#${name}`, count: Number(tag.postCount) }]
				: [];
		});
	} catch {
		return [];
	}
}
