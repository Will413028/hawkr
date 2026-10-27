// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AgreementKind,
	StoreOpeningUnavailableReason,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
	OPENING_SENTENCE,
	unavailable,
} from "@/tests/support/server-unavailability";

const mocks = vi.hoisted(() => ({
	hasSeller: false,
	hasMeasures: false,
	fetchSession: vi.fn(),
	rejectSession: vi.fn(async () => {}),
	SESSION_UNAVAILABLE: Symbol("unavailable"),
	apply: vi.fn(async (_input: unknown) => ({ etag: "e1" })),
	complete: vi.fn(async (_etag: string) => {}),
	update: vi.fn(),
	opening: vi.fn(
		async (): Promise<{
			canApplyStore: boolean;
			unavailabilities: ReturnType<typeof unavailable>[];
		}> => ({ canApplyStore: true, unavailabilities: [] }),
	),
	fetchStore: vi.fn(async (): Promise<unknown> => {
		throw new ConnectError("no store", Code.NotFound);
	}),
	router: { replace: vi.fn() },
}));
vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: React.ReactNode }) =>
		React.createElement("a", { href }, children),
}));
vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useHasMeasures: () => mocks.hasMeasures,
}));
vi.mock("@/lib/analytics/events", () => ({
	trackSellerOnboardingCompleted: vi.fn(),
}));
vi.mock("@/lib/api/auth", () => ({
	fetchSession: mocks.fetchSession,
	SESSION_UNAVAILABLE: mocks.SESSION_UNAVAILABLE,
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		hydrated: true,
		isLoggedIn: true,
		hasSeller: mocks.hasSeller,
		rejectSession: mocks.rejectSession,
	}),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => mocks.router,
}));
vi.mock("@/features/legal/documents/sellerAgreements.gen", () => ({
	SELLER_AGREEMENTS: {
		SELLER_TERMS: { version: "2031-01-02", url: "/served-seller-terms" },
		KYC_PRIVACY_NOTICE: { version: "2031-01-03", url: "/served-notice" },
		PAYUNI_TERMS: { version: "2031-01-04", url: "https://example.com/p" },
	},
}));
vi.mock("@/lib/seller/repo", () => ({
	applyStore: mocks.apply,
	completeOnboarding: mocks.complete,
	fetchMyStore: mocks.fetchStore,
	fetchStoreOpening: mocks.opening,
	updateMyStore: mocks.update,
}));
vi.mock("@/components/ui", () => ({
	Input: ({
		label,
		value,
		onChange,
	}: {
		label: string;
		value: string;
		onChange: React.ChangeEventHandler<HTMLInputElement>;
	}) => React.createElement("input", { "aria-label": label, value, onChange }),
	Textarea: () => null,
	Dropdown: ({
		label,
		onChange,
	}: {
		label: string;
		onChange: (s: string) => void;
	}) =>
		React.createElement("input", {
			"aria-label": label,
			onChange: (e: React.ChangeEvent<HTMLInputElement>) =>
				onChange(e.target.value),
		}),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		type,
		disabled,
		onClick,
	}: React.ComponentProps<"button">) =>
		React.createElement(
			"button",
			{ type: type === "submit" ? "submit" : "button", disabled, onClick },
			children,
		),
}));

import { COUNTIES, townsByCounty } from "@hawkr/twaddr/lookup";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerOnboardingClient from "./SellerOnboardingClient";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

const TAICHUNG_COUNTY = COUNTIES.find((c) => c.name === "臺中市")?.code ?? "";
const HEPING_TOWN =
	townsByCounty(TAICHUNG_COUNTY).find((t) => t.name === "和平區")?.code ?? "";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	mocks.hasSeller = false;
	mocks.hasMeasures = false;
	mocks.opening.mockImplementation(async () => ({
		canApplyStore: true,
		unavailabilities: [],
	}));
	mocks.fetchStore.mockImplementation(async () => {
		throw new ConnectError("no store", Code.NotFound);
	});
});

