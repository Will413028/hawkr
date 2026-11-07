"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code } from "@connectrpc/connect";
import {
	OnboardingSurface,
	type PaymentAccount,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQueryClient } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import { formatTaipeiMonthDayTime } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useMyPaymentAccount } from "@/lib/hooks/useSellerSelf";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { useLocalizedHref } from "@/lib/intl/use-router";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { queryKeys } from "@/lib/query/keys";
import {
	createPaymentOnboardingSession,
	fetchMyPaymentAccount,
	paymentAccountCanOnboard,
	paymentAccountCopy,
	paymentAccountTone,
} from "@/lib/seller/payment-account";
import {
	embeddedClientSecret,
	handlePaymentParam,
	onboardingUrls,
	PAYMENT_PARAM,
	paymentParamOf,
	startOnboarding,
} from "@/lib/seller/payment-onboarding";

const EmbeddedPaymentOnboarding = dynamic(
	() => import("./EmbeddedPaymentOnboarding"),
	{ ssr: false },
);

const CERT_PATH = "/seller-center/me/certifications";
const CONTACT_HREF = "/contact";
const sleep = (ms: number) =>
	new Promise<void>((resolve) => window.setTimeout(resolve, ms));

function formatUpdated(account: PaymentAccount): string | null {
	if (!account.fetchTime) return null;
	return formatTaipeiMonthDayTime(timestampDate(account.fetchTime)) || null;
}

/**
 * 「收款帳戶」區（D15）：平台審核核准（`platformApproved`）之前是灰色的提示；核准之後依 `GetMyPaymentAccount`
 * 顯示共用文案目錄的狀態、說明與動作，並開始開通（託管頁面導頁、內嵌元件展開）。
 * 不顯示金流商的待補條件細項：金流商自己的頁面或元件會列出要補什麼。
 */
export function PaymentAccountSection({
	platformApproved,
}: {
	platformApproved: boolean;
}) {
	const ut = useUiText();
	const defaults = useDefaultErrorText();
	const searchParams = useSearchParams();
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	const {
		account,
		loading,
		error: loadError,
		refetch,
	} = useMyPaymentAccount({
		enabled: platformApproved,
	});
	const localizedPath = useLocalizedHref(CERT_PATH);
	const [busy, setBusy] = React.useState(false);
	const [confirming, setConfirming] = React.useState(false);
	const [embeddedOpen, setEmbeddedOpen] = React.useState(false);
	const [failed, setFailed] = React.useState(false);
	const accountRef = React.useRef(account);
	accountRef.current = account;
	const handledRef = React.useRef(false);

	const urls = React.useCallback(
		() => onboardingUrls(window.location.origin, localizedPath),
		[localizedPath],
	);
	const createSession = React.useCallback(
		() => createPaymentOnboardingSession(urls()),
		[urls],
	);
	const readAccount = React.useCallback(async () => {
		const fresh = await fetchMyPaymentAccount();
		queryClient.setQueryData(queryKeys.seller.paymentAccount(identity), fresh);
		return fresh;
	}, [queryClient, identity]);

	const begin = React.useCallback(
		async (surface: OnboardingSurface) => {
			setBusy(true);
			setFailed(false);
			try {
				const started = await startOnboarding(surface, { createSession });
				if (started.kind === "redirect") {
					commitDocumentNavigation(started.url);
					return;
				}
				setEmbeddedOpen(true);
			} catch {
				setFailed(true);
			} finally {
				setBusy(false);
			}
		},
		[createSession],
	);

	const confirm = React.useCallback(async () => {
		setConfirming(true);
		setFailed(false);
		try {
			await handlePaymentParam("return", {
				readAccount,
				sleep,
				createSession,
				surface: accountRef.current?.onboardingSurface,
			});
		} catch {
			setFailed(true);
		} finally {
			setConfirming(false);
		}
	}, [readAccount, createSession]);

	const param = paymentParamOf(searchParams?.get(PAYMENT_PARAM) ?? null);
	React.useEffect(() => {
		if (!platformApproved || !param || handledRef.current) return;
		if (param === "refresh" && !account) return;
		handledRef.current = true;
		window.history.replaceState(null, "", window.location.pathname);
		if (param === "return") {
			void confirm();
		} else if (account) {
			void begin(account.onboardingSurface);
		}
	}, [platformApproved, param, account, confirm, begin]);

	const publishableKey = process.env.NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY ?? "";
	const copy = account ? paymentAccountCopy(ut, account) : null;
	const tone = account ? paymentAccountTone(account.status) : undefined;
	const genericError = defaults(Code.Internal);

	if (!platformApproved) {
		return (
			<section className="seller-card" aria-label="收款帳戶">
				<header className="seller-card__head">
					<h2>收款帳戶</h2>
				</header>
				<div className="seller-cert__status is-neutral">
					<p>平台審核通過後開放設定</p>
				</div>
			</section>
		);
	}

	const canOnboard = account ? paymentAccountCanOnboard(account) : false;
	const disabled = account?.status === PaymentAccountStatus.DISABLED;
	const missingKey =
		embeddedOpen &&
		account?.onboardingSurface === OnboardingSurface.EMBEDDED &&
		publishableKey === "";

	return (
		<section
			className="seller-card"
			aria-label="收款帳戶"
			aria-busy={confirming}
		>
			<header className="seller-card__head">
				<h2>收款帳戶</h2>
			</header>
			{loading ? (
				<p>載入中…</p>
			) : confirming ? (
				<div className="seller-cert__status is-info" role="status">
					<p>正在確認資料…</p>
				</div>
			) : loadError || !copy || !copy.badge ? (
				<div className="seller-cert__status is-warning" role="alert">
					<p>{genericError}</p>
					<Button variant="bordered" size="sm" onClick={() => void refetch()}>
						重新整理
					</Button>
				</div>
			) : (
				<>
					<div className={`seller-cert__status is-${tone ?? "neutral"}`}>
						<strong>{copy.badge}</strong>
						{copy.description ? <p>{copy.description}</p> : null}
						{copy.payoutsNote ? <p>{copy.payoutsNote}</p> : null}
					</div>
					{copy.action ? (
						disabled ? (
							<Button href={CONTACT_HREF} variant="bordered" size="sm">
								{copy.action}
							</Button>
						) : canOnboard ? (
							<Button
								size="sm"
								isLoading={busy}
								loadingLabel="開啟設定中…"
								disabled={busy || embeddedOpen}
								onClick={() =>
									void begin(
										account?.onboardingSurface ?? OnboardingSurface.UNSPECIFIED,
									)
								}
							>
								{copy.action}
							</Button>
						) : account?.status ===
							PaymentAccountStatus.ACTION_REQUIRED ? null : (
							<Button
								variant="bordered"
								size="sm"
								disabled={confirming}
								onClick={() => void refetch()}
							>
								{copy.action}
							</Button>
						)
					) : null}
					{account && formatUpdated(account) ? (
						<small>上次更新 {formatUpdated(account)}</small>
					) : null}
				</>
			)}
			{failed || missingKey ? (
				<p className="seller-cert__error" role="alert">
					{genericError}
				</p>
			) : null}
			{embeddedOpen && !missingKey && publishableKey !== "" ? (
				<EmbeddedPaymentOnboarding
					publishableKey={publishableKey}
					fetchClientSecret={() => embeddedClientSecret(createSession)}
					onExit={() => {
						setEmbeddedOpen(false);
						void confirm();
					}}
					onLoadError={() => setFailed(true)}
				/>
			) : null}
		</section>
	);
}
