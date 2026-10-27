import type React from "react";
import "./StateRow.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";

export interface StateRowProps {
	/** useQuery 的 isLoading。 */
	loading: boolean;
	/**
	 * useQuery 的 error（沒有錯誤時傳 null）。
	 *
	 * 這個欄位是本元件存在的理由。ADM 既有的佇列頁（`anomalies` / `disputes`）
	 * 只解構 `data, isLoading`，而它們的 `queryFn` 在 `code !== 0` 時 return
	 * 空清單 ⇒ **錯誤被吞成「暫無資料」**：畫面完全正常，管理員以為今天沒有案件。
	 * 發票佇列不得複製那個缺口，所以 error 是必填參數而不是選填。
	 */
	error: unknown;
	/** 資料筆數為 0。 */
	empty: boolean;
	/**
	 * 空集合時要說的話。必填，因為「一筆都還沒有」與「篩選後沒有結果」是兩件事，
	 * 共用一句會讓管理員以為佇列是空的而不是自己把條件設窄了。
	 */
	emptyText: string;
	/** 空狀態下方的額外節點（典型用途：「清除篩選」鍵）。 */
	emptyAction?: React.ReactNode;
	colSpan: number;
	/** refetch()。 */
	onRetry: () => void;
	children: React.ReactNode;
}

/**
 * 表格 tbody 的三態（載入中／錯誤／空）包裝；三態皆不成立時原樣渲染 children。
 *
 * ADM 全站零 skeleton 元件，載入態一律是一列置中文字（`admin/page.tsx` 的 `--`
 * 同源）。錯誤態帶業務碼與追蹤編號（`governanceErrorText`），因為 ADM 的錯誤
 * 多半要拿著 traceId 去查 log，只說「載入失敗」等於要人重跑一次。
 */
export default function StateRow({
	loading,
	error,
	empty,
	emptyText,
	emptyAction,
	colSpan,
	onRetry,
	children,
}: StateRowProps) {
	if (loading) {
		return (
			<tr>
				<td colSpan={colSpan} className="adm-state-row">
					載入中...
				</td>
			</tr>
		);
	}
	if (error) {
		return (
			<tr>
				<td colSpan={colSpan} className="adm-state-row adm-state-row--error">
					<div role="alert">
						{governanceErrorText(error, "載入失敗，請稍後重試。")}
					</div>
					<button
						type="button"
						className="btn adm-state-row__retry"
						onClick={onRetry}
					>
						<MaterialSymbol name="refresh" />
						重試
					</button>
				</td>
			</tr>
		);
	}
	if (empty) {
		return (
			<tr>
				<td colSpan={colSpan} className="adm-state-row">
					<div>{emptyText}</div>
					{emptyAction}
				</td>
			</tr>
		);
	}
	return <>{children}</>;
}