function outdated() {
	return new ConnectError("stale", Code.FailedPrecondition, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: ErrorReason[ErrorReason.AGREEMENT_OUTDATED],
				domain: "hawkr",
			},
		},
	]);
}

function refused(reason: ErrorReason, message?: string) {
	return new ConnectError("refused", Code.FailedPrecondition, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: ErrorReason[reason], domain: "hawkr" },
		},
		...(message === undefined
			? []
			: [
					{
						desc: LocalizedMessageSchema,
						value: { locale: "zh-TW", message },
					},
				]),
	]);
}

function termsCheckbox(): HTMLInputElement {
	return screen.getByRole("checkbox") as HTMLInputElement;
}

async function acceptTerms() {
	const box = await screen.findByRole("checkbox");
	fireEvent.click(box);
	expect((box as HTMLInputElement).checked).toBe(true);
}

function fillStore(name = "新商家") {
	fireEvent.change(screen.getByLabelText("商家名稱"), {
		target: { value: name },
	});
	fireEvent.change(screen.getByLabelText("縣市"), {
		target: { value: TAICHUNG_COUNTY },
	});
	fireEvent.change(screen.getByLabelText("鄉鎮市區"), {
		target: { value: HEPING_TOWN },
	});
	fireEvent.change(screen.getByLabelText("路名門牌"), {
		target: { value: "測試路1號" },
	});
}

function submitForm(container: HTMLElement) {
	const form = container.querySelector("form");
	if (!form) throw new Error("form missing");
	fireEvent.submit(form);
}
function createButton(): HTMLButtonElement {
	return screen.getByText("建立賣場").closest("button") as HTMLButtonElement;
}

describe("建立賣場按鈕：店名、鄉鎮市區、路名門牌都有才能送出", () => {
	it("三者都填且同意契約時可以送出", async () => {
		render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		expect(createButton().disabled).toBe(false);
	});

	it("沒有選鄉鎮市區時不能送出", async () => {
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		fireEvent.change(screen.getByLabelText("縣市"), {
			target: { value: "" },
		});
		await acceptTerms();
		expect(createButton().disabled).toBe(true);
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
	});

	it("沒有填路名門牌（或只有空白）時不能送出", async () => {
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		fireEvent.change(screen.getByLabelText("路名門牌"), {
			target: { value: "  " },
		});
		expect(createButton().disabled).toBe(true);
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
		fireEvent.change(screen.getByLabelText("路名門牌"), {
			target: { value: "測試路1號" },
		});
		expect(createButton().disabled).toBe(false);
	});
});

for (const code of [
	Code.PermissionDenied,
	Code.Internal,
	Code.InvalidArgument,
	"signed-out",
] as const) {
	it(`handles post-submit session refresh result ${code}`, async () => {
		if (code === "signed-out") mocks.fetchSession.mockResolvedValue(null);
		else if (code === Code.Internal || code === Code.InvalidArgument)
			mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
		else
			mocks.fetchSession.mockRejectedValue(
				new ConnectError("refresh failed", code),
			);
		const { container } = render(React.createElement(SellerOnboardingClient));
		fireEvent.change(screen.getByLabelText("商家名稱"), {
			target: { value: "新商家" },
		});
		fireEvent.change(screen.getByLabelText("縣市"), {
			target: { value: TAICHUNG_COUNTY },
		});
		fireEvent.change(screen.getByLabelText("鄉鎮市區"), {
			target: { value: HEPING_TOWN },
		});
		fireEvent.change(screen.getByLabelText("路名門牌"), {
			target: { value: "測試路1號" },
		});
		await acceptTerms();
		const form = container.querySelector("form");
		if (!form) throw new Error("form missing");
		fireEvent.submit(form);
		if (code === Code.Internal || code === Code.InvalidArgument) {
			await waitFor(() => expect(screen.getByText("賣場已建立")).toBeTruthy());
			expect(mocks.rejectSession).not.toHaveBeenCalled();
		} else {
			await waitFor(() => expect(mocks.router.replace).toHaveBeenCalledOnce());
			expect(mocks.rejectSession).toHaveBeenCalledOnce();
			if (code === "signed-out")
				expect(mocks.rejectSession).toHaveBeenCalledWith(undefined);
			else
				expect(mocks.rejectSession).toHaveBeenCalledWith(
					code === Code.PermissionDenied
						? expect.objectContaining({ code, rawMessage: "refresh failed" })
						: undefined,
				);
			expect(screen.queryByText("賣場已建立")).toBeNull();
		}
		expect(mocks.apply).toHaveBeenCalledOnce();
		expect(mocks.apply).toHaveBeenCalledWith(
			expect.objectContaining({
				storeName: "新商家",
				address: { townCode: HEPING_TOWN, addressDetail: "測試路1號" },
			}),
		);
		expect(mocks.complete).toHaveBeenCalledOnce();
		expect(mocks.complete).toHaveBeenCalledWith("e1");
		if (code !== Code.InvalidArgument)
			expect(screen.queryByRole("alert")).toBeNull();
	});
}

