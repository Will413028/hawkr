/**
 * 評價的資料層：ReviewPublicService（公開列表與統計）與 ReviewAppService（買家寫、讀自己的、店主回覆）。
 * 列表是 page_token 分頁，評價數與平均分、星等分佈只取自統計 RPC（GetProductReviewStats、
 * GetStoreReviewStats），不由 client 加總已載入的頁。讀取失敗一律拋出，由呼叫端與「沒有評價」分開呈現。
 */

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Review as WireReview } from "@hawkr/api/gen/hawkr/review/v1alpha1/review_pb";
import { reviewAppClient, reviewPublicClient } from "../connect/review";
import type { PublicTransportOptions } from "../connect/transport";
import { imageUri } from "../media/image";

/** 店家回覆。 */
export interface ReviewReply {
	content: string;
	/** ISO 8601 UTC。 */
	repliedAt: string;
}

/** 一則評價（公開列表、買家自己的列表共用）；圖片是伺服器給的 uri_template 代入 preset 後的網址。 */
export interface Review {
	id: string;
	productId: string;
	storeId: string;
	/** 作者名稱；作者沒有名稱或已刪帳號時是 undefined，由呈現層決定顯示。 */
	username?: string;
	avatarUrl?: string;
	rating: number;
	content: string;
	imageUrls: string[];
	/** 下單當時的規格名稱。 */
	skuName: string;
	productImageUrl?: string;
	/** 下單當時的單價，新台幣元。 */
	price: number;
	reply?: ReviewReply;
	/** 建立時間（unix 毫秒）。 */
	createdAt: number;
	orderNo?: string;
	orderItemId?: string;
}

/** 評價統計：數量、平均分與星等分佈（`GetProductReviewStats`、`GetStoreReviewStats`）。 */
export interface ReviewStats {
	totalCount: number;
	averageRating: number;
	rating1Count: number;
	rating2Count: number;
	rating3Count: number;
	rating4Count: number;
	rating5Count: number;
}

export interface ReviewPage {
	reviews: Review[];
	/** 空字串是沒有下一頁。 */
	nextPageToken: string;
}

export interface ReadOptions extends PublicTransportOptions {
	signal?: AbortSignal;
	timeoutMs?: number;
}

/** 買家一次最多附的照片張數（review.proto `image_staged_tokens`）。 */
export const REVIEW_PHOTO_LIMIT = 4;

function secondsOf(ts: Parameters<typeof timestampDate>[0]): number {
	return timestampDate(ts).getTime();
}

export function reviewFromWire(wire: WireReview): Review {
	return {
		id: wire.id,
		productId: wire.productId,
		storeId: wire.storeId,
		username: wire.author?.name?.trim() || undefined,
		avatarUrl: imageUri(wire.author?.avatar, "avatar"),
		rating: wire.rating,
		content: wire.content,
		imageUrls: wire.images.flatMap((img) => {
			const url = imageUri(img, "card");
			return url ? [url] : [];
		}),
		skuName: wire.skuName,
		productImageUrl: imageUri(wire.itemImage, "thumb"),
		price: Number(wire.price),
		reply: wire.reply
			? {
					content: wire.reply.content,
					repliedAt: wire.reply.replyTime
						? timestampDate(wire.reply.replyTime).toISOString()
						: "",
				}
			: undefined,
		createdAt: wire.createTime ? secondsOf(wire.createTime) : 0,
		orderNo: wire.orderNo || undefined,
		orderItemId: wire.orderItemId || undefined,
	};
}

interface WireStats {
	count: number;
	average: number;
	distribution: readonly { rating: number; count: number }[];
}

/** 統計回應整理成固定 1 到 5 星的欄位；分佈缺的星等是 0。 */
export function statsFromWire(wire: WireStats): ReviewStats {
	const at = (star: number) =>
		wire.distribution.find((d) => d.rating === star)?.count ?? 0;
	return {
		totalCount: wire.count,
		averageRating: wire.average,
		rating1Count: at(1),
		rating2Count: at(2),
		rating3Count: at(3),
		rating4Count: at(4),
		rating5Count: at(5),
	};
}

function splitOptions(opts: ReadOptions) {
	const { signal, timeoutMs, ...transport } = opts;
	return { transport, call: { signal, timeoutMs } };
}

export const PRODUCT_REVIEWS_PAGE_SIZE = 10;

