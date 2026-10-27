"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useMutation, useQuery } from "@tanstack/react-query";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import type React from "react";
import { useEffect, useRef, useState } from "react";
import { Dialog } from "@/components/UI";
import {
	AFTER_SALE_APPEAL_STATE_LABEL,
	AFTER_SALE_PLATFORM_DECISION_OUTCOME_LABEL,
	codeLabel,
} from "@/lib/codemeta";
import {
	type AdminContext,
	type Arbitration,
	type CaseDetail,
	selectAppealSubmittedAt,
	selectDisputePhotoKeys,
} from "@/lib/disputes/detail";
import { formatDateTime as formatTaipeiDateTimeShared } from "@/lib/format";
import { api } from "@/services/api";
import { buildImageUrl } from "@/utils/media";

// Evidence URLs come from buyer/seller uploads. Only treat http(s) as a
// clickable link — a javascript:/data: URL in href would be DOM XSS when an
// admin clicks the thumbnail.
function isSafeHttpUrl(u: string): boolean {
	try {
		const p = new URL(u);
		return p.protocol === "http:" || p.protocol === "https:";
	} catch {
		return false;
	}
}

type RulingType = "support_seller" | "support_buyer" | "compromise";

const OUTCOME_MAP: Record<
	RulingType,
	"REJECT_APPEAL" | "FULL_REFUND" | "PARTIAL_REFUND"
> = {
	support_seller: "REJECT_APPEAL",
	support_buyer: "FULL_REFUND",
	compromise: "PARTIAL_REFUND",
};

interface DetailResponse {
	code: number;
	message: string;
	data: {
		caseDetail: CaseDetail;
		arbitration: Arbitration;
		adminContext: AdminContext;
	};
}

interface DecideResponse {
	code: number;
	message: string;
	data: { caseNo: string; outcome: string; refundAmount: string };
}

// AfterSaleAppealState value：已裁決的案件不再開放裁決操作。
const APPEAL_STATE_RESOLVED = "30";

// 裁決結果的**名稱**走生成的 AFTER_SALE_PLATFORM_DECISION_OUTCOME_LABEL（SSOT 是
// backend 的 業務類型碼.md）。這裡只留「這個裁決對誰有利」的補充說明——那是本頁的
// 呈現決定，後端不下發，也不屬於碼表。
//
// 以前這裡是 `REJECT_APPEAL: "駁回申訴（支持賣家）"` 這種把兩者黏在一起的手寫表，
// 結果名稱那一半就跟著漂了（SSOT 是「駁回申訴」，這裡自己長出括號後綴）。
const OUTCOME_HINT: Record<string, string> = {
	REJECT_APPEAL: "支持賣家",
	FULL_REFUND: "支持買家",
	PARTIAL_REFUND: "折衷裁決",
};

const outcomeLabel = (outcome: string): string => {
	const hint = OUTCOME_HINT[outcome];
	const label = codeLabel(AFTER_SALE_PLATFORM_DECISION_OUTCOME_LABEL, outcome);
	return hint ? `${label}（${hint}）` : label;
};

// 時間一律走 lib/format 的台北釘死版（backend#3053 的稽查）：本地這份用
// toLocaleString 會跟著看的人的瀏覽器時區跑，而管理員不一定在台灣。
const formatDateTime = formatTaipeiDateTimeShared;

// 讀不到金額時回 "-"。舊版回 `String(raw)`，欄位名一錯就把字串 "undefined" 印在畫面上
// （售後商品區塊的實際症狀）——寧可顯示缺值也不要把 JS 型別漏到管理員眼前。
// 幣別前綴是 `NT$ ` 不是裸 `$`：這一頁在裁決要退多少錢，`$225` 讀不出是台幣還是美元。
// 與同 feature 的列表頁（`disputes/page.tsx`）及 `anomalies` 的同名函式一致。
function formatAmount(raw: string | number | undefined): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return "-";
	return `NT$ ${n.toLocaleString()}`;
}

