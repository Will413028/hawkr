/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({ recordPostView: vi.fn() }));
vi.mock("@/lib/posts/repo", () => repo);

import PostViewTracker from "./PostViewTracker";

afterEach(() => {
	cleanup();
	repo.recordPostView.mockReset();
});

describe("PostViewTracker", () => {
	it("records one view for the opened post", () => {
		render(React.createElement(PostViewTracker, { postId: "p-1" }));
		expect(repo.recordPostView).toHaveBeenCalledExactlyOnceWith("p-1");
	});

	it("records again when the viewer moves to another post", () => {
		const view = render(
			React.createElement(PostViewTracker, { postId: "p-1" }),
		);
		view.rerender(React.createElement(PostViewTracker, { postId: "p-2" }));
		expect(repo.recordPostView.mock.calls.map(([id]) => id)).toEqual([
			"p-1",
			"p-2",
		]);
	});
});
