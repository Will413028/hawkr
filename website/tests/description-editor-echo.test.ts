import assert from "node:assert/strict";
import test from "node:test";
import { DescriptionEditorEcho } from "../lib/products/descriptionEditorEcho";

test("two commits before the first echo arrives are both echoes", () => {
	const echo = new DescriptionEditorEcho("s0");
	echo.emit("s1");
	echo.emit("s2");
	assert.equal(echo.receive("s1"), "echo");
	assert.equal(echo.receive("s2"), "echo");
	assert.equal(echo.isCurrent("s2"), true);
});

test("a batched parent that only delivers the latest value is an echo", () => {
	const echo = new DescriptionEditorEcho("s0");
	echo.emit("s1");
	echo.emit("s2");
	assert.equal(echo.receive("s2"), "echo");
});

test("an external value after the echoes is external and becomes current", () => {
	const echo = new DescriptionEditorEcho("s0");
	echo.emit("s1");
	assert.equal(echo.receive("s1"), "echo");
	assert.equal(echo.receive("ext"), "external");
	assert.equal(echo.isCurrent("ext"), true);
	assert.equal(echo.receive("ext"), "echo");
});

test("a stale echo state does not swallow the next external value", () => {
	const echo = new DescriptionEditorEcho("s0");
	echo.emit("s1");
	assert.equal(echo.receive("s1"), "echo");
	assert.equal(echo.receive("s0"), "external");
});

test("undo path: adopt, then emit, then both echoes make no new edit", () => {
	const echo = new DescriptionEditorEcho("s2");
	echo.adopt("s1");
	assert.equal(echo.isCurrent("s1"), true);
	echo.emit("s1");
	assert.equal(echo.receive("s1"), "echo");
	assert.equal(echo.isCurrent("s1"), true);
	assert.equal(echo.receive("s0"), "external");
});

test("the editor's current signature with nothing pending is not external", () => {
	const echo = new DescriptionEditorEcho("s0");
	assert.equal(echo.receive("s0"), "echo");
	assert.equal(echo.receive("s0"), "echo");
});