// imageKey ＝ 頭像/商家 logo 的 objectKey（SSOT）；顯示 URL 由 buildImageUrl 依 preset 組。
function Avatar({
	imageKey,
	name,
	size = 36,
}: {
	imageKey: string;
	name: string;
	size?: number;
}) {
	const initials = (name || "?").charAt(0).toUpperCase();
	if (imageKey) {
		return (
			<Image
				src={buildImageUrl(imageKey, "avatar")}
				alt={name}
				width={size}
				height={size}
				unoptimized
				style={{
					width: size,
					height: size,
					borderRadius: "50%",
					objectFit: "cover",
					flexShrink: 0,
				}}
				onError={(e) => {
					(e.target as HTMLImageElement).style.display = "none";
				}}
			/>
		);
	}
	return (
		<div
			style={{
				width: size,
				height: size,
				borderRadius: "50%",
				background: "#e5e7eb",
				display: "flex",
				alignItems: "center",
				justifyContent: "center",
				fontSize: size * 0.4,
				fontWeight: 700,
				color: "#374151",
				flexShrink: 0,
			}}
		>
			{initials}
		</div>
	);
}

function PhotoGrid({
	photoKeys,
	emptyText,
}: {
	photoKeys: string[];
	emptyText?: string;
}) {
	if (!photoKeys.length) {
		return emptyText ? (
			<span style={{ color: "#9ca3af", fontSize: 13 }}>{emptyText}</span>
		) : null;
	}
	return (
		<div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 8 }}>
			{photoKeys.map((key, i) => {
				// 傳進來的是 objectKey（SSOT）；縮圖走 thumb preset，開新分頁看原圖。
				const full = buildImageUrl(key, "original");
				return (
					<a
						key={key}
						href={isSafeHttpUrl(full) ? full : undefined}
						target="_blank"
						rel="noreferrer"
					>
						<Image
							src={buildImageUrl(key, "thumb")}
							alt={`照片${i + 1}`}
							width={80}
							height={80}
							unoptimized
							style={{
								width: 80,
								height: 80,
								objectFit: "cover",
								borderRadius: 8,
								border: "1px solid #e5e7eb",
							}}
						/>
					</a>
				);
			})}
		</div>
	);
}

