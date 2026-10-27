import {
	KYCFlowStatus,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { commonUnknown, sellerSellerRequirement } from "@hawkr/ui-text/keys";
import type { AppIconName } from "@/components/shared/AppIcon";
import type { UiText } from "@/lib/i18n/ui-text";
import { CERT_PAGE_HREF } from "@/lib/seller/cert-gate";
import { kycFlowActionLabel, kycFlowStatusLabel } from "@/lib/seller/kyc-flow";
import {
	isApproved,
	isProcessing,
	type ShippingOnboarding,
} from "@/lib/seller/shipping";
import {
	isStoreCertified,
	type MyStore,
	missingRequirements,
	shippingGateOf,
} from "@/lib/seller/store";
import type { SellerOnboardingTaskId } from "./dashboardUiStore";

export type OnboardingTaskGroup = "required" | "recommended";

export interface OnboardingTask {
	id: SellerOnboardingTaskId;
	group: OnboardingTaskGroup;
	title: string;
	description: string;
	panelTitle: string;
	panelDescription: string;
	href: string;
	/** Omitted when the task leaves nothing to do — the panel then shows
	 * status only, with no action button. */
	cta?: string;
	icon: AppIconName;
	completed: boolean;
	statusLabel: string;
	locked?: boolean;
}

export const ONBOARDING_GROUP_LABEL: Record<OnboardingTaskGroup, string> = {
	required: "公開前必須",
	recommended: "建議完成",
};

/** 商店資料任務涵蓋的公開條件；名詞取自共用文案目錄。 */
const PROFILE_REQUIREMENTS: readonly SellerRequirement[] = [
	SellerRequirement.STORE_NAME,
	SellerRequirement.LOGO,
	SellerRequirement.TOWN_CODE,
];

/**
 * 缺這三個條件時外框一律導去公告頁（`pageCapabilities.ts`），開店進度卡不會在這時顯示，所以沒有專屬列；
 * 仍要列為已知，否則會掉進「其他公開條件」。
 */
const HANDLED_BY_NOTICE_PAGE: readonly SellerRequirement[] = [
	SellerRequirement.STORE_ACTIVE,
	SellerRequirement.NOT_PENDING_DELETION,
	SellerRequirement.NOT_SUSPENDED,
];

const KNOWN_REQUIREMENTS: ReadonlySet<SellerRequirement> = new Set([
	...HANDLED_BY_NOTICE_PAGE,
	SellerRequirement.KYC_APPROVED,
	...PROFILE_REQUIREMENTS,
]);

/**
 * 「公開前必須」只由伺服器的 `PUBLIC_LISTING` 缺的條件（`missing`）推導，client 不自己判斷：伺服器新增
 * 條件時，沒有專屬列的條件會以「其他公開條件」出現，只要補呈現層的文字。沒有 `PUBLIC_LISTING`（還沒
 * 讀到）時每個條件都當成還缺。
 */
function buildRequiredTasks(
	ut: UiText,
	my: MyStore | null,
	flowStatus: KYCFlowStatus | undefined,
): OnboardingTask[] {
	const missing = missingRequirements(my);
	const lacks = (requirement: SellerRequirement) =>
		missing === undefined || missing.includes(requirement);

	const missingProfile = PROFILE_REQUIREMENTS.filter(lacks).map((requirement) =>
		ut(sellerSellerRequirement(requirement) ?? commonUnknown()),
	);
	const profileDone = missingProfile.length === 0;
	const certDone = !lacks(SellerRequirement.KYC_APPROVED);

	const tasks: OnboardingTask[] = [
		{
			id: "profile",
			group: "required",
			title: "商店資料",
			description: "店名、logo 與所在地。",
			panelTitle: "完成商店資料",
			panelDescription: "買家要看得到你的店，需要有店名、logo 與所在地。",
			href: "/seller-center/me",
			cta: "去設定",
			icon: "home",
			completed: profileDone,
			statusLabel: profileDone
				? "已完成"
				: `缺少：${missingProfile.join("、")}`,
		},
		{
			id: "cert",
			group: "required",
			title: "商家認證",
			description: "身分認證與 PayUni 收款審核。",
			panelTitle: "完成商家認證",
			panelDescription:
				"通過商家認證（含 PayUni 收款審核）後，店面才會公開，也才能正式收款。",
			href: CERT_PAGE_HREF,
			// 已通過就沒有要辦的認證了 — 不再給按鈕。
			cta: certDone ? undefined : kycFlowActionLabel(ut, flowStatus),
			icon: "shield",
			completed: certDone,
			statusLabel: kycFlowStatusLabel(
				ut,
				certDone ? KYCFlowStatus.KYC_FLOW_STATUS_APPROVED : flowStatus,
			),
		},
	];

	const unrecognized = (missing ?? []).filter(
		(requirement) =>
			requirement !== SellerRequirement.UNSPECIFIED &&
			!KNOWN_REQUIREMENTS.has(requirement),
	);
	if (unrecognized.length > 0) {
		tasks.push({
			id: "other_requirement",
			group: "required",
			title: "其他公開條件",
			description: "還有公開條件尚未滿足。",
			panelTitle: "其他公開條件",
			panelDescription: "還有公開條件尚未滿足，請聯絡客服。",
			href: "/seller-center/me",
			icon: "home",
			completed: false,
			statusLabel: "待完成",
		});
	}
	return tasks;
}

function buildRecommendedTasks({
	my,
	flowStatus,
	shippingRows,
	productCount,
}: {
	my: MyStore | null;
	flowStatus: KYCFlowStatus | undefined;
	shippingRows: ShippingOnboarding[];
	productCount: number | null;
}): OnboardingTask[] {
	// 物流閘門只有 shippingGateOf 一個推導（等待刪除 > 停權 > 認證）。
	const shippingGate = shippingGateOf(my);
	const shippingOpen = shippingGate === "open";
	const shippingProcessing = shippingRows.some(isProcessing);
	const shippingDone = shippingRows.some(isApproved);
	const productsDone = (productCount ?? 0) > 0;
	const hasDescription = (my?.store?.description ?? "").trim() !== "";

	return [
		{
			id: "shipping",
			group: "recommended",
			title: "物流設定",
			description: "設定配送與退貨資訊。",
			panelTitle: "設定物流與退貨",
			panelDescription:
				"開通可用的配送方式，包含黑貓宅配、7-11 店到店、寄件地址與退貨資訊。",
			href: "/seller-center/me/shipping",
			cta: shippingDone
				? "查看物流設定"
				: shippingProcessing
					? "查看審核進度"
					: "設定物流",
			icon: "truck",
			completed: shippingDone,
			locked: !shippingOpen,
			statusLabel: !shippingOpen
				? shippingGate === "certification"
					? "需先通過認證"
					: "未營業"
				: shippingDone
					? "已開通"
					: shippingProcessing
						? "審核中"
						: "待設定",
		},
		{
			id: "product",
			group: "recommended",
			title: "上架商品",
			description: "建立第一個可購買商品。",
			panelTitle: "上架第一個商品",
			panelDescription:
				"完成商品圖片、規格、售價、庫存與配送方式後，買家就能開始下單。",
			href: "/seller-center/products/new",
			cta: "新增商品",
			icon: "bag",
			completed: productsDone,
			statusLabel: productsDone ? "已上架" : "待上架",
		},
		{
			id: "description",
			group: "recommended",
			title: "商店介紹",
			description: "說明你賣什麼、產地與品牌故事。",
			panelTitle: "完成商店介紹",
			panelDescription: "一段話告訴買家你是誰、種了什麼、為什麼買你的。",
			href: "/seller-center/me",
			cta: "去設定",
			icon: "edit",
			completed: hasDescription,
			statusLabel: hasDescription ? "已完成" : "待完成",
		},
	];
}

export function buildOnboardingTasks({
	ut,
	my,
	flowStatus,
	shippingRows,
	productCount,
}: {
	ut: UiText;
	my: MyStore | null;
	flowStatus: KYCFlowStatus | undefined;
	shippingRows: ShippingOnboarding[];
	productCount: number | null;
}): OnboardingTask[] {
	return [
		...buildRequiredTasks(ut, my, flowStatus),
		...buildRecommendedTasks({ my, flowStatus, shippingRows, productCount }),
	];
}
