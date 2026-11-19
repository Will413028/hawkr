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

import { optionalText, uiTextFor } from "@/lib/i18n/ui-text";
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
			// 台北時間 2026-11-01 08:30
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
});
