// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AgreementKind,
	KYCApplicationSchema,
	KYCField,
	KYCIssueCode,
	KYCStatus,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
	StoreField,
	StoreSchema,
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
import { onboardingFixture } from "@/lib/seller/onboarding-fixtures";

type AppInit = Parameters<typeof create<typeof KYCApplicationSchema>>[1];

const kycStore = vi.hoisted(() => {
	const listeners = new Set<() => void>();
	let current: unknown = null;
	return {
		get: () => current,
		set: (next: unknown) => {
			current = next;
			for (const l of listeners) l();
		},
		subscribe: (l: () => void) => {
			listeners.add(l);
			return () => listeners.delete(l);
		},
	};
});

const SERVED = vi.hoisted(() => ({
	notice: { version: "2031-01-03", url: "/served-notice" },
}));

const mocks = vi.hoisted(() => ({
	type: null as string | null,
	store: { etag: "s1" } as Record<string, unknown>,
	saveKycDraft: vi.fn(),
	fetchMyKyc: vi.fn(),
	submitKyc: vi.fn(),
	cancelKyc: vi.fn(),
	updateMyStore: vi.fn(),
	fetchMyStore: vi.fn(),
	replace: vi.fn(),
	scrollIntoView: vi.fn(),
	contact: null as { email: string; source: string } | null,
	contactLoadError: null as Error | null,
	setContactEmail: vi.fn(),
	refetchContact: vi.fn(),
	/** GetMyOnboarding 的回應；null 是還沒讀到或讀取失敗。 */
	onboarding: null as unknown,
	onboardingLoading: false,
	onboardingError: null as Error | null,
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => ({ get: () => mocks.type }),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "signedIn" }),
}));
vi.mock("@/lib/hooks/useSellerSelf", async () => {
	const react = await import("react");
	return {
		useSellerSelf: () => ({
			data: {
				store: mocks.store,
				capabilities: [{ capability: 16, active: false, missing: [3] }],
			},
			loading: false,
			error: null,
			refresh: async () => {},
		}),
		useMyKyc: () => ({
			application: react.useSyncExternalStore(
				kycStore.subscribe,
				kycStore.get,
				kycStore.get,
			),
			loading: false,
			error: null,
		}),
		useMyOnboarding: () => ({
			onboarding: mocks.onboarding,
			loading: mocks.onboardingLoading,
			error: mocks.onboardingError,
			refetch: async () => {},
		}),
		useSetMyKyc: () => kycStore.set,
		useMyPaymentAccount: () => ({
			account: null,
			loading: false,
			error: null,
			refetch: async () => {},
		}),
		useInvalidateSellerSelf: () => async () => {},
	};
});
vi.mock("@/lib/hooks/useContactEmail", () => ({
	useContactEmail: () => ({
		contact: mocks.contact,
		loading: false,
		error: mocks.contactLoadError,
		refetch: mocks.refetchContact,
	}),
}));
vi.mock("@/lib/seller/contact-email", async (importActual) => ({
	...(await importActual<typeof import("@/lib/seller/contact-email")>()),
	setContactEmail: mocks.setContactEmail,
}));
vi.mock("./PaymentAccountSection", () => ({
	PaymentAccountSection: ({
		platformApproved,
		summary,
	}: {
		platformApproved: boolean;
		summary: { loading: boolean; error: Error | null };
	}) =>
		React.createElement("div", {
			"data-testid": "payment-section",
			"data-approved": platformApproved ? "true" : "false",
			"data-summary-loading": summary.loading ? "true" : "false",
			"data-summary-error": summary.error?.message ?? "",
		}),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ replace: mocks.replace }),
}));
vi.mock("@/features/legal/documents/sellerAgreements.gen", () => ({
	SELLER_AGREEMENTS: {
		SELLER_TERMS: { version: "2031-01-02", url: "/seller-terms" },
		KYC_PRIVACY_NOTICE: SERVED.notice,
	},
}));
vi.mock("@/lib/seller/repo", () => ({
	updateMyStore: mocks.updateMyStore,
	fetchMyStore: mocks.fetchMyStore,
}));
vi.mock("@/lib/media/secure-upload", () => ({
	uploadSecureDocument: async () => ({ stagedToken: "tok" }),
}));
vi.mock("@/lib/seller/kyc", async (importActual) => ({
	...(await importActual<typeof import("@/lib/seller/kyc")>()),
	saveKycDraft: mocks.saveKycDraft,
	fetchMyKyc: mocks.fetchMyKyc,
	submitKyc: mocks.submitKyc,
	cancelKyc: mocks.cancelKyc,
	lookupBusinessRegistration: async () => null,
}));
vi.mock("@/components/shared/BackLink", () => ({ BackLink: () => null }));
vi.mock("@/components/shared/BrandLogo", () => ({ default: () => null }));
vi.mock("@/components/shared/BrandLogoLoader", () => ({ default: () => null }));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children }: { children: React.ReactNode }) =>
		React.createElement("a", null, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		onClick,
		disabled,
	}: {
		children: React.ReactNode;
		onClick?: () => void;
		disabled?: boolean;
	}) =>
		React.createElement(
			"button",
			{ type: "button", onClick, disabled },
			children,
		),
}));
vi.mock("@/components/ui/DatePicker", () => ({
	DatePicker: ({
		label,
		isRequired,
	}: {
		label: string;
		isRequired?: boolean;
	}) =>
		React.createElement("input", {
			"aria-label": label,
			"aria-required": isRequired ? "true" : undefined,
			readOnly: true,
		}),
}));
vi.mock("@/components/ui/Dropdown", () => ({
	Dropdown: ({ label, isRequired }: { label: string; isRequired?: boolean }) =>
		React.createElement("select", {
			"aria-label": label,
			"aria-required": isRequired ? "true" : undefined,
		}),
}));
vi.mock("@/components/ui/ImageUploader", () => ({
	ImageUploader: ({
		label,
		required,
	}: {
		label?: string;
		required?: boolean;
	}) =>
		React.createElement("div", {
			"data-uploader": label ?? "doc",
			"data-required": required ? "true" : undefined,
		}),
}));
vi.mock("@/components/ui/Input", () => ({
	Input: ({
		label,
		value,
		onChange,
		readOnly,
		errorMessage,
	}: {
		label: string;
		value: string;
		onChange: React.ChangeEventHandler<HTMLInputElement>;
		readOnly?: boolean;
		errorMessage?: string;
	}) =>
		React.createElement(
			React.Fragment,
			null,
			React.createElement("input", {
				"aria-label": label,
				value,
				onChange,
				readOnly,
			}),
			errorMessage
				? React.createElement(
						"small",
						{ "data-field-error": label },
						errorMessage,
					)
				: null,
		),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import CertificationsClient from "./CertificationsClient";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function company(companyName: string) {
	return { case: "business", value: { companyName } } as const;
}

function app(init: AppInit = {}) {
	const base: AppInit = {
		etag: "e1",
		status: KYCStatus.KYC_STATUS_DRAFT,
		subject: { case: "business", value: {} },
	};
	return create(KYCApplicationSchema, { ...base, ...init } as AppInit);
}

function conflict() {
	return new ConnectError("stale", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: ErrorReason[ErrorReason.VERSION_CONFLICT],
				domain: "hawkr",
			},
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "資料已被更新" },
		},
	]);
}

