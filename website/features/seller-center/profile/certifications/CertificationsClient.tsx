"use client";

/**
 * Seller certification (KYC) — single-page form, modelled after the App's
 * wallet-driven flow but compressed onto one Web surface.
 *
 *   1. 載入時讀 GetMyKYC：沒有申請（not_found）→ 「個人賣家 / 企業用戶」二選一卡片，
 *      按一下以 SaveKYCDraft（不帶 etag）建立申請
 *   2. 有申請且可編輯（草稿、被退回、PayUni 拒絕但能重送）→ 整頁長表單，分區塊：
 *        - 公司資訊（僅企業用戶）
 *        - 身分證正反面 + 個人欄位
 *        - 銀行存摺
 *        - 其他資訊（聯絡方式、英文名、店家地址）
 *        - 補充文件（僅企業用戶）
 *      Sticky 底部列：捨棄草稿 + 送出認證；Sticky 左側錨點導覽，跳到對應區塊。
 *      續填的位置由伺服器的 `missing_fields` 推導（第一個還有缺項的區塊）。
 *   3. 已通過 / 審核中 → 只顯示狀態頁，不再開放編輯
 *
 * 申請是店家的單例資源（沒有申請 ID）；每次寫入帶讀到的 etag，版本衝突時保留使用者的輸入並載入最新版本。
 * 文件上傳一律走 uploadSecureDocument()（私密上傳，bytes 不會落公開 OSS），存檔時以 staged token 寫入。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import {
	submitError as classifySubmitError,
	type SubmitError,
} from "@hawkr/api/connect/submit";
import {
	AgreementKind,
	KYCField,
	KYCFlowStatus,
	KYCStatus,
	SellerAppService,
	type StoreField,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { countyCodeOfTownCode } from "@hawkr/twaddr/lookup";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { BackLink } from "@/components/shared/BackLink";
import BrandLogo from "@/components/shared/BrandLogo";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { DatePicker } from "@/components/ui/DatePicker";
import { Dropdown } from "@/components/ui/Dropdown";
import { ImageUploader } from "@/components/ui/ImageUploader";
import { Input } from "@/components/ui/Input";
import { TownFields } from "@/features/account/screens/addresses/TownFields";
import { AgreementCheckbox } from "@/features/seller/components/AgreementCheckbox";
import { PayuniStatusBody } from "@/features/seller-center/components/PayuniStatusBody";
import { useAuth } from "@/lib/auth/context";
import { useResettableError } from "@/lib/hooks/useResettableError";
import {
	useInvalidateSellerSelf,
	useMyKyc,
	useSellerSelf,
	useSetMyKyc,
} from "@/lib/hooks/useSellerSelf";
import {
	useVersionConflict,
	versionConflictNotice,
} from "@/lib/hooks/useVersionConflict";
import {
	type UiText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import { uploadSecureDocument } from "@/lib/media/secure-upload";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	AGREEMENT_OUTDATED_NOTICE,
	acceptancesOf,
	agreementUrl,
	isAgreementOutdated,
} from "@/lib/seller/agreements";
import {
	getBankCodeForValue,
	getBankOptionsForValue,
	normaliseBankName,
	TAIWAN_CITY_OPTIONS,
} from "@/lib/seller/certification-options";
import {
	adoptExistingApplication,
	adoptSavedDocs,
	type BusinessLookup,
	baselineAfterSave,
	buildSaveKycDraftRequest,
	type CertificationSectionId,
	cancelKyc,
	EMPTY_KYC_DOCS,
	EMPTY_KYC_FORM,
	fetchMyKyc,
	type KycDoc,
	type KycDocSlot,
	type KycDocs,
	type KycForm,
	type KycIssueMessages,
	kycDocsOf,
	kycDocumentNotice,
	kycEditable,
	kycFieldLabel,
	kycFieldOfPath,
	kycFormKeyOfDraftPath,
	kycFormOf,
	kycInputErrors,
	kycIssueMessages,
	kycTypeOf,
	lookupBusinessRegistration,
	mergeLatestDocs,
	mergeLatestForm,
	missingItemsOf,
	resumeSectionOf,
	SECTION_LABELS,
	type SellerType,
	saveKycDraft,
	submitKyc,
} from "@/lib/seller/kyc";
import { flowOf, kycFlowBanner, shownFlowStatus } from "@/lib/seller/kyc-flow";
import {
	payuniHasState,
	payuniStateAlert,
	payuniStateTitle,
} from "@/lib/seller/payuni";
import { PAYUNI_LOCK_ON_SUBMIT_NOTICE } from "@/lib/seller/payuni-locked-fields";
import { fetchMyStore, updateMyStore } from "@/lib/seller/repo";
import { isStoreCertified, type MyStore } from "@/lib/seller/store";
import {
	EMPTY_STORE_CONTACT,
	hasStoreContactErrors,
	type StoreContact,
	type StoreContactErrors,
	storeContactErrors,
	storeContactServerErrors,
	storeMissingItemsOf,
	submitPreconditionMessage,
} from "@/lib/seller/store-contact";
import {
	createStoreContactSaver,
	type StoreFormDraft,
	storeDraftOf,
} from "@/lib/seller/store-contact-saver";
import { storeAddressInputError } from "@/lib/seller/store-form";

/* ============================================================
 *               Client-held, non-data wizard state
 * ============================================================ */

/** 店家地址（寫到店家，不是 KYC 申請的欄位）。 */
interface StoreAddressState {
	countyCode: string;
	townCode: string;
	detail: string;
}

type ErrorOf = (key: keyof KycForm) => string | null;

interface SectionAnchor {
	id: CertificationSectionId;
	label: string;
	requiredFor: SellerType | "both";
}

type DiscardDraftInitiator = "type-switch" | "action-bar";

const SECTIONS: SectionAnchor[] = [
	{ id: "company", label: "1. 公司資訊", requiredFor: "business" },
	{ id: "identity", label: "2. 身分證明", requiredFor: "both" },
	{ id: "bank", label: "3. 銀行帳戶", requiredFor: "both" },
	{ id: "other", label: "4. 其他資訊", requiredFor: "both" },
	{ id: "documents", label: "5. 補充文件", requiredFor: "business" },
];

function visibleSections(t: SellerType) {
	return SECTIONS.filter(
		(s) => s.requiredFor === "both" || s.requiredFor === t,
	).map((s, i) => ({
		...s,
		label: `${i + 1}. ${s.label.replace(/^\d+\.\s/, "")}`,
	}));
}

function withCurrentOption(
	options: readonly string[],
	current: string | undefined,
): string[] {
	const value = (current ?? "").trim();
	if (!value || options.includes(value)) return [...options];
	return [value, ...options];
}

/** 從店家帶入的預設值：只在對應欄位還是空的時候補上。 */
function storeDefaults(me: MyStore | null): {
	form: Partial<KycForm>;
	address: StoreAddressState;
	contact: StoreContact;
} {
	const store = me?.store;
	const draft = storeDraftOf(store);
	return {
		form: {
			companyPhone: (store?.contactPhone ?? "").trim(),
		},
		address: draft.address,
		contact: draft.contact,
	};
}

/** 新申請才套用店家的預設值；既有申請的欄位以申請為準，開頁不寫入。 */
function withStoreDefaults(form: KycForm, defaults: Partial<KycForm>): KycForm {
	return {
		...form,
		companyPhone: defaults.companyPhone ?? form.companyPhone,
	};
}

interface ServerFieldError {
	key: keyof KycForm;
	value: string;
	message: string;
}

const KYC_AGREEMENT_KINDS = [
	AgreementKind.KYC_PRIVACY_NOTICE,
	AgreementKind.PAYUNI_TERMS,
] as const;

const VERSION_CONFLICT_NOTICE = versionConflictNotice("申請資料");
const STORE_VERSION_CONFLICT_NOTICE = versionConflictNotice("店家資料");

/* ============================================================
 *                    Page entry
 * ============================================================ */

