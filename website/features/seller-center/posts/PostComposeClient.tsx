"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AiSparkleIcon from "@/components/shared/AiSparkleIcon";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import { Input, Textarea } from "@/components/ui/Input";
import { targetStoreLabel } from "@/features/delegated-operations/confirm-content";
import { useOptionalDelegatedSession } from "@/features/delegated-operations/state/DelegatedSessionContext";
import AiImageEnhanceDialog from "@/features/seller/components/AiImageEnhanceDialog";
import { fetchSellerProducts } from "@/lib/api";
import { useResettableError } from "@/lib/hooks/useResettableError";
import { useInvalidateSellerPosts } from "@/lib/hooks/useSellerPosts";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { resolveCdnUrl } from "@/lib/media/cdn";
import {
	isGifMedia,
	isGifUploadFile,
	normalizeGifUploadFile,
} from "@/lib/media/gif";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { type PostTagSuggestion, suggestPostTags } from "@/lib/posts/repo";
import {
	detectPostTaggingMode,
	extractPostTags,
	POST_TAG_LIMIT,
	type PostTaggingMode,
} from "@/lib/posts/tags";
import { queryKeys } from "@/lib/query/keys";
import { storeIdOf } from "@/lib/seller/store";
import {
	OPERATION_REASON_MAX,
	type SellerMutationDecision,
	useSellerDataScope,
} from "@/lib/seller-data/scope";
import { highlightHashtags } from "./highlightHashtags";
import {
	POST_IMAGE_REQUIRED_MESSAGE,
	showPostImageRequiredToast,
} from "./postImageRequirement";
import { mapProductRecords } from "./productPickerAdapter";

/**
 * Seller post compose / edit — mirrors the App's `PostComposeScreen` and
 * `PostEditScreen` (Flutter,
 * `hawkr_seller/lib/features/post/presentation/`).
 *
 * Layout (top → bottom):
 *   1. Page head with back link + 新增/編輯貼文 title + 儲存 button
 *   2. Image preview card (4:3) with gallery picker + trash icons
 *   3. 貼文內容 white card with multi-line textarea
 *   4. 目前上架商品 white card with radio-style single-select list
 *
 * Create — still images and GIF animations both enter the same 4:3 framing
 * editor, then `uploadImageFile` (presigned URL flow) →
 * `createSellerPost({ stagedToken: asset.stagedToken, productId })`.
 * 注意：送 stagedToken（短效 token），非 previewUrl —— 後端 finalize 後才存成貼文圖片。
 *
 * Edit (`edit` prop set) — mirrors the App's keep/replace image contract:
 *   - 圖片沒動           → keepAssetId: "keep"
 *   - 換了新圖           → newStagedToken: <stagedToken>
 *   - 賣家移除圖片       → 發布 guard 要求補圖，不送出清空圖片的公開更新
 *   - 非公開代行草稿清空 → 兩者皆 ""
 *
 * AI image editing mirrors the App's top-right ✨ icon via
 * AiImageEnhanceDialog (category="post") — web doesn't have a
 * dedicated /photo-editor route, but the same enhance-dialog flow
 * applies directly to the already-picked/cropped image.
 */

/** 排程發布的值域（L3225／L3230-3239），與 Gateway 同一組邊界。 */
const SCHEDULE_MIN_LEAD_MS = 5 * 60 * 1000;
const SCHEDULE_MAX_LEAD_MS = 90 * 24 * 60 * 60 * 1000;

export interface PostComposeEditTarget {
	id: number | string;
	content: string;
	/** 既有貼文圖的 objectKey（`PostDetail.imageKeys[0]`），給預覽用。
	 *  名字必須誠實：存 key 就叫 *Key（49 §6）。它以前叫 imageUrl，而貼文的
	 *  單數 imageKey/imageUrl 正是 A0（編輯貼文清空 DB 圖片）的成因欄位。 */
	imageKey?: string;
	/** Currently linked product id (stringified) to pre-select. */
	productId?: string;
	/** 載入時的貼文版本（`MyPostDetail.etag`），儲存時帶回做樂觀鎖。 */
	etag?: string;
}

