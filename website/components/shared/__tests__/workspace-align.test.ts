// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	auth: {
		status: "unknown" as string,
		activeRole: null as "buyer" | "seller" | null,
		switchRole: vi.fn(() => true),
	},
	stored: null as "buyer" | "seller" | null,
	commit: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => mocks.auth }));
vi.mock("@/lib/auth/storage", () => ({ getActiveRole: () => mocks.stored }));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: mocks.commit,
}));
vi.mock("@/lib/hooks/useHydrationSafeClientRender", () => ({
	useHydrationSafeClientRender: () => true,
}));
vi.mock("@/components/shared/RouteLoadingSkeleton", () => ({
	default: () => React.createElement("div", { "data-skeleton": true }),
}));

import WorkspaceAlign from "@/components/shared/WorkspaceAlign";

const reload = vi.fn();
const storage = new Map<string, string>();

beforeEach(() => {
	mocks.auth.status = "unknown";
	mocks.auth.activeRole = null;
	mocks.auth.switchRole = vi.fn(() => true);
	mocks.stored = null;
	mocks.commit.mockReset();
	reload.mockReset();
	storage.clear();
	vi.stubGlobal("sessionStorage", {
		getItem: (k: string) => storage.get(k) ?? null,
		setItem: (k: string, v: string) => void storage.set(k, v),
		removeItem: (k: string) => void storage.delete(k),
	});
	Object.defineProperty(window, "location", {
		configurable: true,
		value: { ...window.location, reload },
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function mount() {
	await act(async () => {
		render(
			React.createElement(WorkspaceAlign, {
				required: "seller",
				workspace: "seller",
				children: React.createElement("p", null, "zone"),
			}),
		);
	});
}

describe("WorkspaceAlign acts only on a signed-in session", () => {
	it.each([
		"unknown",
		"unavailable",
		"guest",
	])("%s: no switch, no reload, no navigation", async (status) => {
		mocks.auth.status = status;
		await mount();
		expect(mocks.auth.switchRole).not.toHaveBeenCalled();
		expect(reload).not.toHaveBeenCalled();
		expect(mocks.commit).not.toHaveBeenCalled();
	});

	it("signedIn dual-role account in the buyer workspace on a seller zone switches and reloads", async () => {
		mocks.auth.status = "signedIn";
		mocks.auth.activeRole = "buyer";
		mocks.stored = "buyer";
		await mount();
		expect(mocks.auth.switchRole).toHaveBeenCalledWith("seller");
		expect(reload).toHaveBeenCalledTimes(1);
		expect(mocks.commit).not.toHaveBeenCalled();
	});

	it("signedIn after the switch was already tried goes to its workspace's default page", async () => {
		mocks.auth.status = "signedIn";
		mocks.auth.activeRole = "buyer";
		mocks.stored = "buyer";
		storage.set("hkui:role-zone:auto-switched", "1");
		await mount();
		expect(reload).not.toHaveBeenCalled();
		expect(mocks.commit).toHaveBeenCalledWith("/", true);
	});

	it("signedIn in the matching workspace renders the zone", async () => {
		mocks.auth.status = "signedIn";
		mocks.auth.activeRole = "seller";
		mocks.stored = "seller";
		await mount();
		expect(document.querySelector("[data-hk-role-zone-ready]")).not.toBeNull();
		expect(mocks.commit).not.toHaveBeenCalled();
	});
});
