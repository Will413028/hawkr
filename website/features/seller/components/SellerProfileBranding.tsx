"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import { targetStoreLabel } from "@/features/delegated-operations/confirm-content";
import SellerCoverMedia from "@/features/seller/components/SellerCoverMedia";
import { ossImageUrl } from "@/lib/media/cdn";
import type { StorefrontWriteResult } from "@/lib/seller/repo";
import { useSellerDataScope } from "@/lib/seller-data/scope";

/**
 * Seller storefront branding preview — scaled-down mock of the buyer-app
 * merchant page (`merchant_screen.dart` in hawkr_buyer) so the seller
 * sees their store the way buyers will.
 *
 * The phone preview is read-only chrome; the actual "更換大頭貼 / 更換背景"
 * affordances are externalised as callout pills with dashed indicator
 * lines pointing into the avatar / cover regions on the device. That keeps
 * the preview crisp and makes the editing action discoverable at a glance.
 */

export interface SellerProfileBrandingProps {
	logoUrl: string | null | undefined;
	coverUrl: string | null | undefined;
	storeName?: string | null;
	accountId?: string | number | null;
	/** 對外顯示碼；優先於 accountId 顯示在「ID :」欄位，僅供顯示，不影響下方網址預覽（仍走 accountId）。 */
	storeCode?: string | null;
	description?: string | null;
	city?: string | null;
	district?: string | null;
	/** Called after the image has been persisted on the store.
	 * The parent must refetch the store and use its canonical display URL.
	 *
	 * ⚠️ 帶著**這一次寫入**的回執：換一張圖就把店面版本（etag）推進一格，呼叫端得用它
	 * 更新自己的 etag，否則接著按「儲存商店資訊」必版本衝突。 */
	onSaved: (written: StorefrontWriteResult) => void | Promise<void>;
	onError?: (message: string, cause?: unknown) => void | Promise<void>;
	/** Reports whether a logo/cover upload is in flight, so the parent can
	 * gate its save action until the new image URL has been committed. */
	onUploadingChange?: (uploading: boolean) => void;
	/**
	 * 店面的 etag：換圖與儲存商店資訊共用同一個版本，每次寫入都必須帶上最新的那一個。
	 */
	storeEtag: string;
	/**
	 * 確認抽屜要指稱的目標店家名稱。
	 *
	 * 與 `storeName` 是**兩個不同的值**：那一個是預覽卡用的表單狀態，改名進行中時
	 * 它是還沒存進去的新名字，拿去說「替 X 更換頭像」會指稱一家目前並不存在的店。
	 * 未給時退回 `storeName`。
	 */
	savedStoreName?: string | null;
	/** 店家目前沒有店面編輯能力（例如停權）：換圖入口不渲染。 */
	readOnly?: boolean;
}

type SellerPreviewTab = "posts" | "shop" | "chat" | "reviews";
type PreviewImageField = "logo" | "cover_url";

