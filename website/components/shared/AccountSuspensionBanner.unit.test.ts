// @vitest-environment jsdom

import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Identity } from "@/lib/auth/storage";

const mocks = vi.hoisted(() => ({
	auth: { status: "unknown", identity: null } as {
		status: string;
		identity: unknown;
	},
}));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => mocks.auth }));
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

import { optionalText, uiTextFor } from "@/lib/i18n/ui-text";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import AccountSuspensionBanner from "./AccountSuspensionBanner";

const ut = uiTextFor("zh-TW");
const CATEGORY_TEXT = optionalText(
	ut,
	commonPolicyCategory(PolicyCategory.HARASSMENT),
) as string;

function identity(suspension?: Identity["suspension"]): Identity {
	return {
		accountId: "7",
		isSeller: false,
		...(suspension ? { suspension } : {}),
	};
}
function show(auth: { status: string; identity: unknown }) {
	mocks.auth = auth;
	return render(React.createElement(AccountSuspensionBanner), {
		wrapper: IntlTestWrapper,
	});
}

afterEach(() => {
	cleanup();
	mocks.auth = { status: "unknown", identity: null };
});

describe("AccountSuspensionBanner", () => {
	it("a suspended signed-in account sees the banner with category, reason and the settings link", () => {
		show({
			status: "signedIn",
			identity: identity({
				suspendedAt: "2026-11-01T00:00:00.000Z",
				category: PolicyCategory.HARASSMENT,
				reason: "repeated abuse",
			}),
		});
		expect(CATEGORY_TEXT).toBeTruthy();
		const banner = screen.getByRole("status");
		expect(banner.textContent).toContain("帳號已停權，部分功能受限");
		expect(banner.textContent).toContain(CATEGORY_TEXT);
		expect(banner.textContent).toContain("repeated abuse");
		expect(
			screen.getByRole("link", { name: "查看詳情" }).getAttribute("href"),
		).toBe("/account/settings");
	});

	it("shows nothing for a signed-in account without suspension", () => {
		show({ status: "signedIn", identity: identity() });
		expect(screen.queryByRole("status")).toBeNull();
	});

	it.each(["guest", "unknown", "unavailable"])(
		"shows nothing while %s",
		(status) => {
			show({ status, identity: null });
			expect(screen.queryByRole("status")).toBeNull();
		},
	);

	it("UNSPECIFIED category and an empty reason add no text", () => {
		show({
			status: "signedIn",
			identity: identity({
				category: PolicyCategory.UNSPECIFIED,
				reason: "",
			}),
		});
		const banner = screen.getByRole("status");
		expect(banner.querySelectorAll("span")).toHaveLength(1);
		expect(banner.textContent).toBe("帳號已停權，部分功能受限查看詳情");
	});
});
