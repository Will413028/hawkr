// @vitest-environment jsdom

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	CommunityReportSchema,
	ListMyReportsResponseSchema,
	ReportOutcome as ModerationOutcome,
	MyReportItemSchema,
	ReportTargetKind,
	ReportTargetSchema,
} from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	GetMyReportResponseSchema,
	ReportEvidenceSchema,
	ReportOutcome,
	ReportSupplementSchema,
	StoreReportSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	moderationReportTargetKind,
	sellerReportOutcome,
	sellerStoreReportOutcomeUnset,
} from "@hawkr/ui-text/keys";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	waitFor,
} from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { messagesFor } from "@/lib/intl/config";

const mocks = vi.hoisted(() => ({
	listMyReports: vi.fn(),
	getMyReport: vi.fn(),
	addReportSupplement: vi.fn(),
}));
vi.mock("./api", () => mocks);
vi.mock("@/lib/auth/use-account-status", () => ({
	useAccountStatus: () => "signedIn",
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "buyer" }),
}));
vi.mock("@/lib/seller/useStoreCardViews", () => ({
	useStoreCardViews: (ids: string[]) => ({
		views: new Map(
			ids.map((id) => [id, { id, name: `店-${id}`, logoUrl: "", href: "" }]),
		),
		settled: true,
	}),
}));
vi.mock("@/components/shared/Link", async () => {
	const react = await import("react");
	return {
		default: ({
			href,
			children,
			...rest
		}: {
			href: string;
			children?: React.ReactNode;
		}) => react.createElement("a", { href, ...rest }, children),
	};
});
vi.mock("@/features/after-sales/components/AfterSalePhotoUploader", () => ({
	default: () => null,
}));

import MyReportsClient from "@/features/account/screens/reports/MyReportsClient";
import StoreReportDetailClient from "@/features/account/screens/reports/StoreReportDetailClient";

const ut = uiTextFor("zh-TW");
const when = timestampFromDate(new Date("2026-11-01T02:03:00Z"));

function wrap(node: React.ReactNode) {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	});
	return React.createElement(
		QueryClientProvider,
		{ client },
		React.createElement(NextIntlClientProvider, {
			locale: "zh-TW",
			messages: messagesFor("zh-TW"),
			children: node,
		}),
	);
}

function storeReport(over: MessageInitShape<typeof StoreReportSchema> = {}) {
	return create(StoreReportSchema, {
		id: "r1",
		storeId: "s1",
		category: PolicyCategory.FRAUD,
		description: "我的檢舉內容",
		createTime: when,
		...over,
	});
}

function detail(
	report: ReturnType<typeof storeReport>,
	supplementCount = 0,
	evidencePositions: number[] = [],
	supplementEvidence: number[] = [],
) {
	return create(GetMyReportResponseSchema, {
		report,
		supplements: Array.from({ length: supplementCount }, (_, i) =>
			create(ReportSupplementSchema, {
				id: `p${i}`,
				description: `補充內容${i}`,
				createTime: when,
				evidencePositions: i === 0 ? supplementEvidence : [],
			}),
		),
		evidence: evidencePositions.map((position) =>
			create(ReportEvidenceSchema, { position }),
		),
	});
}

const renderDetail = () =>
	render(
		wrap(React.createElement(StoreReportDetailClient, { reportId: "r1" })),
	);

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

describe("MyReportsClient", () => {
	it("renders community and store items; only the store item links to detail", async () => {
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, {
				reports: [
					create(MyReportItemSchema, {
						id: "c1",
						category: PolicyCategory.SPAM,
						createTime: when,
						subject: {
							case: "community",
							value: create(CommunityReportSchema, {
								target: create(ReportTargetSchema, {
									kind: ReportTargetKind.POST,
									id: "post1",
								}),
								outcome: ModerationOutcome.PENDING,
							}),
						},
					}),
					create(MyReportItemSchema, {
						id: "r1",
						category: PolicyCategory.FRAUD,
						description: "店家檢舉說明",
						createTime: when,
						subject: { case: "store", value: storeReport() },
					}),
				],
			}),
		);
		render(wrap(React.createElement(MyReportsClient)));
		const kind = ut(
			moderationReportTargetKind(ReportTargetKind.POST) ?? {
				key: "common.unknown",
			},
		);
		await waitFor(() => expect(screen.getByText(kind)).toBeTruthy());
		expect(screen.getByText("店-s1")).toBeTruthy();
		expect(screen.getByText("店家檢舉說明")).toBeTruthy();
		const links = screen.getAllByRole("link");
		expect(links).toHaveLength(1);
		expect(links[0].getAttribute("href")).toBe("/account/reports/r1");
	});

	it("shows the empty state when there are no reports", async () => {
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, {}),
		);
		render(wrap(React.createElement(MyReportsClient)));
		await waitFor(() => expect(screen.getByText("尚無檢舉紀錄")).toBeTruthy());
	});
});

