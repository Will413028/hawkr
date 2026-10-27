"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/UI";
import {
	CERTIFICATION_TYPE_LABEL,
	codeLabel,
	PRODUCT_CERTIFICATION_SOURCE_LABEL,
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	ProductCertificationStatus,
	toOptions,
} from "@/lib/codemeta";
import {
	type CertificationApplicationWire,
	formatDateTime,
	isProductGone,
	productGoneLabel,
	toUtcDayEnd,
	toUtcDayStart,
} from "@/lib/product-certifications";
import { listCertificationApplications } from "@/services/product-certifications";
import { CertificationStatusPill, GovVerifyStatePill } from "./badges";
import "@/styles/pages/ProductCertifications.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 商品農產品認證審核佇列（backend-payuni 19 號設計書 §9.2）。
 *
 * ## 為什麼裁決按鈕不在這一頁
 *
 * 佇列只負責「找到那一筆」，核准／駁回一律要進詳情頁。19 §9.2 把 `govVerifyPayload`
 * （政府查驗原始結果）與 `currentApproved`（核准後會被取代掉的現行標章）列為 ADM 的
 * 判斷依據，而那兩塊在列表上放不下也讀不完。在列表直接按核准，等於在沒有依據的情況
 * 下硬核准——特別是 3703 CAS：查驗通過只代表**該工廠**有 CAS 資格，不代表這個商品是
 * CAS 產品（19 §8.5 實測一個編號對到 173 個產品）。
 *
 * 這是與 `/admin/reports` 的刻意差異：檢舉的裁決依據就在列表那幾欄裡，認證不是。
 */

const PAGE_SIZE = 20;

/** 送出的篩選條件（唯讀快照）；輸入框改動不會直接進 query key，要按「查詢」才生效。 */
interface AppliedFilters {
	status: string;
	certificationType: string;
	storeId: string;
	keyword: string;
	submittedFrom: string;
	submittedTo: string;
}

const EMPTY_FILTERS: AppliedFilters = {
	// 預設看待審的那些——佇列存在的理由就是「還沒有人裁決的申請」。
	status: ProductCertificationStatus.SUBMITTED,
	certificationType: "",
	storeId: "",
	keyword: "",
	submittedFrom: "",
	submittedTo: "",
};

