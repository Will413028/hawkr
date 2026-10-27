"use client";

/**
 * 自助開店精靈 — 新賣家建店或 buyer → seller 升級（/seller/onboarding）。
 *
 * 本頁只收店鋪資料。送出時 ApplyStore 建立店家並授予賣家角色（帳號已經有店家時改成更新它）。
 *
 * 送出後以 fetchSession() 重新讀角色：GetSession 每個請求都從資料庫讀角色，
 * ApplyStore 之後直接拿得到賣家角色並切進工作區。拿不到（後端暫時答不
 * 出來）時停在完成頁，由重新登入的 CTA 收尾。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import { AgreementKind } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import Link from "@/components/shared/Link";
import { Input, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TownFields } from "@/features/account/screens/addresses/TownFields";
import { AgreementCheckbox } from "@/features/seller/components/AgreementCheckbox";
import { MEASURES_PATH } from "@/features/seller/measures/measureDisplay";
import { trackSellerOnboardingCompleted } from "@/lib/analytics/events";
import { fetchSession, SESSION_UNAVAILABLE } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import { sellerWorkspaceRedirect } from "@/lib/auth/post-login-redirect";
import { loginPath } from "@/lib/auth/redirect";
import { useHasMeasures } from "@/lib/hooks/useSellerMeasures";
import { useVersionConflict } from "@/lib/hooks/useVersionConflict";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	AGREEMENT_OUTDATED_NOTICE,
	acceptancesOf,
	agreementUrl,
	isAgreementOutdated,
} from "@/lib/seller/agreements";
import {
	finalizeSellerOnboarding,
	type StoreOpeningNotice,
	storeOpeningGate,
	storeOpeningRefusal,
	storeOpeningRefusalHasAppealLink,
} from "@/lib/seller/onboarding";
import {
	applyStore,
	completeOnboarding,
	fetchMyStore,
	fetchStoreOpening,
	updateMyStore,
} from "@/lib/seller/repo";
import {
	isStoreMissing,
	needsOnboarding,
	STORE_STATE_UNKNOWN_MESSAGE,
} from "@/lib/seller/store";
import { storeFormOf, townLocation } from "@/lib/seller/store-form";

type Step = "store" | "done";
type SellerLookupState = "idle" | "checking" | "ready" | "failed";

const SELLER_TERMS_KINDS = [AgreementKind.SELLER_TERMS] as const;

const STORE_EXISTS_NOTICE =
	"店家資料已在其他地方建立，已載入最新版本，你輸入的內容仍保留，請確認後再送出。";

/** 開店精靈頂部的 Hawkr 標誌。 */
function OnboardingMark() {
	return (
		<span
			role="img"
			aria-label="Hawkr"
			className="mx-auto block size-[88px] shrink-0 overflow-hidden sm:size-[112px]"
		>
			<BrandLogoLoader size="fit" />
		</span>
	);
}

