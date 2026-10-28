"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { TruckSpinner } from "@/components/shared/TruckSpinner";
import Button from "@/components/ui/Button";
import { Dropdown } from "@/components/ui/Dropdown";
import { Input } from "@/components/ui/Input";
import Switch from "@/components/ui/Switch";
import { AddressAutocompleteField } from "@/features/seller/components/AddressAutocompleteField";
import { SellerCertGate } from "@/features/seller-center";
import {
	createStoreMapSession,
	openStoreMapPopup,
} from "@/lib/checkout/store-pickups";
import { SHIPPING_TYPE_LABEL } from "@/lib/codemeta/enums";
import { useResettableError } from "@/lib/hooks/useResettableError";
import { useShippingOnboardings } from "@/lib/hooks/useShippingOnboardings";
import { AMBIENT_TEMPERATURE_CODE } from "@/lib/seller/ambient-temperature";
import { useCertGate } from "@/lib/seller/cert-gate";
import {
	getDistrictOptions,
	getKnownCity,
	getKnownDistrict,
	TAIWAN_CITY_OPTIONS,
} from "@/lib/seller/certification-options";
import {
	findOnboarding,
	isApproved,
	isFailed,
	isLocked,
	isProcessing,
	PAYUNI_SUBMIT_STATUS,
	SHIPPING_TYPE,
	type ShippingOnboarding,
	statusLabel,
	submitCvsOnboarding,
	submitHomeDeliveryOnboarding,
	toggleOnboarding,
	unsupportedOnboardingTypes,
	validateAddress,
	validationMatchesSelection,
} from "@/lib/seller/shipping";
import { type MyStore, STORE_STATE_UNKNOWN_MESSAGE } from "@/lib/seller/store";
import { checkDisplayLength } from "@/lib/validation/display-length";
import {
	cvsSenderInitial,
	type HomeForm,
	homeFormInitial,
	senderPostalCodeOf,
} from "./prefill";

/**
 * 物流與退貨設定頁。對應 mobile shipping_settings_screen.dart。
 * 三件事一頁完成：
 *   1) 物流 gate — 伺服器的 `SHIPPING_SETUP` 能力沒有啟用時擋住：缺 KYC_APPROVED 引導到
 *      /seller-center/me/certifications 完成認證；缺停權、等待刪除顯示對應說明。
 *   2) 黑貓宅配 — 一個 onboarding row。
 *   3) 7-11 店到店 — 一個 onboarding row。
 *
 * 訂單實際出貨在訂單頁（per-row 預約 / 下載託運單 / 取號）— 此頁只
 * 處理「我這個賣家能不能用哪些配送方式」。
 *
 * 視覺上沿用商家認證頁（seller-cert）的 `seller-card` + Dropdown/Input
 * UI 一致，縣市/區域改為下拉選單。
 */

/**
 * 地址驗證沒過時給賣家看的訊息。
 *
 * 驗證失敗有兩種：自己填錯（縣市／區域與門牌對不上）與地址真的存在、
 * 但地址資料庫查不到（新社區、村里改編、特殊門牌）。後者賣家再怎麼改
 * 也不會過，所以直接把客服指出來，不要讓人卡在這裡反覆重送。
 */
const SUPPORT_EMAIL = "supports@example.com";
const ADDRESS_VERIFY_FAILED_MSG = `地址未通過驗證。請先確認縣市、區域與路名門牌是否一致；若這是真實存在的地址仍無法通過，請聯絡客服 ${SUPPORT_EMAIL}，我們會協助你完成開通。`;

function withCurrentOption(
	options: readonly string[],
	current: string | undefined,
): string[] {
	const value = (current ?? "").trim();
	if (!value || options.includes(value)) return [...options];
	return [value, ...options];
}

