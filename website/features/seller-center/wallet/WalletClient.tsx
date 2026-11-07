"use client";

import {
	type PaymentAccount,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { SellerCertGate } from "@/features/seller-center";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDate as formatTxnDate } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useUiText } from "@/lib/i18n/ui-text";
import { fetchOrders } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { queryKeys } from "@/lib/query/keys";
import { CERT_PAGE_HREF, useCertGate } from "@/lib/seller/cert-gate";
import {
	paymentAccountCopy,
	paymentAccountTone,
} from "@/lib/seller/payment-account";
import { walletShowsTransactions } from "@/lib/seller/wallet-transactions";

/**
 * Wallet & certification gateway — mirrors the seller App's
 * `wallet_screen.dart`. Reads cert + payment-account state and surfaces the
 * matching status panel.
 *
 * Seller-type choice lives on `/seller-center/me/certifications`
 * (the original `<TypePicker>`), not here. When the seller hasn't
 * started yet, this page just shows a CTA button that hands off to
 * that picker.
 *
 * UX rule: 收款帳戶的狀態在這裡以唯讀面板顯示；開通本身（金流商的託管頁面或內嵌元件）
 * 在 `/seller-center/me/certifications` 的「收款帳戶」區，面板只引導過去。
 */

export default function WalletClient() {
	const cert = useCertGate();
	const identity = useQueryIdentity();

	const paymentAccount = cert.paymentAccount;
	const fullyApproved = walletShowsTransactions(cert.seller, paymentAccount);
	const transactionParams = React.useMemo(
		() => ({ view: "seller", status: "completed", pageSize: 20 }) as const,
		[],
	);
	const transactionsQuery = useQuery({
		queryKey: queryKeys.seller.orders(identity, transactionParams),
		queryFn: () => fetchOrders(transactionParams),
		enabled: fullyApproved,
		staleTime: 30_000,
	});
	const walletReady =
		!cert.loading && (!fullyApproved || !transactionsQuery.isLoading);

	return (
		<SellerPageShell
			className="seller-wallet"
			title="錢包與認證"
			subtitle="通過平台審核、完成收款帳戶設定後即可正式收款。身分與銀行資料由金流商收集，平台不經手；金流商可能另外要求補件，請留意通知與此頁狀態。"
		>
			{walletReady ? (
				<span data-hk-page-content-ready="seller-wallet" hidden />
			) : null}
			{cert.loading ? (
				<div className="min-h-[420px]" aria-hidden="true" />
			) : (
				<>
					{paymentAccount &&
					paymentAccount.status !== PaymentAccountStatus.ENABLED ? (
						<StatusPanel account={paymentAccount} />
					) : paymentAccount ? null : (
						<SellerCertGate
							application={cert.application}
							paymentAccount={paymentAccount}
							subject="啟用錢包與收款"
						/>
					)}
					{cert.approved && <AccountCertSection />}
					{fullyApproved && (
						<TransactionList
							orders={transactionsQuery.data?.records ?? null}
							error={transactionsQuery.error !== null}
						/>
					)}
				</>
			)}
		</SellerPageShell>
	);
}

/**
 * Format a numeric sales amount as `NT$ X,XXX`. Returns "NT$ --" for
 * missing / NaN so the card never flashes the JS literal "undefined".
 */
function formatTwd(amount: number | undefined): string {
	if (amount == null || !Number.isFinite(amount)) return "NT$ --";
	const hasFraction = Math.floor(amount) !== amount;
	return `NT$ ${amount.toLocaleString("zh-TW", {
		minimumFractionDigits: hasFraction ? 2 : 0,
		maximumFractionDigits: 2,
	})}`;
}

const CONTACT_HREF = "/contact";

/**
 * 平台審核已核准、收款帳戶還不能收款的賣家看到的狀態面板（唯讀）。標題、說明與動作文字都是共用文案目錄的文字；
 * 不顯示金流商的待補條件細項（金流商自己的頁面會列出要補什麼）。動作：能開通或補件的狀態去商家認證頁，
 * 無法啟用的狀態去聯絡客服，其餘（準備中、審核中）去商家認證頁重新整理。
 */
function StatusPanel({ account }: { account: PaymentAccount }) {
	const ut = useUiText();
	const copy = paymentAccountCopy(ut, account);
	const tone = paymentAccountTone(account.status);
	const alert = tone === "warning" || tone === "danger";
	const actionHref =
		account.status === PaymentAccountStatus.DISABLED
			? CONTACT_HREF
			: CERT_PAGE_HREF;
	return (
		<section
			className={
				alert
					? "seller-wallet__panel seller-wallet__panel--alert"
					: "seller-wallet__panel"
			}
		>
			<span aria-hidden="true" className="seller-wallet__panel-emoji">
				{alert ? <StatusAlertIcon /> : <StatusClockIcon />}
			</span>
			{copy.badge ? <h2>{copy.badge}</h2> : null}
			{copy.description ? <p>{copy.description}</p> : null}
			{copy.action ? (
				<Button href={actionHref} variant="bordered" size="sm">
					{copy.action}
				</Button>
			) : null}
		</section>
	);
}

/**
 * 交易明細 — lists the seller's completed orders (每筆交易成功的訂單).
 * Mounted only when the payment account is enabled, so the fetch never fires
 * for sellers who can't yet collect. Uses the seller order view
 * (`view: "seller"`, backend already scopes to this store — no client
 * re-filter) and links each row to the seller order detail page.
 */