export function SellerProfileBranding({
	logoUrl,
	coverUrl,
	storeName,
	accountId,
	storeCode,
	description,
	city,
	district,
	onSaved,
	onError,
	onUploadingChange,
	storeEtag,
	savedStoreName,
	readOnly = false,
}: SellerProfileBrandingProps) {
	const scope = useSellerDataScope();
	/**
	 * 換頭像／換封面是兩支獨立的寫入端點（`storefront.write`，§6.1.7）。缺這個
	 * capability 時整組上傳入口不渲染 —— 判斷放在**擁有那兩顆按鈕的元件裡**，
	 * 而不是由呼叫端傳旗標進來：後者只要有人忘記傳，寫入 CTA 就又長回來了。
	 * 預覽本身留著（那是唯讀的，正是唯讀工作階段要看的東西）。
	 *
	 * 賣家自己的路徑沒有 scope 能力：店家停權時 `GetMyStore.capabilities` 沒有 STOREFRONT_WRITE，
	 * 由已經讀了店家的呼叫端以 `readOnly` 傳入。
	 */
	const canWrite =
		!readOnly &&
		(scope.mode !== "delegated" || scope.hasCapability("storefront.write"));
	const [uploadingLogo, setUploadingLogo] = React.useState(false);
	const [uploadingCover, setUploadingCover] = React.useState(false);
	const [activeTab, setActiveTab] = React.useState<SellerPreviewTab>("shop");
	// 代行工作區的預覽預設落在「網頁」（mockup §4.4.7 的 segmented 以網頁為選中態）：
	// 代行是桌面瀏覽器上的管理作業，賣家自助端的主力才是手機 App。
	const [viewMode, setViewMode] = React.useState<"mobile" | "web">(
		scope.mode === "delegated" ? "web" : "mobile",
	);
	const coverInputRef = React.useRef<HTMLInputElement>(null);
	const avatarInputRef = React.useRef<HTMLInputElement>(null);
	const coverObjectUrlRef = React.useRef<string | null>(null);
	const logoObjectUrlRef = React.useRef<string | null>(null);
	const previousCoverUrlRef = React.useRef(coverUrl);
	const previousLogoUrlRef = React.useRef(logoUrl);
	const [localCoverPreviewUrl, setLocalCoverPreviewUrl] = React.useState<
		string | null
	>(null);
	const [localLogoPreviewUrl, setLocalLogoPreviewUrl] = React.useState<
		string | null
	>(null);
	const [cropTarget, setCropTarget] = React.useState<{
		field: PreviewImageField;
		file: File;
	} | null>(null);
	const remoteCoverPreviewUrl = coverUrl
		? ossImageUrl(coverUrl, "sellerCover")
		: "";
	const remoteLogoPreviewUrl = logoUrl ? ossImageUrl(logoUrl, "avatar") : "";
	const coverPreviewUrl = localCoverPreviewUrl || remoteCoverPreviewUrl;
	const logoPreviewUrl = localLogoPreviewUrl || remoteLogoPreviewUrl;

	const replaceLocalPreview = React.useCallback(
		(field: PreviewImageField, file: File | null) => {
			const objectRef =
				field === "cover_url" ? coverObjectUrlRef : logoObjectUrlRef;
			const setPreview =
				field === "cover_url"
					? setLocalCoverPreviewUrl
					: setLocalLogoPreviewUrl;
			if (objectRef.current) {
				URL.revokeObjectURL(objectRef.current);
				objectRef.current = null;
			}
			if (!file) {
				setPreview(null);
				return;
			}
			const nextPreview = URL.createObjectURL(file);
			objectRef.current = nextPreview;
			setPreview(nextPreview);
		},
		[],
	);

	React.useEffect(() => {
		return () => {
			if (coverObjectUrlRef.current)
				URL.revokeObjectURL(coverObjectUrlRef.current);
			if (logoObjectUrlRef.current)
				URL.revokeObjectURL(logoObjectUrlRef.current);
		};
	}, []);

	React.useEffect(() => {
		if (coverUrl !== previousCoverUrlRef.current) {
			previousCoverUrlRef.current = coverUrl;
			replaceLocalPreview("cover_url", null);
		}
	}, [coverUrl, replaceLocalPreview]);

	React.useEffect(() => {
		if (logoUrl !== previousLogoUrlRef.current) {
			previousLogoUrlRef.current = logoUrl;
			replaceLocalPreview("logo", null);
		}
	}, [logoUrl, replaceLocalPreview]);

	const handleUpload = React.useCallback(
		async (
			file: File,
			field: PreviewImageField,
			setBusy: (v: boolean) => void,
		) => {
			const logo = field === "logo";
			// 代行下 finalize 是一次會寫進稽核的代行寫入，要收齊三件必備；賣家路徑
			// 立刻放行且不帶 meta（行為與先前逐位元相同）。
			const decision = await scope.confirmMutation({
				title: logo ? "更換商店頭像" : "更換商店封面",
				intent: `以 ADM 身分替「${targetStoreLabel(
					savedStoreName || storeName,
					scope.ownerStoreId,
				)}」${logo ? "更換商店頭像" : "更換商店封面"}`,
				hint: "圖片會立即顯示在買家看到的商店頁。",
				reasonPlaceholder: logo
					? "例：協助更換商店頭像"
					: "例：協助更換商店封面",
				expectedVersion: storeEtag,
				confirmLabel: "確認更換",
			});
			if (!decision.ok) {
				replaceLocalPreview(field, null);
				return;
			}
			setBusy(true);
			try {
				// Upload, then finalize the staged token on the store — never persist
				// the tmp previewUrl.
				const uploaded = await scope.repos.media.uploadImageFile(file);
				const written = logo
					? await scope.repos.storefront.updateStoreLogo(
							storeEtag,
							uploaded.stagedToken,
							decision.meta,
						)
					: await scope.repos.storefront.updateStoreCover(
							storeEtag,
							uploaded.stagedToken,
							decision.meta,
						);
				await onSaved(written);
			} catch (e) {
				replaceLocalPreview(field, null);
				await onError?.(e instanceof Error ? e.message : "上傳失敗", e);
			} finally {
				setBusy(false);
			}
		},
		[
			onError,
			onSaved,
			replaceLocalPreview,
			savedStoreName,
			scope,
			storeName,
			storeEtag,
		],
	);

	// Picking a file opens the crop step first; upload starts only after the
	// user confirms the framing (see ImageCropDialog below).
	const onFileInput =
		(field: PreviewImageField) => (e: React.ChangeEvent<HTMLInputElement>) => {
			const f = e.target.files?.[0];
			if (f) setCropTarget({ field, file: f });
			e.target.value = "";
		};

	const handleCropped = React.useCallback(
		(file: File) => {
			if (!cropTarget) return;
			const { field } = cropTarget;
			replaceLocalPreview(field, file);
			void handleUpload(
				file,
				field,
				field === "logo" ? setUploadingLogo : setUploadingCover,
			);
		},
		[cropTarget, handleUpload, replaceLocalPreview],
	);

	React.useEffect(() => {
		onUploadingChange?.(uploadingLogo || uploadingCover);
	}, [uploadingLogo, uploadingCover, onUploadingChange]);

	const locationLine = [city, district].filter(Boolean).join(" · ") || "—";
	const switchViewMode = React.useCallback(
		(nextMode: "mobile" | "web") => {
			setViewMode(nextMode);
			if (nextMode === "mobile" && activeTab === "reviews") {
				setActiveTab("shop");
			}
		},
		[activeTab],
	);

	return (
		<aside
			className="@container flex w-full min-w-0 max-w-[860px] flex-col gap-4 self-start"
			aria-label="商店外觀預覽"
		>
			<header className="flex min-w-0 flex-nowrap items-center justify-between gap-3 rounded-[24px] border border-hkui-primary/10 bg-[linear-gradient(135deg,rgba(255,255,255,0.82),rgb(var(--color-ink-50-rgb)/0.64))] p-3 pl-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.72),0_14px_34px_-30px_rgb(var(--color-ink-950-rgb)/0.58)] backdrop-blur dark:border-[rgba(245,222,165,0.16)] dark:bg-[linear-gradient(135deg,rgba(255,255,255,0.07),rgba(255,255,255,0.025))] @max-[560px]:flex-col @max-[560px]:items-stretch @max-[560px]:p-3">
				<div className="flex min-w-0 flex-1 items-center gap-3">
					<span
						className="grid size-10 shrink-0 place-items-center rounded-full bg-hkui-primary/10 text-hkui-primary shadow-[inset_0_0_0_1px_rgb(var(--color-ink-900-rgb)/0.12)] dark:bg-[#f1cd70]/14 dark:text-[#f1cd70] dark:shadow-[inset_0_0_0_1px_rgba(241,205,112,0.18)]"
						aria-hidden="true"
					>
						<AppIcon name="preview" size={20} />
					</span>
					<span className="flex min-w-0 flex-col gap-0.5">
						<strong className="overflow-hidden text-ellipsis whitespace-nowrap text-sm font-bold text-foreground">
							商店外觀預覽
						</strong>
						<small className="overflow-hidden text-ellipsis whitespace-nowrap text-xs text-foreground-muted">
							{viewMode === "mobile"
								? "買家在手機 App 看到的店鋪頁"
								: "買家在網頁看到的店鋪頁"}
						</small>
					</span>
				</div>
				<div
					className="inline-flex shrink-0 gap-1 rounded-full border border-hkui-primary/15 bg-hkui-primary/8 p-1 shadow-[inset_0_1px_0_rgba(255,255,255,0.58),0_8px_20px_-18px_rgb(var(--color-ink-950-rgb)/0.42)] dark:border-[rgba(245,222,165,0.18)] dark:bg-white/7 @max-[560px]:w-full @max-[560px]:justify-stretch"
					role="tablist"
					aria-label="切換預覽裝置"
				>
					<button
						type="button"
						role="tab"
						aria-selected={viewMode === "mobile"}
						className={`inline-flex min-h-[34px] shrink-0 cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-full border px-3.5 text-xs font-bold transition-[background-color,border-color,box-shadow,color,transform] duration-160 hover:-translate-y-px @max-[560px]:flex-1 ${
							viewMode === "mobile"
								? "border-hkui-primary bg-hkui-primary text-hkui-primary-foreground shadow-[0_10px_22px_-12px_rgb(var(--color-ink-900-rgb)/0.62)] dark:border-[rgba(241,205,112,0.34)] dark:bg-[rgba(241,205,112,0.14)] dark:text-[#f1cd70] dark:shadow-[inset_0_1px_0_rgba(245,222,165,0.08),0_10px_22px_-16px_rgba(0,0,0,0.7)]"
								: "border-transparent bg-transparent text-foreground-muted hover:bg-content1 hover:text-foreground dark:hover:bg-white/8"
						}`}
						onClick={() => switchViewMode("mobile")}
					>
						<DeviceMobileIcon />
						<span>手機</span>
					</button>
					<button
						type="button"
						role="tab"
						aria-selected={viewMode === "web"}
						className={`inline-flex min-h-[34px] shrink-0 cursor-pointer items-center justify-center gap-1.5 whitespace-nowrap rounded-full border px-3.5 text-xs font-bold transition-[background-color,border-color,box-shadow,color,transform] duration-160 hover:-translate-y-px @max-[560px]:flex-1 ${
							viewMode === "web"
								? "border-hkui-primary bg-hkui-primary text-hkui-primary-foreground shadow-[0_10px_22px_-12px_rgb(var(--color-ink-900-rgb)/0.62)] dark:border-[rgba(241,205,112,0.34)] dark:bg-[rgba(241,205,112,0.14)] dark:text-[#f1cd70] dark:shadow-[inset_0_1px_0_rgba(245,222,165,0.08),0_10px_22px_-16px_rgba(0,0,0,0.7)]"
								: "border-transparent bg-transparent text-foreground-muted hover:bg-content1 hover:text-foreground dark:hover:bg-white/8"
						}`}
						onClick={() => switchViewMode("web")}
					>
						<DeviceWebIcon />
						<span>網頁</span>
					</button>
				</div>
			</header>

			<div
				className={`seller-profile-branding__stage relative isolate grid grid-cols-1 justify-items-center w-full max-w-[860px] h-[620px] min-h-[620px] overflow-visible box-border rounded-[34px] border border-hkui-primary/12 bg-[radial-gradient(circle_at_12%_0%,rgba(241,205,112,0.24),transparent_34%),radial-gradient(circle_at_84%_78%,rgb(var(--color-ink-400-rgb)/0.17),transparent_40%),linear-gradient(135deg,rgba(255,253,247,0.9),rgba(248,246,238,0.66))] shadow-[inset_0_1px_0_rgba(255,255,255,0.78),0_22px_54px_-42px_rgb(var(--color-ink-950-rgb)/0.62)] transition-[padding,box-shadow,border-color] duration-220 ease-[cubic-bezier(0.16,1,0.3,1)] dark:border-[rgba(245,222,165,0.18)] dark:bg-[radial-gradient(circle_at_16%_0%,rgba(241,205,112,0.12),transparent_34%),radial-gradient(circle_at_84%_70%,rgb(var(--color-ink-400-rgb)/0.14),transparent_38%),linear-gradient(135deg,rgba(255,255,255,0.055),rgba(255,255,255,0.022))] dark:shadow-[inset_0_1px_0_rgba(245,222,165,0.1),0_24px_58px_-44px_rgba(0,0,0,0.76)] @max-[760px]:h-auto @max-[760px]:min-h-0 ${
					viewMode === "web"
						? "content-center p-[24px_16px] @max-[760px]:p-[20px_12px]"
						: "p-[32px_130px] @max-[760px]:p-[32px_16px_140px]"
				}`}
				data-view-mode={viewMode}
			>
				{/* Hidden file inputs live at panel level so either preview's
				 * callouts can trigger them. A button cannot legally nest
				 * interactive content, and a disabled button can swallow a
				 * nested input's change event — so they stay siblings. */}
				{canWrite && (
					<>
						<input
							ref={coverInputRef}
							type="file"
							accept="image/*"
							hidden
							disabled={uploadingCover}
							onChange={onFileInput("cover_url")}
						/>
						<input
							ref={avatarInputRef}
							type="file"
							accept="image/*"
							hidden
							disabled={uploadingLogo}
							onChange={onFileInput("logo")}
						/>
						<ImageCropDialog
							files={cropTarget ? [cropTarget.file] : []}
							crop={cropTarget?.field === "logo" ? "avatar" : "sellerCover"}
							onCropped={handleCropped}
							onClose={() => setCropTarget(null)}
						/>
					</>
				)}

				{/* ── Phone preview ─────────────────────────────────── */}
				{viewMode === "mobile" && (
					<>
						<div className="relative w-[250px] max-w-full aspect-[9/19.5] min-h-[540px] flex-shrink-0">
							<span className="absolute bg-gradient-to-r from-[#1a1a1a] via-[#3a3a3c] to-[#1a1a1a] rounded-[1.5px] z-0 left-[-2px] top-[14%] w-[3px] h-[26px]" />
							<span className="absolute bg-gradient-to-r from-[#1a1a1a] via-[#3a3a3c] to-[#1a1a1a] rounded-[1.5px] z-0 left-[-3px] top-[22%] w-[4px] h-[44px] rounded-[2px]" />
							<span className="absolute bg-gradient-to-r from-[#1a1a1a] via-[#3a3a3c] to-[#1a1a1a] rounded-[1.5px] z-0 left-[-3px] top-[31%] w-[4px] h-[44px] rounded-[2px]" />
							<span className="absolute bg-gradient-to-r from-[#1a1a1a] via-[#3a3a3c] to-[#1a1a1a] rounded-[1.5px] z-0 right-[-3px] top-[24%] w-[4px] h-[64px] rounded-[2px]" />

							<div className="absolute inset-0 rounded-[46px] p-[11px] bg-gradient-to-br from-[#2a2a2c] via-[#050505] to-[#1c1c1e] shadow-[0_30px_60px_-30px_rgb(var(--color-ink-950-rgb)/0.55),0_8px_16px_-8px_rgba(0,0,0,0.4),inset_0_0_0_1.5px_rgba(255,255,255,0.08),inset_0_0_0_4px_#0a0a0a] dark:shadow-[0_32px_72px_-34px_rgba(0,0,0,0.88),0_10px_22px_-10px_rgba(0,0,0,0.74),inset_0_0_0_1.5px_rgba(245,222,165,0.06),inset_0_0_0_4px_#050807]">
								<div className="relative w-full h-full rounded-[35px] overflow-hidden bg-hk-cream text-[#1a1a1a] text-xs leading-[1.4] flex flex-col dark:bg-hk-ink-950 dark:text-[#f5e9c3]">
									{/* The preview uses the same fixed 4:1 background ratio as
									 * the responsive public seller page and upload editor. */}
									<div
										className={`relative block w-full aspect-[4/1] text-hk-primary-strong bg-hk-ink-100 overflow-hidden dark:bg-hk-ink-850 dark:text-[#f1cd70] ${
											coverPreviewUrl ? "text-hk-cream" : ""
										}`}
										aria-hidden="true"
									>
										{coverPreviewUrl ? (
											<SellerCoverMedia
												src={coverPreviewUrl}
												sizes="228px"
												quality={60}
											/>
										) : (
											<CoverPattern />
										)}
										{uploadingCover && (
											<span className="absolute inset-0 z-[3] flex items-center justify-center bg-black/50 text-white">
												<UploadLoadingIndicator
													size={28}
													className="size-7 shrink-0"
												/>
											</span>
										)}
									</div>

									<StatusBar />
									<TopBar />

									<div className="flex items-center gap-2.5 px-3 -mt-7 relative z-2">
										<div
											className="relative flex items-center justify-center w-14 h-14 rounded-full bg-hk-cream border-3 border-hk-cream overflow-hidden shadow-[0_3px_10px_rgba(0,0,0,0.14)] text-foreground-soft flex-shrink-0 dark:bg-hk-ink-850 dark:border-hk-ink-850 dark:text-white/62 dark:shadow-[0_10px_22px_rgba(0,0,0,0.34)]"
											aria-hidden="true"
										>
											{logoPreviewUrl ? (
												// biome-ignore lint/performance/noImgElement: blob/CDN preview
												<img
													src={logoPreviewUrl}
													alt=""
													className="w-full h-full object-cover rounded-full"
												/>
											) : null}
											{uploadingLogo && (
												<span className="absolute inset-0 flex items-center justify-center bg-black/40 text-white rounded-full">
													<UploadLoadingIndicator
														size={22}
														className="size-[22px] shrink-0"
													/>
												</span>
											)}
										</div>
										<div className="flex flex-col gap-[2px] min-w-0 pt-7">
											<strong className="text-[0.8125rem] font-bold overflow-hidden text-ellipsis whitespace-nowrap text-[#1a1a1a] dark:text-[#f5e9c3]">
												{storeName?.trim() || "店舖名稱"}
											</strong>
											<small className="text-[0.625rem] text-[#565656] overflow-hidden text-ellipsis whitespace-nowrap dark:text-white/62">
												ID : {storeCode ?? accountId ?? "—"}
											</small>
											<small className="text-[0.625rem] text-[#565656] overflow-hidden text-ellipsis whitespace-nowrap mt-[1px] dark:text-white/62">
												{locationLine}
											</small>
										</div>
									</div>

									{description?.trim() && (
										<p className="m-[8px_14px_0] text-[0.6875rem] leading-[1.45] text-[#565656] line-clamp-2 overflow-hidden dark:text-white/62">
											{description.trim()}
										</p>
									)}

									<nav
										className="grid grid-cols-3 mt-2.5 border-b border-[rgba(0,0,0,0.08)] dark:border-b-[rgba(245,222,165,0.14)]"
										aria-label="商家分頁"
									>
										<TabButton
											active={activeTab === "posts"}
											onClick={() => setActiveTab("posts")}
										>
											貼文
										</TabButton>
										<TabButton
											active={activeTab === "shop"}
											onClick={() => setActiveTab("shop")}
										>
											商城
										</TabButton>
										<TabButton
											active={activeTab === "chat"}
											onClick={() => setActiveTab("chat")}
										>
											留言
										</TabButton>
									</nav>

									{activeTab === "shop" && (
										<StorefrontPreviewPanel type="shop" />
									)}
									{activeTab === "posts" && (
										<StorefrontPreviewPanel type="posts" />
									)}
									{activeTab === "chat" && (
										<StorefrontPreviewPanel type="chat" />
									)}
								</div>
							</div>
						</div>
						{canWrite && (
							<>
								<EditCallout
									target="cover"
									lineVariant="curve"
									posClassName="seller-profile-branding__mobile-callout seller-profile-branding__mobile-callout--cover absolute w-max max-w-[142px] flex-col items-start @max-[520px]:max-w-[132px]"
									onClick={() => coverInputRef.current?.click()}
									busy={uploadingCover}
									icon="image"
									title="點此換背景"
									loadingTitle="背景上傳中…"
									hint="可調整填滿方式與背景色"
								/>
								<EditCallout
									target="avatar"
									flipLine
									lineVariant="curve"
									posClassName="seller-profile-branding__mobile-callout seller-profile-branding__mobile-callout--avatar absolute w-max max-w-[142px] flex-col items-start @max-[520px]:max-w-[132px]"
									onClick={() => avatarInputRef.current?.click()}
									busy={uploadingLogo}
									icon="user"
									title="點此換大頭貼"
									loadingTitle="大頭貼上傳中…"
									hint="正方形圖片效果最佳"
								/>
							</>
						)}
					</>
				)}

				{/* ── Web preview ───────────────────────────────────── */}
				{viewMode === "web" && (
					<div className="relative w-full max-w-[420px]">
						<WebPreview
							coverPreviewUrl={coverPreviewUrl}
							logoPreviewUrl={logoPreviewUrl}
							storeName={storeName}
							accountId={accountId}
							storeCode={storeCode}
							description={description}
							locationLine={locationLine}
							activeTab={activeTab}
							onTabChange={setActiveTab}
							uploadingCover={uploadingCover}
							uploadingLogo={uploadingLogo}
						/>

						{canWrite && (
							<>
								<EditCallout
									target="cover"
									posClassName="seller-profile-branding__web-callout seller-profile-branding__web-callout--cover absolute w-max max-w-[152px] flex-row @max-[520px]:max-w-[138px]"
									onClick={() => coverInputRef.current?.click()}
									busy={uploadingCover}
									icon="image"
									title="點此換背景"
									loadingTitle="背景上傳中…"
									hint="可調整填滿方式與背景色"
								/>
								<EditCallout
									target="avatar"
									flipLine
									posClassName="seller-profile-branding__web-callout seller-profile-branding__web-callout--avatar absolute w-max max-w-[152px] flex-row-reverse @max-[520px]:max-w-[138px]"
									onClick={() => avatarInputRef.current?.click()}
									busy={uploadingLogo}
									icon="user"
									title="點此換大頭貼"
									loadingTitle="大頭貼上傳中…"
									hint="正方形圖片效果最佳"
								/>
							</>
						)}
					</div>
				)}
			</div>
		</aside>
	);
}