export default function CertificationsClient() {
	const defaults = useDefaultErrorText();
	const ut = useUiText();
	const searchParams = useSearchParams();
	const invalidateSelf = useInvalidateSellerSelf();
	const setMyKyc = useSetMyKyc();
	const { sessionSettled } = useAuth();
	const { data: seller, loading: sellerLoading } = useSellerSelf();
	const { application, loading: kycLoading, error: kycLoadError } = useMyKyc();
	const [hydrated, setHydrated] = React.useState(false);
	const [error, setError] = useResettableError();

	const [form, setForm] = React.useState<KycForm>(EMPTY_KYC_FORM);
	const [docs, setDocs] = React.useState<KycDocs>(EMPTY_KYC_DOCS);
	const [storeAddress, setStoreAddress] = React.useState<StoreAddressState>({
		countyCode: "",
		townCode: "",
		detail: "",
	});
	const [storeContact, setStoreContact] =
		React.useState<StoreContact>(EMPTY_STORE_CONTACT);
	const [storeRetry, setStoreRetry] = React.useState(0);
	const [storeServerErrors, setStoreServerErrors] = React.useState<{
		contact: StoreContact;
		errors: StoreContactErrors;
	} | null>(null);
	const storeSaverRef = React.useRef(
		createStoreContactSaver({
			save: (update) => updateMyStore(update),
			defaults,
		}),
	);
	const storeFailedRef = React.useRef<StoreFormDraft | null>(null);
	const [storeFailedDraft, setStoreFailedDraft] =
		React.useState<StoreFormDraft | null>(null);
	const [storeSaving, setStoreSaving] = React.useState(false);
	const [busy, setBusy] = React.useState<string | null>(null);
	const discardPendingRef = React.useRef(false);
	const [discardInitiator, setDiscardInitiator] =
		React.useState<DiscardDraftInitiator | null>(null);
	const discardPending = discardInitiator !== null;
	const [banReg, setBanReg] = React.useState<BusinessLookup | null>(null);
	const [submitError, setSubmitError] = useResettableError();
	const [noticeRead, setNoticeRead] = React.useState(false);
	const [payuniAccepted, setPayuniAccepted] = React.useState(false);
	const [submitOk, setSubmitOk] = React.useState(false);
	const [lastSavedAt, setLastSavedAt] = React.useState<number | null>(null);
	const [saving, setSaving] = React.useState(false);
	const [saveFailed, setSaveFailed] = React.useState(false);
	const [serverFieldErrors, setServerFieldErrors] = React.useState<
		ServerFieldError[]
	>([]);

	const formRef = React.useRef(form);
	const docsRef = React.useRef(docs);
	formRef.current = form;
	docsRef.current = docs;
	const etagRef = React.useRef("");
	const baselineRef = React.useRef<KycForm>(EMPTY_KYC_FORM);
	const baselineDocsRef = React.useRef<KycDocs>(EMPTY_KYC_DOCS);
	const saveChainRef = React.useRef<Promise<void>>(Promise.resolve());
	const failedSnapshotRef = React.useRef<{
		form: KycForm;
		docs: KycDocs;
	} | null>(null);
	const resumeRef = React.useRef<CertificationSectionId | null>(null);
	const latestAppRef = React.useRef(application);
	latestAppRef.current = application;

	const adoptApplication = React.useCallback(
		(app: NonNullable<typeof application>) => {
			etagRef.current = app.etag;
			baselineRef.current = kycFormOf(app);
			baselineDocsRef.current = kycDocsOf(app);
		},
		[],
	);

	// Initial load — hydrate the form once from the application (resume) and the store
	React.useEffect(() => {
		if (!sessionSettled) return;
		if (sellerLoading || kycLoading) return;
		if (hydrated) return;
		const defaults = storeDefaults(seller);
		setStoreAddress(defaults.address);
		setStoreContact(defaults.contact);
		storeSaverRef.current.adopt(seller?.store);
		if (application) {
			adoptApplication(application);
			setForm(kycFormOf(application));
			setDocs(kycDocsOf(application));
			resumeRef.current = resumeSectionOf(application.missingFields);
		} else {
			setForm(withStoreDefaults(EMPTY_KYC_FORM, defaults.form));
		}
		if (kycLoadError) {
			setError(
				kycLoadError instanceof Error
					? kycLoadError.message
					: "載入認證資料失敗",
			);
		}
		setHydrated(true);
	}, [
		sessionSettled,
		sellerLoading,
		kycLoading,
		hydrated,
		seller,
		application,
		kycLoadError,
		adoptApplication,
	]);

	const flow = flowOf(application);
	const certified = isStoreCertified(seller);
	const shownStatus = shownFlowStatus(certified, flow.status);
	const editable = kycEditable(application);
	const identityLocked = flow.identityLocked;

	const payuni = payuniHasState(flow.payuni) ? flow.payuni : null;

	// 已通過認證的賣家不需要這頁 — 認證狀態已移到「錢包與認證」頁的
	// 展開式 row。任何已通過的賣家（剛通過或事後造訪）
	// 一律導回錢包頁。此頁僅保留給尚未通過的賣家填 KYC 表單。
	const router = useObservedRouter("seller_certifications");
	React.useEffect(() => {
		if (!hydrated) return;
		if (certified) {
			router.replace("/seller-center/wallet");
		}
	}, [certified, hydrated, router]);

	/* ===== Autosave ===== */

	const draftRequest = React.useCallback(
		(nextForm: KycForm, nextDocs: KycDocs) =>
			buildSaveKycDraftRequest({
				form: nextForm,
				docs: nextDocs,
				baseline: baselineRef.current,
				baselineDocs: baselineDocsRef.current,
				etag: etagRef.current || undefined,
			}),
		[],
	);

	const resolveConflict = useVersionConflict({
		reload: fetchMyKyc,
		adopt: (latest) => {
			latestAppRef.current = latest;
			setMyKyc(latest);
			if (!latest) {
				etagRef.current = "";
				baselineRef.current = EMPTY_KYC_FORM;
				baselineDocsRef.current = EMPTY_KYC_DOCS;
				return "認證申請已取消，請重新開始。";
			}
			const previous = baselineRef.current;
			const previousDocs = baselineDocsRef.current;
			etagRef.current = latest.etag;
			baselineRef.current = {
				...kycFormOf(latest),
				idCardNo: previous.idCardNo,
				bankAccount: previous.bankAccount,
			};
			baselineDocsRef.current = kycDocsOf(latest);
			setForm((current) =>
				mergeLatestForm(current, previous, kycFormOf(latest)),
			);
			setDocs((current) =>
				mergeLatestDocs(current, previousDocs, kycDocsOf(latest)),
			);
			return undefined;
		},
		notify: setError,
		notice: VERSION_CONFLICT_NOTICE,
	});

	const cacheSaved = React.useCallback(
		(saved: NonNullable<typeof application>) => {
			latestAppRef.current = saved;
			setMyKyc(saved);
		},
		[setMyKyc],
	);

	const onSaveFailure = React.useCallback(
		(failure: SubmitError, sentForm: KycForm) => {
			switch (failure.kind) {
				case "conflict":
					return;
				case "fields": {
					const entries: ServerFieldError[] = [];
					const labels = new Set<string>();
					const documentNotices = new Set<string>();
					for (const [path, message] of Object.entries(failure.fields)) {
						const applicationPath = path.replace(/^application\./, "");
						const field = kycFieldOfPath(applicationPath);
						// 證件沒有行內的提示位置：把伺服器對那一張的訊息放進提示，賣家刪掉那一張重新上傳。
						const notice =
							field === undefined
								? undefined
								: kycDocumentNotice(ut, applicationPath, field, message);
						if (notice) {
							documentNotices.add(notice);
							continue;
						}
						const key = kycFormKeyOfDraftPath(path);
						if (key) {
							entries.push({ key, value: String(sentForm[key]), message });
						}
						const label =
							field === undefined ? undefined : kycFieldLabel(ut, field);
						if (label) labels.add(label);
					}
					setServerFieldErrors(entries);
					const parts = [
						...(labels.size > 0
							? [`下列欄位格式不正確：${[...labels].join("、")}`]
							: []),
						...documentNotices,
					];
					setError(
						parts.length > 0
							? parts.join("；")
							: "有欄位格式不正確，請檢查後再試。",
					);
					return;
				}
				case "message":
					setError(failure.message);
					return;
			}
		},
		[ut],
	);

	const persist = React.useCallback(() => {
		const run = async () => {
			const sent = { form: formRef.current, docs: docsRef.current };
			const request = draftRequest(sent.form, sent.docs);
			if (!request) return;
			setSaving(true);
			try {
				const saved = await saveKycDraft(request);
				etagRef.current = saved.etag;
				baselineRef.current = baselineAfterSave(
					baselineRef.current,
					sent.form,
					request.updateMask?.paths ?? [],
				);
				const savedDocs = kycDocsOf(saved);
				baselineDocsRef.current = savedDocs;
				setDocs((current) => adoptSavedDocs(current, sent.docs, savedDocs));
				cacheSaved(saved);
				failedSnapshotRef.current = null;
				setSaveFailed(false);
				setServerFieldErrors([]);
				setLastSavedAt(Date.now());
			} catch (e) {
				const failure = classifySubmitError(
					e,
					defaults,
					SellerAppService.method.saveKYCDraft,
				);
				if (failure.kind === "conflict") {
					await resolveConflict();
				} else {
					failedSnapshotRef.current = sent;
					setSaveFailed(true);
					onSaveFailure(failure, sent.form);
				}
			} finally {
				setSaving(false);
			}
		};
		saveChainRef.current = saveChainRef.current.then(run, run);
		return saveChainRef.current;
	}, [draftRequest, resolveConflict, cacheSaved, onSaveFailure, defaults]);

	const storeDraft = React.useMemo<StoreFormDraft>(
		() => ({ contact: storeContact, address: storeAddress }),
		[storeContact, storeAddress],
	);
	const storeDraftRef = React.useRef(storeDraft);
	storeDraftRef.current = storeDraft;

	const resolveStoreConflict = useVersionConflict({
		reload: fetchMyStore,
		adopt: (latest) => {
			const next = storeSaverRef.current.rebase(
				storeDraftRef.current,
				latest.store,
			);
			setStoreContact(next.contact);
			setStoreAddress(next.address);
			return undefined;
		},
		notify: setError,
		notice: STORE_VERSION_CONFLICT_NOTICE,
	});

	/** 把店家資料寫進店家（排在前一個寫入之後）；成功後重讀申請，缺項以伺服器的清單為準。回傳是否已寫完。 */
	const flushStore = React.useCallback(async (): Promise<boolean> => {
		const draft = storeDraftRef.current;
		if (hasStoreContactErrors(storeContactErrors(draft.contact))) return false;
		setStoreSaving(true);
		let result: Awaited<ReturnType<typeof storeSaverRef.current.flush>>;
		try {
			result = await storeSaverRef.current.flush(draft);
		} finally {
			setStoreSaving(false);
		}
		switch (result.kind) {
			case "idle":
				return true;
			case "saved": {
				storeFailedRef.current = null;
				setStoreFailedDraft(null);
				setLastSavedAt(Date.now());
				setStoreServerErrors(null);
				void invalidateSelf();
				try {
					const fresh = await fetchMyKyc();
					latestAppRef.current = fresh;
					setMyKyc(fresh);
				} catch {
					// 缺項清單稍後的重讀或送出時會更新。
				}
				return true;
			}
			case "conflict":
				await resolveStoreConflict();
				setStoreRetry((n) => n + 1);
				return false;
			case "failed": {
				storeFailedRef.current = draft;
				setStoreFailedDraft(draft);
				const { failure } = result;
				if (failure.kind === "fields") {
					setStoreServerErrors({
						contact: draft.contact,
						errors: storeContactServerErrors(failure.fields),
					});
					setError(failure.message ?? Object.values(failure.fields)[0] ?? null);
				} else {
					setError(failure.message);
				}
				return false;
			}
		}
	}, [invalidateSelf, resolveStoreConflict, setMyKyc]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: storeRetry 只用來在版本衝突之後重新排一次寫入
	React.useEffect(() => {
		if (!hydrated || !application || !editable) return;
		if (hasStoreContactErrors(storeContactErrors(storeDraft.contact))) return;
		if (storeSaverRef.current.pending(storeDraft) === null) return;
		if (storeFailedRef.current === storeDraft) return;
		const t = window.setTimeout(() => {
			void flushStore();
		}, 1000);
		return () => window.clearTimeout(t);
	}, [hydrated, application, editable, storeDraft, flushStore, storeRetry]);

	const hasUnsaved =
		application !== null && editable && draftRequest(form, docs) !== null;
	const storeFailed = storeFailedDraft === storeDraft;
	const storeAddressErr = storeAddressInputError(storeAddress);
	const storeDirty =
		hydrated &&
		application !== null &&
		editable &&
		(storeAddressErr !== undefined ||
			(!hasStoreContactErrors(storeContactErrors(storeDraft.contact)) &&
				storeSaverRef.current.pending(storeDraft) !== null));

	React.useEffect(() => {
		if (!hydrated || !application || !editable) return;
		if (draftRequest(form, docs) === null) return;
		const failed = failedSnapshotRef.current;
		if (failed && failed.form === form && failed.docs === docs) return;
		const t = window.setTimeout(() => {
			void persist();
		}, 1000);
		return () => window.clearTimeout(t);
	}, [hydrated, application, editable, form, docs, draftRequest, persist]);

	// 續填：載入後捲到第一個還有缺項的區塊。
	React.useEffect(() => {
		if (!hydrated || !resumeRef.current) return;
		const target = document.getElementById(`cert-${resumeRef.current}`);
		resumeRef.current = null;
		target?.scrollIntoView({ behavior: "smooth", block: "start" });
	}, [hydrated]);

	/** 建立時別處已經建立了申請：原樣採用，店家預設值不當成修改。 */
	const adoptExisting = React.useCallback(async () => {
		try {
			const latest = await fetchMyKyc();
			latestAppRef.current = latest;
			setMyKyc(latest);
			if (!latest) {
				setError("認證申請已取消，請重新開始。");
				return;
			}
			const adopted = adoptExistingApplication(latest);
			etagRef.current = adopted.etag;
			baselineRef.current = adopted.baseline;
			baselineDocsRef.current = adopted.baselineDocs;
			setForm(adopted.form);
			setDocs(adopted.docs);
			resumeRef.current = resumeSectionOf(latest.missingFields);
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入最新版本失敗");
		}
	}, [setMyKyc]);

	/* ===== Actions ===== */

	const startDraft = React.useCallback(
		async (sellerType: SellerType) => {
			setBusy("creating");
			setError(null);
			try {
				const defaults = storeDefaults(seller);
				const nextForm = withStoreDefaults(
					{ ...EMPTY_KYC_FORM, sellerType },
					defaults.form,
				);
				etagRef.current = "";
				baselineRef.current = EMPTY_KYC_FORM;
				baselineDocsRef.current = EMPTY_KYC_DOCS;
				const request = draftRequest(nextForm, EMPTY_KYC_DOCS);
				if (!request) throw new Error("無法建立認證草稿");
				const saved = await saveKycDraft(request);
				adoptApplication(saved);
				baselineRef.current = baselineAfterSave(
					EMPTY_KYC_FORM,
					nextForm,
					request.updateMask?.paths ?? [],
				);
				setForm(nextForm);
				setDocs(kycDocsOf(saved));
				cacheSaved(saved);
			} catch (e) {
				const failure = classifySubmitError(
					e,
					defaults,
					SellerAppService.method.saveKYCDraft,
				);
				if (failure.kind === "conflict") {
					await adoptExisting();
				} else {
					setError(
						failure.kind === "fields" ? "建立認證草稿失敗" : failure.message,
					);
				}
			} finally {
				setBusy(null);
			}
		},
		[
			seller,
			draftRequest,
			adoptApplication,
			cacheSaved,
			adoptExisting,
			defaults,
		],
	);

	// Auto-start the draft when arriving with `?type=individual|business`
	// from the wallet page's seller-type picker. Skips if an application already
	// exists (so we don't trample an in-progress application).
	React.useEffect(() => {
		if (!hydrated || application) return;
		const requested = searchParams?.get("type");
		if (requested !== "individual" && requested !== "business") return;
		void startDraft(requested as SellerType);
	}, [hydrated, application, searchParams, startDraft]);

	const applyBusinessLookup = React.useCallback((reg: BusinessLookup) => {
		setForm((prev) => ({
			...prev,
			companyName: reg.name || prev.companyName,
			companyRegisterDate: reg.registerDate || prev.companyRegisterDate,
			companyCountyCode: reg.townCode
				? countyCodeOfTownCode(reg.townCode)
				: prev.companyCountyCode,
			companyTownCode: reg.townCode || prev.companyTownCode,
			companyAddressDetail: reg.addressDetail || prev.companyAddressDetail,
		}));
	}, []);

	const handleBanLookup = React.useCallback(
		async ({
			ban: explicitBan,
			silent = false,
		}: {
			ban?: string;
			silent?: boolean;
		} = {}) => {
			const ban = (explicitBan ?? form.taxId).trim();
			if (!/^\d{8}$/.test(ban)) {
				if (!silent) setError("統一編號需為 8 位數字");
				return;
			}
			setBusy("ban-lookup");
			if (!silent) setError(null);
			try {
				const reg = await lookupBusinessRegistration(ban);
				setBanReg(reg);
				if (reg) {
					applyBusinessLookup(reg);
				} else if (!silent) {
					setError("Hawkr 系統查無此統編資料，請手動填入公司資訊。");
				}
			} catch (e) {
				if (!silent) {
					setError(errorText(e, defaults));
				}
			} finally {
				setBusy(null);
			}
		},
		[applyBusinessLookup, form.taxId, defaults],
	);

	const lastAutoBanRef = React.useRef("");
	React.useEffect(() => {
		if (form.sellerType !== "business") return;
		const ban = form.taxId.trim();
		if (ban.length < 8) {
			setBanReg(null);
			return;
		}
		if (!/^\d{8}$/.test(ban) || lastAutoBanRef.current === ban) return;
		const timer = window.setTimeout(() => {
			lastAutoBanRef.current = ban;
			void handleBanLookup({ ban, silent: true });
		}, 500);
		return () => window.clearTimeout(timer);
	}, [form.sellerType, form.taxId, handleBanLookup]);

	const handleSecureUpload = async (
		slot: KycDocSlot,
		file: File,
		docType: string,
	) => {
		setBusy(`upload-${slot}`);
		setError(null);
		try {
			const local = URL.createObjectURL(file);
			const result = await uploadSecureDocument(file, docType);
			setDocs((prev) => ({
				...prev,
				[slot]: {
					assetId: "",
					stagedToken: result.stagedToken,
					preview: local,
				},
			}));
		} catch (e) {
			setError(e instanceof Error ? e.message : "上傳失敗");
		} finally {
			setBusy(null);
		}
	};

	const handleAddDocument = async (file: File) => {
		setBusy("upload-doc");
		setError(null);
		try {
			const local = URL.createObjectURL(file);
			const result = await uploadSecureDocument(file, "supplement");
			setDocs((prev) => ({
				...prev,
				companyDocuments: [
					...prev.companyDocuments,
					{ assetId: "", stagedToken: result.stagedToken, preview: local },
				],
			}));
		} catch (e) {
			setError(e instanceof Error ? e.message : "上傳失敗");
		} finally {
			setBusy(null);
		}
	};

	const removeDocument = (idx: number) => {
		setDocs((prev) => ({
			...prev,
			companyDocuments: prev.companyDocuments.filter((_, i) => i !== idx),
		}));
	};

	const clearDocument = (slot: KycDocSlot) => {
		setDocs((prev) => ({ ...prev, [slot]: null }));
	};

	/* ===== Submit gate — the server decides what is still missing ===== */

	const { idErr, acctErr } = kycInputErrors(form);
	const localContactErrors = storeContactErrors(storeContact);
	const contactErrors: StoreContactErrors =
		storeServerErrors && storeServerErrors.contact === storeContact
			? {
					nameErr:
						localContactErrors.nameErr ?? storeServerErrors.errors.nameErr,
					mobileErr:
						localContactErrors.mobileErr ?? storeServerErrors.errors.mobileErr,
					emailErr:
						localContactErrors.emailErr ?? storeServerErrors.errors.emailErr,
				}
			: localContactErrors;
	const issues = React.useMemo(
		() => kycIssueMessages(application),
		[application],
	);
	const errorOf = (key: keyof KycForm): string | null =>
		serverFieldErrors.find((e) => e.key === key && form[key] === e.value)
			?.message ??
		issues.form[key] ??
		null;
	const missingItems = React.useMemo(
		() => [
			...missingItemsOf(ut, application?.missingFields ?? []),
			...storeMissingItemsOf(ut, application?.missingStoreFields ?? []),
		],
		[ut, application],
	);

	const agreementsAccepted = noticeRead && payuniAccepted;

	const finishSubmit = React.useCallback(async () => {
		setSubmitOk(true);
		void invalidateSelf();
		router.replace("/seller-center/dashboard");
	}, [invalidateSelf, router]);

	const handleSubmit = async () => {
		if (!application || !editable || !agreementsAccepted) return;
		setBusy("submit");
		setSubmitError(null);
		try {
			if (hasStoreContactErrors(localContactErrors)) {
				setSubmitError("聯絡資料格式不正確，請檢查後再試。");
				return;
			}
			if (storeAddressErr) {
				setSubmitError(storeAddressErr);
				return;
			}
			if (!(await flushStore())) {
				setSubmitError("認證資料尚未儲存完成，請稍後再試。");
				return;
			}
			const current = latestAppRef.current;
			const stillMissing = [
				...missingItemsOf(ut, current?.missingFields ?? []),
				...storeMissingItemsOf(ut, current?.missingStoreFields ?? []),
			];
			if (stillMissing.length > 0) {
				setSubmitError(
					`下列欄位尚未填寫：${stillMissing.map((item) => item.label).join("、")}`,
				);
				return;
			}

			await persist();
			if (draftRequest(formRef.current, docsRef.current) !== null) {
				setSubmitError("認證資料尚未儲存完成，請稍後再試。");
				return;
			}
			const submitted = await submitKyc(
				etagRef.current,
				acceptancesOf(KYC_AGREEMENT_KINDS),
			);
			setMyKyc(submitted);
			await finishSubmit();
		} catch (e) {
			const failure = classifySubmitError(
				e,
				defaults,
				SellerAppService.method.submitKYC,
			);
			if (isAgreementOutdated(e)) {
				setNoticeRead(false);
				setPayuniAccepted(false);
				setSubmitError(AGREEMENT_OUTDATED_NOTICE);
			} else if (failure.kind === "conflict") {
				await resolveConflict();
				setSubmitError(VERSION_CONFLICT_NOTICE);
			} else if (failure.kind === "fields") {
				const labels = Object.keys(failure.fields)
					.map((path) => {
						const field = kycFieldOfPath(path);
						return field === undefined ? undefined : kycFieldLabel(ut, field);
					})
					.filter((label): label is string => Boolean(label));
				setSubmitError(
					labels.length > 0
						? `下列欄位尚未填寫：${[...new Set(labels)].join("、")}`
						: (failure.message ?? "送出認證失敗"),
				);
				setMyKyc(await fetchMyKyc());
			} else if (ConnectError.from(e).code === Code.FailedPrecondition) {
				// 店家資料還缺（店家的狀態不在請求裡）：重讀申請，缺哪些由伺服器的清單決定；否則是身分比對等固定訊息。
				let missing: readonly StoreField[] = [];
				try {
					const fresh = await fetchMyKyc();
					latestAppRef.current = fresh;
					setMyKyc(fresh);
					missing = fresh?.missingStoreFields ?? [];
				} catch {
					missing = [];
				}
				setSubmitError(submitPreconditionMessage(ut, missing, failure.message));
			} else {
				setSubmitError(failure.message);
			}
		} finally {
			setBusy(null);
		}
	};

	const handleCancel = async (initiator: DiscardDraftInitiator) => {
		if (!application || discardPendingRef.current) return;
		if (!window.confirm("確定要捨棄目前的認證草稿？已上傳資料會清除。")) return;
		discardPendingRef.current = true;
		setDiscardInitiator(initiator);
		setError(null);
		try {
			await cancelKyc(etagRef.current);
			latestAppRef.current = null;
			setServerFieldErrors([]);
			setMyKyc(null);
			etagRef.current = "";
			baselineRef.current = EMPTY_KYC_FORM;
			baselineDocsRef.current = EMPTY_KYC_DOCS;
			setForm(withStoreDefaults(EMPTY_KYC_FORM, storeDefaults(seller).form));
			setDocs(EMPTY_KYC_DOCS);
			setSubmitOk(false);
		} catch (e) {
			const failure = classifySubmitError(
				e,
				defaults,
				SellerAppService.method.cancelKYC,
			);
			if (failure.kind === "conflict") {
				await resolveConflict();
			} else {
				setError(errorText(e, defaults));
			}
		} finally {
			discardPendingRef.current = false;
			setDiscardInitiator(null);
		}
	};

	const formMainRef = React.useRef<HTMLElement | null>(null);
	const handleAnchorClick = React.useCallback(
		(
			event: React.MouseEvent<HTMLAnchorElement>,
			sectionId: CertificationSectionId,
		) => {
			event.preventDefault();
			const target = formMainRef.current?.querySelector<HTMLElement>(
				`#cert-${sectionId}`,
			);
			if (!target) return;

			target.scrollIntoView({ behavior: "smooth", block: "start" });
			window.history.replaceState(
				null,
				"",
				`${window.location.pathname}${window.location.search}#cert-${sectionId}`,
			);
		},
		[],
	);

	/* ============================================================
	 *                       Render branches
	 * ============================================================ */

	if (!hydrated) {
		return (
			<div className="seller-cert">
				<p className="seller-cert__loading">載入中…</p>
			</div>
		);
	}

	const banner = kycFlowBanner(ut, shownStatus);
	const isApproved = certified;

	if (isApproved || (application !== null && !editable) || submitOk) {
		const lockedPayuni = !isApproved && !submitOk ? payuni : null;
		const lockedTone = lockedPayuni
			? payuniStateAlert(lockedPayuni.state)
				? "warning"
				: "info"
			: (banner?.tone ?? "neutral");
		return (
			<div className="seller-cert">
				<header className="seller-cert__head">
					<BackLink
						fallbackHref="/seller-center/wallet"
						className="seller-cert__back"
					>
						<AppIcon name="arrow-left" size={16} />
						返回
					</BackLink>
					<h1>商家認證</h1>
				</header>
				<section className={`seller-cert__status is-${lockedTone}`}>
					{submitOk ? (
						<>
							<strong>已送出申請</strong>
							<p>審核完成後會以通知告知你結果。</p>
						</>
					) : lockedPayuni ? (
						<>
							<strong>{payuniStateTitle(ut, lockedPayuni.state)}</strong>
							<PayuniStatusBody payuni={lockedPayuni} />
						</>
					) : banner ? (
						<>
							<strong>{banner.title}</strong>
							<p>{banner.description}</p>
						</>
					) : null}
				</section>
				{isApproved && (
					<section className="seller-card">
						<header className="seller-card__head">
							<h2>下一步</h2>
						</header>
						<ul className="seller-cert__next">
							<li>
								<Link href="/seller-center/wallet">回到錢包與認證</Link>
							</li>
							<li>
								<Link href="/seller-center/products/new">新增第一個商品</Link>
							</li>
							<li>
								<Link href="/seller-center/me/shipping">設定配送方式</Link>
							</li>
						</ul>
					</section>
				)}
			</div>
		);
	}

	// Type picker — shown once before any application exists. Source of truth
	// for the seller-type choice; the wallet page links here directly.
	if (!application) {
		return (
			<div className="seller-cert seller-cert--shell">
				<aside className="seller-cert__panel">
					<BackLink
						fallbackHref="/seller-center/wallet"
						className="seller-cert__back"
					>
						<AppIcon name="arrow-left" size={16} />
						返回
					</BackLink>
					<div className="seller-cert__panel-mark" aria-hidden="true">
						<BrandLogo variant="mark" />
					</div>
					<small className="seller-cert__panel-eyebrow">Hawkr · 商家認證</small>
					<h1 className="seller-cert__panel-title">選擇商家類型開始認證</h1>
					<p className="seller-cert__panel-lede">
						通過商家認證後即可開通收款、提領、配送與優惠券。資料皆採私密上傳並加密保存。
					</p>
				</aside>
				<section className="seller-cert__main">
					{error && (
						<p className="seller-cert__error" role="alert">
							{error}
						</p>
					)}
					<TypePicker
						value={form.sellerType}
						onChange={(v) => setForm((prev) => ({ ...prev, sellerType: v }))}
						onStart={() => void startDraft(form.sellerType)}
						busy={busy === "creating"}
					/>
				</section>
			</div>
		);
	}

	const sellerType = kycTypeOf(application) ?? form.sellerType;
	const sections = visibleSections(sellerType);
	const rejection = application.lastRejection;
	const outstandingIssues = application.outstandingIssues;
	const needsResubmit = rejection !== undefined;
	const rejectedBanner = kycFlowBanner(
		ut,
		KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
	);
	const canDiscard =
		application.status === KYCStatus.KYC_STATUS_DRAFT && !identityLocked;

	return (
		<div className="seller-cert seller-cert--shell seller-cert--form-shell">
			<aside className="seller-cert__panel seller-cert__panel--form">
				<BackLink
					fallbackHref="/seller-center/wallet"
					className="seller-cert__back"
				>
					<AppIcon name="arrow-left" size={16} />
					返回
				</BackLink>
				<div className="seller-cert__panel-mark" aria-hidden="true">
					<BrandLogo variant="mark" />
				</div>
				<small className="seller-cert__panel-eyebrow">Hawkr · 商家認證</small>
				<h1 className="seller-cert__panel-title">
					{sellerType === "business" ? "企業用戶認證" : "個人賣家認證"}
				</h1>
				<p className="seller-cert__panel-lede">
					一頁完成所有資料 — 系統會自動暫存草稿，可隨時離開回來繼續。
				</p>

				<nav className="seller-cert__panel-anchors" aria-label="認證表單區塊">
					<ol>
						{sections.map((s, idx) => (
							<li key={s.id}>
								<a
									href={`#cert-${s.id}`}
									onClick={(event) => handleAnchorClick(event, s.id)}
								>
									<span className="seller-cert__panel-anchors-num">
										{idx + 1}
									</span>
									{/* `visibleSections` pre-prefixes label with "N. " —
									 * strip it so the num pill is the only numbering. */}
									{s.label.replace(/^\d+\.\s*/, "")}
								</a>
							</li>
						))}
					</ol>
				</nav>

				<div className="seller-cert__panel-summary">
					<strong>尚需補齊</strong>
					{missingItems.length === 0 ? (
						<small className="is-ok">
							所有必填皆已完成
							<AppIcon name="check" size={14} fill />
						</small>
					) : (
						<>
							<small>{missingItems.length} 項待填</small>
							<ul className="seller-cert__missing-list" aria-label="待填欄位">
								{missingItems.map((item) => (
									<li key={`${item.sectionId}-${item.label}`}>
										<a
											href={`#cert-${item.sectionId}`}
											onClick={(event) =>
												handleAnchorClick(event, item.sectionId)
											}
										>
											<span>{SECTION_LABELS[item.sectionId]}</span>
											<strong>{item.label}</strong>
										</a>
									</li>
								))}
							</ul>
						</>
					)}
				</div>

				<footer className="seller-cert__panel-footer">
					<SaveIndicator
						dirty={
							hasUnsaved ||
							saving ||
							storeSaving ||
							(storeDirty && !storeFailed && storeAddressErr === undefined)
						}
						failed={storeFailed || storeAddressErr !== undefined}
						lastSavedAt={lastSavedAt}
					/>
					{canDiscard && (
						<Button
							variant="bordered"
							color="default"
							size="sm"
							className="seller-cert__type-switch"
							onClick={() => void handleCancel("type-switch")}
							disabled={busy === "submit" || discardPending}
							isLoading={discardInitiator === "type-switch"}
							loadingLabel="捨棄草稿中…"
						>
							更換商家類型 / 重新開始
						</Button>
					)}
				</footer>
			</aside>

			<section
				ref={formMainRef}
				className="seller-cert__main seller-cert__main--form"
			>
				<div className="seller-cert__form-scroll">
					{needsResubmit && rejectedBanner && (
						<section
							className={`seller-cert__status is-${rejectedBanner.tone} is-rejected`}
						>
							<strong>{rejectedBanner.title}</strong>
							<p>{rejectedBanner.description}</p>
							{rejection && (
								<p>{rejection.reason || "請重新確認並提交完整資料。"}</p>
							)}
							{outstandingIssues.map((issue, i) => (
								<p key={`${issue.field}-${issue.documentAssetId}-${i}`}>
									{kycFieldLabel(ut, issue.field) ?? "其他"}：{issue.message}
								</p>
							))}
						</section>
					)}
					{payuni && (
						<section className="seller-cert__status is-warning is-pending_info">
							<strong>{payuniStateTitle(ut, payuni.state)}</strong>
							<PayuniStatusBody payuni={payuni} />
						</section>
					)}

					{error && (
						<p className="seller-cert__error" role="alert">
							{error}
						</p>
					)}

					<form
						className="seller-cert__form"
						onSubmit={(e) => {
							e.preventDefault();
							void handleSubmit();
						}}
					>
						{sellerType === "business" && (
							<SectionCompany
								form={form}
								setForm={setForm}
								onLookupBan={() => void handleBanLookup()}
								banReg={banReg}
								banLooking={busy === "ban-lookup"}
								locked={identityLocked}
								errorOf={errorOf}
							/>
						)}
						<SectionIdentity
							form={form}
							setForm={setForm}
							sellerType={sellerType}
							docs={docs}
							issues={issues}
							idErr={idErr}
							errorOf={errorOf}
							idMasked={maskedIdOf(application)}
							busy={busy}
							locked={identityLocked}
							onUpload={handleSecureUpload}
							onClearDocument={clearDocument}
						/>
						<SectionBank
							form={form}
							setForm={setForm}
							docs={docs}
							issues={issues}
							acctErr={acctErr}
							errorOf={errorOf}
							acctMasked={application.bankAccountMasked}
							busy={busy}
							onUpload={handleSecureUpload}
							onClearDocument={clearDocument}
						/>
						<SectionOther
							form={form}
							setForm={setForm}
							contactErrors={contactErrors}
							addressErr={storeAddressErr}
							storeContact={storeContact}
							setStoreContact={setStoreContact}
							errorOf={errorOf}
							storeAddress={storeAddress}
							setStoreAddress={setStoreAddress}
							locked={identityLocked}
						/>

						{sellerType === "business" && (
							<SectionDocuments
								docs={docs.companyDocuments}
								issues={issues}
								busy={busy}
								onAddDocument={handleAddDocument}
								onRemoveDocument={removeDocument}
							/>
						)}

						<div className="flex flex-col gap-2">
							<AgreementCheckbox
								checked={noticeRead}
								disabled={busy === "submit"}
								onChange={setNoticeRead}
								before="我已閱讀"
								linkText="賣家身分認證個人資料蒐集告知事項"
								href={agreementUrl(AgreementKind.KYC_PRIVACY_NOTICE)}
							/>
							<AgreementCheckbox
								checked={payuniAccepted}
								disabled={busy === "submit"}
								onChange={setPayuniAccepted}
								before="我同意"
								linkText="PAYUNi 服務條款"
								href={agreementUrl(AgreementKind.PAYUNI_TERMS)}
								after="，並了解平台會為我向 PAYUNi 申請子商店與會員"
							/>
						</div>

						{submitError && (
							<p className="seller-cert__error" role="alert">
								{submitError}
							</p>
						)}
					</form>
				</div>

				<div
					className="seller-cert__bar"
					role="toolbar"
					aria-label="認證表單操作"
				>
					<div className="seller-cert__bar-meta">
						{missingItems.length > 0 ? (
							<>
								<strong>尚需 {missingItems.length} 項必填</strong>
								<span>
									{missingItems
										.slice(0, 3)
										.map((item) => item.label)
										.join("、")}
									{missingItems.length > 3 ? "…" : ""}
								</span>
							</>
						) : (
							<>
								<strong>
									所有必填欄位已完成
									<AppIcon name="check" size={14} fill />
								</strong>
								<span>
									送出後 PayUni 預設密碼為你的
									{kycFieldLabel(
										ut,
										sellerType === "business"
											? KYCField.KYC_FIELD_BUSINESS_TAX_ID
											: KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
									)}
								</span>
								{/*
								 * 商店名稱與商店介紹住在「商店資訊」頁，這個流程原本一個字
								 * 都沒提到它們——但送出那一刻它們就鎖了，而鎖定生效的時點
								 * （admin 核准）不在賣家的控制範圍內，也不會有人通知他。
								 * 送出前是他最後有選擇權的時點。
								 */}
								<span>{PAYUNI_LOCK_ON_SUBMIT_NOTICE}</span>
							</>
						)}
					</div>
					<div className="seller-cert__bar-actions">
						{canDiscard && (
							<Button
								variant="bordered"
								color="danger"
								onClick={() => void handleCancel("action-bar")}
								disabled={busy === "submit" || discardPending}
								isLoading={discardInitiator === "action-bar"}
								loadingLabel="捨棄草稿中…"
							>
								捨棄草稿
							</Button>
						)}
						<Button
							onClick={() => void handleSubmit()}
							disabled={
								busy === "submit" ||
								discardPending ||
								missingItems.length > 0 ||
								!agreementsAccepted ||
								storeAddressErr !== undefined ||
								(hasUnsaved && !saveFailed) ||
								saving
							}
							isLoading={busy === "submit"}
							loadingLabel="送出認證中…"
						>
							送出認證
						</Button>
					</div>
				</div>
			</section>
		</div>
	);
}

function maskedIdOf(app: {
	subject: { case?: string; value?: unknown };
}): string {
	const subject = app.subject as
		| { case: "individual"; value: { idCardNoMasked: string } }
		| { case: "business"; value: { repIdCardNoMasked: string } }
		| { case: undefined };
	if (subject.case === "individual") return subject.value.idCardNoMasked;
	if (subject.case === "business") return subject.value.repIdCardNoMasked;
	return "";
}

/* ============================================================
 *                    Section components
 * ============================================================ */

function TypePicker({
	value,
	onChange,
	onStart,
	busy,
}: {
	value: SellerType;
	onChange: (v: SellerType) => void;
	onStart: () => void;
	busy: boolean;
}) {
	return (
		<section className="seller-card seller-cert__step-card">
			<h2>選擇商家類型</h2>
			<p>選擇後將建立認證草稿，可隨時暫存進度。</p>
			<div className="seller-cert__type-grid">
				<button
					type="button"
					className={`seller-cert__type-card ${value === "individual" ? "is-active" : ""}`}
					onClick={() => onChange("individual")}
				>
					<TypePickerIndividualIcon />
					<strong>個人賣家</strong>
					<small>以個人名義販售農產品 / 自家產品</small>
					<small>需身分證 + 存摺</small>
				</button>
				<button
					type="button"
					className={`seller-cert__type-card ${value === "business" ? "is-active" : ""}`}
					onClick={() => onChange("business")}
				>
					<TypePickerBusinessIcon />
					<strong>企業用戶</strong>
					<small>以行號或公司名義販售（需統一編號）</small>
					<small>需公司資訊 + 負責人身分證 + 公司帳戶存摺</small>
				</button>
			</div>
			<footer>
				<Button
					onClick={onStart}
					disabled={busy}
					isLoading={busy}
					loadingLabel="建立認證草稿中…"
				>
					開始填寫
				</Button>
			</footer>
		</section>
	);
}

function TypePickerIndividualIcon() {
	return <AppIcon name="user" size={48} />;
}

function TypePickerBusinessIcon() {
	return <AppIcon name="work" size={48} />;
}

function SaveIndicator({
	dirty,
	failed,
	lastSavedAt,
}: {
	dirty: boolean;
	failed: boolean;
	lastSavedAt: number | null;
}) {
	if (failed) {
		return (
			<span className="seller-cert__save-state is-dirty">店家資料尚未儲存</span>
		);
	}
	if (dirty) {
		return <span className="seller-cert__save-state is-dirty">儲存中…</span>;
	}
	if (!lastSavedAt) return null;
	const ago = Math.floor((Date.now() - lastSavedAt) / 1000);
	const label =
		ago < 5
			? "剛剛已儲存"
			: ago < 60
				? `${ago} 秒前已儲存`
				: `${Math.floor(ago / 60)} 分鐘前已儲存`;
	return <span className="seller-cert__save-state is-clean">{label}</span>;
}

/** 表單控制項的標籤：與缺項清單同一組共用文案目錄的欄位名詞。 */
function useKycFieldLabel(): (field: KYCField) => string {
	const ut = useUiText();
	return (field) => kycFieldLabel(ut, field) ?? "";
}

function SectionCompany({
	form,
	setForm,
	onLookupBan,
	banReg,
	banLooking,
	locked,
	errorOf,
}: {
	form: KycForm;
	setForm: React.Dispatch<React.SetStateAction<KycForm>>;
	onLookupBan: () => void;
	banReg: BusinessLookup | null;
	banLooking: boolean;
	locked: boolean;
	errorOf: ErrorOf;
}) {
	const fieldLabel = useKycFieldLabel();
	return (
		<section
			className="seller-card seller-cert__form-section seller-cert__form-section--company"
			id="cert-company"
			aria-busy={banLooking}
		>
			<header>
				<h2>公司資訊</h2>
				<p>輸入統編後可自動帶入公司名稱、地址與設立日期。</p>
			</header>
			<div className="seller-cert__grid">
				<div className="seller-cert__ban-row">
					<TextField
						label={fieldLabel(KYCField.KYC_FIELD_BUSINESS_TAX_ID)}
						value={form.taxId}
						onChange={(v) =>
							setForm((p) => ({
								...p,
								taxId: v.replace(/\D/g, "").slice(0, 8),
							}))
						}
						hint="輸入 8 位數字會自動查詢，也可按「查詢」重試"
						error={errorOf("taxId")}
						required
						readOnly={locked}
					/>
					<div className="seller-cert__ban-action">
						<Button
							variant="bordered"
							color="default"
							onClick={onLookupBan}
							disabled={banLooking || form.taxId.length !== 8 || locked}
							isLoading={banLooking}
							loadingLabel="查詢公司資料中…"
						>
							查詢
						</Button>
					</div>
				</div>
				{banReg && (
					<p className="seller-cert__hint">
						查得 <strong>{banReg.name}</strong>，登記日{" "}
						{banReg.registerDate || "—"}
					</p>
				)}
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME)}
					value={form.companyName}
					onChange={(v) => setForm((p) => ({ ...p, companyName: v }))}
					error={errorOf("companyName")}
					required
					readOnly={locked}
				/>
				<DatePicker
					label={fieldLabel(KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE)}
					variant="bordered"
					size="lg"
					value={form.companyRegisterDate}
					errorMessage={errorOf("companyRegisterDate")}
					onChange={(v) => setForm((p) => ({ ...p, companyRegisterDate: v }))}
					maxDate={new Date().toISOString().slice(0, 10)}
				/>
				<TownFields
					required={false}
					countyLabel="公司縣市"
					townLabel="公司鄉鎮市區"
					countyCode={form.companyCountyCode}
					townCode={form.companyTownCode}
					onChange={(companyCountyCode, companyTownCode) =>
						setForm((p) => ({ ...p, companyCountyCode, companyTownCode }))
					}
				/>
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS)}
					value={form.companyAddressDetail}
					onChange={(v) => setForm((p) => ({ ...p, companyAddressDetail: v }))}
					error={errorOf("companyAddressDetail")}
				/>
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE)}
					value={form.companyPhone}
					onChange={(v) => setForm((p) => ({ ...p, companyPhone: v }))}
					error={errorOf("companyPhone")}
				/>
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_PRODUCT_TYPE)}
					value={form.productType}
					onChange={(v) => setForm((p) => ({ ...p, productType: v }))}
					hint="例如：水果 / 蔬菜 / 海鮮 / 加工食品"
					error={errorOf("productType")}
					readOnly={locked}
				/>
			</div>
			{banLooking && (
				<div
					className="seller-cert__company-loading"
					role="status"
					aria-live="polite"
					aria-label="正在查詢並整理公司資料"
				>
					<div className="seller-cert__company-loading-content">
						<div
							className="seller-cert__company-loading-mark"
							aria-hidden="true"
						>
							<BrandLogoLoader size="fit" />
						</div>
						<strong>正在查詢公司資料</strong>
						<span>完成後會自動帶入公司名稱、設立日期與地址</span>
					</div>
				</div>
			)}
		</section>
	);
}