async function mount() {
	const view = render(React.createElement(CertificationsClient));
	await act(async () => {});
	return view;
}

async function typeInto(label: string, value: string) {
	fireEvent.change(screen.getByLabelText(label), { target: { value } });
	await act(async () => {});
}

async function autosave() {
	await act(async () => {
		await vi.advanceTimersByTimeAsync(1100);
	});
}

const ACCEPTED_KYC = [
	{ kind: AgreementKind.KYC_PRIVACY_NOTICE, version: SERVED.notice.version },
];

const NOTICE_LABEL = /賣家身分認證個人資料蒐集告知事項/;

function noticeBox(): HTMLInputElement {
	return screen.getByLabelText(NOTICE_LABEL) as HTMLInputElement;
}

async function acceptAgreements() {
	if (!noticeBox().checked) fireEvent.click(noticeBox());
	await act(async () => {});
}

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

beforeEach(() => {
	vi.useFakeTimers();
	mocks.type = null;
	mocks.onboarding = null;
	mocks.onboardingLoading = false;
	mocks.onboardingError = null;
	Element.prototype.scrollIntoView = mocks.scrollIntoView;
	window.confirm = () => true;
	kycStore.set(null);
	mocks.store = { etag: "s1" };
	mocks.updateMyStore.mockReset();
	mocks.updateMyStore.mockImplementation(
		async (update: Record<string, unknown>) => ({
			etag: "s2",
			store: create(StoreSchema, {
				...mocks.store,
				...update,
				etag: "s2",
			} as Parameters<typeof create<typeof StoreSchema>>[1]),
		}),
	);
	mocks.contact = { email: "login@example.com", source: "verified" };
	mocks.contactLoadError = null;
	mocks.setContactEmail.mockReset();
	mocks.refetchContact.mockReset();
	mocks.fetchMyStore.mockReset();
	mocks.fetchMyKyc.mockReset();
	mocks.fetchMyKyc.mockImplementation(async () => kycStore.get());
});

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.clearAllMocks();
	mocks.saveKycDraft.mockReset();
	mocks.fetchMyKyc.mockReset();
	mocks.submitKyc.mockReset();
});

describe("續填：由 GetMyKYC 還原", () => {
	it("既有草稿直接進表單（不是選型態），並捲到 missing_fields 的第一個區塊", async () => {
		kycStore.set(
			app({
				subject: company("pay@example.com"),
				missingFields: [
					KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME,
					KYCField.KYC_FIELD_BUSINESS_TAX_ID,
				],
			}),
		);
		await mount();
		expect(screen.getByLabelText("公司名稱")).toHaveProperty(
			"value",
			"pay@example.com",
		);
		expect(screen.queryByText("選擇商家類型")).toBeNull();
		await act(async () => {});
		const scrolled = mocks.scrollIntoView.mock.instances.map(
			(el) => (el as HTMLElement).id,
		);
		expect(scrolled).toEqual(["cert-company"]);
	});

	it("缺項換了，續填位置跟著換", async () => {
		kycStore.set(app({ missingFields: [KYCField.KYC_FIELD_WEBSITE_URL] }));
		await mount();
		const scrolled = mocks.scrollIntoView.mock.instances.map(
			(el) => (el as HTMLElement).id,
		);
		expect(scrolled).toEqual(["cert-other"]);
	});

	it("被退回的申請：顯示 last_rejection 的原因，與 outstanding_issues 逐項的說明", async () => {
		kycStore.set(
			app({
				status: KYCStatus.KYC_STATUS_REJECTED,
				lastRejection: { reason: "公司資料不符" },
				outstandingIssues: [
					{
						field: KYCField.KYC_FIELD_BUSINESS_TAX_ID,
						code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
						message: "與登記不符",
					},
				],
				subject: { case: "business", value: {} },
			}),
		);
		await mount();
		expect(screen.getByText("需修改")).toBeTruthy();
		expect(screen.getByText("公司資料不符")).toBeTruthy();
		expect(screen.getByText("統一編號：與登記不符")).toBeTruthy();
	});

	it("退回之後回到草稿：逐項說明看 outstanding_issues，處理完之後總理由仍在，直到重送（伺服器不再回 last_rejection）為止", async () => {
		const draft = (
			outstandingIssues: NonNullable<AppInit>["outstandingIssues"],
		) =>
			app({
				status: KYCStatus.KYC_STATUS_DRAFT,
				lastRejection: { reason: "統編不符" },
				outstandingIssues,
				subject: { case: "business", value: {} },
			});
		kycStore.set(
			draft([
				{
					field: KYCField.KYC_FIELD_BUSINESS_TAX_ID,
					code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
					message: "請與登記一致",
				},
			]),
		);
		await mount();
		expect(screen.getByText("統編不符")).toBeTruthy();
		expect(screen.getAllByText("請與登記一致").length).toBeGreaterThan(0);
		cleanup();
		kycStore.set(draft([]));
		await mount();
		expect(screen.getByText("統編不符")).toBeTruthy();
		expect(screen.queryByText("請與登記一致")).toBeNull();
	});

	it("審核中是只讀的狀態頁，不是表單", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_SUBMITTED }));
		await mount();
		expect(screen.queryByLabelText("公司名稱")).toBeNull();
		expect(screen.getByText("審核中")).toBeTruthy();
	});
});