/* ============================================================
   Edit callout — a pill with a dashed line that points into the
   preview's cover / avatar region. Shared by both the phone and
   the web previews; only the positioning classes differ.
   ============================================================ */

function EditCallout({
	target,
	posClassName,
	flipLine = false,
	lineVariant = "straight",
	onClick,
	busy,
	icon,
	title,
	loadingTitle,
	hint,
}: {
	target: "cover" | "avatar";
	posClassName: string;
	flipLine?: boolean;
	lineVariant?: "straight" | "curve";
	onClick: () => void;
	busy: boolean;
	icon: AppIconName;
	title: string;
	loadingTitle: string;
	hint: string;
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			disabled={busy}
			aria-busy={busy}
			className={`group/callout z-4 m-0 flex cursor-pointer items-center gap-0 overflow-visible border-0 bg-transparent p-0 text-hkui-primary opacity-55 transition-opacity duration-160 hover:opacity-100 focus-visible:opacity-100 disabled:cursor-progress disabled:opacity-70 dark:text-[#f1cd70] ${posClassName}`}
		>
			{lineVariant === "curve" ? (
				<svg
					className={`seller-profile-branding__mobile-line seller-profile-branding__mobile-line--${target} pointer-events-none absolute z-0 text-current opacity-95 drop-shadow-[0_2px_3px_rgb(var(--color-ink-950-rgb)/0.14)]`}
					viewBox="0 0 120 70"
					aria-hidden="true"
					fill="none"
				>
					<path
						d={
							target === "cover"
								? "M110 12 C 84 18, 54 32, 14 48"
								: "M8 58 C 42 40, 76 22, 112 14"
						}
						stroke="currentColor"
						strokeWidth="2"
						strokeDasharray="2 6"
						strokeLinecap="round"
					/>
					<circle
						cx={target === "cover" ? 14 : 112}
						cy={target === "cover" ? 48 : 14}
						r="4"
						fill="currentColor"
					/>
				</svg>
			) : (
				<span
					className={`pointer-events-none relative h-12 w-12 flex-shrink-0 text-current opacity-95 drop-shadow-[0_2px_3px_rgb(var(--color-ink-950-rgb)/0.14)] @max-[520px]:w-9 ${
						target === "cover" ? "mt-1" : ""
					}`}
					aria-hidden="true"
				>
					<span
						className={`absolute top-1/2 h-[2px] w-full -translate-y-1/2 rounded-full bg-[repeating-linear-gradient(90deg,currentColor_0_3px,transparent_3px_8px)] ${
							target === "cover" ? "-rotate-12" : "rotate-6"
						}`}
					/>
					<span
						className={`absolute top-1/2 size-2 -translate-y-1/2 rounded-full bg-current shadow-[0_0_0_3px_rgba(255,255,255,0.72),0_2px_6px_rgb(var(--color-ink-950-rgb)/0.22)] dark:shadow-[0_0_0_3px_rgb(var(--color-ink-900-rgb)/0.88),0_2px_8px_rgba(0,0,0,0.42)] ${
							flipLine ? "right-0" : "left-0"
						}`}
					/>
				</span>
			)}
			<span className="relative z-1 inline-flex min-w-0 flex-none items-center gap-1.5 rounded-full border border-hkui-primary/35 bg-content1/95 p-1.5 px-2.5 shadow-[0_12px_28px_-18px_rgb(var(--color-ink-950-rgb)/0.48),inset_0_1px_0_rgba(255,255,255,0.72)] backdrop-blur transition-[border-color,box-shadow,transform] duration-160 group-hover/callout:-translate-y-0.5 group-hover/callout:border-hkui-primary group-hover/callout:shadow-[0_18px_34px_-20px_rgb(var(--color-ink-950-rgb)/0.58),inset_0_1px_0_rgba(255,255,255,0.8)] dark:border-[#f1cd70]/34 dark:bg-hk-ink-900/92">
				<span
					className="inline-flex size-7 flex-shrink-0 items-center justify-center rounded-full bg-hkui-primary/12 ring-1 ring-inset ring-hkui-primary/20 shadow-[inset_0_1px_0_rgba(255,255,255,0.5)] dark:bg-[#f1cd70]/14 dark:ring-[#f1cd70]/24"
					aria-hidden="true"
				>
					{busy ? (
						<UploadLoadingIndicator
							size={19}
							className="size-[19px] shrink-0"
						/>
					) : (
						<AppIcon name={icon} size={19} />
					)}
				</span>
				<span className="flex min-w-0 flex-none flex-col gap-[1px] text-left">
					<strong className="grid overflow-hidden text-ellipsis whitespace-nowrap text-[0.6875rem] font-bold leading-tight text-foreground dark:text-[#f5e9c3]">
						<span
							className={`col-start-1 row-start-1 ${busy ? "opacity-0" : ""}`}
							aria-hidden={busy || undefined}
						>
							{title}
						</span>
						<span
							className={`col-start-1 row-start-1 ${busy ? "" : "opacity-0"}`}
							role={busy ? "status" : undefined}
							aria-live={busy ? "polite" : undefined}
							aria-atomic={busy ? "true" : undefined}
							aria-hidden={busy ? undefined : true}
						>
							{loadingTitle}
						</span>
					</strong>
					<small className="overflow-hidden text-ellipsis whitespace-nowrap text-[0.5625rem] leading-tight text-foreground-muted dark:text-white/60">
						{hint}
					</small>
				</span>
			</span>
		</button>
	);
}