function SectionIdentity({
	form,
	setForm,
	sellerType,
	docs,
	issues,
	idErr,
	errorOf,
	idMasked,
	busy,
	locked,
	onUpload,
	onClearDocument,
}: {
	form: KycForm;
	setForm: React.Dispatch<React.SetStateAction<KycForm>>;
	sellerType: SellerType;
	docs: KycDocs;
	issues: KycIssueMessages;
	idErr: string | null;
	errorOf: ErrorOf;
	idMasked: string;
	busy: string | null;
	locked: boolean;
	onUpload: (slot: KycDocSlot, file: File, docType: string) => void;
	onClearDocument: (slot: KycDocSlot) => void;
}) {
	const fieldLabel = useKycFieldLabel();
	return (
		<section
			className="seller-card seller-cert__form-section"
			id="cert-identity"
		>
			<header>
				<h2>身分證明</h2>
				<p>
					{sellerType === "business"
						? "公司負責人或聯絡窗口的身分證正反面照，姓名與字號需可辨識。"
						: "請上傳清晰、無反光的身分證正反面照，姓名與字號需可辨識。"}
				</p>
			</header>
			<div className="seller-cert__field-notes">
				<span>身分證照片僅供認證使用，會於伺服器端加密儲存。</span>
			</div>
			<div className="seller-cert__upload-pair">
				<UploadCard
					label={fieldLabel(KYCField.KYC_FIELD_ID_CARD_FRONT)}
					required
					hint={null}
					preview={docs.idCardFront?.preview ?? ""}
					error={issues.slots.idCardFront}
					uploading={busy === "upload-idCardFront"}
					onPick={(file) => onUpload("idCardFront", file, "id_card_front")}
					onClear={() => onClearDocument("idCardFront")}
				/>
				<UploadCard
					label={fieldLabel(KYCField.KYC_FIELD_ID_CARD_BACK)}
					required
					hint={null}
					preview={docs.idCardBack?.preview ?? ""}
					error={issues.slots.idCardBack}
					uploading={busy === "upload-idCardBack"}
					onPick={(file) => onUpload("idCardBack", file, "id_card_back")}
					onClear={() => onClearDocument("idCardBack")}
				/>
			</div>
			<div className="seller-cert__grid">
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_REAL_NAME)}
					value={form.realName}
					onChange={(v) => setForm((p) => ({ ...p, realName: v }))}
					error={errorOf("realName")}
					required
					readOnly={locked}
				/>
				<TextField
					label={fieldLabel(
						sellerType === "business"
							? KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO
							: KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
					)}
					value={form.idCardNo}
					onChange={(v) =>
						setForm((p) => ({ ...p, idCardNo: v.toUpperCase() }))
					}
					error={idErr ?? errorOf("idCardNo")}
					hint={
						idMasked && !form.idCardNo
							? `已填寫（${idMasked}），如需修改請重新輸入`
							: undefined
					}
					required
					readOnly={locked}
				/>
				<DatePicker
					label={fieldLabel(KYCField.KYC_FIELD_BIRTH_DATE)}
					variant="bordered"
					size="lg"
					value={form.birthDate}
					errorMessage={errorOf("birthDate")}
					onChange={(v) => setForm((p) => ({ ...p, birthDate: v }))}
					maxDate={new Date().toISOString().slice(0, 10)}
				/>
				<DatePicker
					label={fieldLabel(KYCField.KYC_FIELD_ID_CARD_ISSUE_DATE)}
					variant="bordered"
					size="lg"
					yearMode="minguo"
					value={form.idCardIssueDate}
					errorMessage={errorOf("idCardIssueDate")}
					onChange={(v) => setForm((p) => ({ ...p, idCardIssueDate: v }))}
					maxDate={new Date().toISOString().slice(0, 10)}
				/>
				<Dropdown
					label={fieldLabel(KYCField.KYC_FIELD_ID_CARD_ISSUE_TYPE)}
					placeholder="選擇發證類型"
					variant="bordered"
					size="lg"
					floatingLabel
					value={form.idCardIssueType || undefined}
					errorMessage={errorOf("idCardIssueType")}
					onChange={(v) => setForm((p) => ({ ...p, idCardIssueType: v }))}
					items={["初發", "補發", "換發"]}
				/>
				<Dropdown
					label={fieldLabel(KYCField.KYC_FIELD_ID_CARD_ISSUE_LOCATION)}
					placeholder="選擇發證縣市"
					variant="bordered"
					size="lg"
					floatingLabel
					value={form.idCardIssueLocation || undefined}
					errorMessage={errorOf("idCardIssueLocation")}
					onChange={(v) => setForm((p) => ({ ...p, idCardIssueLocation: v }))}
					items={withCurrentOption(
						TAIWAN_CITY_OPTIONS,
						form.idCardIssueLocation,
					)}
					searchable
					noResultsLabel="找不到符合的縣市"
				/>
			</div>
		</section>
	);
}

