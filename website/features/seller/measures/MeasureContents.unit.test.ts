// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
	ContentKind,
	type ContentRef,
	ContentRefSchema,
	ContentSnapshotState,
	ForumThreadLinkSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/Link", () => ({
	default: ({ href, children }: { href: string; children: React.ReactNode }) =>
		React.createElement("a", { href }, children),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { contentHref, MeasureContents } from "./MeasureContents";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

afterEach(cleanup);

const AUTHORED = new Date("2026-11-02T06:30:00Z");

function ref(over: Record<string, unknown> = {}): ContentRef {
	return create(ContentRefSchema, {
		kind: ContentKind.POST,
		id: "c1",
		snapshotState: ContentSnapshotState.CAPTURED,
		excerpt: "摘要文字",
		authoredTime: timestampFromDate(AUTHORED),
		linkAvailable: true,
		link: { case: "postId", value: "p1" },
		...over,
	});
}

describe("contentHref", () => {
	it("貼文連到貼文頁，討論區留言連到單一討論串頁", () => {
		expect(contentHref(ref())).toBe("/seller-center/posts?post=p1");
		expect(
			contentHref(
				ref({
					kind: ContentKind.FORUM_ENTRY,
					link: {
						case: "forumThread",
						value: create(ForumThreadLinkSchema, {
							storeId: "7",
							threadId: "t-9",
						}),
					},
				}),
			),
		).toBe("/sellers/7/messages/t-9");
	});

	it("link_available 為 false 或沒有連結目標時沒有連結", () => {
		expect(contentHref(ref({ linkAvailable: false }))).toBeUndefined();
		expect(contentHref(ref({ link: { case: undefined } }))).toBeUndefined();
	});
});

describe("MeasureContents", () => {
	it("沒有內容時不顯示區塊", () => {
		const { container } = render(
			React.createElement(MeasureContents, { contents: [] }),
		);
		expect(container.textContent).toBe("");
	});

	it("逐則列出標題、種類、摘要、台北發文時間與查看貼文連結", () => {
		render(
			React.createElement(MeasureContents, {
				contents: [
					ref(),
					ref({
						id: "c2",
						kind: ContentKind.COMMENT,
						excerpt: "另一則留言",
						link: { case: "postId", value: "p2" },
					}),
				],
			}),
		);
		expect(screen.getByText("下架的內容")).toBeTruthy();
		expect(screen.getByText("貼文")).toBeTruthy();
		expect(screen.getByText("留言")).toBeTruthy();
		expect(screen.getByText("摘要文字")).toBeTruthy();
		expect(screen.getByText("另一則留言")).toBeTruthy();
		expect(screen.getAllByText("2026/11/02 14:30")).toHaveLength(2);
		expect(screen.queryByText("內容副本已超過保留期限並刪除。")).toBeNull();
		expect(
			screen
				.getAllByText("查看貼文")
				.map((a) => a.closest("a")?.getAttribute("href")),
		).toEqual(["/seller-center/posts?post=p1", "/seller-center/posts?post=p2"]);
	});

	it("目標已刪除（link_available 為 false）的項目不顯示連結，其餘照列", () => {
		render(
			React.createElement(MeasureContents, {
				contents: [ref({ linkAvailable: false })],
			}),
		);
		expect(screen.getByText("摘要文字")).toBeTruthy();
		expect(screen.queryByText("查看貼文")).toBeNull();
	});

	it("快照不在（CLEARED）只列種類，不顯示摘要、時間與連結", () => {
		render(
			React.createElement(MeasureContents, {
				contents: [
					ref({
						snapshotState: ContentSnapshotState.CLEARED,
						excerpt: "",
						authoredTime: undefined,
						linkAvailable: true,
					}),
				],
			}),
		);
		expect(screen.getByText("貼文")).toBeTruthy();
		expect(screen.getByText("內容副本已超過保留期限並刪除。")).toBeTruthy();
		expect(screen.queryByText("查看貼文")).toBeNull();
		expect(document.querySelector("time")).toBeNull();
	});
});
