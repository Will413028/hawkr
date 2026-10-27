import assert from "node:assert/strict";
import { test } from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import {
	AgreementKind,
	type Store,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	finalizeSellerOnboarding,
	type PersistSellerOnboardingDeps,
	persistSellerOnboarding,
} from "../lib/seller/onboarding";
import type { ApplyStoreInput, StoreUpdate } from "../lib/seller/repo";

const submitted: ApplyStoreInput = {
	storeName: "山漾食品",
	description: "山裡的好味道",
	contactPhone: "0911111111",
	address: { townCode: "66000290", addressDetail: "東關路一段100號" },
	acceptedAgreements: [
		{ kind: AgreementKind.SELLER_TERMS, version: "2026-10-16" },
	],
};

function store(etag: string, extra: Partial<Store> = {}): Store {
	return { etag, id: "1", ...extra } as Store;
}

function depsWith(overrides: Partial<PersistSellerOnboardingDeps> = {}) {
	const calls = {
		applied: [] as ApplyStoreInput[],
		updated: [] as StoreUpdate[],
	};
	const deps: PersistSellerOnboardingDeps = {
		applyStore: async (input) => {
			calls.applied.push(input);
			return store("created-etag");
		},
		fetchStore: async () => {
			throw new ConnectError("no store", Code.NotFound);
		},
		updateStore: async (update) => {
			calls.updated.push(update);
			return { store: store("updated-etag") };
		},
		...overrides,
	};
	return { calls, deps };
}

test("updates a store that registration already created, with its etag", async () => {
	const { calls, deps } = depsWith({
		fetchStore: async () => ({ store: store("e7", { prize: "金牌農村" }) }),
	});

	const { result, store: persisted } = await persistSellerOnboarding(
		submitted,
		deps,
	);

	assert.equal(result, "updated");
	assert.equal(calls.applied.length, 0);
	assert.deepEqual(calls.updated, [
		{
			etag: "e7",
			storeName: "山漾食品",
			description: "山裡的好味道",
			contactPhone: "0911111111",
			address: { townCode: "66000290", addressDetail: "東關路一段100號" },
		},
	]);
	// 之後標記完成要用寫入之後的版本。
	assert.equal(persisted.etag, "updated-etag");
});

test("creates a store when GetMyStore says there is none", async () => {
	const { calls, deps } = depsWith();

	const { result, store: persisted } = await persistSellerOnboarding(
		submitted,
		deps,
	);

	assert.equal(result, "created");
	assert.deepEqual(calls.applied, [submitted]);
	assert.equal(calls.updated.length, 0);
	assert.equal(persisted.etag, "created-etag");
});

test("ApplyStore already_exists is a conflict: no overwrite with a freshly read etag", async () => {
	const applyError = new ConnectError("exists", Code.AlreadyExists);
	let fetchCount = 0;
	const { calls, deps } = depsWith({
		applyStore: async () => {
			throw applyError;
		},
		fetchStore: async () => {
			fetchCount += 1;
			if (fetchCount === 1) throw new ConnectError("none", Code.NotFound);
			return { store: store("e9") };
		},
	});

	await assert.rejects(
		persistSellerOnboarding(submitted, deps),
		(error) => error === applyError,
	);
	assert.equal(calls.updated.length, 0);
});

test("only lists the fields the form filled; the rest of the store is untouched", async () => {
	const { calls, deps } = depsWith({
		fetchStore: async () => ({
			store: store("e3", {
				address: { townCode: "66000290", addressDetail: "志學村1號" },
			} as Partial<Store>),
		}),
	});

	await persistSellerOnboarding(
		{ storeName: "新店名", acceptedAgreements: [] },
		deps,
	);

	assert.deepEqual(calls.updated, [{ etag: "e3", storeName: "新店名" }]);
	const update = calls.updated[0] as StoreUpdate;
	assert.ok(
		!("address" in update),
		"沒填地址就不能出現在更新裡（會清掉既有地址）",
	);
	assert.ok(!("description" in update));
});

test("a transient failure reading the store never becomes a second ApplyStore", async () => {
	const { calls, deps } = depsWith({
		fetchStore: async () => {
			throw new ConnectError("down", Code.Unavailable);
		},
	});

	await assert.rejects(
		persistSellerOnboarding(submitted, deps),
		(error) => ConnectError.from(error).code === Code.Unavailable,
	);
	assert.equal(calls.applied.length, 0);
});

test("keeps the original apply error when it is not already_exists", async () => {
	const applyError = new ConnectError("suspended", Code.FailedPrecondition);
	const { deps } = depsWith({
		applyStore: async () => {
			throw applyError;
		},
	});

	await assert.rejects(
		persistSellerOnboarding(submitted, deps),
		(error) => error === applyError,
	);
});

test("marks onboarding complete with the etag the last write returned", async () => {
	const completed: string[] = [];
	const { deps } = depsWith();

	const result = await finalizeSellerOnboarding(submitted, {
		...deps,
		complete: async (etag) => {
			completed.push(etag);
		},
		onCompleted: () => {},
	});

	assert.equal(result, "created");
	assert.deepEqual(completed, ["created-etag"]);
});
