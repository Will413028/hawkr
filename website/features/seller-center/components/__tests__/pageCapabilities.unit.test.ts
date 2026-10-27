import { readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	SellerCapability,
	SellerRequirement,
	StoreStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import {
	activeStatuses,
	capabilityStatuses,
	publicListingStatus,
} from "@/lib/seller/capability-fixtures";
import { shippingGateOf } from "@/lib/seller/store";
import {
	capabilityOfPage,
	isBlockedByCapabilities,
	isDeclaredPage,
	isPageAvailable,
} from "../pageCapabilities";
import { SELLER_FOOTER_ITEMS, SELLER_NAV_GROUPS } from "../sellerNav";

const C = SellerCapability;

/** 停權時伺服器仍授予的能力（backend `grantedWhenSuspended`）。 */
const SUSPENDED = [
	C.ACCOUNT_SELF_READ,
	C.ORDER_FULFILL,
	C.ORDER_CONTRACT,
	C.CHAT_SEND,
	C.CHAT_READ,
	C.NOTIFICATION,
	C.CONTENT_CONTRACT,
	C.ACCOUNT_CLOSE,
	C.INVOICE_TAX_REMEDY,
];

/** 等待刪除時：沒有經營類（店面設定、商品、行銷、發票管理）與關店。 */
const PENDING_DELETION = [
	C.ACCOUNT_SELF_READ,
	C.ORDER_FULFILL,
	C.ORDER_CONTRACT,
	C.CONTENT_CONTRACT,
	C.CHAT_SEND,
	C.CHAT_READ,
	C.NOTIFICATION,
	C.INVOICE_TAX_REMEDY,
];

const ALL = Object.values(C).filter(
	(v): v is SellerCapability => typeof v === "number" && v !== C.UNSPECIFIED,
);

const APP_ROOT = fileURLToPath(
	new URL("../../../../app/[locale]/(seller)/seller-center", import.meta.url),
);

function sellerCenterRoutes(dir: string): string[] {
	const routes: string[] = [];
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const full = join(dir, entry.name);
		if (entry.isDirectory()) routes.push(...sellerCenterRoutes(full));
		else if (entry.name === "page.tsx") {
			const rel = relative(APP_ROOT, dir).replace(/\[[^\]]+\]/g, "x");
			routes.push(rel === "" ? "/seller-center" : `/seller-center/${rel}`);
		}
	}
	return routes;
}

describe("每個賣家中心路由都宣告了需要的能力", () => {
	it("app 目錄下的每個頁面都有宣告", () => {
		const routes = sellerCenterRoutes(APP_ROOT);
		expect(routes.length).toBeGreaterThan(20);
		const undeclared = routes.filter((route) => !isDeclaredPage(route));
		expect(undeclared).toEqual([]);
	});

	it("側欄裡工作區內的項目都有宣告（連出外框的帳號頁不在能力導向的範圍）", () => {
		const hrefs = [
			...SELLER_NAV_GROUPS.flatMap((g) => g.items),
			...SELLER_FOOTER_ITEMS,
		]
			.map((item) => item.href)
			.filter((href) => href.startsWith("/seller-center/"));
		expect(hrefs.length).toBeGreaterThan(10);
		expect(hrefs.filter((href) => !isDeclaredPage(href))).toEqual([]);
	});
});

describe("isPageAvailable", () => {
	it("停權時伺服器授予的能力涵蓋的頁面開放", () => {
		for (const path of [
			"/seller-center/orders",
			"/seller-center/orders/O123",
			"/seller-center/notifications",
			"/seller-center/notifications/preferences",
			"/seller-center/suspended",
			"/seller-center/me",
			"/seller-center/messages",
			"/seller-center/messages/T1",
			"/seller-center/shipments/anomalies",
			"/seller-center/invoices/tax-status",
			"/seller-center/invoices/self-issue-terms",
			"/seller-center/invoices/appeals",
			"/seller-center/invoices/appeals/C123",
		]) {
			expect(isPageAvailable(path, SUSPENDED), path).toBe(true);
		}
	});

	it("需要經營類能力的頁面在停權時不開放", () => {
		for (const path of [
			"/seller-center/dashboard",
			"/seller-center/products",
			"/seller-center/products/new",
			"/seller-center/inventory",
			"/seller-center/posts",
			"/seller-center/wallet",
			"/seller-center/coupons",
			"/seller-center/customers",
			"/seller-center/insights",
			"/seller-center/ai",
			"/seller-center/preview",
			"/seller-center/me/certifications",
			"/seller-center/me/shipping",
			"/seller-center/invoices",
			"/seller-center/invoices/documents",
			"/seller-center/invoices/jobs/J1",
			"/seller-center/invoices/D123",
		]) {
			expect(isPageAvailable(path, SUSPENDED), path).toBe(false);
		}
	});

	it("營業中的店家擁有全部能力，每一頁都開放", () => {
		for (const route of sellerCenterRoutes(APP_ROOT)) {
			expect(isPageAvailable(route, ALL), route).toBe(true);
		}
	});

	it("由能力決定，不只看路徑", () => {
		expect(
			isPageAvailable("/seller-center/orders", [C.ACCOUNT_SELF_READ]),
		).toBe(false);
		expect(isPageAvailable("/seller-center/orders", [C.ORDER_FULFILL])).toBe(
			true,
		);
		expect(isPageAvailable("/seller-center/messages", [C.ORDER_FULFILL])).toBe(
			false,
		);
	});

	it("等待刪除：沒有經營類與關店，仍可處理訂單", () => {
		expect(isPageAvailable("/seller-center/orders", PENDING_DELETION)).toBe(
			true,
		);
		expect(isPageAvailable("/seller-center/products", PENDING_DELETION)).toBe(
			false,
		);
		expect(isPageAvailable("/seller-center/me", PENDING_DELETION)).toBe(false);
	});

	it("公告頁一律開放；沒有宣告的頁面不開放；不比對只是前綴字串相同的路徑", () => {
		expect(isPageAvailable("/seller-center/suspended", [])).toBe(true);
		expect(isPageAvailable("/seller-center/unknown", ALL)).toBe(false);
		expect(isPageAvailable("/seller-center/orders-archive", ALL)).toBe(false);
	});

	it("商店設定頁只比對本身：子路徑各有各的能力", () => {
		expect(capabilityOfPage("/seller-center/me")).toBe(C.ACCOUNT_CLOSE);
		expect(capabilityOfPage("/seller-center/me/shipping")).toBe(
			C.PRODUCT_MANAGE,
		);
		expect(capabilityOfPage("/seller-center/me/certifications")).toBe(
			C.STOREFRONT_WRITE,
		);
	});
});

