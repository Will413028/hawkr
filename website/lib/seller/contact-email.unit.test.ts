import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { describe, expect, it } from "vitest";
import {
	isContactEmailRequired,
	normalizeContactEmail,
	validateContactEmail,
} from "./contact-email";

describe("validateContactEmail", () => {
	it("接受 local@domain（網域要有點），去空白轉小寫", () => {
		expect(validateContactEmail(" Pay@Example.com ")).toBeNull();
		expect(normalizeContactEmail(" Pay@Example.com ")).toBe("pay@example.com");
	});

	it("空、沒有網域點、含空白都不行；上限 254 字", () => {
		expect(validateContactEmail("  ")).not.toBeNull();
		expect(validateContactEmail("a@b")).not.toBeNull();
		expect(validateContactEmail("a b@c.com")).not.toBeNull();
		expect(validateContactEmail(`${"a".repeat(246)}@b.com`)).toBeNull();
		expect(validateContactEmail(`${"a".repeat(250)}@b.com`)).not.toBeNull();
	});
});

describe("isContactEmailRequired", () => {
	const err = (code: Code, reason: ErrorReason) =>
		new ConnectError("x", code, undefined, [
			{
				desc: ErrorInfoSchema,
				value: { reason: ErrorReason[reason], domain: "hawkr" },
			},
		]);

	it("failed_precondition 加 CONTACT_EMAIL_REQUIRED 才算", () => {
		expect(
			isContactEmailRequired(
				err(Code.FailedPrecondition, ErrorReason.CONTACT_EMAIL_REQUIRED),
			),
		).toBe(true);
		expect(
			isContactEmailRequired(
				err(Code.FailedPrecondition, ErrorReason.AGREEMENT_OUTDATED),
			),
		).toBe(false);
		expect(isContactEmailRequired(new ConnectError("x", Code.Internal))).toBe(
			false,
		);
	});
});