describe("StoreReportDetailClient", () => {
	it("shows the under-review text when the outcome is unset, with the supplement button", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 2));
		renderDetail();
		await waitFor(() => expect(screen.getByText("店-s1")).toBeTruthy());
		expect(ut(sellerStoreReportOutcomeUnset())).toBe("審核中");
		expect(screen.getByText("審核中")).toBeTruthy();
		expect(screen.getByRole("button", { name: "補充說明" })).toBeTruthy();
	});

	it("hides the supplement button once the report has an outcome", async () => {
		mocks.getMyReport.mockResolvedValue(
			detail(
				storeReport({
					outcome: ReportOutcome.ACTION_TAKEN,
					resolutionNote: "已下架商品",
				}),
				1,
			),
		);
		renderDetail();
		await waitFor(() => expect(screen.getByText("已下架商品")).toBeTruthy());
		const text = ut(
			sellerReportOutcome(ReportOutcome.ACTION_TAKEN) ?? {
				key: "common.unknown",
			},
		);
		expect(screen.getByText(text)).toBeTruthy();
		expect(screen.queryByRole("button", { name: "補充說明" })).toBeNull();
	});

	it("hides the supplement button at 10 supplements and shows the count", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 10));
		renderDetail();
		await waitFor(() =>
			expect(screen.getByText("補充說明（10/10）")).toBeTruthy(),
		);
		expect(screen.queryByRole("button", { name: "補充說明" })).toBeNull();
	});

	it("shows the (n/10) count and the evidence counts without thumbnails", async () => {
		mocks.getMyReport.mockResolvedValue(
			detail(storeReport(), 2, [1, 2, 3], [3]),
		);
		renderDetail();
		await waitFor(() =>
			expect(screen.getByText("補充說明（2/10）")).toBeTruthy(),
		);
		expect(screen.getByText("已附 2 張佐證")).toBeTruthy();
		expect(screen.getByText("已附 1 張佐證")).toBeTruthy();
		expect(screen.queryByRole("img")).toBeNull();
	});

	it("renders the not-found presentation for a missing report", async () => {
		mocks.getMyReport.mockRejectedValue(
			new ConnectError("missing", Code.NotFound),
		);
		renderDetail();
		await waitFor(() =>
			expect(screen.getByTestId("store-report-not-found")).toBeTruthy(),
		);
		expect(screen.queryByRole("button", { name: "補充說明" })).toBeNull();
	});

	it("submits a supplement, shows the received text and refreshes the detail", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 2));
		mocks.addReportSupplement.mockResolvedValue(detail(storeReport(), 3));
		renderDetail();
		fireEvent.click(await screen.findByRole("button", { name: "補充說明" }));
		fireEvent.change(screen.getByLabelText(/^說明/), {
			target: { value: "  新的補充  " },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出補充說明" }));
		await waitFor(() =>
			expect(screen.getByText("已收到補充說明")).toBeTruthy(),
		);
		expect(mocks.addReportSupplement).toHaveBeenCalledWith(
			expect.objectContaining({
				reportId: "r1",
				description: "新的補充",
				evidenceStagedTokens: [],
			}),
		);
		expect(screen.getByText("補充說明（3/10）")).toBeTruthy();
	});

	it("does not submit a blank supplement", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 0));
		renderDetail();
		fireEvent.click(await screen.findByRole("button", { name: "補充說明" }));
		fireEvent.change(screen.getByLabelText(/^說明/), {
			target: { value: "   " },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出補充說明" }));
		await waitFor(() => expect(screen.getByText("請填寫說明")).toBeTruthy());
		expect(mocks.addReportSupplement).not.toHaveBeenCalled();
	});

	it("on failed_precondition shows the server error and re-reads the detail", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 0));
		mocks.addReportSupplement.mockRejectedValue(
			new ConnectError("resolved", Code.FailedPrecondition),
		);
		renderDetail();
		fireEvent.click(await screen.findByRole("button", { name: "補充說明" }));
		fireEvent.change(screen.getByLabelText(/^說明/), {
			target: { value: "內容" },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出補充說明" }));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		await waitFor(() => expect(mocks.getMyReport).toHaveBeenCalledTimes(2));
	});
});