describe("isBlockedByCapabilities", () => {
	const R = SellerRequirement;
	const store = (status: StoreStatus) => ({ status });
	const active = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.ACTIVE),
		capabilities: [
			...activeStatuses(ALL.filter((c) => c !== C.PUBLIC_LISTING)),
			publicListingStatus([]),
		],
	});
	const pendingDeletion = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.PENDING_DELETION),
		capabilities: capabilityStatuses(PENDING_DELETION, [
			R.NOT_PENDING_DELETION,
		]),
	});
	const suspended = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.ACTIVE),
		suspended: true,
		capabilities: capabilityStatuses(SUSPENDED, [R.NOT_SUSPENDED]),
	});
	// 營業中、沒停權，只是認證還沒核准：物流建店缺 KYC_APPROVED，其他能力都有。
	const kycPending = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.ACTIVE),
		capabilities: [
			...capabilityStatuses(
				ALL.filter((c) => c !== C.SHIPPING_SETUP),
				[],
				{ [C.SHIPPING_SETUP]: [R.KYC_APPROVED] },
			),
			publicListingStatus([R.KYC_APPROVED]),
		],
	});
	// 認證已核准但被停權：物流建店只缺 NOT_SUSPENDED（不再缺 KYC_APPROVED）。
	const kycApprovedSuspended = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.ACTIVE),
		suspended: true,
		capabilities: capabilityStatuses(SUSPENDED, [R.NOT_SUSPENDED]),
	});
	const closedStore = create(GetMyStoreResponseSchema, {
		store: store(StoreStatus.CLOSED),
		capabilities: capabilityStatuses([C.ACCOUNT_SELF_READ], [R.STORE_ACTIVE]),
	});

	it("營業中的店家不被擋", () => {
		expect(isBlockedByCapabilities(active, "/seller-center/products")).toBe(
			false,
		);
		expect(isBlockedByCapabilities(active, "/seller-center/dashboard")).toBe(
			false,
		);
	});

	it("缺的條件不是停權、等待刪除、非營業中：留在原頁，不導向公告頁", () => {
		const lacksOnlyKyc = create(GetMyStoreResponseSchema, {
			store: store(StoreStatus.ACTIVE),
			capabilities: capabilityStatuses(
				ALL.filter((c) => c !== C.MARKETING),
				[R.KYC_APPROVED],
			),
		});
		expect(isBlockedByCapabilities(lacksOnlyKyc, "/seller-center/posts")).toBe(
			false,
		);
	});

	it("認證未核准：物流頁留在原頁（主要讀取是 PRODUCT_MANAGE），建店由頁內依 SHIPPING_SETUP 閘門", () => {
		expect(
			isBlockedByCapabilities(kycPending, "/seller-center/me/shipping"),
		).toBe(false);
		expect(isBlockedByCapabilities(kycPending, "/seller-center/products")).toBe(
			false,
		);
		expect(shippingGateOf(kycPending)).toBe("certification");
	});

	it("認證已核准但被停權：只有主要能力缺 NOT_SUSPENDED 的頁面導向公告頁", () => {
		expect(
			isBlockedByCapabilities(
				kycApprovedSuspended,
				"/seller-center/me/shipping",
			),
		).toBe(true);
		expect(
			isBlockedByCapabilities(kycApprovedSuspended, "/seller-center/orders"),
		).toBe(false);
		expect(shippingGateOf(kycApprovedSuspended)).toBe("suspended");
	});

	it("等待刪除與停權的店家：缺能力的頁面導向公告頁，仍有能力的頁面與公告頁不擋", () => {
		expect(
			isBlockedByCapabilities(pendingDeletion, "/seller-center/products"),
		).toBe(true);
		expect(
			isBlockedByCapabilities(pendingDeletion, "/seller-center/orders"),
		).toBe(false);
		expect(shippingGateOf(pendingDeletion)).toBe("pending_deletion");
		expect(isBlockedByCapabilities(suspended, "/seller-center/products")).toBe(
			true,
		);
		expect(isBlockedByCapabilities(suspended, "/seller-center/suspended")).toBe(
			false,
		);
	});

	it("缺 STORE_ACTIVE（非營業中）也導向公告頁", () => {
		expect(
			isBlockedByCapabilities(closedStore, "/seller-center/products"),
		).toBe(true);
	});

	it("沒有宣告的頁面、還沒讀到店家、伺服器沒回報這個能力時不擋", () => {
		expect(isBlockedByCapabilities(suspended, "/seller-center/unknown")).toBe(
			false,
		);
		expect(isBlockedByCapabilities(null, "/seller-center/products")).toBe(
			false,
		);
		const noStatuses = create(GetMyStoreResponseSchema, {
			store: store(StoreStatus.CLOSED),
		});
		expect(isBlockedByCapabilities(noStatuses, "/seller-center/me")).toBe(
			false,
		);
	});
});
