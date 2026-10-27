"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { useQuery } from "@connectrpc/connect-query";
import {
	type Prospect,
	ProspectStatus,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { type ReactNode, useState } from "react";
import { Button, StatusBadge } from "@/components/UI";
import type { BadgeVariant } from "@/components/UI/StatusBadge";
import { connectErrorText } from "@/lib/error-text";
import { prospectStatusText } from "@/lib/seller-prospects";
import { LIST_PATH } from "../ProspectFilters";
import { useProspectCache } from "../prospect-queries";
import ArchiveDialog from "./ArchiveDialog";
import ConvertPanel from "./ConvertPanel";
import ProspectEditForm from "./ProspectEditForm";
import ProspectMediaPanel from "./ProspectMediaPanel";
import "@/styles/pages/DelegatedOperations.css";
import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";

/**
 * 站外業者詳情頁：頁面持有唯一的 `prospect`（含 etag），文字表單、圖片區塊、封存還原與轉成店家都以它的 etag 寫入，
 * 成功後以回應換掉它。只有進行中的業者可以換圖、儲存編輯與轉成店家；其他狀態欄位唯讀顯示並附狀態說明。
 */

function statusBadgeVariant(status: ProspectStatus): BadgeVariant {
	switch (status) {
		case ProspectStatus.ACTIVE:
			return "success";
		case ProspectStatus.CONVERTED:
			return "info";
		default:
			return "gray"; // ARCHIVED 與未知值都算「不是進行中」，同列表頁的判斷
	}
}

interface LockNotice {
	/** 「已入駐」是好消息，不該套用「已封存／已被合併」那種警示紅；其餘兩種維持既有的鎖定紅。 */
	tone: "success" | "danger";
	icon: MaterialSymbolName;
	title: string;
	text: ReactNode;
}

/**
 * 三種鎖定狀態的說明，優先序：已入駐 > 已合併 > 已封存；同一筆只顯示排序最前面的一種。
 *
 * 不用 `GovernanceNoticeBanner`：它的 `text` 是純字串，容不下「已入駐，見〈商家詳情頁〉」這種需要內嵌連結的內容，
 * 改直接用它背後的兩個 class：`adm-authorization-notice`（綠色，已入駐是好消息）在 `DelegatedOperations.css`，
 * `audit-notice`（紅色，連同 `-icon`／`-content`／`-title`／`-text`，已封存與已被合併是「這一列現在不能做什麼事」）
 * 在 `src/app/globals.css`（全域載入）。
 */
function buildLockNotice(prospect: Prospect): LockNotice | null {
	if (prospect.status === ProspectStatus.CONVERTED) {
		return {
			tone: "success",
			icon: "storefront",
			title: "已入駐，此列不可再編輯",
			text: prospect.convertedStoreId ? (
				<>
					這筆站外業者已入駐平台，對應商家詳情頁：
					<Link href={`/admin/merchants/${prospect.convertedStoreId}`}>
						查看商家
					</Link>
					。入駐之後由商家自行維護資料。
				</>
			) : (
				"這筆站外業者已入駐平台，由商家自行維護資料。"
			),
		};
	}
	if (prospect.mergedIntoId) {
		return {
			tone: "danger",
			icon: "merge",
			title: "已被合併，此列不可再還原",
			text: (
				<>
					這筆站外業者已被合併，資料以合併後的主檔為準：
					<Link href={`/admin/seller-prospects/${prospect.mergedIntoId}`}>
						查看主檔
					</Link>
					，此列不可再還原，以下欄位僅供對照。
				</>
			),
		};
	}
	if (prospect.status === ProspectStatus.ARCHIVED) {
		return {
			tone: "danger",
			icon: "archive",
			title: "已封存",
			text: "這筆站外業者目前為已封存狀態，暫停聯絡追蹤中；封存期間欄位僅供檢視，須先還原為進行中才能編輯；封存時刪除的 logo 與封面不會回來。",
		};
	}
	return null;
}

export default function SellerProspectDetailPage() {
	const params = useParams<{ id: string }>();
	const id = params.id;
	const router = useRouter();
	const cache = useProspectCache();
	const [archiveMode, setArchiveMode] = useState<"archive" | "restore" | null>(
		null,
	);

	const detail = useQuery(
		SellerProspectAdminService.method.getProspect,
		{ id },
		{ enabled: !!id },
	);
	const prospect = detail.data?.prospect ?? null;
	const reload = () => detail.refetch();

	if (detail.isLoading) {
		return <div style={{ padding: 40, textAlign: "center" }}>載入中…</div>;
	}

	const notFound =
		detail.isError && ConnectError.from(detail.error).code === Code.NotFound;
	if (notFound || (!detail.isError && !prospect)) {
		return (
			<div style={{ padding: 40, textAlign: "center" }}>
				<p style={{ marginBottom: 16 }}>查無此站外業者</p>
				<Button variant="primary" onClick={() => router.push(LIST_PATH)}>
					返回列表
				</Button>
			</div>
		);
	}

	if (detail.isError || !prospect) {
		return (
			<div style={{ padding: 40, textAlign: "center" }}>
				<p style={{ marginBottom: 16 }}>
					讀取失敗：{connectErrorText(detail.error, "請稍後再試")}
				</p>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const lockNotice = buildLockNotice(prospect);
	// 只有進行中的業者可以編輯、換圖與轉成店家；封存的可以還原，被合併的不能。
	const canEdit = prospect.status === ProspectStatus.ACTIVE;
	const canArchive = canEdit;
	const canRestore =
		prospect.status === ProspectStatus.ARCHIVED && !prospect.mergedIntoId;
	const onUpdated = (updated: Prospect) => cache.written(updated);

	return (
		<div>
			<div className="detail-header">
				<div className="detail-header-left">
					<button
						type="button"
						className="btn-back"
						aria-label="返回站外業者列表"
						onClick={() => router.push(LIST_PATH)}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="detail-title-section">
						<div className="detail-title-row">
							<h1 className="detail-title">{prospect.displayName}</h1>
							<StatusBadge variant={statusBadgeVariant(prospect.status)}>
								{prospectStatusText(prospect.status)}
							</StatusBadge>
						</div>
					</div>
				</div>
				{(canArchive || canRestore) && (
					// 兩者互斥，同一顆按鈕依當下狀態切換文案／icon，不必疊兩顆各自判斷。
					<div className="detail-header-actions">
						<Button
							variant="outline"
							icon={<MaterialSymbol name={canArchive ? "archive" : "undo"} />}
							onClick={() => setArchiveMode(canArchive ? "archive" : "restore")}
						>
							{canArchive ? "封存" : "還原"}
						</Button>
					</div>
				)}
			</div>

			{archiveMode && (
				<ArchiveDialog
					mode={archiveMode}
					prospect={prospect}
					onClose={() => setArchiveMode(null)}
					onUpdated={onUpdated}
					reload={reload}
				/>
			)}

			{lockNotice?.tone === "success" ? (
				// `.adm-authorization-notice`（DelegatedOperations.css）：icon 必須是
				// 直接子層的圖示 svg，同 GovernanceNoticeBanner 自己 success 分支的形狀。
				// 這個 class 沒有另外的 title／text 二段式樣式，所以標題用 <strong> 內嵌。
				<div className="adm-authorization-notice" role="status">
					<MaterialSymbol name={lockNotice.icon} />
					<div>
						<strong>{lockNotice.title}。</strong> {lockNotice.text}
					</div>
				</div>
			) : lockNotice ? (
				<div className="audit-notice">
					<div className="audit-notice-icon">
						<MaterialSymbol name={lockNotice.icon} />
					</div>
					<div className="audit-notice-content">
						<div className="audit-notice-title">{lockNotice.title}</div>
						<div className="audit-notice-text">{lockNotice.text}</div>
					</div>
				</div>
			) : null}

			<ProspectMediaPanel
				key={`media-${prospect.id}`}
				prospect={prospect}
				onUpdated={onUpdated}
				reload={reload}
				readOnly={!canEdit}
			/>

			<div className="card">
				<div className="card-header">
					<h3>
						<MaterialSymbol name="edit_document" /> 主體資訊
					</h3>
				</div>
				<div className="card-body">
					<ProspectEditForm
						key={prospect.id}
						prospect={prospect}
						onUpdated={onUpdated}
						reload={reload}
						readOnly={!canEdit}
					/>
				</div>
			</div>

			{canEdit && (
				// `key={prospect.id}`：同一個 route 換到另一筆（例如從列表頁點另一列）不會整頁重新 mount，
				// 沒有這個 key 的話上一筆的搜尋輸入與已選定的店家會帶到這一筆，讓人對著錯的業者按下確認。
				<ConvertPanel
					key={prospect.id}
					prospect={prospect}
					onUpdated={onUpdated}
					reload={reload}
				/>
			)}
		</div>
	);
}
