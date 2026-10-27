// @vitest-environment jsdom

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	AppealUnavailableReason,
	GetMyStoreResponseSchema,
	type Measure,
	MeasureBasis,
	MeasureFindingSchema,
	MeasureKind,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
	RiskGround,
	SellerCapability,
	StoreStatus,
	TerminationGround,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { activeStatuses } from "@/lib/seller/capability-fixtures";
import {
	APPEAL_SENTENCE,
	REMEDIATION_SENTENCE,
	unavailable,
} from "@/tests/support/server-unavailability";

const store = vi.hoisted(() => ({
	current: null as unknown,
	measures: [] as unknown[],
	measuresLoading: false,
	measuresError: null as string | null,
	measuresState: undefined as unknown,
}));
vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useSellerMeasures: (state: unknown) => {
		store.measuresState = state;
		return {
			records: store.measures,
			loading: store.measuresLoading,
			error: store.measuresError,
			refresh: async () => {},
		};
	},
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({
		data: store.current,
		loading: false,
		error: null,
		refresh: async () => {},
	}),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({
		href,
		children,
		className,
	}: {
		href: string;
		children: React.ReactNode;
		className?: string;
	}) => React.createElement("a", { href, className }, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		href,
		disabled,
		children,
		...rest
	}: {
		href?: string;
		disabled?: boolean;
		children: React.ReactNode;
	}) =>
		href === undefined
			? React.createElement(
					"button",
					{ type: "button", disabled, ...rest },
					children,
				)
			: React.createElement("a", { href }, children),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerSuspendedNotice, {
	PENDING_DELETION_NOTICE,
} from "./SellerSuspendedNotice";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

const C = SellerCapability;
const SUSPENDED = [
	C.ACCOUNT_SELF_READ,
	C.ACCOUNT_CLOSE,
	C.ORDER_FULFILL,
	C.NOTIFICATION,
	C.INVOICE_TAX_REMEDY,
];
const PENDING_DELETION = [
	C.ACCOUNT_SELF_READ,
	C.ORDER_FULFILL,
	C.NOTIFICATION,
	C.INVOICE_TAX_REMEDY,
];

function render_(
	init: Parameters<typeof create<typeof GetMyStoreResponseSchema>>[1],
) {
	store.current = create(GetMyStoreResponseSchema, init);
	return render(React.createElement(SellerSuspendedNotice));
}

afterEach(() => {
	cleanup();
	store.current = null;
	store.measures = [];
	store.measuresLoading = false;
	store.measuresError = null;
	store.measuresState = undefined;
});

function violationMeasure(id: string, canAppeal: boolean): Measure {
	return create(MeasureSchema, {
		id,
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.VIOLATION,
		findings: [
			create(MeasureFindingSchema, {
				id: `f-${id}`,
				category: PolicyCategory.FOOD_SAFETY,
				severity: ViolationSeverity.HIGH,
			}),
		],
		statement: `理由 ${id}`,
		state: MeasureState.ACTIVE,
		canAppeal,
	});
}

function riskMeasure(
	id: string,
	canAppeal: boolean,
	extra: MessageInitShape<typeof MeasureSchema> = {},
): Measure {
	return create(MeasureSchema, {
		id,
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.RISK,
		riskGround: RiskGround.FRAUD,
		statement: `理由 ${id}`,
		state: MeasureState.ACTIVE,
		canAppeal,
		...extra,
	});
}