export default function ShippingClient() {
	// Cert status comes from the shared store and KYC-flow queries (deduped with the
	// navbar / shell); this page only owns its shipping-onboarding rows.
	const certGate = useCertGate();
	// Shared with DashboardPage / SkuShippingEditor — `refresh` (passed to the
	// sections as `onChange`) refetches the one query, so a method opened here
	// updates the dashboard badge and the product 可配送方案 picker too.
	// `loaded` (first success), not `loading` (= isLoading||isFetching): the
	// shared query refetches on every `onChange`/mutation, and gating the
	// full-page skeleton on isFetching would blank the page — discarding the
	// section forms' in-progress input — on each toggle. Skeleton only until the
	// first load resolves; later refetches update rows in place.
	const { rows, loaded, loading, error, refresh } = useShippingOnboardings();
	// 表單初值只算一次，所以要等「本次掛載後的資料」：快取命中時 loaded 先成立、
	// 背景重新讀取還在跑，這時算初值會拿到舊的 rows。第一次讀取結束（成功或失敗）後才放行，之後不再收回。
	const [settled, setSettled] = React.useState(false);
	if (!settled && (loaded || error) && !loading) setSettled(true);

	const gate = certGate.shippingGate;

	if (!settled || certGate.loading) {
		return (
			<SellerPageShell
				className="shipping-client"
				title="物流與退貨"
				subtitle="設定要開通的配送方式。每張訂單實際出貨流程在「訂單管理」內處理。"
			>
				<p className="shipping-client__loading">載入中…</p>
			</SellerPageShell>
		);
	}

	return (
		<SellerPageShell
			className="shipping-client"
			title="物流與退貨"
			subtitle="設定要開通的配送方式。每張訂單實際出貨流程在「訂單管理」內處理。"
		>
			{gate === "certification" ? (
				<SellerCertGate flow={certGate.flow} subject="設定物流與退貨" />
			) : gate !== "open" ? (
				<p role="status">
					{gate === "pending_deletion"
						? "帳號刪除處理中"
						: gate === "suspended"
							? "商店已暫停營業"
							: STORE_STATE_UNKNOWN_MESSAGE}
				</p>
			) : !loaded ? (
				<Button onClick={() => void refresh()}>重新載入</Button>
			) : (
				<>
					<HomeDeliverySection
						rows={rows}
						store={certGate.seller}
						onChange={refresh}
					/>
					<CvsSection rows={rows} store={certGate.seller} onChange={refresh} />
					<UnsupportedOnboardingNotice rows={rows} />
				</>
			)}

			{error && (
				<p className="shipping-client__error" role="alert">
					{error}
				</p>
			)}
		</SellerPageShell>
	);
}

/**
 * 🔴 本頁只畫得出黑貓（3652）與 7-11 店到店（3651）兩張卡。後端回了第三種
 * `shippingType`（`3654` 商家自送／未來任何新值）時，那筆
 * onboarding 原本會在 UI 上**完全不存在** —— 賣家看不到、關不掉、也不知道
 * 它開著。不可用要說明、不得憑空消失，所以這裡至少要說出來。
 *
 * ⚠️ 這不是最終形狀。正解是 provider slot registry：
 * 新增一個 provider ＝ 加一列 registry ＋ 一個 slot 元件，本檔 diff 為空。
 * 在那之前，這段是「靜默丟棄」與「有東西告訴你」之間的差別。
 */
function UnsupportedOnboardingNotice({ rows }: { rows: ShippingOnboarding[] }) {
	const unsupported = unsupportedOnboardingTypes(rows);
	if (unsupported.length === 0) return null;
	return (
		<p className="shipping-client__error" role="alert">
			你的帳號還有 {unsupported.length} 種配送方式（
			{unsupported
				.map((code) => `${SHIPPING_TYPE_LABEL[code] ?? "未知"}（${code}）`)
				.join("、")}
			）尚未支援在本頁設定，請聯繫平台客服調整。
		</p>
	);
}

/* ============================================================
   黑貓宅配
   ============================================================ */