export default function SellerOnboardingClient() {
	const router = useObservedRouter("seller_onboarding");
	const searchParams = useSearchParams();
	const {
		hydrated,
		isLoggedIn,
		hasSeller,
		commitIdentity,
		switchRole,
		signOut,
		rejectSession,
	} = useAuth();
	const hasMeasures = useHasMeasures();

	const [step, setStep] = React.useState<Step>("store");
	const [storeName, setStoreName] = React.useState("");
	const [description, setDescription] = React.useState("");
	const [countyCode, setCountyCode] = React.useState("");
	const [townCode, setTownCode] = React.useState("");
	const [addressDetail, setAddressDetail] = React.useState("");

	const [submitting, setSubmitting] = React.useState(false);
	const [pendingAction, setPendingAction] = React.useState<
		"application" | "relogin" | null
	>(null);
	const [error, setError] = React.useState<string | null>(null);
	const [refusal, setRefusal] = React.useState<StoreOpeningNotice | null>(null);
	// 開店可用性（GetMyStoreOpening）：帳號沒有目前店家（會呼叫 ApplyStore）時送出前先讀；有店家走更新流程，不讀。
	// 讀不到不擋，ApplyStore 的 ErrorReason 是後備。
	const [opening, setOpening] = React.useState<
		| { status: "loading" }
		| { status: "failed" }
		| {
				status: "ready";
				blocked: boolean;
				notice: StoreOpeningNotice;
		  }
	>({ status: "loading" });
	const [sellerLookupState, setSellerLookupState] =
		React.useState<SellerLookupState>("idle");
	const sellerOnboardingTrackedRef = React.useRef(false);
	const [termsAccepted, setTermsAccepted] = React.useState(false);
	const [storeExists, setStoreExists] = React.useState(false);

	const redirect = React.useMemo(() => {
		const r = searchParams?.get("redirect") ?? "";
		return sellerWorkspaceRedirect(r);
	}, [searchParams]);

	// 入口 gate：未登入導去登入。有賣家角色不能直接踢去工作區 —— App 註冊到
	// 一半的帳號（拿到角色、店家沒建）GetMyStore 會回 not_found，dashboard
	// 整頁壞掉；只有店家真的存在且 onboarding 完成才導走，否則留在精靈補完。
	React.useEffect(() => {
		if (!hydrated || step !== "store") return;
		if (!isLoggedIn) {
			router.replace(
				loginPath({ role: "seller", redirect: "/seller/onboarding" }),
			);
			return;
		}
		if (!hasSeller) return;
		let cancelled = false;
		setSellerLookupState("checking");
		fetchMyStore()
			.then((res) => {
				if (cancelled) return;
				if (res.store?.id && !needsOnboarding(res)) {
					// This page and SellerHeader can briefly disagree when the
					// shared seller-self query still holds the pre-onboarding
					// result. A document replacement creates a fresh QueryClient
					// before SellerCenterShell evaluates the profile, preventing
					// dashboard ↔ onboarding ping-pong.
					commitDocumentNavigation(redirect, true);
					return;
				}
				setStoreExists(Boolean(res.store?.id));
				setSellerLookupState("ready");
			})
			.catch((cause: unknown) => {
				if (cancelled) return;
				if (isStoreMissing(cause)) {
					setSellerLookupState("ready");
					return;
				}
				// Only a confirmed not_found means "no store yet". Treating a temporary
				// 5xx/network failure as a new store could submit the application
				// twice, so hold the form until the status can be checked again.
				setSellerLookupState("failed");
				setError(STORE_STATE_UNKNOWN_MESSAGE);
			});
		return () => {
			cancelled = true;
		};
	}, [hydrated, step, isLoggedIn, hasSeller, redirect, router]);

	React.useEffect(() => {
		if (!hydrated || !isLoggedIn || step !== "store") return;
		// 有賣家角色時等店家查詢有結果；帳號有店家就不查（送出走更新流程）。
		if (hasSeller && sellerLookupState !== "ready") return;
		if (storeExists) return;
		let cancelled = false;
		fetchStoreOpening()
			.then((res) => {
				if (cancelled) return;
				const { blocked, refusal, message } = storeOpeningGate(res);
				setOpening({ status: "ready", blocked, notice: { refusal, message } });
			})
			.catch(() => {
				if (!cancelled) setOpening({ status: "failed" });
			});
		return () => {
			cancelled = true;
		};
	}, [hydrated, isLoggedIn, step, hasSeller, sellerLookupState, storeExists]);

	const finishSellerApplication = async () => {
		await finalizeSellerOnboarding(
			{
				storeName: storeName.trim(),
				acceptedAgreements:
					willApplyStore && termsAccepted
						? acceptancesOf(SELLER_TERMS_KINDS)
						: [],
				description: description.trim() || undefined,
				address: townCode
					? { townCode, addressDetail: addressDetail.trim() }
					: undefined,
			},
			{
				applyStore,
				fetchStore: fetchMyStore,
				updateStore: updateMyStore,
				complete: completeOnboarding,
				onCompleted: (sellerProfileAction) => {
					if (sellerOnboardingTrackedRef.current) return;
					sellerOnboardingTrackedRef.current = true;
					trackSellerOnboardingCompleted(sellerProfileAction);
				},
			},
		);
		// ApplyStore 改了 DB 的角色集 —— 重新解析一次；拿得到新角色就直接切進
		// 工作區，拿不到就停在完成頁請使用者重新登入。
		let denied: ConnectError | undefined;
		let id: Awaited<ReturnType<typeof fetchSession>> = null;
		try {
			id = await fetchSession();
		} catch (error) {
			denied = ConnectError.from(error);
		}
		if (id === null) {
			await rejectSession(denied);
			router.replace(loginPath({ role: "seller", redirect }));
			return;
		}
		// SESSION_UNAVAILABLE（後端短暫不可用，B-199）視同「確認不了新角色」，
		// 一樣走完成頁的重新登入路徑。
		if (id !== SESSION_UNAVAILABLE && id?.isSeller) {
			commitIdentity(id);
			switchRole("seller");
			// Hard replace is intentional: SellerHeader may still have the old
			// `needsOnboarding=true` / 404 result cached for 30 seconds. Keeping
			// that QueryClient across a soft route would send the completed seller
			// straight back here and create an endless redirect loop.
			commitDocumentNavigation(redirect, true);
			return;
		}
		setStep("done");
	};

	const adoptExistingStore = useVersionConflict({
		reload: fetchMyStore,
		adopt: ({ store }) => {
			if (store?.id) setStoreExists(true);
			const form = storeFormOf(store);
			setStoreName((current) => current || form.storeName);
			setDescription((current) => current || form.description);
			if (!townCode && form.townCode) {
				setCountyCode(form.countyCode);
				setTownCode(form.townCode);
				setAddressDetail((current) => current || form.addressDetail);
			}
			return undefined;
		},
		notify: setError,
		notice: STORE_EXISTS_NOTICE,
	});

	const submitApplication = async () => {
		if (submitting) return;
		setPendingAction("application");
		setSubmitting(true);
		setError(null);
		setRefusal(null);
		try {
			await finishSellerApplication();
		} catch (e) {
			const refused = storeOpeningRefusal(e);
			if (refused) {
				setRefusal(refused);
			} else if (ConnectError.from(e).code === Code.AlreadyExists) {
				await adoptExistingStore();
			} else if (isAgreementOutdated(e)) {
				setTermsAccepted(false);
				setError(AGREEMENT_OUTDATED_NOTICE);
			} else {
				setError(e instanceof Error ? e.message : "申請送出失敗，請再試一次");
			}
		} finally {
			setSubmitting(false);
			setPendingAction(null);
		}
	};

	const reLogin = async () => {
		setPendingAction("relogin");
		setSubmitting(true);
		try {
			await signOut();
		} catch {
			// 撤銷失敗＝session 仍有效（signOut 已跳 toast）。導去登入頁只會拿同一
			// 個仍生效的 session 直接彈回來，等於什麼都沒換——留在原地讓人重試。
			setSubmitting(false);
			setPendingAction(null);
			return;
		}
		// role=seller 讓登入頁預選賣家分頁 —— 重新登入後 switchRole("seller")
		// 才會生效、直接落地賣家中心（否則雙角色帳號預設買家，RoleZoneGuard
		// 會把人從 /seller-center 踢回首頁）。
		// 整頁重載（replace，不留歷史）到登入頁：登出後若 session 期間上了新
		// 部署，soft 導航會去要已刪除的舊 chunk 卡骨架（同其他登出入口）。
		commitDocumentNavigation(loginPath({ role: "seller", redirect }), true);
	};

	// Existing sellers must wait for the admission lookup to resolve before the
	// form can mutate anything. This closes the slow-network window where a user
	// could submit while we still did not know whether a completed store existed.
	const sellerStatusReady =
		hydrated && isLoggedIn && (!hasSeller || sellerLookupState === "ready");
	const willApplyStore = !storeExists;
	// 伺服器說不能開店時擋送出並顯示第一個原因；已有店家走更新，不經 ApplyStore，不受影響。
	const openingReady = willApplyStore && opening.status === "ready";
	const openingBlocked = openingReady && opening.blocked;
	const openingBlock = openingReady && opening.blocked ? opening.notice : null;
	const openingPending = willApplyStore && opening.status === "loading";
	const visibleRefusal = openingBlock ?? refusal;
	const visibleRefusalMessage = visibleRefusal?.message;
	const termsReady = !willApplyStore || termsAccepted;
	const { county: cityValue, town: districtValue } = townLocation(townCode);
	const storeReady =
		storeName.trim().length > 0 &&
		Boolean(cityValue && districtValue && addressDetail.trim()) &&
		sellerStatusReady &&
		termsReady &&
		!openingBlocked &&
		!openingPending;

	const submitStore = () => {
		if (!storeReady || submitting) return;
		setError(null);
		void submitApplication();
	};

	return (
		<main
			data-standalone-flow="seller-onboarding"
			className="mx-auto min-h-[100dvh] w-full max-w-[640px] px-4 pb-8 pt-6 sm:px-5 sm:pb-16 sm:pt-12 lg:pt-20"
		>
			<header className="mb-6 text-center sm:mb-8">
				<OnboardingMark />
				<p className="text-[0.8125rem] font-medium tracking-[0.12em] text-hkui-primary dark:text-[var(--theme-text-accent,#f5dea5)]">
					SELLER · 自助開店
				</p>
				<h1 className="mt-2 text-[1.625rem] font-black leading-tight text-[var(--color-home-accent-700,var(--color-ink-500))] sm:text-[1.75rem] dark:text-[var(--hkui-foreground,var(--color-ink-100))]">
					{step === "done" ? "賣場已建立" : "建立你的賣場"}
				</h1>
			</header>

			{step === "store" && hasMeasures && (
				<Card
					as={Link}
					href="/seller/measures"
					isHoverable
					className="mb-4 flex items-center gap-3 p-4 no-underline"
				>
					<AppIcon name="shield-fail" size={24} />
					<span className="flex-1 text-[0.9375rem] font-bold text-[var(--hkui-foreground)]">
						你有措施紀錄，查看與申覆
					</span>
					<AppIcon name="arrow-right" size={16} />
				</Card>
			)}

			{/* 注意：這張卡不能加 backdrop-filter（blur）— 它會把 Dropdown 的
			    position:fixed 選單改成以卡片定位，整個選單就飄離欄位。 */}
			<section className="rounded-[28px] border border-white/70 bg-white/90 p-5 shadow-[0_24px_64px_-32px_rgb(var(--color-ink-700-rgb)/0.18)] sm:rounded-[36px] sm:p-10 dark:border-white/10 dark:bg-[rgb(var(--color-ink-900-rgb)/0.92)] dark:shadow-[0_24px_64px_-32px_rgba(0,0,0,0.5)]">
				{step === "store" && (
					<form
						className="flex flex-col gap-4"
						onSubmit={(e) => {
							e.preventDefault();
							void submitStore();
						}}
					>
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="商家名稱"
							value={storeName}
							onChange={(e) => setStoreName(e.target.value)}
							maxLength={40}
							description="顯示在買家眼前的商家名稱，之後可以再改"
						/>
						<div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
							<TownFields
								countyCode={countyCode}
								townCode={townCode}
								onChange={(nextCounty, nextTown) => {
									setCountyCode(nextCounty);
									setTownCode(nextTown);
								}}
							/>
						</div>
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="路名門牌"
							value={addressDetail}
							placeholder="例：松仁路 100 號"
							onChange={(e) => setAddressDetail(e.target.value)}
						/>
						<Textarea
							variant="bordered"
							size="lg"
							label="商店介紹"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							minRows={4}
							maxLength={500}
							placeholder="請介紹商店特色、主要商品與產地來源，內容將顯示於您的商店頁面，協助買家認識您的品牌。"
						/>
						{willApplyStore && (
							<AgreementCheckbox
								checked={termsAccepted}
								disabled={submitting}
								onChange={setTermsAccepted}
								before="我已閱讀並同意"
								linkText="Hawkr 賣家契約"
								href={agreementUrl(AgreementKind.SELLER_TERMS)}
							/>
						)}
						<Button
							type="submit"
							disabled={!storeReady || submitting}
							isLoading={submitting && pendingAction === "application"}
							loadingLabel="正在建立賣場…"
						>
							{hasSeller && sellerLookupState === "checking"
								? "確認商店狀態…"
								: "建立賣場"}
						</Button>
					</form>
				)}

				{step === "done" && (
					<div className="flex flex-col items-center gap-4 text-center">
						<span
							aria-hidden
							className="flex h-14 w-14 items-center justify-center rounded-full bg-hkui-primary-50 text-hkui-primary dark:bg-white/10 dark:text-hk-ink-200"
						>
							<AppIcon name="check" size={28} fill />
						</span>
						<p className="text-[1.0625rem] font-bold">
							「{storeName.trim()}」已建立
						</p>
						<p className="max-w-[36ch] text-[0.875rem] leading-relaxed text-black/60 dark:text-white/60">
							賣場資料已建立。商品正式販售前，仍需完成適用的認證、收款、物流與商品設定；重新登入後可繼續處理。
						</p>
						<Button
							type="button"
							onClick={reLogin}
							disabled={submitting}
							isLoading={submitting && pendingAction === "relogin"}
							loadingLabel="準備重新登入…"
						>
							重新登入，進入賣家中心
						</Button>
					</div>
				)}

				{error && (
					<p
						className="mt-4 text-[0.8125rem] text-[var(--hkui-danger,#d4380d)] dark:text-[#f08c6c]"
						role="alert"
					>
						{error}
					</p>
				)}
				{visibleRefusal && visibleRefusalMessage && (
					<div
						className="mt-4 text-[0.8125rem] text-[var(--hkui-danger,#d4380d)] dark:text-[#f08c6c]"
						role="alert"
					>
						<p>{visibleRefusalMessage}</p>
						{storeOpeningRefusalHasAppealLink(visibleRefusal.refusal) && (
							<Link
								href={MEASURES_PATH}
								className="mt-1 inline-block underline"
							>
								措施與申覆
							</Link>
						)}
					</div>
				)}
			</section>
		</main>
	);
}