describe("表單欄位", () => {
	it("企業：沒有組織型態、營運說明、身分與銀行；有聯絡人姓名、手機與客服電子信箱、統編與公司電話", async () => {
		kycStore.set(app({ subject: { case: "business", value: {} } }));
		await mount();
		expect(screen.queryByLabelText("組織型態")).toBeNull();
		expect(screen.queryByLabelText(/營運說明/)).toBeNull();
		expect(document.body.textContent).not.toMatch(/組織型態|營運說明/);
		for (const label of [
			"統一編號",
			"公司電話",
			"聯絡人姓名",
			"聯絡人手機",
			"客服電子信箱",
			"公司名稱",
			"主要商品類型",
			"公司地址",
		]) {
			expect(screen.getByLabelText(label), label).toBeTruthy();
		}
		for (const label of [
			"負責人身分證字號",
			"銀行名稱",
			"銀行帳號",
			"英文姓名",
		]) {
			expect(screen.queryByLabelText(label), label).toBeNull();
		}
	});

	it("個人：沒有公司區塊與補充文件，有確認送出區與金流商收集身分的說明", async () => {
		kycStore.set(app({ subject: { case: "individual", value: {} } }));
		await mount();
		expect(screen.queryByLabelText("統一編號")).toBeNull();
		expect(screen.queryByText("補充文件（選填）")).toBeNull();
		expect(screen.getByLabelText("客服電子信箱")).toBeTruthy();
		expect(screen.queryByLabelText("公司名稱")).toBeNull();
		expect(
			screen.getByText(
				"身分與銀行帳號會在審核通過後，由金流商收集，平台不經手。",
			),
		).toBeTruthy();
	});
});

describe("SaveKYCDraft", () => {
	it("還沒有申請：選型態後不帶 etag 建立", async () => {
		mocks.saveKycDraft.mockResolvedValue(app({ etag: "e1" }));
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("開始填寫"));
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		const request = mocks.saveKycDraft.mock.calls[0]?.[0];
		expect(request.application.etag).toBe("");
		expect(request.application.subject.case).toBe("individual");
	});

	it("已經有申請：存檔帶 etag，並用回應的新 etag 接著寫", async () => {
		kycStore.set(app({ etag: "e1" }));
		mocks.saveKycDraft
			.mockResolvedValueOnce(app({ etag: "e2", subject: company("a@x.co") }))
			.mockResolvedValueOnce(app({ etag: "e3", subject: company("b@x.co") }));
		await mount();
		await typeInto("公司名稱", "a@x.co");
		await autosave();
		expect(mocks.saveKycDraft.mock.calls[0]?.[0].application.etag).toBe("e1");
		expect(mocks.saveKycDraft.mock.calls[0]?.[0].updateMask.paths).toEqual([
			"business.company_name",
		]);
		await typeInto("公司名稱", "b@x.co");
		await autosave();
		expect(mocks.saveKycDraft.mock.calls[1]?.[0].application.etag).toBe("e2");
	});

	it("聯絡資料不是申請的欄位：輸入不進草稿", async () => {
		kycStore.set(app());
		await mount();
		await typeInto("聯絡人姓名", "王小明");
		await autosave();
		expect(mocks.saveKycDraft).not.toHaveBeenCalled();
	});

	it("版本衝突：保留使用者輸入、載入最新版本，下一次存檔用新的 etag", async () => {
		kycStore.set(app({ etag: "e1", subject: { case: "business", value: {} } }));
		mocks.saveKycDraft
			.mockRejectedValueOnce(conflict())
			.mockResolvedValueOnce(app({ etag: "e10" }));
		mocks.fetchMyKyc.mockResolvedValue(
			app({
				etag: "e9",
				productType: "Latest",
				subject: { case: "business", value: {} },
			}),
		);
		await mount();
		await typeInto("公司名稱", "mine@example.com");
		await autosave();
		expect(screen.getByLabelText("公司名稱")).toHaveProperty(
			"value",
			"mine@example.com",
		);
		expect(screen.getByLabelText("主要商品類型")).toHaveProperty(
			"value",
			"Latest",
		);
		expect(screen.getByRole("alert").textContent).toContain(
			"你輸入的內容仍保留",
		);
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(2);
		const retry = mocks.saveKycDraft.mock.calls[1]?.[0];
		expect(retry.application.etag).toBe("e9");
		expect(retry.application.subject.value.companyName).toBe(
			"mine@example.com",
		);
	});
});

describe("SubmitKYC", () => {
	it("先把暫存存完，再以最新的 etag 送出", async () => {
		kycStore.set(app({ etag: "e1", missingFields: [] }));
		mocks.saveKycDraft.mockResolvedValue(
			app({
				etag: "e2",
				subject: company("wang@example.com"),
				missingFields: [],
			}),
		);
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.submitKyc).toHaveBeenCalledTimes(1);
		expect(mocks.submitKyc).toHaveBeenCalledWith("e2", ACCEPTED_KYC);
		expect(mocks.replace).toHaveBeenCalledWith("/seller-center/dashboard");
	});

	it("缺項由伺服器的 missing_fields 決定：還有缺項不能送出", async () => {
		kycStore.set(
			app({ missingFields: [KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME] }),
		);
		await mount();
		expect(screen.getByText("尚需 1 項必填")).toBeTruthy();
		expect((screen.getByText("送出認證") as HTMLButtonElement).disabled).toBe(
			true,
		);
		expect(mocks.submitKyc).not.toHaveBeenCalled();
	});
});

function rejectedApp(init: AppInit = {}) {
	return app({
		status: KYCStatus.KYC_STATUS_REJECTED,
		submitTime: timestampFromMs(1000),
		lastRejection: {
			reason: "統編資料不符",
			decideTime: timestampFromMs(2000),
		},
		outstandingIssues: [
			{
				field: KYCField.KYC_FIELD_BUSINESS_TAX_ID,
				code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
				message: "與登記不符",
			},
		],
		missingFields: [],
		...init,
	} as AppInit);
}

function unavailable() {
	return new ConnectError("down", Code.Unavailable);
}

