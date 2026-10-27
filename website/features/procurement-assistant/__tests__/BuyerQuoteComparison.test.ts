/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
	within,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import BuyerQuoteComparison from "../BuyerQuoteComparison";
import type {
	BuyerFacingSellerResponse,
	BuyerQuoteAcceptanceCommand,
	BuyerQuoteAcceptanceResponse,
	BuyerQuoteFailure,
	BuyerQuoteResult,
} from "../buyer-quote-contract";
import { buyerQuoteFailure } from "../buyer-quote-contract";

// `Button href` 會渲染 `@/components/shared/Link`，那支需要 App Router context。
// 與 MobileBottomNav.unit.test 同樣把它換成一個純 <a>：這裡要驗的是「連到哪裡」，
// 不是 Next 的導航實作。
vi.mock("@/components/shared/Link", async () => {
	const ReactModule = await import("react");
	return {
		default: ReactModule.forwardRef<
			HTMLAnchorElement,
			React.AnchorHTMLAttributes<HTMLAnchorElement> & {
				href?: string;
				prefetch?: boolean;
				hardNavigation?: boolean;
			}
		>(({ prefetch: _p, hardNavigation: _h, href, ...props }, ref) =>
			ReactModule.createElement("a", { ...props, href, ref }),
		),
	};
});

afterEach(cleanup);

const SNAPSHOT_HASH = "b".repeat(64);

function response(
	overrides: Partial<BuyerFacingSellerResponse> = {},
	quoteOverrides: Partial<BuyerFacingSellerResponse["quote"]> = {},
): BuyerFacingSellerResponse {
	return {
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		status: "SUBMITTED",
		payloadHash: "a".repeat(64),
		submittedAt: "2026-09-13T02:00:00Z",
		seller: {
			displayName: "範例鮮果合作社",
			partyType: "PLATFORM_SELLER",
			storeCode: "greenfield",
		},
		quote: {
			responseType: "MANUAL_QUOTE",
			offeredQuality: { grade: "特級", inspectionNotes: "抽驗三箱" },
			unitBasis: { quantity: "1", unit: "KG" },
			offeredQuantity: "500",
			offeredQuantityUnit: "KG",
			conversionBasis: "",
			currency: "TWD",
			goodsAmount: "18000",
			shippingAmount: "0",
			taxAmount: "900",
			taxInclusionBasis: "TAX_EXCLUDED",
			totalAmount: "18900",
			leadTimeDays: "3",
			promisedDeliveryAt: "2026-09-20T02:00:00Z",
			validUntil: "2026-09-18T02:00:00Z",
			termsNote: "需自備棧板",
			deviationDisclosure: "",
			...quoteOverrides,
		},
		outsideBuyerRequestedRange: false,
		shortlisted: false,
		acceptability: "ACCEPTABLE",
		...overrides,
	};
}

function ack(
	overrides: Partial<BuyerQuoteAcceptanceResponse> = {},
): BuyerQuoteAcceptanceResponse {
	return {
		buyerAcceptanceId: "1234567890123456805",
		sellerResponseId: "1234567890123456788",
		sellerResponseVersion: "1234567890123456791",
		acceptedDealPrice: "18900",
		currency: "TWD",
		acceptedAt: "2026-09-13T03:00:00Z",
		orderProvisioningState: "CREATING_ORDER",
		orderNo: "",
		paymentDeadline: "",
		replayed: false,
		...overrides,
	};
}

type AcceptHandler = (
	command: BuyerQuoteAcceptanceCommand,
) => Promise<BuyerQuoteResult<BuyerQuoteAcceptanceResponse>>;

function renderPanel(
	options: {
		responses?: BuyerFacingSellerResponse[];
		rfqSnapshotHash?: string;
		onAcceptQuote?: AcceptHandler;
		onToggleShortlist?: (
			target: { sellerResponseId: string; sellerResponseVersion: string },
			shortlisted: boolean,
		) => Promise<BuyerQuoteResult<boolean>>;
		createIdempotencyKey?: () => string;
		onRefresh?: () => void;
	} = {},
) {
	let counter = 0;
	const onAcceptQuote =
		options.onAcceptQuote ??
		(vi.fn(async () => ({ ok: true, data: ack() })) as AcceptHandler);
	const onToggleShortlist =
		options.onToggleShortlist ??
		vi.fn(async (_target, shortlisted: boolean) => ({
			ok: true as const,
			data: shortlisted,
		}));
	const createIdempotencyKey =
		options.createIdempotencyKey ??
		(() => {
			counter += 1;
			return `bqa-key-${counter}`;
		});
	render(
		React.createElement(BuyerQuoteComparison, {
			responses: options.responses ?? [response()],
			rfqSnapshotHash: options.rfqSnapshotHash ?? SNAPSHOT_HASH,
			onAcceptQuote,
			onToggleShortlist,
			createIdempotencyKey,
			onRefresh: options.onRefresh,
		}),
	);
	return { onAcceptQuote, onToggleShortlist };
}

