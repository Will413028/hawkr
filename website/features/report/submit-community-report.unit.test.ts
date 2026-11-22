import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import { describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({ submitReport: vi.fn() }));
vi.mock("@connectrpc/connect", async (importOriginal) => ({
	...(await importOriginal<typeof import("@connectrpc/connect")>()),
	createClient: () => rpc,
}));
vi.mock("@/lib/connect/transport", () => ({ authedTransport: () => ({}) }));

import { submitCommunityReport } from "./api";

describe("submitCommunityReport", () => {
	it.each([
		["review", ReportTargetKind.REVIEW],
		["chatMessage", ReportTargetKind.CHAT_MESSAGE],
	] as const)(
		"%s maps to the proto target kind and carries the request id",
		async (kind, proto) => {
			rpc.submitReport.mockResolvedValue({ reportId: "r" });
			await submitCommunityReport({
				target: { kind, id: "x1" },
				category: PolicyCategory.SPAM,
				description: "d",
				requestId: "u1",
			});
			expect(rpc.submitReport).toHaveBeenCalledWith({
				target: { kind: proto, id: "x1" },
				category: PolicyCategory.SPAM,
				description: "d",
				requestId: "u1",
			});
		},
	);
});
