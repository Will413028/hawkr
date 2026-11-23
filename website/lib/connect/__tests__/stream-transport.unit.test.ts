import { ChatAppService } from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";

const captured: { options?: Record<string, unknown> }[] = [];

vi.mock("@connectrpc/connect-web", () => ({
	createConnectTransport: (options: Record<string, unknown>) => {
		captured.push({ options });
		return { unary: vi.fn(), stream: vi.fn() };
	},
}));

import {
	authedTransport,
	BACKGROUND_WRITE_METHODS,
	streamTransport,
} from "../transport";

beforeEach(() => {
	captured.length = 0;
});

describe("streamTransport", () => {
	it("has no default timeout (a stream lives until the server's max_lifetime)", () => {
		streamTransport({ locale: "zh-TW" });
		expect(captured[0]?.options?.defaultTimeoutMs).toBeUndefined();
	});

	it("has no retry interceptor, unlike authedTransport (reconnect is the stream client's job)", () => {
		streamTransport({ locale: "zh-TW" });
		authedTransport({ locale: "zh-TW" });
		const streamInterceptors = captured[0]?.options?.interceptors as unknown[];
		const authedInterceptors = captured[1]?.options?.interceptors as unknown[];
		expect(authedInterceptors.length).toBe(streamInterceptors.length + 1);
		expect(typeof captured[1]?.options?.defaultTimeoutMs).toBe("number");
	});
});

describe("automatic chat writes", () => {
	it("marking a conversation read is a background write (silent for a suspended account)", () => {
		expect(
			BACKGROUND_WRITE_METHODS.has(
				`${ChatAppService.typeName}/${ChatAppService.method.markConversationRead.name}`,
			),
		).toBe(true);
	});
});
