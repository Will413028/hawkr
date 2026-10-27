// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { DurationSchema } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	BadRequestSchema,
	ErrorInfoSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AppealUnavailableReason,
	type Measure,
	MeasureBasis,
	MeasureKind,
	MeasureRequestKind,
	MeasureRequestSchema,
	MeasureRequestState,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
	RiskGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	APPEAL_SENTENCE,
	REMEDIATION_SENTENCE,
	unavailable,
} from "@/tests/support/server-unavailability";

const mocks = vi.hoisted(() => ({
	query: {} as {
		measure: unknown;
		loading: boolean;
		error: string | null;
		notFound: boolean;
		refetch: () => Promise<unknown>;
	},
	submitAppeal: vi.fn(),
	submitRemediation: vi.fn(),
	adopt: vi.fn(),
	fetchMyMeasure: vi.fn(),
	replace: vi.fn(),
	toastError: vi.fn(),
	toastSuccess: vi.fn(),
	uploadSecureDocument: vi.fn(),
}));

vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useSellerMeasure: () => mocks.query,
	useSubmitSellerAppeal: () => mocks.submitAppeal,
	useSubmitSellerRemediation: () => mocks.submitRemediation,
	useAdoptMeasure: () => mocks.adopt,
}));
vi.mock("@/lib/seller/measures", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/seller/measures")>()),
	fetchMyMeasure: mocks.fetchMyMeasure,
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ replace: mocks.replace }),
}));
vi.mock("@/lib/feedback/toast", () => ({
	toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));
vi.mock("@/lib/media/secure-upload", () => ({
	uploadSecureDocument: mocks.uploadSecureDocument,
}));
vi.mock("@/lib/media/upload", () => ({ uploadImageFile: vi.fn() }));
vi.mock("@/components/ui/ImageUploader", () => ({
	ImageUploader: ({
		onPick,
		onClear,
		value,
		error,
		"aria-label": label,
	}: {
		onPick: (file: File) => void;
		onClear?: () => void;
		value: string | null;
		error?: string;
		"aria-label"?: string;
	}) =>
		value
			? React.createElement(
					"div",
					{ "aria-label": label },
					error ? React.createElement("small", null, error) : null,
					React.createElement(
						"button",
						{ type: "button", onClick: onClear },
						`移除${label}`,
					),
				)
			: React.createElement("input", {
					type: "file",
					"aria-label": label,
					onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
						const file = e.target.files?.[0];
						if (file) onPick(file);
					},
				}),
}));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({
		title,
		children,
	}: {
		title?: React.ReactNode;
		children?: React.ReactNode;
	}) =>
		React.createElement(
			"div",
			null,
			React.createElement("h1", null, title),
			children,
		),
}));
vi.mock("@/features/seller-center/components/SellerSkeletons", () => ({
	SellerCardGridSkeleton: () =>
		React.createElement("div", { "data-testid": "skeleton" }),
}));
vi.mock("@/components/ui/Card", () => ({
	Card: ({ children }: { children?: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
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
import SellerMeasureRequestPage from "./SellerMeasureRequestPage";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

const measure = (
	init: Parameters<typeof create<typeof MeasureSchema>>[1] = {},
) =>
	create(MeasureSchema, {
		id: "m1",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.RISK,
		riskGround: RiskGround.SECURITY,
		statement: "帳號異常登入",
		state: MeasureState.ACTIVE,
		canAppeal: true,
		canRequestRemediation: true,
		etag: "etag-1",
		...init,
	}) as Measure;

const MEASURE = measure();
const MEASURES_PATH = "/seller/measures";
const DETAIL_PATH = "/seller/measures/m1";

const pendingRequest = (kind: MeasureRequestKind) =>
	create(MeasureRequestSchema, { kind, state: MeasureRequestState.PENDING });

/** U+1F600，UTF-16 兩個 code unit、一個 rune。 */
const EMOJI = String.fromCodePoint(0x1f600);

const versionConflict = () =>
	new ConnectError("stale", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: {
				reason: ErrorReason[ErrorReason.VERSION_CONFLICT],
				domain: "hawkr",
			},
		},
	]);

const invalidArgument = (fields: Record<string, string>) =>
	new ConnectError("invalid", Code.InvalidArgument, undefined, [
		{
			desc: BadRequestSchema,
			value: {
				fieldViolations: Object.entries(fields).map(([field, message]) => ({
					field,
					description: "invalid",
					localizedMessage: { locale: "zh-TW", message },
				})),
			},
		},
	]);

const failedPrecondition = () =>
	new ConnectError("done", Code.FailedPrecondition);

type Kind = "appeal" | "remediation";

function renderPage(kind: Kind) {
	return render(
		React.createElement(SellerMeasureRequestPage, { kind, measureId: "m1" }),
	);
}

const reasonBox = () => screen.getByRole("textbox") as HTMLTextAreaElement;
const type = (value: string) =>
	fireEvent.change(reasonBox(), { target: { value } });

async function addPhoto(name = "e.jpg") {
	const before = screen.queryAllByLabelText(/^已上傳照片/).length;
	fireEvent.change(screen.getByLabelText("新增照片"), {
		target: {
			files: [new File(["x"], name, { type: "image/jpeg" })],
		},
	});
	await waitFor(() =>
		expect(screen.queryAllByLabelText(/^已上傳照片/)).toHaveLength(before + 1),
	);
}

const setQuery = (over: Partial<typeof mocks.query> = {}) => {
	mocks.query = {
		measure: MEASURE,
		loading: false,
		error: null,
		notFound: false,
		refetch: vi.fn(async () => {}),
		...over,
	};
};

beforeEach(() => {
	setQuery();
	for (const m of [
		mocks.submitAppeal,
		mocks.submitRemediation,
		mocks.adopt,
		mocks.fetchMyMeasure,
		mocks.replace,
		mocks.toastError,
		mocks.toastSuccess,
		mocks.uploadSecureDocument,
	]) {
		m.mockReset();
	}
	let n = 0;
	mocks.uploadSecureDocument.mockImplementation(async () => ({
		stagedToken: `tok-${++n}`,
		referenceId: "s",
	}));
	URL.createObjectURL = () => "blob:preview";
});

afterEach(cleanup);

/** 兩種請求共同的行為；`submitLabel`、`reasonLabel` 與提示字不同，其餘規則相同。 */
const KINDS = [
	{
		kind: "appeal",
		submitLabel: "送出申覆",
		tooLong: "申覆理由最多 2000 字",
		submit: () => mocks.submitAppeal,
		other: () => mocks.submitRemediation,
		successToast: "申覆已送出，請等候審核",
		canSubmitOff: { canAppeal: false },
	},
	{
		kind: "remediation",
		submitLabel: "送出請求",
		tooLong: "改善說明最多 2000 字",
		submit: () => mocks.submitRemediation,
		other: () => mocks.submitAppeal,
		successToast: "解除請求已送出，請等候審核",
		canSubmitOff: { canRequestRemediation: false },
	},
] as const;

describe.each(KINDS)("SellerMeasureRequestPage（$kind）的讀取狀態", ({
	kind,
}) => {
	it("讀取中顯示骨架，不畫表單", () => {
		setQuery({ measure: null, loading: true });
		renderPage(kind);
		expect(screen.getByTestId("skeleton")).toBeTruthy();
		expect(screen.queryByRole("textbox")).toBeNull();
	});

	it("找不到（not_found、permission_denied）：回到措施與申覆", () => {
		setQuery({ measure: null, notFound: true, error: "資源不存在" });
		renderPage(kind);
		expect(screen.getByText("找不到這項措施")).toBeTruthy();
		expect(
			screen.getByRole("link", { name: "回到措施與申覆" }).getAttribute("href"),
		).toBe(MEASURES_PATH);
	});

	it("暫時性的錯誤可以重試", () => {
		const refetch = vi.fn();
		setQuery({ measure: null, error: "暫時無法使用", refetch });
		renderPage(kind);
		expect(screen.getByText("無法載入措施紀錄")).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: "重試" }));
		expect(refetch).toHaveBeenCalledTimes(1);
	});

	it("表單顯示之後，背景重讀到不能提出也不卸載表單（輸入不會消失）", () => {
		const { rerender } = renderPage(kind);
		type("寫到一半的理由");
		setQuery({
			measure: measure({
				canAppeal: false,
				canRequestRemediation: false,
				etag: "etag-2",
			}),
		});
		rerender(
			React.createElement(SellerMeasureRequestPage, {
				kind,
				measureId: "m1",
			}),
		);
		expect(reasonBox().value).toBe("寫到一半的理由");
	});
});

