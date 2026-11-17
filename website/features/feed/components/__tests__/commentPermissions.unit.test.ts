import { describe, expect, it } from "vitest";
import {
	canBlockCommentAuthor,
	canComposeComment,
	canDeleteComment,
	canEditComment,
	canReplyToComment,
	storeActorName,
} from "../commentPermissions";

describe("per-comment permissions come from the server flags", () => {
	it("edit follows viewerCanEdit only", () => {
		expect(canEditComment({ viewerCanEdit: true })).toBe(true);
		expect(canEditComment({ viewerCanEdit: false })).toBe(false);
	});

	it("delete follows viewerCanDelete only", () => {
		expect(canDeleteComment({ viewerCanDelete: true })).toBe(true);
		expect(canDeleteComment({ viewerCanDelete: false })).toBe(false);
	});

	it("reply follows viewerCanReply only", () => {
		expect(canReplyToComment({ viewerCanReply: true })).toBe(true);
		expect(canReplyToComment({ viewerCanReply: false })).toBe(false);
	});

	it("a post owner may delete but not edit someone else's comment", () => {
		const othersUnderMyPost = {
			viewerCanEdit: false,
			viewerCanDelete: true,
		};
		expect(canDeleteComment(othersUnderMyPost)).toBe(true);
		expect(canEditComment(othersUnderMyPost)).toBe(false);
	});
});

describe("canBlockCommentAuthor", () => {
	it("is for another author's comment that the viewer's store may delete", () => {
		expect(
			canBlockCommentAuthor({ isMine: false, viewerCanDelete: true }),
		).toBe(true);
	});

	it("not for the viewer's own comment, even though they may delete it", () => {
		expect(canBlockCommentAuthor({ isMine: true, viewerCanDelete: true })).toBe(
			false,
		);
	});

	it("not for a comment the viewer cannot delete (it is not in their store's content)", () => {
		expect(
			canBlockCommentAuthor({ isMine: false, viewerCanDelete: false }),
		).toBe(false);
	});
});

describe("canComposeComment", () => {
	it("needs a signed-in viewer and the server's viewerCanComment", () => {
		expect(canComposeComment(true, { canComment: true })).toBe(true);
		expect(canComposeComment(true, { canComment: false })).toBe(false);
		expect(canComposeComment(false, { canComment: true })).toBe(false);
	});

	it("is closed until the server's answer is known", () => {
		expect(canComposeComment(true, undefined)).toBe(false);
	});
});

describe("storeActorName", () => {
	it("is the store name only for a store actor", () => {
		expect(
			storeActorName({
				canComment: true,
				actor: { kind: "store", id: "7", name: "小店" },
			}),
		).toBe("小店");
		expect(
			storeActorName({
				canComment: true,
				actor: { kind: "buyer", id: "9", name: "阿明" },
			}),
		).toBeUndefined();
		expect(storeActorName({ canComment: true })).toBeUndefined();
		expect(storeActorName(undefined)).toBeUndefined();
	});

	it("a store actor whose name could not be read is not announced by an empty name", () => {
		expect(
			storeActorName({
				canComment: true,
				actor: { kind: "store", id: "7" },
			}),
		).toBeUndefined();
	});
});