/* ============================================================
   Phone-frame internals
   ============================================================ */

function StatusBar() {
	return (
		<div
			className="absolute top-0 left-0 right-0 flex items-center justify-between p-[8px_18px_6px] text-[#1a1a1a] text-xs font-semibold z-5 dark:text-[#f5e9c3]"
			aria-hidden="true"
		>
			<span className="tracking-tight">9:41</span>
			<span className="absolute left-1/2 top-[5px] -translate-x-1/2 w-[78px] h-[22px] rounded-[14px] bg-neutral-ink dark:bg-[#050807]" />
			<span className="inline-flex items-center gap-1.5 text-current">
				<SignalIcon />
				<WifiIcon />
				<BatteryIcon />
			</span>
		</div>
	);
}

function TopBar() {
	// Mirrors hawkr_buyer `_TopBarIconButton` (merchant_header.dart): a
	// 32px circle filled rgba(255,255,255,0.7) with #565656 glyphs, the back /
	// chat icons taken from the app's own ic_back / ic_chat SVGs and the cart
	// from Material `shopping_cart_outlined`. Gap between cart & chat is 8px.
	return (
		<div
			className="absolute top-[38px] left-0 right-0 flex items-center gap-2 p-[6px_12px] z-3"
			aria-hidden="true"
		>
			<span className="w-7 h-7 rounded-full border-0 bg-white/70 text-[#565656] inline-flex items-center justify-center backdrop-blur-[8px] shadow-[0_1px_4px_rgba(0,0,0,0.1)] dark:bg-hk-ink-900/74 dark:text-[#f5e9c3] dark:ring-1 dark:ring-[rgba(245,222,165,0.14)]">
				<MerchantBackIcon />
			</span>
			<span className="flex-1" />
			<span className="w-7 h-7 rounded-full border-0 bg-white/70 text-[#565656] inline-flex items-center justify-center backdrop-blur-[8px] shadow-[0_1px_4px_rgba(0,0,0,0.1)] dark:bg-hk-ink-900/74 dark:text-[#f5e9c3] dark:ring-1 dark:ring-[rgba(245,222,165,0.14)]">
				<MerchantCartIcon />
			</span>
			<span className="w-7 h-7 rounded-full border-0 bg-white/70 text-[#565656] inline-flex items-center justify-center backdrop-blur-[8px] shadow-[0_1px_4px_rgba(0,0,0,0.1)] dark:bg-hk-ink-900/74 dark:text-[#f5e9c3] dark:ring-1 dark:ring-[rgba(245,222,165,0.14)]">
				<MerchantChatIcon />
			</span>
		</div>
	);
}