it("ApplyStore 撞到 already_exists：保留輸入、不覆寫既有店家、顯示衝突提示", async () => {
	mocks.apply.mockRejectedValueOnce(
		new ConnectError("exists", Code.AlreadyExists),
	);
	mocks.fetchStore
		.mockRejectedValueOnce(new ConnectError("no store", Code.NotFound))
		.mockResolvedValueOnce({ store: { storeName: "最新店名", etag: "e9" } });
	const { container } = render(React.createElement(SellerOnboardingClient));
	fireEvent.change(screen.getByLabelText("商家名稱"), {
		target: { value: "我輸入的店名" },
	});
	fireEvent.change(screen.getByLabelText("縣市"), {
		target: { value: TAICHUNG_COUNTY },
	});
	fireEvent.change(screen.getByLabelText("鄉鎮市區"), {
		target: { value: HEPING_TOWN },
	});
	fireEvent.change(screen.getByLabelText("路名門牌"), {
		target: { value: "測試路1號" },
	});
	await acceptTerms();
	const form = container.querySelector("form");
	if (!form) throw new Error("form missing");
	fireEvent.submit(form);
	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toContain(
			"店家資料已在其他地方建立",
		),
	);
	expect(screen.getByLabelText("商家名稱")).toHaveProperty(
		"value",
		"我輸入的店名",
	);
	expect(mocks.update).not.toHaveBeenCalled();
	expect(mocks.complete).not.toHaveBeenCalled();
});

it("ApplyStore 撞到 already_exists 且重讀失敗：顯示無法載入最新版本，不說已載入", async () => {
	mocks.apply.mockRejectedValueOnce(
		new ConnectError("exists", Code.AlreadyExists),
	);
	mocks.fetchStore
		.mockRejectedValueOnce(new ConnectError("no store", Code.NotFound))
		.mockRejectedValueOnce(new ConnectError("down", Code.Unavailable));
	const { container } = render(React.createElement(SellerOnboardingClient));
	fireEvent.change(screen.getByLabelText("商家名稱"), {
		target: { value: "我輸入的店名" },
	});
	fireEvent.change(screen.getByLabelText("縣市"), {
		target: { value: TAICHUNG_COUNTY },
	});
	fireEvent.change(screen.getByLabelText("鄉鎮市區"), {
		target: { value: HEPING_TOWN },
	});
	fireEvent.change(screen.getByLabelText("路名門牌"), {
		target: { value: "測試路1號" },
	});
	await acceptTerms();
	const form = container.querySelector("form");
	if (!form) throw new Error("form missing");
	fireEvent.submit(form);
	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toContain("無法載入最新版本"),
	);
	expect(screen.getByRole("alert").textContent).not.toContain("已載入最新版本");
	expect(screen.getByLabelText("商家名稱")).toHaveProperty(
		"value",
		"我輸入的店名",
	);
});

const SUBMIT_LABEL = "建立賣場";

function submitButton(): HTMLButtonElement {
	return screen.getByRole("button", {
		name: SUBMIT_LABEL,
	}) as HTMLButtonElement;
}