export async function fetchProductReviewsPage(
	productId: string,
	{
		pageToken = "",
		pageSize = PRODUCT_REVIEWS_PAGE_SIZE,
		rating,
		withImages = false,
		...opts
	}: {
		pageToken?: string;
		pageSize?: number;
		/** 只列這個星等（1 到 5，等於不是下限）。 */
		rating?: number;
		withImages?: boolean;
	} & ReadOptions = {},
): Promise<ReviewPage> {
	const { transport, call } = splitOptions(opts);
	const res = await reviewPublicClient(transport).listProductReviews(
		{ productId, pageSize, pageToken, rating, withImages },
		call,
	);
	return {
		reviews: res.reviews.map(reviewFromWire),
		nextPageToken: res.nextPageToken,
	};
}

export async function fetchProductReviewStats(
	productId: string,
	opts: ReadOptions = {},
): Promise<ReviewStats> {
	const { transport, call } = splitOptions(opts);
	return statsFromWire(
		await reviewPublicClient(transport).getProductReviewStats(
			{ productId },
			call,
		),
	);
}

export const STORE_REVIEWS_PAGE_SIZE = 10;

export async function fetchStoreReviewsPage(
	storeId: string,
	{
		pageToken = "",
		pageSize = STORE_REVIEWS_PAGE_SIZE,
		...opts
	}: { pageToken?: string; pageSize?: number } & ReadOptions = {},
): Promise<ReviewPage> {
	const { transport, call } = splitOptions(opts);
	const res = await reviewPublicClient(transport).listStoreReviews(
		{ storeId, pageSize, pageToken },
		call,
	);
	return {
		reviews: res.reviews.map(reviewFromWire),
		nextPageToken: res.nextPageToken,
	};
}

export async function fetchStoreReviewStats(
	storeId: string,
	opts: ReadOptions = {},
): Promise<ReviewStats> {
	const { transport, call } = splitOptions(opts);
	return statsFromWire(
		await reviewPublicClient(transport).getStoreReviewStats({ storeId }, call),
	);
}

/**
 * 店家評價的第 [page] 頁（從 1 起）：契約只有 page_token，所以從第一頁沿著 next_page_token 走到目標頁。
 * 中途沒有下一頁表示那一頁在結尾之後，回空清單（這是真的沒有，不是讀取失敗）；任何一頁讀取失敗照拋。
 */
export async function fetchStoreReviewsAtPage(
	storeId: string,
	page: number,
	opts: ReadOptions = {},
): Promise<ReviewPage> {
	let token = "";
	for (let current = 1; current < page; current++) {
		const skipped = await fetchStoreReviewsPage(storeId, {
			...opts,
			pageToken: token,
		});
		if (skipped.nextPageToken === "") return { reviews: [], nextPageToken: "" };
		token = skipped.nextPageToken;
	}
	return fetchStoreReviewsPage(storeId, { ...opts, pageToken: token });
}

export const MY_REVIEWS_PAGE_SIZE = 20;

/** 買家自己寫過的評價（`ListMyReviews`），新到舊由伺服器決定。 */
export async function fetchMyReviewsPage({
	pageToken = "",
	pageSize = MY_REVIEWS_PAGE_SIZE,
	signal,
}: {
	pageToken?: string;
	pageSize?: number;
	signal?: AbortSignal;
} = {}): Promise<ReviewPage> {
	const res = await reviewAppClient().listMyReviews(
		{ pageSize, pageToken },
		{ signal },
	);
	return {
		reviews: res.reviews.map(reviewFromWire),
		nextPageToken: res.nextPageToken,
	};
}

export interface SubmitOrderItemReviewBody {
	orderNo: string;
	orderItemId: string;
	rating: number;
	content?: string;
	/** 最多 {@link REVIEW_PHOTO_LIMIT} 個 `uploadImageFile` 回的 stagedToken。 */
	stagedTokens?: string[];
}

export async function submitOrderItemReview(
	body: SubmitOrderItemReviewBody,
): Promise<Review> {
	const res = await reviewAppClient().createReview({
		orderNo: body.orderNo,
		orderItemId: body.orderItemId,
		rating: body.rating,
		content: body.content ?? "",
		imageStagedTokens: body.stagedTokens ?? [],
	});
	if (!res.review) throw new Error("CreateReview returned no review");
	return reviewFromWire(res.review);
}

export async function replyReview(
	reviewId: string,
	content: string,
): Promise<Review> {
	const res = await reviewAppClient().replyReview({ reviewId, content });
	if (!res.review) throw new Error("ReplyReview returned no review");
	return reviewFromWire(res.review);
}
