import {
	type GetMyOnboardingResponse,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
	type OnboardingTask as ServerOnboardingTask,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerOnboardingActionButton } from "@hawkr/ui-text/keys";
import type { AppIconName } from "@/components/shared/AppIcon";
import { optionalText, type UiText } from "@/lib/i18n/ui-text";
import { CERT_PAGE_HREF } from "@/lib/seller/cert-gate";
import { CONTACT_HREF } from "@/lib/seller/payment-account";
import {
	isApproved,
	isProcessing,
	type ShippingOnboarding,
} from "@/lib/seller/shipping";
import { type MyStore, shippingGateOf } from "@/lib/seller/store";
import type { SellerOnboardingTaskId } from "./dashboardUiStore";

export type OnboardingTaskGroup = "required" | "recommended";

/** 任務的下一步：換頁，或在原地重讀收款帳戶（`RefreshMyPaymentAccount`）。 */
export type OnboardingNext =
	| { kind: "link"; href: string; label: string }
	| { kind: "refresh"; label: string };

export interface OnboardingTask {
	id: SellerOnboardingTaskId;
	group: OnboardingTaskGroup;
	title: string;
	description: string;
	panelTitle: string;
	panelDescription: string;
	/** Omitted when the task leaves nothing to do — the panel then shows
	 * status only, with no action button. */
	next?: OnboardingNext;
	icon: AppIconName;
	completed: boolean;
	statusLabel: string;
	locked?: boolean;
}

export const ONBOARDING_GROUP_LABEL: Record<OnboardingTaskGroup, string> = {
	required: "公開前必須",
	recommended: "建議完成",
};

/**
 * 必要任務只有圖示與面板文字是 client 的（依任務 id，不依狀態）；標題、狀態句、狀態與下一步都來自伺服器的
 * `GetMyOnboarding`（D26、D27）。
 */
const REQUIRED_PRESENTATION: Readonly<
	Partial<
		Record<
			OnboardingTaskId,
			Pick<
				OnboardingTask,
				"id" | "description" | "panelTitle" | "panelDescription" | "icon"
			>
		>
	>
> = {
	[OnboardingTaskId.PROFILE]: {
		id: "profile",
		description: "店名、logo 與所在地。",
		panelTitle: "完成商店資料",
		panelDescription: "買家要看得到你的店，需要有店名、logo 與所在地。",
		icon: "home",
	},
	[OnboardingTaskId.PLATFORM_REVIEW]: {
		id: "platform_review",
		description: "商家資料與統編的平台審核。",
		panelTitle: "完成平台審核",
		panelDescription: "平台審核通過後，店面才能公開，並開放設定收款帳戶。",
		icon: "shield",
	},
	[OnboardingTaskId.PAYMENT_ACCOUNT]: {
		id: "payment_account",
		description: "向金流商設定身分與銀行資料。",
		panelTitle: "設定收款帳戶",
		panelDescription:
			"身分與銀行資料由金流商收集，平台不經手；收款帳戶啟用後才能正式收款。",
		icon: "wallet",
	},
};

/** action 對到網頁的位置（網頁與 App 的位置不同，路由是 client 的）；按鈕文字由共用文案目錄依 action 決定。 */
function nextOf(
	ut: UiText,
	action: OnboardingAction,
): OnboardingNext | undefined {
	const label = optionalText(ut, sellerOnboardingActionButton(action));
	if (label === undefined) return undefined;
	switch (action) {
		case OnboardingAction.EDIT_PROFILE:
			return { kind: "link", href: "/seller-center/me", label };
		case OnboardingAction.SUBMIT_CERTIFICATION:
		case OnboardingAction.START_PAYMENT_ONBOARDING:
			return { kind: "link", href: CERT_PAGE_HREF, label };
		case OnboardingAction.CONTACT_SUPPORT:
			return { kind: "link", href: CONTACT_HREF, label };
		case OnboardingAction.REFRESH_PAYMENT_ACCOUNT:
			return { kind: "refresh", label };
		default:
			return undefined;
	}
}

function requiredTaskOf(
	ut: UiText,
	task: ServerOnboardingTask,
): OnboardingTask | undefined {
	const presentation = REQUIRED_PRESENTATION[task.id];
	if (!presentation) return undefined;
	return {
		...presentation,
		group: "required",
		title: task.title?.message ?? "",
		statusLabel: task.status?.message ?? "",
		completed: task.state === OnboardingTaskState.COMPLETED,
		locked: task.state === OnboardingTaskState.LOCKED,
		next: nextOf(ut, task.action),
	};
}

function buildRecommendedTasks({
	my,
	shippingRows,
	productCount,
}: {
	my: MyStore | null;
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
			next: {
				kind: "link",
				href: "/seller-center/me/shipping",
				label: shippingDone
					? "查看物流設定"
					: shippingProcessing
						? "查看審核進度"
						: "設定物流",
			},
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
			next: {
				kind: "link",
				href: "/seller-center/products/new",
				label: "新增商品",
			},
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
			next: { kind: "link", href: "/seller-center/me", label: "去設定" },
			icon: "edit",
			completed: hasDescription,
			statusLabel: hasDescription ? "已完成" : "待完成",
		},
	];
}

/**
 * 必要任務取自伺服器的開通摘要（`onboarding`，還沒讀到或讀取失敗是 null：沒有必要任務）；建議任務由 client 組（D31 Q1）。
 */
export function buildOnboardingTasks({
	ut,
	my,
	onboarding,
	shippingRows,
	productCount,
}: {
	ut: UiText;
	my: MyStore | null;
	onboarding: GetMyOnboardingResponse | null;
	shippingRows: ShippingOnboarding[];
	productCount: number | null;
}): OnboardingTask[] {
	const required = (onboarding?.tasks ?? []).flatMap((task) => {
		const built = requiredTaskOf(ut, task);
		return built ? [built] : [];
	});
	return [
		...required,
		...buildRecommendedTasks({ my, shippingRows, productCount }),
	];
}
