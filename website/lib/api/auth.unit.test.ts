import { beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({
	getSession: vi.fn(),
	deleteAccount: vi.fn(),
}));
const store = vi.hoisted(() => ({ fetchMyStore: vi.fn() }));
vi.mock("@connectrpc/connect", async (importOriginal) => ({
	...(await importOriginal<typeof import("@connectrpc/connect")>()),
	createClient: () => rpc,
}));
vi.mock("@/lib/connect/transport", () => ({
	authedTransport: () => ({}),
	SUPPRESS_AUTH_EXPIRED: { id: Symbol("suppress"), defaultValue: false },
}));
vi.mock("@clerk/nextjs/errors", () => ({
	isClerkAPIResponseError: () => false,
}));
vi.mock("@/lib/analytics/events", () => ({ trackLogin: vi.fn() }));
vi.mock("../seller/repo", () => ({ fetchMyStore: store.fetchMyStore }));

import { Code, ConnectError } from "@connectrpc/connect";
import {
	DeleteAccountResponse_DeletionState,
	Role,
} from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { completeSignIn, deleteAccount, fetchSession } from "./auth";

// 契約（identity.proto）：每個帳號都是買家，roles 只列另外申請的 ROLE_SELLER；工作區是 client 的狀態，GetSession 不回。
describe("GetSession → Identity", () => {
	beforeEach(() => rpc.getSession.mockReset());

	it("an account without roles is a buyer", async () => {
		rpc.getSession.mockResolvedValue({ account: { id: "7", roles: [] } });
		expect(await fetchSession()).toEqual({ accountId: "7", isSeller: false });
	});

	it("a seller is also a buyer", async () => {
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		expect(await fetchSession()).toEqual({ accountId: "8", isSeller: true });
	});
});

describe("completeSignIn: 開店是否完成由 GetMyStore 判斷", () => {
	beforeEach(() => {
		rpc.getSession.mockReset();
		store.fetchMyStore.mockReset();
	});

	it("沒有賣家角色：需要開店，不查店家", async () => {
		rpc.getSession.mockResolvedValue({ account: { id: "7", roles: [] } });
		expect(await completeSignIn(null)).toMatchObject({ needsOnboarding: true });
		expect(store.fetchMyStore).not.toHaveBeenCalled();
	});

	it("店家完成新手引導：不需要開店", async () => {
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		store.fetchMyStore.mockResolvedValue({
			store: { id: "1", onboardingCompleted: true },
		});
		expect(await completeSignIn(null)).toMatchObject({
			needsOnboarding: false,
		});
	});

	it("店家還沒完成新手引導：需要開店", async () => {
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		store.fetchMyStore.mockResolvedValue({
			store: { id: "1", onboardingCompleted: false },
		});
		expect(await completeSignIn(null)).toMatchObject({ needsOnboarding: true });
	});

	it("GetMyStore 回 not_found：需要開店", async () => {
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		store.fetchMyStore.mockRejectedValue(
			new ConnectError("no store", Code.NotFound),
		);
		expect(await completeSignIn(null)).toMatchObject({ needsOnboarding: true });
	});

	it("暫時失敗是「不知道」，不當成已完成也不當成沒店家", async () => {
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		store.fetchMyStore.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		expect((await completeSignIn(null)).needsOnboarding).toBeUndefined();
	});
});

describe("deleteAccount", () => {
	beforeEach(() => {
		rpc.deleteAccount.mockReset();
	});

	it("回傳刪除後的狀態", async () => {
		rpc.deleteAccount.mockResolvedValue({
			state: DeleteAccountResponse_DeletionState.PENDING_ORDERS,
		});
		expect(await deleteAccount()).toBe(
			DeleteAccountResponse_DeletionState.PENDING_ORDERS,
		);
	});

	it("failed_precondition 的錯誤原樣丟出，由畫面顯示", async () => {
		rpc.deleteAccount.mockRejectedValue(
			new ConnectError("has store", Code.FailedPrecondition),
		);
		await expect(deleteAccount()).rejects.toMatchObject({
			code: Code.FailedPrecondition,
		});
	});

	it("沒收到回應時以 GetSession 確認：unauthenticated 表示已刪除", async () => {
		rpc.getSession.mockReset();
		rpc.deleteAccount.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		rpc.getSession.mockRejectedValue(
			new ConnectError("gone", Code.Unauthenticated),
		);
		expect(await deleteAccount()).toBe(
			DeleteAccountResponse_DeletionState.DELETED,
		);
		expect(rpc.deleteAccount).toHaveBeenCalledOnce();
	});

	it("沒收到回應而且仍在登入：原本的錯誤丟出，不自動重送", async () => {
		rpc.getSession.mockReset();
		rpc.deleteAccount.mockRejectedValue(
			new ConnectError("down", Code.Unavailable),
		);
		rpc.getSession.mockResolvedValue({ account: { id: "7", roles: [] } });
		await expect(deleteAccount()).rejects.toMatchObject({
			code: Code.Unavailable,
		});
		expect(rpc.deleteAccount).toHaveBeenCalledOnce();
	});

	it("有收到回應的錯誤不查 GetSession", async () => {
		rpc.getSession.mockReset();
		rpc.deleteAccount.mockRejectedValue(
			new ConnectError("has store", Code.FailedPrecondition),
		);
		await expect(deleteAccount()).rejects.toBeTruthy();
		expect(rpc.getSession).not.toHaveBeenCalled();
	});
});