describe("SellerMeasureRequestPage 打開時已不能提出", () => {
	it.each([
		[
			"已申覆",
			measure({
				canAppeal: false,
				appealUnavailabilities: [
					unavailable(
						AppealUnavailableReason.ALREADY_APPEALED,
						APPEAL_SENTENCE.ALREADY_APPEALED,
					),
				],
			}),
			APPEAL_SENTENCE.ALREADY_APPEALED,
		],
		[
			"已撤銷",
			measure({
				canAppeal: false,
				state: MeasureState.REVOKED,
				appealUnavailabilities: [
					unavailable(AppealUnavailableReason.REVOKED, APPEAL_SENTENCE.REVOKED),
				],
			}),
			APPEAL_SENTENCE.REVOKED,
		],
		[
			"申覆期限已過（第一筆原因，後面的不影響）",
			measure({
				canAppeal: false,
				appealUnavailabilities: [
					unavailable(
						AppealUnavailableReason.APPEAL_WINDOW_ENDED,
						APPEAL_SENTENCE.APPEAL_WINDOW_ENDED,
					),
					unavailable(
						AppealUnavailableReason.REQUEST_PENDING,
						APPEAL_SENTENCE.REQUEST_PENDING,
					),
				],
			}),
			APPEAL_SENTENCE.APPEAL_WINDOW_ENDED,
		],
		[
			"有審核中的請求",
			measure({
				canAppeal: false,
				appealUnavailabilities: [
					unavailable(
						AppealUnavailableReason.REQUEST_PENDING,
						APPEAL_SENTENCE.REQUEST_PENDING,
					),
				],
			}),
			APPEAL_SENTENCE.REQUEST_PENDING,
		],
		[
			"原因沒有句子（用通用說明，不依 reason 自己組字）",
			measure({
				canAppeal: false,
				appealUnavailabilities: [unavailable(AppealUnavailableReason.REVOKED)],
			}),
			"這項措施目前不能提出申覆",
		],
	])("申覆（%s）：顯示原因與回措施列表，不畫表單", (_name, data, title) => {
		setQuery({ measure: data });
		renderPage("appeal");
		expect(screen.getByText(title)).toBeTruthy();
		expect(screen.getByText("請回到措施與申覆查看目前狀態。")).toBeTruthy();
		expect(screen.queryByRole("textbox")).toBeNull();
	});

	it("解除請求：顯示伺服器原因清單的第一筆句子，不畫表單；canAppeal 為真也不影響", () => {
		setQuery({
			measure: measure({
				canRequestRemediation: false,
				canAppeal: true,
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.REMEDIATION_PENDING,
						REMEDIATION_SENTENCE.REMEDIATION_PENDING,
					),
					unavailable(
						RemediationUnavailableReason.COOLDOWN,
						REMEDIATION_SENTENCE.COOLDOWN,
					),
				],
			}),
		});
		renderPage("remediation");
		expect(
			screen.getByText(REMEDIATION_SENTENCE.REMEDIATION_PENDING),
		).toBeTruthy();
		expect(screen.queryByRole("textbox")).toBeNull();
	});

	it("解除請求：沒有入口的原因（NOT_RISK）也顯示伺服器的句子", () => {
		setQuery({
			measure: measure({
				canRequestRemediation: false,
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.NOT_RISK,
						REMEDIATION_SENTENCE.NOT_RISK,
					),
				],
			}),
		});
		renderPage("remediation");
		expect(screen.getByText(REMEDIATION_SENTENCE.NOT_RISK)).toBeTruthy();
	});

	it("解除請求：伺服器沒有句子時用通用說明", () => {
		setQuery({
			measure: measure({
				canRequestRemediation: false,
				remediationUnavailabilities: [
					unavailable(RemediationUnavailableReason.NOT_RISK),
				],
			}),
		});
		renderPage("remediation");
		expect(screen.getByText("這項措施目前不能請求解除")).toBeTruthy();
	});
});

