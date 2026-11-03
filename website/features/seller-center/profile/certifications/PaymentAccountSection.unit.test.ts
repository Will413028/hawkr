// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import {
	OnboardingSurface,
	type PaymentAccount,
	PaymentAccountSchema,
	PaymentAccountStatus,
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
	fetchAccount: vi.fn(),
	navigate: vi.fn(),
	refetch: vi.fn(),
	setQueryData: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => ({ get: () => mocks.param }),
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
	...(await importOriginal<typeof import("@tanstack/react-query")>()),
	useQueryClient: () => ({ setQueryData: mocks.setQueryData }),
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "seller" }),
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
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
vi.mock("@/lib/intl/use-router", () => ({
	useLocalizedHref: (href: string) => `/en${href}`,
}));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: mocks.navigate,
}));
vi.mock("@/lib/seller/payment-account", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/payment-account")>()),
	createPaymentOnboardingSession: mocks.createSession,
	fetchMyPaymentAccount: mocks.fetchAccount,
}));
vi.mock("next/dynamic", async () => {
	const react = await import("react");
	return {
		default: () =>
			function EmbeddedStub(props: { publishableKey: string }) {
				return react.createElement("div", {
					"data-testid": "embedded",
					"data-key": props.publishableKey,
				});
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

async function mount(approved = true) {
	const view = render(
		React.createElement(PaymentAccountSection, { platformApproved: approved }),
	);
	await act(async () => {});
	return view;
}

beforeEach(() => {
	mocks.account = null;
	mocks.hookEnabled = [];
	mocks.param = null;
	mocks.createSession.mockReset();
	mocks.fetchAccount.mockReset();
	mocks.navigate.mockReset();
	mocks.refetch.mockReset();
	mocks.setQueryData.mockReset();
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

	it("NOT_CREATED 與 UNDER_REVIEW：重新整理只重讀，不開通", async () => {
		for (const status of [
			PaymentAccountStatus.NOT_CREATED,
			PaymentAccountStatus.UNDER_REVIEW,
		]) {
			mocks.account = account(status);
			mocks.refetch.mockReset();
			const view = await mount();
			await act(async () => {
				fireEvent.click(screen.getByText("重新整理"));
			});
			expect(mocks.refetch).toHaveBeenCalledTimes(1);
			expect(mocks.createSession).not.toHaveBeenCalled();
			view.unmount();
		}
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

	it("不顯示金流商的待補條件細項", async () => {
		mocks.account = create(PaymentAccountSchema, {
			status: PaymentAccountStatus.ACTION_REQUIRED,
			onboardingSurface: OnboardingSurface.HOSTED,
			requirements: { currently_due: ["individual.id_number"] },
		});
		await mount();
		expect(document.body.textContent).not.toContain("individual.id_number");
	});
});

describe("HOSTED：導頁", () => {
	it("建立 session（return 與 refresh 帶語系與 origin）後整頁導到連結", async () => {
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		mocks.createSession.mockResolvedValue({
			surface: "hosted",
			url: "https://connect.stripe.test/setup/abc",
		});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("前往設定收款帳戶"));
		});
		expect(mocks.createSession).toHaveBeenCalledWith({
			returnUrl: `${window.location.origin}/en/seller-center/me/certifications?payment=return`,
			refreshUrl: `${window.location.origin}/en/seller-center/me/certifications?payment=refresh`,
		});
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
	it("return：重讀 GetMyPaymentAccount，等待時顯示「正在確認資料…」，並寫進快取", async () => {
		mocks.param = "return";
		mocks.account = account(PaymentAccountStatus.ACTION_REQUIRED);
		let resolve: (value: PaymentAccount) => void = () => {};
		mocks.fetchAccount.mockReturnValue(
			new Promise<PaymentAccount>((r) => {
				resolve = r;
			}),
		);
		await mount();
		expect(mocks.fetchAccount).toHaveBeenCalledTimes(1);
		expect(screen.getByText("正在確認資料…")).toBeTruthy();
		await act(async () => {
			resolve(account(PaymentAccountStatus.UNDER_REVIEW));
		});
		expect(screen.queryByText("正在確認資料…")).toBeNull();
		expect(mocks.setQueryData).toHaveBeenCalled();
		expect(mocks.createSession).not.toHaveBeenCalled();
		expect(window.location.search).toBe("");
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
		expect(mocks.fetchAccount).not.toHaveBeenCalled();
	});

	it("平台審核還沒通過時忽略 ?payment=", async () => {
		mocks.param = "return";
		await mount(false);
		expect(mocks.fetchAccount).not.toHaveBeenCalled();
	});
});
