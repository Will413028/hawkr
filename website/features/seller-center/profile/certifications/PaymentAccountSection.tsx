"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import {
	OnboardingSurface,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import type { PaymentAccount } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import dynamic from "next/dynamic";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import { formatTaipeiMonthDayTime } from "@/lib/date/taipei";
import {
	useMyPaymentAccount,
	useRefreshPaymentAccount,
} from "@/lib/hooks/useSellerSelf";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import {
	CONTACT_HREF,
	createPaymentOnboardingSession,
	paymentAccountCanOnboard,
	paymentAccountCopy,
	paymentAccountTone,
} from "@/lib/seller/payment-account";
import {
	type OnboardingStart,
	PAYMENT_PARAM,
	paymentParamOf,
	startOnboarding,
} from "@/lib/seller/payment-onboarding";

const EmbeddedPaymentOnboarding = dynamic(
	() => import("./EmbeddedPaymentOnboarding"),
	{ ssr: false },
);

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
	summary,
}: {
	platformApproved: boolean;
	/** 開通摘要（`useMyOnboarding`）的載入與錯誤狀態：讀失敗時不能判斷平台審核，顯示錯誤而不是「尚未開放」。 */
	summary: {
		loading: boolean;
		error: Error | null;
		refetch: () => unknown;
	};
}) {
	const ut = useUiText();
	const defaults = useDefaultErrorText();
	const searchParams = useSearchParams();
	const {
		account,
		loading,
		error: loadError,
		refetch,
	} = useMyPaymentAccount({
		enabled: platformApproved,
	});
	const locale = useRoutedLocale();
	const [busy, setBusy] = React.useState(false);
	const [confirming, setConfirming] = React.useState(false);
	const [embedded, setEmbedded] = React.useState<Extract<
		OnboardingStart,
		{ kind: "embedded" }
	> | null>(null);
	const [failure, setFailure] = React.useState<{ error: unknown } | null>(null);
	const accountRef = React.useRef(account);
	accountRef.current = account;
	const handledRef = React.useRef(false);

	const createSession = React.useCallback(
		() => createPaymentOnboardingSession(locale),
		[locale],
	);
	const refreshPaymentAccount = useRefreshPaymentAccount();

	const begin = React.useCallback(
		async (surface: OnboardingSurface) => {
			setBusy(true);
			setFailure(null);
			try {
				const started = await startOnboarding(surface, { createSession });
				if (started.kind === "redirect") {
					commitDocumentNavigation(started.url);
					return;
				}
				setEmbedded(started);
			} catch (error) {
				setFailure({ error });
			} finally {
				setBusy(false);
			}
		},
		[createSession],
	);

	// 從金流商回來與狀態按鈕是同一條路徑：先同步重讀，再重新讀開通摘要與帳戶。
	const confirm = React.useCallback(async () => {
		setConfirming(true);
		setFailure(null);
		try {
			await refreshPaymentAccount();
		} catch (error) {
			setFailure({ error });
		} finally {
			setConfirming(false);
		}
	}, [refreshPaymentAccount]);

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

	const copy = account ? paymentAccountCopy(ut, account) : null;
	const tone = account ? paymentAccountTone(account.status) : undefined;
	const genericError = defaults(Code.Internal);

	if (summary.loading) {
		return (
			<section className="seller-card" aria-label="收款帳戶">
				<header className="seller-card__head">
					<h2>收款帳戶</h2>
				</header>
				<p>載入中…</p>
			</section>
		);
	}

	if (summary.error && !platformApproved) {
		return (
			<section className="seller-card" aria-label="收款帳戶">
				<header className="seller-card__head">
					<h2>收款帳戶</h2>
				</header>
				<div className="seller-cert__status is-warning" role="alert">
					<p>{errorText(summary.error, defaults)}</p>
					<Button
						variant="bordered"
						size="sm"
						onClick={() => void summary.refetch()}
					>
						重新整理
					</Button>
				</div>
			</section>
		);
	}

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
								disabled={busy || embedded !== null}
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
								onClick={() => void confirm()}
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
			{failure ? (
				<p className="seller-cert__error" role="alert">
					{errorText(failure.error, defaults)}
				</p>
			) : null}
			{embedded?.provider === "stripe" ? (
				<EmbeddedPaymentOnboarding
					publishableKey={embedded.publishableKey}
					fetchClientSecret={embedded.fetchClientSecret}
					onExit={() => {
						setEmbedded(null);
						void confirm();
					}}
					onLoadError={() =>
						setFailure({
							error: new ConnectError("embedded load", Code.Internal),
						})
					}
				/>
			) : null}
		</section>
	);
}
