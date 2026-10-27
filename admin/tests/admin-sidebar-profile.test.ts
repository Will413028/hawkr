import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { Code, ConnectError, createRouterTransport } from "@connectrpc/connect";
import {
	createQueryOptions,
	TransportProvider,
} from "@connectrpc/connect-query";
import {
	type GetSessionResponseSchema,
	IdentityAdminService,
} from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { QueryClientProvider } from "@tanstack/react-query";
import { PathnameContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import Sidebar from "../src/components/Layout/Sidebar";
import { createAppQueryClient } from "../src/lib/query/client";

/**
 * Sidebar 的管理員資料是伺服端狀態：以 connect-query 讀 IdentityAdminService.GetSession，放在 query cache，
 * 不寫進 localStorage。頭像顯示 email 第一個字，名稱顯示 email；角色固定是「平台管理員」。
 * 載入中與讀取失敗時顯示預設值。
 */

type Session = MessageInitShape<typeof GetSessionResponseSchema>;

/**
 * 後端的 GetSession：回一個 session、失敗（down），或還沒回應（pending，server render 不會發出請求）。
 * 先以同一個 transport 讓 query cache 走完 GetSession，再渲染 Sidebar；Sidebar 的 query key 若不是這個方法與
 * 請求，就讀不到 cache 裡的結果而停在預設值。
 */
async function renderSidebar(backend: (() => Session) | "down" | "pending") {
	let calls = 0;
	const transport = createRouterTransport(({ service }) => {
		service(IdentityAdminService, {
			getSession: () => {
				calls++;
				if (typeof backend === "function") return backend();
				throw new ConnectError("backend down", Code.Unavailable);
			},
		});
	});
	const queryClient = createAppQueryClient();
	const options = createQueryOptions(
		IdentityAdminService.method.getSession,
		{},
		{ transport },
	);
	if (backend !== "pending") await queryClient.prefetchQuery(options);
	const status = queryClient.getQueryState(options.queryKey)?.status;

	const html = renderToStaticMarkup(
		createElement(
			QueryClientProvider,
			{ client: queryClient },
			createElement(
				TransportProvider,
				{ transport },
				createElement(
					PathnameContext.Provider,
					{ value: "/admin" },
					createElement(Sidebar, {
						open: false,
						mobile: false,
						onClose: () => {},
					}),
				),
			),
		),
	);
	const avatar = html.match(/<div class="user-avatar">([\s\S]*?)<\/div>/u)?.[1];
	const name = html.match(/<div class="user-name">([^<]*)<\/div>/u)?.[1];
	const role = html.match(/<div class="user-role">([^<]*)<\/div>/u)?.[1];
	return { calls, status, profile: { avatar, name, role } };
}

const DEFAULTS = { avatar: "A", name: "管理員", role: "平台管理員" };

test("Access email 顯示在側欄，頭像使用第一個字", async () => {
	const { calls, profile } = await renderSidebar(() => ({
		admin: {
			id: "42",
			email: "ops@example.test",
		},
	}));

	assert.equal(calls, 1);
	assert.deepEqual(profile, {
		avatar: "O",
		name: "ops@example.test",
		role: "平台管理員",
	});
});

test("沒有 email 時顯示預設值", async () => {
	const { profile } = await renderSidebar(() => ({
		admin: { id: "42" },
	}));

	assert.deepEqual(profile, DEFAULTS);
});

test("載入中：顯示預設值", async () => {
	const { calls, status, profile } = await renderSidebar("pending");

	assert.equal(calls, 0);
	assert.equal(status, undefined);
	assert.deepEqual(profile, DEFAULTS);
});

test("讀取失敗：顯示預設值", async () => {
	const { calls, status, profile } = await renderSidebar("down");

	assert.equal(calls, 1);
	assert.equal(status, "error");
	assert.deepEqual(profile, DEFAULTS);
});

test("顯示用的管理員資料不經 localStorage", async () => {
	const [sidebar, auth] = await Promise.all([
		readFile("src/components/Layout/Sidebar.tsx", "utf8"),
		readFile("src/services/auth.ts", "utf8"),
	]);
	for (const source of [sidebar, auth]) {
		assert.doesNotMatch(source, /localStorage|hawkr_admin_user/u);
	}
});
