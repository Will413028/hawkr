"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import {
	FoodRegistrationDecision,
	FoodRegistrationEndCause,
	FoodRegistrationState,
	SellerAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Button, DeadlineLine, StatusBadge } from "@/components/UI";
import { connectErrorText, errorMessage } from "@/lib/error-text";
import {
	decideBlockedLabel,
	decideTimeText,
	endCauseLabel,
	endTimeText,
	FADENBOOK_LINK_TEXT,
	FADENBOOK_URL,
	REVOKE_BUTTON_TEXT,
	stateBadge,
	subjectRows,
	submitTimeText,
} from "@/lib/food-registration";
import { formatDateTimeShort } from "@/lib/format";
import { kycStatusText } from "@/lib/kyc-status";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "../../merchants/[id]/GovernanceNotice";
import { REASON_MAX_LENGTH } from "../../merchants/[id]/store-detail";
import RejectFoodRegistrationDialog from "./RejectFoodRegistrationDialog";
import RevokeFoodRegistrationDialog from "./RevokeFoodRegistrationDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

function Field({
	label,
	value,
	fullWidth,
}: {
	label: string;
	value: string;
	fullWidth?: boolean;
}) {
	return (
		<div className={`info-field${fullWidth ? " full-width" : ""}`}>
			<div className="field-label">{label}</div>
			<div className="field-value">{value}</div>
		</div>
	);
}

