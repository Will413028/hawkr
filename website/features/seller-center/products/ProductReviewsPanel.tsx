"use client";

import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { type KeyboardEvent, useState } from "react";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Button from "@/components/ui/Button";
import { Stars } from "@/components/ui/Stars";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchProductReviews,
	type Review,
	replyReview,
} from "@/lib/reviews/repo";

const PAGE_SIZE = 10;

/** Wire `createdAt` is unix seconds or ms; render as a Taipei date. */
function formatReviewDate(ts: number): string {
	return formatTaipeiDate(ts);
}

/** Reply `repliedAt` is ISO 8601 UTC; render as a Taipei date. */
function formatRepliedAt(iso: string): string {
	return formatTaipeiDate(iso);
}

/**
 * Seller reply block under one buyer review: shows the existing 賣家回覆 (or a
 * 尚未回覆 hint) and an inline composer to post / edit it. On success it
 * invalidates the product-reviews query so the list re-fetches the saved reply.
 */
function SellerReplyBlock({
	review,
	productId,
}: {
	review: Review;
	productId: string;
}) {
	const queryClient = useQueryClient();
	const existing = review.reply?.content?.trim() ?? "";
	const [editing, setEditing] = useState(false);
	const [draft, setDraft] = useState(existing);
	const [submitting, setSubmitting] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const openComposer = () => {
		setDraft(existing);
		setError(null);
		setEditing(true);
	};

	const submit = async () => {
		const content = draft.trim();
		if (!content || submitting) return;
		setSubmitting(true);
		setError(null);
		try {
			await replyReview(review.id, content);
			await queryClient.invalidateQueries({
				queryKey: queryKeys.products.reviews(productId),
			});
			setEditing(false);
		} catch {
			setError("回覆送出失敗，請再試一次");
		} finally {
			setSubmitting(false);
		}
	};

	// Enter 送出、Shift+Enter 換行；IME 組字中跳過避免送出半字。
	const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
		if (
			event.key === "Enter" &&
			!event.shiftKey &&
			!event.nativeEvent.isComposing
		) {
			event.preventDefault();
			void submit();
		}
	};

	if (editing) {
		return (
			<div className="mt-[var(--space-3)] flex flex-col gap-[var(--space-2)] border-t border-[var(--hkui-divider)] pt-[var(--space-3)]">
				<span className="text-[length:var(--text-xs)] font-bold text-[color:var(--text-heading)]">
					賣家回覆
				</span>
				<textarea
					className="w-full resize-none rounded-[var(--hkui-radius-md)] border border-[var(--hkui-divider)] bg-[var(--hkui-content2)] px-[var(--space-3)] py-[var(--space-2)] text-[length:var(--text-sm)] leading-relaxed text-[color:var(--text-body)] [font-family:inherit] focus:border-[var(--hkui-primary)] focus:outline-none"
					value={draft}
					onChange={(event) => setDraft(event.target.value)}
					onKeyDown={onKeyDown}
					placeholder="輸入回覆內容…"
					rows={2}
					maxLength={1000}
				/>
				{error ? (
					<p className="text-[length:var(--text-xs)] text-[var(--hkui-danger)]">
						{error}
					</p>
				) : null}
				<div className="flex justify-end gap-[var(--space-2)]">
					<Button
						type="button"
						variant="bordered"
						color="default"
						size="sm"
						disabled={submitting}
						onClick={() => {
							setEditing(false);
							setError(null);
						}}
					>
						取消
					</Button>
					<Button
						type="button"
						size="sm"
						disabled={!draft.trim() || submitting}
						isLoading={submitting}
						loadingLabel={existing ? "回覆更新中…" : "回覆送出中…"}
						onClick={() => void submit()}
					>
						{existing ? "更新回覆" : "送出回覆"}
					</Button>
				</div>
			</div>
		);
	}

	if (existing) {
		return (
			<div className="mt-[var(--space-3)] border-t border-[var(--hkui-divider)] pt-[var(--space-3)]">
				<div className="rounded-[var(--hkui-radius-md)] bg-[var(--hkui-content2)] p-[var(--space-3)]">
					<div className="mb-[var(--space-2)] flex items-center justify-between gap-[var(--space-2)]">
						<span className="text-[length:var(--text-xs)] font-bold text-[color:var(--text-heading)]">
							賣家回覆
						</span>
						<span className="shrink-0 text-[length:var(--text-xs)] text-[var(--color-gray-mid)]">
							{formatRepliedAt(review.reply?.repliedAt ?? "")}
						</span>
					</div>
					<p className="whitespace-pre-line text-[length:var(--text-sm)] leading-relaxed text-[color:var(--text-body)]">
						{existing}
					</p>
					<div className="mt-[var(--space-2)] flex justify-end">
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							onClick={openComposer}
						>
							編輯回覆
						</Button>
					</div>
				</div>
			</div>
		);
	}

	return (
		<div className="mt-[var(--space-3)] flex items-center justify-between gap-[var(--space-2)] border-t border-[var(--hkui-divider)] pt-[var(--space-3)]">
			<span className="text-[length:var(--text-xs)] text-[var(--color-gray-mid)]">
				尚未回覆
			</span>
			<Button
				type="button"
				variant="bordered"
				color="default"
				size="sm"
				onClick={openComposer}
			>
				回覆
			</Button>
		</div>
	);
}

