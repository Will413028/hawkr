"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	ContactKind,
	type Prospect,
	type ProspectMatchReason,
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
	defaultWinnerId,
	matchReasonsText,
	primaryContactValue,
} from "@/lib/seller-prospects";
import { ProspectCompareTable } from "./ProspectCompareTable";
import { useProspectCache } from "./prospect-queries";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 站外業者的合併確認框（疑似重複頁、修改時的合併出口共用）。
 *
 * 合併是全案不可逆性最高的操作：敗方會被封存並指向勝方，詢價連結、聯絡方式與統編依規則併入勝方，敗方的圖片刪除，
 * 全部沒有反向操作。所以並列全部關鍵欄位、逐條列出後果、要求明文勾選，且勝方與敗方的方向由管理員自己選。
 *
 * 開啟時兩邊各 `GetProspect`（疑似重複清單不帶聯絡方式，也要拿最新的 etag）；送出帶讀到的兩個 etag。
 * 預設勝方：`defaultWinnerId` 指定的優先（統編出口以統編持有者為準），否則完整度高的一邊；每張身分卡都有「設為勝方」鍵，
 * 切換方向時清掉勾選，舊的勾選是對著舊方向做的核對。
 */

const TEXT = {
	title: "確定要合併這兩筆站外業者嗎？",
	loading: "正在讀取兩筆業者的完整資料…",
	loadFailed: "無法讀取業者資料。",
	conflict: "其中一筆已被其他操作更新。畫面已重新讀取，請重新核對後再合併。",
	unconfirmed:
		"無法確認合併是否完成。畫面已重新讀取，請確認兩筆的狀態後再決定。",
	draftLost: "這筆業者尚未儲存的修改不會保留。",
} as const;

interface MergePair {
	left: Prospect;
	right: Prospect;
}

const nameOf = (prospect: Prospect, fallback: string) =>
	prospect.displayName || fallback;