function HomeDeliverySection({
	rows,
	store,
	onChange,
}: {
	rows: ShippingOnboarding[];
	store: MyStore | null;
	onChange: () => void | Promise<void>;
}) {
	// 以單一 home_delivery row 為主，顯示「目前已開通 / 處理中 / 失敗」三態之一。
	const row = findOnboarding(rows, SHIPPING_TYPE.HOME_DELIVERY);
	const locked = isLocked(row);
	const processing = isProcessing(row);
	const failed = isFailed(row);
	const approved = isApproved(row);

	// 初值只算一次（父層等到 rows 與店家都讀完才渲染本區塊）；之後重新讀取不覆蓋使用者尚未送出的輸入。
	const [form, setForm] = React.useState<HomeForm>(() =>
		homeFormInitial(row, store),
	);
	const [submitting, setSubmitting] = React.useState(false);
	const [validateMsg, setValidateMsg] = React.useState<string | null>(null);
	const [submitError, setSubmitError] = useResettableError();

	// Address verification state — drives the 「驗證 / 已驗證」 button
	// next to 路名門牌 and gates the submit. Approved rows from the
	// backend are shown as verified so seller doesn't have to
	// re-verify just to view.
	const [addressVerified, setAddressVerified] = React.useState(false);
	const [addressVerifying, setAddressVerifying] = React.useState(false);
	const [addrVerifyFeedback, setAddrVerifyFeedback] = React.useState<
		"error" | null
	>(null);
	const addrVerifyFeedbackTimerRef = React.useRef<number | null>(null);

	React.useEffect(() => {
		return () => {
			if (addrVerifyFeedbackTimerRef.current !== null) {
				window.clearTimeout(addrVerifyFeedbackTimerRef.current);
			}
		};
	}, []);

	const update = (patch: Partial<HomeForm>) =>
		setForm((prev) => ({ ...prev, ...patch }));

	// Any change to county / district / address invalidates a previous
	// verification — the seller must re-trigger TGOS or re-pick from the
	// suggestions list.
	const invalidateVerification = React.useCallback(() => {
		setAddressVerified(false);
		setValidateMsg(null);
		setAddrVerifyFeedback(null);
	}, []);

	const flashAddressVerifyError = React.useCallback(() => {
		if (addrVerifyFeedbackTimerRef.current !== null) {
			window.clearTimeout(addrVerifyFeedbackTimerRef.current);
		}
		// 紅色閃動只有 1.1 秒，賣家看不出「為什麼不過、接下來要做什麼」。
		// 真實存在卻驗不過的地址不是賣家自己能解的，直接把客服指出來。
		setValidateMsg(ADDRESS_VERIFY_FAILED_MSG);
		setAddrVerifyFeedback("error");
		addrVerifyFeedbackTimerRef.current = window.setTimeout(() => {
			setAddrVerifyFeedback(null);
			addrVerifyFeedbackTimerRef.current = null;
		}, 1100);
	}, []);

	const handleVerifyClick = async () => {
		setValidateMsg(null);
		setAddressVerifying(true);
		setAddrVerifyFeedback(null);
		try {
			const v = await validateAddress(fullAddress);
			if (
				!validationMatchesSelection(v, form.senderCity, form.senderDistrict)
			) {
				setAddressVerified(false);
				flashAddressVerifyError();
				return;
			}
			setAddressVerified(true);
			setAddrVerifyFeedback(null);
		} catch (e) {
			setAddressVerified(false);
			setValidateMsg(e instanceof Error ? e.message : "驗證失敗，請稍後再試");
		} finally {
			setAddressVerifying(false);
		}
	};

	const fullAddress =
		`${form.senderCity}${form.senderDistrict}${form.senderAddressDetail}`.trim();

	// 黑貓寄件人姓名：顯示長度 [1, 30]（中文 1 字算 2、英文 1 字算 1），
	// 對齊後端 PayUni 物流欄位約束。只有使用者已輸入時才即時提示，避免
	// 空欄一進頁面就亮紅。
	const senderNameError =
		form.senderName.trim().length > 0
			? checkDisplayLength(form.senderName, {
					label: "寄件人姓名",
					min: 1,
					max: 30,
				})
			: null;

	const submit = async () => {
		setSubmitError(null);
		setValidateMsg(null);

		if (
			!form.senderName.trim() ||
			!form.senderPhone.trim() ||
			!form.senderCity.trim() ||
			!form.senderDistrict.trim() ||
			!form.senderAddressDetail.trim()
		) {
			setSubmitError("請填寫所有寄件人資料");
			return;
		}

		const nameLengthError = checkDisplayLength(form.senderName, {
			label: "寄件人姓名",
			min: 1,
			max: 30,
		});
		if (nameLengthError) {
			setSubmitError(nameLengthError);
			return;
		}

		// 郵遞區號由表單選的縣市與區推導；對不到代碼就不能送出。
		const postalCode = senderPostalCodeOf(form.senderCity, form.senderDistrict);
		if (!postalCode) {
			setSubmitError("請重新選擇行政區");
			return;
		}

		setSubmitting(true);
		try {
			// 1) Address validation against TGOS — re-runs even if the user
			//    already clicked 驗證, because the address could have shifted
			//    in a way that didn't fire `invalidateVerification` (e.g.
			//    react state race) and we'd rather double-check than ship a
			//    bad address to PayUni.
			const v = await validateAddress(fullAddress);
			if (
				!validationMatchesSelection(v, form.senderCity, form.senderDistrict)
			) {
				setAddressVerified(false);
				flashAddressVerifyError();
				return;
			}
			setAddressVerified(true);
			setAddrVerifyFeedback(null);

			// 2) Submit to PayUni
			await submitHomeDeliveryOnboarding({
				...form,
				senderPostalCode: postalCode,
			});
			await onChange();
		} catch (e) {
			setSubmitError(e instanceof Error ? e.message : "送出失敗，請稍後再試");
		} finally {
			setSubmitting(false);
		}
	};

	const cityValue = getKnownCity(form.senderCity) || form.senderCity.trim();
	const districtOptions = getDistrictOptions(cityValue);
	const districtValue =
		getKnownDistrict(cityValue, form.senderDistrict) ||
		form.senderDistrict.trim();
	const canPickDistrict = Boolean(cityValue) && districtOptions.length > 0;

	return (
		<section className="seller-card shipping-card">
			<header className="shipping-card__head">
				<div className="shipping-card__title">
					<span
						className="shipping-card__icon shipping-card__icon--home"
						aria-hidden="true"
					>
						<HomeDeliveryIcon />
					</span>
					<div>
						<h2>黑貓宅急便</h2>
						<p>地址審核後即可使用</p>
					</div>
				</div>
				<StatusPill row={row} />
			</header>

			{processing && (
				<p className="shipping-card__hint">
					PayUni 正在審核中，通常 1–3 個工作天完成。完成後會自動啟用。
				</p>
			)}
			{approved && (
				<p className="shipping-card__hint">
					地址已鎖定。如需變更請聯繫 PayUni 客服。
				</p>
			)}
			{failed && row?.providerMessage && (
				<p className="shipping-card__error" role="alert">
					{row.providerMessage}
				</p>
			)}
			<p className="shipping-card__notice">
				<strong>請填寫正確寄件地址。</strong>
				黑貓宅急便開通與後續到府收件會使用這個地址；地址需通過系統驗證，才能送出物流開通申請。
			</p>

			<div className="seller-cert__grid">
				<TextField
					label="寄件人姓名"
					value={form.senderName}
					onChange={(v) => update({ senderName: v })}
					disabled={locked}
					required
					maxLength={30}
					errorMessage={senderNameError ?? undefined}
				/>
				<TextField
					label="聯絡電話"
					value={form.senderPhone}
					onChange={(v) => update({ senderPhone: v })}
					disabled={locked}
					placeholder="0912345678"
					required
				/>
				<Dropdown
					label="縣市"
					placeholder="選擇縣市"
					variant="bordered"
					size="lg"
					floatingLabel
					isRequired
					disabled={locked}
					value={cityValue || undefined}
					onChange={(nextCity) => {
						const nextDistrict = getKnownDistrict(
							nextCity,
							form.senderDistrict,
						);
						update({
							senderCity: nextCity,
							senderDistrict: nextDistrict,
						});
						invalidateVerification();
					}}
					items={withCurrentOption(TAIWAN_CITY_OPTIONS, cityValue)}
					searchable
					noResultsLabel="找不到符合的縣市"
				/>
				<Dropdown
					label="區域"
					placeholder={canPickDistrict ? "選擇鄉鎮市區" : "請先選擇縣市"}
					variant="bordered"
					size="lg"
					floatingLabel
					isRequired
					disabled={locked || !canPickDistrict}
					value={districtValue || undefined}
					onChange={(nextDistrict) => {
						update({ senderDistrict: nextDistrict });
						invalidateVerification();
					}}
					items={withCurrentOption(districtOptions, districtValue)}
					searchable
					noResultsLabel="找不到符合的鄉鎮市區"
				/>
				<div className="shipping-form__full">
					<AddressAutocompleteField
						value={form.senderAddressDetail}
						onChange={(v) => update({ senderAddressDetail: v })}
						county={cityValue || undefined}
						district={districtValue || undefined}
						disabled={locked}
						verified={addressVerified || approved}
						verifying={addressVerifying}
						onSuggestionSelected={() => {
							// Picking a suggestion is itself a TGOS-validated path —
							// flip the verified flag without re-hitting the server.
							setAddressVerified(true);
							setValidateMsg(null);
							setAddrVerifyFeedback(null);
						}}
						onManualEdit={invalidateVerification}
						onVerifyClick={() => void handleVerifyClick()}
						verifyFeedback={addrVerifyFeedback}
						required
					/>
				</div>
			</div>

			{validateMsg && (
				<p className="shipping-card__error" role="alert">
					{validateMsg}
				</p>
			)}
			{submitError && (
				<p className="shipping-card__error" role="alert">
					{submitError}
				</p>
			)}

			<footer className="shipping-card__foot">
				<small className="shipping-card__layers">
					可配送：常溫、冷藏、冷凍
				</small>
				{!locked && (
					<Button
						onClick={submit}
						disabled={submitting}
						isLoading={submitting}
						loadingLabel="送出黑貓開通申請中…"
						loadingIndicator={
							<TruckSpinner size={18} label="送出黑貓開通申請中" />
						}
					>
						{failed ? "重新嘗試開通" : "送出開通申請"}
					</Button>
				)}
			</footer>
		</section>
	);
}

