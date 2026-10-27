/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it } from "vitest";
import HawkrAgentCapabilityIcon from "./HawkrAgentCapabilityIcon";
import HawkrAgentConversationMarble from "./HawkrAgentConversationMarble";
import HawkrAgentMark from "./HawkrAgentMark";
import HawkrAgentSprite from "./HawkrAgentSprite";

afterEach(cleanup);

function host(container: HTMLElement): HTMLElement {
	return container.firstElementChild as HTMLElement;
}

describe("HawkrAgentMark", () => {
	it("有 label 時承載語意，並畫出 Hawkr 標誌與 AI 角標", () => {
		const { container } = render(
			React.createElement(HawkrAgentMark, { label: "Hawkr Agent" }),
		);
		const mark = host(container);
		expect(mark.getAttribute("role")).toBe("img");
		expect(mark.getAttribute("aria-label")).toBe("Hawkr Agent");
		expect(mark.querySelector("img.brand-logo__lockup")).toBeTruthy();
		expect(mark.querySelector("small")?.textContent).toBe("AI");
	});

	it("省略 label 時整顆視為裝飾，hideBadge 去掉角標", () => {
		const { container } = render(
			React.createElement(HawkrAgentMark, { hideBadge: true }),
		);
		const mark = host(container);
		expect(mark.getAttribute("aria-hidden")).toBe("true");
		expect(mark.getAttribute("role")).toBeNull();
		expect(mark.querySelector("small")).toBeNull();
	});
});

describe("HawkrAgentSprite", () => {
	it("處理中的狀態用標誌的載入動態，標籤與 data-motion 保留狀態", () => {
		const { container } = render(
			React.createElement(HawkrAgentSprite, { state: "SEARCHING" }),
		);
		const sprite = host(container);
		expect(sprite.getAttribute("role")).toBe("img");
		expect(sprite.getAttribute("aria-label")).toBe("Hawkr Agent 正在找貨");
		expect(sprite.getAttribute("data-motion")).toBe("SEARCHING");
		expect(sprite.querySelector(".il-logo")).toBeTruthy();
	});

	it("非循環狀態用靜態標誌，不掛載入動態", () => {
		const { container } = render(
			React.createElement(HawkrAgentSprite, { state: "SUCCESS" }),
		);
		const sprite = host(container);
		expect(sprite.getAttribute("aria-label")).toBe("Hawkr Agent 已完成");
		expect(sprite.querySelector(".il-logo")).toBeNull();
		expect(sprite.querySelector("img.brand-logo__lockup")).toBeTruthy();
	});

	it("重新連線用靜態標誌，不畫成仍在工作的載入動態", () => {
		const { container } = render(
			React.createElement(HawkrAgentSprite, { state: "RECONNECTING" }),
		);
		const sprite = host(container);
		expect(sprite.getAttribute("data-motion")).toBe("RECONNECTING");
		expect(sprite.querySelector(".il-logo")).toBeNull();
		expect(sprite.querySelector("img.brand-logo__lockup")).toBeTruthy();
	});

	it("label 傳空字串時整顆裝飾，disabled 時不渲染", () => {
		const decorative = render(
			React.createElement(HawkrAgentSprite, { state: "IDLE", label: "" }),
		);
		expect(host(decorative.container).getAttribute("aria-hidden")).toBe("true");
		expect(host(decorative.container).getAttribute("role")).toBeNull();
		cleanup();

		const disabled = render(
			React.createElement(HawkrAgentSprite, { state: "IDLE", disabled: true }),
		);
		expect(disabled.container.firstElementChild).toBeNull();
	});
});

describe("HawkrAgentConversationMarble", () => {
	it("已知分類帶可讀名稱與對應 glyph", () => {
		const { container } = render(
			React.createElement(HawkrAgentConversationMarble, {
				primaryCategoryId: "vegetables",
			}),
		);
		const marble = host(container);
		expect(marble.getAttribute("role")).toBe("img");
		expect(marble.getAttribute("aria-label")).toBe("大分類：蔬菜");
		expect(marble.getAttribute("data-category")).toBe("vegetables");
		expect(
			marble.querySelector('[data-food-category="vegetables"] svg'),
		).toBeTruthy();
	});

	it("分類不明時保持裝飾，不猜品類", () => {
		const { container } = render(
			React.createElement(HawkrAgentConversationMarble, {
				primaryCategoryId: null,
			}),
		);
		const marble = host(container);
		expect(marble.getAttribute("aria-hidden")).toBe("true");
		expect(marble.getAttribute("role")).toBeNull();
		expect(marble.getAttribute("data-category")).toBe("unclassified");
		expect(marble.querySelector("svg")).toBeNull();
	});
});

describe("HawkrAgentCapabilityIcon", () => {
	it.each([
		"find-suppliers",
		"organise-requirements",
		"compare-candidates",
	] as const)("%s 是裝飾性的靜態 glyph", (icon) => {
		const { container } = render(
			React.createElement(HawkrAgentCapabilityIcon, { icon }),
		);
		const node = host(container);
		expect(node.getAttribute("aria-hidden")).toBe("true");
		expect(node.getAttribute("data-capability-icon")).toBe(icon);
		expect(node.querySelector("svg")).toBeTruthy();
	});
});
