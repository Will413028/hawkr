// @vitest-environment jsdom

import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { commonPolicyCategory } from "@hawkr/ui-text/keys";
import { cleanup, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
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

import { optionalText, uiTextFor } from "@/lib/i18n/ui-text";
import { messagesFor } from "@/lib/intl/config";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import AccountRestrictedSection from "./AccountRestrictedSection";

const ut = uiTextFor("zh-TW");
const CATEGORY_TEXT = optionalText(
	ut,
	commonPolicyCategory(PolicyCategory.HARASSMENT),
) as string;
const SENTENCE = "停權期間可以瀏覽，無法下單、留言、按讚、追蹤或修改資料。";

function show(suspension?: Identity["suspension"]) {
	mocks.auth = {
		status: "signedIn",
		identity: {
			accountId: "7",
			isSeller: false,
			...(suspension ? { suspension } : {}),
		},
	};
	return render(React.createElement(AccountRestrictedSection), {
		wrapper: IntlTestWrapper,
	});
}

afterEach(() => {
	cleanup();
	mocks.auth = { status: "unknown", identity: null };
});

describe("AccountRestrictedSection", () => {
	it("lists the date, category, reason and the restriction sentence", () => {
		show({
			suspendedAt: "2026-11-01T00:30:00.000Z",
			category: PolicyCategory.HARASSMENT,
			reason: "repeated abuse",
		});
		expect(CATEGORY_TEXT).toBeTruthy();
		expect(screen.getByRole("heading", { name: "帳號受限" })).toBeTruthy();
		expect(screen.getByText("2026/11/01")).toBeTruthy();
		expect(screen.getByText(CATEGORY_TEXT)).toBeTruthy();
		expect(screen.getByText("repeated abuse")).toBeTruthy();
		expect(screen.getByText(SENTENCE)).toBeTruthy();
	});

	it("renders nothing for an account without suspension", () => {
		show();
		expect(screen.queryByText("帳號受限")).toBeNull();
		expect(screen.queryByText(SENTENCE)).toBeNull();
	});

	it("UNSPECIFIED category and an empty reason are not listed", () => {
		show({
			suspendedAt: "2026-11-01T00:30:00.000Z",
			category: PolicyCategory.UNSPECIFIED,
			reason: "",
		});
		expect(screen.queryByText("類別")).toBeNull();
		expect(screen.queryByText("說明")).toBeNull();
		expect(screen.getByText("停權日期")).toBeTruthy();
		expect(screen.getByText(SENTENCE)).toBeTruthy();
	});

	it("shows the English wording under the en locale", () => {
		mocks.auth = {
			status: "signedIn",
			identity: {
				accountId: "7",
				isSeller: false,
				suspension: {
					suspendedAt: "2026-11-01T00:00:00.000Z",
					category: PolicyCategory.UNSPECIFIED,
					reason: "repeated abuse",
				},
			},
		};
		render(React.createElement(AccountRestrictedSection), {
			wrapper: ({ children }: { children: React.ReactNode }) =>
				React.createElement(NextIntlClientProvider, {
					locale: "en",
					messages: messagesFor("en"),
					children,
				}),
		});
		expect(screen.getByRole("heading").textContent).toBe("Account restricted");
		expect(screen.getByText("Suspension date")).toBeTruthy();
		expect(screen.getByText("Explanation")).toBeTruthy();
		expect(screen.queryByText("Category")).toBeNull();
		expect(
			screen.getByText(
				"While suspended you can browse, but you cannot place orders, comment, like, follow or edit your data.",
			),
		).toBeTruthy();
	});
});
