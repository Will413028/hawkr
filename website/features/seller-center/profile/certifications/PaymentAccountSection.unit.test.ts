// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
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

import { onboardingFixture } from "@/lib/seller/onboarding-fixtures";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { PaymentAccountSection } from "./PaymentAccountSection";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function account(
	status: PaymentAccountStatus,
	extra: { payoutsActive?: boolean } = {},
): PaymentAccount {
	return create(PaymentAccountSchema, {
		status,
		payoutsActive: extra.payoutsActive ?? false,
	});
}

/** 開通摘要的收款帳戶任務：按鈕與說明句只讀它。 */
function paymentTask(
	state: OnboardingTaskState,
	action: OnboardingAction,
	status = "伺服器的收款句",
) {
	return onboardingFixture([
		{ id: OnboardingTaskId.PAYMENT_ACCOUNT, state, action, status },
	]).tasks[0];
}

const START = paymentTask(
	OnboardingTaskState.ACTION_REQUIRED,
	OnboardingAction.START_PAYMENT_ONBOARDING,
);

async function mount(
	approved = true,
	summary: { loading?: boolean; error?: Error | null } = {},
	task: ReturnType<typeof paymentTask> | undefined = undefined,
) {
	const view = render(
		React.createElement(PaymentAccountSection, {
			platformApproved: approved,
			task,
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

describe("徽章讀帳戶狀態，說明句與按鈕只讀任務", () => {
	it("ACTION_REQUIRED + START：待設定、任務的句子、任務的按鈕，不說逾期或停用", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(true, {}, START);
		expect(screen.getByText("待設定")).toBeTruthy();
		expect(screen.getByText("伺服器的收款句")).toBeTruthy();
		expect(screen.getByText("前往設定收款帳戶")).toBeTruthy();
		expect(document.body.textContent).not.toMatch(/逾期|停用|過期/);
	});

	it("帳戶狀態與任務不一致：帳戶 ACTION_REQUIRED 但任務 NONE，沒有按鈕", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(
			true,
			{},
			paymentTask(OnboardingTaskState.IN_PROGRESS, OnboardingAction.NONE),
		);
		expect(screen.getByText("待設定")).toBeTruthy();
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
		expect(screen.queryByRole("button")).toBeNull();
		expect(screen.queryByRole("link")).toBeNull();
	});

	it("帳戶狀態與任務不一致：帳戶 NOT_CREATED 但任務 CONTACT_SUPPORT，按鈕是聯絡客服", async () => {
		mocks.account = account(PaymentAccountStatus.NOT_CREATED);
		await mount(
			true,
			{},
			paymentTask(
				OnboardingTaskState.ACTION_REQUIRED,
				OnboardingAction.CONTACT_SUPPORT,
			),
		);
		expect(screen.getByText("準備中")).toBeTruthy();
		expect(
			screen.getByText("聯絡客服").closest("a")?.getAttribute("href"),
		).toBe("/contact");
	});

	it("帳戶 NOT_CREATED 或 UNDER_REVIEW、任務 NONE：沒有任何按鈕，也沒有「重新整理」", async () => {
		for (const status of [
			PaymentAccountStatus.NOT_CREATED,
			PaymentAccountStatus.UNDER_REVIEW,
		]) {
			mocks.account = account(status);
			const view = await mount(
				true,
				{},
				paymentTask(OnboardingTaskState.IN_PROGRESS, OnboardingAction.NONE),
			);
			expect(screen.queryByRole("button")).toBeNull();
			expect(screen.queryByText("重新整理")).toBeNull();
			view.unmount();
		}
	});

	it("不認得的任務動作：沒有按鈕", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		for (const action of [
			OnboardingAction.UNSPECIFIED,
			99 as OnboardingAction,
		]) {
			const view = await mount(
				true,
				{},
				paymentTask(OnboardingTaskState.ACTION_REQUIRED, action),
			);
			expect(screen.queryByRole("button")).toBeNull();
			expect(screen.queryByRole("link")).toBeNull();
			view.unmount();
		}
	});

	it("載入失敗的「重新整理」只是重讀 GetMyPaymentAccount", async () => {
		mocks.account = account(PaymentAccountStatus.UNSPECIFIED);
		await mount(true, {}, START);
		await act(async () => {
			fireEvent.click(screen.getByText("重新整理"));
		});
		expect(mocks.refetch).toHaveBeenCalledTimes(1);
		// 只有進入頁面的那一次自動重讀，「重新整理」沒有再呼叫它。
		expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
	});

	it("ENABLED 而撥款未啟用：多一行；沒有動作", async () => {
		mocks.account = account(PaymentAccountStatus.ENABLED);
		await mount(
			true,
			{},
			paymentTask(OnboardingTaskState.COMPLETED, OnboardingAction.NONE),
		);
		expect(screen.getByText("已啟用")).toBeTruthy();
		expect(
			screen.getByText("撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。"),
		).toBeTruthy();
		expect(screen.queryByRole("button")).toBeNull();
	});

	it("UNSPECIFIED：不是有效狀態，顯示一般的錯誤與重新整理", async () => {
		mocks.account = account(PaymentAccountStatus.UNSPECIFIED);
		await mount();
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.queryByText("待設定")).toBeNull();
	});
});

