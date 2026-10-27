"use client";

import { useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StatusBadge } from "@/components/UI";
import { FULFILLMENT_MODE_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import InvoiceTimeline from "@/features/invoices/components/InvoiceTimeline";
import type {
	InvoiceEnvelope,
	InvoicePagedData,
	ProviderCutoverItem,
} from "@/features/invoices/types";
import { codeLabel, INVOICE_FULFILLMENT_MODE_LABEL } from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A09｜AMEGO 啟用切點與版本歷程（58A §A09，端點 46
// `GET /api/v1/admin/invoices/provider_accounts/{store_id}/cutovers`）。
//
// ## 完全唯讀，而且「不畫」比「畫成 disabled」重要
//
// 這一頁一顆會發出寫入請求的按鈕都沒有，也**完全不提供「批次重分類」入口**——不是
// 做成 disabled 的按鈕，是根本不畫。disabled 會讓人去問「怎樣才能啟用它」，而正確
// 答案是「永遠不行」：已付款訂單的開票方式在付款當下就寫死了，不隨賣家後續狀態回溯
// 改寫（`InvoiceFulfillmentMode` 的碼表描述逐字）。修改切點的入口在 A04。
//
// 本頁也**完全不碰 `pendingOrderCount`**：那是 A04 停用流程的數字，出現在這裡會被
// 讀成「這些訂單會被重新分類」。
//
// ## 排序：後端給什麼就照什麼順序畫
//
// 58A 寫「依 `cutoverAt` 遞減」，而後端的 SQL 是 `ORDER BY created_at DESC, id DESC`
// （`services/invoice-service/db/queries/invoice_provider_cutovers.sql`）。
// 前端**不重排**：端點是分頁的，前端排序只排得到當前頁，做出來的是一個看起來像
// 全域排序、其實不是的東西（A04 列表頁「不做表頭排序」記的是同一件事）。
// 兩個時間欄都印出來，差異在畫面上看得見。

const PROVIDER_ACCOUNTS_ENDPOINT = "/api/v1/admin/invoices/provider_accounts";
const PAGE_SIZE = 20;

/**
 * 適用範圍逐字文案。
 *
 * 🔴 **是「付款時間」不是「下單時間」**：切點的判定基準是 `automation_cutover_at`
 * 與訂單**付款時點**的比較，寫成「下單」會讓 ADM 在爭議時算錯範圍——而這一頁的用途
 * 正是爭議時指給人看。
 */
function scopeSentence(cutoverAt: string, deactivatedAt: string): string {
	const from = formatDateTime(cutoverAt);
	const to = deactivatedAt ? formatDateTime(deactivatedAt) : "現在";
	return `本版本適用於付款時間在 ${from} 之後、${to} 之前的訂單。較早付款的訂單維持它們付款當下的開票方式。`;
}

export default function ProviderCutoversPage() {
	const router = useRouter();
	const params = useParams<{ storeId: string }>();
	const storeId = String(params?.storeId ?? "");
	const [page, setPage] = useState(1);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "cutovers", storeId, page],
		enabled: storeId !== "",
		queryFn: async () => {
			const search = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<ProviderCutoverItem>>
			>(
				`${PROVIDER_ACCOUNTS_ENDPOINT}/${encodeURIComponent(storeId)}/cutovers?${search}`,
			);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	// 9330（404）＝ 這個賣家根本沒設過 AMEGO 連線，與「設過但還沒設切點」是兩件事
	// （後端 `ListProviderCutoversForAdmin` 的註解逐字）：前者要去看 A04 的串接狀態，
	// 後者是空清單。回一句共用的「查無資料」會把前者顯示成後者。
	const notFound =
		isError && error instanceof ApiClientError && error.code === 9330;

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const back = () =>
		router.push(
			`/admin/invoices/provider-accounts/${encodeURIComponent(storeId)}`,
		);

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回 AMEGO 串接詳情"
						onClick={back}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">AMEGO 啟用切點與版本歷程</h1>
						<div className="invoice-meta">
							<span>store_id：{storeId}</span>
							<span>共 {total} 個版本</span>
						</div>
					</div>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						size="sm"
						onClick={() => refetch()}
						loading={isFetching}
					>
						手動更新
					</Button>
				</div>
			</div>

			<div className="invoice-scope-notice">
				<MaterialSymbol name="info" />
				<div>
					<p className="invoice-scope-notice__title">這一頁只能檢視</p>
					<p className="invoice-scope-notice__text">
						每一個版本的適用範圍以<strong>付款時間</strong>
						判定，不是下單時間。已付款的訂單不會因為切點變更而改變開票方式，也沒有任何介面可以把它們重新分類；要調整之後的開票方式請回上一頁的
						AMEGO 串接詳情。
					</p>
				</div>
			</div>

			{notFound ? (
				<section className="invoice-section">
					<p className="invoice-empty-hint">這個賣家尚未建立 AMEGO 串接。</p>
					<Button variant="primary" onClick={back}>
						返回串接詳情
					</Button>
				</section>
			) : isLoading ? (
				<section className="invoice-section">
					<p className="invoice-empty-hint">載入中...</p>
				</section>
			) : isError ? (
				<section className="invoice-section">
					<p className="invoice-empty-hint" role="alert">
						{governanceErrorText(error, "載入失敗，請稍後重試。")}
					</p>
					<Button variant="outline" onClick={() => refetch()}>
						重試
					</Button>
				</section>
			) : (
				<>
					<section className="invoice-section">
						<h2 className="invoice-section__title">版本歷程</h2>
						<p className="invoice-section__hint">
							資料為載入當下的快照，請按「手動更新」重新取得。
						</p>
						<InvoiceTimeline
							emptyText="這個賣家還沒有任何啟用切點紀錄。"
							entries={records.map((item, index) => ({
								id: `${item.version || index}-${item.createdAt || index}`,
								time: formatDateTime(item.cutoverAt),
								title: (
									<>
										<span className="invoice-cutover-version">
											v{item.version || "?"}
										</span>{" "}
										<StatusBadge
											variant={
												FULFILLMENT_MODE_BADGE_VARIANT[item.mode] ?? "gray"
											}
										>
											{codeLabel(INVOICE_FULFILLMENT_MODE_LABEL, item.mode)}
										</StatusBadge>{" "}
										{item.deactivatedAt ? (
											<span className="invoice-cutover-state">
												停用於 {formatDateTime(item.deactivatedAt)}
											</span>
										) : (
											<span className="invoice-cutover-state invoice-cutover-state--live">
												仍生效中
											</span>
										)}
									</>
								),
								actor: (
									<>
										建立於 {formatDateTime(item.createdAt)} 由{" "}
										{item.createdByAccountId
											? `管理員（${item.createdByAccountId}）`
											: "未記錄操作者"}
										{/*
										 * ADM 沒有 displayName 查詢端點 ⇒ 照代行系列
										 * `sessionRevokedByText` 的既有做法印數字 id。
										 * email 會換、id 不會，所以 id 才是可追溯的那一個。
										 */}
									</>
								),
								body: (
									<>
										<p className="invoice-cutover-scope">
											{scopeSentence(item.cutoverAt, item.deactivatedAt)}
										</p>
										{item.reason ? <p>建立理由：{item.reason}</p> : null}
									</>
								),
							}))}
						/>
					</section>

					<InvoicePagination
						page={page}
						total={total}
						pageSize={PAGE_SIZE}
						onPage={setPage}
					/>
				</>
			)}
		</div>
	);
}