export default function DisputeDetail() {
	const router = useRouter();
	const params = useParams();
	const caseNo = decodeURIComponent((params?.id as string) ?? "");

	const [modalType, setModalType] = useState<RulingType | null>(null);
	const [rejectReason, setRejectReason] = useState("");
	const [refundReason, setRefundReason] = useState("");
	const [partialRefundAmount, setPartialRefundAmount] = useState(0);
	const [partialReason, setPartialReason] = useState("");

	const {
		data: detail = null,
		isLoading: loading,
		error,
		refetch,
	} = useQuery({
		queryKey: ["disputes", "detail", caseNo],
		queryFn: async () => {
			const res = await api.get<DetailResponse>(
				`/api/v1/admin/after_sales/${encodeURIComponent(caseNo)}`,
			);
			if (res?.code === 0 && res.data) return res.data;
			throw new Error(res?.message || "載入失敗");
		},
		enabled: !!caseNo,
	});
	const loadError = error
		? error instanceof Error
			? error.message
			: "載入失敗"
		: "";

	// 一次性 seed：partialRefundAmount 預設為可退金額的一半。用 ref 確保只在
	// detail 首次到達時 seed，背景 refetch 不會覆蓋管理員已輸入的金額。
	const seededRef = useRef(false);
	useEffect(() => {
		if (detail && !seededRef.current) {
			seededRef.current = true;
			const refundable = Number(detail.caseDetail.refundableAmount) || 0;
			setPartialRefundAmount(Math.floor(refundable / 2));
		}
	}, [detail]);

	// 裁決送出 → 成功後導回列表（不 invalidate：離開本頁，無 re-seed 風險）
	const decideMutation = useMutation({
		mutationFn: (body: Record<string, string>) =>
			api.post<DecideResponse>(
				`/api/v1/admin/after_sales/${encodeURIComponent(caseNo)}/decide`,
				body,
			),
		onSuccess: () => {
			alert("裁決已送出");
			router.push("/admin/disputes");
		},
		onError: (err) => {
			alert(`裁決失敗：${err instanceof Error ? err.message : "請稍後再試"}`);
		},
		onSettled: () => setModalType(null),
	});
	const submitting = decideMutation.isPending;

	const caseDetail = detail?.caseDetail;
	const arbitration = detail?.arbitration;
	const adminCtx = detail?.adminContext;

	const refundableAmount = Number(caseDetail?.refundableAmount) || 0;
	const isResolved = caseDetail?.appealState === APPEAL_STATE_RESOLVED;

	const appealSubmittedAt = selectAppealSubmittedAt(arbitration);
	const photoKeys = selectDisputePhotoKeys(caseDetail, appealSubmittedAt);

	const submitRuling = (type: RulingType) => {
		if (submitting) return;
		const outcome = OUTCOME_MAP[type];
		let reason = "";
		let refundAmount = "";

		if (type === "support_seller") {
			if (!rejectReason.trim()) {
				alert("請輸入駁回原因");
				return;
			}
			reason = rejectReason.trim();
		} else if (type === "support_buyer") {
			reason = refundReason.trim() || "全額退款";
		} else {
			if (!partialReason.trim()) {
				alert("請輸入裁決說明");
				return;
			}
			if (partialRefundAmount <= 0 || partialRefundAmount > refundableAmount) {
				alert(`退款金額必須介於 1 ~ ${refundableAmount}`);
				return;
			}
			reason = partialReason.trim();
			refundAmount = String(partialRefundAmount);
		}

		const body: Record<string, string> = { outcome, reason };
		if (refundAmount) body.refundAmount = refundAmount;
		decideMutation.mutate(body);
	};

	if (loading) {
		return (
			<div
				style={{
					display: "flex",
					alignItems: "center",
					justifyContent: "center",
					minHeight: 300,
					color: "#9ca3af",
					fontSize: 16,
				}}
			>
				載入中...
			</div>
		);
	}

	if (loadError || !detail) {
		return (
			<div
				style={{
					display: "flex",
					flexDirection: "column",
					alignItems: "center",
					justifyContent: "center",
					minHeight: 300,
					gap: 16,
				}}
			>
				<p style={{ color: "#ef4444", fontSize: 16 }}>
					{loadError || "載入失敗"}
				</p>
				<button
					type="button"
					onClick={() => refetch()}
					style={{
						background: "var(--admin-primary-strong)",
						color: "white",
						border: "none",
						borderRadius: 8,
						padding: "8px 20px",
						cursor: "pointer",
						fontWeight: 600,
					}}
				>
					重試
				</button>
			</div>
		);
	}

	return (
		<div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
			{/* Header */}
			<div style={headerCardStyle}>
				<div style={{ display: "flex", alignItems: "center", gap: 24 }}>
					<button
						type="button"
						onClick={() => router.push("/admin/disputes")}
						style={backBtnStyle}
						aria-label="返回"
					>
						<MaterialSymbol
							name="keyboard_arrow_left"
							style={{ fontSize: 24 }}
						/>
					</button>
					<div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
						<h1
							style={{
								fontSize: 28,
								fontWeight: 700,
								margin: 0,
								color: "#000",
							}}
						>
							售後爭議處理
						</h1>
						<div
							style={{
								display: "flex",
								gap: 16,
								color: "#565656",
								fontSize: 16,
								flexWrap: "wrap",
							}}
						>
							<span>
								<MaterialSymbol name="list_alt" style={{ marginRight: 4 }} />
								案件編號：{caseDetail?.caseNo}
							</span>
							<span>
								<MaterialSymbol
									name="shopping_bag"
									style={{ marginRight: 4 }}
								/>
								訂單編號：{caseDetail?.orderNo || "-"}
							</span>
							<span>
								<MaterialSymbol name="schedule" style={{ marginRight: 4 }} />
								申訴時間：{formatDateTime(appealSubmittedAt)}
							</span>
						</div>
					</div>
				</div>
				<div
					style={{
						border: `2px solid ${isResolved ? "#9ca3af" : "var(--admin-primary-strong)"}`,
						borderRadius: 8,
						padding: "10px 16px",
						color: isResolved ? "#9ca3af" : "var(--admin-primary-strong)",
						fontWeight: 700,
						fontSize: 16,
					}}
				>
					{codeLabel(AFTER_SALE_APPEAL_STATE_LABEL, caseDetail?.appealState)}
				</div>
			</div>

			<div
				style={{
					display: "grid",
					gridTemplateColumns: "2fr 1fr",
					gap: 24,
					alignItems: "start",
				}}
			>
				{/* Left: case details */}
				<div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
					{/* Buyer / Seller */}
					<div style={cardStyle}>
						<div style={cardHeaderStyle}>
							<MaterialSymbol name="person" style={{ fontSize: 22 }} />
							<span>買賣家資訊</span>
						</div>
						<div style={{ padding: 24, display: "flex", gap: 32 }}>
							<PartyCard
								partyLabel="買家"
								avatarKey={adminCtx?.avatarKey ?? ""}
								name={adminCtx?.buyerName || "（未知）"}
								id={caseDetail?.buyerId ?? ""}
								color="#2b7fff"
							/>
							<div style={{ width: 1, background: "#e5e7eb" }} />
							<PartyCard
								partyLabel="賣家"
								avatarKey={adminCtx?.logoKey ?? ""}
								name={adminCtx?.sellerName || "（未知）"}
								id=""
								color="var(--admin-primary-strong)"
							/>
						</div>
					</div>

					{/* Items */}
					{(caseDetail?.items?.length ?? 0) > 0 && (
						<div style={cardStyle}>
							<div style={cardHeaderStyle}>
								<MaterialSymbol
									name="shopping_basket"
									style={{ fontSize: 22 }}
								/>
								<span>售後商品</span>
							</div>
							<div style={{ padding: "0 24px 16px" }}>
								{caseDetail?.items.map((item) => (
									<div
										key={item.orderItemId}
										style={{
											display: "flex",
											justifyContent: "space-between",
											alignItems: "center",
											padding: "12px 0",
											borderBottom: "1px solid #f3f4f6",
										}}
									>
										<div>
											{/* 售後明細沒有商品名，規格名就是這一列的標題（proto
											    AfterSaleCaseItemView 只有 sku_name）。 */}
											<div style={{ fontWeight: 600, color: "#101828" }}>
												{item.skuName || "（未命名規格）"}
											</div>
											<div
												style={{
													color: "#6b7280",
													fontSize: 13,
													marginTop: 2,
												}}
											>
												SKU {item.skuId}
											</div>
										</div>
										<div style={{ textAlign: "right", fontSize: 14 }}>
											<div style={{ color: "#374151" }}>× {item.quantity}</div>
											<div style={{ fontWeight: 700, color: "#101828" }}>
												{formatAmount(item.actualPayAmount)}
											</div>
										</div>
									</div>
								))}
							</div>
						</div>
					)}

					{/* Buyer original complaint */}
					<div style={cardStyle}>
						<div style={cardHeaderStyle}>
							<MaterialSymbol
								name="record_voice_over"
								style={{ fontSize: 22 }}
							/>
							<span>買家原始申請</span>
						</div>
						<div
							style={{
								padding: 24,
								display: "flex",
								flexDirection: "column",
								gap: 12,
							}}
						>
							<InfoRow label="申訴理由">
								{adminCtx?.buyerAppealReason || "-"}
							</InfoRow>
							{adminCtx?.buyerAppealDescription && (
								<div>
									<div
										style={{
											fontSize: 13,
											color: "#6b7280",
											fontWeight: 500,
											marginBottom: 4,
										}}
									>
										說明
									</div>
									<div
										style={{
											background: "#f9fafb",
											borderRadius: 8,
											padding: 12,
											fontSize: 14,
											color: "#374151",
											lineHeight: 1.6,
										}}
									>
										{adminCtx.buyerAppealDescription}
									</div>
								</div>
							)}
							<div>
								<div
									style={{
										fontSize: 13,
										color: "#6b7280",
										fontWeight: 500,
										marginBottom: 2,
									}}
								>
									申請時附上的照片
								</div>
								<PhotoGrid
									photoKeys={photoKeys.buyerOriginal}
									emptyText="無照片"
								/>
							</div>
						</div>
					</div>

					{/* Seller rejection */}
					<div style={cardStyle}>
						<div style={cardHeaderStyle}>
							<MaterialSymbol name="storefront" style={{ fontSize: 22 }} />
							<span>賣家婉拒原因</span>
						</div>
						<div
							style={{
								padding: 24,
								display: "flex",
								flexDirection: "column",
								gap: 12,
							}}
						>
							<InfoRow label="拒絕原因代碼">
								{adminCtx?.sellerRejectReason || "-"}
							</InfoRow>
							{adminCtx?.sellerRejectMessage && (
								<div>
									<div
										style={{
											fontSize: 13,
											color: "#6b7280",
											fontWeight: 500,
											marginBottom: 4,
										}}
									>
										賣家說明
									</div>
									<div
										style={{
											background: "#f9fafb",
											borderRadius: 8,
											padding: 12,
											fontSize: 14,
											color: "#374151",
											lineHeight: 1.6,
										}}
									>
										{adminCtx.sellerRejectMessage}
									</div>
								</div>
							)}
							<div>
								<div
									style={{
										fontSize: 13,
										color: "#6b7280",
										fontWeight: 500,
										marginBottom: 2,
									}}
								>
									驗貨照片
								</div>
								<PhotoGrid
									photoKeys={photoKeys.sellerInspection}
									emptyText="無照片"
								/>
							</div>
							<div>
								<div
									style={{
										fontSize: 13,
										color: "#6b7280",
										fontWeight: 500,
										marginBottom: 2,
									}}
								>
									出貨照片
								</div>
								<PhotoGrid
									photoKeys={photoKeys.sellerShipment}
									emptyText="無照片"
								/>
							</div>
						</div>
					</div>

					{/* Buyer appeal (additional evidence) */}
					{photoKeys.buyerAppeal.length > 0 && (
						<div style={cardStyle}>
							<div style={cardHeaderStyle}>
								<MaterialSymbol name="upload" style={{ fontSize: 22 }} />
								<span>買家申訴追加證據</span>
							</div>
							<div style={{ padding: 24 }}>
								<PhotoGrid photoKeys={photoKeys.buyerAppeal} />
							</div>
						</div>
					)}

					{/* Arbitration timeline */}
					{(arbitration?.timeline?.length ?? 0) > 0 && (
						<div style={cardStyle}>
							<div style={cardHeaderStyle}>
								<MaterialSymbol name="history" style={{ fontSize: 22 }} />
								<span>平台仲裁時間軸</span>
							</div>
							<div style={{ padding: 24 }}>
								{arbitration?.timeline.map((entry, i) => (
									<div
										key={`${entry.label}-${entry.timestamp}`}
										style={{
											display: "flex",
											gap: 16,
											paddingBottom:
												i < arbitration.timeline.length - 1 ? 20 : 0,
										}}
									>
										<div
											style={{
												display: "flex",
												flexDirection: "column",
												alignItems: "center",
											}}
										>
											<div
												style={{
													width: 10,
													height: 10,
													borderRadius: "50%",
													background: "var(--admin-primary-strong)",
													flexShrink: 0,
													marginTop: 4,
												}}
											/>
											{i < arbitration.timeline.length - 1 && (
												<div
													style={{
														width: 2,
														flex: 1,
														background: "#e5e7eb",
														marginTop: 4,
													}}
												/>
											)}
										</div>
										<div
											style={{
												paddingBottom:
													i < arbitration.timeline.length - 1 ? 0 : 0,
											}}
										>
											<div
												style={{
													fontWeight: 600,
													fontSize: 15,
													color: "#101828",
												}}
											>
												{entry.label}
											</div>
											<div
												style={{ fontSize: 13, color: "#6b7280", marginTop: 2 }}
											>
												{formatDateTime(entry.timestamp)}
											</div>
											{entry.description && (
												<div
													style={{
														fontSize: 14,
														color: "#374151",
														marginTop: 4,
													}}
												>
													{entry.description}
												</div>
											)}
										</div>
									</div>
								))}
							</div>
						</div>
					)}

					{/* Decision result */}
					{arbitration?.decision && (
						<div style={{ ...cardStyle, border: "2px solid #9ca3af" }}>
							<div style={{ ...cardHeaderStyle, background: "#f3f4f6" }}>
								<MaterialSymbol name="balance" style={{ fontSize: 22 }} />
								<span>裁決結果</span>
							</div>
							<div
								style={{
									padding: 24,
									display: "flex",
									flexDirection: "column",
									gap: 12,
								}}
							>
								<InfoRow label="裁決結果">
									{outcomeLabel(arbitration.decision.outcome)}
								</InfoRow>
								{arbitration.decision.refundAmount && (
									<InfoRow label="退款金額">
										{formatAmount(arbitration.decision.refundAmount)}
									</InfoRow>
								)}
								<InfoRow label="裁決時間">
									{formatDateTime(arbitration.decision.decidedAt)}
								</InfoRow>
								{adminCtx?.decidedByAdminId && (
									<InfoRow label="裁決管理員 ID">
										{adminCtx.decidedByAdminId}
									</InfoRow>
								)}
								{arbitration.decision.reason && (
									<div>
										<div
											style={{
												fontSize: 13,
												color: "#6b7280",
												fontWeight: 500,
												marginBottom: 4,
											}}
										>
											裁決理由
										</div>
										<div
											style={{
												background: "#f3f4f6",
												borderRadius: 8,
												padding: 12,
												fontSize: 14,
												color: "#374151",
												lineHeight: 1.6,
											}}
										>
											{arbitration.decision.reason}
										</div>
									</div>
								)}
							</div>
						</div>
					)}
				</div>

				{/* Right: ruling panel */}
				<div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
					<div style={cardStyle}>
						<div style={cardHeaderStyle}>
							<MaterialSymbol name="attach_money" style={{ fontSize: 22 }} />
							<span>退款資訊</span>
						</div>
						<div style={{ padding: 24 }}>
							<div style={{ textAlign: "center" }}>
								<div
									style={{ fontSize: 13, color: "#6b7280", marginBottom: 4 }}
								>
									可退款上限
								</div>
								<div
									style={{ fontSize: 32, fontWeight: 700, color: "#101828" }}
								>
									{formatAmount(refundableAmount)}
								</div>
							</div>
						</div>
					</div>

					<div style={cardStyle}>
						<div style={cardHeaderStyle}>
							<MaterialSymbol name="balance" style={{ fontSize: 22 }} />
							<span>管理員裁決</span>
						</div>
						<div
							style={{
								padding: 20,
								display: "flex",
								flexDirection: "column",
								gap: 12,
							}}
						>
							<RulingOption
								title="支持賣家"
								desc="認定賣家無過失，駁回買家申訴，案件結案。"
								btnLabel="駁回申訴"
								btnColor="#ff2e31"
								highlighted
								disabled={isResolved || submitting}
								onClick={() => setModalType("support_seller")}
							/>
							<RulingOption
								title="支持買家"
								desc="認定賣家有過失，全額退款給買家。"
								btnLabel="全額退款"
								btnColor="var(--admin-primary-strong)"
								disabled={isResolved || submitting}
								onClick={() => setModalType("support_buyer")}
							/>
							<RulingOption
								title="折衷裁決"
								desc="雙方皆有責任，部分退款給買家。"
								btnLabel="部分退款"
								btnColor="var(--admin-primary-strong)"
								disabled={isResolved || submitting}
								onClick={() => setModalType("compromise")}
							/>
							{isResolved && (
								<div
									style={{
										background: "#f3f4f6",
										border: "1px solid #dadada",
										borderRadius: 10,
										padding: 12,
										color: "#6b7280",
										fontSize: 14,
										textAlign: "center",
									}}
								>
									此案件已裁決，無法再次操作
								</div>
							)}
						</div>
					</div>
				</div>
			</div>

			{/* Ruling Modals */}
			{modalType === "support_seller" && (
				<RulingModal
					title="確認駁回申訴？"
					subtitle="系統將通知買家，案件結案且無法再次申訴。"
					confirmLabel="駁回申訴"
					danger
					submitting={submitting}
					onCancel={() => setModalType(null)}
					onConfirm={() => submitRuling("support_seller")}
				>
					<ModalField label="駁回理由（對買賣家可見）">
						<textarea
							value={rejectReason}
							onChange={(e) => setRejectReason(e.target.value)}
							placeholder="請輸入駁回理由..."
							maxLength={1000}
							style={modalTextareaStyle}
						/>
					</ModalField>
				</RulingModal>
			)}

			{modalType === "support_buyer" && (
				<RulingModal
					title="確認全額退款？"
					subtitle={`將退款 ${formatAmount(refundableAmount)} 給買家。`}
					confirmLabel="全額退款"
					submitting={submitting}
					onCancel={() => setModalType(null)}
					onConfirm={() => submitRuling("support_buyer")}
				>
					<ModalField label="退款說明（選填，對買賣家可見）">
						<textarea
							value={refundReason}
							onChange={(e) => setRefundReason(e.target.value)}
							placeholder="輸入退款說明..."
							maxLength={1000}
							style={modalTextareaStyle}
						/>
					</ModalField>
				</RulingModal>
			)}

			{modalType === "compromise" && (
				<RulingModal
					title="確認部分退款？"
					subtitle={`可退款上限 ${formatAmount(refundableAmount)}，請輸入實際退款金額。`}
					confirmLabel="送出裁決"
					submitting={submitting}
					onCancel={() => setModalType(null)}
					onConfirm={() => submitRuling("compromise")}
				>
					<ModalField label={`退款金額（1 ~ ${refundableAmount}）`}>
						<div
							style={{
								background: "white",
								border: "1px solid #d1d5dc",
								borderRadius: 8,
								padding: "4px 12px",
								height: 40,
								display: "flex",
								alignItems: "center",
								gap: 4,
							}}
						>
							{/* 與同頁 formatAmount 同一個幣別前綴：管理員在這一格輸入退款金額，
							    顯示處寫 NT$ 而輸入處寫 $ 等於在同一個畫面上問「這是哪一國的錢」。 */}
							<span style={{ color: "#6a7282", fontSize: 16 }}>NT$</span>
							<input
								type="number"
								min={1}
								max={refundableAmount}
								value={partialRefundAmount}
								onChange={(e) =>
									setPartialRefundAmount(
										Math.max(0, Math.floor(Number(e.target.value) || 0)),
									)
								}
								style={{
									flex: 1,
									minWidth: 0,
									border: "none",
									outline: "none",
									textAlign: "right",
									fontWeight: 700,
									fontSize: 18,
									color: "#101828",
									background: "transparent",
								}}
							/>
						</div>
					</ModalField>
					<ModalField label="裁決理由（對買賣家可見）">
						<textarea
							value={partialReason}
							onChange={(e) => setPartialReason(e.target.value)}
							placeholder="說明責任比例與裁決依據..."
							maxLength={1000}
							style={modalTextareaStyle}
						/>
					</ModalField>
				</RulingModal>
			)}
		</div>
	);
}