function SectionBank({
	form,
	setForm,
	docs,
	issues,
	acctErr,
	errorOf,
	acctMasked,
	busy,
	onUpload,
	onClearDocument,
}: {
	form: KycForm;
	setForm: React.Dispatch<React.SetStateAction<KycForm>>;
	docs: KycDocs;
	issues: KycIssueMessages;
	acctErr: string | null;
	errorOf: ErrorOf;
	acctMasked: string;
	busy: string | null;
	onUpload: (slot: KycDocSlot, file: File, docType: string) => void;
	onClearDocument: (slot: KycDocSlot) => void;
}) {
	const fieldLabel = useKycFieldLabel();
	return (
		<section className="seller-card seller-cert__form-section" id="cert-bank">
			<header>
				<h2>銀行帳戶</h2>
				<p>提領貨款使用，請確認帳戶名稱與認證主體一致。</p>
			</header>
			<UploadCard
				label={fieldLabel(KYCField.KYC_FIELD_BANK_BOOK)}
				required
				preview={docs.bankBook?.preview ?? ""}
				error={issues.slots.bankBook}
				uploading={busy === "upload-bankBook"}
				onPick={(file) => onUpload("bankBook", file, "bank_book")}
				onClear={() => onClearDocument("bankBook")}
			/>
			<div className="seller-cert__grid">
				<Dropdown
					label={fieldLabel(KYCField.KYC_FIELD_BANK_NAME)}
					placeholder="選擇銀行或金融機構"
					variant="bordered"
					size="lg"
					floatingLabel
					isRequired
					value={
						form.bankCode || getBankCodeForValue(form.bankName) || undefined
					}
					onChange={(v) => {
						const bankName = normaliseBankName(v);
						setForm((p) => ({
							...p,
							bankCode: getBankCodeForValue(v),
							bankName,
						}));
					}}
					items={getBankOptionsForValue(form.bankCode, form.bankName)}
					errorMessage={errorOf("bankCode")}
					searchable
					description="支援銀行、郵局、信用合作社；送審會保留代碼與名稱。"
					noResultsLabel="找不到符合的金融機構"
				/>
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_BANK_ACCOUNT)}
					value={form.bankAccount}
					onChange={(v) =>
						setForm((p) => ({
							...p,
							bankAccount: v.replace(/\D/g, ""),
						}))
					}
					hint={
						acctErr ??
						(acctMasked && !form.bankAccount
							? `已填寫（${acctMasked}），如需修改請重新輸入`
							: "只能輸入數字（8–16 碼）")
					}
					error={acctErr ?? errorOf("bankAccount")}
					required
				/>
			</div>
		</section>
	);
}

