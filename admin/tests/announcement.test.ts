import assert from "node:assert/strict";
import test from "node:test";
import { NotificationAudience } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import {
	announcementFormError,
	audienceText,
	createAnnouncementRequest,
	emptyAnnouncementForm,
	toggleAudience,
} from "../src/lib/announcement";

test("受眾顯示文字：買家與賣家，其他顯示未知", () => {
	assert.equal(audienceText(NotificationAudience.BUYER), "買家");
	assert.equal(audienceText(NotificationAudience.STORE), "賣家");
	assert.equal(audienceText(NotificationAudience.UNSPECIFIED), "未知");
});

test("標題或內容只有空白時不能送出，沒選對象也不能送出", () => {
	const form = emptyAnnouncementForm();
	assert.equal(announcementFormError(form), "請填入標題和內容");
	assert.equal(
		announcementFormError({ ...form, title: "  ", body: "x" }),
		"請填入標題和內容",
	);
	assert.equal(
		announcementFormError({ ...form, title: "t", body: "b", audiences: [] }),
		"請至少選一個發送對象",
	);
	assert.equal(announcementFormError({ ...form, title: "t", body: "b" }), null);
});

test("切換受眾保持固定順序", () => {
	const both = [NotificationAudience.BUYER, NotificationAudience.STORE];
	assert.deepEqual(toggleAudience(both, NotificationAudience.BUYER), [
		NotificationAudience.STORE,
	]);
	assert.deepEqual(
		toggleAudience([NotificationAudience.STORE], NotificationAudience.BUYER),
		both,
	);
});

test("建立請求帶修剪後的標題與內容、受眾與 request_id", () => {
	const form = {
		title: " Maintenance ",
		body: "Down at 2am\n",
		audiences: [NotificationAudience.STORE],
	};
	assert.deepEqual(createAnnouncementRequest(form, "req-1"), {
		audiences: [NotificationAudience.STORE],
		title: "Maintenance",
		body: "Down at 2am",
		requestId: "req-1",
	});
});