function PartyCard({
	partyLabel,
	avatarKey,
	name,
	id,
	color,
}: {
	partyLabel: string;
	avatarKey: string;
	name: string;
	id: string;
	color: string;
}) {
	return (
		<div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 8 }}>
			<span
				style={{
					fontSize: 12,
					padding: "2px 8px",
					border: `1px solid ${color}`,
					borderRadius: 4,
					color,
					fontWeight: 600,
					alignSelf: "flex-start",
				}}
			>
				{partyLabel}
			</span>
			<div style={{ display: "flex", alignItems: "center", gap: 10 }}>
				<Avatar imageKey={avatarKey} name={name} size={40} />
				<div>
					<div style={{ fontWeight: 700, fontSize: 15, color: "#101828" }}>
						{name}
					</div>
					{id && (
						<div style={{ fontSize: 12, color: "#6b7280", marginTop: 2 }}>
							ID: {id}
						</div>
					)}
				</div>
			</div>
		</div>
	);
}

function InfoRow({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div
			style={{
				display: "flex",
				justifyContent: "space-between",
				alignItems: "center",
				borderBottom: "1px solid #f3f4f6",
				paddingBottom: 12,
			}}
		>
			<span style={{ color: "#6b7280", fontSize: 14, fontWeight: 500 }}>
				{label}
			</span>
			<span style={{ color: "#101828", fontSize: 15, fontWeight: 600 }}>
				{children}
			</span>
		</div>
	);
}

