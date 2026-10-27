"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useInfiniteQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { ViolationStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { skipToken } from "@tanstack/react-query";
import { useMemo } from "react";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";

/**
 * 認定屬於哪個帳號：已知，或還在由店家詳情讀取（建立措施只有店家 ID）。讀取中或失敗時欄位照樣出現，
 * 顯示載入中或可重試的失敗，不是整欄消失。
 */
export type CitedFindingsAccount =
	| { status: "ready"; accountId: string }
	| { status: "loading" }
	| { status: "failed"; error: unknown; retry: () => void };

interface CitedFindingsFieldProps {
	/** 帳號的認定（違規）；只列生效中的（引用已撤銷的認定伺服器會拒絕）。 */
	account: CitedFindingsAccount;
	selected: readonly string[];
	disabled: boolean;
	/** 必選（欄位標題帶 *）：終止合作的重大違約必引用；建立措施的引用是選填。 */
	required: boolean;
	/** 帳號沒有生效中的認定時的說明，依用途而不同。 */
	emptyText: string;
	/** 欄位底下的條件說明，依用途而不同（引用條件由伺服器檢查）。 */
	noteText: string;
	onChange: (ids: string[]) => void;
}

const NOTE_ID = "cited-findings-note";
const READ_FAILED_TEXT = "認定讀取失敗";

/**
 * 「依據的認定」複選：列出帳號生效中的認定，勾選要引用的。引用條件由伺服器檢查，這裡顯示每一筆的建立時間，不自己判斷。
 * 標題、空狀態與條件說明的文字由呼叫端依用途給（重大違約終止、建立措施）。帳號還沒讀到時（account 不是 ready）不發認定的查詢。
 */
export default function CitedFindingsField({
	account,
	selected,
	disabled,
	required,
	emptyText,
	noteText,
	onChange,
}: CitedFindingsFieldProps) {
	const accountId = account.status === "ready" ? account.accountId : undefined;
	const query = useInfiniteQuery(
		SellerGovernanceAdminService.method.listViolations,
		accountId === undefined
			? skipToken
			: {
					accountId,
					status: ViolationStatus.ENFORCED,
					pageSize: 100,
					pageToken: "",
				},
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const findings = useMemo(
		() => query.data?.pages.flatMap((page) => page.violations) ?? [],
		[query.data],
	);

	// 讀取帳號（店家詳情）與讀取認定是同一個欄位的兩個前後步驟，共用同一組載入與失敗的呈現。
	const loading =
		account.status === "loading" ||
		(accountId !== undefined && query.isLoading);
	const failure =
		account.status === "failed"
			? { error: account.error, retry: account.retry }
			: accountId !== undefined && query.isError
				? { error: query.error, retry: () => void query.refetch() }
				: undefined;

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
			<legend className="form-label">
				依據的認定 {required && <span className="required">*</span>}
			</legend>
			{loading ? (
				<p className="adm-operation-field-note">載入認定中...</p>
			) : failure ? (
				<div>
					<p className="adm-operation-error" role="alert">
						{failure.error === undefined
							? READ_FAILED_TEXT
							: connectErrorText(failure.error, READ_FAILED_TEXT)}
					</p>
					<Button variant="outline" onClick={failure.retry}>
						重試
					</Button>
				</div>
			) : findings.length === 0 ? (
				<p className="adm-operation-field-note">{emptyText}</p>
			) : (
				<>
					{findings.map((finding) => (
						<label
							key={finding.id}
							style={{
								display: "flex",
								gap: 8,
								alignItems: "center",
								minHeight: 44,
							}}
						>
							<input
								type="checkbox"
								checked={selected.includes(finding.id)}
								disabled={disabled}
								onChange={(e) => toggle(finding.id, e.target.checked)}
							/>
							<span>
								認定 {finding.id}
								{finding.createTime
									? `・建立 ${formatDateTimeShort(timestampDate(finding.createTime))}`
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
				{noteText}
			</p>
		</fieldset>
	);
}