export default function PostComposeClient({
	edit,
}: {
	edit?: PostComposeEditTarget;
} = {}) {
	const router = useObservedRouter("seller_post_compose");
	const scope = useSellerDataScope();
	const delegated = scope.mode === "delegated";
	const delegatedSession = useOptionalDelegatedSession();
	const canPublishPosts = scope.hasCapability("posts.publish");
	const invalidateSellerPosts = useInvalidateSellerPosts();
	const isEdit = edit != null;
	const [content, setContent] = React.useState(edit?.content ?? "");
	const [imageFile, setImageFile] = React.useState<File | null>(null);
	const [imagePreview, setImagePreview] = React.useState<string | null>(
		edit?.imageKey ? resolveCdnUrl(edit.imageKey) : null,
	);
	const [aiOpen, setAiOpen] = React.useState(false);
	const [pendingImage, setPendingImage] = React.useState<File | null>(null);
	const [selectedProductId, setSelectedProductId] = React.useState<
		string | null
	>(edit?.productId ?? null);
	/**
	 * 送出中的是哪一條路徑。用意圖而不是布林：代行底部條有兩顆按鈕，布林會讓
	 * 「儲存並發布」的等待轉圈畫在「暫不發布」那顆上，操作者會以為自己按錯了。
	 */
	const [submittingIntent, setSubmittingIntent] = React.useState<
		null | "draft" | "publish"
	>(null);
	const submitting = submittingIntent !== null;
	/**
	 * 預計發布（mockup §4.9.3）。**只有代行有這一段**：賣家自助端的貼文沒有未發布／
	 * 排程分權，`POST .../posts` 一律建為未發布，帶 `publishMode` 會 9260 —— 排程是
	 * 接著那一支 `/publish` 的事（兩支端點、兩筆稽核列）。
	 */
	const [publishMode, setPublishMode] = React.useState<
		"immediate" | "scheduled"
	>("immediate");
	const [scheduledAt, setScheduledAt] = React.useState("");
	/**
	 * 底部固定條的「代行原因」與「通知賣家」（mockup §4.9.5）。
	 *
	 * **只有代行有這一段**，而且它是「儲存，暫不發布」這條路徑的**唯一**輸入點：
	 * 建立未發布的貼文沒有 `expectedVersion`（新資源）也不產生第二筆稽核列，三件
	 * 必備只剩 ①③，兩者都畫在底部條上了，再開一次抽屜只是多一次點擊（定案 §12）。
	 * 「儲存並發布」仍然開抽屜（兩支端點、兩筆稽核列要交代），並把這裡填的值帶進去
	 * 當初值——同一次寫入不該要求操作者打兩次理由。
	 */
	const [barReason, setBarReason] = React.useState("");
	const [barNotifySeller, setBarNotifySeller] = React.useState(true);
	const [barReasonTouched, setBarReasonTouched] = React.useState(false);
	const [tagInput, setTagInput] = React.useState("");
	const [error, setError] = useResettableError(edit?.id ?? "new");
	const fileInputRef = React.useRef<HTMLInputElement>(null);
	const imagePickerButtonRef = React.useRef<HTMLButtonElement>(null);
	const textareaRef = React.useRef<HTMLTextAreaElement>(null);
	const [taggingMode, setTaggingMode] = React.useState<PostTaggingMode | null>(
		null,
	);
	const [tagSuggestions, setTagSuggestions] = React.useState<
		PostTagSuggestion[]
	>([]);
	const [tagSuggestionsLoading, setTagSuggestionsLoading] =
		React.useState(false);

	const { data: seller, loading: sellerLoading } = useSellerSelf();
	const sellerId = scope.ownerStoreId ?? storeIdOf(seller);
	const postsHref = delegated
		? `/adm/${encodeURIComponent(scope.ownerStoreId ?? "")}/posts`
		: "/seller-center/posts";
	const productsQuery = useQuery({
		queryKey: queryKeys.sellers.products(sellerId ?? "none", {
			page: 1,
			pageSize: 50,
		}),
		queryFn: async () => {
			const data = await fetchSellerProducts(String(sellerId), 1, 50);
			if (data == null) throw new Error("seller products unreachable");
			const records =
				(data?.records as Array<Record<string, unknown>> | undefined) ?? [];
			return mapProductRecords(records);
		},
		enabled: sellerId != null,
		staleTime: 30_000,
	});
	const products = productsQuery.data ?? [];
	const productsLoading = sellerLoading || productsQuery.isLoading;

	React.useEffect(() => {
		return () => {
			if (imagePreview?.startsWith("blob:")) URL.revokeObjectURL(imagePreview);
		};
	}, [imagePreview]);

	const handlePickImage = () => {
		fileInputRef.current?.click();
	};

	const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0];
		event.target.value = "";
		if (!file) return;
		const isGif = isGifUploadFile(file);
		if (!file.type.startsWith("image/") && !isGif) {
			setError("僅支援圖片檔案（JPEG / PNG / WebP / GIF）");
			return;
		}
		if (isGif) {
			setPendingImage(normalizeGifUploadFile(file));
			return;
		}
		setPendingImage(file);
	};

	const handleCroppedImage = (file: File) => {
		if (imagePreview?.startsWith("blob:")) URL.revokeObjectURL(imagePreview);
		setImageFile(file);
		setImagePreview(URL.createObjectURL(file));
		setError(null);
	};

	const handleAiApply = (enhanced: Blob) => {
		const file = new File([enhanced], `ai-${Date.now()}.jpg`, {
			type: enhanced.type || "image/jpeg",
		});
		if (imagePreview?.startsWith("blob:")) URL.revokeObjectURL(imagePreview);
		setImageFile(file);
		setImagePreview(URL.createObjectURL(file));
	};

	const handleRemoveImage = () => {
		if (imagePreview?.startsWith("blob:")) URL.revokeObjectURL(imagePreview);
		setImageFile(null);
		setImagePreview(null);
	};

	const promptForRequiredImage = React.useCallback(() => {
		showPostImageRequiredToast();
		window.requestAnimationFrame(() => imagePickerButtonRef.current?.focus());
	}, []);

	const isAnimatedGifPreview = Boolean(
		imagePreview &&
			(imageFile?.type === "image/gif" ||
				isGifMedia(imageFile?.name ?? edit?.imageKey ?? imagePreview)),
	);

	const recomputeTaggingMode = React.useCallback((value: string) => {
		const textarea = textareaRef.current;
		const caret = textarea?.selectionStart ?? value.length;
		setTaggingMode(detectPostTaggingMode(value.slice(0, caret)));
	}, []);

	React.useEffect(() => {
		const mode = taggingMode;
		if (!mode) {
			setTagSuggestions([]);
			setTagSuggestionsLoading(false);
			return;
		}
		const controller = new AbortController();
		const timer = window.setTimeout(() => {
			setTagSuggestionsLoading(true);
			suggestPostTags({
				q: mode.prefix,
				mode: "prefix",
				limit: 10,
				signal: controller.signal,
			})
				.then((items) => setTagSuggestions(items))
				.catch(() => setTagSuggestions([]))
				.finally(() => setTagSuggestionsLoading(false));
		}, 180);
		return () => {
			window.clearTimeout(timer);
			controller.abort();
		};
	}, [taggingMode]);

	const handleSelectPostTag = React.useCallback(
		(name: string) => {
			const mode = taggingMode;
			if (!mode) return;
			const before = content.slice(0, mode.hashAt);
			const after = content.slice(mode.hashAt + 1 + mode.prefix.length);
			const inserted = `#${name} `;
			const next = `${before}${inserted}${after}`;
			const nextCaret = before.length + inserted.length;
			setContent(next);
			setTaggingMode(null);
			setTagSuggestions([]);
			window.requestAnimationFrame(() => {
				const textarea = textareaRef.current;
				if (!textarea) return;
				textarea.focus();
				textarea.setSelectionRange(nextCaret, nextCaret);
			});
		},
		[content, taggingMode],
	);

	const submitPost = async (intent: "draft" | "publish") => {
		const publishing = !delegated || intent === "publish";
		if (publishing && !imagePreview) {
			promptForRequiredImage();
			return;
		}
		const trimmed = content.trim();
		if (!trimmed) {
			setError("請輸入貼文內容");
			return;
		}
		const publishAfterCreate =
			delegated && !edit && canPublishPosts && intent === "publish";
		let scheduledPublishAt: string | undefined;
		if (publishAfterCreate && publishMode === "scheduled") {
			const at = Date.parse(scheduledAt);
			if (!Number.isFinite(at)) {
				setError("請選擇排程發布的日期與時間。");
				return;
			}
			if (at < Date.now() + SCHEDULE_MIN_LEAD_MS) {
				setError("排程時間至少要在 5 分鐘之後。");
				return;
			}
			if (at > Date.now() + SCHEDULE_MAX_LEAD_MS) {
				setError("排程時間最多只能排到 90 天之後。");
				return;
			}
			scheduledPublishAt = new Date(Math.floor(at / 1000) * 1000)
				.toISOString()
				.replace(/\.\d{3}Z$/, "Z");
		}
		const inlineDraft = delegated && !edit && !publishAfterCreate;
		const barReasonTrimmed = barReason.trim();
		if (inlineDraft) {
			setBarReasonTouched(true);
			if (
				barReasonTrimmed.length < 1 ||
				barReasonTrimmed.length > OPERATION_REASON_MAX
			) {
				setError(
					`請填寫代行原因（1–${OPERATION_REASON_MAX} 字），它會寫進這次操作的代行紀錄。`,
				);
				return;
			}
		}
		const decision: SellerMutationDecision = inlineDraft
			? {
					ok: true,
					meta: {
						operationReason: barReasonTrimmed,
						notifySeller: barNotifySeller,
					},
				}
			: await scope.confirmMutation({
					title: edit
						? "檢視變更並儲存"
						: publishAfterCreate
							? "檢視內容並儲存後發布"
							: "檢視內容並儲存（暫不發布）",
					intent: `以 ADM 身分替「${targetStoreLabel(sellerName, scope.ownerStoreId)}」${
						edit
							? "編輯貼文"
							: publishAfterCreate
								? "建立並發布貼文"
								: "建立貼文（暫不發布）"
					}`,
					hint: "請確認以下內容無誤後執行。",
					reasonPlaceholder: publishAfterCreate
						? "例：協助新品上市內容上稿"
						: "例：協助修正貼文內容",
					confirmLabel: edit
						? "確認儲存"
						: publishAfterCreate
							? "確認儲存並發布"
							: "確認建立，暫不發布",
					initialReason: barReasonTrimmed,
					initialNotifySeller: barNotifySeller,
					...(edit
						? {}
						: {
								versionNote: publishAfterCreate
									? "不適用（新建貼文，發布時套用建立回應的版本）"
									: "不適用（新建貼文尚未有版本）",
							}),
					...(publishAfterCreate
						? { auditNote: "將產生兩筆不可竄改的代行紀錄（建立、發布）" }
						: {}),
				});
		if (!decision.ok) return;
		setSubmittingIntent(intent);
		setError(null);
		try {
			const productId = selectedProductId ?? undefined;
			const tags = extractPostTags(trimmed);
			if (edit) {
				let keepAssetId = "";
				let newStagedToken = "";
				if (imageFile) {
					const asset = await scope.repos.media.uploadImageFile(imageFile);
					newStagedToken = asset.stagedToken;
				} else if (imagePreview) {
					keepAssetId = "keep";
				}
				await scope.repos.posts.updateSellerPost(
					edit.id,
					{
						content: trimmed,
						keepAssetId: keepAssetId,
						newStagedToken: newStagedToken,
						productId: selectedProductId,
						tags,
						etag: edit.etag,
					},
					decision.meta,
				);
			} else {
				let stagedToken: string | undefined;
				if (imageFile) {
					const asset = await scope.repos.media.uploadImageFile(imageFile);
					stagedToken = asset.stagedToken;
				}
				const created = await scope.repos.posts.createSellerPost(
					{
						content: trimmed,
						stagedToken: stagedToken,
						productId,
						tags,
					},
					decision.meta,
				);
				if (publishAfterCreate && created.id != null && decision.meta) {
					await scope.delegatedCommands?.publishPost(
						created.id,
						{ ...decision.meta, expectedVersion: created.version },
						scheduledPublishAt
							? { publishMode: "scheduled", scheduledPublishAt }
							: { publishMode: "immediate" },
					);
				}
			}
			await invalidateSellerPosts();
			router.push(postsHref);
			router.refresh();
		} catch (e) {
			setError(
				e instanceof Error ? e.message : isEdit ? "更新失敗" : "儲存失敗",
			);
			setSubmittingIntent(null);
		}
	};

	/**
	 * 顯示用的店名。
	 *
	 * 代行下 `useSellerSelf()` 可能整支沒有資料（`storefront.read` 可以不核發），此時
	 * 退到工作階段 target 的 `storeName` —— 那是 `GET current` 一定會帶的欄位，且它
	 * 正是這次代行綁定的那一家店。少了這一階，預覽卡的作者名會退到 `ownerStoreId`，
	 * 畫面上就是一串 19 位數字：對讀的人來說那既不是店名、也看不出是不是對的店。
	 */
	const sellerName =
		seller?.store?.storeName ||
		delegatedSession?.summary?.target.storeName ||
		"";
	const barReasonLength = barReason.trim().length;
	const barReasonValid =
		barReasonLength >= 1 && barReasonLength <= OPERATION_REASON_MAX;
	const currentTags = React.useMemo(() => extractPostTags(content), [content]);
	const hasContent = content.trim().length > 0;
	const hasImage = Boolean(imagePreview);
	const canSaveDraft = hasContent && !submitting;
	const sellerSubmitDisabled = submitting || (hasImage && !hasContent);
	const sellerImageRequired = !delegated && !hasImage;
	const delegatedPublishImageRequired = delegated && !isEdit && !hasImage;
	const selectedProduct =
		products.find((product) => product.id === selectedProductId) ?? null;
	/**
	 * datepicker 的 min／max 直接對齊後端值域（L3239 明令不得退化成「只擋過去
	 * 時間」）。`datetime-local` 吃的是本地時間字串，所以這裡不能用 toISOString。
	 */
	const scheduleBounds = React.useMemo(() => {
		const local = (at: number) => {
			const d = new Date(at - new Date(at).getTimezoneOffset() * 60_000);
			return d.toISOString().slice(0, 16);
		};
		const now = Date.now();
		return {
			min: local(now + SCHEDULE_MIN_LEAD_MS),
			max: local(now + SCHEDULE_MAX_LEAD_MS),
		};
	}, []);
	/**
	 * 標籤 chips（mockup §4.9.3）。
	 *
	 * ⚠️ chips **不是第二份資料**：貼文的 `tags` 一律由內文的 `#hashtag` 萃取
	 * （`extractPostTags`，送出時也是它），所以加減 chip 就是加減內文裡的那個
	 * token。另存一份陣列會讓「內文改了、chips 沒改」變成兩個都對不上的真相。
	 */
	const removeTag = (tag: string) => {
		setContent((prev) =>
			prev
				.replace(new RegExp(`#${tag}(?![\\p{L}\\p{N}_])`, "gu"), "")
				.replace(/[ \t]{2,}/g, " ")
				.trimEnd(),
		);
	};
	const addTag = () => {
		const raw = tagInput.trim().replace(/^#/, "");
		if (!raw) return;
		if (!/^[\p{L}\p{N}_]+$/u.test(raw)) {
			setError("標籤只能使用文字、數字與底線。");
			return;
		}
		if (currentTags.length >= POST_TAG_LIMIT) {
			setError(`標籤最多 ${POST_TAG_LIMIT} 個。`);
			return;
		}
		if (!currentTags.some((tag) => tag.toLowerCase() === raw.toLowerCase())) {
			setContent(
				(prev) => `${prev.trimEnd()}${prev.trim() ? " " : ""}#${raw} `,
			);
		}
		setTagInput("");
	};
	const exactSuggestionMatch =
		taggingMode &&
		tagSuggestions.some(
			(item) =>
				item.name.toLowerCase() === taggingMode.prefix.trim().toLowerCase(),
		);
	const showCreateTag =
		Boolean(taggingMode?.prefix) &&
		!tagSuggestionsLoading &&
		!exactSuggestionMatch;

	return (
		<div className="post-compose">
			<BackLink
				fallbackHref={postsHref}
				className="inline-flex items-center gap-[var(--hkui-space-2)] text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)] hover:text-[var(--hkui-foreground)] transition-colors no-underline post-compose__back"
			>
				<AppIcon name="arrow-left" size={16} />
				返回
			</BackLink>
			<SellerPageShell
				title={isEdit ? "編輯貼文" : "新增貼文"}
				subtitle={
					delegated
						? `以「${targetStoreLabel(sellerName, scope.ownerStoreId)}」名義，由 ADM 代行發布`
						: sellerName
							? `以「${sellerName}」名義發布`
							: undefined
				}
				actions={
					delegated ? null : (
						<Button
							type="submit"
							form="post-compose-form"
							className="post-compose__save"
							disabled={sellerSubmitDisabled}
							aria-disabled={sellerImageRequired || undefined}
							aria-describedby={
								sellerImageRequired
									? "post-compose-image-requirement"
									: undefined
							}
							data-image-required={sellerImageRequired || undefined}
							title={
								sellerImageRequired ? POST_IMAGE_REQUIRED_MESSAGE : undefined
							}
							isLoading={submitting}
							loadingLabel="儲存貼文中…"
						>
							儲存
						</Button>
					)
				}
			>
				<form
					id="post-compose-form"
					className={`post-compose__form${delegated ? " adm-post-compose" : ""}`}
					onSubmit={(event) => {
						event.preventDefault();
						void submitPost(delegated && canPublishPosts ? "publish" : "draft");
					}}
				>
					<div className="post-compose__split">
						<section
							className="post-compose__media"
							data-empty={imagePreview ? undefined : "true"}
						>
							{imagePreview ? (
								<AppImage
									src={imagePreview}
									alt="貼文預覽"
									fill
									sizes="(max-width: 768px) 100vw, 480px"
									className="post-compose__media-img"
								/>
							) : (
								<div className="post-compose__media-empty">
									<AppIcon name="image" size={72} title="還沒選圖片" />
									<p id="post-compose-image-requirement">
										發布貼文前請先加入一張照片，點下方相片圖示選取
									</p>
								</div>
							)}
							{isAnimatedGifPreview ? (
								<span className="absolute left-3 top-3 z-[1] rounded-full bg-black/70 px-2.5 py-1 text-xs font-bold text-white backdrop-blur-sm">
									GIF · 動畫已保留
								</span>
							) : null}

							{imagePreview ? (
								<button
									type="button"
									className="post-compose__media-btn post-compose__media-btn--trash"
									onClick={handleRemoveImage}
									aria-label="移除圖片"
								>
									<AppIcon name="delete" size={20} />
								</button>
							) : null}

							<button
								ref={imagePickerButtonRef}
								type="button"
								className="post-compose__media-btn post-compose__media-btn--gallery"
								onClick={handlePickImage}
								aria-label={imagePreview ? "更換圖片" : "選擇圖片"}
							>
								<AppIcon name="image" size={20} />
							</button>
							{imagePreview && !isAnimatedGifPreview ? (
								<button
									type="button"
									className="post-compose__media-btn post-compose__media-btn--ai"
									onClick={() => setAiOpen(true)}
									aria-label="AI 圖片美化"
									title="AI 圖片美化"
								>
									<AiSparkleIcon size={20} />
								</button>
							) : null}
							<input
								ref={fileInputRef}
								type="file"
								accept="image/*"
								className="post-compose__file-input"
								onChange={handleFileChange}
							/>
						</section>

						<ImageCropDialog
							files={pendingImage ? [pendingImage] : []}
							crop="post"
							onCropped={handleCroppedImage}
							onClose={() => setPendingImage(null)}
						/>

						<AiImageEnhanceDialog
							open={aiOpen}
							category="post"
							source={imageFile}
							onApply={handleAiApply}
							onClose={() => setAiOpen(false)}
						/>

						<div className="post-compose__stack">
							<section className="post-compose__card">
								<div className="post-compose__textarea-wrap">
									<Textarea
										ref={textareaRef}
										floatingLabel
										variant="bordered"
										size="lg"
										label="貼文內容"
										value={content}
										onChange={(e) => {
											const next = e.target.value;
											setContent(next);
											recomputeTaggingMode(next);
										}}
										onKeyUp={(e) => recomputeTaggingMode(e.currentTarget.value)}
										onClick={(e) => recomputeTaggingMode(e.currentTarget.value)}
										placeholder="分享產地故事、出貨狀況或當季推薦…"
										minRows={6}
										maxLength={2000}
										renderHighlight={highlightHashtags}
									/>
									{taggingMode ? (
										<div className="post-compose__hashtag-popover">
											{tagSuggestionsLoading ? (
												<div className="post-compose__hashtag-state">
													搜尋中…
												</div>
											) : null}
											{tagSuggestions.map((tag) => (
												<button
													key={tag.name}
													type="button"
													className="post-compose__hashtag-row"
													onClick={() => handleSelectPostTag(tag.name)}
												>
													<span>#{tag.name}</span>
													<small>{tag.postCount} 篇貼文</small>
												</button>
											))}
											{showCreateTag ? (
												<button
													type="button"
													className="post-compose__hashtag-row"
													onClick={() =>
														handleSelectPostTag(taggingMode.prefix)
													}
												>
													<span>#{taggingMode.prefix}</span>
													<small>建立新 hashtag</small>
												</button>
											) : null}
										</div>
									) : null}
								</div>
								<p className="post-compose__field-desc">
									輸入 # 可加標籤；買家會在動態牆看到這則內容
								</p>
								<small className="post-compose__hint">
									{content.length} / 2000 字 · # {currentTags.length} /{" "}
									{POST_TAG_LIMIT}
								</small>
							</section>


							{delegated ? (
								<section className="post-compose__card">
									<header className="post-compose__card-head">
										<h2>標籤</h2>
										<p>最多 {POST_TAG_LIMIT} 個；也可以直接在內文輸入 #標籤</p>
									</header>
									<ul className="adm-post-tags">
										{currentTags.map((tag) => (
											<li key={tag}>
												<span>#{tag}</span>
												<button
													type="button"
													onClick={() => removeTag(tag)}
													aria-label={`移除標籤 ${tag}`}
												>
													<AppIcon name="close" size={12} />
												</button>
											</li>
										))}
									</ul>
									<div className="adm-post-tags__add">
										<Input
											variant="bordered"
											size="lg"
											label="新增標籤"
											value={tagInput}
											onChange={(e) => setTagInput(e.target.value)}
											onKeyDown={(e) => {
												if (e.key !== "Enter") return;
												e.preventDefault();
												addTag();
											}}
											placeholder="例：產地直送"
											maxLength={20}
										/>
										<Button
											type="button"
											variant="bordered"
											onClick={addTag}
											disabled={tagInput.trim().length === 0}
										>
											加入
										</Button>
									</div>
								</section>
							) : null}

							<section className="post-compose__card">
								<header className="post-compose__card-head">
									<h2>目前上架商品</h2>
									<p>選一個商品關聯到這則貼文（可選）</p>
								</header>
								{productsLoading ? (
									<div className="post-compose__products-loading">
										<span className="post-compose__skeleton post-compose__skeleton--row" />
										<span className="post-compose__skeleton post-compose__skeleton--row" />
									</div>
								) : products.length === 0 ? (
									<div className="post-compose__products-empty">
										<AppIcon name="bag" size={48} />
										<p>暫無上架商品</p>
									</div>
								) : (
									<ul className="post-compose__products">
										{products.map((p) => {
											const selected = selectedProductId === p.id;
											return (
												<li key={p.id}>
													<button
														type="button"
														className="post-compose__product"
														data-selected={selected ? "true" : undefined}
														onClick={() =>
															setSelectedProductId(selected ? null : p.id)
														}
														aria-pressed={selected}
													>
														<span
															className="post-compose__product-radio"
															data-on={selected ? "true" : undefined}
															aria-hidden="true"
														>
															{selected ? (
																<AppIcon name="check" size={14} fill />
															) : null}
														</span>
														<span className="post-compose__product-thumb">
															{p.imageUrl ? (
																<AppImage
																	src={resolveCdnUrl(p.imageUrl)}
																	width={56}
																	height={56}
																	sizes="56px"
																	alt=""
																	loading="lazy"
																/>
															) : (
																<AppIcon name="image" size={20} />
															)}
														</span>
														<span className="post-compose__product-body">
															<span className="post-compose__product-name">
																{p.name}
															</span>
															<span className="post-compose__product-prices">
																<strong>{p.price.effectiveText}</strong>
																{p.price.isOnSale ? (
																	<s>{p.price.originalText}</s>
																) : null}
															</span>
														</span>
													</button>
												</li>
											);
										})}
									</ul>
								)}
							</section>


							{delegated && canPublishPosts && !isEdit ? (
								<section className="post-compose__card">
									<header className="post-compose__card-head">
										<h2>預計發布</h2>
										<p>排程時間需介於 5 分鐘後與 90 天內</p>
									</header>
									<div className="adm-post-schedule">
										<label className="adm-post-schedule__option">
											<input
												type="radio"
												name="adm-post-publish-mode"
												checked={publishMode === "immediate"}
												onChange={() => setPublishMode("immediate")}
											/>
											<span>立即發布</span>
										</label>
										<label className="adm-post-schedule__option">
											<input
												type="radio"
												name="adm-post-publish-mode"
												checked={publishMode === "scheduled"}
												onChange={() => setPublishMode("scheduled")}
											/>
											<span>排程發布</span>
										</label>
										{publishMode === "scheduled" ? (
											<Input
												type="datetime-local"
												variant="bordered"
												size="lg"
												label="發布時間"
												value={scheduledAt}
												onChange={(e) => setScheduledAt(e.target.value)}
												min={scheduleBounds.min}
												max={scheduleBounds.max}
											/>
										) : null}
									</div>
									<p className="post-compose__field-desc">
										工作階段結束不會取消已排程的發布；要取消請改由賣家本人或撤銷授權處理。
									</p>
								</section>
							) : null}
						</div>


						{delegated ? (
							<aside className="adm-post-preview" aria-label="商店動態預覽">
								<header className="adm-post-preview__head">
									<strong>商店動態預覽</strong>
									<span>預覽，不會真的互動</span>
								</header>
								<article className="adm-post-preview__card">
									<header className="adm-post-preview__author">
										<span
											className="adm-post-preview__avatar"
											aria-hidden="true"
										>
											{(sellerName || "店").slice(0, 1)}
										</span>
										<span>

											<strong>{sellerName || "本店"}</strong>
											<small>剛剛</small>
										</span>
									</header>
									<p className="adm-post-preview__content">
										{content.trim() || "這則貼文還沒有內容。"}
									</p>
									{imagePreview ? (
										// biome-ignore lint/performance/noImgElement: 預覽來源是本機 blob 或 CDN 動態網址。
										<img
											className="adm-post-preview__image"
											src={imagePreview}
											alt=""
										/>
									) : null}
									{currentTags.length > 0 ? (
										<p className="adm-post-preview__tags">
											{currentTags.map((tag) => (
												<span key={tag}>#{tag}</span>
											))}
										</p>
									) : null}
									{selectedProduct ? (
										<div className="adm-post-preview__product">
											{selectedProduct.imageUrl ? (
												// biome-ignore lint/performance/noImgElement: 商品縮圖是 CDN 動態網址。
												<img
													src={resolveCdnUrl(selectedProduct.imageUrl)}
													alt=""
												/>
											) : null}
											<span>
												<strong>{selectedProduct.name}</strong>
												<small>{selectedProduct.price.effectiveText}</small>
											</span>
										</div>
									) : null}
								</article>
							</aside>
						) : null}
					</div>

					{error ? (
						<p className="post-compose__error" role="alert">
							{error}
						</p>
					) : null}
				</form>
			</SellerPageShell>
			{delegated ? (
				<div className="adm-post-compose__bar" role="toolbar">
					<span className="adm-post-compose__bar-hint">
						<AppIcon name="shield" size={16} />
						所有內容變更將記錄操作者、目標店家與發布理由
					</span>

					<span className="adm-post-compose__bar-reason">
						<Input
							variant="bordered"
							size="sm"
							label="代行原因"
							isRequired
							value={barReason}
							onChange={(e) =>
								setBarReason(e.target.value.slice(0, OPERATION_REASON_MAX))
							}
							onBlur={() => setBarReasonTouched(true)}
							placeholder="例：協助新品上市內容上稿"
							description={`${barReasonLength} / ${OPERATION_REASON_MAX} 字`}
							errorMessage={
								barReasonTouched && !barReasonValid
									? `代行原因為必填，且不得超過 ${OPERATION_REASON_MAX} 字。`
									: undefined
							}
							isInvalid={barReasonTouched && !barReasonValid}
						/>
						<label className="adm-post-compose__bar-notify">
							<input
								type="checkbox"
								checked={barNotifySeller}
								onChange={(e) => setBarNotifySeller(e.target.checked)}
							/>
							<span>通知賣家</span>
						</label>
					</span>
					<span className="adm-post-compose__bar-actions">

						{canPublishPosts || isEdit ? null : (
							<span className="adm-post-compose__bar-note">
								本次工作階段沒有「貼文發布」授權，只能建立未發布的貼文；要讓買家看到，請由賣家自行發布。
							</span>
						)}
						<Button
							type="button"
							variant="bordered"
							disabled={submitting}
							onClick={() => router.push(postsHref)}
						>
							取消
						</Button>

						<Button
							type="button"
							variant={canPublishPosts && !isEdit ? "bordered" : "solid"}
							disabled={!canSaveDraft}
							isLoading={submittingIntent === "draft"}
							loadingLabel="正在寫入代行紀錄"
							onClick={() => void submitPost("draft")}
						>
							{isEdit ? "儲存變更" : "儲存，暫不發布"}
						</Button>
						{canPublishPosts && !isEdit ? (
							<Button
								type="button"
								className="post-compose__image-gated-action"
								disabled={submitting || (hasImage && !hasContent)}
								aria-disabled={delegatedPublishImageRequired || undefined}
								aria-describedby={
									delegatedPublishImageRequired
										? "post-compose-image-requirement"
										: undefined
								}
								data-image-required={delegatedPublishImageRequired || undefined}
								title={
									delegatedPublishImageRequired
										? POST_IMAGE_REQUIRED_MESSAGE
										: undefined
								}
								isLoading={submittingIntent === "publish"}
								loadingLabel="正在寫入代行紀錄"
								onClick={() => void submitPost("publish")}
							>
								儲存並發布
							</Button>
						) : null}
					</span>
				</div>
			) : null}
		</div>
	);
}