describe.each(KINDS)("SellerMeasureRequestPage（$kind）的字數", ({
	kind,
	submitLabel,
	tooLong,
	submit,
}) => {
	it("以 rune 計：2000 個 emoji 剛好可送出，2001 個被擋下；沒有原生 maxlength", () => {
		renderPage(kind);
		expect(reasonBox().getAttribute("maxlength")).toBeNull();
		type(EMOJI.repeat(2000));
		expect(screen.getByText("2000 / 2000")).toBeTruthy();
		expect((screen.getByText(submitLabel) as HTMLButtonElement).disabled).toBe(
			false,
		);
		type(EMOJI.repeat(2001));
		expect(screen.getByText("2001 / 2000")).toBeTruthy();
		expect(screen.getByText(tooLong)).toBeTruthy();
		expect((screen.getByText(submitLabel) as HTMLButtonElement).disabled).toBe(
			true,
		);
	});

	it("字數與送出的理由都先照 Go 的規則修剪；只有空白不能送出", async () => {
		submit().mockResolvedValue(MEASURE);
		renderPage(kind);
		type("\u3000\u00a0 ");
		expect(screen.getByText("0 / 2000")).toBeTruthy();
		expect((screen.getByText(submitLabel) as HTMLButtonElement).disabled).toBe(
			true,
		);
		type("\u3000已補上標示\u00a0\n");
		expect(screen.getByText("5 / 2000")).toBeTruthy();
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(submit()).toHaveBeenCalled());
		expect(submit()).toHaveBeenCalledWith(MEASURE, {
			reason: "已補上標示",
			evidenceStagedTokens: [],
		});
	});
});

