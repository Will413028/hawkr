"use client";

import type { SellerPayUni } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { SellerCertGate } from "@/features/seller-center";
import { PayuniStatusBody } from "@/features/seller-center/components/PayuniStatusBody";
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
	payuniFixPointsAtStore,
	payuniHasState,
	payuniStateAlert,
	payuniStateTitle,
} from "@/lib/seller/payuni";
import { walletShowsTransactions } from "@/lib/seller/wallet-transactions";

/**
 * Wallet & certification gateway — mirrors the seller App's
 * `wallet_screen.dart`. Reads cert + PayUni state and surfaces the
 * matching status panel.
 *
 * Seller-type choice lives on `/seller-center/me/certifications`
 * (the original `<TypePicker>`), not here. When the seller hasn't
 * started yet, this page just shows a CTA button that hands off to
 * that picker.
 *
 * UX rule: PayUni status is shown inline here. The standalone
 * `/seller-center/me/payuni` settings page has been retired — system
 * auto-registers after cert approval with the fields PayUni's bind API
 * accepts. PayUni may still separately request member-level supplementary
 * data (no formal resubmission API exists yet, issue #2806 Phase 2/3) —
 * this page surfaces the real status/reason so support can route it.
 */

export default function WalletClient() {
	const cert = useCertGate();
	const identity = useQueryIdentity();

	const payuni = cert.flow?.payuni;
	const fullyApproved = walletShowsTransactions(cert.seller, cert.flow);
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
			subtitle="通過商家認證、完成 PayUni 建店後即可正式收款。系統會在身分認證通過後，自動代為向 PayUni 送出已知的聯絡與商家資料；PayUni 仍可能另外要求補充會員層資料，請留意通知與此頁狀態。"
		>
			{walletReady ? (
				<span data-hk-page-content-ready="seller-wallet" hidden />
			) : null}
			{cert.loading ? (
				<div className="min-h-[420px]" aria-hidden="true" />
			) : (
				<>
					{payuniHasState(payuni) ? (
						<StatusPanel payuni={payuni} canResubmit={cert.canResubmit} />
					) : payuni ? null : (
						<SellerCertGate flow={cert.flow} subject="啟用錢包與收款" />
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

const STORE_SETTINGS_HREF = "/seller-center/me";

interface StatusPanelProps {
	payuni: SellerPayUni;
	canResubmit: boolean;
}

/**
 * 商家認證已核准、PayUni 還沒有可以收款的賣家看到的狀態面板（唯讀）。標題是目錄的名詞、說明與審核意見、欄位清單都來自
 * 伺服器（`flow.payuni`）；「前往補件」只看 `can_resubmit`，「前往店家設定」只看店家欄位的清單。
 */
function StatusPanel({ payuni, canResubmit }: StatusPanelProps) {
	const ut = useUiText();
	const alert = payuniStateAlert(payuni.state);
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
			<h2>{payuniStateTitle(ut, payuni.state)}</h2>
			<PayuniStatusBody payuni={payuni} />
			{payuniFixPointsAtStore(payuni) && (
				<Button href={STORE_SETTINGS_HREF} variant="bordered" size="sm">
					前往店家設定
				</Button>
			)}
			{canResubmit && (
				<Button href={CERT_PAGE_HREF} variant="bordered" size="sm">
					前往補件
				</Button>
			)}
		</section>
	);
}

/**
 * 交易明細 — lists the seller's completed orders (每筆交易成功的訂單).
 * Mounted only in the PayUni-approved branch, so the fetch never fires
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
 * 帳戶與認證：商家認證與 PayUni 都已核准後才顯示，一行 row，點開看認證摘要。
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
					<span>身分、銀行帳戶、PayUni 資料</span>
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
							身分、銀行帳戶與 PayUni
							收款皆已完成驗證，可正常收款與出貨。撥款週期為訂單完成 + 7 天。
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
