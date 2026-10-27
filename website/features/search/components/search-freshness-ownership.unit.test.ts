/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import SearchClient from "../SearchClient";

const state = vi.hoisted(() => ({ search: "", refresh: vi.fn() }));
vi.mock("@/lib/hooks/useUrlSearchParams", () => ({
	useUrlSearchParams: () => new URLSearchParams(state.search),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ refresh: state.refresh }),
}));
vi.mock("./SearchForm", () => ({ default: () => null }));
vi.mock("./SearchHashtagNav", () => ({ default: () => null }));
vi.mock("./SearchResultsClient", () => ({
	default: ({ query }: { query: string }) =>
		React.createElement("p", {}, `Query:${query}`),
}));

function element() {
	return React.createElement(SearchClient, {
		landing: React.createElement("p", {}, "Landing editorial"),
	});
}
beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date("2026-09-07T12:00:00Z"));
	state.search = "";
	state.refresh.mockReset();
	Object.defineProperty(document, "visibilityState", {
		configurable: true,
		value: "visible",
	});
});
afterEach(() => {
	cleanup();
	vi.useRealTimers();
});

describe("search freshness ownership", () => {
	it("refreshes an aged landing first revealed after arriving on a keyword URL", () => {
		state.search = "q=芒果";
		const view = render(element());
		act(() => {
			vi.advanceTimersByTime(6 * 60_000);
		});
		state.search = "";
		view.rerender(element());
		expect(state.refresh).toHaveBeenCalledTimes(1);
	});

	it("does not refresh the landing RSC tree while keyword Query results are active", () => {
		state.search = "q=芒果";
		render(element());
		expect(screen.getByText("Query:芒果")).toBeTruthy();
		act(() => {
			vi.advanceTimersByTime(6 * 60_000);
		});
		fireEvent.focus(window);
		fireEvent(window, new Event("online"));
		fireEvent(document, new Event("visibilitychange"));
		expect(state.refresh).not.toHaveBeenCalled();
	});

	it("remembers landing age across keyword navigation and refreshes its editorial content on return", () => {
		const view = render(element());
		expect(screen.getByText("Landing editorial")).toBeTruthy();
		act(() => {
			vi.advanceTimersByTime(60_000);
		});
		state.search = "q=芒果";
		view.rerender(element());
		act(() => {
			vi.advanceTimersByTime(5 * 60_000);
		});
		fireEvent.focus(window);
		expect(state.refresh).not.toHaveBeenCalled();
		state.search = "";
		view.rerender(element());
		expect(screen.getByText("Landing editorial")).toBeTruthy();
		expect(state.refresh).toHaveBeenCalledTimes(1);
		fireEvent.focus(window);
		expect(state.refresh).toHaveBeenCalledTimes(1);
	});

	it("keeps a fresh landing return free of additional RSC updates", () => {
		const view = render(element());
		state.search = "q=芒果";
		view.rerender(element());
		act(() => {
			vi.advanceTimersByTime(30_000);
		});
		state.search = "";
		view.rerender(element());
		expect(state.refresh).not.toHaveBeenCalled();
	});
});
