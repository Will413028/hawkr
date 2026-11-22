// @vitest-environment jsdom

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { BadRequestSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
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
import { createTranslator, NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import { messagesFor } from "@/lib/intl/config";

const mocks = vi.hoisted(() => ({
	listMyReports: vi.fn(),
	getMyReport: vi.fn(),
	addReportSupplement: vi.fn(),
	cardCalls: vi.fn(),
	cards: { settled: true, missing: false },
}));
vi.mock("./api", () => mocks);
vi.mock("@/lib/auth/use-account-status", () => ({
	useAccountStatus: () => "signedIn",
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "buyer" }),
}));
vi.mock("@/lib/seller/useStoreCardViews", () => ({
	useStoreCardViews: (ids: string[]) => {
		mocks.cardCalls(ids);
		return {
			views: new Map(
				mocks.cards.missing || !mocks.cards.settled
					? []
					: ids.map((id) => [
							id,
							{ id, name: `店-${id}`, logoUrl: "", href: "" },
						]),
			),
			settled: mocks.cards.settled,
		};
	},
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
	flags: { canSupplement?: boolean; supplementMax?: number } = {},
) {
	return create(GetMyReportResponseSchema, {
		report,
		canSupplement: flags.canSupplement ?? true,
		supplementMax: flags.supplementMax ?? 10,
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
	mocks.cards.settled = true;
	mocks.cards.missing = false;
});

const storeItem = (id: string, storeId: string) =>
	create(MyReportItemSchema, {
		id,
		category: PolicyCategory.FRAUD,
		createTime: when,
		subject: { case: "store", value: storeReport({ id, storeId }) },
	});
const communityItem = (id: string) =>
	create(MyReportItemSchema, {
		id,
		category: PolicyCategory.SPAM,
		createTime: when,
		subject: {
			case: "community",
			value: create(CommunityReportSchema, {
				target: create(ReportTargetSchema, {
					kind: ReportTargetKind.POST,
					id: "post1",
				}),
			}),
		},
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

describe("MyReportsClient fixes", () => {
	it("keeps loaded items and shows the error inline when load more fails", async () => {
		mocks.listMyReports
			.mockResolvedValueOnce(
				create(ListMyReportsResponseSchema, {
					reports: [storeItem("r1", "s1")],
					nextPageToken: "t2",
				}),
			)
			.mockRejectedValueOnce(new ConnectError("boom", Code.Unavailable));
		render(wrap(React.createElement(MyReportsClient)));
		fireEvent.click(await screen.findByRole("button", { name: "載入更多" }));
		await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
		expect(screen.getByText("店-s1")).toBeTruthy();
		expect(screen.getByRole("button", { name: "載入更多" })).toBeTruthy();
	});

	it("looks up all store cards in one call", async () => {
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, {
				reports: [storeItem("r1", "s1"), storeItem("r2", "s2")],
			}),
		);
		render(wrap(React.createElement(MyReportsClient)));
		await screen.findByText("店-s1");
		expect(screen.getByText("店-s2")).toBeTruthy();
		const withIds = mocks.cardCalls.mock.calls.filter((c) => c[0].length > 0);
		expect(withIds.length).toBeGreaterThan(0);
		for (const c of withIds) expect(c[0]).toEqual(["s1", "s2"]);
	});

	it("shows a skeleton until settled and no placeholder once settled without a card", async () => {
		mocks.cards.settled = false;
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, { reports: [storeItem("r1", "s1")] }),
		);
		const { container, rerender } = render(
			wrap(React.createElement(MyReportsClient)),
		);
		await waitFor(() =>
			expect(
				container.querySelector("[data-testid=store-name-skeleton]"),
			).not.toBeNull(),
		);
		mocks.cards.settled = true;
		mocks.cards.missing = true;
		rerender(wrap(React.createElement(MyReportsClient)));
		await waitFor(() =>
			expect(
				container.querySelector("[data-testid=store-name-skeleton]"),
			).toBeNull(),
		);
	});

	it("keeps the load-more button when every loaded item is of an unknown case", async () => {
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, {
				reports: [create(MyReportItemSchema, { id: "x1" })],
				nextPageToken: "t2",
			}),
		);
		render(wrap(React.createElement(MyReportsClient)));
		await screen.findByRole("button", { name: "載入更多" });
		expect(screen.queryByText("尚無檢舉紀錄")).toBeNull();
	});

	it("does not collide keys for equal ids of different subject cases", async () => {
		const spy = vi.spyOn(console, "error").mockImplementation(() => {});
		mocks.listMyReports.mockResolvedValue(
			create(ListMyReportsResponseSchema, {
				reports: [communityItem("1"), storeItem("1", "s1")],
			}),
		);
		render(wrap(React.createElement(MyReportsClient)));
		await screen.findByText("店-s1");
		expect(spy.mock.calls.some((c) => String(c[0]).includes("same key"))).toBe(
			false,
		);
		spy.mockRestore();
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
				[],
				[],
				{ canSupplement: false },
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

	it("hides the supplement button when the server says can_supplement is false and shows the count", async () => {
		mocks.getMyReport.mockResolvedValue(
			detail(storeReport(), 10, [], [], { canSupplement: false }),
		);
		renderDetail();
		await waitFor(() =>
			expect(screen.getByText("補充說明（10/10）")).toBeTruthy(),
		);
		expect(screen.queryByRole("button", { name: "補充說明" })).toBeNull();
	});

	it("takes the heading's maximum from supplement_max, not a client constant", async () => {
		mocks.getMyReport.mockResolvedValue(
			detail(storeReport(), 2, [], [], { supplementMax: 7 }),
		);
		renderDetail();
		await waitFor(() =>
			expect(screen.getByText("補充說明（2/7）")).toBeTruthy(),
		);
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

	it("uses can_supplement and supplement_max of the AddReportSupplement response afterwards", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 2));
		mocks.addReportSupplement.mockResolvedValue(
			detail(storeReport(), 3, [], [], {
				canSupplement: false,
				supplementMax: 3,
			}),
		);
		renderDetail();
		fireEvent.click(await screen.findByRole("button", { name: "補充說明" }));
		fireEvent.change(screen.getByLabelText(/^說明/), {
			target: { value: "最後一則" },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出補充說明" }));
		await waitFor(() =>
			expect(screen.getByText("補充說明（3/3）")).toBeTruthy(),
		);
		expect(screen.queryByRole("button", { name: "補充說明" })).toBeNull();
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

describe("SupplementDialog fixes", () => {
	async function openAndSubmit() {
		renderDetail();
		fireEvent.click(await screen.findByRole("button", { name: "補充說明" }));
		fireEvent.change(screen.getByLabelText(/^說明/), {
			target: { value: "內容" },
		});
		fireEvent.click(screen.getByRole("button", { name: "送出補充說明" }));
	}

	it("shows a server field violation on the description field", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 0));
		mocks.addReportSupplement.mockRejectedValue(
			new ConnectError("bad", Code.InvalidArgument, undefined, [
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							{
								field: "description",
								description: "x",
								localizedMessage: {
									locale: "zh-TW",
									message: "說明含不允許的內容",
								},
							},
						],
					},
				},
			]),
		);
		await openAndSubmit();
		await waitFor(() =>
			expect(screen.getByText("說明含不允許的內容")).toBeTruthy(),
		);
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("closes the dialog on failed_precondition and shows the error on the page", async () => {
		mocks.getMyReport.mockResolvedValue(detail(storeReport(), 0));
		mocks.addReportSupplement.mockRejectedValue(
			new ConnectError("已有處理結果", Code.FailedPrecondition),
		);
		await openAndSubmit();
		await waitFor(() => expect(mocks.getMyReport).toHaveBeenCalledTimes(2));
		await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
		expect(screen.getByRole("alert")).toBeTruthy();
		expect(screen.queryByRole("button", { name: "送出補充說明" })).toBeNull();
	});
});

describe("report.evidenceCount", () => {
	const en = createTranslator({
		locale: "en",
		messages: messagesFor("en"),
		namespace: "report",
	});
	it("pluralizes in en", () => {
		expect(en("evidenceCount", { count: 1 })).toBe("1 evidence photo attached");
		expect(en("evidenceCount", { count: 2 })).toBe(
			"2 evidence photos attached",
		);
	});
});
