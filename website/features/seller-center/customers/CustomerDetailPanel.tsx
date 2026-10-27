"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { useDrawerMutated } from "@/components/shared/Drawer";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useCustomerDetail,
	useInvalidateCustomerDetail,
} from "@/lib/hooks/useCustomerDetail";
import { STATUS_LABEL } from "@/lib/orders/types";
import {
	type SellerCustomer,
	type SellerCustomerType,
	updateCustomerType,
} from "@/lib/seller/repo";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import CustomerTagEditor from "./CustomerTagEditor";

const CLASS_OPTIONS: Array<{
	value: SellerCustomerType;
	label: string;
	loadingLabel: string;
}> = [
	{ value: "normal", label: "一般", loadingLabel: "設為一般客戶中…" },
	{ value: "vip", label: "VIP", loadingLabel: "設為 VIP 客戶中…" },
	{ value: "blocklist", label: "黑名單", loadingLabel: "加入黑名單中…" },
];

/**
 * VIP / 黑名單 / 一般 三態切換 — `PATCH …/customers/{id}` with `customer_type`.
 * Optimistic; rolls back on failure. Mirrors the app's customer classification.
 */
function CustomerClassControl({
	buyerId,
	initial,
	onChanged,
}: {
	buyerId: string | number;
	initial: SellerCustomerType;
	onChanged?: () => void;
}) {
	const [value, setValue] = React.useState<SellerCustomerType>(initial);
	const [pendingType, setPendingType] =
		React.useState<SellerCustomerType | null>(null);

	// Keep in sync when the detail load resolves a different class than the
	// optimistic summary the drawer opened with.
	React.useEffect(() => {
		setValue(initial);
	}, [initial]);

	const pick = async (next: SellerCustomerType) => {
		if (next === value || pendingType !== null) return;
		const prev = value;
		setValue(next);
		setPendingType(next);
		try {
			await updateCustomerType(buyerId, next);
			onChanged?.();
		} catch {
			setValue(prev);
		} finally {
			setPendingType(null);
		}
	};

	return (
		<fieldset className="seller-customer-detail__class">
			<legend className="visually-hidden">客戶分類</legend>
			{CLASS_OPTIONS.map((opt) => (
				<Button
					key={opt.value}
					type="button"
					variant="light"
					color="default"
					size="sm"
					className={`seller-customer-detail__class-btn h-auto px-[10px] py-[3px] is-${opt.value}${
						value === opt.value ? " is-active" : ""
					}`}
					aria-pressed={value === opt.value}
					onClick={() => void pick(opt.value)}
					disabled={pendingType !== null}
					isLoading={pendingType === opt.value}
					loadingLabel={opt.loadingLabel}
				>
					{opt.label}
				</Button>
			))}
		</fieldset>
	);
}

/** Format a unix timestamp (seconds or ms) as Taipei wall-clock date-time.
 *  Unit detection lives in `toEpochMs`, so this no longer carries its own copy. */
function formatOrderTime(ts?: number): string {
	return formatTaipeiDateTime(ts) || "—";
}

interface Props {
	buyerId: string | number;
	/** List-row data for an instant header before the detail request resolves
	 * (used when the panel opens inside the customer-list drawer). */
	summary?: SellerCustomer | null;
}

/**
 * The CRM customer-detail body — profile + classification + tags + stats +
 * purchase history. Rendered both as a full page (wrapped in SellerPageShell)
 * and inside the right-side drawer on the customer list, so a seller can flick
 * between customers without leaving the list.
 */