function invalidField(field: string, description: string) {
	return new ConnectError("bad", Code.InvalidArgument, undefined, [
		{
			desc: BadRequestSchema,
			value: {
				fieldViolations: [
					{
						field,
						description,
						localizedMessage: { locale: "zh-TW", message: description },
					},
				],
			},
		},
	]);
}

describe("D1：自動存檔的失敗不會無限重試", () => {
	it("欄位錯誤：標在那個欄位上、不重試，使用者改了欄位才再存", async () => {
		kycStore.set(app());
		mocks.saveKycDraft
			.mockRejectedValueOnce(
				invalidField("application.business.company_name", "此信箱無法收信"),
			)
			.mockResolvedValueOnce(
				app({ etag: "e2", subject: company("wang@example.com") }),
			);
		await mount();
		await typeInto("公司名稱", "bad@example.com");
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		expect(
			document.querySelector('[data-field-error="公司名稱"]')?.textContent,
		).toBe("此信箱無法收信");
		expect(screen.getByRole("alert").textContent).toContain("公司名稱");
		await act(async () => {
			await vi.advanceTimersByTimeAsync(60_000);
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(2);
		expect(document.querySelector('[data-field-error="公司名稱"]')).toBeNull();
	});

	it("伺服器拒絕補充文件：提示顯示伺服器對那一張的訊息，不說格式不正確", async () => {
		kycStore.set(app({ subject: { case: "business", value: {} } }));
		mocks.saveKycDraft.mockRejectedValueOnce(
			new ConnectError("bad", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "application.business.company_name",
								description: "此信箱無法收信",
								localizedMessage: {
									locale: "zh-TW",
									message: "此信箱無法收信",
								},
							},
							{
								field: "application.business.company_documents[1]",
								description: "圖片無法使用，請重新上傳",
								localizedMessage: {
									locale: "zh-TW",
									message: "圖片無法使用，請重新上傳",
								},
							},
						],
					},
				},
			]),
		);
		await mount();
		await typeInto("公司名稱", "bad@example.com");
		await autosave();
		expect(screen.getByRole("alert").textContent).toBe(
			"下列欄位格式不正確：公司名稱；補充文件（第 2 張）：圖片無法使用，請重新上傳",
		);
	});

	it("暫時性失敗：不自動重試，使用者再改欄位才存", async () => {
		kycStore.set(app());
		mocks.saveKycDraft.mockRejectedValue(unavailable());
		await mount();
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		await act(async () => {
			await vi.advanceTimersByTimeAsync(120_000);
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		expect(screen.getByRole("alert").textContent).toBeTruthy();
		mocks.saveKycDraft.mockResolvedValue(app({ etag: "e2" }));
		await typeInto("公司名稱", "other@example.com");
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(2);
	});

	it("存檔失敗之後可以送出：送出前再存一次", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.saveKycDraft
			.mockRejectedValueOnce(unavailable())
			.mockResolvedValueOnce(
				app({
					etag: "e2",
					subject: company("wang@example.com"),
					missingFields: [],
				}),
			);
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(2);
		expect(mocks.submitKyc).toHaveBeenCalledWith("e2", ACCEPTED_KYC);
	});

	it("連續失敗的存檔每次編輯只送一次，不疊出重送", async () => {
		kycStore.set(app());
		mocks.saveKycDraft.mockRejectedValue(unavailable());
		await mount();
		await typeInto("公司名稱", "a@x.co");
		await autosave();
		await typeInto("公司名稱", "ab@x.co");
		await autosave();
		await typeInto("公司名稱", "abc@x.co");
		await autosave();
		await act(async () => {
			await vi.advanceTimersByTimeAsync(120_000);
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(3);
	});
});

describe("審核中與已通過不能編輯", () => {
	it("已通過：是狀態頁，沒有表單與送出入口", async () => {
		kycStore.set(
			app({
				status: KYCStatus.KYC_STATUS_APPROVED,
				missingFields: [],
			} as AppInit),
		);
		await mount();
		expect(screen.queryByLabelText("公司名稱")).toBeNull();
		expect(screen.getByText("平台審核")).toBeTruthy();
	});
});

describe("D2：退回原因在存檔之後還在", () => {
	it("被退回的申請第一次自動存檔（回到草稿、回應帶 last_rejection）後仍顯示原因與欄位", async () => {
		kycStore.set(rejectedApp());
		mocks.saveKycDraft.mockResolvedValue(
			rejectedApp({
				etag: "e2",
				subject: company("wang@example.com"),
				status: KYCStatus.KYC_STATUS_DRAFT,
			}),
		);
		await mount();
		expect(screen.getByText("統編資料不符")).toBeTruthy();
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		expect(screen.getByText("統編資料不符")).toBeTruthy();
		expect(screen.getByText("統一編號：與登記不符")).toBeTruthy();
	});

	it("重新整理後（流程是草稿、伺服器還有待處理的問題）仍顯示原因", async () => {
		kycStore.set(
			rejectedApp({
				status: KYCStatus.KYC_STATUS_DRAFT,
			}),
		);
		await mount();
		expect(screen.getByText("統編資料不符")).toBeTruthy();
	});

	it("重送、核准後回到草稿：伺服器不回 last_rejection，不顯示退回橫幅", async () => {
		kycStore.set(
			rejectedApp({
				status: KYCStatus.KYC_STATUS_DRAFT,
				lastRejection: undefined,
				outstandingIssues: [],
			}),
		);
		await mount();
		expect(screen.queryByText("統編資料不符")).toBeNull();
		expect(screen.queryByText("認證未通過")).toBeNull();
	});
});

describe("D3：被退回的申請送出", () => {
	it("被退回、沒有改任何欄位：直接送出，不先存檔", async () => {
		kycStore.set(rejectedApp({ etag: "e1" }));
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.saveKycDraft).not.toHaveBeenCalled();
		expect(mocks.submitKyc).toHaveBeenCalledTimes(1);
		expect(mocks.submitKyc).toHaveBeenCalledWith("e1", ACCEPTED_KYC);
		expect(mocks.replace).toHaveBeenCalledWith("/seller-center/dashboard");
	});

	it("被退回、有改欄位：存檔本身就回到草稿，不多送一次", async () => {
		kycStore.set(rejectedApp({ etag: "e1" }));
		mocks.saveKycDraft.mockResolvedValue(
			app({
				etag: "e2",
				subject: company("wang@example.com"),
				missingFields: [],
			}),
		);
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await typeInto("公司名稱", "wang@example.com");
		await autosave();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.saveKycDraft).toHaveBeenCalledTimes(1);
		expect(mocks.saveKycDraft.mock.calls[0]?.[0].updateMask.paths).toEqual([
			"business.company_name",
		]);
		expect(mocks.submitKyc).toHaveBeenCalledWith("e2", ACCEPTED_KYC);
	});
});