function SectionOther({
	form,
	setForm,
	contactErrors,
	addressErr,
	storeContact,
	setStoreContact,
	errorOf,
	storeAddress,
	setStoreAddress,
	locked,
}: {
	form: KycForm;
	setForm: React.Dispatch<React.SetStateAction<KycForm>>;
	contactErrors: StoreContactErrors;
	addressErr: string | undefined;
	storeContact: StoreContact;
	setStoreContact: React.Dispatch<React.SetStateAction<StoreContact>>;
	errorOf: ErrorOf;
	storeAddress: StoreAddressState;
	setStoreAddress: React.Dispatch<React.SetStateAction<StoreAddressState>>;
	locked: boolean;
}) {
	const fieldLabel = useKycFieldLabel();
	return (
		<section className="seller-card seller-cert__form-section" id="cert-other">
			<header>
				<h2>其他資訊</h2>
				<p>店家地址會同步到你的店鋪頁；其他欄位幫助 PayUni 加速審核。</p>
			</header>
			<div className="seller-cert__grid">
				<TextField
					label="聯絡人姓名"
					value={storeContact.contactName}
					onChange={(v) => setStoreContact((p) => ({ ...p, contactName: v }))}
					hint={contactErrors.nameErr ?? "最多 20 字，會寫入商店設定"}
					error={contactErrors.nameErr}
					required
				/>
				<TextField
					label="聯絡人手機"
					value={storeContact.contactMobile}
					onChange={(v) =>
						setStoreContact((p) => ({
							...p,
							contactMobile: v.replace(/\D/g, "").slice(0, 10),
						}))
					}
					hint={contactErrors.mobileErr ?? "09 開頭的 10 碼手機號碼"}
					error={contactErrors.mobileErr}
					required
				/>
				<TextField
					label="客服電子信箱"
					type="email"
					value={storeContact.supportEmail}
					onChange={(v) => setStoreContact((p) => ({ ...p, supportEmail: v }))}
					hint={contactErrors.emailErr ?? "PayUni 會寄金流相關通知到這個信箱"}
					error={contactErrors.emailErr}
					required
				/>
				<TextField
					label={fieldLabel(KYCField.KYC_FIELD_ENGLISH_NAME)}
					value={form.englishName}
					onChange={(v) => setForm((p) => ({ ...p, englishName: v }))}
					hint="PayUni 金流註冊必填，請與護照英文姓名一致"
					error={errorOf("englishName")}
					required
					readOnly={locked}
				/>
				<TownFields
					countyLabel="商家所在縣市"
					townLabel="商家鄉鎮市區"
					countyCode={storeAddress.countyCode}
					townCode={storeAddress.townCode}
					onChange={(countyCode, townCode) =>
						setStoreAddress((p) => ({ ...p, countyCode, townCode }))
					}
				/>
				<TextField
					label="店家地址"
					value={storeAddress.detail}
					onChange={(v) => setStoreAddress((p) => ({ ...p, detail: v }))}
					hint={addressErr}
					error={addressErr}
					required
				/>
			</div>
		</section>
	);
}

