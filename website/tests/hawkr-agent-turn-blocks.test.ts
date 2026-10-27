import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (rel: string) => readFileSync(root + rel, "utf8");

test("#2549 Phase 2：ProcurementChatTurn 型別帶 blocks 欄位", () => {
	const types = read("features/procurement-assistant/types.ts");
	assert.match(types, /blocks\?:\s*PublicNarrativeBlock\[\]/);
});

test("#2549 Phase 2：applyTurnResult 把 data.blocks 帶進 assistantTurn", () => {
	const hook = read("features/procurement-assistant/useProcurementSession.ts");
	assert.match(hook, /blocks:\s*data\.blocks/);
});