describe("D4：開頁不寫入", () => {
	it("被退回的企業申請：店家有電話，開頁也不存檔、不改欄位", async () => {
		mocks.store = { etag: "s1", contactPhone: "0223456789" };
		kycStore.set(rejectedApp({ subject: { case: "business", value: {} } }));
		await mount();
		await act(async () => {
			await vi.advanceTimersByTimeAsync(10_000);
		});
		expect(mocks.saveKycDraft).not.toHaveBeenCalled();
		expect(screen.getByLabelText("公司電話")).toHaveProperty("value", "");
	});
});

/** 摘要（伺服器）裡平台審核的狀態。 */
function summary(state: OnboardingTaskState) {
	mocks.onboarding = onboardingFixture([
		{
			id: OnboardingTaskId.PLATFORM_REVIEW,
			state,
			action:
				state === OnboardingTaskState.COMPLETED
					? OnboardingAction.NONE
					: OnboardingAction.SUBMIT_CERTIFICATION,
		},
	]);
}

describe("兩區狀態頁（D15）", () => {
	it("審核中：平台審核區顯示目錄的徽章與送出時間，可以取消；收款帳戶區是灰色的", async () => {
		kycStore.set(
			app({
				status: KYCStatus.KYC_STATUS_SUBMITTED,
				submitTime: timestampFromMs(Date.UTC(2026, 10, 3, 6, 20)),
			}),
		);
		summary(OnboardingTaskState.IN_PROGRESS);
		await mount();
		expect(screen.getByText("審核中")).toBeTruthy();
		expect(screen.getByText(/送出時間：2026\/11\/03 14:20/)).toBeTruthy();
		expect(screen.getByText("取消申請")).toBeTruthy();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe(
			"false",
		);
	});

	it("已通過（平台審核核准）：徽章是已通過、有核准時間，收款帳戶區開放", async () => {
		kycStore.set(
			app({
				status: KYCStatus.KYC_STATUS_APPROVED,
				decideTime: timestampFromMs(Date.UTC(2026, 10, 3, 6, 20)),
			}),
		);
		summary(OnboardingTaskState.COMPLETED);
		await mount();
		expect(screen.getByText("已通過")).toBeTruthy();
		expect(screen.getByText(/核准時間：2026\/11\/03 14:20/)).toBeTruthy();
		expect(screen.queryByText("取消申請")).toBeNull();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe("true");
	});

	it("收款帳戶區是否開放只看摘要：申請已核准而摘要說平台審核還沒完成，區塊仍是灰的", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		summary(OnboardingTaskState.ACTION_REQUIRED);
		await mount();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe(
			"false",
		);
	});

	it("收款帳戶區是否開放只看摘要：申請還在審核而摘要說平台審核已完成，區塊開放", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_SUBMITTED }));
		summary(OnboardingTaskState.COMPLETED);
		await mount();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe("true");
	});

	it("摘要沒讀到或平台審核是 UNKNOWN：不當成已完成", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		summary(OnboardingTaskState.UNKNOWN);
		const view = await mount();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe(
			"false",
		);
		view.unmount();
		mocks.onboarding = null;
		await mount();
		expect(screen.getByTestId("payment-section").dataset.approved).toBe(
			"false",
		);
	});

	it("摘要還在載入：只有收款帳戶區等它，頁面其餘部分照常畫出", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		mocks.onboardingLoading = true;
		await mount();
		expect(screen.getByTestId("payment-section").dataset.summaryLoading).toBe(
			"true",
		);
		expect(screen.getByText("已通過")).toBeTruthy();
	});

	it("摘要讀取失敗：把錯誤交給收款帳戶區，不由頁面推導平台審核", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		mocks.onboarding = null;
		mocks.onboardingError = new Error("summary down");
		await mount();
		const section = screen.getByTestId("payment-section");
		expect(section.dataset.summaryError).toBe("summary down");
		expect(section.dataset.approved).toBe("false");
	});

	it("摘要有舊資料而背景重讀失敗：不當成讀取失敗", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		summary(OnboardingTaskState.COMPLETED);
		mocks.onboardingError = new Error("blip");
		await mount();
		const section = screen.getByTestId("payment-section");
		expect(section.dataset.summaryError).toBe("");
		expect(section.dataset.approved).toBe("true");
	});

	it("取消申請：確認之後呼叫 CancelKYC，回到選型態", async () => {
		kycStore.set(app({ etag: "e5", status: KYCStatus.KYC_STATUS_SUBMITTED }));
		mocks.cancelKyc.mockResolvedValue({});
		await mount();
		await act(async () => {
			fireEvent.click(screen.getByText("取消申請"));
		});
		expect(mocks.cancelKyc).toHaveBeenCalledWith("e5");
		expect(screen.getByText("選擇商家類型")).toBeTruthy();
	});

	it("類型選擇的副標：個人沒有需求副標，企業只列公司資訊與統一編號", async () => {
		kycStore.set(null);
		await mount();
		const card = (name: string) =>
			screen.getByText(name).closest("button") as HTMLElement;
		const subtitles = (name: string) =>
			[...card(name).querySelectorAll("small")].map((el) => el.textContent);
		expect(subtitles("個人賣家")).toEqual(["以個人名義販售農產品 / 自家產品"]);
		expect(subtitles("企業用戶")).toEqual([
			"以行號或公司名義販售（需統一編號）",
			"需公司資訊與統一編號",
		]);
	});

	it("畫面不寫死 PayUni 與逾期、停用的字眼", async () => {
		kycStore.set(app({ status: KYCStatus.KYC_STATUS_APPROVED }));
		await mount();
		expect(document.body.textContent).not.toMatch(/PayUni|逾期|停用/);
	});
});