function RulingModal({
	title,
	subtitle,
	confirmLabel,
	danger = false,
	submitting,
	children,
	onCancel,
	onConfirm,
}: {
	title: string;
	subtitle: string;
	confirmLabel: string;
	/** 不可逆的駁回用危險色；其餘用主色。 */
	danger?: boolean;
	submitting: boolean;
	children: React.ReactNode;
	onCancel: () => void;
	onConfirm: () => void;
}) {
	return (
		<Dialog
			busy={submitting}
			title={title}
			maxWidth={494}
			onClose={onCancel}
			footer={
				<>
					<button
						type="button"
						className="btn"
						onClick={onCancel}
						disabled={submitting}
					>
						取消
					</button>
					<button
						type="button"
						className={danger ? "btn btn-danger" : "btn btn-primary"}
						onClick={onConfirm}
						disabled={submitting}
					>
						{submitting ? "送出中..." : confirmLabel}
					</button>
				</>
			}
		>
			<p style={{ margin: "0 0 16px", color: "var(--admin-muted-aa)" }}>
				{subtitle}
			</p>
			<div style={{ display: "grid", gap: 20 }}>{children}</div>
		</Dialog>
	);
}

function ModalField({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
			<span style={{ fontWeight: 700, fontSize: 15, color: "#000" }}>
				{label}
			</span>
			{children}
		</div>
	);
}

