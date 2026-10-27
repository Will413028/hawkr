import assert from "node:assert/strict";
import test from "node:test";
import {
	isForbiddenProductionMediaReference,
	isProductionArtifact,
	TEST_MEDIA_HOSTNAME,
	testMediaHostname,
} from "./e2e/helpers/production-media-guard";

test("the media guard is enabled only for an explicit production artifact", () => {
	assert.equal(isProductionArtifact("prod"), true);
	assert.equal(isProductionArtifact(" PROD "), true);
	assert.equal(isProductionArtifact("dev"), false);
	assert.equal(isProductionArtifact(undefined), false);

	const testMediaUrl = `https://${TEST_MEDIA_HOSTNAME}/i/avatar/public/user/1.png`;
	assert.equal(isForbiddenProductionMediaReference("prod", testMediaUrl), true);
	assert.equal(
		isForbiddenProductionMediaReference("dev", testMediaUrl),
		false,
		"the dev artifact legitimately uses the test media gateway",
	);
});

test("the production media guard compares parsed hostnames instead of URL substrings", () => {
	assert.equal(
		isForbiddenProductionMediaReference(
			"prod",
			`https://${TEST_MEDIA_HOSTNAME.toUpperCase()}./image.png`,
		),
		true,
	);
	assert.equal(
		isForbiddenProductionMediaReference(
			"prod",
			`/proxy?next=https://${TEST_MEDIA_HOSTNAME}/image.png`,
			"https://example.com",
		),
		false,
	);
	assert.equal(
		isForbiddenProductionMediaReference(
			"prod",
			`https://${TEST_MEDIA_HOSTNAME}.example.com/image.png`,
		),
		false,
	);
	assert.equal(
		isForbiddenProductionMediaReference(
			"prod",
			"https://media.example.com/image.png",
		),
		false,
	);
});

test("the test media hostname follows NEXT_PUBLIC_MEDIA_BASE_URL, the app's own media setting", () => {
	assert.equal(
		testMediaHostname({
			NEXT_PUBLIC_MEDIA_BASE_URL: "https://Media.Test.Example:8443/x",
		}),
		"media.test.example",
	);
	assert.equal(testMediaHostname({}), "test-media.example.com");
	assert.equal(
		testMediaHostname({ NEXT_PUBLIC_MEDIA_BASE_URL: "not a url" }),
		"test-media.example.com",
	);
});