describe("D8：必填標記與伺服器一致", () => {
	it("表單沒有身分、銀行、證件與英文名稱", async () => {
		kycStore.set(app({ subject: { case: "business", value: {} } }));
		await mount();
		for (const label of [
			"真實姓名",
			"英文姓名",
			"身分證字號",
			"負責人身分證字號",
			"出生日期",
			"銀行名稱",
			"銀行帳號",
		]) {
			expect(screen.queryByLabelText(label), label).toBeNull();
		}
		expect(document.querySelectorAll("[data-uploader]")).toHaveLength(1);
		expect(
			[...document.querySelectorAll<HTMLElement>("[data-uploader]")].map(
				(el) => el.dataset.required,
			),
		).toEqual([undefined]);
		expect(document.body.textContent).not.toMatch(/PayUni|身分證正面|存摺/);
	});
});

describe("公司與店家地址：必填標記與標籤", () => {
	it("公司地址不要求、店家地址要求：標籤各自區分", async () => {
		kycStore.set(app({ subject: { case: "business", value: {} } }));
		await mount();
		for (const label of ["公司縣市", "公司鄉鎮市區"]) {
			expect(screen.getByLabelText(label).getAttribute("aria-required")).toBe(
				null,
			);
		}
		for (const label of ["商家所在縣市", "商家鄉鎮市區"]) {
			expect(screen.getByLabelText(label).getAttribute("aria-required")).toBe(
				"true",
			);
		}
	});
});