function RulingOption({
	title,
	desc,
	btnLabel,
	btnColor,
	highlighted,
	disabled,
	onClick,
}: {
	title: string;
	desc: string;
	btnLabel: string;
	btnColor: string;
	highlighted?: boolean;
	disabled?: boolean;
	onClick: () => void;
}) {
	return (
		<div
			style={{
				background: highlighted ? "var(--color-ink-100)" : "white",
				border: `2px solid ${highlighted ? "var(--admin-primary-strong)" : "#dadada"}`,
				borderRadius: 14,
				padding: 18,
				display: "flex",
				alignItems: "center",
				justifyContent: "space-between",
				gap: 12,
			}}
		>
			<div>
				<div style={{ fontWeight: 700, fontSize: 18, color: "#101828" }}>
					{title}
				</div>
				<div style={{ color: "#565656", fontSize: 15, marginTop: 4 }}>
					{desc}
				</div>
			</div>
			<button
				type="button"
				onClick={onClick}
				disabled={disabled}
				style={{
					background: btnColor,
					color: "white",
					border: "none",
					borderRadius: 8,
					padding: "8px 14px",
					fontSize: 15,
					fontWeight: 700,
					cursor: disabled ? "not-allowed" : "pointer",
					opacity: disabled ? 0.5 : 1,
					whiteSpace: "nowrap",
				}}
			>
				{btnLabel}
			</button>
		</div>
	);
}

