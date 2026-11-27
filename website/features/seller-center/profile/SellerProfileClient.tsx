"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { submitError } from "@hawkr/api/connect/submit";
import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import { Input, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { TownFields } from "@/features/account/screens/addresses/TownFields";
import { targetStoreLabel } from "@/features/delegated-operations/confirm-content";
import { SellerProfileBranding } from "@/features/seller/components/SellerProfileBranding";
import { revalidateOwnSellerProfile } from "@/features/seller-center/profile/actions";
import CloseStoreSection from "@/features/seller-center/profile/CloseStoreSection";
import SellerShareCardPreview from "@/features/seller-center/profile/SellerShareCardPreview";
import { toast } from "@/lib/feedback/toast";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import {
	useInvalidateSellerSelf,
	useSellerSelf,
} from "@/lib/hooks/useSellerSelf";
import { useVersionConflict } from "@/lib/hooks/useVersionConflict";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { fetchProfile } from "@/lib/profile/repo";
import { queryKeys } from "@/lib/query/keys";
import { hasCapability, storeImageUrls } from "@/lib/seller/store";
import {
	hasStoreContactErrors,
	type StoreContactErrors,
	storeContactServerErrors,
} from "@/lib/seller/store-contact";
import {
	STORE_CHANGE_LABELS,
	type StoreChangeField,
	type StoreForm,
	storeFormAddressError,
	storeFormChanges,
	storeFormContactErrors,
	storeFormDisplay,
	storeFormOf,
	townLocation,
} from "@/lib/seller/store-form";
import {
	CONTACT_NAME_MAX_LENGTH,
	SUPPORT_EMAIL_MAX_LENGTH,
} from "@/lib/seller/validators";
import {
	isValidWebsiteUrl,
	normalizeWebsiteUrl,
} from "@/lib/seller/website-url";
import { useSellerDataScope } from "@/lib/seller-data/scope";

const CHANGE_FIELDS = Object.keys(STORE_CHANGE_LABELS) as StoreChangeField[];

function formatSaveHint(savedAt: number | null, hasUnsavedChanges: boolean) {
	if (hasUnsavedChanges) return "尚有未儲存的商店資料";
	if (!savedAt) return "修改商店資料後即可儲存";
	const seconds = Math.floor((Date.now() - savedAt) / 1000);
	if (seconds < 60) return "上次儲存：剛剛";
	return `上次儲存：${Math.floor(seconds / 60)} 分鐘前`;
}

export default function SellerProfileClient() {
	const defaults = useDefaultErrorText();
	const {
		data: serverSelf,
		loading: selfLoading,
		error: selfError,
	} = useSellerSelf();
	const invalidateSelf = useInvalidateSellerSelf();
	const scope = useSellerDataScope();
	const delegated = scope.mode === "delegated";
	/**
	 * 這個工作階段能不能寫店面（§5.3：有 read、沒有 write ⇒ 頁面呈唯讀，不出現
	 * write CTA 或 handler）。
	 *
	 * 賣家路徑看 `GetMyStore.capabilities` 的 STOREFRONT_WRITE（停權時沒有；`scope.hasCapability`
	 * 在賣家路徑恆回 true，不能用）。缺 `storefront.write` 時整份表單
	 * 只能看：欄位 `readOnly`、兩顆換圖入口與儲存鍵**不渲染**（不是 disabled ——
	 * disabled 的儲存鍵與「尚未修改」在畫面上完全一樣，操作者分不出是沒權限還是
	 * 還沒改東西）。
	 */
	const canWrite = delegated
		? scope.hasCapability("storefront.write")
		: hasCapability(serverSelf, SellerCapability.STOREFRONT_WRITE);
	const identity = useQueryIdentity();
	const serverStore = serverSelf?.store;
	const baseline = React.useMemo(() => storeFormOf(serverStore), [serverStore]);
	const [edits, setEdits] = React.useState<Partial<StoreForm>>({});
	const data: StoreForm = React.useMemo(
		() => ({ ...baseline, ...edits }),
		[baseline, edits],
	);
	const [etag, setEtag] = React.useState<string | null>(null);
	const serverEtag = serverStore?.etag;
	React.useEffect(() => {
		if (serverEtag && etag === null) setEtag(serverEtag);
	}, [serverEtag, etag]);
	const adoptLatestStore = useVersionConflict({
		reload: async () => {
			try {
				return await scope.repos.storefront.fetchMyStore();
			} finally {
				await invalidateSelf();
			}
		},
		adopt: (latest) => {
			if (latest.store?.etag) setEtag(latest.store.etag);
			return undefined;
		},
		notify: (message) =>
			toast.error("商店資料已在其他地方更新", { description: message }),
		notice: "已載入最新版本，你輸入的內容仍保留，請確認後再儲存。",
	});
	const [saving, setSaving] = React.useState(false);
	const [uploading, setUploading] = React.useState(false);
	const [savedAt, setSavedAt] = React.useState<number | null>(null);
	const [websiteError, setWebsiteError] = React.useState<string | null>(null);
	const [addressError, setAddressError] = React.useState<string | null>(null);
	const [serverContactErrors, setServerContactErrors] = React.useState<{
		edits: Partial<StoreForm>;
		errors: StoreContactErrors;
	} | null>(null);

	const accountQuery = useQuery({
		queryKey: queryKeys.buyer.profile(identity),
		queryFn: () => fetchProfile({ suppressAuthExpired: true }),
		enabled: !delegated,
	});
	const account = accountQuery.data ?? null;
	const loading = accountQuery.isLoading;

	const [, tick] = React.useReducer((n: number) => n + 1, 0);
	React.useEffect(() => {
		if (!savedAt) return;
		const id = setInterval(tick, 30_000);
		return () => clearInterval(id);
	}, [savedAt]);

	const update = (patch: Partial<StoreForm>) =>
		setEdits((prev) => ({ ...prev, ...patch }));
	const pendingProfileChanges = React.useMemo(
		() => storeFormChanges(data, baseline),
		[data, baseline],
	);
	const localContactErrors = storeFormContactErrors(data);
	const contactErrors: StoreContactErrors =
		serverContactErrors && serverContactErrors.edits === edits
			? {
					nameErr:
						localContactErrors.nameErr ?? serverContactErrors.errors.nameErr,
					mobileErr:
						localContactErrors.mobileErr ??
						serverContactErrors.errors.mobileErr,
					emailErr:
						localContactErrors.emailErr ?? serverContactErrors.errors.emailErr,
				}
			: localContactErrors;
	const hasPendingProfileChanges =
		Object.keys(pendingProfileChanges).length > 0;

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!canWrite) return;
		if (!serverStore || etag === null || !hasPendingProfileChanges) return;
		const websiteUrl = normalizeWebsiteUrl(data.websiteUrl);
		if (!isValidWebsiteUrl(websiteUrl)) {
			setWebsiteError("請輸入有效的網站網址，例如 https://example.com");
			return;
		}
		setWebsiteError(null);
		if (hasStoreContactErrors(localContactErrors)) return;
		const addressMessage = storeFormAddressError(data);
		if (addressMessage) {
			setAddressError(addressMessage);
			return;
		}
		setAddressError(null);
		const submitted: StoreForm = { ...data, websiteUrl };
		if (data.websiteUrl !== websiteUrl) update({ websiteUrl });
		const changes = storeFormChanges(submitted, baseline);
		const decision = await scope.confirmMutation({
			title: "檢視變更並儲存",
			intent: `以 ADM 身分替「${targetStoreLabel(serverStore.storeName, scope.ownerStoreId)}」儲存商店資訊`,
			hint: "請確認以下變更內容無誤後儲存。",
			reasonPlaceholder: "例：依賣家來信更新營業資訊",
			changes: CHANGE_FIELDS.filter((field) => field in changes).map(
				(field) => ({
					field: STORE_CHANGE_LABELS[field],
					before: storeFormDisplay(baseline, field),
					after: storeFormDisplay(submitted, field),
				}),
			),
			expectedVersion: etag,
			confirmLabel: "確認儲存",
		});
		if (!decision.ok) return;
		setSaving(true);
		try {
			const written = await scope.repos.storefront.updateMyStore(
				{ etag, ...changes },
				decision.meta,
			);
			if (written.etag) setEtag(written.etag);
			setEdits({});
			setServerContactErrors(null);
			setSavedAt(Date.now());
			await invalidateSelf();
			if (!delegated) {
				await revalidateOwnSellerProfile({
					accountId: serverStore.id,
					storeCode: serverStore.storeCode,
				});
			}
			toast.success("商店資訊已儲存", {
				description: "買家看到的商店頁資料已更新。",
			});
		} catch (e2) {
			const failure = delegated ? null : submitError(e2, defaults);
			if (failure?.kind === "conflict") {
				await adoptLatestStore();
			} else {
				if (failure?.kind === "fields") {
					setServerContactErrors({
						edits,
						errors: storeContactServerErrors(failure.fields),
					});
				}
				const message =
					failure?.kind === "fields"
						? (failure.message ?? Object.values(failure.fields)[0])
						: failure?.kind === "message"
							? failure.message
							: e2 instanceof Error
								? e2.message
								: errorText(e2, defaults);
				toast.error("商店資訊儲存失敗", { description: message });
			}
		} finally {
			setSaving(false);
		}
	};

	if (loading || selfLoading) {
		return (
			<div className="seller-profile">
				<div className="seller-card seller-profile__loading">載入中…</div>
			</div>
		);
	}

	if (!serverStore) {
		return (
			<div className="seller-profile">
				<div className="seller-card seller-profile__loading">
					<p className="seller-profile__error" role="alert">
						{selfError?.message || "無法載入商店資料"}
					</p>
				</div>
			</div>
		);
	}

	const images = storeImageUrls(serverStore);
	const location = townLocation(data.townCode);

	return (
		<div className="seller-profile">
			<header className="seller-profile__head">
				<div>
					<h1>商店設定</h1>
					<p>編輯顯示在買家眼前的商店名片、簡介與所在地。</p>
				</div>
			</header>

			<form className="seller-profile__form" onSubmit={handleSubmit}>
				{(account?.email || account?.phone) && (
					<section className="seller-card seller-profile__main">
						<div className="seller-card__head">
							<h2>登入資訊</h2>
						</div>

						<div className="seller-profile__fields">
							<div className="seller-profile__grid">
								{account?.email && (
									<Input
										variant="bordered"
										size="lg"
										label="登入 Email"
										value={account.email}
										readOnly
										description="如需變更請聯絡客服"
									/>
								)}
								{account?.phone && (
									<Input
										variant="bordered"
										size="lg"
										label="登入手機"
										value={account.phone}
										readOnly
										description="如需變更請聯絡客服"
									/>
								)}
							</div>
						</div>
					</section>
				)}

				<section className="seller-card seller-profile__main">
					<div className="seller-card__head">
						<h2>商店顯示資訊</h2>
					</div>

					<div className="seller-profile__layout">
						<div className="seller-profile__fields">
							<div className="seller-profile__grid">
								<Input
									variant="bordered"
									size="lg"
									label="商店名稱"
									value={data.storeName}
									onChange={(e) => update({ storeName: e.target.value })}
									readOnly={!canWrite}
									maxLength={40}
								/>
								<Input
									variant="bordered"
									size="lg"
									label="商店電話"
									value={data.contactPhone}
									onChange={(e) => update({ contactPhone: e.target.value })}
									readOnly={!canWrite}
									maxLength={20}
								/>
								<Input
									variant="bordered"
									size="lg"
									label="聯絡人姓名"
									value={data.contactName}
									onChange={(e) => update({ contactName: e.target.value })}
									errorMessage={contactErrors.nameErr ?? undefined}
									isInvalid={contactErrors.nameErr !== null}
									readOnly={!canWrite}
									maxLength={CONTACT_NAME_MAX_LENGTH}
								/>
								<Input
									variant="bordered"
									size="lg"
									type="tel"
									inputMode="numeric"
									label="聯絡人手機"
									value={data.contactMobile}
									onChange={(e) =>
										update({
											contactMobile: e.target.value
												.replace(/\D/g, "")
												.slice(0, 10),
										})
									}
									description="09 開頭的 10 碼手機號碼"
									errorMessage={contactErrors.mobileErr ?? undefined}
									isInvalid={contactErrors.mobileErr !== null}
									readOnly={!canWrite}
								/>
								<Input
									className="seller-profile__grid-full"
									variant="bordered"
									size="lg"
									type="email"
									label="客服電子信箱"
									value={data.supportEmail}
									onChange={(e) => update({ supportEmail: e.target.value })}
									errorMessage={contactErrors.emailErr ?? undefined}
									isInvalid={contactErrors.emailErr !== null}
									readOnly={!canWrite}
									maxLength={SUPPORT_EMAIL_MAX_LENGTH}
								/>

								<TownFields
									countyCode={data.countyCode}
									townCode={data.townCode}
									disabled={!canWrite}
									townError={addressError ?? undefined}
									onChange={(countyCode, townCode) => {
										setAddressError(null);
										update({ countyCode, townCode });
									}}
								/>
								<Input
									className="seller-profile__grid-full"
									variant="bordered"
									size="lg"
									label="店家地址"
									value={data.addressDetail}
									onChange={(e) => update({ addressDetail: e.target.value })}
									readOnly={!canWrite}
									maxLength={120}
								/>
								<Input
									className="seller-profile__grid-full"
									variant="bordered"
									size="lg"
									type="url"
									inputMode="url"
									label="官方網站"
									value={data.websiteUrl}
									onChange={(e) => {
										update({ websiteUrl: e.target.value });
										if (websiteError) setWebsiteError(null);
									}}
									onBlur={() => {
										const normalized = normalizeWebsiteUrl(data.websiteUrl);
										if (normalized === "" || isValidWebsiteUrl(normalized)) {
											update({ websiteUrl: normalized });
										}
									}}
									placeholder="https://example.com"
									description="可填寫品牌官網、Facebook 粉絲專頁或其他公開網站"
									errorMessage={websiteError ?? undefined}
									isInvalid={Boolean(websiteError)}
									readOnly={!canWrite}
									maxLength={240}
								/>
								<Input
									className="seller-profile__grid-full"
									variant="bordered"
									size="lg"
									label="得獎紀錄 / 店鋪亮點"
									value={data.prize}
									onChange={(e) => update({ prize: e.target.value })}
									readOnly={!canWrite}
									maxLength={120}
								/>
								<Textarea
									className="seller-profile__grid-full"
									variant="bordered"
									size="lg"
									label="商店介紹"
									value={data.description}
									onChange={(e) => update({ description: e.target.value })}
									minRows={5}
									readOnly={!canWrite}
									maxLength={500}
									placeholder="一段話告訴買家你是誰、種了什麼、為什麼買你的"
								/>
							</div>
						</div>

						<div className="seller-profile__preview-row">
							<SellerProfileBranding
								logoUrl={images.logoUrl}
								coverUrl={images.coverUrl}
								storeName={data.storeName}
								savedStoreName={serverStore.storeName}
								accountId={serverStore.id}
								storeCode={serverStore.storeCode}
								description={data.description}
								city={location.county}
								district={location.town}
								storeEtag={etag ?? serverStore.etag}
								readOnly={!canWrite}
								onSaved={async (written) => {
									if (written.etag) setEtag(written.etag);
									setSavedAt(Date.now());
									await invalidateSelf();
								}}
								onError={async (msg, cause) => {
									if (
										!delegated &&
										submitError(cause, defaults).kind === "conflict"
									) {
										await adoptLatestStore("已載入最新版本，請重新上傳圖片。");
										return;
									}
									toast.error("圖片上傳失敗", { description: msg });
								}}
								onUploadingChange={setUploading}
							/>
							<SellerShareCardPreview
								storeId={serverStore.id}
								storeCode={serverStore.storeCode}
								storeName={data.storeName}
								description={data.description}
								prize={data.prize}
								city={location.county}
								district={location.town}
								logoUrl={images.logoUrl}
								coverUrl={images.coverUrl}
							/>
						</div>
					</div>
				</section>

				<div className="seller-profile__save-bar">
					<small className="seller-profile__save-hint">
						{!canWrite
							? delegated
								? "本工作階段沒有核發店面編輯權限，這裡只能檢視"
								: "商店目前無法編輯，這裡只能檢視"
							: delegated
								? "此修改將以平台管理員身分寫入並通知賣家"
								: uploading
									? "圖片上傳中，請稍候…"
									: formatSaveHint(savedAt, hasPendingProfileChanges)}
					</small>
					{canWrite && (
						<Button
							type="submit"
							disabled={saving || uploading || !hasPendingProfileChanges}
							isLoading={saving || uploading}
							loadingLabel={uploading ? "上傳商店圖片中…" : "儲存商店資訊中…"}
							loadingIndicator={
								uploading ? <UploadLoadingIndicator size={18} /> : undefined
							}
						>
							{delegated ? "檢視變更並儲存" : "儲存商店資訊"}
						</Button>
					)}
				</div>
			</form>

			{delegated ? null : <CloseStoreSection />}
		</div>
	);
}