describe.each(KINDS)("SellerMeasureRequestPage（$kind）送出", ({
	kind,
	submitLabel,
	submit,
	other,
	successToast,
	canSubmitOff,
}) => {
	it("佐證走機敏上傳，送出的是機敏 staged token 與措施的 etag，只呼叫自己的 RPC；成功後提示並回這一筆措施頁", async () => {
		submit().mockResolvedValue(MEASURE);
		renderPage(kind);
		type("已補上標示");
		await addPhoto();
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(mocks.replace).toHaveBeenCalledWith(DETAIL_PATH),
		);
		expect(mocks.uploadSecureDocument).toHaveBeenCalledTimes(1);
		expect(submit()).toHaveBeenCalledWith(MEASURE, {
			reason: "已補上標示",
			evidenceStagedTokens: ["tok-1"],
		});
		expect(other()).not.toHaveBeenCalled();
		expect(mocks.toastSuccess).toHaveBeenCalledWith(successToast);
	});

	it("返回：回到這一筆措施頁，不是列表", () => {
		renderPage(kind);
		fireEvent.click(screen.getByText("返回"));
		expect(mocks.replace).toHaveBeenCalledWith(DETAIL_PATH);
		expect(mocks.replace).not.toHaveBeenCalledWith(MEASURES_PATH);
	});

	it("失敗時不提示成功、不離開", async () => {
		submit().mockRejectedValueOnce(new Error("boom"));
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(mocks.toastSuccess).not.toHaveBeenCalled();
		expect(mocks.replace).not.toHaveBeenCalled();
	});

	it("VERSION_CONFLICT 重讀後仍可提出：保留輸入、換新的 etag、顯示更新提示", async () => {
		const fresh = measure({ etag: "etag-2", statement: "更新後的說明" });
		submit().mockRejectedValueOnce(versionConflict());
		mocks.fetchMyMeasure.mockResolvedValue(fresh);
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain("這項措施已更新"),
		);
		expect(mocks.fetchMyMeasure).toHaveBeenCalledWith("m1");
		expect(mocks.adopt).toHaveBeenCalledWith(fresh);
		expect(reasonBox().value).toBe("已補上標示");
		expect(mocks.replace).not.toHaveBeenCalled();
		expect(mocks.toastError).not.toHaveBeenCalled();

		submit().mockResolvedValueOnce(fresh);
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(submit()).toHaveBeenCalledTimes(2));
		expect(submit().mock.calls[1][0]).toBe(fresh);
	});

	it("failed_precondition 重讀後伺服器說仍可提出：保留輸入，重試送出同一筆內容", async () => {
		const fresh = measure({ etag: "etag-2" });
		submit().mockRejectedValueOnce(failedPrecondition());
		submit().mockResolvedValueOnce(fresh);
		mocks.fetchMyMeasure.mockResolvedValue(fresh);
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(reasonBox().value).toBe("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(submit()).toHaveBeenCalledTimes(2));
		expect(submit().mock.calls[1][1]).toEqual(submit().mock.calls[0][1]);
	});

	it("failed_precondition 重讀後伺服器說不能提出：以 toast 說明並回這一筆措施頁，不留錯誤文字", async () => {
		const fresh = measure({
			...canSubmitOff,
			state: MeasureState.REVOKED,
			remediationUnavailabilities: [
				unavailable(
					RemediationUnavailableReason.REMEDIATION_PENDING,
					REMEDIATION_SENTENCE.REMEDIATION_PENDING,
				),
			],
			requests: [pendingRequest(MeasureRequestKind.APPEAL)],
		});
		submit().mockRejectedValueOnce(failedPrecondition());
		mocks.fetchMyMeasure.mockResolvedValue(fresh);
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(mocks.replace).toHaveBeenCalledWith(DETAIL_PATH),
		);
		expect(mocks.toastError).toHaveBeenCalledTimes(1);
		expect(mocks.adopt).toHaveBeenCalledWith(fresh);
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("重讀失敗：顯示無法載入最新版本，不離開", async () => {
		submit().mockRejectedValueOnce(versionConflict());
		mocks.fetchMyMeasure.mockRejectedValue(new Error("network"));
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toContain(
				"無法載入最新版本",
			),
		);
		expect(mocks.replace).not.toHaveBeenCalled();
		expect(mocks.adopt).not.toHaveBeenCalled();
		expect(mocks.toastError).not.toHaveBeenCalled();
		expect(reasonBox().value).toBe("已補上標示");
	});

	it("理由欄的伺服器錯誤標在理由欄", async () => {
		submit().mockRejectedValueOnce(invalidArgument({ reason: "理由不可空白" }));
		renderPage(kind);
		type("x");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(screen.getByText("理由不可空白")).toBeTruthy());
		expect(reasonBox().getAttribute("aria-invalid")).toBe("true");
	});

	it("evidence_staged_tokens[i] 標在送出的第 i 張（以 token 對應），有失敗的照片時不能再送出", async () => {
		submit().mockRejectedValueOnce(
			invalidArgument({
				"evidence_staged_tokens[1]": "這張已過期",
				reason: "理由有問題",
			}),
		);
		renderPage(kind);
		type("已補上標示");
		await addPhoto("a.jpg");
		await addPhoto("b.jpg");
		await addPhoto("c.jpg");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(screen.getByText("這張已過期")).toBeTruthy());
		expect(submit().mock.calls[0][1].evidenceStagedTokens).toEqual([
			"tok-1",
			"tok-2",
			"tok-3",
		]);
		const photos = screen.getAllByLabelText(/^已上傳照片/);
		expect(photos[0].textContent).not.toContain("這張已過期");
		expect(photos[1].textContent).toContain("這張已過期");
		expect(photos[2].textContent).not.toContain("這張已過期");
		expect((screen.getByText(submitLabel) as HTMLButtonElement).disabled).toBe(
			true,
		);
		expect(screen.getByText("理由有問題")).toBeTruthy();
	});

	it("照片的錯誤跟著該張照片：前面的照片被移除後錯誤留在同一張，那張被移除後錯誤消失", async () => {
		submit().mockRejectedValueOnce(
			invalidArgument({ "evidence_staged_tokens[1]": "這張已過期" }),
		);
		renderPage(kind);
		type("已補上標示");
		await addPhoto("a.jpg");
		await addPhoto("b.jpg");
		await addPhoto("c.jpg");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() => expect(screen.getByText("這張已過期")).toBeTruthy());

		fireEvent.click(screen.getByText("移除已上傳照片 1"));
		const afterShift = screen.getAllByLabelText(/^已上傳照片/);
		expect(afterShift).toHaveLength(2);
		expect(afterShift[0].textContent).toContain("這張已過期");
		expect(afterShift[1].textContent).not.toContain("這張已過期");

		fireEvent.click(screen.getByText("移除已上傳照片 1"));
		expect(screen.queryByText("這張已過期")).toBeNull();
		expect(screen.getAllByLabelText(/^已上傳照片/)).toHaveLength(1);
	});

	it("沒有索引的 evidence_staged_tokens 是整個佐證欄位的錯誤，不標任何一張", async () => {
		submit().mockRejectedValueOnce(
			invalidArgument({ evidence_staged_tokens: "佐證圖片最多 5 張" }),
		);
		renderPage(kind);
		type("已補上標示");
		await addPhoto();
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(screen.getByText("佐證圖片最多 5 張")).toBeTruthy(),
		);
		const [photo] = screen.getAllByLabelText(/^已上傳照片/);
		expect(photo.textContent).not.toContain("佐證圖片最多 5 張");
	});

	it("其他失敗顯示伺服器的文字，不重讀", async () => {
		submit().mockRejectedValueOnce(new ConnectError("x", Code.NotFound));
		renderPage(kind);
		type("已補上標示");
		fireEvent.click(screen.getByText(submitLabel));
		await waitFor(() =>
			expect(screen.getByRole("alert").textContent).toBe("資源不存在"),
		);
		expect(mocks.fetchMyMeasure).not.toHaveBeenCalled();
	});
});

