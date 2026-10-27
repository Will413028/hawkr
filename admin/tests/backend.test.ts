import assert from "node:assert/strict";
import test from "node:test";
import { resolveBackendBaseUrl } from "../src/services/backend";

test("uses BACKEND_BASE_URL when set", () => {
	assert.equal(
		resolveBackendBaseUrl({ BACKEND_BASE_URL: "https://api.example.test" }),
		"https://api.example.test",
	);
});

test("trims trailing slash", () => {
	assert.equal(
		resolveBackendBaseUrl({ BACKEND_BASE_URL: "https://api.example.test/" }),
		"https://api.example.test",
	);
});

test("falls back to the local monolith when unset", () => {
	assert.equal(resolveBackendBaseUrl({}), "http://localhost:8080");
});

test("falls back to the local monolith when empty string", () => {
	assert.equal(
		resolveBackendBaseUrl({ BACKEND_BASE_URL: "" }),
		"http://localhost:8080",
	);
});
