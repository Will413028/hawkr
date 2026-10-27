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
import {
	detectPostTaggingMode,
	extractPostTags,
	POST_TAG_LIMIT,
	type PostTaggingMode,
	type PostTagSuggestion,
	suggestPostTags,
} from "@/lib/post-tags/repo";
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
}

export default function PostComposeClient({
	edit,
}: {
	edit?: PostComposeEditTarget;
} = {}) {
	const router = useObservedRouter("seller_post_compose");
	const scope = useSellerDataScope();
	const delegated = scope.mode === "delegated";
	// 賣家路徑沒有 provider ⇒ null，這一支元件在 `/seller-center` 上行為逐位元不變。
	const delegatedSession = useOptionalDelegatedSession();
	// 未發布與發布分權是刻意的（L3207）：只核發 `posts.write` 的工作階段只能建立
	// 未發布的貼文，發布是另一支端點、另一個 capability。
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
	// 同 form 在 create / edit 切換時不一定 unmount(由賣家中心 layout 內部
	// 路由處理),用 edit?.id 當 session boundary,從編輯一篇 post 切到新增
	// 或切到另一篇時,上次的錯誤訊息不會殘留。
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

	// seller-self 與 navbar / SellerCenterShell 共用同一個 RQ cache entry
	// （GetMyStore 只發一次）。
	// 商品清單走公開端點，key 掛 sellers.products。Mirrors the App's
	// `listSellerProducts(mid)` call.
	const { data: seller, loading: sellerLoading } = useSellerSelf();
	// 代行下店家鍵來自工作階段（`storefront.read` 可以不核發，那時 seller 為 null）。
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
			// fetchSellerProducts is soft-fail (resolves null when unreachable).
			// Rethrow instead of caching the miss: a resolved null would be stored
			// as a *successful* empty list for staleTime 30s — an invisible outage
			// the picker renders as「沒有商品」. Throwing keeps RQ's error state
			// (retry on next mount) and surfaces it to the QueryCache onError.
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

	// Revoke object URLs to avoid leaking blob memory on unmount / re-pick.
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
		// Reset value so picking the same file twice still triggers `change`.
		event.target.value = "";
		if (!file) return;
		const isGif = isGifUploadFile(file);
		if (!file.type.startsWith("image/") && !isGif) {
			setError("僅支援圖片檔案（JPEG / PNG / WebP / GIF）");
			return;
		}
		// Normalize the key/MIME before the shared editor. ImageCropDialog loads its
		// animation-aware worker only after confirmation, so GIF bytes keep every
		// frame while still receiving the same seller-authored 4:3 framing.
		if (isGif) {
			setPendingImage(normalizeGifUploadFile(file));
			return;
		}
		// Crop to the feed-card 4:3 framing first; the confirmed file lands
		// in handleCroppedImage below.
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
		// 賣家自助建立會直接公開；代行只有「儲存並發布」會公開。兩條路徑都必須
		// 以已確認的 imagePreview 為準：pendingImage 尚在裁切，不算已附照片。
		// 這一層是最後防線，避免 Enter submit 或未來新增 caller 繞過按鈕狀態。
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
		// 代行下開確認抽屜（§3.4 三件必備）；賣家路徑立刻放行且不帶 meta。
		// ⚠️「建立即發布」是**兩支端點、兩筆稽核列**：`POST .../posts` 一律建為未發布
		// （帶 publishMode 會 9260），再 `POST .../posts/{postId}/publish`。修改原因與
		// 通知賣家只收一次、同值套用兩支呼叫 —— 操作者的意圖是「這次代行要不要通知
		// 賣家」，不是「建立要不要、發布要不要」。
		const publishAfterCreate =
			delegated && !edit && canPublishPosts && intent === "publish";
		// 排程值域：now+5 分鐘 ≤ t ≤ now+90 天（L3225）。只擋過去時間是不夠的 ——
		// 那等於接受「1 秒後」（與立即發布競態）與「10 年後」（永久待執行列），
		// 兩者都不會報錯。
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
		// 「儲存，暫不發布」在代行下不開抽屜：三件必備的 ①③ 就在底部條上，② 對新資源
		// 不存在。⚠️ 這條路徑因此是**唯一**不經確認抽屜的代行寫入，所以修改原因的
		// 值域檢查（trim 後 1–100）必須在這裡自己做一次——抽屜的那份守不到它。
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
					// 店名用引號括起：中英混排的店名直接串接會與下一個中文字黏在一起。
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
					// 底部條已填的值帶進抽屜；賣家路徑不會走到這裡的 meta（`confirmMutation`
					// 立刻放行且不帶 meta），兩個初值對它是空操作。
					initialReason: barReasonTrimmed,
					initialNotifySeller: barNotifySeller,
					// §3.4 ②「目前版本」是三件必備之一，而建立類是它唯一合法缺席的
					// 情境。空著與「漏畫了」在畫面上分不出來 —— 商品建立頁已經用
					// `versionNote` 補上這一格，貼文這一支沒有，於是同一種寫入在兩個
					// 面上長得不一樣，操作者無從判斷這裡是「沒有版本」還是「壞了」。
					//
					// 發布那一支確實會用到版本，只是它取自這一次 create 的回應（見下方
					// publishPost），操作者此刻無從得知也無從選擇 —— 所以文案講清楚它
					// 從哪裡來，而不是含糊寫「不適用」。
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
				// Mirror the App's keep/replace image contract: upload only when
				// the seller picked a new file; otherwise "keep" the existing one.
				// Both empty can only reach this branch for a non-publishing delegated
				// draft; seller-side public updates are stopped by the image guard above.
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
						productId,
						tags,
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
						// ⚠️ `expectedVersion` 取自**這一次 create 的回應**：新資源的版本沒有
						// 第二個來源，而 `/publish` 必填它（缺了是 9260／400，症狀是貼文停在
						// 未發布而畫面已經導回列表）。建立那一步的 meta 不帶版本（新資源無版本
						// 可比對），所以在這裡補。
						{ ...decision.meta, expectedVersion: created.version },
						scheduledPublishAt
							? { publishMode: "scheduled", scheduledPublishAt }
							: { publishMode: "immediate" },
					);
				}
			}
			// SellerPostsClient's list query has a 30s staleTime — router.refresh()
			// only re-runs Server Component data, not this TanStack Query cache, so
			// without awaiting the invalidation here the seller lands back on a
			// stale list until it refetches in the background.
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
	// 缺照片時保留按鈕的 focus/click，讓 guard 能彈出補圖提示；真正送出仍被
	// submitPost fail closed。照片已存在時，空內容維持原本的 native disabled。
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
					// 代行下這一行是**署名**（設計書 §6.6 的固定文案），不是可有可無的副標：
					// 拿不到店名時整行消失，消失的偏偏是「這則貼文以誰的名義發出」。店名退不
					// 到時退店家編號，仍然指得出是哪一家。賣家自己那條分支一個字都沒動。
					delegated
						? `以「${targetStoreLabel(sellerName, scope.ownerStoreId)}」名義，由 ADM 代行發布`
						: sellerName
							? `以「${sellerName}」名義發布`
							: undefined
				}
				actions={
					// 代行的送出在底部固定條（mockup §4.9.5）：未發布與發布是兩個 capability、
					// 兩筆稽核列，頂部單一顆「儲存」表達不了。
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
						// 賣家路徑只有一種送出；代行的兩顆按鈕各自帶 intent（暫不發布／發布是
						// 兩個 capability，也是兩筆稽核列）。
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

							{/* 標籤 chips（mockup §4.9.3）。代行專屬：賣家自助端的標籤沿用內文 hashtag
								的既有輸入方式，這裡多給一個 chips 面板讓代行者看得到自己加了什麼。 */}
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

							{/* 預計發布（mockup §4.9.3）。⚠️ 建立端點不收 `publishMode`／
								`scheduledPublishAt`（出現即 9260）—— 這一段的值是接著那一支
								`/publish` 的參數，所以只有拿得到 `posts.publish` 時才畫。 */}
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

						{/* 商店動態預覽（mockup §4.9.4）。純前端自繪、不打任何端點；
							互動計數不渲染 —— 新貼文沒有那個數字，畫 0 也是一個宣稱。 */}
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
											{/* ⚠️ 這是**預覽卡**不是確認關卡，兩者的退路刻意相反。
											    確認抽屜（`confirm-content.ts` 的 `targetStoreLabel`）
											    退不到店名時退店家編號 —— 那一刻要的是「唯一指向一家
											    店」。這裡要的是「這則貼文長什麼樣」，畫一串 19 位數字
											    既不是名字、也無法讓人確認代行的是不是對的那一家，所以
											    寧可寫「本店」（`tests/delegated-authorization-honesty.test.ts`
											    守著這條）。情境列與側欄都已經在講目標店家是誰。
											    實務上 `sellerName` 已退到 `summary.target.storeName`
											    （見上方 `sellerName` 的組法），這個 fallback 到不了。 */}
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
					{/* 代行原因（mockup §4.9.5 中段）。兩條路徑共用它：「儲存，暫不發布」直接
						用這裡的值送出，「儲存並發布」把它帶進確認抽屜當初值。 */}
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
						{/* 沒有 `posts.publish` 時，發布那顆**不渲染**（不是 disabled，L3857）。
							但只是拿掉它會讓畫面退回「只有一顆按鈕」＝看起來沒有選擇，所以補
							一句說明：不是操作者沒看到，是這次沒有這個授權。編輯既有貼文本來
							就沒有發布鍵（發布是另一支端點），那不是授權問題，不畫這一句。 */}
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
						{/* 主要按鈕是「儲存並發布」：貼文的常態是要見人的，把「暫不發布」畫成
							預設會讓代行者建了一堆賣家以為已經上線的貼文。缺 `posts.publish`
							時沒有主要按鈕可留，於是「暫不發布」升為 primary —— 一顆孤零零的
							次要樣式按鈕看起來像被停用的殘骸。 */}
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
