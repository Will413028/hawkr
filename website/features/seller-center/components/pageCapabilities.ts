import {
	SellerCapability,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { capabilityStatus, type MyStore } from "@/lib/seller/store";

const C = SellerCapability;

interface PageRule {
	prefix: string;
	/** 只比對這一頁本身，不含子路徑（商店設定頁的子路徑是認證與物流，各有各的能力）。 */
	exact?: boolean;
	/**
	 * 這一頁需要的賣家能力；沒有指定的頁面一律可進：公告頁、純導向頁。帳號層級的頁面（例如 `/seller/measures`）不在 `/seller-center` 之下，不在這張表。
	 */
	capability?: SellerCapability;
}

/**
 * 賣家中心每一頁需要的賣家能力。能不能進一頁由 `GetMyStore.capabilities` 決定：伺服器依店家狀態推導能力、
 * 逐方法檢查，這裡是主動導頁的 UX 捷徑，不是安全邊界。營業中的店家擁有全部能力，所以這張表只在能力
 * 被收回（停權、等待刪除）時才有作用。
 *
 * 每一頁只宣告它主要的讀取或操作所屬的能力（對照 `backend/internal/httpapi/seller/declaration.go` 與 gateway
 * 路由宣告的能力）。新增頁面時在這裡宣告；`pageCapabilities.unit.test.ts` 檢查每個路由都有宣告。
 *
 * ⚠️ 不得加 `/seller-center/invoices/` 這個總前綴：總覽、紀錄、異常詳情與發票詳情需要 `INVOICE_MANAGE`，
 * 稅務身分與自行開立條款是 `INVOICE_TAX_REMEDY`，買家申訴是 `ORDER_FULFILL`，各自宣告。
 */
const PAGES: readonly PageRule[] = [
	{ prefix: "/seller-center", exact: true },
	{ prefix: "/seller-center/suspended" },
	{ prefix: "/seller-center/dashboard", capability: C.MARKETING },
	{ prefix: "/seller-center/notifications", capability: C.NOTIFICATION },
	{ prefix: "/seller-center/preview", capability: C.STOREFRONT_WRITE },
	{ prefix: "/seller-center/orders", capability: C.ORDER_FULFILL },
	{ prefix: "/seller-center/shipments", capability: C.ORDER_FULFILL },
	{ prefix: "/seller-center/products", capability: C.PRODUCT_MANAGE },
	{ prefix: "/seller-center/inventory", capability: C.PRODUCT_MANAGE },
	{ prefix: "/seller-center/posts", capability: C.MARKETING },
	{ prefix: "/seller-center/coupons", capability: C.MARKETING },
	{ prefix: "/seller-center/customers", capability: C.MARKETING },
	{ prefix: "/seller-center/ai", capability: C.MARKETING },
	{ prefix: "/seller-center/messages", capability: C.CHAT_READ },
	{ prefix: "/seller-center/wallet", capability: C.STOREFRONT_WRITE },
	{ prefix: "/seller-center/invoices", capability: C.INVOICE_MANAGE },
	{
		prefix: "/seller-center/invoices/tax-status",
		capability: C.INVOICE_TAX_REMEDY,
	},
	{
		prefix: "/seller-center/invoices/self-issue-terms",
		capability: C.INVOICE_TAX_REMEDY,
	},
	{
		prefix: "/seller-center/invoices/appeals",
		capability: C.ORDER_FULFILL,
	},
	// 商店設定頁本身：停權時仍可以關店（ACCOUNT_CLOSE），頁面在沒有 STOREFRONT_WRITE 時只能檢視。
	{ prefix: "/seller-center/me", exact: true, capability: C.ACCOUNT_CLOSE },
	{
		prefix: "/seller-center/me/certifications",
		capability: C.STOREFRONT_WRITE,
	},
	// 主要讀取是 PRODUCT_MANAGE；開通、啟用與關閉配送方式要 SHIPPING_SETUP，由頁內依它的缺項顯示閘門。
	{ prefix: "/seller-center/me/shipping", capability: C.PRODUCT_MANAGE },
];

function matchPage(pathname: string): PageRule | undefined {
	let best: PageRule | undefined;
	for (const page of PAGES) {
		const matches =
			pathname === page.prefix ||
			(!page.exact && pathname.startsWith(`${page.prefix}/`));
		if (matches && (!best || page.prefix.length > best.prefix.length)) {
			best = page;
		}
	}
	return best;
}

/** 這一頁有沒有宣告（含不需要能力的頁面）。 */
export function isDeclaredPage(pathname: string): boolean {
	return matchPage(pathname) !== undefined;
}

/** 這一頁需要的能力；沒有宣告或不需要能力時是 undefined。 */
export function capabilityOfPage(
	pathname: string,
): SellerCapability | undefined {
	return matchPage(pathname)?.capability;
}

/** 依 `capabilities` 判斷這一頁現在開放嗎；沒有宣告的頁面不開放。 */
export function isPageAvailable(
	pathname: string,
	capabilities: readonly SellerCapability[],
): boolean {
	const page = matchPage(pathname);
	if (!page) return false;
	return (
		page.capability === undefined || capabilities.includes(page.capability)
	);
}

/** 缺這些條件代表店家整體被限制（停權、等待刪除、非營業中）：導去公告頁；其他缺項由頁內自己顯示閘門。 */
const NOTICE_REQUIREMENTS: readonly SellerRequirement[] = [
	SellerRequirement.NOT_SUSPENDED,
	SellerRequirement.NOT_PENDING_DELETION,
	SellerRequirement.STORE_ACTIVE,
];

/**
 * 這一頁現在該不該把賣家導去公告頁：只看這一頁宣告的主要讀取能力，它沒有啟用、而且缺的條件含
 * `NOT_SUSPENDED`、`NOT_PENDING_DELETION` 或 `STORE_ACTIVE` 時才導。缺其他條件（例如 `KYC_APPROVED`）
 * 的頁面留在原地，由頁內依相關能力的缺項顯示閘門。還沒讀到店家、伺服器沒回報這個能力、或這一頁
 * 沒有宣告能力時不擋，交給伺服器判斷。client 不自己推前置條件。
 */
export function isBlockedByCapabilities(
	my: MyStore | null | undefined,
	pathname: string,
): boolean {
	const capability = capabilityOfPage(pathname);
	if (capability === undefined) return false;
	const status = capabilityStatus(my, capability);
	if (!status || status.active) return false;
	return status.missing.some((requirement) =>
		NOTICE_REQUIREMENTS.includes(requirement),
	);
}
