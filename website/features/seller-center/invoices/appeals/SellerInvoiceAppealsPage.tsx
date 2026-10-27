"use client";

/**
 * S09｜買家發票申訴列表（58A §S09）。端點 34 是本頁唯一的資料源，**本頁沒有任何
 * mutation**——回覆一律在 S10。
 *
 * 權限：`CapOrderFulfill`，**必須在 `SUSPENSION_WHITELIST_PREFIXES` 內**。
 * 不放行會製造死結：申訴有 3 工作天 SLA、逾期即計違規、90 天內 3 次即停權 ⇒
 * 鎖住等於「因為停權所以無法回覆，因為無法回覆所以繼續累積違規」（58 §7.3.5）。
 * 對應的 hook 因此在 `useSellerInvoiceRemedy.ts`（**不含 `!isSuspended`**），
 * 不在 `useSellerInvoices.ts`。兩類分檔就是為了讓這個差別在檔案層級看得見。
 *
 * 🔴 `?tab=` 的值是**視圖名不是 DB 狀態值**（`AWAITING_SELLER` 等三個），
 * 兩套字串不可互相代入——送 DB 值給端點 34 會拿到 400。轉換與窮盡性都在
 * `../invoiceComplaintDisplay.ts`，並有 unit test 守著。
 *
 * 🔴 `sort` 固定 `due_at_asc`，**不提供排序 UI、不做任何 client-side sort**：
 * 這一頁的唯一目的就是「先處理快逾期的」，給一個能把它關掉的開關等於讓賣家自己
 * 製造違規。
 *
 * 🔴 工作天倒數**前端零計算**（§0.6 C10）：值全取後端的 `remainingBusinessDays`／
 * `overdueBusinessDays`，空字串顯示「處理期限計算中」而不是退化成日曆天。
 * 元件沿用買家批已建好的 `features/invoices/components/SlaCountdown.tsx`——同一個
 * repo 內再寫第二份就是製造兩套分級規則。
 */

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Link from "@/components/shared/Link";
import Pagination from "@/components/shared/Pagination";
import { ScFilters, ScPill } from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { SlaCountdown } from "@/features/invoices/components/SlaCountdown";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useSellerInvoiceComplaints } from "@/lib/hooks/useSellerInvoiceRemedy";
import { usePathname } from "@/lib/intl/navigation";
import type { SellerInvoiceComplaint } from "@/lib/invoices/seller-types";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { SellerEmptyState } from "../../components/SellerEmptyState";
import {
	invoiceComplaintDisplay,
	normalizeSellerComplaintView,
	SELLER_COMPLAINT_VIEWS,
} from "../invoiceComplaintDisplay";

const OVERVIEW_HREF = "/seller-center/invoices";
const PAGE_SIZE = 20;

const EMPTY_COPY: Record<string, { title: string; description: string }> = {
	AWAITING_SELLER: {
		title: "目前沒有待處理的發票申訴",
		description: "買家收不到發票或收據時會在這裡建立申訴。",
	},
	SELLER_RESPONDED: {
		title: "目前沒有已回覆的申訴",
		description: "你回覆過的申訴會留在這裡，直到平台或買家結案。",
	},
	CLOSED: {
		title: "目前沒有已結案的申訴",
		description: "結案的申訴會留在這裡供查閱。",
	},
};

