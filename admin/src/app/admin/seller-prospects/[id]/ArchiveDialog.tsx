"use client";

import { useMutation } from "@connectrpc/connect-query";
import {
	type Prospect,
	ProspectStatus,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog, StatusBadge, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import {
	previewRequest,
	prospectStatusText,
	prospectToForm,
} from "@/lib/seller-prospects";
import ProspectMatchDialog from "../ProspectMatchDialog";
import { useProspectMatchRetry } from "../use-prospect-match-retry";
import "@/styles/pages/DelegatedOperations.css";

/**
 * 封存／還原這筆站外業者的確認框。兩者互為逆動作，共用同一支對話框：骨架、送出狀態與錯誤顯示相同，差異只在後果說明
 * 與送出的呼叫。模式在開啟時由頁面固定；重讀後狀態不再符合該模式時只留「取消」。
 *
 * 封存帶讀到的 etag；還原不帶 etag（依據的是封存這個狀態），聯絡方式被其他進行中的業者佔走時（`PROSPECT_MATCH_FOUND`）
 * 開 `ProspectMatchDialog`，確認都不是同一家後帶全部 ID 重送。
 */

const ARCHIVE_FAILED_TEXT =
	"封存失敗，這筆站外業者的狀態沒有變動；請確認網路狀態後再試一次。";
const RESTORE_FAILED_TEXT =
	"還原失敗，這筆站外業者的狀態沒有變動；請確認網路狀態後再試一次。";
const CONFLICT_TEXT =
	"資料已被其他操作更新。畫面已重新讀取，請確認最新狀態後再操作。";

const STALE_NOTICE: GovernanceNotice = { tone: "error", text: CONFLICT_TEXT };

export default function ArchiveDialog({
	mode,
	prospect,
	onClose,
	onUpdated,
	reload,
}: {
	mode: "archive" | "restore";
	prospect: Prospect;
	onClose: () => void;
	onUpdated: (prospect: Prospect) => void;
	reload: () => Promise<unknown>;
}) {
	const { showToast } = useToast();
	const archive = useMutation(
		SellerProspectAdminService.method.archiveProspect,
	);
	const restore = useMutation(
		SellerProspectAdminService.method.restoreProspect,
	);
	const [submitting, setSubmitting] = useState(false);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const stale =
		mode === "archive"
			? prospect.status !== ProspectStatus.ACTIVE
			: prospect.status !== ProspectStatus.ARCHIVED || !!prospect.mergedIntoId;

	const send = async (confirmedIds: string[]) => {
		const result = await runAction<{ prospect?: Prospect }>(
			() =>
				mode === "archive"
					? archive.mutateAsync({ id: prospect.id, etag: prospect.etag })
					: restore.mutateAsync({
							id: prospect.id,
							confirmedDistinctProspectIds: confirmedIds,
						}),
			{
				reload,
				fallback:
					mode === "archive" ? ARCHIVE_FAILED_TEXT : RESTORE_FAILED_TEXT,
				conflictText: CONFLICT_TEXT,
			},
		);
		if (result.ok) {
			if (result.value.prospect) onUpdated(result.value.prospect);
			showToast(mode === "archive" ? "已封存" : "已還原", "success");
			onClose();
		}
		return result;
	};

	const match = useProspectMatchRetry({
		send,
		onFailure: (failure) => setNotice({ tone: "error", text: failure.text }),
	});

	const handleConfirm = async () => {
		if (submitting) return;
		setNotice(null);
		setSubmitting(true);
		try {
			await match.submit();
		} finally {
			setSubmitting(false);
		}
	};

	const descId = "adm-prospect-archive-description";

	return (
		<Dialog
			title={
				mode === "archive"
					? "確定要封存這筆站外業者嗎？"
					: "確定要還原這筆站外業者嗎？"
			}
			closeLabel={mode === "archive" ? "關閉封存視窗" : "關閉還原視窗"}
			describedBy={descId}
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					{stale ? null : (
						<Button
							variant={mode === "archive" ? "danger" : "primary"}
							onClick={() => void handleConfirm()}
							loading={submitting}
						>
							{submitting
								? mode === "archive"
									? "封存中…"
									: "還原中…"
								: mode === "archive"
									? "確認封存"
									: "確認還原"}
						</Button>
					)}
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>{mode === "archive" ? "封存對象" : "還原對象"}</span>
				<strong>{prospect.displayName}</strong>
				<span>
					目前狀態 ·{" "}
					<StatusBadge
						variant={
							prospect.status === ProspectStatus.ACTIVE ? "success" : "gray"
						}
					>
						{prospectStatusText(prospect.status)}
					</StatusBadge>
				</span>
			</div>

			<p id={descId} className="adm-operation-field-note">
				{mode === "archive"
					? "封存後這筆業者會立即從採購候選搜尋、統編對帳與疑似重複清單消失，寄給它的詢價連結會失效，logo 與封面會刪除；欄位僅供檢視，之後可以還原為進行中（圖片與詢價連結不會回來）。"
					: "還原後這筆業者會重新出現在採購候選搜尋、統編對帳與疑似重複清單，可以繼續編輯與聯絡追蹤；封存時刪除的圖片不會回來。聯絡方式與其他業者相同時，會先請你確認是不是同一家。"}
			</p>

			<GovernanceNoticeBanner notice={stale ? STALE_NOTICE : notice} />

			{match.pending ? (
				<ProspectMatchDialog
					key={match.pending.ids.join(",")}
					mode="restore"
					prospectIds={match.pending.ids}
					candidate={previewRequest(prospectToForm(prospect)) ?? {}}
					initiallyConfirmed={match.pending.confirmed}
					onConfirm={match.confirm}
					onCancel={match.cancel}
				/>
			) : null}
		</Dialog>
	);
}
