import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path: string) => readFileSync(path, "utf8");
const dialog = read("src/app/admin/announcements/CreateAnnouncementDialog.tsx");
const page = read("src/app/admin/announcements/page.tsx");
const sidebar = read("src/components/Layout/Sidebar.tsx");

test("建立公告走 NotificationAdminService.CreateAnnouncement，請求由 createAnnouncementRequest 組出", () => {
	assert.match(
		dialog,
		/useMutation\(\s*NotificationAdminService\.method\.createAnnouncement,?\s*\)/,
	);
	assert.match(
		dialog,
		/create\.mutateAsync\(\s*createAnnouncementRequest\(form, attempt\.requestId\),?\s*\)/,
	);
	// request_id 由 announcementAttempt 依內容決定：內容不變才沿用，改過就換新的。
	assert.match(dialog, /announcementAttempt\(form, attemptRef\.current/);
	assert.equal(dialog.match(/crypto\.randomUUID\(\)/g)?.length, 1);
	// 送出前先驗欄位，不通過就不呼叫。
	assert.ok(
		dialog.indexOf("announcementFormError(form)") <
			dialog.indexOf("create.mutateAsync"),
	);
});

test("建立成功後列表失效重讀", () => {
	assert.match(
		dialog,
		/schema:\s*NotificationAdminService\.method\.listAnnouncements/,
	);
	assert.match(dialog, /invalidateQueries/);
});

test("列表走 ListAnnouncements，側欄指向公告頁而不是模板頁", () => {
	assert.match(
		page,
		/useQuery\(NotificationAdminService\.method\.listAnnouncements/,
	);
	assert.match(sidebar, /href: "\/admin\/announcements"/);
	assert.doesNotMatch(sidebar, /notification-templates/);
});