export function SellerInvoiceAppealsPage() {
	// 走 observed router 而不是裸 useRouter：`tests/navigation-route-contract.test.ts`
	// 要求所有程式化導航都經過這一層。
	const router = useObservedRouter("seller_invoice_appeals");
	const pathname = usePathname() ?? "/seller-center/invoices/appeals";
	const searchParams = useSearchParams();

	const view = normalizeSellerComplaintView(searchParams?.get("tab") ?? "");
	const page = Math.max(1, Number(searchParams?.get("page") ?? "1") || 1);

	const setParams = React.useCallback(
		(patch: Record<string, string | null>) => {
			const next = new URLSearchParams(searchParams?.toString() ?? "");
			for (const [key, value] of Object.entries(patch)) {
				if (value === null || value === "") next.delete(key);
				else next.set(key, value);
			}
			// 切分頁時 `page` 一律重設為 1，否則會停在一個不存在的頁碼。
			if (!("page" in patch)) next.delete("page");
			const qs = next.toString();
			router.replace(qs ? `${pathname}?${qs}` : pathname);
		},
		[pathname, router, searchParams],
	);

	const queryParams = React.useMemo(
		() => ({ view, page, pageSize: PAGE_SIZE }),
		[view, page],
	);

	const {
		page: result,
		loading,
		error,
		refresh,
	} = useSellerInvoiceComplaints(queryParams);

	const records = result?.records ?? [];
	// 🔴 `total` 是總筆數不是頁數（§B-2.0 T25）。
	const totalPages = result
		? Math.max(1, Math.ceil(result.total / PAGE_SIZE))
		: 1;
	const empty = EMPTY_COPY[view] ?? EMPTY_COPY.AWAITING_SELLER;

	return (
		<SellerPageShell
			title="買家發票申訴"
			eyebrow="發票與稅籍"
			subtitle="買家表示沒有收到發票或收據時會建立申訴，請在 3 個工作天內回覆。"
			actions={
				<Button size="lg" variant="bordered" onClick={() => void refresh()}>
					重新整理
				</Button>
			}
			toolbar={
				<ScFilters aria-label="申訴狀態">
					{SELLER_COMPLAINT_VIEWS.map((item) => (
						<ScPill
							key={item.value}
							as="button"
							active={view === item.value}
							aria-selected={view === item.value}
							onClick={() => setParams({ tab: item.value })}
						>
							{item.label}
						</ScPill>
					))}
				</ScFilters>
			}
		>
			<Link href={OVERVIEW_HREF} className="text-sm underline">
				返回發票與稅籍
			</Link>

			{loading ? (
				<ComplaintListSkeleton />
			) : error ? (
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						申訴列表載入失敗，請稍後再試。
					</p>
					<Button size="lg" variant="bordered" onClick={() => void refresh()}>
						重試
					</Button>
				</section>
			) : records.length === 0 ? (
				<SellerEmptyState
					variant="invoices"
					title={empty.title}
					description={empty.description}
				/>
			) : (
				<>
					<ul className="seller-mobile-disclosure-desktop m-0 flex list-none flex-col gap-[var(--hkui-space-3)] p-0">
						{records.map((item) => (
							<ComplaintCard key={item.complaintId} complaint={item} />
						))}
					</ul>
					<SellerMobileDisclosureList
						items={records}
						getKey={complaintKey}
						resetKey={`${view}:${page}`}
						ariaLabel="手機版買家發票申訴"
						getSummary={(complaint) => {
							const display = invoiceComplaintDisplay(complaint.status);
							return {
								label: `發票申訴 ${complaint.orderNo || complaint.complaintId}`,
								leading: <SellerMobileDisclosureIcon name="shield-fail" />,
								title: complaint.orderNo || "未提供訂單編號",
								meta: formatTaipeiDateTime(complaint.createdAt) || "—",
								status: display.label,
							};
						}}
						renderDetails={(complaint) => (
							<ComplaintMobileDetails complaint={complaint} />
						)}
					/>
					<Pagination
						current={page}
						total={totalPages}
						onPageChange={(next) => setParams({ page: String(next) })}
					/>
				</>
			)}
		</SellerPageShell>
	);
}

function complaintKey(complaint: SellerInvoiceComplaint) {
	return complaint.complaintId;
}

function ComplaintListSkeleton() {
	return (
		<div
			role="status"
			aria-busy="true"
			aria-label="載入中"
			className="flex flex-col gap-[var(--hkui-space-3)]"
		>
			{["a", "b", "c"].map((key) => (
				<section
					key={key}
					className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]"
				>
					<span className="block h-3 w-1/3 animate-pulse rounded bg-[var(--hkui-default-200)] motion-reduce:animate-none motion-reduce:opacity-70" />
					<span className="block h-3 w-1/2 animate-pulse rounded bg-[var(--hkui-default-200)] motion-reduce:animate-none motion-reduce:opacity-70" />
				</section>
			))}
			<span className="visually-hidden">資料載入中</span>
		</div>
	);
}

