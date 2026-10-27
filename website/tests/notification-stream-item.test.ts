import assert from "node:assert/strict";
import test from "node:test";
import { normalizeNotificationStreamItem } from "../lib/notifications/stream-item";

test("notification.new keeps the workspace role and data keys", () => {
	const item = normalizeNotificationStreamItem({
		id: "9007199254740993",
		userId: "42",
		userType: "SELLER",
		type: "merchant.verification.pending_info",
		title: "請補充實名認證資料",
		isRead: false,
		createdAt: "2026-08-27T09:00:00Z",
		data: {
			type: "MERCHANT.VERIFICATION.PENDING_INFO",
			refType: "merchant_verification",
			refId: 7001,
			attempts: 2,
		},
	});

	assert.equal(item.userType, "seller");
	assert.equal(item.accountId, undefined);
	assert.equal(item.refType, "merchant_verification");
	assert.equal(item.refId, "7001");
	assert.deepEqual(item.data, {
		type: "MERCHANT.VERIFICATION.PENDING_INFO",
		refType: "merchant_verification",
		refId: "7001",
		attempts: "2",
	});
});

test("notification.new never treats the role recipient key as an account id", () => {
	const item = normalizeNotificationStreamItem({
		id: 2,
		accountId: "9001",
		userId: "42",
		userType: "seller",
		type: "system",
		title: "系統通知",
		isRead: false,
		createdAt: "2026-08-27T09:00:00Z",
	});

	assert.equal(item.accountId, "9001");
});

test("an unknown workspace role stays unknown instead of defaulting to buyer", () => {
	const item = normalizeNotificationStreamItem({
		id: 1,
		userType: "admin",
		type: "system",
		title: "系統通知",
		isRead: false,
		createdAt: "2026-08-27T09:00:00Z",
	});

	assert.equal(item.userType, undefined);
});