function SectionDocuments({
	docs,
	issues,
	busy,
	onAddDocument,
	onRemoveDocument,
}: {
	docs: KycDoc[];
	issues: KycIssueMessages;
	busy: string | null;
	onAddDocument: (file: File) => void;
	onRemoveDocument: (idx: number) => void;
}) {
	return (
		<section
			className="seller-card seller-cert__form-section"
			id="cert-documents"
		>
			<header>
				<h2>補充文件（選填）</h2>
				<p>
					農產驗證 / 產地證明 / 食品檢驗 /
					冷鏈配送等可上傳作為加分；未上傳也可送審。
				</p>
			</header>
			{issues.companyDocuments && (
				<p className="seller-cert__error" role="alert">
					{issues.companyDocuments}
				</p>
			)}
			<div className="seller-cert__upload-grid">
				{docs.map((doc, i) => (
					<ImageUploader
						key={`${doc.assetId || doc.stagedToken}-${i}`}
						variant="compact"
						size="md"
						value={doc.preview}
						error={issues.companyDocumentByAsset[doc.assetId]}
						crop="photo"
						onPick={(f) => onAddDocument(f)}
						onClear={() => onRemoveDocument(i)}
					/>
				))}
				<ImageUploader
					variant="compact"
					size="md"
					value=""
					uploading={busy === "upload-doc"}
					crop="photo"
					onPick={(f) => onAddDocument(f)}
					hint="新增文件"
				/>
			</div>
		</section>
	);
}