describe("店家聯絡資料：自動儲存與缺項", () => {
	it("缺項只由伺服器的 missing_store_fields 決定：輸入不增減，寫入之後重讀申請才更新", async () => {
		kycStore.set(
			app({
				missingFields: [],
				missingStoreFields: [StoreField.CONTACT_NAME, StoreField.SUPPORT_EMAIL],
			}),
		);
		mocks.fetchMyKyc.mockResolvedValue(
			app({
				missingFields: [],
				missingStoreFields: [StoreField.SUPPORT_EMAIL],
			}),
		);
		await mount();
		expect(screen.getByText("尚需 2 項必填")).toBeTruthy();
		await typeInto("聯絡人姓名", "王小明");
		expect(screen.getByText("尚需 2 項必填")).toBeTruthy();
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(1);
		expect(screen.getByText("尚需 1 項必填")).toBeTruthy();
	});

	it("自動儲存：停手一秒後只送改過的欄位，帶店家的 etag；之後的寫入用回應的新 etag", async () => {
		kycStore.set(app({ missingFields: [] }));
		await mount();
		await typeInto("聯絡人姓名", "王");
		await act(async () => {
			await vi.advanceTimersByTimeAsync(500);
		});
		await typeInto("聯絡人姓名", "王小明");
		await act(async () => {
			await vi.advanceTimersByTimeAsync(500);
		});
		expect(mocks.updateMyStore).not.toHaveBeenCalled();
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(1);
		expect(mocks.updateMyStore).toHaveBeenLastCalledWith({
			etag: "s1",
			contactName: "王小明",
		});
		await typeInto("聯絡人手機", "0912345678");
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(2);
		expect(mocks.updateMyStore).toHaveBeenLastCalledWith({
			etag: "s2",
			contactMobile: "0912345678",
		});
		expect(mocks.saveKycDraft).not.toHaveBeenCalled();
	});

	it("重新整理：已存的聯絡資料從店家讀回輸入框，開頁不寫入", async () => {
		mocks.store = {
			etag: "s1",
			contactName: "王小明",
			contactMobile: "0912345678",
			supportEmail: "shop@example.com",
		};
		kycStore.set(app({ missingFields: [] }));
		await mount();
		expect(screen.getByLabelText("聯絡人姓名")).toHaveProperty(
			"value",
			"王小明",
		);
		expect(screen.getByLabelText("客服電子信箱")).toHaveProperty(
			"value",
			"shop@example.com",
		);
		await act(async () => {
			await vi.advanceTimersByTimeAsync(10_000);
		});
		expect(mocks.updateMyStore).not.toHaveBeenCalled();
	});

	it("版本衝突：保留輸入、讀最新的店家，下一次寫入用新的 etag", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.updateMyStore.mockRejectedValueOnce(conflict());
		mocks.fetchMyStore.mockResolvedValue({ store: { etag: "s7" } });
		await mount();
		await typeInto("聯絡人姓名", "王小明");
		await autosave();
		expect(mocks.fetchMyStore).toHaveBeenCalledTimes(1);
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(2);
		expect(mocks.updateMyStore).toHaveBeenLastCalledWith({
			etag: "s7",
			contactName: "王小明",
		});
		expect(screen.getByLabelText("聯絡人姓名")).toHaveProperty(
			"value",
			"王小明",
		);
	});

	it("版本衝突：使用者沒改過、別處已改的欄位取伺服器的值，不被寫回（只寫使用者自己的修改）", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.store = {
			etag: "s1",
			supportEmail: "old@example.com",
			address: { townCode: "63000050", addressDetail: "舊路一號" },
		};
		const latest = {
			etag: "s7",
			contactMobile: "0911222333",
			supportEmail: "new@example.com",
			address: { townCode: "64000120", addressDetail: "新路二號" },
		};
		mocks.updateMyStore.mockRejectedValueOnce(conflict());
		mocks.updateMyStore.mockImplementation(
			async (update: Record<string, unknown>) => ({
				etag: "s8",
				store: create(StoreSchema, {
					...latest,
					...update,
					etag: "s8",
				} as Parameters<typeof create<typeof StoreSchema>>[1]),
			}),
		);
		mocks.fetchMyStore.mockResolvedValue({ store: latest });
		await mount();
		await typeInto("聯絡人姓名", "王小明");
		await autosave();
		expect(mocks.fetchMyStore).toHaveBeenCalledTimes(1);
		expect(screen.getByLabelText("客服電子信箱")).toHaveProperty(
			"value",
			"new@example.com",
		);
		expect(screen.getByLabelText("聯絡人手機")).toHaveProperty(
			"value",
			"0911222333",
		);
		await autosave();
		await act(async () => {
			await vi.advanceTimersByTimeAsync(10_000);
		});
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(2);
		expect(mocks.updateMyStore).toHaveBeenLastCalledWith({
			etag: "s7",
			contactName: "王小明",
		});
	});

	it("版本衝突：使用者自己改過的欄位保留，即使別處也改了同一欄", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.store = { etag: "s1", supportEmail: "old@example.com" };
		mocks.updateMyStore.mockRejectedValueOnce(conflict());
		mocks.fetchMyStore.mockResolvedValue({
			store: { etag: "s7", supportEmail: "elsewhere@example.com" },
		});
		await mount();
		await typeInto("客服電子信箱", "mine@example.com");
		await autosave();
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenLastCalledWith({
			etag: "s7",
			supportEmail: "mine@example.com",
		});
	});

	it("儲存狀態把店家資料算進去：排隊或寫入中是「儲存中」，寫完是「已儲存」，非欄位錯誤的失敗是「店家資料尚未儲存」", async () => {
		kycStore.set(app({ missingFields: [] }));
		await mount();
		expect(screen.queryByText("儲存中…")).toBeNull();
		await typeInto("聯絡人姓名", "王小明");
		expect(screen.getByText("儲存中…")).toBeTruthy();
		await autosave();
		expect(screen.queryByText("儲存中…")).toBeNull();
		expect(screen.getByText("剛剛已儲存")).toBeTruthy();
		mocks.updateMyStore.mockRejectedValue(new Error("boom"));
		await typeInto("聯絡人手機", "0912345678");
		await autosave();
		expect(screen.getByText("店家資料尚未儲存")).toBeTruthy();
		expect(screen.queryByText("剛剛已儲存")).toBeNull();
	});

	it("地址不完整（有明細卻沒有鄉鎮市區）：不顯示已儲存、顯示地址錯誤、不能送出", async () => {
		kycStore.set(app({ missingFields: [] }));
		await mount();
		await typeInto("店家地址", "新路二號");
		await autosave();
		expect(screen.queryByText("剛剛已儲存")).toBeNull();
		expect(screen.queryByText("儲存中…")).toBeNull();
		expect(screen.getByText("店家資料尚未儲存")).toBeTruthy();
		expect(screen.getAllByText("請選擇縣市與鄉鎮市區").length).toBeGreaterThan(
			0,
		);
		expect(
			(screen.getByText("送出認證").closest("button") as HTMLButtonElement)
				.disabled,
		).toBe(true);
		expect(mocks.submitKyc).not.toHaveBeenCalled();
		expect(mocks.updateMyStore).not.toHaveBeenCalled();
	});

	it("伺服器拒絕的欄位標在那個欄位上，不重試", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.store = { etag: "s1", contactName: "王小明" };
		mocks.updateMyStore.mockRejectedValue(
			new ConnectError("必填", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "store.contact_name",
								description: "必填",
								localizedMessage: { locale: "zh-TW", message: "必填" },
							},
						],
					},
				},
			]),
		);
		await mount();
		await typeInto("聯絡人姓名", "");
		await autosave();
		await act(async () => {
			await vi.advanceTimersByTimeAsync(60_000);
		});
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(1);
		expect(
			document.querySelector('[data-field-error="聯絡人姓名"]')?.textContent,
		).toContain("必填");
	});

	it("送出認證時先寫完店家資料，再送出", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await typeInto("聯絡人姓名", "王小明");
		await typeInto("聯絡人手機", "0912345678");
		await typeInto("客服電子信箱", "shop@example.com");
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.updateMyStore).toHaveBeenCalledTimes(1);
		expect(mocks.updateMyStore).toHaveBeenCalledWith({
			etag: "s1",
			contactName: "王小明",
			contactMobile: "0912345678",
			supportEmail: "shop@example.com",
		});
		expect(mocks.submitKyc).toHaveBeenCalledTimes(1);
	});

	it("店家寫入失敗時不送出認證", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.updateMyStore.mockRejectedValue(new Error("boom"));
		await mount();
		await typeInto("聯絡人姓名", "王小明");
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.submitKyc).not.toHaveBeenCalled();
	});

	it("手機格式不合：不寫店家、不送出", async () => {
		kycStore.set(app({ missingFields: [] }));
		await mount();
		await typeInto("聯絡人手機", "0912");
		await autosave();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.updateMyStore).not.toHaveBeenCalled();
		expect(mocks.submitKyc).not.toHaveBeenCalled();
	});

	it("SubmitKYC 回 failed_precondition：重讀申請，列出伺服器的店家缺項", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockRejectedValue(
			new ConnectError("店家資料不完整", Code.FailedPrecondition),
		);
		mocks.fetchMyKyc.mockResolvedValue(
			app({
				missingFields: [],
				missingStoreFields: [StoreField.SUPPORT_EMAIL],
			}),
		);
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.fetchMyKyc).toHaveBeenCalled();
		expect(
			screen.getAllByText("下列欄位尚未填寫：客服電子信箱").length,
		).toBeGreaterThan(0);
	});

	it("SubmitKYC 回 failed_precondition 而伺服器沒有店家缺項（身分比對）：顯示伺服器的訊息", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockRejectedValue(
			new ConnectError(
				"無法通過認證，請聯絡客服",
				Code.FailedPrecondition,
				undefined,
				[
					{
						desc: LocalizedMessageSchema,
						value: { locale: "zh-TW", message: "無法通過認證，請聯絡客服" },
					},
				],
			),
		);
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(screen.getAllByText(/無法通過認證/).length).toBeGreaterThan(0);
	});
});

describe("店家地址：以鄉鎮市區代碼寫到店家", () => {
	it("地址相對於店家有改才自動寫入", async () => {
		mocks.store = {
			etag: "s1",
			address: { townCode: "63000010", addressDetail: "某路 1 號" },
		};
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await typeInto("店家地址", "某路 2 號");
		await autosave();
		expect(mocks.updateMyStore).toHaveBeenCalledWith({
			etag: "s1",
			address: { townCode: "63000010", addressDetail: "某路 2 號" },
		});
	});

	it("店家沒有地址時不更新店家", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.updateMyStore).not.toHaveBeenCalled();
	});
});

