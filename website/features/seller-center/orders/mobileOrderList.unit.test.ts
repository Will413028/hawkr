import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const ordersSource = readFileSync(
	new URL("./SellerOrdersPage.tsx", import.meta.url),
	"utf8",
);
const afterSaleSource = readFileSync(
	new URL("./AfterSalePanel.tsx", import.meta.url),
	"utf8",
);
const orderDetailSource = readFileSync(
	new URL("./OrderDetailClient.tsx", import.meta.url),
	"utf8",
);
const shipmentDialogsSource = readFileSync(
	new URL("../../seller/components/ShipmentDialogs.tsx", import.meta.url),
	"utf8",
);
const temperatureTagSource = readFileSync(
	new URL("../../cart/components/TemperatureTag.tsx", import.meta.url),
	"utf8",
);
const sellerCss = readFileSync(
	new URL("../../../styles/features/seller-center.css", import.meta.url),
	"utf8",
);

function sourceRegion(source: string, start: string, end: string): string {
	const startIndex = source.indexOf(start);
	const endIndex = source.indexOf(end, startIndex + start.length);
	expect(startIndex).toBeGreaterThanOrEqual(0);
	expect(endIndex).toBeGreaterThan(startIndex);
	return source.slice(startIndex, endIndex);
}

describe("seller order mobile work list", () => {
	it("commits a status-card switch as one URL/view-state update", () => {
		const tabHandler = sourceRegion(
			ordersSource,
			"const setActiveTab = React.useCallback",
			"React.useEffect(() => {",
		);

		expect(ordersSource).toContain("useQueryStates({");
		expect(ordersSource).not.toContain("previousActiveTabRef");
		expect(ordersSource).toContain(
			"window.addEventListener(LOCATION_CHANGE_EVENT, syncFromLocation)",
		);
		expect(tabHandler).toContain("setOrderViewParams({");
		expect(tabHandler).toContain("tab,");
		expect(tabHandler).toContain('filter: "all"');
		expect(tabHandler).toContain("caseNo: null");
		expect(tabHandler).toContain("setOrderView((current)");
		expect(tabHandler).toContain("setVisibleLimit(ORDER_PAGE_SIZE)");
		expect(tabHandler).toContain("setMobileSelectionMode(false)");
	});

	it("keeps the desktop table and exposes a dedicated mobile list seam", () => {
		expect(ordersSource).toContain(
			'className="seller-orders__card seller-orders__desktop-view"',
		);
		expect(ordersSource).toContain('className="seller-orders__mobile-view"');
		expect(ordersSource).toContain("function MobileOrderCard");
	});

	it("derives every phone primary action from the shipment state machine", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(mobileCard).toContain("nextShipmentAction(");
		expect(mobileCard).toContain("ACTION_LABEL[action.kind]");
		expect(mobileCard).toContain("formatDeadline(order.autoCancelAt)");
		expect(mobileCard).toContain("deliveryTempLabel(");
		expect(mobileCard).toContain("<OrderStatusSummary");
	});

	it("uses an explicit phone selection mode instead of permanent checkboxes", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(ordersSource).toContain("mobileSelectionMode");
		expect(ordersSource).toContain("選取訂單");
		expect(ordersSource).toContain("已選 {selectedOrderNos.size} 筆");
		expect(ordersSource).toContain("seller-orders__bulk--mobile");
		expect(mobileCard).toContain("{selectionMode && (");
		expect(mobileCard).toContain("if (selectionMode && selectable)");
		expect(mobileCard).toContain("hidden={!expanded || selectionMode}");
	});

	it("keeps detail, primary action and overflow controls as separate targets", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(mobileCard).toContain("seller-orders__mobile-card-main");
		expect(mobileCard).toContain("seller-orders__mobile-card-detail");
		expect(mobileCard).toContain("seller-orders__mobile-card-primary");
		expect(mobileCard).toContain("<OrderActionsMenu");
		expect(ordersSource).toContain(
			"onClick={(event) => event.stopPropagation()}",
		);
		expect(sellerCss).toMatch(
			/\.seller-orders__mobile-card \.seller-orders__menu-list\s*\{[\s\S]*?top:\s*auto;[\s\S]*?bottom:\s*calc\(100% \+ 4px\)/,
		);
		expect(sellerCss).toMatch(
			/\.seller-orders__mobile-card-actions \.seller-orders__action-icon svg\s*\{[\s\S]*?width:\s*24px;[\s\S]*?height:\s*24px;/,
		);
	});

	it("clears hidden selections whenever search changes", () => {
		expect(ordersSource).toMatch(
			/setMobileSelectionMode\(false\);[\s\S]*?\}, \[activeTab, clearSelection, quickFilter, search\]\);/,
		);
	});

	it("keeps the visible card summary in the accessible name", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(mobileCard).toContain('className="visually-hidden"');
		expect(mobileCard).not.toMatch(
			/className="seller-orders__mobile-card-main"[^>]*aria-label=/,
		);
		expect(afterSaleSource).toContain("查看售後案件詳情");
	});

	it("collapses every phone order to one controlled summary row", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(ordersSource).toContain("expandedMobileOrderNo");
		expect(ordersSource).toContain(
			"expanded={expandedMobileOrderNo === order.orderNo}",
		);
		expect(mobileCard).toContain("aria-expanded=");
		expect(mobileCard).toContain("aria-controls=");
		expect(mobileCard).toContain("hidden={!expanded || selectionMode}");
		expect(mobileCard).toContain("if (!selectionMode) onToggleExpanded()");
		expect(sellerCss).toMatch(
			/\.seller-orders__mobile-card-toggle\s*\{[\s\S]*?grid-template-columns:\s*32px minmax\(0, 1fr\) auto auto 44px;[\s\S]*?min-height:\s*56px/,
		);
		expect(sellerCss).toMatch(
			/\.seller-orders__mobile-card-details\[hidden\]\s*\{[\s\S]*?display:\s*none/,
		);
	});

	it("reuses authoritative carrier artwork and the temperature tag", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		expect(mobileCard).toContain("ShippingMethod.TAQBIN");
		expect(mobileCard).toContain("ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP");
		expect(mobileCard).toContain("<ShippingBrandLogo brand={brand} compact />");
		expect(mobileCard).toContain('<AppIcon name="truck"');
		expect(mobileCard).toContain("<TemperatureTag");
		expect(mobileCard).toContain("const hasMobileFacts = Boolean(");
		expect(mobileCard).toContain("{hasMobileFacts && (");
		expect(mobileCard).not.toContain(
			"<span>{shippingMethodLabel(order.shipping?.method)}</span>",
		);
		expect(shipmentDialogsSource).toContain('tcat: { text: "黑貓"');
		expect(shipmentDialogsSource).toContain('711: { text: "7-11"');
		expect(temperatureTagSource).toContain("data-temperature={label}");
		expect(temperatureTagSource).toContain("whitespace-nowrap");
	});

	it("uses named media presets without invoking the width-based Next image loader", () => {
		const mobileCard = sourceRegion(
			ordersSource,
			"function MobileOrderCard",
			"function MobileBuyerName",
		);
		const desktopRow = sourceRegion(
			ordersSource,
			"function OrderRow",
			"function BuyerCell",
		);
		const buyerCell = sourceRegion(
			ordersSource,
			"function BuyerCell",
			"function IconSearch",
		);

		expect(ordersSource).toContain(
			'import AppImage from "@/components/shared/AppImage"',
		);
		expect(ordersSource).not.toContain('import Image from "next/image"');
		expect(mobileCard).toContain("<AppImage");
		expect(mobileCard).toContain('preset="thumb"');
		expect(desktopRow).toContain('preset="thumb"');
		expect(buyerCell).toContain('preset="avatar"');
		expect(orderDetailSource).toContain(
			'import AppImage from "@/components/shared/AppImage"',
		);
		expect(orderDetailSource).not.toContain('import Image from "next/image"');
		expect(
			[...orderDetailSource.matchAll(/preset="thumb"/g)].length,
		).toBeGreaterThanOrEqual(2);
		expect(orderDetailSource).toContain("href={src}");
	});

	it("gives after-sale cases the same phone list-to-drawer path", () => {
		expect(afterSaleSource).toContain("seller-orders__mobile-view--aftersale");
		expect(afterSaleSource).toContain("seller-orders__mobile-card--aftersale");
		expect(afterSaleSource).toContain("onClick={() => setActiveCase(c)}");
		expect(afterSaleSource).toContain("<AfterSaleDecisionDrawer");
	});
});