export default function MergeDialog({
	leftId,
	rightId,
	matchedOn,
	defaultWinnerId: preferredWinnerId,
	draftWillBeLost,
	onMerged,
	onClose,
}: {
	leftId: string;
	rightId: string;
	matchedOn?: readonly ProspectMatchReason[];
	/** 預設勝方（兩邊之一）；沒傳時依完整度。 */
	defaultWinnerId?: string;
	/** 從編輯頁開啟時，框內提醒目前這筆尚未儲存的修改不會保留。 */
	draftWillBeLost?: boolean;
	/** 合併成功後以回應的勝方呼叫。 */
	onMerged: (winner: Prospect) => void;
	onClose: () => void;
}) {
	const { showToast } = useToast();
	const cache = useProspectCache();
	const merge = useMutation(SellerProspectAdminService.method.mergeProspects);
	const [chosenWinnerId, setChosenWinnerId] = useState<string | null>(null);
	const [acknowledged, setAcknowledged] = useState(false);
	const [submitting, setSubmitting] = useState(false);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);

	const freshRead = {
		staleTime: 0,
		refetchOnMount: "always",
		retry: false,
	} as const;
	const leftQuery = useQuery(
		SellerProspectAdminService.method.getProspect,
		{ id: leftId },
		freshRead,
	);
	const rightQuery = useQuery(
		SellerProspectAdminService.method.getProspect,
		{ id: rightId },
		freshRead,
	);
	const left = leftQuery.data?.prospect;
	const right = rightQuery.data?.prospect;
	const loaded: MergePair | null = left && right ? { left, right } : null;
	const loadFailed = leftQuery.isError || rightQuery.isError;

	if (loaded && chosenWinnerId === null) {
		setChosenWinnerId(
			defaultWinnerId(loaded.left, loaded.right, preferredWinnerId),
		);
	}

	const chooseWinner = (id: string) => {
		setChosenWinnerId(id);
		setAcknowledged(false);
		setNotice(null);
	};

	const winner = loaded
		? chosenWinnerId === loaded.right.id
			? loaded.right
			: loaded.left
		: null;
	const loser =
		loaded && winner
			? winner === loaded.left
				? loaded.right
				: loaded.left
			: null;
	const notActive = loaded
		? [loaded.left, loaded.right].filter(
				(p) => p.status !== ProspectStatus.ACTIVE,
			)
		: [];
	const reading = leftQuery.isFetching || rightQuery.isFetching;

	const reloadBoth = () =>
		Promise.all([
			leftQuery.refetch({ throwOnError: true }),
			rightQuery.refetch({ throwOnError: true }),
		]);

	const handleConfirm = async () => {
		if (submitting || !winner || !loser || !acknowledged) return;
		setNotice(null);
		setSubmitting(true);
		try {
			const result = await runAction(
				() =>
					merge.mutateAsync({
						winnerId: winner.id,
						winnerEtag: winner.etag,
						loserId: loser.id,
						loserEtag: loser.etag,
					}),
				{
					reload: reloadBoth,
					fallback: TEXT.unconfirmed,
					conflictText: TEXT.conflict,
				},
			);
			if (result.ok && result.value.prospect) {
				const merged = result.value.prospect;
				cache.written(merged);
				cache.retired(loser.id);
				showToast(`已合併到「${nameOf(merged, "勝方")}」`, "success");
				onMerged(merged);
				return;
			}
			setAcknowledged(false);
			if (result.ok) {
				cache.written();
				await reloadBoth().catch(() => undefined);
				setNotice({ tone: "error", text: TEXT.unconfirmed });
				return;
			}
			if (result.kind === "message") {
				// 沒收到回應或其他失敗：合併可能已完成，清單類查詢一併失效重讀
				cache.written();
				await reloadBoth().catch(() => undefined);
			}
			setNotice({
				tone: "error",
				text: result.noResponse ? TEXT.unconfirmed : result.text,
			});
		} finally {
			setSubmitting(false);
		}
	};

	const descId = "adm-prospect-merge-description";
	const busy = submitting || reading;

	const renderIdentityCard = (prospect: Prospect) => {
		const isWinner = winner?.id === prospect.id;
		return (
			<div
				key={prospect.id}
				className={`adm-operation-identity-card${isWinner ? " adm-operation-identity-card--target" : ""}`}
			>
				<span className="adm-operation-identity-label">
					<MaterialSymbol name={isWinner ? "military_tech" : "archive"} />{" "}
					{isWinner ? "勝方・將保留" : "敗方・將被封存"}
				</span>
				<strong>{nameOf(prospect, "（未命名）")}</strong>
				<span>統一編號 · {prospect.taxId || "未填寫"}</span>
				<span>
					信箱 ·{" "}
					{primaryContactValue(prospect.contacts, ContactKind.EMAIL) ||
						"未填寫"}
				</span>
				<span>
					電話 ·{" "}
					{primaryContactValue(prospect.contacts, ContactKind.PHONE) ||
						"未填寫"}
				</span>
				{!isWinner ? (
					<Button
						variant="outline"
						size="sm"
						type="button"
						disabled={busy}
						onClick={() => chooseWinner(prospect.id)}
					>
						設為勝方
					</Button>
				) : null}
			</div>
		);
	};

	const winnerName = winner ? nameOf(winner, "勝方") : "勝方";
	const loserName = loser ? nameOf(loser, "敗方") : "敗方";

	return (
		<Dialog
			title={TEXT.title}
			closeLabel="關閉合併視窗"
			describedBy={descId}
			maxWidth={880}
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button
						variant="outline"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant="danger"
						type="button"
						onClick={() => void handleConfirm()}
						loading={submitting}
						disabled={
							!loaded ||
							loadFailed ||
							reading ||
							notActive.length > 0 ||
							!acknowledged
						}
					>
						{submitting ? "合併中…" : "確認合併"}
					</Button>
				</>
			}
		>
			<p id={descId} className="adm-operation-field-note">
				{matchedOn && matchedOn.length > 0 ? (
					<>
						比對依據 ·{" "}
						<StatusBadge variant="warning" size="sm">
							{matchReasonsText(matchedOn)}
						</StatusBadge>{" "}
						。
					</>
				) : null}
				合併沒有反向操作，請仔細核對下方逐欄資料再送出。
			</p>
			{draftWillBeLost ? (
				<p className="adm-operation-field-note">{TEXT.draftLost}</p>
			) : null}

			{loadFailed ? (
				<div>
					<GovernanceNoticeBanner
						notice={{ tone: "error", text: TEXT.loadFailed }}
					/>
					<Button
						variant="outline"
						type="button"
						onClick={() => void reloadBoth().catch(() => undefined)}
					>
						重試
					</Button>
				</div>
			) : !loaded || !winner || !loser ? (
				<p
					aria-live="polite"
					style={{ fontSize: 13, color: "var(--gray-600)" }}
				>
					<MaterialSymbol name="progress_activity" /> {TEXT.loading}
				</p>
			) : (
				<>
					<div className="adm-operation-identity-grid">
						{renderIdentityCard(loaded.left)}
						{renderIdentityCard(loaded.right)}
					</div>

					<div style={{ marginTop: 16 }}>
						<ProspectCompareTable left={loaded.left} right={loaded.right} />
					</div>

					{notActive.map((prospect) => (
						<GovernanceNoticeBanner
							key={prospect.id}
							notice={{
								tone: "error",
								text: `「${nameOf(prospect, "（未命名）")}」目前不是進行中，不能合併。`,
							}}
						/>
					))}

					<div className="adm-operation-boundary" style={{ marginTop: 16 }}>
						<div className="adm-operation-boundary-heading">
							<MaterialSymbol name="warning" />
							<div>
								<strong>送出後立刻發生，且沒有反向操作</strong>
								<span>合併沒有復原功能</span>
							</div>
						</div>
						<ul>
							{/* 每一條都寫成單一模板字串，避免 JSX 在文字節點與運算式邊界之間插入非預期的空格。 */}
							<li>
								{`「${loserName}」會被封存，並在其資料列指向「${winnerName}」`}
							</li>
							<li>
								{`寄給「${loserName}」的詢價連結改由「${winnerName}」承接`}
							</li>
							<li>
								{`若「${winnerName}」目前沒有統一編號，會把「${loserName}」的統編（${loser.taxId || "無"}）搬過去；若雙方都有，「${loserName}」的統編會被清空`}
							</li>
							<li>
								{`「${loserName}」的聯絡方式會併入「${winnerName}」（相同的只留一筆，「${winnerName}」的主要聯絡方式不變）；「${winnerName}」其他欄位（含分類、地區）不變，要保留「${loserName}」這些欄位的值，請先編輯「${winnerName}」再回來合併`}
							</li>
							<li>
								{`「${loserName}」的 logo 與封面會刪除，不會搬給「${winnerName}」`}
							</li>
							<li>
								{`「${loserName}」標記過的「不是同一家」改掛到「${winnerName}」；兩家之間的標記會撤銷`}
							</li>
						</ul>
					</div>

					<label
						htmlFor="adm-merge-ack"
						style={{
							display: "flex",
							alignItems: "flex-start",
							gap: 8,
							marginTop: 16,
							cursor: "pointer",
							fontSize: 14,
							color: "var(--hk-ink)",
						}}
					>
						<input
							id="adm-merge-ack"
							type="checkbox"
							checked={acknowledged}
							onChange={(event) => setAcknowledged(event.target.checked)}
							disabled={busy}
							style={{
								flex: "0 0 auto",
								width: "auto",
								minWidth: 0,
								margin: "3px 0 0",
								accentColor: "var(--hk-primary-strong)",
							}}
						/>
						<span>
							{`我已逐欄核對上方雙方資料，確認要以「${winnerName}」保留、「${loserName}」封存的方向送出，且了解合併沒有反向操作。`}
						</span>
					</label>
				</>
			)}

			<GovernanceNoticeBanner notice={notice} />
		</Dialog>
	);
}
