"use client";

import { useMutation } from "@connectrpc/connect-query";
import {
	type Prospect,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import type { Store } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useEffect, useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { errorMessage } from "@/lib/error-text";
import { useStoreSearch } from "@/lib/use-store-search";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { ConvertConfirmDialog } from "./ConvertConfirmDialog";

/**
 * 歸因綁定面板（10-task 計畫 Task 7）：typeahead 搜尋賣家＋送出歸因。
 *
 * 確認框抽在 `ConvertConfirmDialog.tsx`（見該檔說明）；本檔只管「找到正確的 store」——輸入框、debounce、既有
 * 店家搜尋（`useStoreSearch`：ListStores，登入信箱先查帳號）的查詢狀態，以及送出 `convert` 的 mutation。
 *
 * 是否掛載本面板由 `page.tsx` 依 `prospect.status` 決定（只有 ACTIVE 才掛）：
 * 已入駐／已封存／已被合併都已經有上方的 lockNotice 說明原因，再掛一張
 * 「搜尋店家」的面板只會讓人誤以為還能對這些狀態做歸因——那正是
 * `AGENTS.md` UI 契約說的「無功能控制」。
 */

const DEBOUNCE_MS = 500;
const SEARCH_LIMIT = 10;

const CONVERT_FAILED_TEXT =
	"歸因綁定失敗，這筆站外業者的狀態沒有變動；請確認網路狀態後再試一次。";
const CONVERT_CONFLICT_TEXT =
	"資料已被其他操作更新。畫面已重新讀取，請確認最新狀態後再操作。";

export default function ConvertPanel({
	prospect,
	onUpdated,
	reload,
}: {
	prospect: Prospect;
	onUpdated: (prospect: Prospect) => void;
	reload: () => Promise<unknown>;
}) {
	const { showToast } = useToast();
	const convert = useMutation(
		SellerProspectAdminService.method.convertProspect,
	);
	const [converting, setConverting] = useState(false);
	const [confirmNotice, setConfirmNotice] = useState<GovernanceNotice | null>(
		null,
	);
	const [queryInput, setQueryInput] = useState("");
	const [debouncedQuery, setDebouncedQuery] = useState("");
	const [selectedStore, setSelectedStore] = useState<Store | null>(null);

	// 每次按鍵都打店家搜尋對後端沒必要，也會讓下拉結果一直閃爍重排，所以停打 DEBOUNCE_MS 後才查。
	useEffect(() => {
		const timer = setTimeout(() => {
			setDebouncedQuery(queryInput.trim());
		}, DEBOUNCE_MS);
		return () => clearTimeout(timer);
	}, [queryInput]);

	const searching = debouncedQuery !== "";
	// 輸入框已經改了、但還沒送出查詢（debounce 中）：畫面上的結果是上一個關鍵字的，不能讓人點。
	const inputSettled = queryInput.trim() === debouncedQuery;

	const searchQuery = useStoreSearch({
		keyword: debouncedQuery,
		tab: "all",
		pageToken: "",
		pageSize: SEARCH_LIMIT,
		enabled: searching,
	});

	const handleConvert = async (storeId: string) => {
		if (converting) return;
		setConfirmNotice(null);
		setConverting(true);
		try {
			const result = await runAction(
				() =>
					convert.mutateAsync({
						id: prospect.id,
						storeId,
						etag: prospect.etag,
					}),
				{
					reload,
					fallback: CONVERT_FAILED_TEXT,
					conflictText: CONVERT_CONFLICT_TEXT,
				},
			);
			if (result.ok) {
				if (result.value.prospect) onUpdated(result.value.prospect);
				showToast("已轉成店家", "success");
				setSelectedStore(null);
				setQueryInput("");
			} else {
				setConfirmNotice({ tone: "error", text: result.text });
			}
		} finally {
			setConverting(false);
		}
	};

	const results = searchQuery.items.flatMap((item) =>
		item.store ? [item.store] : [],
	);

	const pickStore = (candidate: Store) => {
		setConfirmNotice(null);
		setSelectedStore(candidate);
	};

	const closeDialog = () => {
		if (converting) return;
		setSelectedStore(null);
	};

	const searchNotice: GovernanceNotice | null = searchQuery.error
		? {
				tone: "error",
				text: errorMessage(searchQuery.error),
			}
		: null;

	return (
		<div className="card">
			<div className="card-header">
				<h3>
					<MaterialSymbol name="link" /> 歸因綁定
				</h3>
			</div>
			<div className="card-body">
				<p className="adm-operation-field-note">
					把這筆站外業者標記為「已入駐成某家店」，沒有反向操作；請先用下方欄位
					找到正確的店，確認框會再讓你並列核對雙方資料一次。
				</p>

				<div className="form-group" style={{ maxWidth: 420 }}>
					<label className="form-label" htmlFor="adm-convert-search">
						搜尋賣家（店名、電話、商店代號或登入信箱）
					</label>
					<input
						id="adm-convert-search"
						type="text"
						className="form-control"
						style={{ minHeight: 44 }}
						value={queryInput}
						onChange={(event) => setQueryInput(event.target.value)}
						placeholder="輸入店名、電話、商店代號或登入信箱關鍵字"
						disabled={converting}
					/>
				</div>

				{!searching && (
					<p style={{ fontSize: 12, color: "var(--gray-500)" }}>
						輸入至少一個字開始搜尋；店名可能重複，請以商店代號與電話核對。
					</p>
				)}

				{searching && searchQuery.isLoading && (
					<p
						aria-live="polite"
						style={{ fontSize: 13, color: "var(--gray-600)" }}
					>
						<MaterialSymbol name="progress_activity" /> 正在搜尋符合的賣家…
					</p>
				)}

				{searching && !searchQuery.isLoading && searchQuery.error && (
					<div>
						<GovernanceNoticeBanner notice={searchNotice} />
						<Button variant="outline" onClick={searchQuery.refetch}>
							重試搜尋
						</Button>
					</div>
				)}

				{searching &&
					inputSettled &&
					!searchQuery.isLoading &&
					!searchQuery.error &&
					results.length === 0 && (
						<p style={{ fontSize: 13, color: "var(--gray-600)" }}>
							查無符合的賣家
						</p>
					)}

				{inputSettled && !searchQuery.isLoading && results.length > 0 && (
					<ul
						style={{
							listStyle: "none",
							margin: "8px 0 0",
							padding: 0,
							border: "1px solid var(--gray-300)",
							borderRadius: 8,
							overflow: "hidden",
						}}
					>
						{results.map((result, index) => (
							<li
								key={result.id}
								style={{
									borderTop:
										index === 0 ? undefined : "1px solid var(--gray-200)",
								}}
							>
								<button
									type="button"
									onClick={() => pickStore(result)}
									disabled={converting}
									style={{
										display: "flex",
										flexDirection: "column",
										gap: 2,
										width: "100%",
										minHeight: 44,
										padding: "10px 14px",
										textAlign: "left",
										background: "var(--white)",
										border: "none",
										cursor: "pointer",
									}}
								>
									<strong>{result.storeName || "（未命名店鋪）"}</strong>
									<span style={{ fontSize: 12, color: "var(--gray-600)" }}>
										商店代號 · {result.storeCode || "未設定"} ｜ 電話 ·{" "}
										{result.contactPhone || "未登記"}
									</span>
								</button>
							</li>
						))}
					</ul>
				)}
			</div>

			{selectedStore && (
				<ConvertConfirmDialog
					prospect={prospect}
					store={{
						storeId: selectedStore.id,
						storeName: selectedStore.storeName,
						storeCode: selectedStore.storeCode,
						contactPhone: selectedStore.contactPhone,
					}}
					submitting={converting}
					notice={confirmNotice}
					onConfirm={() => void handleConvert(selectedStore.id)}
					onClose={closeDialog}
				/>
			)}
		</div>
	);
}