export default function CustomerDetailPanel({ buyerId, summary }: Props) {
	const { data, loading, error } = useCustomerDetail(buyerId);
	const invalidateDetail = useInvalidateCustomerDetail();
	const openChat = useChatDrawerStore((state) => state.openChat);
	// 抽屜模式：即時通知父頁清單同步（分類 / 標籤變更）；全頁模式為 no-op。
	const { notify } = useDrawerMutated();
	const detail = data?.detail ?? null;
	const orders = data?.orders ?? [];
	const orderTotal = data?.orderTotal ?? 0;

	// After a classification / tag mutation, refetch this customer's detail so a
	// reopen shows the new state, then notify the host list behind the drawer to
	// re-sync its rows immediately (#1247: no longer deferred to drawer close).
	const handleMutate = React.useCallback(() => {
		void invalidateDetail(buyerId);
		notify();
	}, [buyerId, invalidateDetail, notify]);

	// Prefer freshly-loaded detail; fall back to the summary so the drawer header
	// is populated immediately on open.
	const name = detail?.name || summary?.name || "客戶";
	const avatar = detail?.avatar ?? summary?.avatar;
	const phone = detail?.phone ?? summary?.phone;
	const totalSpent = Number(detail?.totalSpent ?? summary?.totalSpent ?? 0);
	const orderCount = detail?.orderCount ?? summary?.orderCount ?? orderTotal;
	const classType = detail?.customerType ?? summary?.customerType ?? "normal";

	return (
		<div className="seller-customer-detail__panel">
			<div className="seller-customer-detail__panel-actions">
				<Link
					href={`/seller-center/coupons/new?buyerId=${buyerId}&buyerName=${encodeURIComponent(name)}`}
					// `.seller-orders__action` 自帶 `--hkui-primary` 實心底＋
					// `--hkui-primary-foreground` 白字（0-1-0）；dark 的
					// `body a:not([data-hkui-button])`（0-2-2）會把白字換成 `--color-ink-200`。
					// 旁邊那顆同款的「傳訊息」是 `<button>`，不受影響 ⇒ 少了這個標記
					// 就會變成同一排兩顆鈕字色不同。
					data-hkui-button=""
					className="seller-orders__action seller-customer-detail__action-primary"
				>
					<AppIcon name="ticket" size={16} />
					發優惠券
				</Link>
				<button
					type="button"
					onClick={() => openChat(buyerId, { name, avatar })}
					className="seller-orders__action"
				>
					發訊息
				</button>
			</div>

			{error && (
				<p className="seller-customers__error" role="alert">
					{error}
				</p>
			)}

			<div className="seller-card seller-customer-detail__profile">
				<Avatar
					src={avatar}
					name={name}
					alt={name}
					entity="buyer"
					className="seller-customer-detail__avatar"
					fallbackClassName="seller-customers__avatar-fallback"
				/>
				<div className="seller-customer-detail__profile-info">
					<div className="seller-customer-detail__name-row">
						<strong>{name}</strong>
						<CustomerClassControl
							buyerId={buyerId}
							initial={classType}
							onChanged={handleMutate}
						/>
					</div>
					<div className="seller-customer-detail__meta">
						{phone && <span>{phone}</span>}
						{detail?.email && <span>{detail.email}</span>}
						{detail?.address && <span>{detail.address}</span>}
					</div>
					<CustomerTagEditor
						buyerId={buyerId}
						initialTags={detail?.tags ?? summary?.tags}
						onChanged={handleMutate}
					/>
				</div>
			</div>

			<section
				className="seller-customers__stats seller-summary-rail"
				aria-label="客戶交易摘要"
				// biome-ignore lint/a11y/noNoninteractiveTabindex: This horizontally scrollable summary needs a keyboard focus target.
				tabIndex={0}
			>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-primary">
						<AppIcon name="box" />
					</span>
					<strong>{orderCount.toLocaleString()}</strong>
					<small>累計訂單</small>
				</div>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-info">
						<AppIcon name="coin" />
					</span>
					<strong>NT$ {Math.round(totalSpent).toLocaleString()}</strong>
					<small>累計消費</small>
				</div>
				<div className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon is-secondary">
						<AppIcon name="ticket" />
					</span>
					<strong>{(detail?.couponCount ?? 0).toLocaleString()}</strong>
					<small>持有優惠券</small>
				</div>
			</section>

			<div className="seller-card seller-customers__card">
				<div className="seller-customer-detail__section-head">
					<h2>購買紀錄</h2>
					<span>{orderTotal.toLocaleString()} 筆</span>
				</div>
				<div className="seller-orders__table-wrap">
					<table className="seller-customers__table seller-customers__table--mini">
						<thead>
							<tr>
								<th>訂單編號</th>
								<th className="is-center">下單時間</th>
								<th className="is-center">訂單狀態</th>
								<th className="is-right">實付金額</th>
							</tr>
						</thead>
						<tbody>
							{loading ? (
								<tr>
									<td colSpan={4} className="seller-orders__empty">
										載入中…
									</td>
								</tr>
							) : orders.length === 0 ? (
								<tr>
									<td colSpan={4} className="seller-orders__empty">
										尚無購買紀錄
									</td>
								</tr>
							) : (
								orders.map((o) => (
									<tr key={o.orderNo}>
										<td>
											<Link
												href={`/seller-center/orders/${encodeURIComponent(o.orderNo)}`}
												className="seller-orders__order-no seller-customer-detail__order-link"
											>
												{o.orderNo}
											</Link>
										</td>
										<td className="is-center seller-orders__shipping">
											{formatOrderTime(o.createdAt)}
										</td>
										<td className="is-center">
											{STATUS_LABEL[o.status] || o.status}
										</td>
										<td className="is-right seller-orders__amount">
											NT$ {Number(o.totalActualAmount || 0).toLocaleString()}
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</div>
			</div>
		</div>
	);
}
