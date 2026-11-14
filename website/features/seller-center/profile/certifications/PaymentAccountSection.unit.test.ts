// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	OnboardingSurface,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	type PaymentAccount,
	PaymentAccountSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	act,
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	account: null as unknown,
	hookEnabled: [] as boolean[],
	param: null as string | null,
	createSession: vi.fn(),
	navigate: vi.fn(),
	refetch: vi.fn(),
	summaryRefetch: vi.fn(),
	/** useRefreshPaymentAccount 回的函式：回跳與狀態按鈕共用（重讀、再讀摘要與帳戶）。 */
	refreshAndReload: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => ({ get: () => mocks.param }),
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useRefreshPaymentAccount: () => mocks.refreshAndReload,
	useMyPaymentAccount: (options?: { enabled?: boolean }) => {
		mocks.hookEnabled.push(options?.enabled ?? true);
		return {
			account: mocks.account,
			loading: false,
			error: null,
			refetch: mocks.refetch,
		};
	},
}));
vi.mock("@/lib/intl/routed-locale", () => ({
	useRoutedLocale: () => "en",
}));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: mocks.navigate,
}));
vi.mock("@/lib/seller/payment-account", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/payment-account")>()),
	createPaymentOnboardingSession: mocks.createSession,
}));
vi.mock("next/dynamic", async () => {
	const react = await import("react");
	return {
		default: () =>
			function EmbeddedStub(props: {
				publishableKey: string;
				onExit: () => void;
			}) {
				return react.createElement(
					"div",
					{ "data-testid": "embedded", "data-key": props.publishableKey },
					react.createElement(
						"button",
						{ type: "button", onClick: props.onExit },
						"exit-embedded",
					),
				);
			},
	};
});
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		onClick,
		disabled,
		href,
	}: {
		children: React.ReactNode;
		onClick?: () => void;
		disabled?: boolean;
		href?: string;
	}) =>
		href
			? React.createElement("a", { href }, children)
			: React.createElement(
					"button",
					{ type: "button", onClick, disabled },
					children,
				),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { PaymentAccountSection } from "./PaymentAccountSection";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function account(
	status: PaymentAccountStatus,
	extra: { surface?: OnboardingSurface; payoutsActive?: boolean } = {},
): PaymentAccount {
	return create(PaymentAccountSchema, {
		status,
		onboardingSurface: extra.surface ?? OnboardingSurface.HOSTED,
		payoutsActive: extra.payoutsActive ?? false,
	});
}

async function mount(
	approved = true,
	summary: { loading?: boolean; error?: Error | null } = {},
) {
	const view = render(
		React.createElement(PaymentAccountSection, {
			platformApproved: approved,
			summary: {
				loading: summary.loading ?? false,
				error: summary.error ?? null,
				refetch: mocks.summaryRefetch,
			},
		}),
	);
	await act(async () => {});
	return view;
}

beforeEach(() => {
	mocks.account = null;
	mocks.hookEnabled = [];
	mocks.param = null;
	mocks.createSession.mockReset();
	mocks.navigate.mockReset();
	mocks.refetch.mockReset();
	mocks.summaryRefetch.mockReset();
	mocks.refreshAndReload.mockReset();
	mocks.refreshAndReload.mockResolvedValue(undefined);
	vi.stubEnv("NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY", "pk_test_dummy");
	window.history.replaceState(null, "", "/seller-center/me/certifications");
});

afterEach(() => {
	cleanup();
	vi.unstubAllEnvs();
});

describe("平台審核還沒通過", () => {
	it("灰色的「平台審核通過後開放設定」，不讀收款帳戶、沒有動作", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(false);
		expect(screen.getByText("平台審核通過後開放設定")).toBeTruthy();
		expect(mocks.hookEnabled.every((enabled) => enabled === false)).toBe(true);
		expect(screen.queryByRole("button")).toBeNull();
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
	});
});

