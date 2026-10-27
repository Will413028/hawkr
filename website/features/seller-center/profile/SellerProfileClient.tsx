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
	useMyKycFlow,
	useSellerSelf,
} from "@/lib/hooks/useSellerSelf";
import { useVersionConflict } from "@/lib/hooks/useVersionConflict";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { fetchProfile } from "@/lib/profile/repo";
import { queryKeys } from "@/lib/query/keys";
import {
	payuniLockedNoticeFor,
	stripPayuniLockedFields,
} from "@/lib/seller/payuni-locked-fields";
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
import { SUPPORT_EMAIL_MAX_LENGTH } from "@/lib/seller/validators";
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
	const { flow } = useMyKycFlow();
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
	// 表單只記使用者改過的欄位，其餘欄位直接顯示伺服器的最新值：版本衝突時使用者的輸入
	// 原樣保留，沒改的欄位跟著最新版本更新。
	const [edits, setEdits] = React.useState<Partial<StoreForm>>({});
	const data: StoreForm = React.useMemo(
		() => ({ ...baseline, ...edits }),
		[baseline, edits],
	);
	// 寫入帶的 etag：第一次載入的版本，之後只從「自己這一次寫入的回應」或衝突後讀到的
	// 最新版本更新。背景 refetch（staleTime ＋ refetchOnWindowFocus）回來的版本不能直接
	// 抄進來，否則別人在這段期間的修改會被當成自己的版本、樂觀鎖從此不會擋。
	const [etag, setEtag] = React.useState<string | null>(null);
	const serverEtag = serverStore?.etag;
	React.useEffect(() => {
		if (serverEtag && etag === null) setEtag(serverEtag);
	}, [serverEtag, etag]);
	// 版本衝突後直接讀一次最新版本取它的 etag：快取可能早已是最新版本（視窗聚焦的
	// refetch），這時 query 的 etag 不會再變，只靠快取更新會讓每次儲存都繼續衝突。
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

	// Login email/phone is page-local — not part of the store.
	// 代行下不掛：那是**代行者自己**的登入資料（`/app/profile` 走賣家傳輸層，在代行
	// 執行期會 throw），而且店家的登入 Email／手機是 storefront allow-list 明確排除
	// 的個資，代行工作區本來就不該顯示它。
	const accountQuery = useQuery({
		queryKey: queryKeys.buyer.profile(identity),
		queryFn: () => fetchProfile({ suppressAuthExpired: true }),
		enabled: !delegated,
	});
	const account = accountQuery.data ?? null;
	const loading = accountQuery.isLoading;

	// Re-render while a save timestamp exists so the relative "N 分鐘前" hint
	// stays accurate without a user interaction.
	const [, tick] = React.useReducer((n: number) => n + 1, 0);
	React.useEffect(() => {
		if (!savedAt) return;
		const id = setInterval(tick, 30_000);
		return () => clearInterval(id);
	}, [savedAt]);

	const update = (patch: Partial<StoreForm>) =>
		setEdits((prev) => ({ ...prev, ...patch }));
	/**
	 * 註冊已有 MerID 之後，商店名稱與商店介紹就鎖定（對應 PayUni 的 `MerName`／`MerDesc`）。
	 *
	 * 取伺服器推導的 `flow.identity_locked` 而不是表單狀態：這是伺服器端的事實。
	 */
	const payuniLockedNotice = payuniLockedNoticeFor(flow?.identityLocked);
	const payuniLocked = payuniLockedNotice !== undefined;
	/**
	 * 鎖定欄位在這裡就從變更拿掉，而不是等送出前才濾。
	 *
	 * 這一個位置同時修好三件事：送出的變更不含必定被伺服器拒絕的欄位、
	 * 「尚有未儲存的商店資料」不會因為一個送不出去的變更而恆亮、代行確認抽屜的
	 * 變更摘要也不會列出根本不會送的欄位。
	 */
	const pendingProfileChanges = React.useMemo(
		() =>
			stripPayuniLockedFields(storeFormChanges(data, baseline), payuniLocked),
		[data, baseline, payuniLocked],
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
		// 儲存鍵在唯讀下不渲染，但表單仍在：多數瀏覽器對只有單一文字欄的 form 會做
		// 隱式送出。少了這一行，「不出現 write CTA」就只擋得住滑鼠。
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
		const changes = stripPayuniLockedFields(
			storeFormChanges(submitted, baseline),
			payuniLocked,
		);
		// 代行下開確認抽屜收齊修改原因／目前版本／通知賣家；賣家路徑立刻放行且不帶
		// meta，這一段對 `/seller-center` 是零行為變化。
		const decision = await scope.confirmMutation({
			title: "檢視變更並儲存",
			// ⚠️ 店名取伺服器版本而不是表單裡的值：這句話回答的是「我正在對哪
			// 一家店動手」，而改名正是這個表單能做的事之一 —— 用表單值會讓抽屜以**還
			// 沒存進去的新名字**指稱目標店家，同一個抽屜的變更摘要卻同時列著舊名。
			// 拿不到伺服器版本時退店家編號而不是「本店」（`targetStoreLabel`）。
			// 店名用引號括起：中英混排的店名（`翠綠有機農場 E2E`）直接串接後會與下一個
			// 中文字黏成「E2E儲存」。
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
			// Logo/cover are persisted by their own staged-upload writes; this request
			// carries the text and address fields that changed.
			const written = await scope.repos.storefront.updateMyStore(
				{ etag, ...changes },
				decision.meta,
			);
			// 在 `invalidateSelf()` 之前更新，且來源是這一次的回應而不是接下來的
			// refetch —— 理由見 etag 的說明。
			if (written.etag) setEtag(written.etag);
			setEdits({});
			setServerContactErrors(null);
			setSavedAt(Date.now());
			// Navbar / shell read the same store query — refresh them.
			await invalidateSelf();
			// 代行下不呼叫：那支 server action revalidate 的是**代行者自己**登入態下的
			// 賣家頁快取標籤，而代行者在 www 上沒有賣家身分。目標店家的公開頁由後端
			// 寫入路徑自己失效。
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
				// 保留使用者的輸入，改顯示最新版本：沒改的欄位跟著更新，寫入改用最新的 etag。
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
									readOnly={!canWrite || payuniLocked}
									description={payuniLockedNotice}
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
									maxLength={20}
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
									description="PayUni 會寄金流相關通知到這個信箱"
									errorMessage={contactErrors.emailErr ?? undefined}
									isInvalid={contactErrors.emailErr !== null}
									readOnly={!canWrite}
									maxLength={SUPPORT_EMAIL_MAX_LENGTH}
								/>
								{/* 下拉沒有 `readOnly`（它不是 `<input>`），唯讀下只能
								    `disabled`。這裡沒有「disabled 與尚未修改長得一樣」
								    的歧義 —— 那個歧義只發生在儲存鍵上。 */}
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
									readOnly={!canWrite || payuniLocked}
									description={payuniLockedNotice}
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
									// 換圖寫入的回應才是我方剛推進的那一格版本；顯示用的圖片網址
									// 由 refetch 帶回來。
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
						{/* mockup §4.4.8 左側提示。⚠️「儲存草稿」那顆次要按鈕不得渲染：
						    店面設定沒有草稿狀態，按下去會當場改掉買家看得到的店面，
						    而按鈕字面承諾「還沒公開」（B4）。 */}
						{/* 唯讀時這一句不得再承諾寫入：它旁邊已經沒有儲存鍵了，
						    留著「將以平台管理員身分寫入」等於指向一個不存在的動作。 */}
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