describe("入駐同意：送出 KYC 只要確認已閱讀告知事項", () => {
	const submitButton = () =>
		screen.getByText("送出認證").closest("button") as HTMLButtonElement;

	it("沒勾不能送出，勾了才能；表單上沒有 PayUni 條款的勾選", async () => {
		kycStore.set(app({ etag: "e1", missingFields: [] }));
		await mount();
		expect(submitButton().disabled).toBe(true);
		expect(screen.queryByLabelText(/PAYUNi/)).toBeNull();

		fireEvent.click(noticeBox());
		expect(submitButton().disabled).toBe(false);

		fireEvent.click(noticeBox());
		expect(submitButton().disabled).toBe(true);
		expect(mocks.submitKyc).not.toHaveBeenCalled();
	});

	it("連結來自生成的常數：告知事項在新分頁", async () => {
		kycStore.set(app({ etag: "e1", missingFields: [] }));
		await mount();
		const notice = screen.getByRole("link", {
			name: "賣家身分認證個人資料蒐集告知事項",
		});
		expect(notice.getAttribute("href")).toBe(SERVED.notice.url);
		expect(notice.getAttribute("target")).toBe("_blank");
		expect(notice.getAttribute("rel")).toContain("noopener");
	});

	it("送出的版本是賣家看到的那一版（生成的常數）", async () => {
		kycStore.set(app({ etag: "e1", missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(
			app({
				etag: "e3",
				status: KYCStatus.KYC_STATUS_SUBMITTED,
			}),
		);
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
		expect(mocks.submitKyc).toHaveBeenCalledWith("e1", ACCEPTED_KYC);
	});

	it("AGREEMENT_OUTDATED：取消勾選並提示重新整理，不當成缺店家欄位", async () => {
		kycStore.set(app({ etag: "e1", missingFields: [] }));
		mocks.submitKyc.mockRejectedValueOnce(outdated());
		await mount();
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});

		expect(screen.getByRole("alert").textContent).toBe(
			"條款已更新，請重新整理頁面後再勾選。",
		);
		expect(noticeBox().checked).toBe(false);
		expect(submitButton().disabled).toBe(true);
		expect(mocks.fetchMyKyc).not.toHaveBeenCalled();
		expect(mocks.replace).not.toHaveBeenCalled();
		expect(mocks.submitKyc).toHaveBeenCalledTimes(1);
	});
});

describe("金流商聯絡信箱（D23 修訂）", () => {
	const CONTACT_LABEL = "金流商聯絡信箱";
	const submittedApp = () =>
		app({ etag: "e3", status: KYCStatus.KYC_STATUS_SUBMITTED });

	async function clickSubmit() {
		await acceptAgreements();
		await act(async () => {
			fireEvent.click(screen.getByText("送出認證"));
		});
	}

	it("已驗證的信箱：唯讀文字，沒有輸入框，送出不呼叫 SetContactEmail", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(submittedApp());
		await mount();
		expect(screen.queryByLabelText(CONTACT_LABEL)).toBeNull();
		expect(screen.getByText("login@example.com")).toBeTruthy();
		expect(screen.getByText("金流商會用這個信箱聯絡你收款事宜")).toBeTruthy();
		await clickSubmit();
		expect(mocks.setContactEmail).not.toHaveBeenCalled();
		expect(mocks.submitKyc).toHaveBeenCalledTimes(1);
	});

	it("自填過的信箱：帶入可編輯的輸入框", async () => {
		mocks.contact = { email: "mine@example.com", source: "self-entered" };
		kycStore.set(app({ missingFields: [] }));
		await mount();
		const input = screen.getByLabelText(CONTACT_LABEL) as HTMLInputElement;
		expect(input.value).toBe("mine@example.com");
		expect(input.readOnly).toBe(false);
	});

	it("沒有信箱：輸入框必填，沒填或格式不對不能送出；填了先 SetContactEmail 再 SubmitKYC", async () => {
		mocks.contact = null;
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockResolvedValue(submittedApp());
		const calls: string[] = [];
		mocks.setContactEmail.mockImplementation(async () => {
			calls.push("set");
			return { email: "pay@example.com", source: "self-entered" };
		});
		mocks.submitKyc.mockImplementation(async () => {
			calls.push("submit");
			return submittedApp();
		});
		await mount();
		const submit = screen.getByText("送出認證") as HTMLButtonElement;
		expect(
			(screen.getByLabelText(CONTACT_LABEL) as HTMLInputElement).value,
		).toBe("");
		await acceptAgreements();
		expect(submit.disabled).toBe(true);
		await typeInto(CONTACT_LABEL, "not-an-email");
		expect(submit.disabled).toBe(true);
		expect(document.querySelector("[data-field-error]")).toBeTruthy();
		await typeInto(CONTACT_LABEL, " Pay@Example.com ");
		expect(submit.disabled).toBe(false);
		await act(async () => {
			fireEvent.click(submit);
		});
		expect(mocks.setContactEmail).toHaveBeenCalledWith("pay@example.com");
		expect(calls).toEqual(["set", "submit"]);
	});

	it("SetContactEmail 失敗：顯示伺服器對欄位的訊息，不送出申請", async () => {
		mocks.contact = null;
		kycStore.set(app({ missingFields: [] }));
		mocks.setContactEmail.mockRejectedValue(
			invalidField("email", "信箱格式不正確"),
		);
		await mount();
		await typeInto(CONTACT_LABEL, "pay@example.com");
		await clickSubmit();
		expect(screen.getByRole("alert").textContent).toContain("信箱格式不正確");
		expect(mocks.submitKyc).not.toHaveBeenCalled();
	});

	it("SubmitKYC 回 CONTACT_EMAIL_REQUIRED：顯示伺服器的文字並重讀聯絡信箱", async () => {
		kycStore.set(app({ missingFields: [] }));
		mocks.submitKyc.mockRejectedValue(
			new ConnectError("need", Code.FailedPrecondition, undefined, [
				{
					desc: ErrorInfoSchema,
					value: {
						reason: ErrorReason[ErrorReason.CONTACT_EMAIL_REQUIRED],
						domain: "hawkr",
					},
				},
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "請先填寫聯絡信箱" },
				},
			]),
		);
		await mount();
		await clickSubmit();
		expect(screen.getByRole("alert").textContent).toContain("請先填寫聯絡信箱");
		expect(mocks.refetchContact).toHaveBeenCalled();
	});
});
