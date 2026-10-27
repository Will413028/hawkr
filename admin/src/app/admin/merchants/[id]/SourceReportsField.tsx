"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useInfiniteQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useMemo } from "react";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { policyCategoryText } from "@/lib/policy-category";
import { SOURCE_REPORTS_MAX } from "./violation-measure";

interface SourceReportsFieldProps {
	/** 只列這家店的通報；來源通報必須是同一家店（伺服器也會檢查）。 */
	storeId: string;
	selected: readonly string[];
	disabled: boolean;
	onChange: (ids: string[]) => void;
}

const NOTE_ID = "source-reports-note";

/**
 * 「來源通報」複選：列出這家店可以當來源的通報，勾選成立新認定的那幾筆（最多 SOURCE_REPORTS_MAX 筆，選填）。
 * 能不能當來源由伺服器推導（`can_cite_as_source`：結果不是未違規、內容不足或濫報，而且店家沒有關閉；建立措施時伺服器用同一個規則檢查），
 * client 不自己看結果。預先勾選的通報（從通報詳情開啟）即使不在已載入的清單裡，也照樣保留在選取中。
 * 濾掉不能當來源的通報是 client 端做的，已載入的通報可能全被濾掉、但還有下一頁：「載入更多」只看有沒有下一頁，
 * 空狀態要沒有下一頁才說「沒有」。
 */
export default function SourceReportsField({
	storeId,
	selected,
	disabled,
	onChange,
}: SourceReportsFieldProps) {
	const query = useInfiniteQuery(
		SellerGovernanceAdminService.method.listReports,
		{ storeId, pageSize: 100, pageToken: "" },
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const reports = useMemo(
		() =>
			(query.data?.pages.flatMap((page) => page.reports) ?? []).filter(
				(report) => report.canCiteAsSource,
			),
		[query.data],
	);

	const toggle = (id: string, checked: boolean) => {
		onChange(
			checked
				? [...selected, id]
				: selected.filter((current) => current !== id),
		);
	};

	return (
		<fieldset
			style={{ marginTop: 16, border: 0, padding: 0 }}
			aria-describedby={NOTE_ID}
		>
			<legend className="form-label">來源通報</legend>
			{query.isLoading ? (
				<p className="adm-operation-field-note">載入通報中...</p>
			) : query.isError ? (
				<div>
					<p className="adm-operation-error" role="alert">
						{connectErrorText(query.error, "通報讀取失敗")}
					</p>
					<Button variant="outline" onClick={() => void query.refetch()}>
						重試
					</Button>
				</div>
			) : (
				<>
					{reports.length === 0 && !query.hasNextPage && (
						<p className="adm-operation-field-note">
							這家店沒有可以當來源的通報。
						</p>
					)}
					{reports.map((report) => (
						<label
							key={report.id}
							style={{
								display: "flex",
								gap: 8,
								alignItems: "center",
								minHeight: 44,
							}}
						>
							<input
								type="checkbox"
								checked={selected.includes(report.id)}
								disabled={disabled}
								onChange={(e) => toggle(report.id, e.target.checked)}
							/>
							<span>
								通報 {report.id}・{policyCategoryText(report.category)}
								{report.createTime
									? `・建立 ${formatDateTimeShort(timestampDate(report.createTime))}`
									: ""}
							</span>
						</label>
					))}
					{query.hasNextPage && (
						<Button
							variant="outline"
							onClick={() => void query.fetchNextPage()}
							disabled={query.isFetchingNextPage}
						>
							{query.isFetchingNextPage ? "載入中…" : "載入更多"}
						</Button>
					)}
				</>
			)}
			<p className="adm-operation-field-note" id={NOTE_ID}>
				{`選填；最多 ${SOURCE_REPORTS_MAX} 筆，只列這家店可以當來源的通報（結果不是未違規、內容不足或濫報）。`}
			</p>
		</fieldset>
	);
}