const cardStyle: React.CSSProperties = {
	background: "white",
	border: "1px solid #dadada",
	borderRadius: 15,
	overflow: "hidden",
	boxShadow: "0 4px 4px rgba(0,0,0,0.1)",
};

const cardHeaderStyle: React.CSSProperties = {
	background: "var(--color-ink-50)",
	borderBottom: "1px solid #dadada",
	padding: "18px 24px",
	display: "flex",
	alignItems: "center",
	gap: 12,
	fontWeight: 700,
	fontSize: 20,
	color: "#000",
};

const headerCardStyle: React.CSSProperties = {
	background: "white",
	border: "1px solid #dadada",
	borderRadius: 15,
	padding: "20px 32px",
	display: "flex",
	alignItems: "center",
	justifyContent: "space-between",
	boxShadow: "0 4px 4px rgba(0,0,0,0.1)",
};

const backBtnStyle: React.CSSProperties = {
	background: "transparent",
	border: "none",
	cursor: "pointer",
	color: "#565656",
	padding: 4,
	display: "flex",
	alignItems: "center",
};

const modalTextareaStyle: React.CSSProperties = {
	background: "#f3f3f5",
	border: "none",
	borderRadius: 8,
	padding: "8px 12px",
	fontSize: 15,
	color: "#0a0a0a",
	width: "100%",
	minHeight: 80,
	resize: "vertical",
	outline: "none",
	fontFamily: "inherit",
};