function acceptRowButtons() {
	return screen.getAllByRole("button", { name: "接受這個版本" });
}

async function openConfirm(index = 0) {
	fireEvent.click(acceptRowButtons()[index]);
	await screen.findByRole("button", { name: "確認接受這個版本" });
}

async function acceptFirst() {
	await openConfirm();
	fireEvent.click(screen.getByRole("button", { name: "確認接受這個版本" }));
}

describe("比較清單", () => {
	it("每個 immutable 版本各一列，不可接受的版本仍留在清單裡", async () => {
		renderPanel({
			responses: [
				response(),
				response({
					sellerResponseId: "1234567890123456789",
					sellerResponseVersion: "1234567890123456792",
					acceptability: "SUPERSEDED",
					seller: {
						displayName: "豐田產地直送",
						partyType: "EXTERNAL_VENDOR",
						storeCode: "",
					},
				}),
			],
		});
		expect(screen.getByText("收到 2 份正式回價")).toBeTruthy();
		expect(screen.getByText("範例鮮果合作社")).toBeTruthy();
		expect(screen.getByText("豐田產地直送")).toBeTruthy();
		expect(screen.getByText("已被新版本取代")).toBeTruthy();
	});

	it("不可接受的版本按不下接受，可接受的按得下", () => {
		renderPanel({
			responses: [
				response(),
				response({
					sellerResponseId: "1234567890123456789",
					acceptability: "EXPIRED",
				}),
			],
		});
		const buttons = acceptRowButtons();
		expect(buttons[0].hasAttribute("disabled")).toBe(false);
		expect(buttons[1].hasAttribute("disabled")).toBe(true);
	});

	it("不顯示任何自算的每單位價", () => {
		renderPanel({ responses: [response()] });
		const table = screen.getByRole("table");
		expect(within(table).queryByText(/每單位價|單價/)).toBeNull();
		// 18900 / 500 = 37.8 —— 這個數字不該在畫面上任何地方出現。
		expect(document.body.textContent).not.toContain("37.8");
	});

	it("計價基準不同的兩份報價會警示不可直接相比", () => {
		renderPanel({
			responses: [
				response(),
				response(
					{ sellerResponseId: "1234567890123456789" },
					{
						unitBasis: { quantity: "1", unit: "CATTY" },
						offeredQuantityUnit: "CATTY",
					},
				),
			],
		});
		expect(
			screen.getByText("各家的計價基準不同，單價不可直接相比"),
		).toBeTruthy();
	});

	it("計價基準一致時不出現那則警示", () => {
		renderPanel({
			responses: [
				response(),
				response({ sellerResponseId: "1234567890123456789" }),
			],
		});
		expect(
			screen.queryByText("各家的計價基準不同，單價不可直接相比"),
		).toBeNull();
	});

	it("超出希望詢價區間只中性揭露，不擋接受", () => {
		renderPanel({
			responses: [response({ outsideBuyerRequestedRange: true })],
		});
		expect(screen.getByText("超出你的希望詢價區間")).toBeTruthy();
		expect(acceptRowButtons()[0].hasAttribute("disabled")).toBe(false);
	});

	it("明細展開後可以看到十七欄裡的換算依據與差異揭露", () => {
		renderPanel({ responses: [response()] });
		fireEvent.click(screen.getByRole("button", { name: "查看明細" }));
		expect(screen.getByText("換算依據")).toBeTruthy();
		expect(screen.getByText("賣家未提供換算依據")).toBeTruthy();
		expect(screen.getByText("差異揭露")).toBeTruthy();
		expect(screen.getByText("條件備註")).toBeTruthy();
	});

	it("沒有回價時說清楚「已發布不等於已看到」", () => {
		renderPanel({ responses: [] });
		expect(screen.queryByRole("table")).toBeNull();
		expect(screen.getByText(/已發布不代表賣家已經看到或已經報價/)).toBeTruthy();
	});
});