export default function FoodRegistrationDetail() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const queryClient = useQueryClient();
	const now = useNow(LIST_REFRESH_MS);
	const [showReject, setShowReject] = useState(false);
	const [showRevoke, setShowRevoke] = useState(false);
	const [note, setNote] = useState("");
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);

	const detail = useQuery(SellerAdminService.method.getFoodRegistration, {
		id,
	});
	const invalidateRegistration = () => {
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getFoodRegistration,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.listFoodRegistrations,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStore,
				cardinality: undefined,
			}),
		});
	};
	const decide = useMutation(SellerAdminService.method.decideFoodRegistration, {
		onSuccess: invalidateRegistration,
	});
	const revoke = useMutation(SellerAdminService.method.revokeFoodRegistration, {
		onSuccess: invalidateRegistration,
	});
	const registration = detail.data?.registration;

	const back = (
		<button
			type="button"
			className="btn-back"
			aria-label="返回列表"
			onClick={() => router.push("/admin/food-registrations")}
		>
			<MaterialSymbol name="keyboard_arrow_left" />
		</button>
	);

	if (detail.isLoading) {
		return (
			<div className="certification-detail-page">
				<p role="status">載入中...</p>
			</div>
		);
	}
	if (detail.error || !registration) {
		return (
			<div className="certification-detail-page">
				<div className="detail-header">
					<div className="header-left">{back}</div>
				</div>
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>
						{detail.error
							? errorMessage(detail.error)
							: "無法讀取這筆食品業者登錄"}
					</div>
				</div>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const badge = stateBadge(registration.state);
	// 不能裁定的原因只讀伺服器（approve_unavailabilities、reject_unavailabilities）：第一個原因是結構性的（不在待審）時沒有入口、不說明；
	// 其餘顯示伺服器第一個原因的句子（店家已關閉、等待刪除、商家認證還沒核准）；伺服器列出全部原因，畫面只顯示第一個。
	const blockedLabel = decideBlockedLabel(registration);
	const subject = subjectRows(registration);
	const deciding = decide.isPending;

	const runDecision = async (
		decision: FoodRegistrationDecision,
		reason: string,
		internalNote: string,
	) => {
		await decide.mutateAsync({ id, decision, reason, internalNote });
		setNotice({
			tone: "success",
			text:
				decision === FoodRegistrationDecision.APPROVE
					? "已核准，新的字號已成為店家公開的字號。"
					: "已退回，賣家會看到退回理由。",
		});
		setNote("");
	};

	const runRevoke = async (reason: string) => {
		await revoke.mutateAsync({ id, reason });
		setNotice({
			tone: "success",
			text: "已撤銷，店面與商品頁不再顯示這個字號，賣家會看到給賣家的理由。",
		});
	};

	const noteTooLong = note.trim().length > REASON_MAX_LENGTH;

	const handleApprove = async () => {
		if (noteTooLong) {
			setNotice({
				tone: "error",
				text: `內部備註最多 ${REASON_MAX_LENGTH} 字。`,
			});
			return;
		}
		setNotice(null);
		try {
			await runDecision(FoodRegistrationDecision.APPROVE, "", note.trim());
		} catch (caught) {
			setNotice({
				tone: "error",
				text: connectErrorText(
					caught,
					"核准失敗，登錄沒有變動；請稍後再試一次。",
				),
			});
		}
	};

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">食品業者登錄審核</h1>
							<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
						</div>
						<div className="header-meta">
							<span>{registration.storeName || "-"}</span>
							<span className="meta-separator">·</span>
							<Link href={`/admin/merchants/${registration.storeId}`}>
								查看店家
							</Link>
						</div>
					</div>
				</div>
			</div>

			<GovernanceNoticeBanner notice={notice} />

			<div className="detail-card">
				<div className="section-title">
					<MaterialSymbol name="verified" />
					<span>登錄內容</span>
				</div>
				<div className="card-body">
					<div className="info-grid">
						<Field label="送出的字號" value={registration.registrationNumber} />
						<Field label="送出時間" value={submitTimeText(registration)} />
						<Field
							label="店家目前核准的字號"
							value={registration.approved?.registrationNumber || "尚無"}
						/>
						{registration.approved?.decideTime && (
							<Field
								label="核准時間"
								value={formatDateTimeShort(
									timestampDate(registration.approved.decideTime),
								)}
							/>
						)}
					</div>
					<p>
						<DeadlineLine
							label="裁定期限"
							deadline={registration.decideDeadline}
							now={now}
						/>
					</p>
					<p className="payment-desc">
						<a href={FADENBOOK_URL} target="_blank" rel="noopener noreferrer">
							{FADENBOOK_LINK_TEXT}
							<MaterialSymbol name="open_in_new" />
						</a>
					</p>
				</div>
			</div>

			<div className="detail-card">
				<div className="section-title">
					<MaterialSymbol name="person_search" />
					<span>商家認證</span>
				</div>
				<div className="card-body">
					<div className="info-grid">
						<Field
							label="認證狀態"
							value={kycStatusText(registration.kycStatus)}
						/>
						{subject.map((row) => (
							<Field key={row.label} label={row.label} value={row.value} />
						))}
					</div>
					{subject.length === 0 && (
						<p className="payment-desc">店家還沒有送出商家認證申請。</p>
					)}
				</div>
			</div>

			{registration.state !== FoodRegistrationState.PENDING && (
				<div className="detail-card">
					<div className="section-title">
						<MaterialSymbol name="list_alt" />
						<span>處理紀錄</span>
					</div>
					<div className="card-body">
						<div className="info-grid">
							{registration.deciderAdminId && (
								<Field
									label="裁定的管理員"
									value={registration.deciderAdminId}
								/>
							)}
							{registration.decideTime && (
								<Field label="裁定時間" value={decideTimeText(registration)} />
							)}
							{registration.rejectReason && (
								<Field
									label="退回理由"
									value={registration.rejectReason}
									fullWidth
								/>
							)}
							{registration.endReason && (
								<Field
									label="給賣家的理由"
									value={registration.endReason}
									fullWidth
								/>
							)}
							{registration.revokerAdminId && (
								<Field
									label="撤銷的管理員"
									value={registration.revokerAdminId}
								/>
							)}
							{registration.endTime && (
								<Field label="結束時間" value={endTimeText(registration)} />
							)}
							{registration.endCause !==
								FoodRegistrationEndCause.UNSPECIFIED && (
								<Field
									label="結束原因"
									value={endCauseLabel(registration.endCause)}
								/>
							)}
							{registration.internalNote && (
								<Field
									label={
										registration.state === FoodRegistrationState.APPROVED ||
										registration.state === FoodRegistrationState.ENDED
											? "核准時的內部註記"
											: "內部備註"
									}
									value={registration.internalNote}
									fullWidth
								/>
							)}
						</div>
					</div>
				</div>
			)}

			{/* 兩個動作都不能做而伺服器給了原因（店家已關閉、等待刪除）時，裁定區只有標題與第一個原因的句子：沒有備註欄、沒有鍵（Will 2026-10-22）。
			    至少一個動作能做時維持原本的版面。 */}
			{!registration.canApprove &&
				!registration.canReject &&
				blockedLabel !== undefined && (
					<div className="detail-card">
						<div className="section-title">
							<MaterialSymbol name="verified_user" />
							<span>裁定</span>
						</div>
						<div className="card-body">
							<p className="payment-desc" role="status">
								{blockedLabel}
							</p>
						</div>
					</div>
				)}
			{(registration.canApprove || registration.canReject) && (
				<div className="detail-card">
					<div className="section-title">
						<MaterialSymbol name="verified_user" />
						<span>裁定</span>
					</div>
					<div className="card-body">
						<div className="adm-operation-field adm-operation-field--wide">
							<label htmlFor="food-registration-note">內部備註（選填）</label>
							<textarea
								id="food-registration-note"
								value={note}
								onChange={(event) => setNote(event.target.value)}
								rows={2}
								placeholder="例如：個人賣家，僅核對有效"
								disabled={deciding}
							/>
							<span className="adm-operation-field-note">
								只給後台看，最多 {REASON_MAX_LENGTH}{" "}
								字；核准或退回時會一併記下。
							</span>
						</div>
						{blockedLabel && (
							<p className="payment-desc" role="status">
								{blockedLabel}
							</p>
						)}
						<div className="header-actions">
							<Button
								variant="primary"
								onClick={() => void handleApprove()}
								disabled={!registration.canApprove || deciding}
								loading={deciding}
							>
								核准
							</Button>
							<Button
								variant="danger"
								onClick={() => setShowReject(true)}
								disabled={!registration.canReject || deciding || noteTooLong}
							>
								退回
							</Button>
						</div>
					</div>
				</div>
			)}

			{registration.canRevoke && (
				<div className="detail-card">
					<div className="section-title">
						<MaterialSymbol name="cancel" />
						<span>撤銷</span>
					</div>
					<div className="card-body">
						<div className="header-actions">
							<Button
								variant="danger"
								onClick={() => setShowRevoke(true)}
								disabled={revoke.isPending}
							>
								{REVOKE_BUTTON_TEXT}
							</Button>
						</div>
					</div>
				</div>
			)}

			{showRevoke && (
				<RevokeFoodRegistrationDialog
					storeName={registration.storeName}
					registrationNumber={registration.registrationNumber}
					onClose={() => setShowRevoke(false)}
					onConfirm={async (reason) => {
						await runRevoke(reason);
						setShowRevoke(false);
					}}
				/>
			)}

			{showReject && (
				<RejectFoodRegistrationDialog
					storeName={registration.storeName}
					registrationNumber={registration.registrationNumber}
					onClose={() => setShowReject(false)}
					onConfirm={async (reason) => {
						await runDecision(
							FoodRegistrationDecision.REJECT,
							reason,
							note.trim(),
						);
						setShowReject(false);
					}}
				/>
			)}
		</div>
	);
}