function TabButton({
	active,
	onClick,
	children,
}: {
	active: boolean;
	onClick: () => void;
	children: React.ReactNode;
}) {
	return (
		<button
			type="button"
			className={`bg-transparent border-0 p-[8px_0_9px] font-inherit text-xs font-semibold text-[#888] cursor-pointer relative dark:text-white/54 ${
				active
					? "text-hk-primary-strong after:content-[''] after:absolute after:left-1/4 after:right-1/4 after:bottom-[-1px] after:h-[2px] after:rounded-[2px] after:bg-hk-primary-strong dark:text-[#f1cd70] dark:after:bg-[#f1cd70]"
					: ""
			}`}
			onClick={onClick}
		>
			<span>{children}</span>
		</button>
	);
}

/**
 * 分頁區塊的說明文案。
 *
 * ⚠️ **這四段話不得是對店家現況的宣稱。** 這個預覽是買家商店頁的**版面**示意，
 * 四個分頁一律不發任何查詢 —— 沒有商品、貼文、留言、評價的任何資料來源。原本
 * 寫的是「尚未上架商品」「尚未發布貼文」，那對一個 8 件商品全部上架中的店家是
 * 假的，而畫面上沒有任何東西會告訴你它是假的（拿不到權威資料時正解是不做宣稱，
 * 與「互動數不渲染 0」同一條裁決）。
 *
 * 同理不放行動按鈕：它不是 `<button>`、點不動，卻長得像入口。
 */