function TransactionList({
	orders,
	error,
}: {
	orders: Order[] | null;
	error: boolean;
}) {
	return (
		<section className="seller-wallet__txns" aria-label="交易明細">
			<header className="seller-wallet__txns-head">
				<h2>交易明細</h2>
				<Link className="seller-wallet__txns-all" href="/seller-center/orders">
					查看全部訂單
				</Link>
			</header>
			{orders === null && !error ? (
				<div className="min-h-[320px]" aria-hidden="true" />
			) : error ? (
				<p className="seller-wallet__txns-empty">
					交易明細載入失敗，請稍後再試。
				</p>
			) : orders && orders.length > 0 ? (
				<>
					<ul className="seller-wallet__txn-list seller-mobile-disclosure-desktop">
						{orders.map((order) => (
							<TransactionRow key={order.orderNo} order={order} />
						))}
					</ul>
					<SellerMobileDisclosureList
						items={orders}
						getKey={transactionKey}
						ariaLabel="手機版交易明細"
						getSummary={(order) => ({
							label: `交易 ${order.orderNo}`,
							leading: <SellerMobileDisclosureIcon name="wallet" />,
							title: transactionTitle(order),
							meta: formatTwd(Number(order.totalActualAmount)),
							status: "已完成",
						})}
						renderDetails={(order) => {
							const dateLabel = formatTxnDate(order.paidAt ?? order.createdAt);
							return (
								<>
									<div className="seller-mobile-disclosure__body">
										<div className="seller-mobile-disclosure__detail-row">
											<span>訂單編號</span>
											<strong>{order.orderNo}</strong>
										</div>
										{dateLabel ? (
											<div className="seller-mobile-disclosure__detail-row">
												<span>交易日期</span>
												<span>{dateLabel}</span>
											</div>
										) : null}
										<div className="seller-mobile-disclosure__detail-row">
											<span>交易金額</span>
											<strong>
												{formatTwd(Number(order.totalActualAmount))}
											</strong>
										</div>
									</div>
									<div className="seller-mobile-disclosure__actions">
										<Link
											className="seller-wallet__txns-all"
											href={`/seller-center/orders/${encodeURIComponent(order.orderNo)}`}
										>
											查看訂單
										</Link>
									</div>
								</>
							);
						}}
					/>
				</>
			) : (
				<p className="seller-wallet__txns-empty">目前還沒有交易成功的訂單。</p>
			)}
		</section>
	);
}

function transactionKey(order: Order) {
	return order.orderNo;
}

function transactionTitle(order: Order) {
	const firstItem = order.items[0];
	return firstItem
		? order.items.length > 1
			? `${firstItem.name} 等 ${order.items.length} 項商品`
			: firstItem.name
		: `訂單 ${order.orderNo}`;
}

function TransactionRow({ order }: { order: Order }) {
	const title = transactionTitle(order);
	const dateLabel = formatTxnDate(order.paidAt ?? order.createdAt);

	return (
		<li>
			<Link
				className="seller-wallet__txn"
				href={`/seller-center/orders/${encodeURIComponent(order.orderNo)}`}
			>
				<div className="seller-wallet__txn-meta">
					<strong>{title}</strong>
					<span>
						訂單編號 {order.orderNo}
						{dateLabel ? ` · ${dateLabel}` : ""}
					</span>
				</div>
				<div className="seller-wallet__txn-amount">
					<strong>{formatTwd(Number(order.totalActualAmount))}</strong>
					<span>已完成</span>
				</div>
			</Link>
		</li>
	);
}

function StatusAlertIcon() {
	return <AppIcon name="danger" />;
}

function StatusClockIcon() {
	return <AppIcon name="clock" />;
}

/**
 * 帳戶與認證：平台審核與收款帳戶都已完成後才顯示，一行 row，點開看認證摘要。
 * 讓錢包頁的「錢」(餘額/交易) 與「認證設定」視覺上分開。
 */
function AccountCertSection() {
	const [certOpen, setCertOpen] = React.useState(false);

	return (
		<section className="seller-wallet__section" aria-label="帳戶與認證">
			<h2 className="seller-wallet__section-title">帳戶與認證</h2>
			<button
				type="button"
				className="seller-wallet__acct-row"
				aria-expanded={certOpen}
				onClick={() => setCertOpen((o) => !o)}
			>
				<span className="seller-wallet__acct-row-main">
					<strong>商家認證</strong>
					<span>平台審核與收款帳戶</span>
				</span>
				<span
					className="seller-wallet__acct-row-icon is-ok"
					role="img"
					aria-label="已通過"
				>
					<CertBadgeIcon />
				</span>
				<span
					className={`seller-wallet__acct-row-chev ${certOpen ? "is-open" : ""}`}
					aria-hidden="true"
				>
					<AppIcon name="chevron-right" size={18} />
				</span>
			</button>
			{certOpen && (
				<div className="seller-wallet__acct-expand">
					<div className="seller-wallet__cert-passed-meta" role="status">
						<strong>商家認證已通過</strong>
						<span>
							平台審核與收款帳戶皆已完成，可正常收款與出貨。撥款週期為訂單完成 +
							7 天。
						</span>
					</div>
				</div>
			)}
		</section>
	);
}

function CertBadgeIcon() {
	return <AppIcon name="shield" />;
}
