"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type {
	KYCApplication,
	KYCDocument,
	Store,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { KYCField } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Image from "next/image";
import { type ReactNode, useState } from "react";
import { Button, StatusBadge } from "@/components/UI";
import { formatDateTimeShort } from "@/lib/format";
import {
	documentSrc,
	documentsOf,
	gcisMarkLabel,
	gcisView,
	kycFieldLabel,
	lastRejection,
	reviewActionLabel,
	reviewerLabel,
	sellerTypeLabel,
	serverIssueView,
} from "./kyc-application";
import { addressText } from "./store-detail";

// 商家詳情的商家認證申請內容（GetKYCApplication）：基本資料（公司型含商業登記 GCIS 並列）、檢附文件與審核紀錄。每個元件只吃已經整理好的 props，沒有自己的查詢。退回時指出問題在 RejectKycDialog。

/** 一個欄位列：標籤與值（沿用 info-field 的樣式）。 */
function FieldRow({
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

// ---------------------------------------------------------------------------
// 基本資料與商業登記（GCIS）並列

/**
 * 基本資料：店家欄位（GetStore）與申請的欄位。公司型申請在同一張卡的下半部把申請內容與商業登記（GCIS）逐列並列：名稱、統編、
 * 地址、設立日期，再加組織型態與登記現況（D4 第 4 點）。不一致標紅色、無法比對標灰色；右欄欄頭是查詢時間與結果，查詢中顯示
 * 「登記資料查詢中…」與最晚的完成時間；登記現況不是核准設立時顯示警示徽章。「重新查詢」只在審核中的公司型申請出現（canRequery）。
 * 沿用卡片與欄位列的樣式（info-grid、info-field），個人型申請沒有並列。
 */
export function KycBasicInfoCard({
	store,
	application,
	now,
	canRequery,
	requerying,
	onRequery,
}: {
	store: Store;
	application: KYCApplication;
	/** 判斷進行中的查詢是否已超過期限用的現在時間（由頁面注入）。 */
	now: Date;
	canRequery: boolean;
	requerying: boolean;
	onRequery: () => void;
}) {
	const view = gcisView(application, now);
	const pending = view?.header.kind === "pending";
	return (
		<div className="detail-card">
			<div className="section-title">
				<MaterialSymbol name="storefront" />
				<span>基本資料</span>
			</div>
			<div className="card-body">
				<div className="info-grid">
					<FieldRow label="用戶類型" value={sellerTypeLabel(application)} />
					<FieldRow label="店鋪名稱" value={store.storeName || "-"} />
					<FieldRow label="電子信箱" value={store.supportEmail || "-"} />
					<FieldRow label="聯絡人姓名" value={store.contactName || "-"} />
					<FieldRow label="聯絡手機" value={store.contactMobile || "-"} />
					{/* 聯絡電話：退件後要打電話請賣家補件。「賣家沒留電話」本身就是審核者要知道的事，所以沒有值也顯示 "-"。 */}
					<FieldRow label="聯絡電話" value={store.contactPhone || "-"} />
					{/* 公司電話是企業賣家限定且選填；個人賣家沒有這個欄位，整欄不顯示而不是顯示空值。 */}
					{application.subject.case === "business" && (
						<FieldRow
							label="公司電話"
							value={application.subject.value.companyPhone || "-"}
						/>
					)}
					<FieldRow
						label="主要商品類型"
						value={application.productType || "-"}
						fullWidth
					/>
				</div>

				{view && (
					<div className="kyc-gcis">
						<div className="info-grid kyc-gcis-head">
							<div className="kyc-gcis-col">
								<strong>申請內容</strong>
							</div>
							<div className="kyc-gcis-col">
								<strong>商業登記（GCIS）</strong>
								<div
									className={`kyc-gcis-summary kyc-gcis-summary--${view.header.kind}`}
									role="status"
								>
									{view.header.text}
								</div>
								{view.header.detail && (
									<div className="kyc-gcis-summary">{view.header.detail}</div>
								)}
								{view.header.previous && (
									<div className="kyc-gcis-summary">
										上一筆：{view.header.previous}
									</div>
								)}
								{canRequery && (
									<div className="kyc-gcis-requery">
										<Button
											variant="outline"
											onClick={onRequery}
											disabled={pending}
											loading={requerying}
										>
											{pending ? "查詢中…" : "重新查詢"}
										</Button>
									</div>
								)}
							</div>
						</div>
						<div className="info-grid">
							{view.rows.map((row) => (
								<GcisRowCells
									key={row.key}
									label={row.label}
									applicant={row.applicant}
									registry={
										row.key === "registration_status" && view.warning ? (
											<StatusBadge variant="warning">
												{view.warning}
											</StatusBadge>
										) : (
											row.registry
										)
									}
									mark={row.mark}
								/>
							))}
						</div>
					</div>
				)}
			</div>
		</div>
	);
}

/** 並列的一列：左格是申請的值、右格是登記的值與比對標記（兩格都是 info-field）。 */
function GcisRowCells({
	label,
	applicant,
	registry,
	mark,
}: {
	label: string;
	applicant: string;
	registry: ReactNode;
	mark?: "mismatch" | "unverifiable";
}) {
	return (
		<>
			<div className="info-field">
				<div className="field-label">{label}</div>
				<div className="field-value">{applicant}</div>
			</div>
			<div className="info-field">
				<div className="field-label">{label}（登記）</div>
				<div className="field-value">
					{registry}
					{mark && (
						<>
							{" "}
							<StatusBadge variant={mark === "mismatch" ? "danger" : "gray"}>
								{gcisMarkLabel(mark)}
							</StatusBadge>
						</>
					)}
				</div>
			</div>
		</>
	);
}

// ---------------------------------------------------------------------------
// 檢附文件

/** 一張公司補充文件影像：經同源 /api/bff 讀取（點圖放大，載入失敗可開原檔）。 */
function DocumentImage({
	document,
	alt,
	width,
	height,
	onPreview,
}: {
	document: KYCDocument;
	alt: string;
	width: number;
	height: number;
	onPreview: (src: string, alt: string) => void;
}) {
	const [failed, setFailed] = useState(false);
	const src = documentSrc(document.viewUri);
	if (!src) {
		return (
			<div className="id-card-placeholder">
				<MaterialSymbol name="image" />
				<span>無法檢視</span>
			</div>
		);
	}
	if (failed) {
		return (
			<div className="id-card-placeholder">
				<MaterialSymbol name="image" />
				<span>載入失敗</span>
				<a
					href={src}
					target="_blank"
					rel="noopener noreferrer"
					className="document-link"
				>
					<MaterialSymbol name="open_in_new" /> 開啟原檔
				</a>
			</div>
		);
	}
	return (
		<button
			type="button"
			className="btn-reset kyc-document-open"
			onClick={() => onPreview(src, alt)}
			aria-label={`放大檢視${alt}`}
		>
			<Image
				src={src}
				alt={alt}
				width={width}
				height={height}
				unoptimized
				onError={() => setFailed(true)}
			/>
		</button>
	);
}

/** 公司補充文件。 */
export function KycDocumentsCard({
	application,
	onPreview,
}: {
	application: KYCApplication;
	onPreview: (src: string, alt: string) => void;
}) {
	const companyDocs = documentsOf(
		application,
		KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
	);
	return (
		<div className="detail-card">
			<div className="section-title">
				<MaterialSymbol name="content_copy" />
				<span>檢附文件</span>
			</div>
			<div className="card-body">
				{companyDocs.length > 0 && (
					<div className="document-list">
						{companyDocs.map((doc) => {
							const src = documentSrc(doc.viewUri);
							const alt = `${kycFieldLabel(doc.field)} ${doc.position + 1}`;
							return (
								<div className="document-item" key={doc.assetId}>
									<div className="document-image">
										<DocumentImage
											document={doc}
											alt={alt}
											width={120}
											height={80}
											onPreview={onPreview}
										/>
									</div>
									<div className="document-info">
										<div className="document-info-text">
											<span className="document-number">{alt}</span>
											{src && (
												<a
													href={src}
													target="_blank"
													rel="noopener noreferrer"
													className="document-link"
												>
													<MaterialSymbol name="open_in_new" /> 查看原圖
												</a>
											)}
										</div>
									</div>
								</div>
							);
						})}
					</div>
				)}

				{companyDocs.length === 0 && (
					<div className="empty-state-small">
						<MaterialSymbol name="content_copy" />
						<span>暫無檢附文件</span>
					</div>
				)}
			</div>
		</div>
	);
}

// ---------------------------------------------------------------------------
// 其他資訊

export function KycOtherInfoCard({
	store,
	application,
}: {
	store: Store;
	application: KYCApplication;
}) {
	return (
		<div className="detail-card">
			<div className="section-title">
				<MaterialSymbol name="info" />
				<span>其他資訊</span>
			</div>
			<div className="card-body">
				<div className="info-grid">
					<FieldRow
						label="送貨地址"
						value={addressText(store.address)}
						fullWidth
					/>
					{store.prize && (
						<FieldRow label="獎項" value={store.prize} fullWidth />
					)}
					<div className="info-field full-width">
						<div className="field-label">官網</div>
						<div className="field-value">
							{application.websiteUrl ? (
								<a
									href={application.websiteUrl}
									target="_blank"
									rel="noopener noreferrer"
								>
									{application.websiteUrl}
								</a>
							) : (
								<StatusBadge variant="gray">未提供</StatusBadge>
							)}
						</div>
					</div>
				</div>
			</div>
		</div>
	);
}

// ---------------------------------------------------------------------------
// 退回的問題與審核紀錄

/** 一份問題清單（欄位、原因、說明；指定文件時註明）。 */
function IssueList({
	issues,
}: {
	issues: readonly {
		field: string;
		code: string;
		message: string;
		document?: string;
	}[];
}) {
	return (
		<ul className="kyc-issue-list">
			{issues.map((i, index) => (
				// 清單是唯讀的、順序固定，沒有穩定的 ID 可用，順序即身分。
				<li key={index}>
					<strong>{i.field}</strong>
					{i.document && <span>（{i.document}）</span>}
					<StatusBadge variant="danger">{i.code}</StatusBadge>
					<span className="kyc-issue-message">{i.message}</span>
				</li>
			))}
		</ul>
	);
}

/**
 * 被退回的申請：最近一次退回的總理由，加上賣家還沒處理的問題（GetKYCApplication.outstanding_issues，由伺服器判定已處理與否）；
 * 全部處理完、還沒重新送出時說明這一點。沿用審核備註的紅色提示樣式。
 */
export function KycRejectionNotice({
	application,
}: {
	application: KYCApplication;
}) {
	const reason = lastRejection(application.reviews)?.reason;
	const outstanding = application.outstandingIssues.map(serverIssueView);
	return (
		<div className="audit-notice rejected">
			<div className="audit-notice-icon">
				<MaterialSymbol name="error" fill />
			</div>
			<div className="audit-notice-content">
				<div className="audit-notice-title">審核失敗</div>
				<div className="audit-notice-text">
					失敗原因：{reason || "審核未通過，請重新上傳"}
					{outstanding.length > 0 ? (
						<>
							<div>賣家尚未處理的問題：</div>
							<IssueList issues={outstanding} />
						</>
					) : (
						<div>賣家已處理退回的所有項目，還沒有重新送出。</div>
					)}
				</div>
			</div>
		</div>
	);
}

/** 審核紀錄（舊到新存放，這裡新的在上）：動作、審核者 ID（只有 ID，K16）、時間、理由、逐項問題與內部備註（只有後台看得到）。 */
export function KycReviewHistoryCard({
	application,
}: {
	application: KYCApplication;
}) {
	const reviews = [...application.reviews].reverse();
	if (reviews.length === 0) return null;
	return (
		<div className="detail-card">
			<div className="section-title">
				<MaterialSymbol name="history" />
				<span>審核紀錄</span>
			</div>
			<div className="card-body">
				<ol className="kyc-review-list">
					{reviews.map((review) => (
						<li key={review.id} className="kyc-review-item">
							<div className="kyc-review-head">
								<strong>{reviewActionLabel(review.action)}</strong>
								<span>{reviewerLabel(review)}</span>
								<span>
									{review.decideTime
										? formatDateTimeShort(timestampDate(review.decideTime))
										: "-"}
								</span>
							</div>
							{review.reason && (
								<div className="kyc-review-reason">{review.reason}</div>
							)}
							{review.issues.length > 0 && (
								<IssueList issues={review.issues.map(serverIssueView)} />
							)}
							{review.internalNote && (
								<div className="kyc-review-fields">
									內部備註：{review.internalNote}
								</div>
							)}
							{review.registrationCheckQueryTime && (
								<div className="kyc-review-fields">
									依據的登記資料查詢：
									{formatDateTimeShort(
										timestampDate(review.registrationCheckQueryTime),
									)}
								</div>
							)}
						</li>
					))}
				</ol>
			</div>
		</div>
	);
}