describe("SellerSuspendedNotice", () => {
	it("a pending-deletion store shows the order-only notice and no doubt card", () => {
		render_({
			store: { status: StoreStatus.PENDING_DELETION },
			capabilities: activeStatuses(PENDING_DELETION),
		});
		expect(screen.getByText(PENDING_DELETION_NOTICE)).toBeTruthy();
		expect(PENDING_DELETION_NOTICE).toBe(
			"帳號刪除處理中，等待未結訂單完成；期間只能處理訂單與售後",
		);
		expect(screen.queryByText("對停權決定有疑義？")).toBeNull();
		expect(screen.queryByText("聯絡客服")).toBeNull();
		expect(screen.queryByText("查看違規紀錄／提出申覆")).toBeNull();
		expect(screen.queryByText("商店已暫停營業")).toBeNull();
	});

	it("an active store never gets the pending-deletion notice", () => {
		render_({
			store: { status: StoreStatus.ACTIVE },
			capabilities: activeStatuses(PENDING_DELETION),
		});
		expect(screen.queryByText(PENDING_DELETION_NOTICE)).toBeNull();
	});

	it("a pending-deletion notice uses no suspension wording and has no store-settings link", () => {
		const { container } = render_({
			store: { status: StoreStatus.PENDING_DELETION },
			capabilities: activeStatuses(PENDING_DELETION),
		});
		expect(container.textContent).not.toContain("停權");
		expect(screen.queryByText("商店設定")).toBeNull();
	});

	it("a suspended store can reach store settings to close the store", () => {
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(
			screen.getByText("商店設定").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me");
	});

	it("only lists the links whose capability the store still has", () => {
		render_({
			store: { status: StoreStatus.PENDING_DELETION },
			capabilities: activeStatuses([C.ACCOUNT_SELF_READ, C.ORDER_FULFILL]),
		});
		expect(screen.getByText("訂單管理")).toBeTruthy();
		expect(screen.queryByText("通知")).toBeNull();
		expect(screen.queryByText("稅務身分")).toBeNull();
	});

	it("reads only the ACTIVE measures", () => {
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(store.measuresState).toBe(MeasureState.ACTIVE);
	});

	it("a pending-deletion store does not read measures", () => {
		render_({
			store: { status: StoreStatus.PENDING_DELETION },
			capabilities: activeStatuses(PENDING_DELETION),
		});
		expect(store.measuresState).toBeUndefined();
	});

	it("shows one card per active measure linking to it, appeal only where canAppeal", () => {
		store.measures = [violationMeasure("m1", true), riskMeasure("m2", false)];
		const { container } = render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(screen.getByText("理由 m1")).toBeTruthy();
		expect(screen.getByText("理由 m2")).toBeTruthy();
		expect(screen.getByText("食品安全")).toBeTruthy();
		expect(screen.getByText("風險：詐欺")).toBeTruthy();
		const hrefs = screen
			.getAllByText("查看措施")
			.map((el) => el.closest("a")?.getAttribute("href"));
		expect(hrefs).toEqual(["/seller/measures/m1", "/seller/measures/m2"]);
		// 申覆入口每張都有：canAppeal 的是連結（不論措施是違規處分還是管理員建立的風險措施），其餘反灰。
		const links = screen.getAllByRole("link", { name: "提出申覆" });
		expect(links).toHaveLength(1);
		expect(links[0]?.getAttribute("href")).toBe("/seller/measures/m1/appeal");
		expect(screen.getAllByRole("button", { name: "提出申覆" })).toHaveLength(1);
		expect(container.textContent).not.toContain("停權僅能由平台管理員解除");
		expect(container.textContent).not.toContain("查看違規紀錄");
		// 4B 的「管理員停權導向客服」已由措施卡取代（10e 計畫段「排程」）。
		expect(container.textContent).not.toContain("聯絡客服");
		expect(container.textContent).not.toContain("對停權決定有疑義");
	});

	it("an admin-created risk measure can be appealed when the server says so", () => {
		store.measures = [riskMeasure("r1", true)];
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(
			screen.getByText("提出申覆").closest("a")?.getAttribute("href"),
		).toBe("/seller/measures/r1/appeal");
	});

	it("RISK 措施的解除請求入口：能用是連到該措施的連結，不能用是反灰按鈕並以 aria-describedby 關聯原因，VIOLATION 沒有", () => {
		store.measures = [
			riskMeasure("ok", false, { canRequestRemediation: true }),
			riskMeasure("wait", false, {
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.REMEDIATION_PENDING,
						REMEDIATION_SENTENCE.REMEDIATION_PENDING,
					),
				],
			}),
			violationMeasure("v", false),
		];
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		const link = screen.getByText("已改善，請求解除", { selector: "a" });
		expect(link.getAttribute("href")).toBe("/seller/measures/ok/remediation");
		const disabled = screen.getByRole("button", { name: "已改善，請求解除" });
		expect((disabled as HTMLButtonElement).disabled).toBe(true);
		const reason = document.getElementById(
			disabled.getAttribute("aria-describedby") ?? "",
		);
		expect(reason?.textContent).toBe(REMEDIATION_SENTENCE.REMEDIATION_PENDING);
		expect(screen.getAllByText("已改善，請求解除")).toHaveLength(2);
	});

	it("no card links to the appeal page when no measure is appealable: the entry is a disabled button with the server's first reason", () => {
		store.measures = [
			riskMeasure("r1", false, {
				appealUnavailabilities: [
					unavailable(
						AppealUnavailableReason.ALREADY_APPEALED,
						APPEAL_SENTENCE.ALREADY_APPEALED,
					),
					unavailable(AppealUnavailableReason.REVOKED, APPEAL_SENTENCE.REVOKED),
				],
			}),
		];
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(screen.queryByRole("link", { name: "提出申覆" })).toBeNull();
		const button = screen.getByRole("button", { name: "提出申覆" });
		expect((button as HTMLButtonElement).disabled).toBe(true);
		expect(
			document.getElementById(button.getAttribute("aria-describedby") ?? "")
				?.textContent,
		).toBe(APPEAL_SENTENCE.ALREADY_APPEALED);
		expect(screen.getByText("查看措施")).toBeTruthy();
	});

	describe("終止合作生效後", () => {
		const termination = (
			extra: {
				canAppeal?: boolean;
				appealUnavailabilities?: ReturnType<typeof unavailable>[];
			} = {},
		) =>
			create(MeasureSchema, {
				id: "t1",
				kind: MeasureKind.TERMINATION,
				basis: MeasureBasis.CONTRACT,
				terminationGround: TerminationGround.LICENSE_REVOKED,
				state: MeasureState.ACTIVE,
				canAppeal: true,
				period: {
					startTime: timestampFromDate(new Date("2026-10-01T02:00:00Z")),
				},
				appealDeadlineTime: timestampFromDate(new Date("2027-04-04T02:00:00Z")),
				...extra,
			});
		const open = () =>
			render_({
				store: { status: StoreStatus.ACTIVE },
				suspended: true,
				capabilities: activeStatuses(SUSPENDED),
			});

		it("標題與副標是終止專屬文字，不再說停權", () => {
			store.measures = [termination()];
			const { container } = open();
			expect(screen.getByRole("heading", { name: "合作已終止" })).toBeTruthy();
			expect(
				screen.getByText(
					"店家已暫停營業，已成立的訂單仍需完成；訂單完成後店家會關閉。終止生效後 6 個月內申覆成立，可恢復同一家店。",
				),
			).toBeTruthy();
			expect(screen.queryByText("商店已暫停營業")).toBeNull();
			expect(container.textContent).not.toContain("停權");
			expect(container.textContent).not.toContain("停權中");
		});

		it("卡片顯示終止生效時間、事由、申覆期限，申覆入口可用", () => {
			store.measures = [termination()];
			open();
			expect(screen.getByText("終止生效：2026/10/01 10:00")).toBeTruthy();
			expect(
				screen.getByText("措施：終止合作 · 事由：必要許可遭撤銷"),
			).toBeTruthy();
			expect(screen.getByText("申覆期限：2027/04/04 10:00")).toBeTruthy();
			expect(
				screen.getByRole("link", { name: "提出申覆" }).getAttribute("href"),
			).toBe("/seller/measures/t1/appeal");
		});

		it("申覆期限已過：反灰並附原因與期限", () => {
			store.measures = [
				termination({
					canAppeal: false,
					appealUnavailabilities: [
						unavailable(
							AppealUnavailableReason.APPEAL_WINDOW_ENDED,
							APPEAL_SENTENCE.APPEAL_WINDOW_ENDED,
						),
					],
				}),
			];
			open();
			expect(
				screen.getByText(
					`${APPEAL_SENTENCE.APPEAL_WINDOW_ENDED} · 申覆期限：2027/04/04 10:00`,
				),
			).toBeTruthy();
			expect(
				(screen.getByRole("button", { name: "提出申覆" }) as HTMLButtonElement)
					.disabled,
			).toBe(true);
		});

		it("終止公告有「終止原因」區段與「對終止決定有疑義？」入口，連到措施與申覆", () => {
			store.measures = [termination()];
			open();
			expect(screen.getByRole("heading", { name: "終止原因" })).toBeTruthy();
			expect(screen.queryByText("生效中的措施")).toBeNull();
			expect(
				screen.getByRole("heading", { name: "對終止決定有疑義？" }),
			).toBeTruthy();
			const entries = screen
				.getAllByRole("link", { name: "措施與申覆" })
				.map((a) => a.getAttribute("href"));
			expect(entries).toEqual(["/seller/measures"]);
		});

		it("停權公告沒有終止專屬的標題與疑義入口", () => {
			store.measures = [violationMeasure("m1", true)];
			open();
			expect(screen.queryByText("終止原因")).toBeNull();
			expect(screen.queryByText("對終止決定有疑義？")).toBeNull();
			expect(screen.getByText("生效中的措施")).toBeTruthy();
		});

		it.each([
			["讀取中", { measuresLoading: true, measuresError: null }],
			["讀取失敗", { measuresLoading: false, measuresError: "無法載入" }],
		])("措施%s時不顯示停權文字（還不知道是不是終止）", (_name, state) => {
			store.measuresLoading = state.measuresLoading;
			store.measuresError = state.measuresError;
			const { container } = open();
			expect(container.textContent).not.toContain("停權");
			expect(container.textContent).not.toContain("對終止決定有疑義");
			expect(
				screen.getByRole("heading", { name: "商店已暫停營業" }),
			).toBeTruthy();
			expect(screen.getByText("目前仍可使用")).toBeTruthy();
		});

		it("措施讀完且只有停權時才顯示停權文字", () => {
			store.measures = [violationMeasure("m1", true)];
			const { container } = open();
			expect(container.textContent).toContain("帳號已被平台停權");
			expect(container.textContent).toContain("停權期間仍可使用");
		});

		it("只有停權時仍是停權公告", () => {
			store.measures = [violationMeasure("m1", true)];
			open();
			expect(
				screen.getByRole("heading", { name: "商店已暫停營業" }),
			).toBeTruthy();
			expect(screen.queryByText("合作已終止")).toBeNull();
		});
	});

	it("shows the read failure instead of an empty list", () => {
		store.measuresError = "無法載入";
		render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(screen.getByText("無法載入")).toBeTruthy();
	});

	it("no suspension notice promises a review or reply, or says only admins can lift it", () => {
		const variants: Measure[][] = [
			[],
			[violationMeasure("m1", true), riskMeasure("m2", false)],
			[riskMeasure("r1", true)],
		];
		for (const measures of variants) {
			store.measures = measures;
			const { container, unmount } = render_({
				store: { status: StoreStatus.ACTIVE },
				suspended: true,
				capabilities: activeStatuses(SUSPENDED),
			});
			expect(container.textContent).not.toContain("停權僅能由平台管理員解除");
			expect(container.textContent).not.toContain("複核");
			expect(container.textContent).not.toContain("回覆審核結果");
			unmount();
		}
		store.measuresError = "無法載入";
		const { container } = render_({
			store: { status: StoreStatus.ACTIVE },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(container.textContent).not.toContain("停權僅能由平台管理員解除");
		expect(container.textContent).not.toContain("複核");
		expect(container.textContent).not.toContain("回覆審核結果");
	});

	it("a suspended store that is also pending deletion keeps the suspension notice", () => {
		render_({
			store: { status: StoreStatus.PENDING_DELETION },
			suspended: true,
			capabilities: activeStatuses(SUSPENDED),
		});
		expect(screen.getByText("商店已暫停營業")).toBeTruthy();
		expect(screen.queryByText(PENDING_DELETION_NOTICE)).toBeNull();
	});
});
