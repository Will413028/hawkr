// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	AppealUnavailableReason,
	type Measure,
	MeasureBasis,
	MeasureFindingSchema,
	MeasureKind,
	MeasureRequestKind,
	MeasureRequestSchema,
	MeasureRequestState,
	MeasureSchema,
	MeasureState,
	RemediationUnavailableReason,
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
	REMEDIATION_SENTENCE,
	unavailable,
} from "@/tests/support/server-unavailability";

const mocks = vi.hoisted(() => ({
	records: [] as unknown[],
}));

vi.mock("@/lib/hooks/useSellerMeasures", () => ({
	useSellerMeasures: () => ({
		records: mocks.records,
		loading: false,
		error: null,
		refresh: async () => {},
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
vi.mock(
	"@/features/seller-center/components/SellerMobileDisclosureList",
	() => ({
		SellerMobileDisclosureIcon: () => null,
		SellerMobileDisclosureList: ({
			items,
			renderDetails,
		}: {
			items: unknown[];
			renderDetails: (item: never) => React.ReactNode;
		}) =>
			React.createElement(
				"div",
				{ "data-testid": "mobile" },
				items.map((item, i) =>
					React.createElement(
						React.Fragment,
						{ key: i },
						renderDetails(item as never),
					),
				),
			),
	}),
);
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

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SellerMeasuresPage from "./SellerMeasuresPage";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

// 伺服器判斷的 can_appeal 才是依據：這兩筆都是生效中的停權，只有 canAppeal 不同。
function measure(canAppeal: boolean): Measure {
	return create(MeasureSchema, {
		id: canAppeal ? "can" : "cannot",
		kind: MeasureKind.SUSPENSION,
		basis: MeasureBasis.VIOLATION,
		findings: [
			create(MeasureFindingSchema, {
				id: "f1",
				category: PolicyCategory.FOOD_SAFETY,
			}),
		],
		statement: "標示不實",
		state: MeasureState.ACTIVE,
		canAppeal,
		etag: "etag-1",
	});
}

describe("SellerMeasuresPage 的申覆入口", () => {
	beforeEach(() => {
		mocks.records = [];
	});
	afterEach(cleanup);

	it("頁名是措施與申覆，卡片顯示依據、理由與措施", () => {
		mocks.records = [measure(false)];
		render(React.createElement(SellerMeasuresPage));
		expect(screen.getByRole("heading", { name: "措施與申覆" })).toBeTruthy();
		expect(screen.getAllByText("食品安全").length).toBeGreaterThan(0);
		expect(screen.getAllByText("標示不實").length).toBeGreaterThan(0);
		expect(screen.getAllByText("措施：停權").length).toBeGreaterThan(0);
	});

	it("canAppeal 為 true：顯示提出申覆，連到該措施的申覆頁", () => {
		mocks.records = [measure(true)];
		render(React.createElement(SellerMeasuresPage));
		const links = screen.getAllByRole("link", { name: "提出申覆" });
		expect(links).toHaveLength(2); // 桌面卡片與手機展開內容
		for (const link of links) {
			expect(link.getAttribute("href")).toBe("/seller/measures/can/appeal");
		}
	});

	it("canAppeal 為 false：入口仍顯示但反灰（不是連結），附伺服器原因的第一項（桌面與手機各一）", () => {
		mocks.records = [
			create(MeasureSchema, {
				...measure(false),
				appealUnavailabilities: [
					appealUnavailability(
						AppealUnavailableReason.REVOKED,
						APPEAL_SENTENCE.REVOKED,
					),
					appealUnavailability(
						AppealUnavailableReason.ALREADY_APPEALED,
						APPEAL_SENTENCE.ALREADY_APPEALED,
					),
				],
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		expect(screen.queryByRole("link", { name: "提出申覆" })).toBeNull();
		const buttons = screen.getAllByRole("button", { name: "提出申覆" });
		expect(buttons).toHaveLength(2);
		for (const button of buttons) {
			expect((button as HTMLButtonElement).disabled).toBe(true);
		}
		expect(screen.getAllByText(APPEAL_SENTENCE.REVOKED)).toHaveLength(2);
		expect(screen.queryByText(APPEAL_SENTENCE.ALREADY_APPEALED)).toBeNull();
	});
});

describe("SellerMeasuresPage 的狀態標籤", () => {
	beforeEach(() => {
		mocks.records = [];
	});
	afterEach(cleanup);

	it("申覆審核中：措施狀態標籤與申覆標籤並列，誰也不蓋掉誰", () => {
		mocks.records = [
			create(MeasureSchema, {
				...measure(false),
				requests: [
					create(MeasureRequestSchema, {
						kind: MeasureRequestKind.APPEAL,
						state: MeasureRequestState.PENDING,
					}),
				],
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		const states = screen.getAllByText("停權中");
		const requests = screen.getAllByText("申覆審核中");
		expect(states.length).toBeGreaterThan(0);
		expect(requests.length).toBe(states.length);
		// 同一張卡片內：狀態在前、請求在後，兩者同一個容器
		expect(states[0].parentElement?.parentElement).toBe(
			requests[0].parentElement?.parentElement,
		);
		expect(
			states[0].compareDocumentPosition(requests[0]) &
				Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
	});

	it("沒有待處理請求：只有措施狀態一個標籤", () => {
		mocks.records = [measure(false)];
		render(React.createElement(SellerMeasuresPage));
		expect(screen.getAllByText("停權中").length).toBeGreaterThan(0);
		expect(screen.queryByText("申覆審核中")).toBeNull();
	});
});

describe("SellerMeasuresPage 的解除請求入口", () => {
	const LABEL = "已改善，請求解除";
	const risk = (init: Parameters<typeof create<typeof MeasureSchema>>[1]) =>
		create(MeasureSchema, {
			id: "r1",
			kind: MeasureKind.SUSPENSION,
			basis: MeasureBasis.RISK,
			statement: "風險待改善",
			state: MeasureState.ACTIVE,
			etag: "etag-1",
			...init,
		});
	const buttons = () => screen.queryAllByRole("button", { name: LABEL });
	const links = () => screen.queryAllByRole("link", { name: LABEL });
	/** 按鈕以 aria-describedby 指向的說明元素文字（不是整頁任何相同文字）。 */
	const reasonOf = (button: HTMLElement) =>
		document.getElementById(button.getAttribute("aria-describedby") ?? "")
			?.textContent;

	beforeEach(() => {
		mocks.records = [];
	});
	afterEach(cleanup);

	it("RISK 生效中且 canRequestRemediation：是連到該措施解除請求頁的連結", () => {
		mocks.records = [risk({ canRequestRemediation: true })];
		render(React.createElement(SellerMeasuresPage));
		expect(links()).toHaveLength(2); // 桌面卡片與手機展開內容
		expect(buttons()).toHaveLength(0);
		for (const link of links()) {
			expect(link.getAttribute("href")).toBe("/seller/measures/r1/remediation");
		}
	});

	it("解除請求審核中：反灰並附原因，不是連結", () => {
		mocks.records = [
			risk({
				requests: [
					create(MeasureRequestSchema, {
						kind: MeasureRequestKind.REMEDIATION,
						state: MeasureRequestState.PENDING,
					}),
				],
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.REMEDIATION_PENDING,
						REMEDIATION_SENTENCE.REMEDIATION_PENDING,
					),
				],
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		expect((buttons()[0] as HTMLButtonElement).disabled).toBe(true);
		expect(reasonOf(buttons()[0])).toBe(
			REMEDIATION_SENTENCE.REMEDIATION_PENDING,
		);
		expect(links()).toHaveLength(0);
	});

	it("申覆審核中：反灰，原因是申覆審核中", () => {
		mocks.records = [
			risk({
				requests: [
					create(MeasureRequestSchema, {
						kind: MeasureRequestKind.APPEAL,
						state: MeasureRequestState.PENDING,
					}),
				],
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.APPEAL_PENDING,
						REMEDIATION_SENTENCE.APPEAL_PENDING,
					),
				],
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		expect((buttons()[0] as HTMLButtonElement).disabled).toBe(true);
		expect(reasonOf(buttons()[0])).toBe(REMEDIATION_SENTENCE.APPEAL_PENDING);
	});

	it("冷卻：反灰，原因是伺服器組好的句子（含台北時間），client 不用 nextRemediationTime 自己組", () => {
		mocks.records = [
			risk({
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.COOLDOWN,
						REMEDIATION_SENTENCE.COOLDOWN,
					),
				],
				nextRemediationTime: timestampFromDate(
					new Date("2026-10-20T02:05:00Z"),
				),
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		expect((buttons()[0] as HTMLButtonElement).disabled).toBe(true);
		expect(reasonOf(buttons()[0])).toBe(REMEDIATION_SENTENCE.COOLDOWN);
		// 冷卻時間只在按鈕的原因，不在說明行（說明行留給裁定理由）
		expect(screen.getAllByText(/後可再提出/)).toHaveLength(2); // 桌面與手機各一個按鈕原因
	});

	it("伺服器說 NOT_ACTIVE 或 NOT_RISK：不顯示；沒有原因也不能請求時同樣不顯示", () => {
		mocks.records = [
			risk({
				id: "e",
				state: MeasureState.ENDED,
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.NOT_ACTIVE,
						REMEDIATION_SENTENCE.NOT_ACTIVE,
					),
				],
			}),
			risk({
				id: "v",
				basis: MeasureBasis.VIOLATION,
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.NOT_RISK,
						REMEDIATION_SENTENCE.NOT_RISK,
					),
				],
			}),
			risk({ id: "n" }),
		];
		render(React.createElement(SellerMeasuresPage));
		expect(buttons()).toHaveLength(0);
	});

	it("入口不看措施的依據與狀態：伺服器說能請求就顯示", () => {
		mocks.records = [
			risk({
				state: MeasureState.ENDED,
				basis: MeasureBasis.VIOLATION,
				canRequestRemediation: true,
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		expect(links()).toHaveLength(2);
	});

	it("同一筆措施的桌面與手機入口，反灰原因的 id 不重複，且各自的按鈕指向自己的原因", () => {
		mocks.records = [
			risk({
				remediationUnavailabilities: [
					unavailable(
						RemediationUnavailableReason.REMEDIATION_PENDING,
						REMEDIATION_SENTENCE.REMEDIATION_PENDING,
					),
				],
			}),
		];
		render(React.createElement(SellerMeasuresPage));
		const ids = buttons().map((b) => b.getAttribute("aria-describedby"));
		expect(ids).toHaveLength(2);
		expect(ids[0]).toBeTruthy();
		expect(new Set(ids).size).toBe(2);
		for (const b of buttons()) {
			const target = document.getElementById(
				b.getAttribute("aria-describedby") ?? "",
			);
			expect(target?.textContent).toBe(
				REMEDIATION_SENTENCE.REMEDIATION_PENDING,
			);
			expect(document.querySelectorAll(`[id="${target?.id}"]`)).toHaveLength(1);
		}
	});
});
