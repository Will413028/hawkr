import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
} from "@connectrpc/connect";
import { BuyerAppService } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { authExpiredInterceptor } from "../connect/transport";

const emit = vi.hoisted(() => vi.fn());
vi.mock("../connect/buyer", async () => {
	const transport = createRouterTransport(
		(router) =>
			router.service(BuyerAppService, {
				getProfile() {
					throw new ConnectError("guest", Code.Unauthenticated);
				},
			}),
		{ transport: { interceptors: [authExpiredInterceptor(emit)] } },
	);
	return { buyerAppClient: () => createClient(BuyerAppService, transport) };
});

import { fetchProfile } from "./repo";

describe("fetchProfile auth-expired broadcast", () => {
	beforeEach(() => emit.mockReset());

	it("does not broadcast AUTH_EXPIRED when the caller suppresses it", async () => {
		await expect(
			fetchProfile({ suppressAuthExpired: true }),
		).rejects.toMatchObject({ code: Code.Unauthenticated });
		expect(emit).not.toHaveBeenCalled();
	});

	it("broadcasts AUTH_EXPIRED by default", async () => {
		await expect(fetchProfile()).rejects.toMatchObject({
			code: Code.Unauthenticated,
		});
		expect(emit).toHaveBeenCalledOnce();
	});
});