/**
 * 一件申訴。
 *
 * **不做整卡可點**：卡內有其他互動（訂單連結、CTA），整卡可點會吃掉它們的點擊。
 *
 * ⚠️ 「買家」與「買家要的是」兩列**整列不渲染**：`buyerMaskedName` 與
 * `expectedDocKind` 後端目前恆為空字串（見 `lib/invoices/seller-types.ts` 的
 * `SellerInvoiceComplaint` 檔頭）。渲染空字串或頂替值，都會讓賣家依一個假的對象
 * 去回覆買家（§B-2.0 T21 的同一條規則）。
 */
function ComplaintCard({ complaint }: { complaint: SellerInvoiceComplaint }) {
	const display = invoiceComplaintDisplay(complaint.status);
	// `reopenCount` 是**屬性不是狀態**：不為它做第四個分頁，也不拿它當 status 判斷。
	const reopened =
		complaint.reopenCount !== "" && Number(complaint.reopenCount) > 0;

	return (
		<li className="seller-card flex flex-col gap-[var(--hkui-space-3)] p-[var(--hkui-space-4)]">
			<div className="flex flex-wrap items-start justify-between gap-2">
				<div className="flex min-w-0 flex-col gap-1">
					<span className="flex flex-wrap items-center gap-2">
						{complaint.orderNo ? (
							<Link
								href={`/seller-center/orders/${encodeURIComponent(complaint.orderNo)}`}
								className="underline"
							>
								<code>{complaint.orderNo}</code>
							</Link>
						) : (
							<span>—</span>
						)}
						{reopened ? (
							<Chip variant="flat" color="warning" size="sm">
								{`已重開 ${complaint.reopenCount} 次`}
							</Chip>
						) : null}
					</span>
					<small className="text-[var(--hkui-foreground-soft)]">
						{`申訴時間 ${formatTaipeiDateTime(complaint.createdAt) || "—"}`}
					</small>
					{complaint.buyerMaskedName ? (
						<small className="text-[var(--hkui-foreground-soft)]">
							{`買家 ${complaint.buyerMaskedName}`}
						</small>
					) : null}
				</div>
				<div className="flex flex-col items-end gap-2">
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
					<SlaCountdown
						remainingBusinessDays={complaint.remainingBusinessDays}
						overdueBusinessDays={complaint.overdueBusinessDays}
					/>
				</div>
			</div>

			{complaint.description ? (
				<p className="m-0 line-clamp-2 text-sm text-[var(--hkui-foreground-soft)]">
					{complaint.description}
				</p>
			) : null}

			<div className="flex justify-end">
				<Link
					href={`/seller-center/invoices/appeals/${encodeURIComponent(complaint.complaintId)}`}
					className="text-sm underline"
				>
					查看並處理
				</Link>
			</div>
		</li>
	);
}

function ComplaintMobileDetails({
	complaint,
}: {
	complaint: SellerInvoiceComplaint;
}) {
	const display = invoiceComplaintDisplay(complaint.status);
	const reopened =
		complaint.reopenCount !== "" && Number(complaint.reopenCount) > 0;

	return (
		<>
			<div className="seller-mobile-disclosure__body">
				<div className="seller-mobile-disclosure__detail-row">
					<span>申訴時間</span>
					<span>{formatTaipeiDateTime(complaint.createdAt) || "—"}</span>
				</div>
				{complaint.buyerMaskedName ? (
					<div className="seller-mobile-disclosure__detail-row">
						<span>買家</span>
						<strong>{complaint.buyerMaskedName}</strong>
					</div>
				) : null}
				<div className="seller-mobile-disclosure__facts">
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
					{reopened ? (
						<Chip variant="flat" color="warning" size="sm">
							{`已重開 ${complaint.reopenCount} 次`}
						</Chip>
					) : null}
					<SlaCountdown
						remainingBusinessDays={complaint.remainingBusinessDays}
						overdueBusinessDays={complaint.overdueBusinessDays}
					/>
				</div>
				{complaint.description ? (
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						{complaint.description}
					</p>
				) : null}
			</div>
			<div className="seller-mobile-disclosure__actions">
				{complaint.orderNo ? (
					<Link
						href={`/seller-center/orders/${encodeURIComponent(complaint.orderNo)}`}
						className="text-sm underline"
					>
						查看訂單
					</Link>
				) : null}
				<Link
					href={`/seller-center/invoices/appeals/${encodeURIComponent(complaint.complaintId)}`}
					className="text-sm underline"
				>
					查看並處理
				</Link>
			</div>
		</>
	);
}

export default SellerInvoiceAppealsPage;