describe("加入與移出比較", () => {
	it("送出的是這一列的 exact version，而且不需要冪等鍵", async () => {
		const { onToggleShortlist } = renderPanel();
		fireEvent.click(
			screen.getByRole("button", { name: "將 範例鮮果合作社 加入比較" }),
		);
		await waitFor(() => {
			expect(onToggleShortlist).toHaveBeenCalledWith(
				{
					sellerResponseId: "1234567890123456788",
					sellerResponseVersion: "1234567890123456791",
				},
				true,
			);
		});
	});

	it("失敗時顯示原因，不宣稱已更新", async () => {
		const onToggleShortlist = vi.fn(async () => ({
			ok: false as const,
			error: buyerQuoteFailure(503, ""),
		}));
		renderPanel({ onToggleShortlist });
		fireEvent.click(
			screen.getByRole("button", { name: "將 範例鮮果合作社 加入比較" }),
		);
		expect(await screen.findByText("比較清單沒有更新")).toBeTruthy();
	});
});

describe("接受一個 exact version", () => {
	it("先跳確認，確認之後才送出", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: true as const,
			data: ack(),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		fireEvent.click(acceptRowButtons()[0]);
		await screen.findByRole("button", { name: "確認接受這個版本" });
		expect(onAcceptQuote).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole("button", { name: "確認接受這個版本" }));
		await waitFor(() => expect(onAcceptQuote).toHaveBeenCalledTimes(1));
	});

	it("送出的命令帶 exact version 與快照雜湊，不帶任何金額", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: true as const,
			data: ack(),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		await acceptFirst();
		await waitFor(() => expect(onAcceptQuote).toHaveBeenCalled());
		const command = vi.mocked(onAcceptQuote).mock.calls[0][0];
		expect(command.sellerResponseId).toBe("1234567890123456788");
		expect(command.expectedSellerResponseVersion).toBe("1234567890123456791");
		expect(command.expectedRfqSnapshotHash).toBe(SNAPSHOT_HASH);
		expect(Object.keys(command).sort()).toEqual([
			"expectedRfqSnapshotHash",
			"expectedSellerResponseVersion",
			"idempotencyKey",
			"sellerResponseId",
		]);
	});

	it("同一版本重試沿用同一把冪等鍵", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: false as const,
			error: buyerQuoteFailure(503, ""),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		await acceptFirst();
		await screen.findByText("沒有接受這個報價版本");
		await acceptFirst();
		await waitFor(() => expect(onAcceptQuote).toHaveBeenCalledTimes(2));
		const [first, second] = vi.mocked(onAcceptQuote).mock.calls;
		expect(second[0].idempotencyKey).toBe(first[0].idempotencyKey);
	});

	it("沒有快照比對權杖時所有接受都鎖住", () => {
		renderPanel({ rfqSnapshotHash: "" });
		expect(screen.getByText("目前無法接受任何版本")).toBeTruthy();
		expect(acceptRowButtons()[0].hasAttribute("disabled")).toBe(true);
	});
});

