import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

type RevalidationMessage = {
	MessageBody: {
		host: string;
		url: string;
		lastModified: number;
	};
	MessageDeduplicationId: string;
};

type FailedState = {
	msg: RevalidationMessage;
	retryCount: number;
	nextAlarmMs: number;
};

class MemorySql {
	readonly statements: Array<{ query: string; params: unknown[] }> = [];
	readonly failedRows = new Map<
		string,
		{ id: string; data: string; buildId: string }
	>();

	constructor(failedState: FailedState) {
		this.failedRows.set(failedState.msg.MessageDeduplicationId, {
			id: failedState.msg.MessageDeduplicationId,
			data: JSON.stringify(failedState),
			buildId: "test-build",
		});
	}

	exec(query: string, ...params: unknown[]) {
		this.statements.push({ query, params });

		if (query === "SELECT * FROM failed_state") {
			return [...this.failedRows.values()];
		}

		if (query === "DELETE FROM failed_state WHERE id = ?") {
			this.failedRows.delete(String(params[0]));
		}

		return { toArray: () => [] };
	}
}

async function loadQueueHandler() {
	let source = await readFile(
		new URL(
			"../node_modules/@opennextjs/cloudflare/dist/api/durable-objects/queue.js",
			import.meta.url,
		),
		"utf8",
	);

	const replaceImport = (pattern: RegExp, replacement: string) => {
		const nextSource = source.replace(pattern, replacement);
		assert.notEqual(
			nextSource,
			source,
			`OpenNext queue import not found: ${pattern}`,
		);
		source = nextSource;
	};

	replaceImport(
		/import \{ debug, error, warn \} from "@opennextjs\/aws\/adapters\/logger\.js";/,
		"const debug = () => {}; const error = () => {}; const warn = () => {};",
	);
	replaceImport(
		/import \{ FatalError, IgnorableError, isOpenNextError, RecoverableError, \} from "@opennextjs\/aws\/utils\/error\.js";/,
		`class OpenNextError extends Error {}
		 class FatalError extends OpenNextError {}
		 class IgnorableError extends OpenNextError {}
		 class RecoverableError extends OpenNextError {}
		 const isOpenNextError = (error) => error instanceof OpenNextError;`,
	);
	replaceImport(
		/import \{ DurableObject \} from "cloudflare:workers";/,
		"class DurableObject { constructor(ctx, env) { this.ctx = ctx; this.env = env; } }",
	);

	const moduleUrl = `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
	return import(moduleUrl) as Promise<{
		DOQueueHandler: new (
			ctx: Record<string, unknown>,
			env: Record<string, unknown>,
		) => {
			executeRevalidation(message: RevalidationMessage): Promise<void>;
			addToFailedState(message: RevalidationMessage): Promise<void>;
		};
	}>;
}

async function createQueueHandler(status: number, retryCount = 2) {
	const msg: RevalidationMessage = {
		MessageBody: {
			host: "example.test",
			url: "/stale-page",
			lastModified: Date.now(),
		},
		MessageDeduplicationId: "revalidation-1",
	};
	const sql = new MemorySql({
		msg,
		retryCount,
		nextAlarmMs: Date.now() - 1,
	});
	let initialized = Promise.resolve();
	const ctx = {
		storage: {
			sql,
			getAlarm: async () => null,
			setAlarm: async () => undefined,
		},
		blockConcurrencyWhile(callback: () => Promise<void>) {
			initialized = callback();
			return initialized;
		},
		waitUntil: () => undefined,
	};
	const env = {
		WORKER_SELF_REFERENCE: {
			fetch: async () => new Response(null, { status }),
		},
	};
	const { DOQueueHandler } = await loadQueueHandler();
	const handler = new DOQueueHandler(ctx, env);
	await initialized;

	return { handler, msg, sql };
}

function assertFailedStateDeleted(sql: MemorySql, id: string) {
	assert.equal(sql.failedRows.has(id), false);
	assert.ok(
		sql.statements.some(
			({ query, params }) =>
				query === "DELETE FROM failed_state WHERE id = ?" && params[0] === id,
		),
		"the Durable Object must delete the persisted failed_state row",
	);
}

test("OpenNext 完成 revalidation 時同步刪除 SQLite failed_state", async () => {
	process.env.__OPEN_NEXT_BUILD_ID = "test-build";
	process.env.__NEXT_PREVIEW_MODE_ID = "test-preview";
	const { handler, msg, sql } = await createQueueHandler(200);

	await handler.executeRevalidation(msg);

	assertFailedStateDeleted(sql, msg.MessageDeduplicationId);
});

test("OpenNext 放棄已刪除的 404 route 時同步刪除 SQLite failed_state", async () => {
	const { handler, msg, sql } = await createQueueHandler(404);

	await handler.executeRevalidation(msg);

	assertFailedStateDeleted(sql, msg.MessageDeduplicationId);
});

test("OpenNext 超過最大重試次數時同步刪除 SQLite failed_state", async () => {
	const { handler, msg, sql } = await createQueueHandler(500, 6);

	await handler.addToFailedState(msg);

	assertFailedStateDeleted(sql, msg.MessageDeduplicationId);
});
