"use client";

import { Code } from "@connectrpc/connect";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	type ProspectTaxIDMatch,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { ConvertConfirmDialog } from "@/app/admin/seller-prospects/[id]/ConvertConfirmDialog";
import { Button, StateRow, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { kycStatusText } from "@/lib/kyc-status";
import { LIST_PATH } from "../ProspectFilters";
import { useProspectCache } from "../prospect-queries";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 統編對帳頁。
 *
 * 這裡列的不是「疑似重複」（`duplicates/page.tsx`：兩筆站外業者互相比對聯絡方式）。這裡是統編對得上、但從來沒有人把
 * 這兩筆連起來的業者：這家店其實已經自己來平台開店、申請時填的統一編號跟站外業者主檔裡某一筆相同，站外那一筆還躺在
 * 名單上。清單 keyset 分頁，只列進行中的業者與還沒被認領的店家。
 *
 * 認證狀態一定要顯示：對到的這家店可能還在申請認證、尚未核准，跟已經是核准通過的正式賣家意義不同，admin 要能分辨這一點
 * 才能決定現在要不要就綁定，所以獨立一欄顯示。
 *
 * 一鍵綁定重用詳情頁的 `ConvertConfirmDialog`。清單不帶聯絡方式也不保證是最新版本，所以開確認框時先 `GetProspect` 那一筆，
 * 讀到之前不能確認，送出帶讀到的 etag。送出失敗的處理：版本衝突留在框內（重讀那一筆後可以再送）；店家已被認領、業者
 * 不再是進行中、店家已關閉這類永久性失敗關框，頁面顯示伺服器的句子並重讀清單，那一列由伺服器決定是否還在；
 * 沒收到回應留在框內可重試。
 */

const PAGE_SIZE = 50;

const CONVERT_FAILED_TEXT = "歸因綁定失敗，請確認網路狀態後再試一次。";
const CONVERT_CONFLICT_TEXT =
	"資料已被其他操作更新。畫面已重新讀取，請確認最新狀態後再操作。";

/** 這些錯誤碼表示這個配對現在不可行，重按只會拿到一樣的結果：關框、讓清單重讀。 */
const PERMANENT_CODES: readonly Code[] = [
	Code.AlreadyExists,
	Code.FailedPrecondition,
	Code.NotFound,
];

/** 對帳列的唯一鍵：業者 ID 加店家 ID，不預設任何單一欄位一定唯一。 */
function matchKey(match: ProspectTaxIDMatch): string {
	return `${match.prospect?.id}:${match.storeId}`;
}

export default function SellerProspectReconciliationPage() {
	const router = useRouter();
	const { showToast } = useToast();
	const cache = useProspectCache();
	const convert = useMutation(
		SellerProspectAdminService.method.convertProspect,
	);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [activeMatch, setActiveMatch] = useState<ProspectTaxIDMatch | null>(
		null,
	);
	const [converting, setConverting] = useState(false);
	const [confirmNotice, setConfirmNotice] = useState<GovernanceNotice | null>(
		null,
	);
	const [pageNotice, setPageNotice] = useState<GovernanceNotice | null>(null);

	const list = useQuery(
		SellerProspectAdminService.method.listProspectTaxIDMatches,
		{ pageSize: PAGE_SIZE, pageToken: currentToken(pageTokens) },
	);
	const matches = list.data?.matches ?? [];
	const next = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const activeProspectId = activeMatch?.prospect?.id ?? "";
	const detail = useQuery(
		SellerProspectAdminService.method.getProspect,
		{ id: activeProspectId },
		{ enabled: activeProspectId !== "" },
	);
	const prospect = detail.data?.prospect ?? null;

	const openDialog = (match: ProspectTaxIDMatch) => {
		setConfirmNotice(null);
		setPageNotice(null);
		setActiveMatch(match);
	};

	const closeDialog = () => {
		if (converting) return;
		setActiveMatch(null);
	};

	const reloadProspect = async () => {
		await detail.refetch({ throwOnError: true });
		cache.written();
	};

	const handleConfirm = async () => {
		if (converting || !activeMatch || !prospect) return;
		setConfirmNotice(null);
		setConverting(true);
		try {
			const result = await runAction(
				() =>
					convert.mutateAsync({
						id: prospect.id,
						storeId: activeMatch.storeId,
						etag: prospect.etag,
					}),
				{
					reload: reloadProspect,
					fallback: CONVERT_FAILED_TEXT,
					conflictText: CONVERT_CONFLICT_TEXT,
				},
			);
			if (result.ok) {
				cache.written(result.value.prospect);
				showToast("已轉成店家", "success");
				setActiveMatch(null);
				return;
			}
			if (
				result.kind === "message" &&
				result.code !== undefined &&
				PERMANENT_CODES.includes(result.code)
			) {
				cache.written();
				setPageNotice({ tone: "error", text: result.text });
				setActiveMatch(null);
				return;
			}
			setConfirmNotice({ tone: "error", text: result.text });
		} finally {
			setConverting(false);
		}
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">統編對帳</h1>
					<p className="page-subtitle">
						統編對得上、但從來沒有人把這兩筆連起來的業者：這家店其實已經自己
						來平台開店了，但站外業者主檔裡這一筆還躺在名單上沒被歸因。下方
						「認證狀態」顯示對到那家店目前的認證申請進度——尚未核准的申請也
						代表「這家已經來了」，但跟已核准的正式賣家意義不同，請先確認清楚
						再決定要不要現在就綁定。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						icon={<MaterialSymbol name="arrow_back" />}
						onClick={() => router.push(LIST_PATH)}
					>
						返回列表
					</Button>
				</div>
			</div>

			<GovernanceNoticeBanner notice={pageNotice} />

			{/* `.table-container` 的 `overflow: hidden`（globals.css）只裁切、不提供橫向捲軸；
			    行內覆寫 `overflow-x: auto` 才是真的讓窄螢幕能在自身容器捲動。 */}
			<div className="table-container" style={{ overflowX: "auto" }}>
				<table className="data-table">
					<thead>
						<tr>
							<th>站外業者（本筆）</th>
							<th>比對到的商家</th>
							<th>認證狀態</th>
							<th style={{ width: 140 }}>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={list.isLoading}
							error={
								list.isError
									? new Error(
											connectErrorText(list.error, "載入失敗，請稍後重試。"),
										)
									: null
							}
							empty={matches.length === 0}
							emptyText="目前沒有需要對帳的業者"
							colSpan={4}
							onRetry={() => void list.refetch()}
						>
							{matches.map((match) => (
								<tr key={matchKey(match)}>
									<td>
										<div className="cell-main">
											<Link
												href={`/admin/seller-prospects/${match.prospect?.id ?? ""}`}
											>
												{match.prospect?.displayName || "（未命名）"}
											</Link>
										</div>
										<div className="cell-sub">
											統編 · {match.prospect?.taxId || "未填寫"}
										</div>
									</td>
									<td>
										<div className="cell-main">
											<Link href={`/admin/merchants/${match.storeId}`}>
												{match.storeName || "（未命名店鋪）"}
											</Link>
										</div>
										<div className="cell-sub">Store ID · {match.storeId}</div>
										<div className="cell-sub">
											認證公司名稱 · {match.companyName || "未填寫"}
										</div>
									</td>
									<td>{kycStatusText(match.kycStatus)}</td>
									<td>
										<Button
											variant="outline"
											size="sm"
											onClick={() => openDialog(match)}
										>
											比對並綁定
										</Button>
									</td>
								</tr>
							))}
						</StateRow>
					</tbody>
				</table>
			</div>

			{page > 1 || next ? (
				<div
					style={{
						display: "flex",
						alignItems: "center",
						justifyContent: "center",
						gap: 16,
						marginTop: 20,
					}}
				>
					<Button
						variant="outline"
						size="sm"
						disabled={page === 1}
						onClick={() => setPageTokens(previousPage(pageTokens))}
					>
						上一頁
					</Button>
					<span>第 {page} 頁</span>
					<Button
						variant="outline"
						size="sm"
						disabled={!next}
						onClick={() => setPageTokens(nextPage(pageTokens, next))}
					>
						下一頁
					</Button>
				</div>
			) : null}

			{activeMatch ? (
				<ConvertConfirmDialog
					prospect={prospect}
					prospectLoading={detail.isFetching}
					prospectLoadFailed={detail.isError && !prospect}
					onRetryProspect={() => void detail.refetch()}
					store={{
						storeId: activeMatch.storeId,
						storeName: activeMatch.storeName,
						certificationCompanyName: activeMatch.companyName,
						certificationStatus: activeMatch.kycStatus,
					}}
					submitting={converting}
					notice={confirmNotice}
					onConfirm={() => void handleConfirm()}
					onClose={closeDialog}
				/>
			) : null}
		</div>
	);
}