describe("開通摘要沒讀到", () => {
	it("讀取失敗：顯示錯誤與「重新整理」重讀摘要，不顯示「尚未開放」", async () => {
		await mount(false, { error: new Error("summary down") });
		expect(screen.queryByText("平台審核通過後開放設定")).toBeNull();
		expect(screen.getByRole("alert").textContent).toContain("系統錯誤");
		await act(async () => {
			fireEvent.click(screen.getByText("重新整理"));
		});
		expect(mocks.summaryRefetch).toHaveBeenCalledTimes(1);
	});

	it("讀取失敗而伺服器有句子：顯示伺服器的句子", async () => {
		const err = new ConnectError("denied", Code.PermissionDenied);
		err.details.push({
			desc: LocalizedMessageSchema,
			value: create(LocalizedMessageSchema, {
				locale: "zh-TW",
				message: "商店已暫停營業",
			}),
		});
		await mount(false, { error: err });
		expect(screen.getByRole("alert").textContent).toContain("商店已暫停營業");
	});

	it("還在載入：顯示載入中，不顯示「尚未開放」", async () => {
		await mount(false, { loading: true });
		expect(screen.getByText("載入中…")).toBeTruthy();
		expect(screen.queryByText("平台審核通過後開放設定")).toBeNull();
	});
});

describe("狀態文字來自目錄", () => {
	it("ACTION_REQUIRED：待設定、說明、動作，不說逾期或停用", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount();
		expect(screen.getByText("待設定")).toBeTruthy();
		expect(
			screen.getByText(
				"提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。",
			),
		).toBeTruthy();
		expect(screen.getByText("前往設定收款帳戶")).toBeTruthy();
		expect(document.body.textContent).not.toMatch(/逾期|停用|過期/);
	});

	it("NOT_CREATED 與 UNDER_REVIEW：重新整理呼叫 RefreshMyPaymentAccount 再重讀，不開通", async () => {
		for (const status of [
			PaymentAccountStatus.NOT_CREATED,
			PaymentAccountStatus.UNDER_REVIEW,
		]) {
			mocks.account = account(status);
			mocks.refetch.mockReset();
			mocks.refreshAndReload.mockClear();
			const view = await mount();
			await act(async () => {
				fireEvent.click(screen.getByText("重新整理"));
			});
			expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
			// 只重讀帳戶（refetch）不夠：伺服器的快取沒更新，讀到的還是舊的。
			expect(mocks.refetch).not.toHaveBeenCalled();
			expect(mocks.createSession).not.toHaveBeenCalled();
			view.unmount();
		}
	});

	it("重新整理的重讀失敗：一般的錯誤，畫面留在原狀態", async () => {
		mocks.account = account(PaymentAccountStatus.UNDER_REVIEW);
		mocks.refreshAndReload.mockRejectedValue(new Error("boom"));
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("重新整理"));
		});
		expect(screen.getByRole("alert").textContent).toBe("系統錯誤");
		expect(screen.getByText("審核中")).toBeTruthy();
	});

	it("重新整理被伺服器拒絕：顯示伺服器的句子", async () => {
		mocks.account = account(PaymentAccountStatus.UNDER_REVIEW);
		const err = new ConnectError("blocked", Code.FailedPrecondition);
		err.details.push({
			desc: LocalizedMessageSchema,
			value: create(LocalizedMessageSchema, {
				locale: "zh-TW",
				message: "商店已暫停營業，暫時無法重新整理",
			}),
		});
		mocks.refreshAndReload.mockRejectedValue(err);
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("重新整理"));
		});
		expect(screen.getByRole("alert").textContent).toBe(
			"商店已暫停營業，暫時無法重新整理",
		);
	});

	it("載入失敗的「重新整理」只是重讀 GetMyPaymentAccount", async () => {
		mocks.account = account(PaymentAccountStatus.UNSPECIFIED);
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("重新整理"));
		});
		expect(mocks.refetch).toHaveBeenCalledTimes(1);
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
	});

	it("ACTION_REQUIRED 但沒有告知開通介面：沒有按鈕", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED, {
			surface: OnboardingSurface.UNSPECIFIED,
		});
		await mount();
		expect(screen.getByText("待設定")).toBeTruthy();
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
		expect(screen.queryByRole("button")).toBeNull();
	});

	it("ENABLED 而撥款未啟用：多一行；沒有動作", async () => {
		mocks.account = account(PaymentAccountStatus.ENABLED);
		await mount();
		expect(screen.getByText("已啟用")).toBeTruthy();
		expect(
			screen.getByText("撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。"),
		).toBeTruthy();
		expect(screen.queryByRole("button")).toBeNull();
	});

	it("DISABLED：聯絡客服連到聯絡頁", async () => {
		mocks.account = account(PaymentAccountStatus.DISABLED);
		await mount();
		expect(screen.getByText("無法啟用")).toBeTruthy();
		expect(
			screen.getByText("聯絡客服").closest("a")?.getAttribute("href"),
		).toBe("/contact");
	});

	it("UNSPECIFIED：不是有效狀態，顯示一般的錯誤與重新整理", async () => {
		mocks.account = account(PaymentAccountStatus.UNSPECIFIED);
		await mount();
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.queryByText("待設定")).toBeNull();
	});
});