it("開店：沒勾選賣家契約不能送出，勾了才能，取消勾選又不能", async () => {
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	const box = await screen.findByRole("checkbox");

	expect(submitButton().disabled).toBe(true);
	submitForm(container);
	expect(mocks.apply).not.toHaveBeenCalled();

	fireEvent.click(box);
	expect(submitButton().disabled).toBe(false);

	fireEvent.click(box);
	expect(submitButton().disabled).toBe(true);
	submitForm(container);
	expect(mocks.apply).not.toHaveBeenCalled();
});

it("開店：勾選框連到賣家契約的網址（來自生成的常數），在新分頁開啟", async () => {
	render(React.createElement(SellerOnboardingClient));
	const link = await screen.findByRole("link", { name: "Hawkr 賣家契約" });
	expect(link.getAttribute("href")).toBe("/served-seller-terms");
	expect(link.getAttribute("target")).toBe("_blank");
	expect(link.getAttribute("rel")).toContain("noopener");
});

it("開店：送出的版本是賣家看到的那一版（生成的常數）", async () => {
	mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	await acceptTerms();
	submitForm(container);
	await waitFor(() => expect(mocks.apply).toHaveBeenCalledOnce());
	expect(mocks.apply).toHaveBeenCalledWith(
		expect.objectContaining({
			acceptedAgreements: [
				{ kind: AgreementKind.SELLER_TERMS, version: "2031-01-02" },
			],
		}),
	);
});

it("開店：AGREEMENT_OUTDATED 取消勾選並提示重新整理，重新勾選前不能再送", async () => {
	mocks.apply.mockRejectedValueOnce(outdated());
	mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	await acceptTerms();
	submitForm(container);

	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toBe(
			"條款已更新，請重新整理頁面後再勾選。",
		),
	);
	expect(termsCheckbox().checked).toBe(false);
	expect(submitButton().disabled).toBe(true);
	submitForm(container);
	expect(mocks.apply).toHaveBeenCalledOnce();
	expect(mocks.complete).not.toHaveBeenCalled();
});

it("開店：STORE_OPENING_TERMINATED 顯示伺服器的句子（LocalizedMessage）並連到措施與申覆", async () => {
	mocks.apply.mockRejectedValueOnce(
		refused(ErrorReason.STORE_OPENING_TERMINATED, OPENING_SENTENCE.TERMINATED),
	);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	await acceptTerms();
	submitForm(container);
	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toContain(
			OPENING_SENTENCE.TERMINATED,
		),
	);
	expect(
		screen.getByRole("link", { name: "措施與申覆" }).getAttribute("href"),
	).toBe("/seller/measures");
	expect(screen.getByRole("alert").textContent).not.toContain("停權");
});

it("開店：STORE_OPENING_SUSPENDED 顯示伺服器的句子並連到措施與申覆", async () => {
	mocks.apply.mockRejectedValueOnce(
		refused(ErrorReason.STORE_OPENING_SUSPENDED, OPENING_SENTENCE.SUSPENDED),
	);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	await acceptTerms();
	submitForm(container);
	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toContain(
			OPENING_SENTENCE.SUSPENDED,
		),
	);
	expect(screen.getByRole("alert").textContent).not.toContain("第 21.3 條");
	expect(
		screen.getByRole("link", { name: "措施與申覆" }).getAttribute("href"),
	).toBe("/seller/measures");
});