const PREVIEW_PANEL_COPY: Record<
	SellerPreviewTab,
	{
		title: string;
		body: string;
		icon: AppIconName;
	}
> = {
	shop: {
		title: "商城版面",
		body: "買家在商店頁會看到你的商品卡片。這裡只示意排版，不載入實際商品。",
		icon: "box",
	},
	posts: {
		title: "動態版面",
		body: "已發布的動態會在商店頁形成動態牆。這裡只示意排版，不載入實際貼文。",
		icon: "document",
	},
	chat: {
		title: "留言版面",
		body: "買家的提問會集中在這一區，方便後續回覆與追蹤。",
		icon: "chat",
	},
	reviews: {
		title: "評價版面",
		body: "完成訂單後，買家的評價與回饋會顯示在這一區。",
		icon: "star",
	},
};

const PREVIEW_SKELETON_CARD_IDS = [
	"empty-card-primary",
	"empty-card-secondary",
	"empty-card-tertiary",
	"empty-card-quaternary",
	"empty-card-quinary",
	"empty-card-senary",
];

function StorefrontPreviewPanel({
	type,
	isWeb = false,
}: {
	type: SellerPreviewTab;
	isWeb?: boolean;
}) {
	const copy = PREVIEW_PANEL_COPY[type];

	return (
		<section
			className={
				isWeb
					? "flex-1 min-h-0 overflow-y-auto scrollbar-gutter-stable bg-[#fbfaf6] p-[16px_22px_18px] dark:bg-hk-ink-900/96"
					: "flex-1 min-h-0 overflow-hidden bg-[#fbfaf6] p-3 dark:bg-hk-ink-900"
			}
			aria-label={`${copy.title}預覽`}
		>
			<div
				className={`grid items-center gap-3 border border-[#e2dfd4] bg-white/72 shadow-[0_10px_24px_rgb(var(--color-ink-850-rgb)/0.06)] ${
					isWeb
						? "grid-cols-[auto_minmax(0,1fr)] rounded-xl p-[14px_16px] dark:border-[rgba(245,222,165,0.16)] dark:bg-white/5 dark:shadow-none"
						: "grid-cols-[auto_minmax(0,1fr)] rounded-lg p-3 dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5 dark:shadow-none"
				}`}
			>
				<span
					className={`grid place-items-center rounded-full bg-hk-ink-50 text-hk-primary-strong ${
						isWeb
							? "h-11 w-11 dark:bg-hk-ink-700 dark:text-[#f1cd70]"
							: "h-9 w-9 dark:bg-hk-ink-700 dark:text-[#f1cd70]"
					}`}
					aria-hidden="true"
				>
					<AppIcon name={copy.icon} size={isWeb ? 24 : 20} />
				</span>
				<div className="min-w-0">
					<strong
						className={`block font-bold text-[#1a1a1a] ${
							isWeb
								? "text-[0.875rem] dark:text-[#f5e9c3]"
								: "text-[0.75rem] dark:text-[#f5e9c3]"
						}`}
					>
						{copy.title}
					</strong>
					<small
						className={`mt-1 block leading-[1.45] text-[#6c6c66] ${
							isWeb
								? "text-[0.75rem] dark:text-white/62"
								: "text-[0.625rem] dark:text-white/62"
						}`}
					>
						{copy.body}
					</small>
				</div>
			</div>

			<PreviewSkeletonSurface type={type} isWeb={isWeb} />
		</section>
	);
}

