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
import {
	type CreateMeasureRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import {
	KYCStatus,
	PayUniBindStatus,
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
	SHIPPING_TEMPERATURE_LABEL,
} from "@/lib/codemeta";
import { type ActionResult, runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { formatProductPriceRange } from "@/lib/product-price";
import { getStatusMeta } from "@/lib/product-status";
import { unavailableLabels } from "@/lib/unavailability";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { useScrollToHash } from "@/lib/use-scroll-to-hash";
import { api } from "@/services/api";
import { imageUri } from "@/utils/image";
import { buildImageUrl } from "@/utils/media";
import { flowBadge } from "../store-row";
import {
	ActiveMeasureCards,
	ScheduledTerminationNotice,
} from "./ActiveMeasuresCard";
import CreateMeasureDialog from "./CreateMeasureDialog";
import CreateRiskMeasureDialog from "./CreateRiskMeasureDialog";
import CreateTerminationDialog from "./CreateTerminationDialog";
import EndMeasureDialog from "./EndMeasureDialog";
import { FoodRegistrationCard } from "./FoodRegistrationCard";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "./GovernanceNotice";
import {
	KycBasicInfoCard,
	KycDocumentsCard,
	KycIdentityCard,
	KycOtherInfoCard,
	KycRejectionNotice,
	KycRevealBar,
	KycReviewHistoryCard,
} from "./KycApplicationCards";
import {
	APPROVE_SUCCESS_TEXT,
	activeReveal,
	buildRejectRequest,
	documentChoices,
	draftAfter,
	EMPTY_REVIEW_DRAFT,
	issuableFieldOptions,
	KYC_CONFLICT_TEXT,
	REBIND_SUCCESS_TEXT,
	REJECT_SUCCESS_TEXT,
	REQUERY_SUCCESS_TEXT,
	type ReviewDraft,
	revealFrom,
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
import { PayUniContactCard, PayUniStatusCard } from "./PayUniCards";
import ProductForceUnpublishDialog from "./ProductForceUnpublishDialog";
import RejectKycDialog from "./RejectKycDialog";
import RevealKycDialog from "./RevealKycDialog";
import {
	closePreview as closePreviewState,
	EMPTY_REVEAL_STATE,
	expireReveal,
	hideReveal,
	openPreview as openPreviewState,
	type RevealState,
	scheduleRevealExpiry,
	showReveal,
} from "./reveal-state";
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
	usePayUniRegistration,
	useStoreDetail,
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
	deliveryTemp: string;
	status: string;
	statusSummary: string;
}

// Order status -> display label + badge variant + description
// `GET /api/v1/admin/orders` 的 `status` 是 **OrderStatus 的數字 Value**（"0" / "10" /
// … / "80"），見 backend `gateway/internal/handler/admin_order.go` 的 AdminOrder.Status
// 註解——gateway 直接透傳後端碼值，不做任何翻譯。
//
// 這裡以前 switch 的是 "pending" / "processing" / "shipping" 這類英文字面值，那組值域
// **全鏈路從未存在過**，所以每一筆都落進 default、把碼值原樣印在畫面上（管理員看到的是
// 「35」而不是「物流異常」）。標籤一律走生成的 ORDER_STATUS_LABEL，不再自建對照表。
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

// Badge 配色是純呈現層判斷，不屬於 CodeMeta（後端不下發顏色），所以留在本檔手寫。
// 但它 switch 的是**生成物裡的值物件**而非字面值——值域改動會在型別層現形。
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
	// sellers.store_id surrogate（自 merchants 列表頁導覽帶入，見 merchants/page.tsx
	// 的 router.push(`/admin/merchants/${m.storeId}`)）——不是登入用的 accountId，
	// 兩者在賣家重開店後會分岔。店家資料、停權與 KYC 用這個值（SellerAdminService 的 store_id）；
	// 需要 account_id 的（商品與訂單的 owner_id 篩選、刪除帳號）一律吃 store.accountId。
	const storeIdStr = storeId;
	const router = useRouter();

	const queryClient = useQueryClient();
	const transport = useTransport();
	// 審核員正在填的內容（退回的逐項問題、總理由與內部備註）。送出失敗（含版本衝突）時整份保留，成功才清空（draftAfter）。
	const [draft, setDraft] = useState<ReviewDraft>(EMPTY_REVIEW_DRAFT);
	const [showRejectModal, setShowRejectModal] = useState(false);
	const [showRevealDialog, setShowRevealDialog] = useState(false);
	// 顯示全文的結果與放大中的證件影像只在這裡（瀏覽器記憶體）：到期、收起或離開頁面就一起沒了（reveal-state.ts），不寫進任何儲存。
	const [secret, setSecret] = useState<RevealState>(EMPTY_REVEAL_STATE);
	// 建立風險措施、建立終止合作、措施詳情、結束措施、刪除帳號、退回、顯示全文的對話框各自在獨立元件（它們的 a11y 三件套與輸入狀態在那裡）。
	// 結束措施記要結束的措施 id，不記整個措施：對話框每次都顯示最新讀到的那一筆（etag 也是），重讀之後不會拿舊的 etag 送出。
	const [showCreateMeasureModal, setShowCreateMeasureModal] = useState(false);
	const [showCreateViolationMeasure, setShowCreateViolationMeasure] =
		useState(false);
	const [endTargetId, setEndTargetId] = useState<string | null>(null);
	const [showTerminationDialog, setShowTerminationDialog] = useState(false);
	// 刪除賣家帳號 / 強制下架商品：兩個治理對話框，各自的目標即開關
	const [showDeleteAccountDialog, setShowDeleteAccountDialog] = useState(false);
	const [unpublishTarget, setUnpublishTarget] = useState<ProductItem | null>(
		null,
	);
	// 本頁唯一的操作結果承載處。開啟任何確認框時清空，所以確認框開著時它只可能是
	// 那一個確認框自己的錯誤——不會有上一個動作的殘留訊息跟著被朗讀第二次。
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const noticeRef = useRef<HTMLDivElement>(null);
	const [activeTab, setActiveTab] = useState<
		"basic" | "products" | "orders" | "finance" | "delegation"
	>("basic");

	// 店家（SellerAdminService.GetStore）。頁面的 loading／error／找不到以它為準；已關閉的店照樣回傳。
	const storeQuery = useStoreDetail(storeIdStr);
	const detail = storeQuery.data;
	const store = detail?.store;
	useScrollToHash(!storeQuery.isLoading && !!store);
	// 登入信箱（K6）：頁首與刪帳號確認框共用。
	const accountEmail = useAccountEmail(store?.accountId);

	// 商家認證的申請內容（GetKYCApplication；申請 ID 來自 GetStore.kyc）。它失敗不擋整頁：KYC 區塊各自顯示錯誤與重試。
	// 沒有送出過的申請（GetStore 沒有 kyc 摘要）時不發請求，也不顯示 KYC 區塊。
	const kycSummary = detail?.kyc;
	// 「已通過」看 KYC 的人工決定（KYCStatus.APPROVED，與清單的「已核准」同義）；PayUni 的結果另看 PayUni 卡。
	const isApproved = kycSummary?.status === KYCStatus.KYC_STATUS_APPROVED;
	// 評分統計卡：與買家看到的店面同一個來源（公開 GetStore 的 StoreStats）；讀不到（含已關閉、未公開）就沒有值，那張卡不渲染。
	const rating = useStoreRating(store?.storeCode, isApproved);
	const kycQuery = useKycApplication(kycSummary?.applicationId);
	const application =
		kycQuery.value.state === "ready" ? kycQuery.value.application : undefined;
	// PayUni 子商店註冊（GetPayUniRegistration）：第一次核准前是 not_found（尚未註冊）。
	const payUni = usePayUniRegistration(store?.id);
	const payUniRegistration =
		payUni.value.state === "ready" ? payUni.value.registration : undefined;
	// 核准、退回與重新查詢的可用性只讀伺服器（申請的 can_approve、can_reject、can_requery_registration；前提 2026-10-15）。
	// 核准不能做而且第一個原因不是結構性的（店家已關閉、等待刪除、帳號停權）時，不顯示按鈕，改顯示伺服器的第一個原因（blockedLabel）；
	// 伺服器列出全部原因，畫面只顯示第一個。
	const reviewable = reviewActions(application);
	const blockedLabel = reviewBlockedLabel(application);
	// 進行中的登記資料查詢超過伺服器保證的期限時要重新判斷（計時器在期限那一刻觸發一次渲染）。
	const pendingCheck = application?.pendingRegistrationCheck;
	const deadlineNow = useNow(LIST_REFRESH_MS);
	const now = useNowUntil(
		pendingCheck?.deadlineTime
			? timestampDate(pendingCheck.deadlineTime)
			: undefined,
	);
	// 綁定從 PENDING 走到終態時，GetStore 的流程狀態（kyc.flow_status，頁首的徽章）也要重讀。
	const bindStatus = payUniRegistration?.bindStatus;
	const previousBindStatus = useRef(bindStatus);
	useEffect(() => {
		if (
			previousBindStatus.current === PayUniBindStatus.PENDING &&
			bindStatus !== PayUniBindStatus.PENDING
		) {
			void storeQuery.refetch();
		}
		previousBindStatus.current = bindStatus;
	}, [bindStatus, storeQuery.refetch]);

	// 商品 / 訂單分頁懶載：用 enabled 取代「切到該 tab 才 fetch」的手刻 effect。
	// owner_id 是 admin_product.go SearchAdminProducts 自己解析的 filter：後端拿到
	// 這個值後呼叫 CurrentSellerAccountID(ownerID) 把「帳號」解析成現任 store
	// surrogate 再過濾——換句話說它要吃真實 account_id，不是 store_id，餵
	// storeIdStr 進去會被誤判成別的帳號 ID 或直接 ErrNoCurrentSeller（見
	// admin_product.go:29-56 的 @Param 說明與 h.SellerID.CurrentSellerAccountID
	// 呼叫）。用店家的 store.accountId，不是 URL 上的 storeIdStr。
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

	// /admin/orders 的實際 query 鍵是 store_id（order.go SearchOrders /
	// resolveOptionalSellerFilterQuery），且同 owner_id 一樣把值當 account_id
	// 解析成現任 store_id 再過濾——鍵名雖叫 store_id，值本身仍是原始
	// account_id，gateway 負責解析。2026-08 硬切自舊的 seller_account_id，
	// 後端已移除舊鍵 fallback：鍵名打 seller_account_id 會被後端 DefaultQuery
	// 忽略，filter 靜默失效變成回傳全平台訂單，而不是 404 或報錯。
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

	// KPI 卡的數字取列表端點的 total，不是上面兩支懶載清單的 records 長度：
	// 那兩支只在對應 tab 開啟時才 fetch（進頁預設 basic tab ⇒ 恆為 0），
	// 且被 page_size=50 夾住（106 筆會顯示成 50）。page_size=1 只為了取 total。
	// 取不到（請求失敗／非 0 code）一律回 null，對應的卡片不渲染——寧可少一張卡，
	// 不可渲染一個編出來的 0。
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
			// status=4004（啟用）＝卡片標籤「上架商品」的語意；不加 filter 會把草稿一起算進去。
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

	// 平台端能對單一商品做的治理動作只有「切狀態」：gateway 沒有任何 admin 刪除商品的
	// 路由——`DeleteProduct` 是賣家 scope 的 RPC。目標狀態 4003＝`ProductStatus.INACTIVE`
	// （CodeMeta Label「未上架」）。值域本身以 `@/lib/codemeta` 的 `ProductStatus` 為準，
	// 不在註解裡複述一份（複述過的那一版漏了 4002）。
	const forceUnpublishMutation = useMutation({
		mutationFn: (productId: string) =>
			api.put(`/api/v1/admin/products/${productId}/status`, {
				status: "4003",
			}),
		onSuccess: invalidateDetail,
	});

	// 店家的操作走 SellerAdminService（K10、K6）。成功後 SellerAdminService 的查詢整組失效：店家、停權紀錄、列表都會重讀。
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

	// 商家認證（K1、K3、K7）：核准與退回帶讀到的 etag；成功後店家（KYC 摘要）、申請、PayUni 註冊與列表一起重讀。
	const invalidateKyc = () => {
		invalidateStore();
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getKYCApplication,
				cardinality: undefined,
			}),
		});
		queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getPayUniRegistration,
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
	// 回應含明文：不讓它留在 mutation cache（gcTime 0），到期、收起時另外 reset()。
	const revealMutation = useConnectMutation(
		SellerAdminService.method.revealKYCSensitive,
		{ gcTime: 0 },
	);
	// 重新查詢成功不整筆重讀申請：回應就是那筆進行中的查詢（帶期限），直接寫進申請的快取（handleRequery），輪詢隨後接手。
	const requeryMutation = useConnectMutation(
		SellerAdminService.method.requeryKYCRegistration,
	);
	const rebindMutation = useConnectMutation(
		SellerAdminService.method.rebindPayUni,
		{ onSuccess: invalidateKyc },
	);
	const submitting = approveMutation.isPending || rejectMutation.isPending;

	// 刪除整個帳號（IdentityAdminService.DeleteAccount）。帳號 ID 一律是店家的 account_id，絕不能是 URL 上的 store_id。
	// 成功後不 invalidate 店家查詢：DELETED 時店家已不存在，重讀會把結果對話框底下的頁面換成錯誤畫面。
	const deleteAccountMutation = useConnectMutation(
		IdentityAdminService.method.deleteAccount,
	);

	const openPreview = (src: string, alt: string) =>
		setSecret((cur) => openPreviewState(cur, { src, alt }));
	const closePreview = () => setSecret((cur) => closePreviewState(cur));
	// 收起全文：明文、放大圖一起清掉，mutation 的回應資料也一起清（明文不留在 mutation data）。
	const hideSecret = () => {
		setSecret(hideReveal());
		revealMutation.reset();
	};

	// 顯示全文到期就收回成遮蔽值（K15、D4 第 5 點）：明文與已打開的放大圖一起清掉。計時器可能提早觸發或在背景分頁被延後，
	// 所以觸發時還沒到期就重排（scheduleRevealExpiry），回到前景時也再檢查一次。換店時整頁以 key 重新掛載，狀態一起沒了。
	const revealMutationReset = revealMutation.reset;
	useEffect(() => {
		const reveal = secret.reveal;
		if (!reveal) return;
		const schedule = scheduleRevealExpiry(
			reveal,
			() => {
				setSecret((cur) => expireReveal(cur, new Date()));
				revealMutationReset();
			},
			{
				now: () => new Date(),
				setTimer: (callback, ms) => setTimeout(callback, ms),
				clearTimer: (handle) =>
					clearTimeout(handle as ReturnType<typeof setTimeout>),
			},
		);
		const onVisible = () => {
			if (document.visibilityState === "visible") schedule.recheck();
		};
		document.addEventListener("visibilitychange", onVisible);
		return () => {
			schedule.cancel();
			document.removeEventListener("visibilitychange", onVisible);
		};
	}, [secret.reveal, revealMutationReset]);

	// 強制下架的按鈕在商品分頁的表格裡，而結果提示掛在頁首下方——不捲進來的話,
	// 使用者會在一個看不到任何回饋的位置以為操作沒有發生。`block: "nearest"` 讓
	// 已經在畫面上的提示不動。
	useEffect(() => {
		if (notice) noticeRef.current?.scrollIntoView({ block: "nearest" });
	}, [notice]);

	// 確認框開啟前一律清掉上一次的結果，見 notice state 的說明
	const openDialog = (open: () => void) => {
		setNotice(null);
		open();
	};

	// 兩個治理對話框把例外交還給呼叫端顯示（它們就地渲染在確認框內，不會被遮罩蓋住），
	// 所以這兩支 handler 不 catch。
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

	// 刪除帳號的結果（DELETED 或 PENDING_ORDERS）由對話框顯示；這裡只送出，回傳契約的狀態。
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
			// 店家已經不存在：列表與統計重讀，然後回到列表。
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
		// 帳號還有未完成的訂單：店家進入等待刪除，重讀顯示目前的狀態，留在本頁。
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

	// 建立終止合作（關聯帳號的終止在措施詳情頁建立）：成功提示用伺服器回的生效時間。
	const handleCreateTermination = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		const res = await createMeasureMutation.mutateAsync(request);
		setShowTerminationDialog(false);
		setNotice({ tone: "success", text: terminationCreatedText(res.measure) });
	};

	// 結束措施：帶讀到的 etag。版本衝突（別人先改過這個措施）時重讀店家，關掉對話框並在頁首說明；其他失敗丟回對話框就地顯示。
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
		// 結束一個措施不會結束帳號的其他措施：重讀後的成功訊息照實說（仍有其他措施時不能說「會恢復公開」）。
		// 重讀失敗時兩種說法都不能斷言，只說結束成功。
		const after = await storeQuery.refetch();
		setNotice({
			tone: "success",
			text:
				after.isError || !after.data
					? END_SUCCESS_NEUTRAL_TEXT
					: endSuccessText(after.data.activeMeasures),
		});
	};

	// 核准：帶讀到的 etag（K3）。版本衝突（別人先處理、或登記資料有新的查詢結果）時重讀申請，頁面上的勾選與理由照舊。
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

	// 退回：失敗時回傳給對話框就地顯示，問題、理由與備註整份保留（draftAfter）。
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

	// 顯示全文：寫稽核、授權 15 分鐘。失敗交回對話框顯示；成功才存下結果。
	const handleReveal = async (reason: string) => {
		if (!application) {
			throw new Error("申請內容還沒載入，請稍後再試。");
		}
		const res = await revealMutation.mutateAsync({
			id: application.id,
			reason,
		});
		const next = revealFrom(application.id, res);
		if (!next) {
			throw new Error(
				"伺服器沒有回傳全文的有效期限，所以不顯示全文；請稍後再試。",
			);
		}
		setSecret(showReveal(next));
		setShowRevealDialog(false);
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

	const handleRebind = async () => {
		if (!store?.id) return;
		setNotice(null);
		try {
			await rebindMutation.mutateAsync({ storeId: store.id });
			setNotice({ tone: "success", text: REBIND_SUCCESS_TEXT });
		} catch (caught) {
			setNotice({
				tone: "error",
				text: connectErrorText(caught, "重新綁定失敗，請稍後再試一次。"),
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
		// 404（查無此商家）語意上是 empty，不是 error：店家查詢回 NotFound（connect code 5）。
		// 其他錯誤是讀取失敗，給重試；沒有 store 也沒有錯誤時照找不到處理。
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

	// 店家的操作可用性（D4 第 13 點：已關閉時停權、解除、關店、刪帳號、核准與退回全部隱藏）。
	// 生效中的措施（GetStore.active_measures，與店家同一次讀取，帳號範圍）：每個措施一張卡；結束對話框對應的措施取最新讀到的那一筆，
	// 而且仍然 can_end 才開。
	const activeMeasures = detail.activeMeasures;
	const endTarget = activeMeasures.find(
		(m) => m.id === endTargetId && m.canEnd,
	);
	// 狀態的顯示（徽章與關店資訊），不是動作的可用性：動作能不能做都讀伺服器的旗標。
	const closed = store.status === StoreStatus.CLOSED;
	const termination = terminationAvailability(detail);
	const lifecycle = lifecycleBadge(store.status);
	const logo = imageUri(store.logo, "avatar");
	// 商家認證與 PayUni：沒有送出過的申請（GetStore 沒有 kyc 摘要）時沒有狀態徽章與 KYC 區塊。
	const statusBadge = kycSummary ? flowBadge(kycSummary.flowStatus) : undefined;
	// 顯示全文：過期的（或別份申請的）一律當作沒有；放大圖只在授權還在時顯示。
	const liveReveal = activeReveal(secret.reveal, application?.id, new Date());
	const previewImage = liveReveal ? secret.preview : null;
	const isRejected = application?.status === KYCStatus.KYC_STATUS_REJECTED;
	const anyDialogOpen =
		showRejectModal ||
		showRevealDialog ||
		showCreateMeasureModal ||
		showCreateViolationMeasure ||
		endTarget !== undefined ||
		showTerminationDialog ||
		showDeleteAccountDialog ||
		unpublishTarget !== null;

	return (
		<div className="certification-detail-page">
			{/* Header Card */}
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
								{kycSummary ? (
									<StatusBadge variant={statusBadge?.variant ?? "gray"}>
										{statusBadge?.label ?? "狀態不明"}
									</StatusBadge>
								) : (
									<StatusBadge variant="gray">未申請認證</StatusBadge>
								)}
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
								{payUniRegistration?.merId && (
									<>
										<span className="meta-separator">·</span>
										{/* `merId` 是 PayUni 的金流商家編號（`HAWK<store_id>`），與上面
											那個「商店代號」（`storeCode`）不是同一個東西 —— 兩者共用
											同一個標籤時，同一列會出現兩個「商店代號：」而值不同。 */}
										<span>金流商家編號：{payUniRegistration.merId}</span>
									</>
								)}
							</div>
						</div>
					</div>
				</div>
				<div className="header-actions">
					{/* 資格整個交給伺服器的 preflight（設計書 6.1 S-1：preflight 就是「入口
					    按鈕的可用性」，觸發點含詳情頁）。這一頁的 profile 沒有 `lifecycleStatus`，
					    而治理停權只寫 `sellers.status`、不寫 `seller_suspensions` 列 —— 修前
					    那組 `isApproved && suspensionConfirmed && !activeSuspension` 因此對治理
					    停權與關店完全失明，按鈕照樣可按、點下去才被 9272 擋；未開通治理權限的
					    管理員同理（9265）。在這裡補一份本地判準就是造第二份判準，而權威只有一份。 */}
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
								核准後系統會向 PayUni 註冊
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
					{/* 建立措施（違規依據的警告與停權）：入口條件同建立風險措施，都是伺服器的 can_create_measure */}
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
					{/* 建立風險措施：伺服器說能建立時顯示（帳號已有生效中的措施也可以再建立）；結束入口在下方紅色狀態卡 */}
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
					{/* 終止合作（條款第 21 條）：沒有直接關店的入口，平台關店一律經終止合作的措施；入口與事由由伺服器給（GetStore.can_create_termination），已關閉的店也可能有（第 21.3 條事由） */}
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
					{/* 刪除帳號：帳號層級的動作，不看店家是否關閉（從零：刪除請求權不看商店狀態）；帳號讀得到才顯示。無商家認證 gating；等待刪除的店仍可呼叫，契約回目前的狀態 */}
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

			{/* 操作結果：確認框開著時不在這裡渲染（會被遮罩蓋住，且同一段文字會被
			    輔助科技朗讀兩次），改由確認框自己就地承載。 */}
			{!anyDialogOpen && (
				<GovernanceNoticeBanner notice={notice} ref={noticeRef} />
			)}

			{/* 停權中 — 紅色狀態卡，一個生效中的措施一張（風險依據：類型/理由/時間/操作者 + 結束入口；違規依據：說明與連結，沒有結束） */}
			<ScheduledTerminationNotice accountId={store.accountId} />
			<ActiveMeasureCards
				active={activeMeasures}
				endPending={endMeasureMutation.isPending}
				onEnd={(measure) => openDialog(() => setEndTargetId(measure.id))}
			/>

			{/* 已通過 — 統計卡片 + Tab 列 */}
			{isApproved && (
				<>
					{/* 統計卡：每一張都必須有權威來源，算不出來的那張不渲染。
					    「總營收」曾是寫死的 NT$ 0——admin 面沒有任何營收聚合端點，
					    留著等於對管理員宣稱這家店零營收，故整張移除。 */}
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

			{/* Audit Notice Banner — 待審核 or 已退回 */}
			{(!isApproved || activeTab === "basic") && reviewable.canReject && (
				<div className="audit-notice">
					<div className="audit-notice-icon">
						<MaterialSymbol name="error" fill />
					</div>
					<div className="audit-notice-content">
						<div className="audit-notice-title">審核備註</div>
						<div className="audit-notice-text">
							重點核對「<span className="highlight">營業登記證</span>」與「
							<span className="highlight">負責人身分證</span>」、「
							<span className="highlight">統一編號</span>
							」是否一致，並確認所有證件皆在有效期內。
						</div>
					</div>
				</div>
			)}
			{(!isApproved || activeTab === "basic") && isRejected && application && (
				<KycRejectionNotice application={application} />
			)}

			{/* PayUni 狀態卡 — 獨立於 Hawkr KYC 核准狀態，approved 後仍顯示；比照本區其他區塊只在 basic tab 顯示。 */}
			{(!isApproved || activeTab === "basic") && (
				<PayUniStatusCard
					state={payUni.value}
					hasApplication={kycSummary !== undefined}
					rebinding={rebindMutation.isPending}
					onRebind={() => void handleRebind()}
					onRetry={() => void payUni.query.refetch()}
				/>
			)}

			{/* PayUni 聯絡資訊 — 店家自己的聯絡欄位（K9）；全空或還沒有註冊時整張卡不渲染。 */}
			{(!isApproved || activeTab === "basic") && (
				<PayUniContactCard store={store} registration={payUniRegistration} />
			)}

			{/* 食品業者登錄：核准的字號與待審的登錄（連到審核頁）。 */}
			{(!isApproved || activeTab === "basic") && (
				<FoodRegistrationCard store={store} />
			)}

			{/* 商家認證：顯示全文列。沒有送出過的申請時不顯示。 */}
			{(!isApproved || activeTab === "basic") && application && (
				<KycRevealBar
					reveal={liveReveal}
					onReveal={() => openDialog(() => setShowRevealDialog(true))}
					onHide={hideSecret}
				/>
			)}

			{/* Main Content: 2-column grid — basic info tab or non-approved view */}
			{(!isApproved || activeTab === "basic") && (
				<div id="kyc" className="detail-content-grid">
					{/* Left Column */}
					<div className="detail-left-column">
						{application ? (
							<>
								<KycBasicInfoCard
									store={store}
									application={application}
									now={now}
									canRequery={reviewable.canRequery}
									requerying={requeryMutation.isPending}
									onRequery={() => void handleRequery()}
								/>
								<KycIdentityCard
									application={application}
									reveal={liveReveal}
									onPreview={openPreview}
								/>
							</>
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

					{/* Right Column */}
					{application && (
						<div className="detail-right-column">
							<KycDocumentsCard
								application={application}
								reveal={liveReveal}
								onPreview={openPreview}
							/>
							<KycOtherInfoCard store={store} application={application} />
							<KycReviewHistoryCard application={application} />
						</div>
					)}
				</div>
			)}

			{/* 措施歷史：帳號的警告與停權措施（新到舊，上一頁／下一頁，不顯示總數）。 */}
			{(!isApproved || activeTab === "basic") && (
				<MeasureHistoryCard accountId={store.accountId} />
			)}

			{/* 商品管理 Tab */}
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
													{/* 已下架的商品沒有可下架的狀態可切，按了也只會送出一次無變化的
													    請求——維持可按等於留一顆什麼都不做的按鈕。 */}
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

			{/* 歷史訂單 Tab */}
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
									<th>物流/溫層</th>
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
											{/* shippingMethod／deliveryTemp 是 CodeMeta 的數字 Value（3001＝黑貓宅急便、
											    3201＝常溫），不是可讀字串。修前這裡直接把值印出來，畫面上就是
											    「3001」配一顆寫著「3201」的溫層徽章。 */}
											<td>
												<div className="order-logistics">
													<span>
														{codeLabel(SHIPPING_METHOD_LABEL, o.shippingMethod)}
													</span>
													{o.deliveryTemp && (
														<span className="temp-badge">
															{codeLabel(
																SHIPPING_TEMPERATURE_LABEL,
																o.deliveryTemp,
															)}
														</span>
													)}
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

			{/* 代行授權 Tab（backend#2444）。條件渲染即懶載：面板內的三支查詢隨掛載才發，
			    與 products / orders 用 `enabled` 達成的效果相同。
			    ⚠️ 傳 `store.id`（與 URL 上的 sellers.store_id 相同），不是 account_id：
			    授權列以 store_id 為鍵，兩者在賣家重開店後會分岔。 */}
			{isApproved && activeTab === "delegation" && (
				<div className="tab-content-card">
					<MerchantDelegationAuthorizationPanel
						storeId={store.id}
						storeName={store.storeName}
					/>
				</div>
			)}

			{/* 財務設定 Tab - placeholder */}
			{isApproved && activeTab === "finance" && (
				<div className="tab-content-card">
					<div className="tab-empty-state">
						<MaterialSymbol name="paid" />
						<span>財務設定功能開發中</span>
					</div>
				</div>
			)}

			{/* 退回申請（RejectKYC）。草稿（逐項問題、總理由、內部備註）由頁面持有，送出失敗時整份保留。 */}
			{showRejectModal && application && (
				<RejectKycDialog
					draft={draft}
					onDraftChange={setDraft}
					options={issuableFieldOptions(application)}
					choices={documentChoices(liveReveal)}
					blockedReason={
						reviewable.canReject
							? undefined
							: unavailableLabels(application.rejectUnavailabilities)[0]
					}
					onClose={() => setShowRejectModal(false)}
					onConfirm={handleReject}
				/>
			)}

			{/* 顯示全文（RevealKYCSensitive）：理由必填，授權 15 分鐘。 */}
			{showRevealDialog && (
				<RevealKycDialog
					storeName={store.storeName}
					onClose={() => setShowRevealDialog(false)}
					onConfirm={handleReveal}
				/>
			)}

			{/* 建立風險措施確認（D4 第 2 點）。確認框自己的 a11y 三件套在 CreateRiskMeasureDialog。 */}
			{showCreateMeasureModal && (
				<CreateRiskMeasureDialog
					storeId={store.id}
					onClose={() => setShowCreateMeasureModal(false)}
					onConfirm={handleCreateMeasure}
				/>
			)}

			{/* 建立措施（違規依據的警告與停權，CreateMeasure 的 warning／suspension.violation） */}
			{showCreateViolationMeasure && (
				<CreateMeasureDialog
					storeId={store.id}
					onClose={() => setShowCreateViolationMeasure(false)}
					onConfirm={handleCreateViolationMeasure}
				/>
			)}

			{/* 結束措施確認：只有 can_end 的措施（風險依據）能在這裡結束；違規依據的措施要到違規紀錄撤銷 */}
			{endTarget && (
				<EndMeasureDialog
					measure={endTarget}
					onClose={() => setEndTargetId(null)}
					onConfirm={() => handleEndMeasure(endTarget)}
				/>
			)}

			{/* 強制下架確認 —— 目標卡片 + 後果說明，失敗就地顯示 */}
			{unpublishTarget && (
				<ProductForceUnpublishDialog
					productName={unpublishTarget.name}
					productId={unpublishTarget.id}
					statusLabel={codeLabel(PRODUCT_STATUS_LABEL, unpublishTarget.status)}
					onClose={() => setUnpublishTarget(null)}
					onConfirm={handleForceUnpublish}
				/>
			)}

			{/* 刪除整個帳號（IdentityAdminService.DeleteAccount）。帳號 ID 是店家的 account_id。 */}
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

			{/* 終止合作（SellerGovernanceAdminService.CreateMeasure，kind=TERMINATION） */}
			{showTerminationDialog && (
				<CreateTerminationDialog
					storeId={store.id}
					accountId={store.accountId}
					options={termination.options}
					onClose={() => setShowTerminationDialog(false)}
					onConfirm={handleCreateTermination}
				/>
			)}

			{/* 圖片放大預覽 Lightbox — 整片點擊關閉是預期 UX,整個遮罩即關閉 button */}
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