describe("開店入口讀 GetMyStoreOpening（送出前）", () => {
	const blocked = (...unavailabilities: ReturnType<typeof unavailable>[]) =>
		mocks.opening.mockImplementation(async () => ({
			canApplyStore: false,
			unavailabilities,
		}));
	const SO = StoreOpeningUnavailableReason;

	it("can_apply_store 為 false：送出停用並顯示第一筆的伺服器句子，ApplyStore 不被呼叫", async () => {
		blocked(
			unavailable(SO.SUSPENDED, OPENING_SENTENCE.SUSPENDED),
			unavailable(SO.ACCOUNT_DELETING, OPENING_SENTENCE.ACCOUNT_DELETING),
		);
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain(
				OPENING_SENTENCE.SUSPENDED,
			),
		);
		expect(screen.getByRole("alert").textContent).not.toContain("正在刪除");
		expect(createButton().disabled).toBe(true);
		expect(
			screen.getByRole("link", { name: "措施與申覆" }).getAttribute("href"),
		).toBe("/seller/measures");
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
	});

	it("TERMINATED 先於停權；帳號刪除沒有措施與申覆連結，句子逐字", async () => {
		blocked(
			unavailable(SO.TERMINATED, OPENING_SENTENCE.TERMINATED),
			unavailable(SO.SUSPENDED, OPENING_SENTENCE.SUSPENDED),
		);
		const first = render(React.createElement(SellerOnboardingClient));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain("第 21.3 條"),
		);
		first.unmount();
		blocked(
			unavailable(SO.ACCOUNT_DELETING, OPENING_SENTENCE.ACCOUNT_DELETING),
		);
		render(React.createElement(SellerOnboardingClient));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				OPENING_SENTENCE.ACCOUNT_DELETING,
			),
		);
		expect(screen.queryByRole("link", { name: "措施與申覆" })).toBeNull();
	});

	it("原因含 STORE_EXISTS（沒有店家）：和其他原因一樣擋送出並顯示伺服器的句子", async () => {
		blocked(unavailable(SO.STORE_EXISTS, OPENING_SENTENCE.STORE_EXISTS));
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe(
				OPENING_SENTENCE.STORE_EXISTS,
			),
		);
		expect(createButton().disabled).toBe(true);
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
	});

	it.each([
		["UNSPECIFIED 且沒有句子", [unavailable(SO.UNSPECIFIED)]],
		["未知值且沒有句子", [unavailable(999)]],
		[
			"已知原因但伺服器沒有句子（client 不自己組字）",
			[unavailable(SO.SUSPENDED)],
		],
		["空原因", []],
	])("can_apply_store 為 false 但原因是%s：仍停用送出、不顯示文字、ApplyStore 不被呼叫", async (_name, unavailabilities) => {
		blocked(...unavailabilities);
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		await waitFor(() => expect(mocks.opening).toHaveBeenCalled());
		await waitFor(() => expect(createButton().disabled).toBe(true));
		expect(screen.queryByRole("alert")).toBeNull();
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
	});

	it("不認得的原因但有伺服器句子：顯示句子、不給措施與申覆連結", async () => {
		blocked(unavailable(999, "伺服器新增的原因句"));
		render(React.createElement(SellerOnboardingClient));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe("伺服器新增的原因句"),
		);
		expect(screen.queryByRole("link", { name: "措施與申覆" })).toBeNull();
	});

	it("開店可用性讀取中：送出停用，答案回來（可以開店）後才可送出", async () => {
		let resolve: (v: {
			canApplyStore: boolean;
			unavailabilities: ReturnType<typeof unavailable>[];
		}) => void = () => {};
		mocks.opening.mockImplementationOnce(
			() =>
				new Promise((r) => {
					resolve = r;
				}),
		);
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		expect(createButton().disabled).toBe(true);
		submitForm(container);
		expect(mocks.apply).not.toHaveBeenCalled();
		resolve({ canApplyStore: true, unavailabilities: [] });
		await waitFor(() => expect(createButton().disabled).toBe(false));
	});

	it("can_apply_store 為 true 才可送出；讀取失敗不擋（ApplyStore 的 ErrorReason 是後備）", async () => {
		mocks.opening.mockRejectedValueOnce(new Error("boom"));
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		await waitFor(() => expect(createButton().disabled).toBe(false));
		mocks.apply.mockRejectedValueOnce(
			refused(ErrorReason.STORE_OPENING_SUSPENDED, OPENING_SENTENCE.SUSPENDED),
		);
		submitForm(container);
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain(
				OPENING_SENTENCE.SUSPENDED,
			),
		);
	});
});

