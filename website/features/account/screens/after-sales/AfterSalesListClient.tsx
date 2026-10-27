"use client";

import * as React from "react";
import {
	ScEmpty,
	ScEmptyDesc,
	ScEmptyTitle,
	ScFilters,
	ScPillCount,
} from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { AfterSaleCaseCard } from "@/features/after-sales/components/AfterSaleCaseCard";
import { afterSaleGroupOf } from "@/lib/after-sales/status";
import type { AfterSaleGroup } from "@/lib/after-sales/types";
import { useMyAfterSales } from "@/lib/hooks/useMyAfterSales";

const PILL_BASE =
	"inline-flex items-center gap-1.5 h-8 px-[14px] border rounded-[var(--radius-pill)] text-[0.8125rem] font-bold no-underline cursor-pointer transition-[background,color,border-color] duration-[160ms] ease-out";
const PILL_INACTIVE =
	"border-divider bg-content1 text-foreground-soft hover:bg-content2 hover:text-foreground";
const PILL_ACTIVE =
	"border-transparent bg-hkui-primary text-hk-cream hover:bg-hkui-primary hover:text-hk-cream";

const GROUP_TABS: Array<{ value: AfterSaleGroup; label: string }> = [
	{ value: "all", label: "全部" },
	{ value: "pending", label: "待處理" },
	{ value: "returning", label: "退貨中" },
	{ value: "refunding", label: "退款中" },
	{ value: "closed", label: "已結案" },
];

export default function AfterSalesListClient() {
	const { records, loading, error } = useMyAfterSales();
	const [group, setGroup] = React.useState<AfterSaleGroup>("all");

	const counts = React.useMemo(() => {
		const c: Record<string, number> = {};
		for (const r of records) {
			const g = afterSaleGroupOf(r);
			c[g] = (c[g] ?? 0) + 1;
		}
		return c;
	}, [records]);

	const filtered = React.useMemo(
		() =>
			group === "all"
				? records
				: records.filter((r) => afterSaleGroupOf(r) === group),
		[records, group],
	);

	return (
		<main className="orders-page">
			<SellerPageShell
				title="售後案件"
				subtitle={`共 ${records.length} 筆售後申請。點案件可查看退貨、退款進度。`}
				toolbar={
					<ScFilters aria-label="售後狀態篩選">
						{GROUP_TABS.map((t) => {
							const active = group === t.value;
							const count =
								t.value === "all" ? records.length : counts[t.value];
							return (
								<button
									type="button"
									key={t.value}
									aria-selected={active}
									className={[PILL_BASE, active ? PILL_ACTIVE : PILL_INACTIVE]
										.filter(Boolean)
										.join(" ")}
									onClick={() => setGroup(t.value)}
									role="tab"
								>
									{t.label}
									{typeof count === "number" && count > 0 ? (
										<ScPillCount active={active}>{count}</ScPillCount>
									) : null}
								</button>
							);
						})}
					</ScFilters>
				}
			>
				{error && (
					<p className="text-danger-700 text-sm font-semibold" role="alert">
						{error}
					</p>
				)}

				{loading && records.length === 0 ? (
					<p className="text-foreground-soft text-sm">載入中…</p>
				) : filtered.length === 0 ? (
					<EmptyAfterSales hasAny={records.length > 0} />
				) : (
					<ul className="list-none m-0 p-0 flex flex-col gap-3">
						{filtered.map((c) => (
							<li key={c.caseNo}>
								<AfterSaleCaseCard
									aftersale={c}
									href={`/account/after-sales/${c.caseNo}`}
								/>
							</li>
						))}
					</ul>
				)}
			</SellerPageShell>
		</main>
	);
}

function EmptyAfterSales({ hasAny }: { hasAny: boolean }) {
	return (
		<ScEmpty>
			<AccountEmptyStateIcon variant="orders" />
			<ScEmptyTitle>
				{hasAny ? "此分類沒有案件" : "目前沒有售後案件"}
			</ScEmptyTitle>
			<ScEmptyDesc>若收到的商品有問題，可到訂單詳情頁申請退換貨。</ScEmptyDesc>
			<Button href="/account/orders" size="lg" radius="full">
				查看我的訂單
			</Button>
		</ScEmpty>
	);
}
