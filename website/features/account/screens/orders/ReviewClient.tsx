"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import { Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import { useOrder } from "@/lib/hooks/useOrders";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { uploadImageFile } from "@/lib/media/upload";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import type { OrderItem } from "@/lib/orders/types";
import { REVIEW_PHOTO_LIMIT, submitOrderItemReview } from "@/lib/reviews/repo";

interface ReviewPhoto {
	previewUrl: string;
	stagedToken: string;
}

interface PerItemDraft {
	rating: number;
	content: string;
	submitted: boolean;
	photos: ReviewPhoto[];
	uploading: boolean;
}

const STAR_OPTIONS = [1, 2, 3, 4, 5];

export default function ReviewClient({ orderNo }: { orderNo: string }) {
	const router = useObservedRouter("order_review");
	const { order, loading } = useOrder(orderNo);
	const [drafts, setDrafts] = React.useState<Record<string, PerItemDraft>>({});
	const [submittingItem, setSubmittingItem] = React.useState<string | null>(
		null,
	);
	const [error, setError] = React.useState<string | null>(null);
	// Photos crop before uploading; cropped files collect here until the
	// queue closes, then upload as one batch.
	const [pendingCrop, setPendingCrop] = React.useState<{
		item: OrderItem;
		files: File[];
	} | null>(null);
	const croppedFilesRef = React.useRef<File[]>([]);

	React.useEffect(() => {
		if (!order) return;
		setDrafts((prev) => {
			const next = { ...prev };
			for (const it of order.items) {
				const key = String(it.orderItemId);
				if (!next[key]) {
					next[key] = {
						rating: 5,
						content: "",
						submitted: it.reviewed === true,
						photos: [],
						uploading: false,
					};
				}
			}
			return next;
		});
	}, [order]);

	if (loading && !order) {
		return (
			<main>
				<p>載入中…</p>
			</main>
		);
	}

	if (!order) {
		return (
			<main>
				<SellerPageShell title="評價商品">
					<p className="text-danger-700 text-sm font-semibold">找不到訂單</p>
				</SellerPageShell>
			</main>
		);
	}

	const handlePickPhotos = (item: OrderItem, files: FileList | null) => {
		const key = String(item.orderItemId);
		const draft = drafts[key];
		if (!draft || draft.submitted || !files || files.length === 0) return;
		const room = REVIEW_PHOTO_LIMIT - draft.photos.length;
		if (room <= 0) {
			setError(`最多只能上傳 ${REVIEW_PHOTO_LIMIT} 張照片`);
			return;
		}
		setError(null);
		croppedFilesRef.current = [];
		setPendingCrop({ item, files: Array.from(files).slice(0, room) });
	};

	const handleAddPhotos = async (item: OrderItem, picked: File[]) => {
		const key = String(item.orderItemId);
		const draft = drafts[key];
		if (!draft || draft.submitted || picked.length === 0) return;
		setError(null);
		setDrafts((prev) => ({
			...prev,
			[key]: { ...prev[key], uploading: true },
		}));
		try {
			const uploaded: ReviewPhoto[] = [];
			for (const file of picked) {
				const asset = await uploadImageFile(file);
				uploaded.push({
					previewUrl: asset.previewUrl,
					stagedToken: asset.stagedToken,
				});
			}
			setDrafts((prev) => ({
				...prev,
				[key]: {
					...prev[key],
					photos: [...prev[key].photos, ...uploaded],
					uploading: false,
				},
			}));
		} catch (e) {
			setError(e instanceof Error ? e.message : "照片上傳失敗");
			setDrafts((prev) => ({
				...prev,
				[key]: { ...prev[key], uploading: false },
			}));
		}
	};

	const handleRemovePhoto = (item: OrderItem, index: number) => {
		const key = String(item.orderItemId);
		setDrafts((prev) => {
			const d = prev[key];
			if (!d) return prev;
			return {
				...prev,
				[key]: { ...d, photos: d.photos.filter((_, i) => i !== index) },
			};
		});
	};

	const handleSubmit = async (item: OrderItem) => {
		const key = String(item.orderItemId);
		const draft = drafts[key];
		if (!draft || draft.submitted || !draft.content.trim() || draft.uploading)
			return;
		setSubmittingItem(key);
		setError(null);
		try {
			await submitOrderItemReview({
				orderNo: order.orderNo,
				orderItemId: item.orderItemId,
				productId: item.productId,
				skuName: item.name,
				productImageUrl: item.image ? resolveCdnUrl(item.image) : undefined,
				price: item.price,
				rating: draft.rating,
				content: draft.content.trim(),
				stagedTokens: draft.photos.map((p) => p.stagedToken),
			});
			setDrafts((prev) => ({
				...prev,
				[key]: { ...draft, submitted: true },
			}));
		} catch (e) {
			setError(e instanceof Error ? e.message : "送出評價失敗");
		} finally {
			setSubmittingItem(null);
		}
	};

	const allDone = order.items.every(
		(it) => drafts[String(it.orderItemId)]?.submitted,
	);

	return (
		<main>
			<SellerPageShell
				title="評價商品"
				subtitle={`訂單 #${orderNo}　·　逐項給分跟留言，協助其他人挑食材。`}
				actions={
					<Button
						href={`/account/orders/${orderNo}`}
						variant="bordered"
						color="default"
						size="sm"
					>
						← 回訂單詳情
					</Button>
				}
			>
				{order.items.map((item) => {
					const key = String(item.orderItemId);
					const d = drafts[key];
					if (!d) return null;
					const imageSrc = item.image ? resolveCdnUrl(item.image) : "";
					return (
						<section
							className="bg-content1 border border-divider p-[18px] mb-4 shadow-hk-card"
							key={key}
						>
							<header className="flex items-center gap-3 mb-4">
								{imageSrc ? (
									<AppImage
										src={imageSrc}
										alt={item.name}
										width={56}
										height={56}
										sizes="56px"
										className="w-[56px] h-[56px] rounded-[var(--hkui-radius-md)] object-cover"
									/>
								) : (
									<span
										className="w-[56px] h-[56px] shrink-0 rounded-[var(--hkui-radius-md)] bg-[var(--hkui-default-100)] border border-[var(--hkui-divider)]"
										aria-hidden="true"
									/>
								)}
								<div className="flex flex-col gap-0.5">
									<strong className="text-[length:var(--hkui-text-medium)]">
										{item.name}
									</strong>
									<span className="text-[length:var(--hkui-text-small)] text-[color:var(--hkui-foreground-soft)]">
										NT$ {item.price} · 數量 {item.quantity}
									</span>
								</div>
							</header>

							{d.submitted ? (
								<p className="m-0 text-[color:var(--hkui-success)] font-semibold">
									✓ 評價已送出，謝謝！
								</p>
							) : (
								<>
									<div
										className="flex items-center gap-1 mb-3"
										role="radiogroup"
										aria-label="評分"
									>
										{STAR_OPTIONS.map((n) => (
											<button
												key={n}
												type="button"
												aria-pressed={d.rating === n}
												className={`bg-transparent border-0 text-[1.75rem] leading-none cursor-pointer px-[2px] transition-colors duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] ${
													d.rating >= n
														? "text-[color:var(--hkui-warning)]"
														: "text-[color:var(--hkui-default-200)]"
												}`}
												onClick={() =>
													setDrafts((prev) => ({
														...prev,
														[key]: { ...d, rating: n },
													}))
												}
											>
												★
											</button>
										))}
										<span className="ml-3 text-[length:var(--hkui-text-small)] text-[color:var(--hkui-foreground-soft)]">
											{d.rating} / 5 顆星
										</span>
									</div>
									<Textarea
										variant="bordered"
										size="lg"
										value={d.content}
										onChange={(e) =>
											setDrafts((prev) => ({
												...prev,
												[key]: { ...d, content: e.target.value },
											}))
										}
										placeholder="分享你的使用體驗：新鮮度 / 包裝 / 物流 / 推不推薦等"
										minRows={4}
										maxLength={500}
									/>

									<div className="mt-3 flex flex-wrap items-center gap-2">
										{d.photos.map((photo, i) => (
											<span
												key={photo.stagedToken || `${key}-photo-${i}`}
												className="relative size-[72px] overflow-hidden rounded-[var(--hkui-radius-md)] border border-[var(--hkui-divider)]"
											>
												<AppImage
													src={photo.previewUrl}
													alt={`評價照片 ${i + 1}`}
													fill
													sizes="72px"
													className="object-cover"
												/>
												<button
													type="button"
													aria-label="移除照片"
													className="absolute right-0.5 top-0.5 flex size-5 items-center justify-center rounded-full bg-black/55 text-white leading-none"
													onClick={() => handleRemovePhoto(item, i)}
												>
													<svg
														width="12"
														height="12"
														viewBox="0 0 24 24"
														fill="none"
														stroke="currentColor"
														strokeWidth="2.5"
														strokeLinecap="round"
														aria-hidden="true"
													>
														<path d="M18 6L6 18M6 6l12 12" />
													</svg>
												</button>
											</span>
										))}
										{d.photos.length < REVIEW_PHOTO_LIMIT ? (
											<label
												className={`flex size-[72px] flex-col items-center justify-center gap-1 rounded-[var(--hkui-radius-md)] border border-dashed border-[var(--hkui-divider)] text-[color:var(--hkui-foreground-soft)] transition-colors hover:border-[var(--hkui-primary)] hover:text-[color:var(--hkui-primary)] ${
													d.uploading ? "cursor-wait" : "cursor-pointer"
												}`}
												aria-busy={d.uploading}
												aria-disabled={d.uploading || undefined}
											>
												{d.uploading ? (
													<span
														className="flex size-full flex-col items-center justify-center gap-1"
														role="status"
														aria-live="polite"
														aria-atomic="true"
													>
														<UploadLoadingIndicator
															size={22}
															className="size-[22px] shrink-0"
														/>
														<span className="max-w-16 text-center text-[0.625rem] leading-[1.15]">
															上傳評價照片中…
														</span>
													</span>
												) : (
													<>
														<svg
															width="22"
															height="22"
															viewBox="0 0 24 24"
															fill="none"
															stroke="currentColor"
															strokeWidth="1.8"
															strokeLinecap="round"
															strokeLinejoin="round"
															aria-hidden="true"
														>
															<rect x="3" y="3" width="18" height="18" rx="3" />
															<circle cx="8.5" cy="8.5" r="1.5" />
															<path d="M21 15l-5-5L5 21" />
														</svg>
														<span className="text-[length:var(--hkui-text-tiny)]">
															{d.photos.length}/{REVIEW_PHOTO_LIMIT}
														</span>
													</>
												)}
												<input
													type="file"
													accept="image/*"
													multiple
													className="hidden"
													disabled={d.uploading}
													onChange={(e) => {
														handlePickPhotos(item, e.target.files);
														e.target.value = "";
													}}
												/>
											</label>
										) : null}
									</div>

									<div className="flex justify-end mt-3">
										<Button
											disabled={
												submittingItem === key ||
												!d.content.trim() ||
												d.rating < 1 ||
												d.uploading
											}
											isLoading={submittingItem === key}
											loadingLabel="送出評價中"
											onClick={() => handleSubmit(item)}
										>
											送出評價
										</Button>
									</div>
								</>
							)}
						</section>
					);
				})}

				{error && (
					<p
						className="my-[var(--hkui-space-4)] p-[var(--hkui-space-3)_var(--hkui-space-4)] rounded-[var(--hkui-radius-md)] bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)] text-[length:var(--hkui-text-small)]"
						role="alert"
					>
						{error}
					</p>
				)}

				<ImageCropDialog
					files={pendingCrop?.files ?? []}
					crop="review"
					onCropped={(file) => {
						croppedFilesRef.current.push(file);
					}}
					onClose={() => {
						const target = pendingCrop;
						const cropped = croppedFilesRef.current;
						croppedFilesRef.current = [];
						setPendingCrop(null);
						if (target && cropped.length > 0) {
							void handleAddPhotos(target.item, cropped);
						}
					}}
				/>

				{allDone && (
					<div className="flex gap-2 flex-wrap justify-end mt-4 w-full">
						<Button onClick={() => router.push(`/account/orders/${orderNo}`)}>
							全部評完，回訂單
						</Button>
					</div>
				)}
			</SellerPageShell>
		</main>
	);
}