describe("SellerMeasureRequestPage 解除請求頁", () => {
	it("標題是請求解除措施；欄位是改善說明", () => {
		renderPage("remediation");
		expect(screen.getByRole("heading", { name: "請求解除措施" })).toBeTruthy();
		expect(screen.getByText("改善說明")).toBeTruthy();
		expect(
			screen.getByPlaceholderText("請說明已完成的改善，平台會盡快複核。"),
		).toBeTruthy();
	});

	it("重讀後 canRequestRemediation 為否：以重讀後的伺服器句子提示（以 canAppeal 判斷會誤判為仍可請求）", async () => {
		mocks.submitRemediation.mockRejectedValueOnce(versionConflict());
		mocks.fetchMyMeasure.mockResolvedValue(
			measure({
				canRequestRemediation: false,
				canAppeal: true,
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.REMEDIATION_PENDING,
						REMEDIATION_SENTENCE.REMEDIATION_PENDING,
					),
				],
			}),
		);
		renderPage("remediation");
		type("已補上標示");
		fireEvent.click(screen.getByText("送出請求"));
		await waitFor(() =>
			expect(mocks.toastError).toHaveBeenCalledWith(
				REMEDIATION_SENTENCE.REMEDIATION_PENDING,
			),
		);
		expect(mocks.replace).toHaveBeenCalledWith(DETAIL_PATH);
	});

	it("重讀後原因沒有句子：用「這項措施目前不能請求解除」", async () => {
		mocks.submitRemediation.mockRejectedValueOnce(failedPrecondition());
		mocks.fetchMyMeasure.mockResolvedValue(
			measure({
				canRequestRemediation: false,
				remediationUnavailabilities: [
					unavailable(RemediationUnavailableReason.NOT_ACTIVE),
				],
			}),
		);
		renderPage("remediation");
		type("已補上標示");
		fireEvent.click(screen.getByText("送出請求"));
		await waitFor(() =>
			expect(mocks.toastError).toHaveBeenCalledWith("這項措施目前不能請求解除"),
		);
	});
});

describe("SellerMeasureRequestPage 的冷卻警告", () => {
	const withCooldown = (days: number) =>
		measure({
			rejectionCooldown: create(DurationSchema, {
				seconds: BigInt(days * 86400),
			}),
		});

	it.each([
		["appeal", 7],
		["remediation", 14],
	] as const)("%s：rejectionCooldown 有設才顯示，N 為整天數", (kind, days) => {
		setQuery({ measure: withCooldown(days) });
		renderPage(kind);
		expect(
			screen.getByText(`若本次經駁回，將於 ${days} 日內不得再提出解除請求`),
		).toBeTruthy();
	});

	it.each([
		"appeal",
		"remediation",
	] as const)("%s：沒有 rejectionCooldown 就不顯示", (kind) => {
		renderPage(kind);
		expect(screen.queryByText(/若本次經駁回/)).toBeNull();
	});
});