export default function ProductCertificationQueuePage() {
	const router = useRouter();
	const [page, setPage] = useState(1);
	const [draft, setDraft] = useState<AppliedFilters>(EMPTY_FILTERS);
	const [applied, setApplied] = useState<AppliedFilters>(EMPTY_FILTERS);

	const { data, isLoading, isError, error, isFetching, refetch } = useQuery({
		queryKey: ["product-certifications", "queue", { page, ...applied }],
		queryFn: () =>
			listCertificationApplications({
				status: applied.status,
				certificationType: applied.certificationType,
				storeId: applied.storeId,
				keyword: applied.keyword,
				// 管理員挑的是本地時區的那一天，轉成 gateway 要的 ISO 8601 UTC 瞬間。
				submittedFrom: toUtcDayStart(applied.submittedFrom),
				submittedTo: toUtcDayEnd(applied.submittedTo),
				page,
				pageSize: PAGE_SIZE,
			}),
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const totalPages = total > 0 ? Math.ceil(total / PAGE_SIZE) : 0;

	const setField = <K extends keyof AppliedFilters>(
		key: K,
		value: AppliedFilters[K],
	) => setDraft((prev) => ({ ...prev, [key]: value }));

	const submitFilters = () => {
		setPage(1);
		setApplied(draft);
	};

	const resetFilters = () => {
		setPage(1);
		setDraft(EMPTY_FILTERS);
		setApplied(EMPTY_FILTERS);
	};

	const openDetail = (row: CertificationApplicationWire) => {
		router.push(`/admin/product-certifications/${row.id}`);
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">商品認證審核</h1>
					<p className="page-subtitle">
						賣家送出的農產品認證申請；只有核准的項目會出現在商品頁的標章上
					</p>
				</div>
			</div>

			<form
				className="cert-filters"
				onSubmit={(e) => {
					e.preventDefault();
					submitFilters();
				}}
			>
				<label className="cert-filter-field">
					<span>審核狀態</span>
					<select
						value={draft.status}
						onChange={(e) => setField("status", e.target.value)}
					>
						<option value="">全部狀態</option>
						{toOptions(PRODUCT_CERTIFICATION_STATUS_LABEL).map(([k, v]) => (
							<option key={k} value={k}>
								{v}
							</option>
						))}
					</select>
				</label>

				<label className="cert-filter-field">
					<span>認證類型</span>
					<select
						value={draft.certificationType}
						onChange={(e) => setField("certificationType", e.target.value)}
					>
						<option value="">全部類型</option>
						{toOptions(CERTIFICATION_TYPE_LABEL).map(([k, v]) => (
							<option key={k} value={k}>
								{v}
							</option>
						))}
					</select>
				</label>

				<label className="cert-filter-field">
					<span>店家 store_id</span>
					<input
						type="text"
						inputMode="numeric"
						placeholder="例：2685914682"
						value={draft.storeId}
						onChange={(e) => setField("storeId", e.target.value)}
					/>
				</label>

				<label className="cert-filter-field">
					<span>關鍵字（商品名 / 證號）</span>
					<input
						type="text"
						placeholder="例：有機小松菜 或 1-009-300022"
						value={draft.keyword}
						onChange={(e) => setField("keyword", e.target.value)}
					/>
				</label>

				<label className="cert-filter-field">
					<span>送件起日</span>
					<input
						type="date"
						value={draft.submittedFrom}
						onChange={(e) => setField("submittedFrom", e.target.value)}
					/>
				</label>

				<label className="cert-filter-field">
					<span>送件迄日</span>
					<input
						type="date"
						value={draft.submittedTo}
						onChange={(e) => setField("submittedTo", e.target.value)}
					/>
				</label>

				<div className="cert-filter-actions">
					<Button type="submit" variant="primary" disabled={isFetching}>
						{isFetching ? "查詢中…" : "查詢"}
					</Button>
					<Button type="button" variant="outline" onClick={resetFilters}>
						重設
					</Button>
				</div>
			</form>

			{isError && (
				<div className="cert-note cert-note--danger" role="alert">
					<MaterialSymbol name="error" />
					<div>
						讀取認證申請失敗：
						{error instanceof Error ? error.message : "請稍後再試"}
						<div style={{ marginTop: 8 }}>
							<Button size="sm" variant="outline" onClick={() => refetch()}>
								重新載入
							</Button>
						</div>
					</div>
				</div>
			)}

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>送件時間</th>
							<th>商品</th>
							<th>店家</th>
							<th>認證類型</th>
							<th>證書編號</th>
							<th>審核狀態</th>
							<th>政府查驗</th>
							<th>來源</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{isLoading ? (
							<tr>
								<td colSpan={9} style={{ textAlign: "center", padding: 40 }}>
									載入中…
								</td>
							</tr>
						) : records.length === 0 ? (
							<tr>
								<td colSpan={9} style={{ textAlign: "center", padding: 40 }}>
									{isError ? "無法載入資料" : "目前沒有符合條件的認證申請"}
								</td>
							</tr>
						) : (
							records.map((row) => (
								<tr key={row.id}>
									<td>{formatDateTime(row.submittedAt)}</td>
									<td>
										{row.productName || "-"}
										{isProductGone(row.productPublicationState) ? (
											<span className="cert-cell-warn">
												（{productGoneLabel(row.productPublicationState)}）
											</span>
										) : null}
										<span className="cert-cell-sub">#{row.productId}</span>
									</td>
									<td>{row.storeId}</td>
									<td>
										{codeLabel(CERTIFICATION_TYPE_LABEL, row.certificationType)}
									</td>
									<td>{row.certificateNo}</td>
									<td>
										<CertificationStatusPill status={row.status} />
										{row.requiresManualReview && (
											<span className="cert-cell-sub">須人工審核</span>
										)}
									</td>
									<td>
										<GovVerifyStatePill state={row.govVerifyState} />
									</td>
									<td>
										{codeLabel(PRODUCT_CERTIFICATION_SOURCE_LABEL, row.source)}
									</td>
									<td>
										{/* 進詳情的唯一入口是這顆按鈕，不是整列 onClick：可點擊的 <tr>
										    在鍵盤與輔助科技下不可達（沒有 role、拿不到焦點），
										    而這一頁通往的是核准／駁回這種不可逆的裁決。 */}
										<Button
											size="sm"
											variant="outline"
											onClick={() => openDetail(row)}
										>
											查看
										</Button>
									</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			<div className="cert-pagination">
				<span className="page-info">
					共 {total.toLocaleString()} 筆
					{totalPages > 0 ? ` · 第 ${page} / ${totalPages} 頁` : ""}
				</span>
				<Button
					size="sm"
					variant="outline"
					disabled={page === 1 || isFetching}
					onClick={() => setPage((p) => Math.max(1, p - 1))}
				>
					上一頁
				</Button>
				<Button
					size="sm"
					variant="outline"
					disabled={isFetching || page >= totalPages}
					onClick={() => setPage((p) => p + 1)}
				>
					下一頁
				</Button>
			</div>
		</div>
	);
}