function PreviewSkeletonSurface({
	type,
	isWeb,
}: {
	type: SellerPreviewTab;
	isWeb: boolean;
}) {
	if (type === "chat") {
		return (
			<div
				className={`mt-3 space-y-2 ${isWeb ? "mx-auto max-w-[420px]" : ""}`}
				aria-hidden="true"
			>
				{[0, 1, 2].map((item) => (
					<div
						key={item}
						className={`flex items-start gap-2 rounded-lg border border-[#ece8dd] bg-white/58 p-2.5 dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5 ${
							item === 1 ? "ml-6" : "mr-6"
						}`}
					>
						<span className="h-6 w-6 flex-none rounded-full bg-hk-ink-100 dark:bg-white/10" />
						<span className="min-w-0 flex-1 space-y-1.5">
							<span className="block h-2 w-1/3 rounded-full bg-[#d9d7ce] dark:bg-white/12" />
							<span className="block h-2 w-full rounded-full bg-[#eeeae0] dark:bg-white/8" />
							<span className="block h-2 w-2/3 rounded-full bg-[#eeeae0] dark:bg-white/8" />
						</span>
					</div>
				))}
			</div>
		);
	}

	if (type === "reviews") {
		return (
			<div
				className="mt-3 grid gap-2 rounded-xl border border-[#ece8dd] bg-white/48 p-3 dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5"
				aria-hidden="true"
			>
				{[0, 1, 2, 3, 4].map((item) => (
					<div
						key={item}
						className="grid grid-cols-[40px_minmax(0,1fr)] items-center gap-2"
					>
						<span className="h-2 rounded-full bg-[#c9c7bd] dark:bg-white/16" />
						<span
							className="h-2 rounded-full bg-[#e8e3d7] dark:bg-white/10"
							style={{ width: `${92 - item * 13}%` }}
						/>
					</div>
				))}
			</div>
		);
	}

	return (
		<div
			className={
				isWeb ? "mt-3 grid grid-cols-3 gap-2" : "mt-3 grid grid-cols-2 gap-2"
			}
			aria-hidden="true"
		>
			{PREVIEW_SKELETON_CARD_IDS.slice(0, isWeb ? 6 : 4).map((cardId) => (
				<div
					key={cardId}
					className="min-w-0 rounded-lg border border-[#ece8dd] bg-white/58 p-2 dark:border-[rgba(245,222,165,0.12)] dark:bg-white/5"
				>
					<span
						className={`block rounded-md bg-hk-ink-100 dark:bg-white/8 ${
							type === "posts" ? "aspect-[16/9]" : "aspect-[4/3]"
						}`}
					/>
					<span className="mt-2 block h-2 w-4/5 rounded-full bg-[#d8d6cc] dark:bg-white/14" />
					<span className="mt-1.5 block h-2 w-1/2 rounded-full bg-[#ece8dd] dark:bg-white/9" />
				</div>
			))}
		</div>
	);
}

/* ============================================================
   Web preview — browser-chrome shell mimicking buyer-side
   `/sellers/[id]`. Fixed 4:1 cover, avatar overlapping bottom-left,
   meta + bio inline to the right, tabs full-width.
   ============================================================ */

interface WebPreviewProps {
	coverPreviewUrl: string;
	logoPreviewUrl: string;
	storeName?: string | null;
	accountId?: string | number | null;
	storeCode?: string | null;
	description?: string | null;
	locationLine: string;
	activeTab: SellerPreviewTab;
	onTabChange: (tab: SellerPreviewTab) => void;
	uploadingCover: boolean;
	uploadingLogo: boolean;
}

function WebPreview({
	coverPreviewUrl,
	logoPreviewUrl,
	storeName,
	accountId,
	storeCode,
	description,
	locationLine,
	activeTab,
	onTabChange,
	uploadingCover,
	uploadingLogo,
}: WebPreviewProps) {
	return (
		<div
			className="flex flex-col w-full h-[540px] bg-content1 border border-divider rounded-[14px] shadow-[0_8px_24px_rgb(var(--color-ink-950-rgb)/0.08),0_2px_6px_rgb(var(--color-ink-950-rgb)/0.04)] overflow-hidden animate-seller-web-in dark:bg-hk-ink-900/92 dark:border-[rgba(245,222,165,0.18)] dark:shadow-[0_28px_70px_rgba(0,0,0,0.28),inset_0_1px_0_rgba(245,222,165,0.1)]"
			aria-hidden="true"
		>
			{/* Browser chrome — traffic-light buttons + URL pill. */}
			<div className="flex items-center gap-1.5 flex-none p-2 px-3 bg-content2 border-b border-divider dark:bg-hk-ink-850/94 dark:border-b-[rgba(245,222,165,0.18)]">
				<span className="w-2 h-2 rounded-full inline-block bg-[#ff5f56]" />
				<span className="w-2 h-2 rounded-full inline-block bg-[#ffbd2e]" />
				<span className="w-2 h-2 rounded-full inline-block bg-hk-success" />
				<span className="ml-2 p-[2px_10px] bg-content1 border border-divider rounded-full text-[0.6875rem] text-foreground-muted font-mono overflow-hidden text-ellipsis whitespace-nowrap max-w-[240px] dark:bg-white/5 dark:border-[rgba(245,222,165,0.18)] dark:text-foreground-soft/78">
					example.com/sellers/{storeCode ?? accountId ?? "—"}
				</span>
			</div>

			<div className="bg-content1 flex flex-1 flex-col min-h-0 dark:bg-hk-ink-900/96">
				<div
					className={`relative flex-none aspect-[4/1] m-[10px_10px_0] bg-default-100 border border-[color:var(--color-neutral-panel)] rounded-[10px] shadow-[0_18px_44px_rgb(var(--color-ink-850-rgb)/0.08)] overflow-hidden after:content-[''] after:absolute after:inset-0 after:z-[2] after:pointer-events-none after:bg-[linear-gradient(180deg,rgb(var(--color-ink-950-rgb)/0.08),transparent_42%),linear-gradient(0deg,rgba(255,253,247,0.16),transparent_44%)] ${
						coverPreviewUrl ? "bg-transparent" : ""
					}`}
				>
					{coverPreviewUrl ? (
						<SellerCoverMedia
							src={coverPreviewUrl}
							sizes="400px"
							quality={60}
						/>
					) : (
						<CoverPattern />
					)}
					<span className="absolute z-2 top-2.5 left-2.5 inline-flex items-center gap-1 p-[5px_9px_5px_7px] rounded-full bg-[#fffdf7]/85 backdrop-blur-[10px] text-[#1a1a1a] text-[0.625rem] font-semibold whitespace-nowrap [&>svg]:size-3.5 dark:bg-[rgb(var(--color-ink-900-rgb)/0.7)] dark:text-[#f5e9c3] dark:border dark:border-[rgba(245,222,165,0.24)]">
						<ChevronIcon />
						<span>返回</span>
					</span>
					{uploadingCover && (
						<span className="absolute inset-0 z-[3] flex items-center justify-center bg-black/50 text-white">
							<UploadLoadingIndicator size={28} className="size-7 shrink-0" />
						</span>
					)}
				</div>

				{/* Match the avatar overlap used by the responsive public page. */}
				<div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-start flex-none h-[126px] gap-4 p-[0_32px_14px] -mt-5">
					<div className="relative w-[78px] h-[78px] rounded-full bg-content1 text-foreground-muted border-2 border-[color:var(--color-neutral-panel)] shadow-[inset_0_1px_0_rgba(255,255,255,0.84),0_14px_30px_rgb(var(--color-ink-850-rgb)/0.14)] overflow-hidden grid place-items-center flex-shrink-0 dark:bg-hk-ink-850/95 dark:border-[rgba(245,222,165,0.28)] dark:shadow-[0_14px_30px_rgba(0,0,0,0.28)]">
						{logoPreviewUrl ? (
							// biome-ignore lint/performance/noImgElement: blob/CDN preview
							<img
								src={logoPreviewUrl}
								alt=""
								className="w-full h-full object-cover rounded-full"
							/>
						) : null}
						{uploadingLogo && (
							<span className="absolute inset-0 flex items-center justify-center bg-black/40 text-white rounded-full">
								<UploadLoadingIndicator
									size={22}
									className="size-[22px] shrink-0"
								/>
							</span>
						)}
					</div>
					<div className="flex min-w-0 flex-1 flex-col gap-[2px] pt-[50px]">
						<strong className="overflow-hidden text-ellipsis whitespace-nowrap text-base font-bold leading-tight text-foreground dark:text-[#f5e9c3]">
							{storeName?.trim() || "店舖名稱"}
						</strong>
						<small className="overflow-hidden text-ellipsis whitespace-nowrap text-xs text-foreground-muted dark:text-white/60">
							ID : {storeCode ?? accountId ?? "—"} · {locationLine}
						</small>
						{description?.trim() && (
							<p className="mt-1 text-xs leading-[1.5] text-foreground-soft overflow-hidden text-ellipsis line-clamp-2 dark:text-white/62">
								{description.trim()}
							</p>
						)}
					</div>
					<div className="flex flex-shrink-0 items-start gap-1.5 pt-[50px]">
						<button
							type="button"
							className="inline-flex items-center justify-center gap-1 h-8 p-[5px_14px] border border-divider bg-transparent text-foreground rounded-full text-xs font-semibold whitespace-nowrap shrink-0 cursor-pointer dark:bg-white/6 dark:text-[#f5e9c3] dark:border-[rgba(245,222,165,0.28)]"
						>
							訊息
						</button>
						<button
							type="button"
							className="inline-flex items-center justify-center gap-1 h-8 p-[5px_14px] border border-transparent bg-hkui-primary text-hkui-primary-foreground rounded-full text-xs font-semibold whitespace-nowrap shrink-0 cursor-pointer [&>svg]:size-3.5 dark:bg-gradient-to-r dark:from-hk-primary-strong dark:to-hk-primary-strong dark:text-hk-cream dark:shadow-[0_8px_20px_rgb(var(--color-brand-primary-strong-rgb)/0.34)]"
						>
							<PlusMiniIcon />
							訂閱
						</button>
					</div>
				</div>

				<nav
					className="flex flex-none h-[46px] border-b border-divider p-[0_32px] gap-5 dark:border-b-[rgba(245,222,165,0.16)]"
					aria-label="商家分頁"
				>
					<WebTabButton
						active={activeTab === "shop"}
						onClick={() => onTabChange("shop")}
					>
						商品
					</WebTabButton>
					<WebTabButton
						active={activeTab === "posts"}
						onClick={() => onTabChange("posts")}
					>
						貼文
					</WebTabButton>
					<WebTabButton
						active={activeTab === "chat"}
						onClick={() => onTabChange("chat")}
					>
						留言
					</WebTabButton>
					<WebTabButton
						active={activeTab === "reviews"}
						onClick={() => onTabChange("reviews")}
					>
						評價
					</WebTabButton>
				</nav>

				{activeTab === "shop" && <StorefrontPreviewPanel type="shop" isWeb />}
				{activeTab === "posts" && <StorefrontPreviewPanel type="posts" isWeb />}
				{activeTab === "chat" && <StorefrontPreviewPanel type="chat" isWeb />}
				{activeTab === "reviews" && (
					<StorefrontPreviewPanel type="reviews" isWeb />
				)}
			</div>
		</div>
	);
}

