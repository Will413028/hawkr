// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	MeasureBasis,
	MeasureKind,
	MeasureSchema,
	MeasureState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	records: [] as unknown[],
	state: undefined as unknown,
	hasMeasures: false,
}));

vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useSellerMeasures: (state: unknown) => {
		mocks.state = state;
		return {
			records: mocks.records,
			loading: false,
			error: null,
			refresh: async () => {},
		};
	},
	useHasMeasures: () => mocks.hasMeasures,
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
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Card", () => ({
	Card: ({ href, children }: { href?: string; children: React.ReactNode }) =>
		React.createElement("a", { href }, children),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import MeasuresAccountEntry from "./MeasuresAccountEntry";
import TerminationNoticeBanner from "./TerminationNoticeBanner";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

const START = timestampFromDate(new Date("2026-10-01T02:00:00Z"));
const scheduled = (id: string, kind = MeasureKind.TERMINATION) =>
	create(MeasureSchema, {
		id,
		kind,
		basis: MeasureBasis.CONTRACT,
		state: MeasureState.SCHEDULED,
		period: { startTime: START },
		// 建立時間與生效時間不差 30 天：生效時間只能讀 period.startTime。
		createTime: timestampFromDate(new Date("2026-09-20T02:00:00Z")),
	});

afterEach(() => {
	cleanup();
	mocks.records = [];
	mocks.state = undefined;
	mocks.hasMeasures = false;
});

describe("TerminationNoticeBanner", () => {
	it("讀 SCHEDULED 的措施；預告終止顯示生效時間與措施頁連結", () => {
		mocks.records = [scheduled("t1")];
		render(React.createElement(TerminationNoticeBanner));
		expect(mocks.state).toBe(MeasureState.SCHEDULED);
		expect(
			screen.getByText(
				"平台已通知將於 2026/10/01 10:00 終止合作。在此之前店家照常營業；如有異議，可提出申覆。",
			),
		).toBeTruthy();
		expect(
			screen.getByRole("link", { name: "措施與申覆" }).getAttribute("href"),
		).toBe("/seller/measures/t1");
		expect(screen.getByRole("status")).toBeTruthy();
	});

	it("多筆預告：每筆各有自己的措施頁連結，順序照伺服器給的", () => {
		mocks.records = [scheduled("t2"), scheduled("t1")];
		render(React.createElement(TerminationNoticeBanner));
		expect(
			screen
				.getAllByRole("link", { name: "措施與申覆" })
				.map((a) => a.getAttribute("href")),
		).toEqual(["/seller/measures/t2", "/seller/measures/t1"]);
		expect(screen.getAllByRole("status")).toHaveLength(2);
	});

	it("沒有預定生效的措施、或不是終止合作：不顯示", () => {
		const { container, rerender } = render(
			React.createElement(TerminationNoticeBanner),
		);
		expect(container.textContent).toBe("");
		mocks.records = [scheduled("s1", MeasureKind.SUSPENSION)];
		rerender(React.createElement(TerminationNoticeBanner));
		expect(container.textContent).toBe("");
	});
});

describe("MeasuresAccountEntry", () => {
	it("帳號有措施才顯示「措施與申覆」，連到帳號層級的措施頁", () => {
		const { container, rerender } = render(
			React.createElement(MeasuresAccountEntry),
		);
		expect(container.textContent).toBe("");
		mocks.hasMeasures = true;
		rerender(React.createElement(MeasuresAccountEntry));
		expect(
			screen.getByText("措施與申覆").closest("a")?.getAttribute("href"),
		).toBe("/seller/measures");
	});
});