describe("HOSTED：導頁", () => {
	it("建立 session（帶路由的語系）後整頁導到連結", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockResolvedValue({
			surface: "hosted",
			url: "https://connect.stripe.test/setup/abc",
		});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(mocks.createSession).toHaveBeenCalledWith("en");
		expect(mocks.navigate).toHaveBeenCalledWith(
			"https://connect.stripe.test/setup/abc",
		);
		expect(screen.queryByTestId("embedded")).toBeNull();
	});

	it("建立 session 失敗：一般的錯誤，不導頁", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockRejectedValue(new Error("boom"));
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(mocks.navigate).not.toHaveBeenCalled();
	});
});

describe("EMBEDDED：內嵌元件", () => {
	it("展開元件、帶公開金鑰；不導頁、不在這裡建立 session", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED, {
			surface: OnboardingSurface.EMBEDDED,
		});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(screen.getByTestId("embedded").dataset.key).toBe("pk_test_dummy");
		expect(mocks.navigate).not.toHaveBeenCalled();
		expect(mocks.createSession).not.toHaveBeenCalled();
	});

	it("元件的 onExit：收起元件，走共用的重讀路徑（hook）一次", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED, {
			surface: OnboardingSurface.EMBEDDED,
		});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
		await act(async () => {
			fireEvent.click(screen.getByText("exit-embedded"));
		});
		expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		expect(screen.queryByTestId("embedded")).toBeNull();
	});

	it("沒有公開金鑰：一般的錯誤，不展開元件、不當掉", async () => {
		vi.stubEnv("NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY", "");
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED, {
			surface: OnboardingSurface.EMBEDDED,
		});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.queryByTestId("embedded")).toBeNull();
		expect(screen.getByText("待設定")).toBeTruthy();
	});
});

describe("回跳的 ?payment=", () => {
	it("return：走共用的重讀路徑一次，等待時顯示「正在確認資料…」，不建立 session", async () => {
		mocks.param = "return";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		let resolve: () => void = () => {};
		mocks.refreshAndReload.mockImplementation(
			() =>
				new Promise<void>((r) => {
					resolve = r;
				}),
		);
		await mount();
		expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		expect(screen.getByText("正在確認資料…")).toBeTruthy();
		await act(async () => {
			resolve();
		});
		expect(screen.queryByText("正在確認資料…")).toBeNull();
		expect(mocks.createSession).not.toHaveBeenCalled();
		expect(window.location.search).toBe("");
	});

	it("return：不輪詢，之後不再讀", async () => {
		vi.useFakeTimers();
		try {
			mocks.param = "return";
			mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
			await mount();
			await act(async () => {
				await vi.advanceTimersByTimeAsync(60_000);
			});
			expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		} finally {
			vi.useRealTimers();
		}
	});

	it("return：失敗顯示錯誤", async () => {
		mocks.param = "return";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.refreshAndReload.mockRejectedValue(new Error("failed_precondition"));
		await mount();
		expect(screen.getByRole("alert").textContent).toBe("系統錯誤");
	});

	it("refresh：重新取得連結再導過去", async () => {
		mocks.param = "refresh";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockResolvedValue({
			surface: "hosted",
			url: "https://connect.stripe.test/setup/new",
		});
		await mount();
		expect(mocks.createSession).toHaveBeenCalledTimes(1);
		expect(mocks.navigate).toHaveBeenCalledWith(
			"https://connect.stripe.test/setup/new",
		);
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
	});

	it("平台審核還沒通過時忽略 ?payment=", async () => {
		mocks.param = "return";
		await mount(false);
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
	});
});
