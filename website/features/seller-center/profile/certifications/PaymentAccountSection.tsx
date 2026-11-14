"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import {
	OnboardingAction,
	type OnboardingTask,
	OnboardingTaskState,
	type PaymentAccount,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerOnboardingActionButton } from "@hawkr/ui-text/keys";
import dynamic from "next/dynamic";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import Button from "@/components/ui/Button";
import { formatTaipeiMonthDayTime } from "@/lib/date/taipei";
import {
	useMyPaymentAccount,
	useRefreshPaymentAccount,
} from "@/lib/hooks/useSellerSelf";
import {
	optionalText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import {
	CONTACT_HREF,
	createPaymentOnboardingSession,
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
 * 顯示共用文案目錄的狀態徽章；說明句與按鈕只讀開通摘要的收款帳戶任務（`task`），不由帳戶狀態推導。開始開通
 * （託管頁面導頁、內嵌元件展開）。進入頁面時自動重讀一次收款帳戶（失敗不顯示）。
 * 不顯示金流商的待補條件細項：金流商自己的頁面或元件會列出要補什麼。
 */
export function PaymentAccountSection({
	platformApproved,
	task,
	summary,
}: {
	platformApproved: boolean;
	/** 開通摘要的收款帳戶任務；還沒讀到是 undefined。 */
	task: OnboardingTask | undefined;
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
	const [busy, setBusy] = React.useState(false);
	const [confirming, setConfirming] = React.useState(false);
	const [embedded, setEmbedded] = React.useState<Extract<
		OnboardingStart,
		{ kind: "embedded" }
	> | null>(null);
	const [failure, setFailure] = React.useState<{ error: unknown } | null>(null);
	const handledRef = React.useRef(false);

	const refreshPaymentAccount = useRefreshPaymentAccount();

	const begin = React.useCallback(async () => {
		setBusy(true);
		setFailure(null);
		try {
			const started = await startOnboarding({
				createSession: () => createPaymentOnboardingSession(),
			});
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
	}, []);

	// 從金流商回來：先同步重讀（失敗不顯示），再重新讀開通摘要與帳戶。
	const autoRefreshedRef = React.useRef(false);
	const confirm = React.useCallback(async () => {
		autoRefreshedRef.current = true;
		setConfirming(true);
		setFailure(null);
		try {
			await refreshPaymentAccount();
		} catch {
			// 重讀失敗不顯示。
		} finally {
			setConfirming(false);
		}
	}, [refreshPaymentAccount]);

	const param = paymentParamOf(searchParams?.get(PAYMENT_PARAM) ?? null);
	React.useEffect(() => {
		if (!platformApproved || !param || handledRef.current) return;
		handledRef.current = true;
		autoRefreshedRef.current = true;
		window.history.replaceState(null, "", window.location.pathname);
		if (param === "return") {
			void confirm();
		} else {
			void begin();
		}
	}, [platformApproved, param, confirm, begin]);

	// 進入頁面：任務還沒完成（也不是鎖住）時同步重讀一次，背景進行；沒有 query 的進入才做（回跳另有 confirm）。
	const taskState = task?.state;
	React.useEffect(() => {
		if (!platformApproved || summary.loading || autoRefreshedRef.current) {
			return;
		}
		if (
			param ||
			taskState === OnboardingTaskState.COMPLETED ||
			taskState === OnboardingTaskState.LOCKED
		) {
			return;
		}
		autoRefreshedRef.current = true;
		refreshPaymentAccount().catch(() => {});
	}, [
		platformApproved,
		summary.loading,
		param,
		taskState,
		refreshPaymentAccount,
	]);

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

	const actionLabel = task
		? optionalText(ut, sellerOnboardingActionButton(task.action))
		: undefined;

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
						{task?.status?.message ? <p>{task.status.message}</p> : null}
						{copy.payoutsNote ? <p>{copy.payoutsNote}</p> : null}
					</div>
					{task?.action === OnboardingAction.CONTACT_SUPPORT && actionLabel ? (
						<Button href={CONTACT_HREF} variant="bordered" size="sm">
							{actionLabel}
						</Button>
					) : task?.action === OnboardingAction.START_PAYMENT_ONBOARDING &&
						actionLabel ? (
						<Button
							size="sm"
							isLoading={busy}
							loadingLabel="開啟設定中…"
							disabled={busy || embedded !== null}
							onClick={() => void begin()}
						>
							{actionLabel}
						</Button>
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