describe("進入頁面自動重讀", () => {
	it("任務還沒完成：呼叫一次重讀，不顯示「正在確認資料…」，不輪詢", async () => {
		vi.useFakeTimers();
		try {
			mocks.account = account(PaymentAccountStatus.UNDER_REVIEW);
			await mount(
				true,
				{},
				paymentTask(OnboardingTaskState.IN_PROGRESS, OnboardingAction.NONE),
			);
			expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
			expect(screen.queryByText("正在確認資料…")).toBeNull();
			await act(async () => {
				await vi.advanceTimersByTimeAsync(60_000);
			});
			expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		} finally {
			vi.useRealTimers();
		}
	});

	it("任務已完成或鎖住：不重讀", async () => {
		mocks.account = account(PaymentAccountStatus.ENABLED);
		for (const state of [
			OnboardingTaskState.COMPLETED,
			OnboardingTaskState.LOCKED,
		]) {
			const view = await mount(
				true,
				{},
				paymentTask(state, OnboardingAction.NONE),
			);
			view.unmount();
		}
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
	});

	it("摘要還在載入或平台審核還沒通過：不重讀", async () => {
		await mount(true, { loading: true });
		await mount(false);
		expect(mocks.refreshAndReload).not.toHaveBeenCalled();
	});

	it("重讀失敗：不顯示錯誤，畫面照常顯示任務", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.refreshAndReload.mockRejectedValue(
			new ConnectError("busy", Code.ResourceExhausted),
		);
		await mount(true, {}, START);
		expect(screen.queryByRole("alert")).toBeNull();
		expect(screen.getByText("前往設定收款帳戶")).toBeTruthy();
	});
});

describe("HOSTED：導頁", () => {
	it("建立 session 後整頁導到連結", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockResolvedValue({
			kind: "hosted",
			url: "https://connect.stripe.test/setup/abc",
		});
		await mount(true, {}, START);
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(mocks.createSession).toHaveBeenCalledWith();
		expect(mocks.navigate).toHaveBeenCalledWith(
			"https://connect.stripe.test/setup/abc",
		);
		expect(screen.queryByTestId("embedded")).toBeNull();
	});

	it("建立 session 失敗：一般的錯誤，不導頁", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockRejectedValue(new Error("boom"));
		await mount(true, {}, START);
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(mocks.navigate).not.toHaveBeenCalled();
	});
});

const embeddedSession = () => ({
	kind: "embedded",
	provider: "stripe",
	clientSecret: "cs_1",
	publishableKey: "pk_from_server",
});

describe("EMBEDDED：內嵌元件", () => {
	it("建立 session，依伺服器給的金流商展開元件、帶伺服器給的公開金鑰；不導頁", async () => {
		mocks.createSession.mockResolvedValue(embeddedSession());
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(true, {}, START);
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(screen.getByTestId("embedded").dataset.key).toBe("pk_from_server");
		expect(mocks.navigate).not.toHaveBeenCalled();
		expect(mocks.createSession).toHaveBeenCalledTimes(1);
	});

	it("元件的 onExit：收起元件，再走共用的重讀路徑（hook）一次", async () => {
		mocks.createSession.mockResolvedValue(embeddedSession());
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(true, {}, START);
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		// 進入頁面的自動重讀已經一次；離開元件再一次。
		expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		await act(async () => {
			fireEvent.click(screen.getByText("exit-embedded"));
		});
		expect(mocks.refreshAndReload).toHaveBeenCalledTimes(2);
		expect(screen.queryByTestId("embedded")).toBeNull();
	});

	it("session 不是兩種已知的形狀：顯示既有的錯誤狀態，不展開元件", async () => {
		mocks.createSession.mockResolvedValue({ kind: "other" });
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		await mount(true, {}, START);
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
		await mount(true, {}, START);
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
			await mount(true, {}, START);
			await act(async () => {
				await vi.advanceTimersByTimeAsync(60_000);
			});
			expect(mocks.refreshAndReload).toHaveBeenCalledTimes(1);
		} finally {
			vi.useRealTimers();
		}
	});

	it("return：重讀失敗不顯示錯誤", async () => {
		mocks.param = "return";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.refreshAndReload.mockRejectedValue(
			new ConnectError("denied", Code.PermissionDenied),
		);
		await mount(true, {}, START);
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("refresh：重新取得連結再導過去", async () => {
		mocks.param = "refresh";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockResolvedValue({
			kind: "hosted",
			url: "https://connect.stripe.test/setup/new",
		});
		await mount(true, {}, START);
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