/* ============================================================
 *                       Atoms
 * ============================================================ */

function TextField({
	label,
	value,
	onChange,
	type = "text",
	hint,
	error,
	required,
	readOnly,
}: {
	label: string;
	value: string;
	onChange: (v: string) => void;
	type?: string;
	hint?: string;
	error?: string | null;
	required?: boolean;
	readOnly?: boolean;
}) {
	return (
		<Input
			floatingLabel
			variant="bordered"
			size="lg"
			label={label}
			value={value}
			type={type}
			onChange={(e) => onChange(e.target.value)}
			isRequired={required}
			readOnly={readOnly}
			isInvalid={Boolean(error)}
			errorMessage={error || undefined}
			description={hint && !error ? hint : undefined}
		/>
	);
}

function UploadCard({
	label,
	hint = "伺服器端加密儲存",
	required,
	preview,
	error,
	uploading,
	onPick,
	onClear,
}: {
	label: string;
	required?: boolean;
	hint?: string | null;
	preview: string;
	error?: string;
	uploading: boolean;
	onPick: (file: File) => void;
	onClear: () => void;
}) {
	return (
		<ImageUploader
			variant="card"
			size="md"
			label={label}
			required={required}
			hint={hint ?? undefined}
			error={error}
			value={preview}
			uploading={uploading}
			crop="photo"
			onPick={onPick}
			onClear={onClear}
			aspect="4 / 3"
			previewFit="contain"
		/>
	);
}
