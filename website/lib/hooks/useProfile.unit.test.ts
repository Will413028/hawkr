/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const repo = vi.hoisted(() => ({
	fetchProfile: vi.fn(async () => ({ nickname: "n" })),
	updateProfile: vi.fn(async () => {}),
	updateBuyerAvatar: vi.fn(async () => {}),
}));
vi.mock("@/lib/profile/repo", () => repo);
const router = vi.hoisted(() => ({ refresh: vi.fn() }));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		status: "signedIn",
		activeRole: "buyer",
	}),
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "42", activeRole: "buyer" }),
}));

import { useProfile } from "./useAccount";

afterEach(() => {
	cleanup();
	router.refresh.mockReset();
	repo.updateProfile.mockClear();
	repo.updateBuyerAvatar.mockClear();
});

function mount() {
	const client = new QueryClient();
	return renderHook(() => useProfile(), {
		wrapper: ({ children }: { children: React.ReactNode }) =>
			React.createElement(QueryClientProvider, { client }, children),
	});
}

describe("useProfile asks the server for the header's nickname and avatar after every profile change", () => {
	it("after saving the profile fields", async () => {
		const { result } = mount();
		await waitFor(() => expect(result.current.profile).not.toBeNull());
		await act(async () => {
			await result.current.save({ nickname: "m" } as never);
		});
		expect(repo.updateProfile).toHaveBeenCalledOnce();
		expect(router.refresh).toHaveBeenCalledTimes(1);
	});

	it("after changing the avatar, before and regardless of any profile refetch", async () => {
		const { result } = mount();
		await waitFor(() => expect(result.current.profile).not.toBeNull());
		await act(async () => {
			await result.current.updateAvatar("tok");
		});
		expect(repo.updateBuyerAvatar).toHaveBeenCalledWith("tok", {
			nickname: "n",
		});
		expect(router.refresh).toHaveBeenCalledTimes(1);
	});

	it("not when the change failed", async () => {
		repo.updateBuyerAvatar.mockRejectedValueOnce(new Error("no"));
		const { result } = mount();
		await waitFor(() => expect(result.current.profile).not.toBeNull());
		await act(async () => {
			await result.current.updateAvatar("tok").catch(() => {});
		});
		expect(router.refresh).not.toHaveBeenCalled();
	});
});