/* ============================================================
   7-11 店到店
   ============================================================ */

interface CvsForm {
	senderName: string;
	senderPhone: string;
	returnStoreCode: string;
	returnStoreName: string;
}

function CvsSection({
	rows,
	store,
	onChange,
}: {
	rows: ShippingOnboarding[];
	store: MyStore | null;
	onChange: () => void | Promise<void>;
}) {
	const reference = findOnboarding(
		rows,
		SHIPPING_TYPE.CVS,
		AMBIENT_TEMPERATURE_CODE,
	);
	const locked = isLocked(reference);
	const approved = isApproved(reference);
	const processing = isProcessing(reference);

	// 初值只算一次；之後重新讀取不覆蓋使用者尚未送出的輸入。
	const [form, setForm] = React.useState<CvsForm>(() => ({
		...cvsSenderInitial(reference, store),
		returnStoreCode: reference?.returnStoreCode ?? "",
		returnStoreName: reference?.returnStoreName ?? "",
	}));
	const [openingMap, setOpeningMap] = React.useState(false);
	const [submitting, setSubmitting] = React.useState(false);
	const [toggling, setToggling] = React.useState(false);
	const [submitError, setSubmitError] = useResettableError();

	const update = (patch: Partial<CvsForm>) =>
		setForm((prev) => ({ ...prev, ...patch }));

	const openStoreMap = async () => {
		setOpeningMap(true);
		setSubmitError(null);
		try {
			// appType 必須是 "web"：桌面瀏覽器要走 /store-map/return 的
			// postMessage 回傳；"seller" 會 302 到 hawkrseller:// deep
			// link，瀏覽器接不住、popup 停在 PayUni 空白頁。
			const session = await createStoreMapSession("web");
			if (!session.redirectUrl) {
				setSubmitError("無法開啟 7-11 電子地圖");
				return;
			}
			const result = await openStoreMapPopup(session.redirectUrl);
			update({
				returnStoreCode: result.storeId,
				returnStoreName: result.storeName,
			});
		} catch (e) {
			setSubmitError(
				e instanceof Error ? e.message : "建立 7-11 地圖工作階段失敗",
			);
		} finally {
			setOpeningMap(false);
		}
	};

	// 超商寄件人姓名：顯示長度 [1, 10]（中文 1 字算 2、英文 1 字算 1），
	// 對齊後端 PayUni 物流欄位約束。只有使用者已輸入時才即時提示。
	const senderNameError =
		form.senderName.trim().length > 0
			? checkDisplayLength(form.senderName, {
					label: "寄件人姓名",
					min: 1,
					max: 10,
				})
			: null;

	const submit = async () => {
		setSubmitError(null);
		if (
			!form.senderName.trim() ||
			!form.senderPhone.trim() ||
			!form.returnStoreCode.trim()
		) {
			setSubmitError("請填寫寄件人姓名、電話，並選擇退貨門市");
			return;
		}

		const nameLengthError = checkDisplayLength(form.senderName, {
			label: "寄件人姓名",
			min: 1,
			max: 10,
		});
		if (nameLengthError) {
			setSubmitError(nameLengthError);
			return;
		}
		setSubmitting(true);
		try {
			await submitCvsOnboarding({
				senderName: form.senderName,
				senderPhone: form.senderPhone,
				returnStoreCode: form.returnStoreCode,
				returnStoreName: form.returnStoreName || undefined,
			});
			await onChange();
		} catch (e) {
			setSubmitError(e instanceof Error ? e.message : "送出失敗");
		} finally {
			setSubmitting(false);
		}
	};

	const toggle = async (row: ShippingOnboarding, enabled: boolean) => {
		if (!row.onboardingId) return;
		setToggling(true);
		setSubmitError(null);
		try {
			await toggleOnboarding(row.onboardingId, enabled);
			await onChange();
		} catch (e) {
			setSubmitError(e instanceof Error ? e.message : "切換失敗");
		} finally {
			setToggling(false);
		}
	};

	const storeDisplay = form.returnStoreName || form.returnStoreCode || "";

	return (
		<section className="seller-card shipping-card">
			<header className="shipping-card__head">
				<div className="shipping-card__title">
					<span
						className="shipping-card__icon shipping-card__icon--cvs"
						aria-hidden="true"
					>
						<CvsStoreIcon />
					</span>
					<div>
						<h2>7-11 店到店</h2>
						<p>買家可從超商取貨</p>
					</div>
				</div>
			</header>

			<div className="seller-cert__grid">
				<TextField
					label="寄件人姓名"
					value={form.senderName}
					onChange={(v) => update({ senderName: v })}
					disabled={locked}
					required
					maxLength={10}
					errorMessage={senderNameError ?? undefined}
				/>
				<TextField
					label="聯絡電話"
					value={form.senderPhone}
					onChange={(v) => update({ senderPhone: v })}
					disabled={locked}
					placeholder="0912345678"
					required
				/>
				<div className="shipping-form__full shipping-form__store">
					<div className="shipping-form__store-row">
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="退貨門市"
							value={storeDisplay}
							readOnly
							placeholder="點右側選擇 7-11 門市"
						/>
						<Button
							variant="bordered"
							color="default"
							className="shrink-0 mb-0"
							onClick={openStoreMap}
							disabled={locked || openingMap}
							isLoading={openingMap}
							loadingLabel="開啟門市地圖中…"
						>
							選擇門市
						</Button>
					</div>
					{!locked && (
						<>
							<small className="seller-cert__hint">
								選完門市後會自動帶入代碼與名稱，也可於下方欄位手動修改。
							</small>
							<div className="shipping-form__store-manual">
								<TextField
									label="門市代碼"
									value={form.returnStoreCode}
									onChange={(v) => update({ returnStoreCode: v })}
									placeholder="必填"
									required
								/>
								<TextField
									label="門市名稱"
									value={form.returnStoreName}
									onChange={(v) => update({ returnStoreName: v })}
									placeholder="選填"
								/>
							</div>
						</>
					)}
				</div>
			</div>

			{submitError && (
				<p className="shipping-card__error" role="alert">
					{submitError}
				</p>
			)}

			<div className="shipping-cvs__rows">
				<div className="shipping-cvs__row">
					<div className="shipping-cvs__row-label">
						<small aria-live="polite">
							{toggling ? "更新中…" : statusLabel(reference)}
						</small>
					</div>
					<div className="shipping-cvs__row-action">
						{!reference ||
						(!approved &&
							!processing &&
							reference.payuniSubmitStatus !==
								PAYUNI_SUBMIT_STATUS.SUCCEEDED) ? (
							<Button
								size="sm"
								onClick={() => void submit()}
								disabled={submitting}
								isLoading={submitting}
								loadingLabel="開通店到店中…"
								loadingIndicator={
									<TruckSpinner size={16} label="開通店到店中" />
								}
							>
								開通
							</Button>
						) : approved ? (
							<Switch
								checked={reference.enabled === "true"}
								onCheckedChange={(enabled) => void toggle(reference, enabled)}
								disabled={toggling}
								busy={toggling}
								ariaLabel={`配送${reference.enabled === "true" ? "已啟用" : "已停用"}`}
							/>
						) : (
							<span className="shipping-cvs__pending">處理中</span>
						)}
					</div>
				</div>
			</div>
		</section>
	);
}