function WebTabButton({
	active,
	onClick,
	children,
}: {
	active: boolean;
	onClick: () => void;
	children: React.ReactNode;
}) {
	return (
		<button
			type="button"
			className={`relative p-[10px_0] bg-transparent border-0 text-foreground-muted text-[0.8125rem] font-semibold cursor-pointer dark:text-white/62 transition-colors ${
				active
					? "text-foreground after:content-[''] after:absolute after:left-0 after:right-0 after:bottom-[-1px] after:h-[2px] after:bg-hkui-primary dark:text-[#ffeec0] dark:after:bg-[#f1cd70] dark:after:shadow-[0_0_8px_rgba(241,205,112,0.45)]"
					: ""
			}`}
			onClick={onClick}
		>
			{children}
		</button>
	);
}

function DeviceMobileIcon() {
	return <AppIcon name="scan" className="w-3.5 h-3.5 shrink-0" />;
}

function DeviceWebIcon() {
	return <AppIcon name="category" className="w-3.5 h-3.5 shrink-0" />;
}

function PlusMiniIcon() {
	return <AppIcon name="plus" />;
}

/* ============================================================
   Glyphs / icons
   ============================================================ */

function CoverPattern() {
	return (
		<svg
			className="absolute inset-0 w-full h-full text-hk-primary-strong opacity-50 pointer-events-none"
			viewBox="0 0 200 60"
			preserveAspectRatio="none"
			aria-hidden="true"
		>
			<defs>
				<pattern
					id="seller-branding-dots"
					width="6"
					height="6"
					patternUnits="userSpaceOnUse"
				>
					<circle cx="1" cy="1" r="0.8" fill="currentColor" opacity="0.35" />
				</pattern>
			</defs>
			<rect width="200" height="60" fill="url(#seller-branding-dots)" />
		</svg>
	);
}

function ChevronIcon() {
	return <AppIcon name="chevron-left" />;
}

// Phone top-bar glyphs copied from the buyer app's merchant page so the
// preview matches what shoppers actually see.

/** Buyer app `HkIconName.back` (`AppIcons.arrowBack`, Material arrow_back). */
function MerchantBackIcon() {
	return <AppIcon name="arrow-left" className="h-[14px] w-auto" />;
}

/** Material `Icons.shopping_cart_outlined` (size 18 in app). */
function MerchantCartIcon() {
	return <AppIcon name="buy" className="size-[17px]" />;
}

/** Buyer app `HkIconName.chat` (`AppIcons.chatBubbleOutline`, Material chat_bubble_outline). */
function MerchantChatIcon() {
	return <AppIcon name="chat" className="size-[15px]" />;
}

function SignalIcon() {
	return <AppIcon name="activity" size={14} />;
}

function WifiIcon() {
	return <AppIcon name="discovery" size={14} />;
}

function BatteryIcon() {
	return <AppIcon name="wallet" size={14} />;
}

export default SellerProfileBranding;