it("開店：其他 failed_precondition 不顯示開店拒絕文案", async () => {
	mocks.apply.mockRejectedValueOnce(
		refused(ErrorReason.SELLER_REQUIREMENT_MISSING),
	);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore();
	await acceptTerms();
	submitForm(container);
	await waitFor(() => expect(mocks.apply).toHaveBeenCalledOnce());
	await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
	expect(screen.queryByRole("link", { name: "措施與申覆" })).toBeNull();
	expect(screen.getByRole("alert").textContent).not.toContain("不能重新開店");
});

it("開店：already_exists 載入既有店家後，勾選框隱藏（之後走更新）", async () => {
	mocks.apply.mockRejectedValueOnce(
		new ConnectError("exists", Code.AlreadyExists),
	);
	mocks.fetchStore
		.mockRejectedValueOnce(new ConnectError("no store", Code.NotFound))
		.mockResolvedValueOnce({
			store: { id: "9", storeName: "最新店名", etag: "e9" },
		});
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore("我輸入的店名");
	await acceptTerms();
	submitForm(container);

	await waitFor(() =>
		expect(screen.getByRole("alert").textContent).toContain(
			"店家資料已在其他地方建立",
		),
	);
	expect(screen.queryByRole("checkbox")).toBeNull();
	expect(submitButton().disabled).toBe(false);
});

it("已經有店家（走更新）：不顯示勾選框，也不需要勾選就能送出", async () => {
	mocks.hasSeller = true;
	mocks.fetchStore.mockResolvedValue({
		store: {
			id: "9",
			etag: "e5",
			storeName: "既有店",
			onboardingCompleted: false,
		},
	});
	// 即使伺服器對「再開一間店」回 STORE_EXISTS，有店家時也不查、不擋。
	mocks.opening.mockImplementation(async () => ({
		canApplyStore: false,
		unavailabilities: [
			unavailable(
				StoreOpeningUnavailableReason.STORE_EXISTS,
				OPENING_SENTENCE.STORE_EXISTS,
			),
		],
	}));
	mocks.update.mockResolvedValue({ store: { id: "9", etag: "e6" } });
	mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
	const { container } = render(React.createElement(SellerOnboardingClient));
	fillStore("既有店改名");
	await waitFor(() => expect(submitButton().disabled).toBe(false));
	expect(screen.queryByRole("checkbox")).toBeNull();
	expect(screen.queryByRole("alert")).toBeNull();

	submitForm(container);
	await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
	expect(mocks.apply).not.toHaveBeenCalled();
	expect(mocks.opening).not.toHaveBeenCalled();
});

describe("措施紀錄提示卡", () => {
	const MEASURES_LINK = "你有措施紀錄，查看與申覆";

	it("有措施時，在頁首與表單之間顯示整張連到措施與申覆頁的連結", () => {
		mocks.hasMeasures = true;
		const { container } = render(React.createElement(SellerOnboardingClient));
		const link = screen.getByRole("link", { name: MEASURES_LINK });
		expect(link.getAttribute("href")).toBe("/seller/measures");
		const heading = screen.getByRole("heading", { level: 1 });
		const form = container.querySelector("form") as HTMLElement;
		expect(
			heading.compareDocumentPosition(link) & Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
		expect(
			link.compareDocumentPosition(form) & Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
	});

	it("讀取中、失敗或沒有措施（hook 回 false）時不顯示，也不擋開店", async () => {
		render(React.createElement(SellerOnboardingClient));
		expect(screen.queryByRole("link", { name: MEASURES_LINK })).toBeNull();
		fillStore();
		await acceptTerms();
		expect(createButton().disabled).toBe(false);
	});

	it("建立完成（done 步驟）之後不顯示", async () => {
		mocks.hasMeasures = true;
		mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
		const { container } = render(React.createElement(SellerOnboardingClient));
		fillStore();
		await acceptTerms();
		submitForm(container);
		await waitFor(() => expect(screen.getByText("賣場已建立")).toBeTruthy());
		expect(screen.queryByRole("link", { name: MEASURES_LINK })).toBeNull();
	});
});