/* ============================================================
   shared bits
   ============================================================ */

function StatusPill({ row }: { row: ShippingOnboarding | undefined }) {
	if (!row?.payuniSubmitStatus) {
		return <span className="shipping-pill shipping-pill--idle">尚未開通</span>;
	}
	const cls = isApproved(row)
		? "shipping-pill--ok"
		: isProcessing(row)
			? "shipping-pill--pending"
			: isFailed(row)
				? "shipping-pill--alert"
				: "shipping-pill--idle";
	return <span className={`shipping-pill ${cls}`}>{statusLabel(row)}</span>;
}

function TextField({
	label,
	value,
	onChange,
	disabled,
	placeholder,
	required,
	maxLength,
	errorMessage,
}: {
	label: string;
	value: string;
	onChange: (v: string) => void;
	disabled?: boolean;
	placeholder?: string;
	required?: boolean;
	maxLength?: number;
	errorMessage?: string;
}) {
	return (
		<Input
			floatingLabel
			variant="bordered"
			size="lg"
			label={label}
			value={value}
			onChange={(e) => onChange(e.target.value)}
			disabled={disabled}
			placeholder={placeholder}
			isRequired={required}
			maxLength={maxLength}
			errorMessage={errorMessage}
			isInvalid={Boolean(errorMessage)}
		/>
	);
}

/** Square carrier glyph card (36px, bordered, white background). */
function CarrierIconBox({ name }: { name: AppIconName }) {
	return (
		<span className="shipping-client__carrier-icon" aria-hidden="true">
			<AppIcon name={name} size={22} />
		</span>
	);
}

function HomeDeliveryIcon() {
	return <CarrierIconBox name="truck" />;
}

function CvsStoreIcon() {
	return <CarrierIconBox name="bag" />;
}