/**
 * Buyer reviews for one seller product, shown inside the 商品管理 reviews drawer.
 * Reads the same public list a buyer sees on the PDP (`fetchProductReviews`),
 * which now carries each review's seller `reply` ({ content, repliedAt }).
 *
 * Each review gets a 賣家回覆 block (SellerReplyBlock) to post / edit a reply via
 * `POST /app/sellers/reviews/{id}/reply` (#1174).
 */
export default function ProductReviewsPanel({
	productId,
	productImage,
}: {
	productId: string;
	productImage?: string;
}) {
	const query = useInfiniteQuery({
		queryKey: queryKeys.products.reviews(productId),
		queryFn: ({ pageParam }) =>
			fetchProductReviews(productId, { page: pageParam, pageSize: PAGE_SIZE }),
		initialPageParam: 1,
		getNextPageParam: (lastPage) => {
			const loaded = lastPage.page * lastPage.pageSize;
			return loaded < lastPage.total ? lastPage.page + 1 : undefined;
		},
	});

	const pages = query.data?.pages ?? [];
	const reviews = pages.flatMap((p) => p.reviews);
	const total = pages[0]?.total ?? 0;

	if (query.isLoading) {
		return (
			<p className="py-[var(--space-8)] text-center text-[length:var(--text-sm)] text-[var(--color-gray-mid)]">
				載入評論中…
			</p>
		);
	}

	if (query.isError) {
		return (
			<div className="flex flex-col items-center gap-[var(--space-3)] py-[var(--space-8)]">
				<p className="text-[length:var(--text-sm)] text-[var(--color-gray-mid)]">
					評論載入失敗
				</p>
				<Button
					variant="bordered"
					color="default"
					size="sm"
					isLoading={query.isFetching}
					loadingLabel="重新載入評論中…"
					onClick={() => query.refetch()}
				>
					重試
				</Button>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-[var(--space-4)]">
			<header className="flex items-center gap-[var(--space-3)] border-b border-[var(--hkui-divider)] pb-[var(--space-3)]">
				{productImage ? (
					<AppImage
						src={resolveCdnUrl(productImage)}
						width={44}
						height={44}
						sizes="44px"
						alt=""
						className="h-11 w-11 shrink-0 rounded-[var(--hkui-radius-sm)] bg-[var(--hkui-content2)] object-cover"
						loading="lazy"
					/>
				) : null}
				<span className="text-[length:var(--text-sm)] text-[color:var(--text-body)]">
					共 <strong className="tabular-nums">{total}</strong> 則買家評論
				</span>
			</header>

			{reviews.length === 0 ? (
				<p className="py-[var(--space-8)] text-center text-[length:var(--text-sm)] text-[var(--color-gray-mid)]">
					目前沒有買家評價
				</p>
			) : (
				reviews.map((review, index) => {
					const photos = review.imageKeys.filter(
						(src) => typeof src === "string" && src.trim().length > 0,
					);
					return (
						<article
							key={review.id || `review-${index}`}
							className="rounded-[var(--hkui-radius-lg)] border border-[var(--hkui-divider)] bg-[var(--hkui-content1)] p-[var(--space-4)]"
						>
							<div className="mb-[var(--space-2)] flex items-center gap-[var(--space-3)]">
								<Avatar
									src={review.avatarKey}
									name={review.username}
									alt={review.username}
									entity="buyer"
									className="h-9 w-9 shrink-0 rounded-full object-cover"
									fallbackClassName="h-9 w-9 shrink-0 rounded-full object-cover"
								/>
								<div className="flex min-w-0 flex-1 flex-col gap-[2px]">
									<span className="truncate text-[length:var(--text-sm)] font-bold text-[color:var(--text-heading)]">
										{review.username}
									</span>
									<div className="flex gap-[1px]">
										<Stars count={review.rating} size={14} />
									</div>
								</div>
								<span className="shrink-0 text-[length:var(--text-xs)] text-[var(--color-gray-mid)]">
									{formatReviewDate(review.createdAt)}
								</span>
							</div>

							{review.content ? (
								<p className="mb-[var(--space-2)] whitespace-pre-line text-[length:var(--text-sm)] leading-relaxed text-[color:var(--text-body)]">
									{review.content}
								</p>
							) : null}

							{photos.length > 0 ? (
								<div className="flex flex-wrap gap-[var(--space-2)]">
									{photos.map((photo) => (
										<AppImage
											key={`${review.id}-${photo}`}
											src={resolveCdnUrl(photo)}
											width={96}
											height={96}
											sizes="96px"
											alt="評價照片"
											className="h-24 w-24 rounded-[var(--hkui-radius-sm)] object-cover"
											loading="lazy"
										/>
									))}
								</div>
							) : null}

							<SellerReplyBlock review={review} productId={productId} />
						</article>
					);
				})
			)}

			{query.hasNextPage ? (
				<div className="flex justify-center pt-[var(--space-2)]">
					<Button
						variant="bordered"
						color="default"
						size="sm"
						disabled={query.isFetchingNextPage}
						isLoading={query.isFetchingNextPage}
						loadingLabel="載入更多評論中…"
						onClick={() => query.fetchNextPage()}
					>
						載入更多
					</Button>
				</div>
			) : null}
		</div>
	);
}
