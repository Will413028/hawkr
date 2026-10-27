// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	AppealUnavailableReason,
	type Measure,
	MeasureBasis,
	MeasureKind,
	MeasureSchema,
	MeasureState,
	RiskGround,
	TerminationGround,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	APPEAL_SENTENCE,
	appealUnavailability,
	unavailable,
} from "@/tests/support/server-unavailability";

const mocks = vi.hoisted(() => ({
	result: {
		measure: null as unknown,
		loading: false,
		error: null as string | null,
	},
	requestedId: undefined as string | undefined,
}));

vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useSellerMeasure: (id: string) => {
		mocks.requestedId = id;
		return mocks.result;
	},
}));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({
		title,
		actions,
		children,
	}: {
		title?: React.ReactNode;
		actions?: React.ReactNode;
		children?: React.ReactNode;
	}) =>
		React.createElement(
			"div",
			null,
			React.createElement("h1", null, title),
			actions,
			children,
		),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: React.ReactNode }) =>
		React.createElement("a", { href }, children),
}));
vi.mock("@/features/seller-center/components/SellerSkeletons", () => ({
	SellerCardGridSkeleton: () =>
		React.createElement("div", { "data-testid": "skeleton" }),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerMeasureDetailPage from "./SellerMeasureDetailPage";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

function measure(canAppeal: boolean): Measure {
	return create(MeasureSchema, {
		id: "m9",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.RISK,
		riskGround: RiskGround.SECURITY,
		statement: "帳號異常登入",
		state: MeasureState.ACTIVE,
		canAppeal,
		etag: "e1",
	});
}

beforeEach(() => {
	mocks.result = { measure: null, loading: false, error: null };
	mocks.requestedId = undefined;
});
afterEach(cleanup);

describe("SellerMeasureDetailPage", () => {
	it("以網址的 id 讀單筆，顯示依據與理由，並可回到全部措施", () => {
		mocks.result.measure = measure(false);
		render(React.createElement(SellerMeasureDetailPage, { measureId: "m9" }));
		expect(mocks.requestedId).toBe("m9");
		expect(screen.getByText("風險：資安")).toBeTruthy();
		expect(screen.getByText("帳號異常登入")).toBeTruthy();
		expect(
			screen.getByText("查看全部措施").closest("a")?.getAttribute("href"),
		).toBe("/seller/measures");
	});

	it("canAppeal 為 true 才有申覆入口，連到該措施的申覆頁", () => {
		mocks.result.measure = measure(true);
		render(React.createElement(SellerMeasureDetailPage, { measureId: "m9" }));
		expect(
			screen.getByText("提出申覆").closest("a")?.getAttribute("href"),
		).toBe("/seller/measures/m9/appeal");
	});

	it("canRequestRemediation 為 true：解除請求入口連到該措施的解除請求頁", () => {
		mocks.result.measure = create(MeasureSchema, {
			...measure(false),
			canRequestRemediation: true,
		});
		render(React.createElement(SellerMeasureDetailPage, { measureId: "m9" }));
		expect(
			screen.getByText("已改善，請求解除").closest("a")?.getAttribute("href"),
		).toBe("/seller/measures/m9/remediation");
	});

	it("canAppeal 為 false：申覆入口仍顯示，是反灰的按鈕並附伺服器原因的第一項，不是連結", () => {
		mocks.result.measure = create(MeasureSchema, {
			...measure(false),
			appealUnavailabilities: [
				appealUnavailability(
					AppealUnavailableReason.ALREADY_APPEALED,
					APPEAL_SENTENCE.ALREADY_APPEALED,
				),
				appealUnavailability(
					AppealUnavailableReason.REQUEST_PENDING,
					APPEAL_SENTENCE.REQUEST_PENDING,
				),
			],
		});
		render(React.createElement(SellerMeasureDetailPage, { measureId: "m9" }));
		const button = screen.getByRole("button", { name: "提出申覆" });
		expect((button as HTMLButtonElement).disabled).toBe(true);
		expect(screen.queryByRole("link", { name: "提出申覆" })).toBeNull();
		const reason = document.getElementById(
			button.getAttribute("aria-describedby") ?? "",
		);
		expect(reason?.textContent).toBe(APPEAL_SENTENCE.ALREADY_APPEALED);
	});

	it("終止合作：申覆入口附申覆期限（伺服器的 appealDeadlineTime）；期限已過時反灰並附原因與期限", () => {
		mocks.result.measure = create(MeasureSchema, {
			id: "t1",
			kind: MeasureKind.TERMINATION,
			basis: MeasureBasis.CONTRACT,
			terminationGround: TerminationGround.NOTICE,
			state: MeasureState.ACTIVE,
			canAppeal: false,
			appealUnavailabilities: [
				unavailable(
					AppealUnavailableReason.APPEAL_WINDOW_ENDED,
					APPEAL_SENTENCE.APPEAL_WINDOW_ENDED,
				),
			],
			// 2026-10-01 02:00 UTC = 台北 10:00
			appealDeadlineTime: timestampFromDate(new Date("2026-10-01T02:00:00Z")),
		});
		render(React.createElement(SellerMeasureDetailPage, { measureId: "t1" }));
		expect(
			screen.getByText(
				`${APPEAL_SENTENCE.APPEAL_WINDOW_ENDED} · 申覆期限：2026/10/01 10:00`,
			),
		).toBeTruthy();
		expect(
			(screen.getByRole("button", { name: "提出申覆" }) as HTMLButtonElement)
				.disabled,
		).toBe(true);
	});

	it("讀取失敗顯示伺服器的文字；讀取中顯示骨架", () => {
		mocks.result.error = "找不到這筆資料";
		const { unmount } = render(
			React.createElement(SellerMeasureDetailPage, { measureId: "m9" }),
		);
		expect(screen.getByText("找不到這筆資料")).toBeTruthy();
		unmount();
		mocks.result = { measure: null, loading: true, error: null };
		render(React.createElement(SellerMeasureDetailPage, { measureId: "m9" }));
		expect(screen.getByTestId("skeleton")).toBeTruthy();
	});
});