describe("接受成功之後的狀態", () => {
	it("固定文案逐字出現，且不宣稱已成交或已下單", async () => {
		renderPanel();
		await acceptFirst();
		expect(
			await screen.findByText("已接受這個報價版本，成交價格已凍結，尚未付款"),
		).toBeTruthy();
		const text = document.body.textContent ?? "";
		expect(text).not.toContain("已成交");
		expect(text).not.toContain("已下單");
		expect(text).not.toContain("訂單成立");
	});

	it("CREATING_ORDER 只說訂單正在建立，沒有訂單編號與付款按鈕", async () => {
		renderPanel();
		await acceptFirst();
		await screen.findByRole("status");
		expect(screen.getByText(/正在建立待付款訂單/)).toBeTruthy();
		expect(screen.queryByText("訂單編號")).toBeNull();
		expect(screen.queryByRole("link", { name: "前往付款" })).toBeNull();
	});

	it("ORDER_CREATED 才出現訂單編號、付款期限與付款入口", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: true as const,
			data: ack({
				orderProvisioningState: "ORDER_CREATED",
				orderNo: "HK20260913001",
				paymentDeadline: "2026-09-14T03:00:00Z",
			}),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		await acceptFirst();
		expect(await screen.findByText("HK20260913001")).toBeTruthy();
		expect(screen.getByText("付款期限")).toBeTruthy();
		expect(
			screen.getByRole("link", { name: "前往付款" }).getAttribute("href"),
		).toBe("/account/orders/HK20260913001");
	});

	it("ORDER_FAILED 說明價格仍然凍結，但不宣稱訂單已建立", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: true as const,
			data: ack({ orderProvisioningState: "ORDER_FAILED" }),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		await acceptFirst();
		expect(await screen.findByText(/待付款訂單沒有建立成功/)).toBeTruthy();
		expect(
			screen.getByText("已接受這個報價版本，成交價格已凍結，尚未付款"),
		).toBeTruthy();
		expect(screen.queryByText("訂單編號")).toBeNull();
	});

	it("replayed 講明沒有建立第二筆", async () => {
		const onAcceptQuote = vi.fn(async () => ({
			ok: true as const,
			data: ack({ replayed: true }),
		})) as AcceptHandler;
		renderPanel({ onAcceptQuote });
		await acceptFirst();
		expect(
			await screen.findByText("這次是同一筆接受的重試結果，沒有建立第二筆。"),
		).toBeTruthy();
	});

	it("接受成立之後其他版本都不能再接受", async () => {
		renderPanel({
			responses: [
				response(),
				response({ sellerResponseId: "1234567890123456789" }),
			],
		});
		await acceptFirst();
		await screen.findByRole("status");
		for (const button of acceptRowButtons()) {
			expect(button.hasAttribute("disabled")).toBe(true);
		}
	});
});

describe("接受失敗的處置", () => {
	async function failWith(error: BuyerQuoteFailure) {
		const onAcceptQuote = vi.fn(async () => ({
			ok: false as const,
			error,
		})) as AcceptHandler;
		const onRefresh = vi.fn();
		renderPanel({ onAcceptQuote, onRefresh });
		await acceptFirst();
		await screen.findByText("沒有接受這個報價版本");
		return { onAcceptQuote, onRefresh };
	}

	it("報價已變動時明講，而且不自動改成接受最新版", async () => {
		const { onAcceptQuote } = await failWith(
			buyerQuoteFailure(409, "STALE_SELLER_RESPONSE"),
		);
		expect(screen.getByText(/已經送出新的報價版本/)).toBeTruthy();
		expect(screen.getByText(/系統不會自動幫你接受新版本/)).toBeTruthy();
		expect(onAcceptQuote).toHaveBeenCalledTimes(1);
		expect(
			screen.queryByText("已接受這個報價版本，成交價格已凍結，尚未付款"),
		).toBeNull();
	});

	it("已在結帳中導去付款，並用伺服器給的既有單號", async () => {
		await failWith(
			buyerQuoteFailure(409, "RFQ_CHECKOUT_ALREADY_ACTIVE", "HK20260913001"),
		);
		expect(
			screen.getByRole("link", { name: "前往付款" }).getAttribute("href"),
		).toBe("/account/orders/HK20260913001");
	});

	it("已在結帳中但沒拿到單號時退回訂單列表，不猜一個單號", async () => {
		await failWith(buyerQuoteFailure(409, "RFQ_CHECKOUT_ALREADY_ACTIVE"));
		expect(
			screen.getByRole("link", { name: "前往付款" }).getAttribute("href"),
		).toBe("/account/orders");
	});

	it("已成交結案是另一句話，而且不給付款入口", async () => {
		await failWith(buyerQuoteFailure(409, "RFQ_ALREADY_AWARDED"));
		expect(screen.getByText(/這張詢價已經成交結案/)).toBeTruthy();
		expect(screen.queryByRole("link", { name: "前往付款" })).toBeNull();
	});

	it("快照過期時提供重新載入", async () => {
		const { onRefresh } = await failWith(
			buyerQuoteFailure(409, "STALE_RFQ_SNAPSHOT"),
		);
		fireEvent.click(screen.getByRole("button", { name: "重新載入報價" }));
		expect(onRefresh).toHaveBeenCalledTimes(1);
	});
});
