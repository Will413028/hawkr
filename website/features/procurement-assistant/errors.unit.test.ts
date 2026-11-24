import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { describe, expect, it } from "vitest";
import { procurementFailureFrom } from "./errors";

const defaults = (code: Code) => `default:${Code[code]}`;

function failure(
	code: Code,
	opts: {
		reason?: string;
		metadata?: Record<string, string>;
		message?: string;
	} = {},
) {
	const details = [];
	if (opts.reason) {
		details.push({
			desc: ErrorInfoSchema,
			value: create(ErrorInfoSchema, {
				domain: "hawkr",
				reason: opts.reason,
				metadata: opts.metadata ?? {},
			}),
		});
	}
	if (opts.message) {
		details.push({
			desc: LocalizedMessageSchema,
			value: create(LocalizedMessageSchema, {
				locale: "zh-TW",
				message: opts.message,
			}),
		});
	}
	return new ConnectError("x", code, undefined, details);
}

describe("procurementFailureFrom", () => {
	it("版本衝突依 conflictTarget 分成需求版本、候選集與核准", () => {
		const of = (target: string) =>
			procurementFailureFrom(
				failure(Code.Aborted, {
					reason: "VERSION_CONFLICT",
					metadata: { conflictTarget: target },
				}),
				defaults,
			).code;
		expect(of("request_revision")).toBe("STALE_REVISION");
		expect(of("candidate_set")).toBe("STALE_CANDIDATE_SET");
		expect(of("publication_approval")).toBe("STALE_APPROVAL");
		expect(of("seller_response")).toBe("STALE_SELLER_RESPONSE");
	});

	it("文字取伺服器的 LocalizedMessage，沒有時是該 code 的預設文字", () => {
		expect(
			procurementFailureFrom(
				failure(Code.FailedPrecondition, {
					reason: "REQUIREMENT_NOT_READY",
					message: "需求還沒齊",
				}),
				defaults,
			),
		).toMatchObject({ code: "REQUIREMENT_NOT_READY", detail: "需求還沒齊" });
		expect(
			procurementFailureFrom(new ConnectError("x", Code.NotFound), defaults),
		).toMatchObject({
			code: "CONVERSATION_GONE",
			detail: "default:NotFound",
		});
	});

	it("既有訂單編號只在 RFQ_CHECKOUT_ALREADY_ACTIVE 帶出", () => {
		const f = procurementFailureFrom(
			failure(Code.FailedPrecondition, {
				reason: "RFQ_CHECKOUT_ALREADY_ACTIVE",
				metadata: { orderNo: "HK1" },
			}),
			defaults,
		);
		expect(f).toMatchObject({
			code: "RFQ_CHECKOUT_ALREADY_ACTIVE",
			orderNo: "HK1",
			retryable: false,
		});
	});

	it("沒有收到回應與 unavailable 才可原操作重試", () => {
		const net = new ConnectError(
			"x",
			Code.Unknown,
			undefined,
			undefined,
			new TypeError("fetch"),
		);
		expect(procurementFailureFrom(net, defaults)).toMatchObject({
			code: "NETWORK_ERROR",
			retryable: true,
		});
		expect(
			procurementFailureFrom(new ConnectError("x", Code.Unavailable), defaults)
				.retryable,
		).toBe(true);
		expect(
			procurementFailureFrom(
				new ConnectError("x", Code.InvalidArgument),
				defaults,
			).retryable,
		).toBe(false);
	});
});
