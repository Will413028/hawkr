"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	useMutation as useConnectMutation,
	useTransport,
} from "@connectrpc/connect-query";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { PaymentAdminService } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	type CreateMeasureRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import {
	KYCStatus,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Button, DeadlineLine, StatusBadge } from "@/components/UI";
import MerchantDelegationAuthorizationPanel from "@/features/delegated-operations/components/MerchantDelegationAuthorizationPanel";
import MerchantOperationEntry from "@/features/delegated-operations/components/MerchantOperationEntry";
import {
	codeLabel,
	ORDER_STATUS_LABEL,
	OrderStatus,
	PRODUCT_STATUS_LABEL,
	SHIPPING_METHOD_LABEL,
} from "@/lib/codemeta";
import { type ActionResult, runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { isKycApproved } from "@/lib/kyc-status";
import { formatProductPriceRange } from "@/lib/product-price";
import { getStatusMeta } from "@/lib/product-status";
import { unavailableLabels } from "@/lib/unavailability";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { useScrollToHash } from "@/lib/use-scroll-to-hash";
import { api } from "@/services/api";
import { imageUri } from "@/utils/image";
import { buildImageUrl } from "@/utils/media";
import { kycBadge } from "../store-row";
import {
	ActiveMeasureCards,
	ScheduledTerminationNotice,
} from "./ActiveMeasuresCard";
import CreateMeasureDialog from "./CreateMeasureDialog";
import CreateRiskMeasureDialog from "./CreateRiskMeasureDialog";
import CreateTerminationDialog from "./CreateTerminationDialog";
import EndMeasureDialog from "./EndMeasureDialog";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "./GovernanceNotice";
import {
	KycBasicInfoCard,
	KycDocumentsCard,
	KycOtherInfoCard,
	KycRejectionNotice,
	KycReviewHistoryCard,
} from "./KycApplicationCards";
import {
	APPROVE_HINT_TEXT,
	APPROVE_SUCCESS_TEXT,
	buildRejectRequest,
	documentChoices,
	draftAfter,
	EMPTY_REVIEW_DRAFT,
	issuableFieldOptions,
	KYC_CONFLICT_TEXT,
	type PreviewImage,
	REJECT_SUCCESS_TEXT,
	REQUERY_SUCCESS_TEXT,
	type ReviewDraft,
	reviewActions,
	reviewBlockedLabel,
	writeRequeryResult,
} from "./kyc-application";
import MeasureHistoryCard from "./MeasureHistoryCard";
import {
	MEASURE_CONFLICT_TEXT,
	terminationAvailability,
	terminationCreatedText,
} from "./measure";
import { OnboardingSummaryCard } from "./OnboardingSummaryCard";
import { PaymentAccountSection } from "./PaymentAccountCard";
import ProductForceUnpublishDialog from "./ProductForceUnpublishDialog";
import RejectKycDialog from "./RejectKycDialog";
import SellerAccountDeleteDialog from "./SellerAccountDeleteDialog";
import {
	closerLabel,
	END_SUCCESS_NEUTRAL_TEXT,
	endSuccessText,
	lifecycleBadge,
	MEASURE_CREATED_TEXT,
	VIOLATION_MEASURE_CREATED_TEXT,
} from "./store-detail";
import {
	useAccountEmail,
	useKycApplication,
	useNowUntil,
	usePaymentAccount,
	useStoreDetail,
	useStoreOnboarding,
	useStoreRating,
} from "./use-store-detail";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/MerchantCertificationDetail.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

interface ProductItem {
	id: string;
	name: string;
	imageUrl: string;
	priceMin: string;
	priceMax: string;
	rating: string;
	reviewCount: string;
	status: string;
	categoryName: string;
	createdAt: string;
}

interface OrderItem {
	orderNo: string;
	createTime: string;
	buyerNickname: string;
	/** 買家頭像 objectKey — SSOT，顯示走 buildImageUrl（49 §6.1；扁平成對不帶主體前綴）。
	 *  gateway 另送 `avatarUrl`（固定 `/i/original/` 原圖），本頁要的是 avatar preset，不讀它。 */
	avatarKey: string;
	itemCount: string;
	totalActualAmount: string;
	shippingMethod: string;
	status: string;
	statusSummary: string;
}

const getOrderStatusDisplay = (
	status: string,
	statusSummary: string,
): {
	label: string;
	variant: "success" | "gray" | "warning" | "danger";
	desc: string;
} => ({
	label: codeLabel(ORDER_STATUS_LABEL, status),
	variant: orderStatusVariant(status),
	desc: statusSummary || "",
});

const orderStatusVariant = (
	status: string,
): "success" | "gray" | "warning" | "danger" => {
	switch (status) {
		case OrderStatus.PENDING_SHIPMENT:
			return "warning";
		case OrderStatus.IN_TRANSIT:
		case OrderStatus.WAITING_PICKUP:
			return "success";
		case OrderStatus.SHIPPING_ABNORMAL:
		case OrderStatus.PARTIALLY_REFUNDED:
		case OrderStatus.FULLY_REFUNDED:
			return "danger";
		default:
			return "gray";
	}
};

/**
 * 商家詳情。外層以店家 ID 當 key：換一家店時整頁的狀態（審核草稿、顯示全文、對話框、提示）一起重來，
 * 不會把上一家店的明文或輸入帶到下一家。
 */
export default function MerchantCertificationDetail() {
	const params = useParams<{ id: string }>();
	return <MerchantDetail key={params.id} storeId={params.id} />;
}

function MerchantDetail({ storeId }: { storeId: string }) {
	const storeIdStr = storeId;
	const router = useRouter();

	const queryClient = useQueryClient();
	const transport = useTransport();
	const [draft, setDraft] = useState<ReviewDraft>(EMPTY_REVIEW_DRAFT);
	const [showRejectModal, setShowRejectModal] = useState(false);
	const [previewImage, setPreviewImage] = useState<PreviewImage | null>(null);
	const [showCreateMeasureModal, setShowCreateMeasureModal] = useState(false);
	const [showCreateViolationMeasure, setShowCreateViolationMeasure] =
		useState(false);
	const [endTargetId, setEndTargetId] = useState<string | null>(null);
	const [showTerminationDialog, setShowTerminationDialog] = useState(false);
	const [showDeleteAccountDialog, setShowDeleteAccountDialog] = useState(false);
	const [unpublishTarget, setUnpublishTarget] = useState<ProductItem | null>(
		null,
	);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const noticeRef = useRef<HTMLDivElement>(null);
	const [activeTab, setActiveTab] = useState<
		"basic" | "products" | "orders" | "finance" | "delegation"
	>("basic");

	const storeQuery = useStoreDetail(storeIdStr);
	const detail = storeQuery.data;
	const store = detail?.store;
	useScrollToHash(!storeQuery.isLoading && !!store);
	const accountEmail = useAccountEmail(store?.accountId);

	const kycSummary = detail?.kyc;
	const isApproved = isKycApproved(kycSummary?.status);
	const rating = useStoreRating(store?.storeCode, isApproved);
	const kycQuery = useKycApplication(kycSummary?.applicationId);
	const application =
		kycQuery.value.state === "ready" ? kycQuery.value.application : undefined;
	const paymentAccount = usePaymentAccount(store?.id);
	const onboarding = useStoreOnboarding(store?.id);
	const reviewable = reviewActions(application);
	const blockedLabel = reviewBlockedLabel(application);
	const pendingCheck = application?.pendingRegistrationCheck;
	const deadlineNow = useNow(LIST_REFRESH_MS);
	const now = useNowUntil(
		pendingCheck?.deadlineTime
			? timestampDate(pendingCheck.deadlineTime)
			: undefined,
	);
	const { data: products = [], isLoading: productsLoading } = useQuery({
		queryKey: ["merchants", "detail", storeIdStr, "products"],
		queryFn: async () => {
			const res = await api.get<{
				code: number;
				data: { records: ProductItem[] };
			}>(
				`/api/v1/admin/products?owner_id=${store?.accountId}&page=1&page_size=50`,
			);
			return res?.code === 0 && res?.data?.records ? res.data.records : [];
		},
		enabled: !!store?.accountId && activeTab === "products",
	});

	const { data: orders = [], isLoading: ordersLoading } = useQuery({
		queryKey: ["merchants", "detail", storeIdStr, "orders"],
		queryFn: async () => {
			const res = await api.get<{
				code: number;
				data: { records: OrderItem[] };
			}>(
				`/api/v1/admin/orders?store_id=${store?.accountId}&page=1&page_size=50`,
			);
			return res?.code === 0 && res?.data?.records ? res.data.records : [];
		},
		enabled: !!store?.accountId && activeTab === "orders",
	});

	const totalFromPage = (
		res: { code: number; data?: { total?: string } } | null,
	) => {
		if (res?.code !== 0) return null;
		const n = Number(res?.data?.total);
		return Number.isFinite(n) ? n : null;
	};

	const { data: activeProductTotal = null } = useQuery({
		queryKey: ["merchants", "detail", storeIdStr, "activeProductTotal"],
		queryFn: async () => {
			const res = await api.get<{
				code: number;
				data: { total: string };
			}>(
				`/api/v1/admin/products?owner_id=${store?.accountId}&status=4004&page=1&page_size=1`,
			);
			return totalFromPage(res);
		},
		enabled: !!store?.accountId,
	});

	const { data: orderTotal = null } = useQuery({
		queryKey: ["merchants", "detail", storeIdStr, "orderTotal"],
		queryFn: async () => {
			const res = await api.get<{
				code: number;
				data: { total: string };
			}>(
				`/api/v1/admin/orders?store_id=${store?.accountId}&page=1&page_size=1`,
			);
			return totalFromPage(res);
		},
		enabled: !!store?.accountId,
	});

	const invalidateDetail = () =>
		queryClient.invalidateQueries({
			queryKey: ["merchants", "detail", storeIdStr],
		});

	const forceUnpublishMutation = useMutation({
		mutationFn: (productId: string) =>
			api.put(`/api/v1/admin/products/${productId}/status`, {
				status: "4003",
			}),
		onSuccess: invalidateDetail,
	});

	const invalidateStore = () => {
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStore,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerGovernanceAdminService.method.listMeasures,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerGovernanceAdminService.method.getMeasure,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.listStores,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStoreStats,
				cardinality: undefined,
			}),
		});
	};

	const createMeasureMutation = useConnectMutation(
		SellerGovernanceAdminService.method.createMeasure,
		{ onSuccess: invalidateStore },
	);
	const endMeasureMutation = useConnectMutation(
		SellerGovernanceAdminService.method.endMeasure,
		{ onSuccess: invalidateStore },
	);

	const invalidateKyc = () => {
		invalidateStore();
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: PaymentAdminService.method.getPaymentAccount,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStoreOnboarding,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getKYCApplication,
				cardinality: undefined,
			}),
		});
	};
	const approveMutation = useConnectMutation(
		SellerAdminService.method.approveKYC,
		{ onSuccess: invalidateKyc },
	);
	const rejectMutation = useConnectMutation(
		SellerAdminService.method.rejectKYC,
		{ onSuccess: invalidateKyc },
	);
	const requeryMutation = useConnectMutation(
		SellerAdminService.method.requeryKYCRegistration,
	);
	const submitting = approveMutation.isPending || rejectMutation.isPending;

	const deleteAccountMutation = useConnectMutation(
		IdentityAdminService.method.deleteAccount,
	);

	const openPreview = (src: string, alt: string) =>
		setPreviewImage({ src, alt });
	const closePreview = () => setPreviewImage(null);

	useEffect(() => {
		if (notice) noticeRef.current?.scrollIntoView({ block: "nearest" });
	}, [notice]);

	const openDialog = (open: () => void) => {
		setNotice(null);
		open();
	};

	const handleForceUnpublish = async () => {
		if (!unpublishTarget) return;
		const target = unpublishTarget;
		await forceUnpublishMutation.mutateAsync(target.id);
		setUnpublishTarget(null);
		setNotice({
			tone: "success",
			text: `已強制下架「${target.name}」，買家端立即看不到這件商品；賣家仍可自行重新上架。`,
		});
	};

	const handleDeleteAccount = async (reason: string) => {
		if (!store?.accountId) {
			throw new Error("帳號 ID 還沒載入，請重新整理頁面後再試。");
		}
		const res = await deleteAccountMutation.mutateAsync({
			accountId: store.accountId,
			reason,
		});
		return res.state;
	};

	const handleDeleteDone = (state: DeleteAccountResponse_DeletionState) => {
		setShowDeleteAccountDialog(false);
		if (state === DeleteAccountResponse_DeletionState.DELETED) {
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: SellerAdminService.method.listStores,
					cardinality: undefined,
				}),
			});
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: SellerAdminService.method.getStoreStats,
					cardinality: undefined,
				}),
			});
			router.push("/admin/merchants");
			return;
		}
		invalidateStore();
	};

	const handleCreateMeasure = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		await createMeasureMutation.mutateAsync(request);
		setShowCreateMeasureModal(false);
		setNotice({ tone: "success", text: MEASURE_CREATED_TEXT });
	};

	const handleCreateViolationMeasure = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		await createMeasureMutation.mutateAsync(request);
		setShowCreateViolationMeasure(false);
		setNotice({ tone: "success", text: VIOLATION_MEASURE_CREATED_TEXT });
	};

	const handleCreateTermination = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		const res = await createMeasureMutation.mutateAsync(request);
		setShowTerminationDialog(false);
		setNotice({ tone: "success", text: terminationCreatedText(res.measure) });
	};

	const handleEndMeasure = async (measure: Measure) => {
		const result = await runAction(
			() =>
				endMeasureMutation.mutateAsync({ id: measure.id, etag: measure.etag }),
			{
				reload: () => storeQuery.refetch(),
				fallback: "結束措施失敗，賣家仍在停權中；請確認網路狀態後再試一次。",
				conflictText: MEASURE_CONFLICT_TEXT,
			},
		);
		if (!result.ok) {
			if (result.kind === "conflict") {
				setEndTargetId(null);
				setNotice({ tone: "error", text: result.text });
				return;
			}
			throw new Error(result.text);
		}
		setEndTargetId(null);
		const after = await storeQuery.refetch();
		setNotice({
			tone: "success",
			text:
				after.isError || !after.data
					? END_SUCCESS_NEUTRAL_TEXT
					: endSuccessText(after.data.activeMeasures),
		});
	};

	const handleApprove = async () => {
		if (!application || !reviewable.canApprove) return;
		setNotice(null);
		const result = await runAction(
			() =>
				approveMutation.mutateAsync({
					id: application.id,
					etag: application.etag,
				}),
			{
				reload: () => kycQuery.query.refetch(),
				fallback: "核准失敗，申請狀態沒有變動；請確認網路狀態後再試一次。",
				conflictText: KYC_CONFLICT_TEXT,
			},
		);
		setDraft((d) => draftAfter(d, result));
		setNotice(
			result.ok
				? { tone: "success", text: APPROVE_SUCCESS_TEXT }
				: { tone: "error", text: result.text },
		);
	};

	const handleReject = async (): Promise<ActionResult> => {
		if (!application || !reviewable.canReject) {
			return {
				ok: false,
				kind: "message",
				text: "這份申請目前不是待審核狀態，無法退回。",
				noResponse: false,
			};
		}
		const result = await runAction(
			() => rejectMutation.mutateAsync(buildRejectRequest(application, draft)),
			{
				reload: () => kycQuery.query.refetch(),
				fallback: "退回失敗，申請狀態沒有變動；請確認網路狀態後再試一次。",
				conflictText: KYC_CONFLICT_TEXT,
			},
		);
		setDraft((d) => draftAfter(d, result));
		if (result.ok) {
			setShowRejectModal(false);
			setNotice({ tone: "success", text: REJECT_SUCCESS_TEXT });
		}
		return result;
	};

	const handleRequery = async () => {
		if (!application) return;
		setNotice(null);
		try {
			const res = await requeryMutation.mutateAsync({ id: application.id });
			writeRequeryResult(
				queryClient,
				transport,
				application.id,
				res.registrationCheck,
			);
			setNotice({ tone: "success", text: REQUERY_SUCCESS_TEXT });
		} catch (caught) {
			setNotice({
				tone: "error",
				text: connectErrorText(caught, "重新查詢失敗，請稍後再試一次。"),
			});
		}
	};

	const formatDate = (dateStr: string | null) => {
		if (!dateStr) return "-";
		return dateStr.substring(0, 16).replace("T", " ");
	};

	const loading = storeQuery.isLoading;
	if (loading) {
		return (
			<div className="loading-container">
				<div className="loading-spinner"></div>
				<p>載入中...</p>
			</div>
		);
	}

	if (storeQuery.isError || !detail || !store) {
		const error = storeQuery.error;
		const notFound =
			!storeQuery.isError ||
			(error instanceof ConnectError && error.code === Code.NotFound);
		return (
			<div className="error-container">
				<MaterialSymbol name="error" />
				<p>
					{notFound
						? "找不到商家資料"
						: `讀取失敗：${error instanceof Error ? error.message : "未知錯誤"}`}
				</p>
				<div style={{ display: "flex", gap: 8, justifyContent: "center" }}>
					{!notFound && (
						<Button variant="outline" onClick={() => void storeQuery.refetch()}>
							重試
						</Button>
					)}
					<Button
						variant="primary"
						onClick={() => router.push("/admin/merchants")}
					>
						返回列表
					</Button>
				</div>
			</div>
		);
	}

	const activeMeasures = detail.activeMeasures;
	const endTarget = activeMeasures.find(
		(m) => m.id === endTargetId && m.canEnd,
	);
	const closed = store.status === StoreStatus.CLOSED;
	const termination = terminationAvailability(detail);
	const lifecycle = lifecycleBadge(store.status);
	const logo = imageUri(store.logo, "avatar");
	const statusBadge = kycBadge(kycSummary?.status);
	const isRejected = application?.status === KYCStatus.KYC_STATUS_REJECTED;
	const anyDialogOpen =
		showRejectModal ||
		showCreateMeasureModal ||
		showCreateViolationMeasure ||
		endTarget !== undefined ||
		showTerminationDialog ||
		showDeleteAccountDialog ||
		unpublishTarget !== null;

	return (
		<div className="certification-detail-page">

			<div className="detail-header">
				<div className="header-left">
					<button
						type="button"
						className="btn-back"
						aria-label="返回列表"
						onClick={() => router.push("/admin/merchants")}
					>
						<svg
							width="24"
							height="24"
							viewBox="0 0 24 24"
							fill="none"
							aria-hidden="true"
						>
							<path
								d="M15 18L9 12L15 6"
								stroke="currentColor"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							/>
						</svg>
					</button>
					<div className="merchant-avatar-group">
						<div className="merchant-avatar">
							{logo ? (
								<Image
									src={logo}
									alt={store.storeName}
									width={80}
									height={80}
									unoptimized
									onError={(e) => {
										(e.target as HTMLImageElement).style.display = "none";
									}}
								/>
							) : (
								<div className="avatar-placeholder">
									<span>{store.storeName?.charAt(0) || "店"}</span>
								</div>
							)}
						</div>
						<div className="header-info">
							<div className="header-title-row">
								<h1 className="merchant-name">{store.storeName}</h1>
								<StatusBadge variant={statusBadge.variant}>
									{statusBadge.label}
								</StatusBadge>
								{lifecycle && (
									<StatusBadge variant={lifecycle.variant}>
										{lifecycle.label}
									</StatusBadge>
								)}
							</div>
							<div className="header-meta">
								<span>ID: {store.accountId}</span>
								{store.storeCode && (
									<>
										<span className="meta-separator">·</span>
										<span>商店代號：{store.storeCode}</span>
									</>
								)}
								{store.id && (
									<>
										<span className="meta-separator">·</span>
										<span>店鋪 ID：{store.id}</span>
									</>
								)}
								{accountEmail.value.state === "loading" && (
									<>
										<span className="meta-separator">·</span>
										<span>登入信箱：讀取中…</span>
									</>
								)}
								{accountEmail.value.state === "error" && (
									<>
										<span className="meta-separator">·</span>
										<span>
											登入信箱：無法讀取{" "}
											<button
												type="button"
												className="btn-reset"
												onClick={accountEmail.retry}
											>
												重試
											</button>
										</span>
									</>
								)}
								{accountEmail.value.state === "notFound" && (
									<>
										<span className="meta-separator">·</span>
										<span>登入信箱：找不到登入帳號</span>
									</>
								)}
								{accountEmail.value.state === "ready" &&
									accountEmail.value.email && (
										<>
											<span className="meta-separator">·</span>
											<span>登入信箱：{accountEmail.value.email}</span>
										</>
									)}
								{closed && (
									<>
										<span className="meta-separator">·</span>
										<span>關閉者：{closerLabel(store.closure)}</span>
										{store.closeTime && (
											<>
												<span className="meta-separator">·</span>
												<span>
													關閉時間：
													{formatDateTimeShort(timestampDate(store.closeTime))}
												</span>
											</>
										)}
									</>
								)}
								{kycSummary?.submitTime && (
									<>
										<span className="meta-separator">·</span>
										<span>
											提交時間：
											{formatDateTimeShort(
												timestampDate(kycSummary.submitTime),
											)}
										</span>
										<span className="meta-separator">·</span>
										<span>
											<DeadlineLine
												label="審核期限"
												deadline={kycSummary.decideDeadline}
												now={deadlineNow}
											/>
										</span>
									</>
								)}
							</div>
						</div>
					</div>
				</div>
				<div className="header-actions">

					<MerchantOperationEntry
						storeId={store.id}
						enabled
						resolveEligibility
					/>
					{blockedLabel && (
						<span className="kyc-review-blocked" role="status">
							{blockedLabel}
						</span>
					)}
					{reviewable.canReject && (
						<button
							type="button"
							className="btn-reject-figma"
							onClick={() => openDialog(() => setShowRejectModal(true))}
							disabled={submitting}
						>
							退回申請
						</button>
					)}
					{reviewable.canApprove && (
						<div
							style={{
								display: "flex",
								flexDirection: "column",
								alignItems: "flex-end",
								gap: "4px",
							}}
						>
							<span style={{ fontSize: "12px", color: "#8a8a8a" }}>
								{APPROVE_HINT_TEXT}
							</span>
							<button
								type="button"
								className="btn-approve-figma"
								onClick={handleApprove}
								disabled={submitting}
							>
								通過審核
							</button>
						</div>
					)}

					{detail.canCreateMeasure && (
						<button
							type="button"
							className="btn-suspend-seller"
							onClick={() =>
								openDialog(() => setShowCreateViolationMeasure(true))
							}
							disabled={createMeasureMutation.isPending}
						>
							<MaterialSymbol name="crisis_alert" /> 建立措施
						</button>
					)}

					{detail.canCreateMeasure && (
						<button
							type="button"
							className="btn-suspend-seller"
							onClick={() => openDialog(() => setShowCreateMeasureModal(true))}
							disabled={createMeasureMutation.isPending}
						>
							<MaterialSymbol name="block" /> 建立風險措施
						</button>
					)}

					{termination.canCreate && (
						<button
							type="button"
							className="btn-close-store"
							onClick={() => openDialog(() => setShowTerminationDialog(true))}
							disabled={createMeasureMutation.isPending}
						>
							<MaterialSymbol name="storefront" /> 終止合作
						</button>
					)}

					{accountEmail.value.state === "ready" && (
						<button
							type="button"
							className="btn-delete-account"
							onClick={() => openDialog(() => setShowDeleteAccountDialog(true))}
							disabled={deleteAccountMutation.isPending}
						>
							<MaterialSymbol name="delete" /> 刪除帳號
						</button>
					)}
				</div>
			</div>


			{!anyDialogOpen && (
				<GovernanceNoticeBanner notice={notice} ref={noticeRef} />
			)}


			<ScheduledTerminationNotice accountId={store.accountId} />
			<ActiveMeasureCards
				active={activeMeasures}
				endPending={endMeasureMutation.isPending}
				onEnd={(measure) => openDialog(() => setEndTargetId(measure.id))}
			/>


			{isApproved && (
				<>

					{(orderTotal !== null ||
						activeProductTotal !== null ||
						rating !== undefined) && (
						<div className="approved-stats-row">
							{orderTotal !== null && (
								<div className="approved-stat-card">
									<div className="approved-stat-icon orange">
										<MaterialSymbol name="list_alt" />
									</div>
									<div>
										<div className="approved-stat-label">總訂單數</div>
										<div className="approved-stat-value">{orderTotal}</div>
									</div>
								</div>
							)}
							{activeProductTotal !== null && (
								<div className="approved-stat-card">
									<div className="approved-stat-icon primary">
										<MaterialSymbol name="shopping_bag" />
									</div>
									<div>
										<div className="approved-stat-label">上架商品</div>
										<div className="approved-stat-value">
											{activeProductTotal}
										</div>
									</div>
								</div>
							)}
							{rating !== undefined && (
								<div className="approved-stat-card">
									<div className="approved-stat-icon orange">
										<MaterialSymbol name="star" />
									</div>
									<div>
										<div className="approved-stat-label">賣家評價</div>
										<div className="approved-stat-value">{rating}</div>
									</div>
								</div>
							)}
						</div>
					)}
					<div className="approved-tabs">
						<button
							type="button"
							className={`approved-tab${activeTab === "basic" ? " active" : ""}`}
							onClick={() => setActiveTab("basic")}
						>
							基本資料
						</button>
						<button
							type="button"
							className={`approved-tab${activeTab === "products" ? " active" : ""}`}
							onClick={() => setActiveTab("products")}
						>
							商品管理
						</button>
						<button
							type="button"
							className={`approved-tab${activeTab === "orders" ? " active" : ""}`}
							onClick={() => setActiveTab("orders")}
						>
							歷史訂單
						</button>
						<button
							type="button"
							className={`approved-tab${activeTab === "finance" ? " active" : ""}`}
							onClick={() => setActiveTab("finance")}
						>
							財務設定
						</button>
						<button
							type="button"
							className={`approved-tab${activeTab === "delegation" ? " active" : ""}`}
							onClick={() => setActiveTab("delegation")}
						>
							代行授權
						</button>
					</div>
				</>
			)}


			{(!isApproved || activeTab === "basic") && reviewable.canReject && (
				<div className="audit-notice">
					<div className="audit-notice-icon">
						<MaterialSymbol name="error" fill />
					</div>
					<div className="audit-notice-content">
						<div className="audit-notice-title">審核備註</div>
						<div className="audit-notice-text">
							重點核對<span className="highlight">營業登記資料</span>與
							<span className="highlight">統一編號</span>是否一致
						</div>
					</div>
				</div>
			)}
			{(!isApproved || activeTab === "basic") && isRejected && application && (
				<KycRejectionNotice application={application} />
			)}


			{(!isApproved || activeTab === "basic") && (
				<OnboardingSummaryCard
					state={onboarding.value}
					onReload={() => void onboarding.query.refetch()}
				/>
			)}


			{(!isApproved || activeTab === "basic") && (
				<PaymentAccountSection
					storeId={store.id}
					payment={paymentAccount}
					onNotice={setNotice}
				/>
			)}


			{(!isApproved || activeTab === "basic") && (
				<div id="kyc" className="detail-content-grid">

					<div className="detail-left-column">
						{application ? (
							<KycBasicInfoCard
								store={store}
								application={application}
								now={now}
								canRequery={reviewable.canRequery}
								requerying={requeryMutation.isPending}
								onRequery={() => void handleRequery()}
							/>
						) : (
							<div
								style={{
									padding: "40px",
									textAlign: "center",
									color: "#6B7280",
									background: "white",
									borderRadius: "15px",
									border: "1px solid #dadada",
								}}
							>
								<MaterialSymbol
									name="list_alt"
									style={{
										fontSize: "48px",
										marginBottom: "12px",
										display: "block",
									}}
								/>
								<p>
									{kycQuery.value.state === "loading"
										? "申請內容載入中…"
										: kycQuery.value.state === "error"
											? "申請內容讀取失敗"
											: "此商家尚未提交認證申請"}
								</p>
								{kycQuery.value.state === "error" && (
									<Button
										variant="outline"
										onClick={() => void kycQuery.query.refetch()}
									>
										重試
									</Button>
								)}
							</div>
						)}
					</div>


					{application && (
						<div className="detail-right-column">
							<KycDocumentsCard
								application={application}
								onPreview={openPreview}
							/>
							<KycOtherInfoCard store={store} application={application} />
							<KycReviewHistoryCard application={application} />
						</div>
					)}
				</div>
			)}


			{(!isApproved || activeTab === "basic") && (
				<MeasureHistoryCard accountId={store.accountId} />
			)}


			{isApproved && activeTab === "products" && (
				<div className="tab-content-card">
					{productsLoading ? (
						<div className="tab-empty-state">
							<div className="loading-spinner"></div>
							<span>載入商品中...</span>
						</div>
					) : products.length > 0 ? (
						<table className="tab-table">
							<thead>
								<tr>
									<th>照片</th>
									<th>商品名稱</th>
									<th>價格</th>
									<th>評分</th>
									<th>狀態</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								{products.map((p) => {
									const statusMeta = getStatusMeta(p.status);
									return (
										<tr key={p.id}>
											<td>
												<Image
													src={
														buildImageUrl(p.imageUrl, "thumb") ||
														"/images/placeholder-product.png"
													}
													alt={p.name}
													className="product-thumb"
													width={80}
													height={60}
													unoptimized
													onError={(e) => {
														(e.target as HTMLImageElement).src =
															"/images/placeholder-product.png";
													}}
												/>
											</td>
											<td>{p.name}</td>
											<td>{formatProductPriceRange(p.priceMin, p.priceMax)}</td>
											<td>
												{p.rating || "-"} ({p.reviewCount || 0})
											</td>
											<td>
												<StatusBadge variant={statusMeta.badgeVariant}>
													{codeLabel(PRODUCT_STATUS_LABEL, p.status)}
												</StatusBadge>
											</td>
											<td>
												<div className="tab-table-actions">

													<button
														type="button"
														className="btn-force-unpublish"
														onClick={() =>
															openDialog(() => setUnpublishTarget(p))
														}
														disabled={p.status === "4003"}
													>
														強制下架
													</button>
													<button
														type="button"
														className="btn-view-item"
														onClick={() =>
															router.push(`/admin/products/${p.id}`)
														}
													>
														查看商品
													</button>
												</div>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					) : (
						<div className="tab-empty-state">
							<MaterialSymbol name="shopping_bag" />
							<span>暫無商品</span>
						</div>
					)}
				</div>
			)}


			{isApproved && activeTab === "orders" && (
				<div className="tab-content-card">
					{ordersLoading ? (
						<div className="tab-empty-state">
							<div className="loading-spinner"></div>
							<span>載入訂單中...</span>
						</div>
					) : orders.length > 0 ? (
						<table className="tab-table">
							<thead>
								<tr>
									<th>訂單編號</th>
									<th>訂單時間</th>
									<th>買家</th>
									<th>商品數量</th>
									<th>訂單金額</th>
									<th>物流</th>
									<th>訂單狀態</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								{orders.map((o) => {
									const statusDisplay = getOrderStatusDisplay(
										o.status,
										o.statusSummary,
									);
									return (
										<tr key={o.orderNo}>
											<td>{o.orderNo}</td>
											<td>{o.createTime ? formatDate(o.createTime) : "-"}</td>
											<td>
												<div className="order-buyer">
													<div className="order-buyer-avatar">
														{o.avatarKey ? (
															<Image
																src={buildImageUrl(o.avatarKey, "avatar")}
																alt={o.buyerNickname}
																width={36}
																height={36}
																unoptimized
																onError={(e) => {
																	(e.target as HTMLImageElement).style.display =
																		"none";
																}}
															/>
														) : (
															<span>{(o.buyerNickname || "?").charAt(0)}</span>
														)}
													</div>
													<span>{o.buyerNickname || "-"}</span>
												</div>
											</td>
											<td style={{ textAlign: "center" }}>{o.itemCount}</td>
											<td>
												NT${Number(o.totalActualAmount || 0).toLocaleString()}
											</td>
											<td>
												<div className="order-logistics">
													<span>
														{codeLabel(SHIPPING_METHOD_LABEL, o.shippingMethod)}
													</span>
												</div>
											</td>
											<td>
												<div className="order-status-cell">
													<StatusBadge variant={statusDisplay.variant}>
														{statusDisplay.label}
													</StatusBadge>
													{statusDisplay.desc && (
														<span className="order-status-desc">
															{statusDisplay.desc}
														</span>
													)}
												</div>
											</td>
											<td>
												<button type="button" className="btn-view-item">
													查看訂單
												</button>
											</td>
										</tr>
									);
								})}
							</tbody>
						</table>
					) : (
						<div className="tab-empty-state">
							<MaterialSymbol name="list_alt" />
							<span>暫無訂單</span>
						</div>
					)}
				</div>
			)}


			{isApproved && activeTab === "delegation" && (
				<div className="tab-content-card">
					<MerchantDelegationAuthorizationPanel
						storeId={store.id}
						storeName={store.storeName}
					/>
				</div>
			)}


			{isApproved && activeTab === "finance" && (
				<div className="tab-content-card">
					<div className="tab-empty-state">
						<MaterialSymbol name="paid" />
						<span>財務設定功能開發中</span>
					</div>
				</div>
			)}


			{showRejectModal && application && (
				<RejectKycDialog
					draft={draft}
					onDraftChange={setDraft}
					options={issuableFieldOptions(application)}
					choices={documentChoices(application)}
					blockedReason={
						reviewable.canReject
							? undefined
							: unavailableLabels(application.rejectUnavailabilities)[0]
					}
					onClose={() => setShowRejectModal(false)}
					onConfirm={handleReject}
				/>
			)}


			{showCreateMeasureModal && (
				<CreateRiskMeasureDialog
					storeId={store.id}
					onClose={() => setShowCreateMeasureModal(false)}
					onConfirm={handleCreateMeasure}
				/>
			)}


			{showCreateViolationMeasure && (
				<CreateMeasureDialog
					storeId={store.id}
					onClose={() => setShowCreateViolationMeasure(false)}
					onConfirm={handleCreateViolationMeasure}
				/>
			)}


			{endTarget && (
				<EndMeasureDialog
					measure={endTarget}
					onClose={() => setEndTargetId(null)}
					onConfirm={() => handleEndMeasure(endTarget)}
				/>
			)}


			{unpublishTarget && (
				<ProductForceUnpublishDialog
					productName={unpublishTarget.name}
					productId={unpublishTarget.id}
					statusLabel={codeLabel(PRODUCT_STATUS_LABEL, unpublishTarget.status)}
					onClose={() => setUnpublishTarget(null)}
					onConfirm={handleForceUnpublish}
				/>
			)}


			{showDeleteAccountDialog && (
				<SellerAccountDeleteDialog
					storeName={store.storeName}
					accountEmail={accountEmail.value}
					accountId={store.accountId}
					onClose={() => setShowDeleteAccountDialog(false)}
					onConfirm={handleDeleteAccount}
					onDone={handleDeleteDone}
				/>
			)}


			{showTerminationDialog && (
				<CreateTerminationDialog
					storeId={store.id}
					accountId={store.accountId}
					options={termination.options}
					onClose={() => setShowTerminationDialog(false)}
					onConfirm={handleCreateTermination}
				/>
			)}


			{previewImage && (
				<button
					type="button"
					className="btn-reset lightbox-overlay"
					aria-label="關閉"
					onClick={closePreview}
				>
					<span className="lightbox-close" aria-hidden="true">
						<MaterialSymbol name="close" />
					</span>
					<Image
						src={previewImage.src}
						alt={previewImage.alt}
						className="lightbox-image"
						width={800}
						height={600}
						unoptimized
					/>
					<span className="lightbox-caption">{previewImage.alt}</span>
				</button>
			)}
		</div>
	);
}
